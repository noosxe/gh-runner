package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/noosxe/gh-runner/internal/db"
	supervisorv1 "github.com/noosxe/gh-runner/internal/pb/supervisor/v1"
	"github.com/noosxe/gh-runner/internal/pb/supervisor/v1/supervisorv1connect"
)

type mockImageUpdateDB struct {
	pools map[int64]db.RunnerPool
	logs  []db.AuditLog
}

func (m *mockImageUpdateDB) ListRunnerPools(_ context.Context) ([]db.RunnerPool, error) {
	var res []db.RunnerPool
	for _, p := range m.pools {
		res = append(res, p)
	}
	return res, nil
}

func (m *mockImageUpdateDB) GetRunnerPoolById(_ context.Context, id int64) (db.RunnerPool, error) {
	p, ok := m.pools[id]
	if !ok {
		return db.RunnerPool{}, fmt.Errorf("not found")
	}
	return p, nil
}

func (m *mockImageUpdateDB) CreateAuditLog(_ context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
	l := db.AuditLog{
		Action: arg.Action,
	}
	m.logs = append(m.logs, l)
	return l, nil
}

type mockImagePuller struct {
	pulledImages []string
}

func (m *mockImagePuller) PullImage(_ context.Context, image string) error {
	m.pulledImages = append(m.pulledImages, image)
	return nil
}

func TestImageUpdateServiceLifecycle(t *testing.T) {
	ctx := context.Background()
	mockDB := &mockImageUpdateDB{
		pools: map[int64]db.RunnerPool{
			1: {
				ID:          1,
				Name:        "pool-linux-ci",
				RunnerImage: "ghcr.io/noosxe/runner-aio:v1.1.0",
			},
		},
	}
	mockPuller := &mockImagePuller{}

	svc := NewImageUpdateService(mockDB, mockPuller)

	mux := http.NewServeMux()
	path, handler := supervisorv1connect.NewImageUpdateServiceHandler(svc, BinaryConnectHandlerOptions()...)
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := supervisorv1connect.NewImageUpdateServiceClient(srv.Client(), srv.URL)

	// 1. Check Image Update
	checkRes, err := client.CheckImageUpdate(ctx, connect.NewRequest(&supervisorv1.CheckImageUpdateRequest{
		PoolId: 1,
	}))
	if err != nil {
		t.Fatalf("CheckImageUpdate failed: %v", err)
	}
	if !checkRes.Msg.UpdateAvailable {
		t.Fatalf("expected update to be available")
	}
	if checkRes.Msg.Update.PoolId != 1 {
		t.Errorf("expected pool_id=1, got %d", checkRes.Msg.Update.PoolId)
	}

	// 2. List Image Updates
	listRes, err := client.ListImageUpdates(ctx, connect.NewRequest(&supervisorv1.ListImageUpdatesRequest{
		PoolId: 1,
	}))
	if err != nil {
		t.Fatalf("ListImageUpdates failed: %v", err)
	}
	if len(listRes.Msg.Updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(listRes.Msg.Updates))
	}
	if listRes.Msg.Updates[0].Id <= 0 {
		t.Errorf("expected positive update ID")
	}

	// 3. Pull Image
	pullRes, err := client.PullImage(ctx, connect.NewRequest(&supervisorv1.PullImageRequest{
		PoolId: 1,
	}))
	if err != nil {
		t.Fatalf("PullImage failed: %v", err)
	}
	if !pullRes.Msg.Success {
		t.Errorf("PullImage response success = false")
	}
	if len(mockPuller.pulledImages) != 1 || mockPuller.pulledImages[0] != "ghcr.io/noosxe/runner-aio:v1.1.0" {
		t.Errorf("expected pulled image %s, got %v", "ghcr.io/noosxe/runner-aio:v1.1.0", mockPuller.pulledImages)
	}

	// 4. Dismiss Image Update
	newUp := svc.FlagUpdate(1, "ghcr.io/noosxe/runner-aio:v1.1.0", "ghcr.io/noosxe/runner-aio:v1.2.0")
	dismissRes, err := client.DismissImageUpdate(ctx, connect.NewRequest(&supervisorv1.DismissImageUpdateRequest{
		Id: newUp.Id,
	}))
	if err != nil {
		t.Fatalf("DismissImageUpdate failed: %v", err)
	}
	if !dismissRes.Msg.Success {
		t.Errorf("DismissImageUpdate success = false")
	}
}
