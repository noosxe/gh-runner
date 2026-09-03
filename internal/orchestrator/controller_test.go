package orchestrator_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

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
