package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/noosxe/gh-runner/internal/db"
	supervisorv1 "github.com/noosxe/gh-runner/internal/pb/supervisor/v1"
	"github.com/noosxe/gh-runner/internal/pb/supervisor/v1/supervisorv1connect"
)

// AnalyticsDatabase defines the database queries required by AnalyticsService.
// *db.DB satisfies this interface.
type AnalyticsDatabase interface {
	CountJobHistory(ctx context.Context) (int64, error)
	CountJobHistoryByPoolId(ctx context.Context, poolID int64) (int64, error)
	ListJobHistory(ctx context.Context, arg db.ListJobHistoryParams) ([]db.JobHistory, error)
	ListJobHistoryByPoolId(ctx context.Context, arg db.ListJobHistoryByPoolIdParams) ([]db.JobHistory, error)
	GetJobStatsSince(ctx context.Context, createdAt time.Time) (db.GetJobStatsSinceRow, error)
}

// SystemStatsProvider provides live active/idle runner counts across all pools.
// *orchestrator.PoolController satisfies this interface.
type SystemStatsProvider interface {
	SystemRunnerStats() (active int32, idle int32)
}

// AnalyticsService implements supervisorv1connect.AnalyticsServiceHandler.
type AnalyticsService struct {
	supervisorv1connect.UnimplementedAnalyticsServiceHandler
	db            AnalyticsDatabase
	statsProvider SystemStatsProvider
}

// NewAnalyticsService constructs an AnalyticsService instance.
func NewAnalyticsService(database AnalyticsDatabase, statsProvider SystemStatsProvider) *AnalyticsService {
	return &AnalyticsService{
		db:            database,
		statsProvider: statsProvider,
	}
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case []byte:
		var f float64
		_, _ = fmt.Sscanf(string(val), "%f", &f)
		return f
	default:
		return 0
	}
}

func toInt32(v any) int32 {
	switch val := v.(type) {
	case int64:
		return int32(val)
	case int:
		return int32(val)
	case float64:
		return int32(val)
	case []byte:
		var n int32
		_, _ = fmt.Sscanf(string(val), "%d", &n)
		return n
	default:
		return 0
	}
}

// GetJobHistory retrieves paginated job history optionally filtered by pool_id.
func (s *AnalyticsService) GetJobHistory(ctx context.Context, req *connect.Request[supervisorv1.GetJobHistoryRequest]) (*connect.Response[supervisorv1.GetJobHistoryResponse], error) {
	limit := req.Msg.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	offset := req.Msg.Offset
	if offset < 0 {
		offset = 0
	}

	var jobs []db.JobHistory
	var totalCount int64
	var err error

	if req.Msg.PoolId > 0 {
		totalCount, err = s.db.CountJobHistoryByPoolId(ctx, req.Msg.PoolId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("counting job history: %w", err))
		}
		jobs, err = s.db.ListJobHistoryByPoolId(ctx, db.ListJobHistoryByPoolIdParams{
			PoolID: req.Msg.PoolId,
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("listing job history: %w", err))
		}
	} else {
		totalCount, err = s.db.CountJobHistory(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("counting job history: %w", err))
		}
		jobs, err = s.db.ListJobHistory(ctx, db.ListJobHistoryParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("listing job history: %w", err))
		}
	}

	resp := &supervisorv1.GetJobHistoryResponse{
		Jobs:       make([]*supervisorv1.JobRecord, 0, len(jobs)),
		TotalCount: int32(totalCount),
	}

	for _, j := range jobs {
		rec := &supervisorv1.JobRecord{
			Id:         j.ID,
			PoolId:     j.PoolID,
			RunnerName: j.RunnerName,
			Status:     j.Status,
		}
		if j.QueuedAt.Valid {
			rec.QueuedAt = j.QueuedAt.Time.Format(time.RFC3339)
		}
		if j.StartedAt.Valid {
			rec.StartedAt = j.StartedAt.Time.Format(time.RFC3339)
		}
		if j.CompletedAt.Valid {
			rec.CompletedAt = j.CompletedAt.Time.Format(time.RFC3339)
		}
		resp.Jobs = append(resp.Jobs, rec)
	}

	return connect.NewResponse(resp), nil
}

// GetSystemStats aggregates system metrics across live runner state and historical DB executions.
func (s *AnalyticsService) GetSystemStats(ctx context.Context, _ *connect.Request[supervisorv1.GetSystemStatsRequest]) (*connect.Response[supervisorv1.GetSystemStatsResponse], error) {
	var active, idle int32
	if s.statsProvider != nil {
		active, idle = s.statsProvider.SystemRunnerStats()
	}

	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	row, err := s.db.GetJobStatsSince(ctx, cutoff)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("calculating job metrics: %w", err))
	}

	total24h := int32(row.TotalJobs)
	successful24h := toInt32(row.SuccessfulJobs)
	failed24h := toInt32(row.FailedJobs)
	avgQueue := toFloat64(row.AvgQueueSeconds)
	avgRuntime := toFloat64(row.AvgRuntimeSeconds)

	successRate := 0.0
	if total24h > 0 {
		successRate = (float64(successful24h) / float64(total24h)) * 100.0
	}

	return connect.NewResponse(&supervisorv1.GetSystemStatsResponse{
		TotalActiveRunners:      active,
		TotalIdleRunners:        idle,
		AverageQueueTimeSeconds: avgQueue,
		TotalJobs_24H:           total24h,
		SuccessfulJobs_24H:      successful24h,
		FailedJobs_24H:          failed24h,
		AverageRuntimeSeconds:   avgRuntime,
		SuccessRatePercent:      successRate,
	}), nil
}
