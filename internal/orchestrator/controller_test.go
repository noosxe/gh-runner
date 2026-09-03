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
