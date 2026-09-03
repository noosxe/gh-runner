package server_test

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/noosxe/gh-runner/internal/db"
	supervisorv1 "github.com/noosxe/gh-runner/internal/pb/supervisor/v1"
	"github.com/noosxe/gh-runner/internal/pb/supervisor/v1/supervisorv1connect"
	"github.com/noosxe/gh-runner/internal/server"
)

type mockStatsProvider struct {
	mu           sync.Mutex
	reloadsCount int
	activeCounts map[string]int32
	idleCounts   map[string]int32
}

func newMockStatsProvider() *mockStatsProvider {
	return &mockStatsProvider{
		activeCounts: make(map[string]int32),
		idleCounts:   make(map[string]int32),
	}
}

func (m *mockStatsProvider) PoolStats(poolName string) (active int32, idle int32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeCounts[poolName], m.idleCounts[poolName]
}

func (m *mockStatsProvider) Reload(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reloadsCount++
	return nil
}

func TestPoolServiceCRUDAndValidation(t *testing.T) {
	ctx := context.Background()
	database, jwtSecret := setupTestDB(t)
	stats := newMockStatsProvider()
	stats.activeCounts["github-arm64"] = 3
	stats.idleCounts["github-arm64"] = 2

	srv := server.New(server.Options{
		Port:             8080,
		AuthDB:           database,
		PoolDB:           database,
		PoolStats:        stats,
		JWTSigningSecret: jwtSecret,
	})

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Authenticate first
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

	// Create test auth profile for foreign key requirement
	authProfile, err := database.CreateAuthProfile(ctx, db.CreateAuthProfileParams{
		Name:                "test-auth-profile",
		AuthMethod:          "pat",
		TokenEncrypted:      sql.NullString{String: "encrypted-token", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateAuthProfile failed: %v", err)
	}

	client := supervisorv1connect.NewPoolServiceClient(ts.Client(), ts.URL)

	// 1. Validation: Gitea / Forgejo require allow_docker=true
	giteaReq := connect.NewRequest(&supervisorv1.CreatePoolRequest{
		Pool: &supervisorv1.Pool{
			Name:          "gitea-pool",
			Provider:      "gitea",
			RepositoryUrl: "https://gitea.local/owner/repo",
			AuthProfileId: authProfile.ID,
			AllowDocker:   false, // Invalid!
		},
	})
	giteaReq.Header().Set("Cookie", "session_token="+rawCookie)
	_, err = client.CreatePool(ctx, giteaReq)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("Gitea allow_docker=false want CodeInvalidArgument, got: %v", err)
	}

	// 2. Validation: invalid provider
	badProviderReq := connect.NewRequest(&supervisorv1.CreatePoolRequest{
		Pool: &supervisorv1.Pool{
			Name:          "bad-pool",
			Provider:      "bitbucket",
			RepositoryUrl: "https://bitbucket.org/owner/repo",
			AuthProfileId: authProfile.ID,
		},
	})
	badProviderReq.Header().Set("Cookie", "session_token="+rawCookie)
	_, err = client.CreatePool(ctx, badProviderReq)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("Unsupported provider want CodeInvalidArgument, got: %v", err)
	}

	// 2b. Validation: non-existent auth_profile_id
	badAuthReq := connect.NewRequest(&supervisorv1.CreatePoolRequest{
		Pool: &supervisorv1.Pool{
			Name:          "bad-auth-pool",
			Provider:      "github",
			RepositoryUrl: "https://github.com/org/repo",
			AuthProfileId: 99999,
		},
	})
	badAuthReq.Header().Set("Cookie", "session_token="+rawCookie)
	_, err = client.CreatePool(ctx, badAuthReq)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("Non-existent auth_profile_id want CodeInvalidArgument, got: %v", err)
	}

	// 3. CreatePool valid GitHub pool
	createReq := connect.NewRequest(&supervisorv1.CreatePoolRequest{
		Pool: &supervisorv1.Pool{
			Name:                     "github-arm64",
			Provider:                 "github",
			RepositoryUrl:            "https://github.com/org/repo",
			Scope:                    "repo",
			AuthProfileId:            authProfile.ID,
			MinIdleRunners:           2,
			MaxConcurrency:           10,
			Labels:                   []string{"self-hosted", "linux", "arm64"},
			RunnerImage:              "ghcr.io/noosxe/gh-runner:latest",
			AllowDocker:              true,
			CpuLimit:                 "2.0",
			MemoryLimit:              "4Gi",
			MaxRunnerLifetimeSeconds: 3600,
		},
	})
	createReq.Header().Set("Cookie", "session_token="+rawCookie)
	createRes, err := client.CreatePool(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePool failed: %v", err)
	}

	createdPool := createRes.Msg.Pool
	if createdPool.Id <= 0 || createdPool.Name != "github-arm64" {
		t.Fatalf("unexpected created pool: %+v", createdPool)
	}
	if len(createdPool.Labels) != 3 || createdPool.Labels[2] != "arm64" {
		t.Errorf("expected 3 labels, got: %+v", createdPool.Labels)
	}
	if createdPool.ActiveRunners != 3 || createdPool.IdleRunners != 2 {
		t.Errorf("stats not populated properly: active=%d, idle=%d", createdPool.ActiveRunners, createdPool.IdleRunners)
	}

	// Verify reload was triggered and audit log was recorded
	stats.mu.Lock()
	if stats.reloadsCount != 1 {
		t.Errorf("expected 1 reload, got: %d", stats.reloadsCount)
	}
	stats.mu.Unlock()

	auditLogs, err := database.ListAuditLogs(ctx, db.ListAuditLogsParams{Limit: 10, Offset: 0})
	if err != nil || len(auditLogs) == 0 || auditLogs[0].Action != "pool_create" {
		t.Fatalf("expected pool_create audit log, got: %+v", auditLogs)
	}

	// 4. ListPools returns the created pool with stats
	listReq := connect.NewRequest(&supervisorv1.ListPoolsRequest{})
	listReq.Header().Set("Cookie", "session_token="+rawCookie)
	listRes, err := client.ListPools(ctx, listReq)
	if err != nil {
		t.Fatalf("ListPools failed: %v", err)
	}
	if len(listRes.Msg.Pools) != 1 || listRes.Msg.Pools[0].Name != "github-arm64" {
		t.Fatalf("expected 1 pool, got: %+v", listRes.Msg.Pools)
	}
	if listRes.Msg.Pools[0].ActiveRunners != 3 || listRes.Msg.Pools[0].IdleRunners != 2 {
		t.Errorf("ListPools stats mismatch: %+v", listRes.Msg.Pools[0])
	}

	// 5. UpdatePool updates properties and triggers reload
	updateReq := connect.NewRequest(&supervisorv1.UpdatePoolRequest{
		Pool: &supervisorv1.Pool{
			Id:                       createdPool.Id,
			Name:                     "github-arm64",
			Provider:                 "github",
			RepositoryUrl:            "https://github.com/org/repo",
			Scope:                    "org",
			AuthProfileId:            authProfile.ID,
			MinIdleRunners:           4,
			MaxConcurrency:           15,
			Labels:                   []string{"self-hosted", "linux", "arm64", "gpu"},
			RunnerImage:              "ghcr.io/noosxe/gh-runner:v2",
			AllowDocker:              true,
			CpuLimit:                 "4.0",
			MemoryLimit:              "8Gi",
			MaxRunnerLifetimeSeconds: 7200,
		},
	})
	updateReq.Header().Set("Cookie", "session_token="+rawCookie)
	updateRes, err := client.UpdatePool(ctx, updateReq)
	if err != nil {
		t.Fatalf("UpdatePool failed: %v", err)
	}
	if updateRes.Msg.Pool.Scope != "org" || updateRes.Msg.Pool.MinIdleRunners != 4 {
		t.Errorf("UpdatePool mismatch: %+v", updateRes.Msg.Pool)
	}

	stats.mu.Lock()
	if stats.reloadsCount != 2 {
		t.Errorf("expected 2 reloads, got: %d", stats.reloadsCount)
	}
	stats.mu.Unlock()

	// 6. DeletePool removes the pool and triggers reload
	deleteReq := connect.NewRequest(&supervisorv1.DeletePoolRequest{
		Id: createdPool.Id,
	})
	deleteReq.Header().Set("Cookie", "session_token="+rawCookie)
	delRes, err := client.DeletePool(ctx, deleteReq)
	if err != nil {
		t.Fatalf("DeletePool failed: %v", err)
	}
	if !delRes.Msg.Success {
		t.Errorf("DeletePool want success=true")
	}

	stats.mu.Lock()
	if stats.reloadsCount != 3 {
		t.Errorf("expected 3 reloads, got: %d", stats.reloadsCount)
	}
	stats.mu.Unlock()

	// Verify pool list is empty now
	listRes2, err := client.ListPools(ctx, listReq)
	if err != nil || len(listRes2.Msg.Pools) != 0 {
		t.Fatalf("expected empty pool list after delete, got: %+v (err %v)", listRes2.Msg.Pools, err)
	}

	// Deleting again returns NotFound
	_, err = client.DeletePool(ctx, deleteReq)
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("second DeletePool want CodeNotFound, got: %v", err)
	}
}
