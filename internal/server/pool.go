package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/noosxe/gh-runner/internal/cron"
	"github.com/noosxe/gh-runner/internal/db"
	supervisorv1 "github.com/noosxe/gh-runner/internal/pb/supervisor/v1"
	"github.com/noosxe/gh-runner/internal/pb/supervisor/v1/supervisorv1connect"
)

// PoolDatabase defines the database queries required by PoolService.
// *db.DB satisfies this interface.
type PoolDatabase interface {
	ListRunnerPools(ctx context.Context) ([]db.RunnerPool, error)
	GetRunnerPoolById(ctx context.Context, id int64) (db.RunnerPool, error)
	GetRunnerPoolByName(ctx context.Context, name string) (db.RunnerPool, error)
	CreateRunnerPool(ctx context.Context, arg db.CreateRunnerPoolParams) (db.RunnerPool, error)
	UpdateRunnerPool(ctx context.Context, arg db.UpdateRunnerPoolParams) (db.RunnerPool, error)
	DeleteRunnerPool(ctx context.Context, id int64) error
	CreateAuditLog(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error)
	GetRenovateConfigByPoolId(ctx context.Context, poolID int64) (db.RenovateConfig, error)
	CreateRenovateConfig(ctx context.Context, arg db.CreateRenovateConfigParams) (db.RenovateConfig, error)
	UpdateRenovateConfig(ctx context.Context, arg db.UpdateRenovateConfigParams) (db.RenovateConfig, error)
}

// PoolStatsProvider provides live active/idle runner counts and runtime reload capabilities.
// *orchestrator.PoolController satisfies this interface.
type PoolStatsProvider interface {
	PoolStats(poolName string) (active int32, idle int32)
	Reload(ctx context.Context) error
}

// RunnerInstanceInfo represents an active runner container's runtime state.
type RunnerInstanceInfo struct {
	ID        string
	Name      string
	PoolName  string
	State     string
	IPAddress string
	SpawnedAt time.Time
	IsBusy    bool
}

// RunnerManager provides live runner container inspection and manual kill operations.
// *orchestrator.PoolController satisfies this interface.
type RunnerManager interface {
	PoolRunners(poolName string) []RunnerInstanceInfo
	TerminateRunner(ctx context.Context, poolName, containerID string) error
}

// PoolService implements supervisorv1connect.PoolServiceHandler.
type PoolService struct {
	supervisorv1connect.UnimplementedPoolServiceHandler
	db            PoolDatabase
	statsProvider PoolStatsProvider
	runnerMgr     RunnerManager
}

// NewPoolService constructs a PoolService instance.
func NewPoolService(database PoolDatabase, statsProvider PoolStatsProvider, runnerMgr RunnerManager) *PoolService {
	return &PoolService{
		db:            database,
		statsProvider: statsProvider,
		runnerMgr:     runnerMgr,
	}
}

func parseLabels(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

func (s *PoolService) toProto(ctx context.Context, p db.RunnerPool) *supervisorv1.Pool {
	proto := ConvertDBPoolToProto(p, s.statsProvider)
	if s.db != nil {
		if cfg, err := s.db.GetRenovateConfigByPoolId(ctx, p.ID); err == nil {
			proto.Renovate = &supervisorv1.RenovateConfig{
				Enabled:      cfg.Enabled,
				CronSchedule: cfg.CronSchedule.String,
				Image:        cfg.Image,
			}
		}
	}
	return proto
}

// ConvertDBPoolToProto converts a db.RunnerPool row into a supervisorv1.Pool protobuf message,
// attaching runtime active/idle runner counts from the provided stats provider if available.
func ConvertDBPoolToProto(p db.RunnerPool, stats PoolStatsProvider) *supervisorv1.Pool {
	protoPool := &supervisorv1.Pool{
		Id:                       p.ID,
		Name:                     p.Name,
		Provider:                 p.Provider,
		RepositoryUrl:            p.RepositoryUrl,
		MinIdleRunners:           int32(p.MinIdleRunners),
		MaxConcurrency:           int32(p.MaxConcurrency),
		Labels:                   parseLabels(p.Labels),
		RunnerImage:              p.RunnerImage,
		AllowDocker:              p.AllowDocker,
		AuthProfileId:            p.AuthProfileID,
		Scope:                    p.Scope,
		CpuLimit:                 p.CpuLimit.String,
		MemoryLimit:              p.MemoryLimit.String,
		MaxRunnerLifetimeSeconds: int32(p.MaxRunnerLifetimeSeconds),
	}

	if stats != nil {
		active, idle := stats.PoolStats(p.Name)
		protoPool.ActiveRunners = active
		protoPool.IdleRunners = idle
	}

	return protoPool
}

func validatePoolInput(p *supervisorv1.Pool) error {
	if p == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("pool payload is required"))
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("pool name must not be empty"))
	}

	provider := strings.ToLower(strings.TrimSpace(p.Provider))
	switch provider {
	case "github", "gitea", "forgejo":
	default:
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported provider %q; must be 'github', 'gitea', or 'forgejo'", p.Provider))
	}

	if strings.TrimSpace(p.RepositoryUrl) == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("repository_url must not be empty"))
	}

	// Gitea and Forgejo require allow_docker=true (docs/05 §4)
	if (provider == "gitea" || provider == "forgejo") && !p.AllowDocker {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("gitea and forgejo pools require allow_docker=true (docs/05 §4)"))
	}

	if p.AuthProfileId <= 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("auth_profile_id must be a valid positive identifier"))
	}

	if p.MinIdleRunners < 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("min_idle_runners must be non-negative"))
	}
	if p.MaxConcurrency < 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("max_concurrency must be non-negative"))
	}
	if p.MaxRunnerLifetimeSeconds < 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("max_runner_lifetime_seconds must be non-negative"))
	}

	if p.Renovate != nil && p.Renovate.Enabled && strings.TrimSpace(p.Renovate.CronSchedule) != "" {
		if _, err := cron.ParseSchedule(strings.TrimSpace(p.Renovate.CronSchedule)); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid renovate cron schedule: %w", err))
		}
	}

	return nil
}

// ListPools retrieves all runner pools with live active/idle stats.
func (s *PoolService) ListPools(ctx context.Context, _ *connect.Request[supervisorv1.ListPoolsRequest]) (*connect.Response[supervisorv1.ListPoolsResponse], error) {
	pools, err := s.db.ListRunnerPools(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("listing runner pools: %w", err))
	}

	resp := &supervisorv1.ListPoolsResponse{
		Pools: make([]*supervisorv1.Pool, 0, len(pools)),
	}
	for _, p := range pools {
		resp.Pools = append(resp.Pools, s.toProto(ctx, p))
	}

	return connect.NewResponse(resp), nil
}

// CreatePool persists a new runner pool, creates an audit log, and notifies the controller loop.
func (s *PoolService) CreatePool(ctx context.Context, req *connect.Request[supervisorv1.CreatePoolRequest]) (*connect.Response[supervisorv1.CreatePoolResponse], error) {
	pool := req.Msg.Pool
	if err := validatePoolInput(pool); err != nil {
		return nil, err
	}

	scope := strings.TrimSpace(pool.Scope)
	if scope == "" {
		scope = "repo"
	}

	labelsStr := strings.Join(pool.Labels, ",")

	created, err := s.db.CreateRunnerPool(ctx, db.CreateRunnerPoolParams{
		Name:                     strings.TrimSpace(pool.Name),
		Provider:                 strings.ToLower(strings.TrimSpace(pool.Provider)),
		RepositoryUrl:            strings.TrimSpace(pool.RepositoryUrl),
		Scope:                    scope,
		AuthProfileID:            pool.AuthProfileId,
		MinIdleRunners:           int64(pool.MinIdleRunners),
		MaxConcurrency:           int64(pool.MaxConcurrency),
		Labels:                   labelsStr,
		RunnerImage:              strings.TrimSpace(pool.RunnerImage),
		AllowDocker:              pool.AllowDocker,
		MaxRunnerLifetimeSeconds: int64(pool.MaxRunnerLifetimeSeconds),
		CpuLimit:                 sql.NullString{String: pool.CpuLimit, Valid: pool.CpuLimit != ""},
		MemoryLimit:              sql.NullString{String: pool.MemoryLimit, Valid: pool.MemoryLimit != ""},
	})
	if err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("auth_profile_id %d does not exist", pool.AuthProfileId))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("creating runner pool: %w", err))
	}

	if pool.Renovate != nil {
		if pool.Renovate.Enabled && strings.TrimSpace(pool.Renovate.CronSchedule) != "" {
			if _, err := cron.ParseSchedule(strings.TrimSpace(pool.Renovate.CronSchedule)); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid renovate cron schedule: %w", err))
			}
		}
		img := strings.TrimSpace(pool.Renovate.Image)
		if img == "" {
			img = "renovate/renovate:latest"
		}
		cronSched := strings.TrimSpace(pool.Renovate.CronSchedule)
		_, _ = s.db.CreateRenovateConfig(ctx, db.CreateRenovateConfigParams{
			PoolID:       created.ID,
			Enabled:      pool.Renovate.Enabled,
			CronSchedule: sql.NullString{String: cronSched, Valid: cronSched != ""},
			Image:        img,
		})
	}

	recordAuditLog(ctx, s.db, "pool.create", "runner_pool", &created.ID, map[string]any{
		"name":            created.Name,
		"provider":        created.Provider,
		"repository_url":  created.RepositoryUrl,
		"scope":           created.Scope,
		"min_idle":        created.MinIdleRunners,
		"max_concurrency": created.MaxConcurrency,
	})

	if s.statsProvider != nil {
		_ = s.statsProvider.Reload(ctx)
	}

	return connect.NewResponse(&supervisorv1.CreatePoolResponse{
		Pool: s.toProto(ctx, created),
	}), nil
}

// UpdatePool updates an existing runner pool, creates an audit log, and notifies the controller loop.
func (s *PoolService) UpdatePool(ctx context.Context, req *connect.Request[supervisorv1.UpdatePoolRequest]) (*connect.Response[supervisorv1.UpdatePoolResponse], error) {
	pool := req.Msg.Pool
	if err := validatePoolInput(pool); err != nil {
		return nil, err
	}
	if pool.Id <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pool id must be specified for update"))
	}

	existing, err := s.db.GetRunnerPoolById(ctx, pool.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("pool id %d not found: %w", pool.Id, err))
	}

	scope := strings.TrimSpace(pool.Scope)
	if scope == "" {
		scope = existing.Scope
	}

	labelsStr := strings.Join(pool.Labels, ",")

	updated, err := s.db.UpdateRunnerPool(ctx, db.UpdateRunnerPoolParams{
		ID:                       pool.Id,
		Name:                     strings.TrimSpace(pool.Name),
		Provider:                 strings.ToLower(strings.TrimSpace(pool.Provider)),
		RepositoryUrl:            strings.TrimSpace(pool.RepositoryUrl),
		Scope:                    scope,
		AuthProfileID:            pool.AuthProfileId,
		MinIdleRunners:           int64(pool.MinIdleRunners),
		MaxConcurrency:           int64(pool.MaxConcurrency),
		Labels:                   labelsStr,
		RunnerImage:              strings.TrimSpace(pool.RunnerImage),
		AllowDocker:              pool.AllowDocker,
		MaxRunnerLifetimeSeconds: int64(pool.MaxRunnerLifetimeSeconds),
		CpuLimit:                 sql.NullString{String: pool.CpuLimit, Valid: pool.CpuLimit != ""},
		MemoryLimit:              sql.NullString{String: pool.MemoryLimit, Valid: pool.MemoryLimit != ""},
	})
	if err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("auth_profile_id %d does not exist", pool.AuthProfileId))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("updating runner pool: %w", err))
	}

	if pool.Renovate != nil {
		if pool.Renovate.Enabled && strings.TrimSpace(pool.Renovate.CronSchedule) != "" {
			if _, err := cron.ParseSchedule(strings.TrimSpace(pool.Renovate.CronSchedule)); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid renovate cron schedule: %w", err))
			}
		}
		img := strings.TrimSpace(pool.Renovate.Image)
		if img == "" {
			img = "renovate/renovate:latest"
		}
		cronSched := strings.TrimSpace(pool.Renovate.CronSchedule)
		_, err := s.db.UpdateRenovateConfig(ctx, db.UpdateRenovateConfigParams{
			PoolID:       updated.ID,
			Enabled:      pool.Renovate.Enabled,
			CronSchedule: sql.NullString{String: cronSched, Valid: cronSched != ""},
			Image:        img,
		})
		if err != nil && errors.Is(err, sql.ErrNoRows) {
			_, _ = s.db.CreateRenovateConfig(ctx, db.CreateRenovateConfigParams{
				PoolID:       updated.ID,
				Enabled:      pool.Renovate.Enabled,
				CronSchedule: sql.NullString{String: cronSched, Valid: cronSched != ""},
				Image:        img,
			})
		}
	}

	recordAuditLog(ctx, s.db, "pool.update", "runner_pool", &updated.ID, map[string]any{
		"name":            updated.Name,
		"provider":        updated.Provider,
		"min_idle":        updated.MinIdleRunners,
		"max_concurrency": updated.MaxConcurrency,
	})

	if s.statsProvider != nil {
		_ = s.statsProvider.Reload(ctx)
	}

	return connect.NewResponse(&supervisorv1.UpdatePoolResponse{
		Pool: s.toProto(ctx, updated),
	}), nil
}

// DeletePool removes a runner pool from the database, emits an audit log, and notifies the controller.
func (s *PoolService) DeletePool(ctx context.Context, req *connect.Request[supervisorv1.DeletePoolRequest]) (*connect.Response[supervisorv1.DeletePoolResponse], error) {
	if req.Msg.Id <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pool id"))
	}

	existing, err := s.db.GetRunnerPoolById(ctx, req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("pool id %d not found: %w", req.Msg.Id, err))
	}

	if err := s.db.DeleteRunnerPool(ctx, req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("deleting runner pool: %w", err))
	}

	recordAuditLog(ctx, s.db, "pool.delete", "runner_pool", &existing.ID, map[string]any{
		"name":     existing.Name,
		"provider": existing.Provider,
	})

	if s.statsProvider != nil {
		_ = s.statsProvider.Reload(ctx)
	}

	return connect.NewResponse(&supervisorv1.DeletePoolResponse{
		Success: true,
	}), nil
}

// WatchPools provides near-realtime server-streaming push of pool states and runner counts.
func (s *PoolService) WatchPools(ctx context.Context, req *connect.Request[supervisorv1.WatchPoolsRequest], stream *connect.ServerStream[supervisorv1.WatchPoolsResponse]) error {
	intervalMs := req.Msg.IntervalMs
	if intervalMs < 250 {
		intervalMs = 1000
	}
	if intervalMs > 10000 {
		intervalMs = 10000
	}
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	sendSnapshot := func() error {
		dbPools, err := s.db.ListRunnerPools(ctx)
		if err != nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("listing runner pools: %w", err))
		}
		protoPools := make([]*supervisorv1.Pool, 0, len(dbPools))
		for _, p := range dbPools {
			protoPools = append(protoPools, s.toProto(ctx, p))
		}
		return stream.Send(&supervisorv1.WatchPoolsResponse{
			Pools: protoPools,
		})
	}

	// Send initial snapshot immediately
	if err := sendSnapshot(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := sendSnapshot(); err != nil {
				return err
			}
		}
	}
}

// ListRunners returns the active container instances for a specified pool.
func (s *PoolService) ListRunners(ctx context.Context, req *connect.Request[supervisorv1.ListRunnersRequest]) (*connect.Response[supervisorv1.ListRunnersResponse], error) {
	if req.Msg.PoolId <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pool_id must be greater than 0"))
	}
	p, err := s.db.GetRunnerPoolById(ctx, req.Msg.PoolId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("pool id %d not found: %w", req.Msg.PoolId, err))
	}

	runners := s.getRunnerInstances(p)
	return connect.NewResponse(&supervisorv1.ListRunnersResponse{
		Runners: runners,
	}), nil
}

// TerminateRunner manually terminates an active runner container instance.
func (s *PoolService) TerminateRunner(ctx context.Context, req *connect.Request[supervisorv1.TerminateRunnerRequest]) (*connect.Response[supervisorv1.TerminateRunnerResponse], error) {
	if req.Msg.PoolId <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pool_id must be greater than 0"))
	}
	containerID := strings.TrimSpace(req.Msg.ContainerId)
	if containerID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("container_id must not be empty"))
	}

	p, err := s.db.GetRunnerPoolById(ctx, req.Msg.PoolId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("pool id %d not found: %w", req.Msg.PoolId, err))
	}

	if s.runnerMgr != nil {
		if err := s.runnerMgr.TerminateRunner(ctx, p.Name, containerID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("terminating runner %q: %w", containerID, err))
		}
	}

	recordAuditLog(ctx, s.db, "runner.terminate", "runner_container", &p.ID, map[string]any{
		"container_id": containerID,
		"pool_name":    p.Name,
		"pool_id":      p.ID,
	})

	if s.statsProvider != nil {
		_ = s.statsProvider.Reload(ctx)
	}

	return connect.NewResponse(&supervisorv1.TerminateRunnerResponse{
		Success: true,
	}), nil
}

// WatchRunners provides near-realtime server-streaming push of active runner instances for a pool.
func (s *PoolService) WatchRunners(ctx context.Context, req *connect.Request[supervisorv1.WatchRunnersRequest], stream *connect.ServerStream[supervisorv1.WatchRunnersResponse]) error {
	if req.Msg.PoolId <= 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("pool_id must be greater than 0"))
	}
	p, err := s.db.GetRunnerPoolById(ctx, req.Msg.PoolId)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("pool id %d not found: %w", req.Msg.PoolId, err))
	}

	intervalMs := req.Msg.IntervalMs
	if intervalMs < 250 {
		intervalMs = 1000
	}
	if intervalMs > 10000 {
		intervalMs = 10000
	}
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	sendSnapshot := func() error {
		runners := s.getRunnerInstances(p)
		return stream.Send(&supervisorv1.WatchRunnersResponse{
			Runners: runners,
		})
	}

	if err := sendSnapshot(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := sendSnapshot(); err != nil {
				return err
			}
		}
	}
}

func (s *PoolService) getRunnerInstances(p db.RunnerPool) []*supervisorv1.RunnerInstance {
	if s.runnerMgr == nil {
		return []*supervisorv1.RunnerInstance{}
	}
	rawRunners := s.runnerMgr.PoolRunners(p.Name)
	res := make([]*supervisorv1.RunnerInstance, 0, len(rawRunners))
	for _, r := range rawRunners {
		status := "idle"
		if r.State != "running" {
			status = r.State
		} else if r.IsBusy {
			status = "busy"
		}
		uptime := int64(0)
		if !r.SpawnedAt.IsZero() {
			uptime = int64(time.Since(r.SpawnedAt).Seconds())
			if uptime < 0 {
				uptime = 0
			}
		}
		res = append(res, &supervisorv1.RunnerInstance{
			ContainerId:   r.ID,
			Name:          r.Name,
			PoolName:      r.PoolName,
			Status:        status,
			IpAddress:     r.IPAddress,
			UptimeSeconds: uptime,
			SpawnedAt:     r.SpawnedAt.Format(time.RFC3339),
			CpuLimit:      p.CpuLimit.String,
			MemoryLimit:   p.MemoryLimit.String,
		})
	}
	return res
}
