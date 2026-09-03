package orchestrator_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/noosxe/gh-runner/internal/db"
	"github.com/noosxe/gh-runner/internal/orchestrator"
	"github.com/noosxe/gh-runner/internal/provider"
	"github.com/noosxe/gh-runner/internal/server"
)

type mockPoolRepo struct {
	pools []db.RunnerPool
	err   error
}

func (m *mockPoolRepo) ListRunnerPools(ctx context.Context) ([]db.RunnerPool, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.pools, nil
}

type mockGitProviderResolver struct {
	providers map[int64]provider.GitProvider
	err       error
}

func (m *mockGitProviderResolver) ResolveProvider(ctx context.Context, authProfileID int64) (provider.GitProvider, error) {
	if m.err != nil {
		return nil, m.err
	}
	p, ok := m.providers[authProfileID]
	if !ok {
		return nil, errors.New("provider not found")
	}
	return p, nil
}

type mockGitProvider struct {
	tokensIssued []string
	validateErr  error
	tokenErr     error
}

func (m *mockGitProvider) GetRegistrationToken(ctx context.Context, scope provider.RegistrationScope, targetURL string) (string, error) {
	if m.tokenErr != nil {
		return "", m.tokenErr
	}
	token := "reg-token-mock"
	m.tokensIssued = append(m.tokensIssued, token)
	return token, nil
}

func (m *mockGitProvider) ValidateCredentials(ctx context.Context) error {
	return m.validateErr
}

func (m *mockGitProvider) ScalingMode() provider.ScalingMode {
	return provider.ScalingWebhook
}

func (m *mockGitProvider) PollQueuedJobs(ctx context.Context, targetURL string) (int, error) {
	return 0, nil
}

func TestPoolController_BootAndMinIdleProvisioning(t *testing.T) {
	ctx := context.Background()

	pool := db.RunnerPool{
		ID:             1,
		Name:           "ci-pool",
		Provider:       "github",
		RepositoryUrl:  "https://github.com/owner/repo",
		Scope:          "repo",
		AuthProfileID:  10,
		MinIdleRunners: 3,
		MaxConcurrency: 5,
		Labels:         `["self-hosted","linux","arm64"]`,
		RunnerImage:    "ghcr.io/noosxe/gh-runner:latest",
		AllowDocker:    true,
		CpuLimit:       sql.NullString{String: "2", Valid: true},
		MemoryLimit:    sql.NullString{String: "4g", Valid: true},
	}

	repo := &mockPoolRepo{pools: []db.RunnerPool{pool}}
	gitProv := &mockGitProvider{}
	resolver := &mockGitProviderResolver{
		providers: map[int64]provider.GitProvider{10: gitProv},
	}

	spawnCount := 0
	mockEngine := &orchestrator.MockContainerProvider{
		SpawnRunnerFn: func(ctx context.Context, config orchestrator.RunnerConfig) (string, error) {
			spawnCount++
			if config.PoolName != "ci-pool" {
				t.Errorf("expected pool ci-pool, got %q", config.PoolName)
			}
			hasToken := false
			hasEphemeral := false
			for _, e := range config.Env {
				if e == "RUNNER_TOKEN=reg-token-mock" {
					hasToken = true
				}
				if e == "RUNNER_EPHEMERAL=1" {
					hasEphemeral = true
				}
			}
			if !hasToken {
				t.Errorf("expected RUNNER_TOKEN injected, got %v", config.Env)
			}
			if !hasEphemeral {
				t.Errorf("expected RUNNER_EPHEMERAL=1, got %v", config.Env)
			}
			if config.CPULimit != "2" || config.MemoryLimit != "4g" {
				t.Errorf("unexpected limits: cpu=%s mem=%s", config.CPULimit, config.MemoryLimit)
			}
			return "container-mock-" + config.Name, nil
		},
	}

	reconciler := orchestrator.NewReconciler(mockEngine)
	ctrl := orchestrator.NewPoolController(orchestrator.ControllerOptions{
		DB:               repo,
		ContainerEngine:  mockEngine,
		ProviderResolver: resolver,
		Reconciler:       reconciler,
	})

	// Initial check before boot
	probe := ctrl.ReadinessCheck()
	if status := probe.Check(ctx); status != server.StatusFail {
		t.Errorf("expected StatusFail before boot, got %v", status)
	}

	// 1. Boot sequence
	if err := ctrl.Boot(ctx); err != nil {
		t.Fatalf("ctrl.Boot failed: %v", err)
	}

	if ctrl.State() != orchestrator.StateRunning {
		t.Fatalf("expected state StateRunning, got %v", ctrl.State())
	}
	if spawnCount != 3 {
		t.Fatalf("expected 3 runners spawned on boot to satisfy min_idle_runners=3, got %d", spawnCount)
	}

	// Readiness check should now pass
	if status := probe.Check(ctx); status != server.StatusOK {
		t.Errorf("expected StatusOK after boot, got %v", status)
	}

	tracked := reconciler.TrackedPoolRunners("ci-pool")
	if len(tracked) != 3 {
		t.Fatalf("expected 3 tracked runners in reconciler, got %d", len(tracked))
	}

	// 2. Acceptance: Maintain pool of N idle runners from empty state
	// Simulate 1 runner exiting (finished job)
	mockEngine.AuditRunnersFn = func(ctx context.Context) ([]orchestrator.RunnerStatus, error) {
		return []orchestrator.RunnerStatus{
			{ID: tracked[0].ID, PoolName: "ci-pool", State: "running"},
			{ID: tracked[1].ID, PoolName: "ci-pool", State: "running"},
			{ID: tracked[2].ID, PoolName: "ci-pool", State: "exited"}, // Exited!
		}, nil
	}

	// Run Reconcile: should detect only 2 running and spawn 1 replacement
	if err := ctrl.Reconcile(ctx); err != nil {
		t.Fatalf("ctrl.Reconcile failed: %v", err)
	}

	if spawnCount != 4 {
		t.Fatalf("expected 4 total spawns after 1 exited runner replaced, got %d", spawnCount)
	}

	// 3. Test Pause and Resume
	ctrl.Pause()
	if ctrl.State() != orchestrator.StatePaused {
		t.Fatalf("expected StatePaused, got %v", ctrl.State())
	}

	// Simulate another exit while paused
	mockEngine.AuditRunnersFn = func(ctx context.Context) ([]orchestrator.RunnerStatus, error) {
		return []orchestrator.RunnerStatus{
			{ID: tracked[0].ID, PoolName: "ci-pool", State: "running"},
		}, nil
	}

	// Reconcile while paused should not spawn anything
	if err := ctrl.Reconcile(ctx); err != nil {
		t.Fatalf("ctrl.Reconcile while paused failed: %v", err)
	}
	if spawnCount != 4 {
		t.Errorf("spawning should not happen while paused, count=%d", spawnCount)
	}

	// Resume and Reconcile: should spawn replacements
	ctrl.Resume()
	if ctrl.State() != orchestrator.StateRunning {
		t.Fatalf("expected StateRunning after resume, got %v", ctrl.State())
	}

	if err := ctrl.Reconcile(ctx); err != nil {
		t.Fatalf("ctrl.Reconcile after resume failed: %v", err)
	}
	if spawnCount <= 4 {
		t.Errorf("expected new spawns after resume, got %d", spawnCount)
	}
}

func TestPoolController_BootEngineUnreachable(t *testing.T) {
	ctx := context.Background()

	mockEngine := &orchestrator.MockContainerProvider{
		PingFn: func(ctx context.Context) error {
			return errors.New("cannot connect to docker.sock")
		},
	}

	ctrl := orchestrator.NewPoolController(orchestrator.ControllerOptions{
		ContainerEngine: mockEngine,
	})

	err := ctrl.Boot(ctx)
	if err == nil || !errors.Is(err, orchestrator.ErrEngineUnreachable) {
		t.Fatalf("expected ErrEngineUnreachable, got %v", err)
	}
	if ctrl.State() != orchestrator.StateStopped {
		t.Errorf("expected StateStopped, got %v", ctrl.State())
	}
}

func TestPoolController_HandleContainerEvent_ReapAndReplenish(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	pool := db.RunnerPool{
		ID:             1,
		Name:           "event-pool",
		Provider:       "github",
		RepositoryUrl:  "https://github.com/owner/event-repo",
		Scope:          "repo",
		AuthProfileID:  20,
		MinIdleRunners: 2,
		RunnerImage:    "ghcr.io/noosxe/gh-runner:latest",
	}

	repo := &mockPoolRepo{pools: []db.RunnerPool{pool}}
	tokensFetched := 0
	gitProv := &mockGitProvider{}
	gitProv.tokensIssued = make([]string, 0)
	resolver := &mockGitProviderResolver{
		providers: map[int64]provider.GitProvider{20: gitProv},
	}

	terminatedIDs := make([]string, 0)
	logsCapturedIDs := make([]string, 0)
	spawnedIDs := make([]string, 0)

	mockEngine := &orchestrator.MockContainerProvider{
		SpawnRunnerFn: func(ctx context.Context, config orchestrator.RunnerConfig) (string, error) {
			tokensFetched++
			id := "runner-" + config.Name
			spawnedIDs = append(spawnedIDs, id)
			return id, nil
		},
		TerminateRunnerFn: func(ctx context.Context, containerID string) error {
			terminatedIDs = append(terminatedIDs, containerID)
			return nil
		},
		CaptureLogsFn: func(ctx context.Context, containerID, dataDir string) (string, error) {
			logsCapturedIDs = append(logsCapturedIDs, containerID)
			return orchestrator.LogPath(dataDir, containerID), nil
		},
	}

	reconciler := orchestrator.NewReconciler(mockEngine)
	ctrl := orchestrator.NewPoolController(orchestrator.ControllerOptions{
		DB:               repo,
		ContainerEngine:  mockEngine,
		ProviderResolver: resolver,
		Reconciler:       reconciler,
		DataDir:          tempDir,
	})

	// 1. Boot up: spawns 2 idle runners
	if err := ctrl.Boot(ctx); err != nil {
		t.Fatalf("ctrl.Boot failed: %v", err)
	}

	if len(spawnedIDs) != 2 {
		t.Fatalf("expected 2 spawned on boot, got %d", len(spawnedIDs))
	}
	firstRunnerID := spawnedIDs[0]

	// 2. Container completes job and dies -> "die" event arrives
	event := orchestrator.ContainerEvent{
		ContainerID: firstRunnerID,
		PoolName:    "event-pool",
		Action:      "die",
		ExitCode:    0,
	}

	// Handle event: must capture logs, terminate dead container, and immediately replenish
	if err := ctrl.HandleContainerEvent(ctx, event); err != nil {
		t.Fatalf("HandleContainerEvent failed: %v", err)
	}

	// Acceptance: dead container reaped, exit logs captured before prune
	if len(logsCapturedIDs) != 1 || logsCapturedIDs[0] != firstRunnerID {
		t.Errorf("expected logs captured for %s, got: %v", firstRunnerID, logsCapturedIDs)
	}
	if len(terminatedIDs) != 1 || terminatedIDs[0] != firstRunnerID {
		t.Errorf("expected container %s terminated, got: %v", firstRunnerID, terminatedIDs)
	}

	// Replacement runner spawned with fresh token, converging back to target 2 idle runners
	if len(spawnedIDs) != 3 {
		t.Fatalf("expected 3 total spawns (2 initial + 1 replacement), got %d", len(spawnedIDs))
	}
	if tokensFetched != 3 {
		t.Errorf("expected fresh token requested per spawn (3 total), got %d", tokensFetched)
	}

	tracked := reconciler.TrackedPoolRunners("event-pool")
	if len(tracked) != 2 {
		t.Fatalf("expected 2 active runners in pool, got %d", len(tracked))
	}
}

func TestPoolController_GlobalQuotaSaturationAndFairQueueDrain(t *testing.T) {
	ctx := context.Background()

	poolA := db.RunnerPool{
		ID:             1,
		Name:           "pool-a",
		Provider:       "github",
		RepositoryUrl:  "https://github.com/owner/repo-a",
		Scope:          "repo",
		AuthProfileID:  10,
		MinIdleRunners: 2,
		MaxConcurrency: 2,
		RunnerImage:    "ghcr.io/noosxe/gh-runner:latest",
	}
	poolB := db.RunnerPool{
		ID:             2,
		Name:           "pool-b",
		Provider:       "github",
		RepositoryUrl:  "https://github.com/owner/repo-b",
		Scope:          "repo",
		AuthProfileID:  10,
		MinIdleRunners: 2,
		MaxConcurrency: 2,
		RunnerImage:    "ghcr.io/noosxe/gh-runner:latest",
	}

	repo := &mockPoolRepo{pools: []db.RunnerPool{poolA, poolB}}
	gitProv := &mockGitProvider{}
	resolver := &mockGitProviderResolver{
		providers: map[int64]provider.GitProvider{10: gitProv},
	}

	spawnedByPool := make(map[string][]string)
	mockEngine := &orchestrator.MockContainerProvider{
		SpawnRunnerFn: func(ctx context.Context, config orchestrator.RunnerConfig) (string, error) {
			id := "runner-" + config.Name
			spawnedByPool[config.PoolName] = append(spawnedByPool[config.PoolName], id)
			return id, nil
		},
		TerminateRunnerFn: func(ctx context.Context, containerID string) error {
			return nil
		},
	}

	reconciler := orchestrator.NewReconciler(mockEngine)
	// Set GlobalMaxRunners = 3 (while poolA + poolB total target is 4)
	ctrl := orchestrator.NewPoolController(orchestrator.ControllerOptions{
		DB:               repo,
		ContainerEngine:  mockEngine,
		ProviderResolver: resolver,
		Reconciler:       reconciler,
		GlobalMaxRunners: 3,
	})

	// 1. Boot controller: should hit circuit breaker at 3 runners and queue the 4th request
	if err := ctrl.Boot(ctx); err != nil {
		t.Fatalf("ctrl.Boot failed: %v", err)
	}

	if ctrl.TotalActiveRunners() != 3 {
		t.Fatalf("expected exactly 3 total active runners (at circuit breaker limit), got %d", ctrl.TotalActiveRunners())
	}
	if ctrl.QueueLength() != 1 {
		t.Fatalf("expected 1 request queued for pool-b due to saturation, got %d", ctrl.QueueLength())
	}
	if ctrl.QueueLengthForPool("pool-b") != 1 {
		t.Errorf("expected queued request to be for pool-b")
	}

	// Verify poolA has 2 and poolB has 1 active runner
	if len(spawnedByPool["pool-a"]) != 2 {
		t.Errorf("expected 2 spawns for pool-a, got %d", len(spawnedByPool["pool-a"]))
	}
	if len(spawnedByPool["pool-b"]) != 1 {
		t.Errorf("expected 1 spawn for pool-b, got %d", len(spawnedByPool["pool-b"]))
	}

	// 2. Terminate a container in pool-a
	runnerA1 := spawnedByPool["pool-a"][0]
	event := orchestrator.ContainerEvent{
		ContainerID: runnerA1,
		PoolName:    "pool-a",
		Action:      "die",
		ExitCode:    0,
	}

	// When container terminates, capacity frees up -> queue drains pool-b request
	// Since pool-b takes the freed slot (reaching 3 active), pool-a's replenishment is queued
	if err := ctrl.HandleContainerEvent(ctx, event); err != nil {
		t.Fatalf("HandleContainerEvent failed: %v", err)
	}

	// Queue for pool-b should now be drained
	if ctrl.QueueLengthForPool("pool-b") != 0 {
		t.Errorf("expected pool-b queue to be drained after capacity freed, got length %d", ctrl.QueueLengthForPool("pool-b"))
	}

	// Total active runners must still never exceed GlobalMaxRunners (3)
	if ctrl.TotalActiveRunners() != 3 {
		t.Fatalf("expected total active runners to stay at global limit 3: got %d", ctrl.TotalActiveRunners())
	}

	// Pool B reached its target of 2 idle runners from queue drain
	if len(spawnedByPool["pool-b"]) != 2 {
		t.Errorf("expected pool-b to have received its 2nd runner from queue drain, got %d", len(spawnedByPool["pool-b"]))
	}

	// Pool A's replenishment request is now queued because capacity is at 3/3
	if ctrl.QueueLengthForPool("pool-a") != 1 {
		t.Errorf("expected pool-a replenishment to be queued, got %d", ctrl.QueueLengthForPool("pool-a"))
	}

	// 3. Now terminate a container in pool-b to free capacity for pool-a's queued request
	runnerB1 := spawnedByPool["pool-b"][0]
	eventB := orchestrator.ContainerEvent{
		ContainerID: runnerB1,
		PoolName:    "pool-b",
		Action:      "die",
		ExitCode:    0,
	}

	if err := ctrl.HandleContainerEvent(ctx, eventB); err != nil {
		t.Fatalf("HandleContainerEvent for pool-b failed: %v", err)
	}

	// Now pool-a's queued request should have drained and spawned!
	if ctrl.QueueLengthForPool("pool-a") != 0 {
		t.Errorf("expected pool-a queue to be drained, got %d", ctrl.QueueLengthForPool("pool-a"))
	}
	if ctrl.TotalActiveRunners() != 3 {
		t.Fatalf("expected total active runners to remain at global limit 3, got %d", ctrl.TotalActiveRunners())
	}
}

type mockJobRecorder struct {
	records []struct {
		poolID     int64
		runnerName string
		status     string
		logPath    string
	}
}

func (m *mockJobRecorder) RecordJobTimeout(ctx context.Context, poolID int64, runnerName, logPath string, startedAt, completedAt time.Time) error {
	m.records = append(m.records, struct {
		poolID     int64
		runnerName string
		status     string
		logPath    string
	}{
		poolID:     poolID,
		runnerName: runnerName,
		status:     "timeout",
		logPath:    logPath,
	})
	return nil
}

func TestPoolController_HungRunnerAutoTermination(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	pool := db.RunnerPool{
		ID:                       42,
		Name:                     "timeout-pool",
		Provider:                 "github",
		RepositoryUrl:            "https://github.com/owner/timeout-repo",
		Scope:                    "repo",
		AuthProfileID:            10,
		MinIdleRunners:           2,
		MaxConcurrency:           5,
		MaxRunnerLifetimeSeconds: 5, // 5 second limit
		RunnerImage:              "ghcr.io/noosxe/gh-runner:latest",
	}

	repo := &mockPoolRepo{pools: []db.RunnerPool{pool}}
	gitProv := &mockGitProvider{}
	resolver := &mockGitProviderResolver{
		providers: map[int64]provider.GitProvider{10: gitProv},
	}

	terminatedIDs := make([]string, 0)
	logsCapturedIDs := make([]string, 0)
	mockEngine := &orchestrator.MockContainerProvider{
		SpawnRunnerFn: func(ctx context.Context, config orchestrator.RunnerConfig) (string, error) {
			return "runner-" + config.Name, nil
		},
		TerminateRunnerFn: func(ctx context.Context, containerID string) error {
			terminatedIDs = append(terminatedIDs, containerID)
			return nil
		},
		CaptureLogsFn: func(ctx context.Context, containerID, dataDir string) (string, error) {
			logsCapturedIDs = append(logsCapturedIDs, containerID)
			return orchestrator.LogPath(dataDir, containerID), nil
		},
	}

	reconciler := orchestrator.NewReconciler(mockEngine)
	jobRecorder := &mockJobRecorder{}

	ctrl := orchestrator.NewPoolController(orchestrator.ControllerOptions{
		DB:               repo,
		JobRecorder:      jobRecorder,
		ContainerEngine:  mockEngine,
		ProviderResolver: resolver,
		Reconciler:       reconciler,
		DataDir:          tempDir,
	})

	// Add a synthetic hung container spawned 10 seconds ago (limit is 5s)
	hungRunner := orchestrator.RunnerStatus{
		ID:        "hung-container-1",
		Name:      "hung-runner-1",
		PoolName:  "timeout-pool",
		State:     "running",
		SpawnedAt: time.Now().UTC().Add(-10 * time.Second),
	}
	// Add a healthy fresh container spawned 1 second ago
	freshRunner := orchestrator.RunnerStatus{
		ID:        "fresh-container-2",
		Name:      "fresh-runner-2",
		PoolName:  "timeout-pool",
		State:     "running",
		SpawnedAt: time.Now().UTC().Add(-1 * time.Second),
	}

	reconciler.TrackRunner(hungRunner)
	reconciler.TrackRunner(freshRunner)

	if len(reconciler.TrackedPoolRunners("timeout-pool")) != 2 {
		t.Fatalf("expected 2 runners tracked initially")
	}

	// Trigger hung runner inspection
	if err := ctrl.CheckHungRunners(ctx); err != nil {
		t.Fatalf("CheckHungRunners failed: %v", err)
	}

	// 1. Acceptance: synthetic hung container killed at limit
	if len(terminatedIDs) != 1 || terminatedIDs[0] != "hung-container-1" {
		t.Fatalf("expected hung-container-1 to be terminated, got: %v", terminatedIDs)
	}

	// 2. Logs captured before container termination
	if len(logsCapturedIDs) != 1 || logsCapturedIDs[0] != "hung-container-1" {
		t.Errorf("expected logs captured for hung-container-1, got: %v", logsCapturedIDs)
	}

	// 3. job_history record created with status 'timeout'
	if len(jobRecorder.records) != 1 {
		t.Fatalf("expected 1 job_history record, got %d", len(jobRecorder.records))
	}
	record := jobRecorder.records[0]
	if record.poolID != 42 || record.runnerName != "hung-runner-1" || record.status != "timeout" {
		t.Errorf("unexpected timeout record: %+v", record)
	}

	// 4. Fresh runner is NOT terminated and remains tracked
	tracked := reconciler.TrackedPoolRunners("timeout-pool")
	if len(tracked) != 1 || tracked[0].ID != "fresh-container-2" {
		t.Errorf("fresh container should still be running, tracked: %+v", tracked)
	}
}

