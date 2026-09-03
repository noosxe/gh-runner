package server_test

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/noosxe/gh-runner/internal/db"
	supervisorv1 "github.com/noosxe/gh-runner/internal/pb/supervisor/v1"
	"github.com/noosxe/gh-runner/internal/pb/supervisor/v1/supervisorv1connect"
	"github.com/noosxe/gh-runner/internal/server"
)

type mockSystemStats struct {
	active int32
	idle   int32
}

func (m *mockSystemStats) SystemRunnerStats() (active int32, idle int32) {
	return m.active, m.idle
}

func TestAnalyticsJobHistoryAndStats(t *testing.T) {
	ctx := context.Background()
	database, jwtSecret := setupTestDB(t)

	stats := &mockSystemStats{
		active: 5,
		idle:   3,
	}

	srv := server.New(server.Options{
		Port:             8080,
		AuthDB:           database,
		AnalyticsDB:      database,
		SystemStats:      stats,
		JWTSigningSecret: jwtSecret,
	})

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Authenticate
	authClient := supervisorv1connect.NewAuthServiceClient(ts.Client(), ts.URL)
	_, err := authClient.SetupAdmin(ctx, connect.NewRequest(&supervisorv1.SetupAdminRequest{
		Username: "admin",
		Password: "password123",
	}))
	if err != nil {
		t.Fatalf("SetupAdmin failed: %v", err)
	}

	loginRes, err := authClient.Login(ctx, connect.NewRequest(&supervisorv1.LoginRequest{
		Username: "admin",
		Password: "password123",
	}))
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	cookie := loginRes.Header().Get("Set-Cookie")
	rawCookie := strings.Split(strings.Split(cookie, ";")[0], "=")[1]

	// Seed auth profile and runner pools
	authProf, err := database.CreateAuthProfile(ctx, db.CreateAuthProfileParams{
		Name:           "analytics-auth",
		AuthMethod:     "pat",
		TokenEncrypted: sql.NullString{String: "enc", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateAuthProfile: %v", err)
	}

	p1, err := database.CreateRunnerPool(ctx, db.CreateRunnerPoolParams{
		Name:          "analytics-pool-1",
		Provider:      "github",
		RepositoryUrl: "https://github.com/org/repo1",
		Scope:         "repo",
		AuthProfileID: authProf.ID,
		Labels:        "linux",
		RunnerImage:   "img",
	})
	if err != nil {
		t.Fatalf("CreateRunnerPool 1: %v", err)
	}

	p2, err := database.CreateRunnerPool(ctx, db.CreateRunnerPoolParams{
		Name:          "analytics-pool-2",
		Provider:      "github",
		RepositoryUrl: "https://github.com/org/repo2",
		Scope:         "repo",
		AuthProfileID: authProf.ID,
		Labels:        "linux",
		RunnerImage:   "img",
	})
	if err != nil {
		t.Fatalf("CreateRunnerPool 2: %v", err)
	}

	// Seed job history in database
	now := time.Now().UTC()
	q1 := now.Add(-10 * time.Minute)
	s1 := now.Add(-9 * time.Minute)
	c1 := now.Add(-5 * time.Minute)

	// Pool 1: 2 successful jobs
	_, err = database.CreateJobHistory(ctx, db.CreateJobHistoryParams{
		PoolID:      p1.ID,
		RunnerName:  "runner-1",
		Status:      "success",
		QueuedAt:    sql.NullTime{Time: q1, Valid: true},
		StartedAt:   sql.NullTime{Time: s1, Valid: true},
		CompletedAt: sql.NullTime{Time: c1, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateJobHistory: %v", err)
	}

	_, err = database.CreateJobHistory(ctx, db.CreateJobHistoryParams{
		PoolID:      p1.ID,
		RunnerName:  "runner-2",
		Status:      "success",
		QueuedAt:    sql.NullTime{Time: q1, Valid: true},
		StartedAt:   sql.NullTime{Time: s1, Valid: true},
		CompletedAt: sql.NullTime{Time: c1, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateJobHistory: %v", err)
	}

	// Pool 2: 1 failed job
	_, err = database.CreateJobHistory(ctx, db.CreateJobHistoryParams{
		PoolID:      p2.ID,
		RunnerName:  "runner-3",
		Status:      "failure",
		QueuedAt:    sql.NullTime{Time: q1, Valid: true},
		StartedAt:   sql.NullTime{Time: s1, Valid: true},
		CompletedAt: sql.NullTime{Time: c1, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateJobHistory: %v", err)
	}

	client := supervisorv1connect.NewAnalyticsServiceClient(ts.Client(), ts.URL)

	// 1. GetJobHistory for all pools (pool_id = 0)
	allReq := connect.NewRequest(&supervisorv1.GetJobHistoryRequest{
		PoolId: 0,
		Limit:  10,
		Offset: 0,
	})
	allReq.Header().Set("Cookie", "session_token="+rawCookie)
	allRes, err := client.GetJobHistory(ctx, allReq)
	if err != nil {
		t.Fatalf("GetJobHistory failed: %v", err)
	}
	if allRes.Msg.TotalCount != 3 || len(allRes.Msg.Jobs) != 3 {
		t.Fatalf("expected 3 jobs, got: total=%d, len=%d", allRes.Msg.TotalCount, len(allRes.Msg.Jobs))
	}

	// 2. GetJobHistory filtered by pool_id = p1.ID
	poolReq := connect.NewRequest(&supervisorv1.GetJobHistoryRequest{
		PoolId: p1.ID,
		Limit:  10,
	})
	poolReq.Header().Set("Cookie", "session_token="+rawCookie)
	poolRes, err := client.GetJobHistory(ctx, poolReq)
	if err != nil {
		t.Fatalf("GetJobHistory for pool 1 failed: %v", err)
	}
	if poolRes.Msg.TotalCount != 2 || len(poolRes.Msg.Jobs) != 2 {
		t.Fatalf("expected 2 jobs for pool 1, got total=%d", poolRes.Msg.TotalCount)
	}

	// 3. GetSystemStats
	statsReq := connect.NewRequest(&supervisorv1.GetSystemStatsRequest{})
	statsReq.Header().Set("Cookie", "session_token="+rawCookie)
	statsRes, err := client.GetSystemStats(ctx, statsReq)
	if err != nil {
		t.Fatalf("GetSystemStats failed: %v", err)
	}

	resMsg := statsRes.Msg
	if resMsg.TotalActiveRunners != 5 || resMsg.TotalIdleRunners != 3 {
		t.Errorf("runner counts mismatch: active=%d, idle=%d", resMsg.TotalActiveRunners, resMsg.TotalIdleRunners)
	}
	if resMsg.TotalJobs_24H != 3 {
		t.Errorf("total_jobs_24h = %d, want 3", resMsg.TotalJobs_24H)
	}
	if resMsg.SuccessfulJobs_24H != 2 {
		t.Errorf("successful_jobs_24h = %d, want 2", resMsg.SuccessfulJobs_24H)
	}
	if resMsg.FailedJobs_24H != 1 {
		t.Errorf("failed_jobs_24h = %d, want 1", resMsg.FailedJobs_24H)
	}
	expectedRate := (2.0 / 3.0) * 100.0
	if resMsg.SuccessRatePercent < expectedRate-1.0 || resMsg.SuccessRatePercent > expectedRate+1.0 {
		t.Errorf("success_rate_percent = %f, want ~%f", resMsg.SuccessRatePercent, expectedRate)
	}
	if resMsg.AverageQueueTimeSeconds <= 0 {
		t.Errorf("average_queue_time_seconds = %f, want > 0", resMsg.AverageQueueTimeSeconds)
	}
	if resMsg.AverageRuntimeSeconds <= 0 {
		t.Errorf("average_runtime_seconds = %f, want > 0", resMsg.AverageRuntimeSeconds)
	}
}
