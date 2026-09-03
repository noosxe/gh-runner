package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/noosxe/gh-runner/internal/db"
	"github.com/noosxe/gh-runner/internal/logging"
	"github.com/noosxe/gh-runner/internal/provider"
	"github.com/noosxe/gh-runner/internal/server"
)

var (
	// ErrControllerStopped is returned when an operation cannot be performed on a stopped controller.
	ErrControllerStopped = errors.New("controller is stopped")
	// ErrEngineUnreachable is returned when the container engine is unreachable during boot.
	ErrEngineUnreachable = errors.New("container engine is unreachable")
)

const (
	// DefaultControlLoopInterval is the default period between reconciliation cycles (10s per docs/03 §3).
	DefaultControlLoopInterval = 10 * time.Second

	// DefaultHeartbeatTimeout is the maximum duration between heartbeats before the auditor is marked degraded.
	DefaultHeartbeatTimeout = 30 * time.Second
)

// PoolRepository abstracts loading active runner pools from the database.
type PoolRepository interface {
	ListRunnerPools(ctx context.Context) ([]db.RunnerPool, error)
}

// GitProviderResolver resolves a GitProvider instance for a given auth profile ID.
type GitProviderResolver interface {
	ResolveProvider(ctx context.Context, authProfileID int64) (provider.GitProvider, error)
}

// RegistryAdapter adapts a *provider.Registry and *db.DB into a GitProviderResolver.
type RegistryAdapter struct {
	Database *db.DB
	Registry *provider.Registry
}

// ResolveProvider loads the decrypted auth profile and builds the GitProvider.
func (a *RegistryAdapter) ResolveProvider(ctx context.Context, authProfileID int64) (provider.GitProvider, error) {
	if a.Database == nil || a.Registry == nil {
		return nil, fmt.Errorf("database or provider registry is nil")
	}
	return a.Registry.BuildFromDB(ctx, a.Database, authProfileID)
}

// ControllerState represents the operational status of the lifecycle controller.
type ControllerState string

const (
	StateStopped ControllerState = "stopped"
	StateBooting ControllerState = "booting"
	StateRunning ControllerState = "running"
	StatePaused  ControllerState = "paused"
)

// ControllerOptions configures the PoolController.
type ControllerOptions struct {
	DB               PoolRepository
	ContainerEngine  ContainerProvider
	ProviderResolver GitProviderResolver
	Reconciler       *Reconciler
	Interval         time.Duration
}

// PoolController orchestrates the lifecycle control loop across all runner pools (docs/03 §1).
type PoolController struct {
	mu               sync.RWMutex
	db               PoolRepository
	engine           ContainerProvider
	providerResolver GitProviderResolver
	reconciler       *Reconciler
	interval         time.Duration

	state         ControllerState
	lastHeartbeat time.Time
	logger        *slog.Logger
}

// NewPoolController creates a new lifecycle control loop engine.
func NewPoolController(opts ControllerOptions) *PoolController {
	if opts.Interval <= 0 {
		opts.Interval = DefaultControlLoopInterval
	}
	if opts.Reconciler == nil && opts.ContainerEngine != nil {
		opts.Reconciler = NewReconciler(opts.ContainerEngine)
	}

	return &PoolController{
		db:               opts.DB,
		engine:           opts.ContainerEngine,
		providerResolver: opts.ProviderResolver,
		reconciler:       opts.Reconciler,
		interval:         opts.Interval,
		state:            StateStopped,
		logger:           logging.For("controller"),
	}
}

// State returns the current lifecycle state of the controller.
func (c *PoolController) State() ControllerState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Pause temporarily halts runner provisioning and pool reconciliation.
func (c *PoolController) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateRunning {
		c.state = StatePaused
		c.logger.Info("pool controller paused")
	}
}

// Resume unpauses the controller, restoring active reconciliation.
func (c *PoolController) Resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StatePaused {
		c.state = StateRunning
		c.logger.Info("pool controller resumed")
	}
}

// Boot executes the structured startup sequence per docs/03 §1:
// 1. Rebuild in-memory container state from host engine
// 2. Verify container engine connectivity
// 3. Load active runner pools from DB
// 4. Validate provider credentials
// 5. Provision min_idle_runners per pool
func (c *PoolController) Boot(ctx context.Context) error {
	c.mu.Lock()
	c.state = StateBooting
	c.mu.Unlock()

	c.logger.Info("executing control loop boot sequence")

	// 1. Rebuild state by querying supervisor-managed containers
	if c.reconciler != nil {
		report, err := c.reconciler.RebuildState(ctx)
		if err != nil {
			c.logger.Warn("failed to rebuild state from engine during boot", "err", err)
		} else {
			c.logger.Info("boot state sync completed", "adopted", len(report.Adopted), "active", len(report.Active))
		}
	}

	// 2. Verify container engine connectivity
	if c.engine != nil {
		if err := c.engine.Ping(ctx); err != nil {
			c.mu.Lock()
			c.state = StateStopped
			c.mu.Unlock()
			return fmt.Errorf("%w: %v", ErrEngineUnreachable, err)
		}
	}

	// 3. Load active pools
	pools, err := c.loadPools(ctx)
	if err != nil {
		c.mu.Lock()
		c.state = StateStopped
		c.mu.Unlock()
		return fmt.Errorf("loading pools during boot: %w", err)
	}

	// 4. Validate provider credentials & 5. Provision min_idle_runners
	for _, p := range pools {
		if err := c.validateAndProvisionPool(ctx, p); err != nil {
			c.logger.Error("failed validating or provisioning pool on boot", "pool", p.Name, "err", err)
		}
	}

	c.mu.Lock()
	c.state = StateRunning
	c.lastHeartbeat = time.Now()
	c.mu.Unlock()

	c.logger.Info("control loop boot completed successfully", "pools", len(pools))
	return nil
}

// Reconcile executes a single reconciliation cycle across all pools:
// audits running containers, detects exited containers, and provisions replacements to maintain min_idle_runners.
func (c *PoolController) Reconcile(ctx context.Context) error {
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	if state == StatePaused || state == StateStopped {
		return nil
	}

	// 1. Audit containers across all pools
	if c.reconciler != nil {
		if _, err := c.reconciler.Audit(ctx); err != nil {
			c.logger.Warn("audit cycle failed", "err", err)
		}
	}

	// 2. Load active pools
	pools, err := c.loadPools(ctx)
	if err != nil {
		return fmt.Errorf("loading pools for reconcile: %w", err)
	}

	// 3. Reconcile pool targets
	for _, p := range pools {
		if err := c.reconcilePool(ctx, p); err != nil {
			c.logger.Error("failed reconciling pool target", "pool", p.Name, "err", err)
		}
	}

	c.mu.Lock()
	c.lastHeartbeat = time.Now()
	c.mu.Unlock()

	return nil
}

// Start boots the controller and runs the continuous periodic reconciliation loop until ctx is canceled.
func (c *PoolController) Start(ctx context.Context) error {
	if err := c.Boot(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			c.state = StateStopped
			c.mu.Unlock()
			c.logger.Info("control loop terminated by context")
			return ctx.Err()

		case <-ticker.C:
			if err := c.Reconcile(ctx); err != nil {
				c.logger.Warn("reconciliation cycle error", "err", err)
			}
		}
	}
}

// ReadinessCheck adapts the control loop's heartbeat into a server.Check probe (OQ #19).
func (c *PoolController) ReadinessCheck() server.Check {
	return server.NewCheck("auditor", func(ctx context.Context) server.Status {
		c.mu.RLock()
		defer c.mu.RUnlock()

		if c.state == StateStopped {
			return server.StatusFail
		}
		if c.lastHeartbeat.IsZero() || time.Since(c.lastHeartbeat) > DefaultHeartbeatTimeout {
			return server.StatusDegraded
		}
		return server.StatusOK
	})
}

func (c *PoolController) loadPools(ctx context.Context) ([]db.RunnerPool, error) {
	if c.db == nil {
		return nil, nil
	}
	return c.db.ListRunnerPools(ctx)
}

func (c *PoolController) validateAndProvisionPool(ctx context.Context, p db.RunnerPool) error {
	if c.providerResolver == nil {
		return nil
	}

	gitProv, err := c.providerResolver.ResolveProvider(ctx, p.AuthProfileID)
	if err != nil {
		return fmt.Errorf("resolving provider for pool %q: %w", p.Name, err)
	}

	if err := gitProv.ValidateCredentials(ctx); err != nil {
		return fmt.Errorf("validating provider credentials for pool %q: %w", p.Name, err)
	}

	return c.reconcilePoolWithProvider(ctx, p, gitProv)
}

func (c *PoolController) reconcilePool(ctx context.Context, p db.RunnerPool) error {
	if c.providerResolver == nil {
		return nil
	}
	gitProv, err := c.providerResolver.ResolveProvider(ctx, p.AuthProfileID)
	if err != nil {
		return fmt.Errorf("resolving provider: %w", err)
	}
	return c.reconcilePoolWithProvider(ctx, p, gitProv)
}

func (c *PoolController) reconcilePoolWithProvider(ctx context.Context, p db.RunnerPool, gitProv provider.GitProvider) error {
	if c.engine == nil || c.reconciler == nil {
		return nil
	}

	tracked := c.reconciler.TrackedPoolRunners(p.Name)
	activeCount := int64(0)
	for _, r := range tracked {
		if r.State == "running" {
			activeCount++
		}
	}

	needed := p.MinIdleRunners - activeCount
	if needed <= 0 {
		return nil
	}

	c.logger.Info("provisioning idle runners for pool", "pool", p.Name, "needed", needed, "active", activeCount, "target", p.MinIdleRunners)

	for i := int64(0); i < needed; i++ {
		token, err := gitProv.GetRegistrationToken(ctx, provider.RegistrationScope(p.Scope), p.RepositoryUrl)
		if err != nil {
			return fmt.Errorf("getting registration token for pool %q: %w", p.Name, err)
		}

		containerName := GenerateContainerName(p.Name)
		labels := formatLabels(p.Labels)

		env := []string{
			"GITHUB_REPOSITORY_URL=" + p.RepositoryUrl,
			"RUNNER_TOKEN=" + token,
			"RUNNER_NAME=" + containerName,
			"RUNNER_LABELS=" + labels,
			"RUNNER_WORKDIR=_work",
			"RUNNER_EPHEMERAL=1",
		}

		config := RunnerConfig{
			Name:        containerName,
			PoolName:    p.Name,
			Image:       p.RunnerImage,
			AllowDocker: p.AllowDocker,
			Env:         env,
		}
		if p.CpuLimit.Valid {
			config.CPULimit = p.CpuLimit.String
		}
		if p.MemoryLimit.Valid {
			config.MemoryLimit = p.MemoryLimit.String
		}

		id, err := c.engine.SpawnRunner(ctx, config)
		if err != nil {
			return fmt.Errorf("spawning runner for pool %q: %w", p.Name, err)
		}

		c.reconciler.TrackRunner(RunnerStatus{
			ID:        id,
			PoolName:  p.Name,
			State:     "running",
			SpawnedAt: time.Now().UTC(),
		})

		c.logger.Info("spawned idle runner", "pool", p.Name, "id", id)
	}

	return nil
}

func formatLabels(raw string) string {
	if raw == "" {
		return "self-hosted,linux"
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err == nil && len(arr) > 0 {
		return strings.Join(arr, ",")
	}
	return raw
}
