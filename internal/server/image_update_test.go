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

type mockLocalInspector struct {
	digests map[string]string
}

func (m *mockLocalInspector) GetLocalImageDigest(_ context.Context, image string) (string, error) {
	if d, ok := m.digests[image]; ok {
		return d, nil
	}
	return "", fmt.Errorf("local image not found")
}

type mockRegistryChecker struct {
	digests map[string]string
}

func (m *mockRegistryChecker) GetRemoteImageDigest(_ context.Context, imageRef string) (string, error) {
	if d, ok := m.digests[imageRef]; ok {
		return d, nil
	}
	return "", fmt.Errorf("remote image not found")
}

func (m *mockRegistryChecker) BumpTag(imageRef, newDigest string) {
	m.digests[imageRef] = newDigest
}

func TestCheckImageUpdate_DetectsBumpedTag(t *testing.T) {
	ctx := context.Background()
	testImage := "ghcr.io/noosxe/runner-aio:v1.0.0"
	initialDigest := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	bumpedDigest := "sha256:2222222222222222222222222222222222222222222222222222222222222222"

	mockDB := &mockImageUpdateDB{
		pools: map[int64]db.RunnerPool{
			10: {
				ID:          10,
				Name:        "pool-test",
				RunnerImage: testImage,
			},
		},
	}
	mockPuller := &mockImagePuller{}
	inspector := &mockLocalInspector{
		digests: map[string]string{
			testImage: initialDigest,
		},
	}
	registry := &mockRegistryChecker{
		digests: map[string]string{
			testImage: initialDigest,
		},
	}

	svc := NewImageUpdateService(mockDB, mockPuller,
		WithLocalInspector(inspector),
		WithRegistryChecker(registry),
	)

	mux := http.NewServeMux()
	path, handler := supervisorv1connect.NewImageUpdateServiceHandler(svc, BinaryConnectHandlerOptions()...)
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := supervisorv1connect.NewImageUpdateServiceClient(srv.Client(), srv.URL)

	// Phase 1: Local digest matches registry digest -> No update available
	res1, err := client.CheckImageUpdate(ctx, connect.NewRequest(&supervisorv1.CheckImageUpdateRequest{
		PoolId: 10,
	}))
	if err != nil {
		t.Fatalf("CheckImageUpdate failed: %v", err)
	}
	if res1.Msg.UpdateAvailable {
		t.Fatalf("expected update_available=false when digests match")
	}
	if res1.Msg.Update != nil {
		t.Errorf("expected update to be nil, got %+v", res1.Msg.Update)
	}

	// Phase 2: Intentionally bump test tag on registry
	registry.BumpTag(testImage, bumpedDigest)

	// Phase 3: CheckImageUpdate must detect the bumped tag (Acceptance requirement)
	res2, err := client.CheckImageUpdate(ctx, connect.NewRequest(&supervisorv1.CheckImageUpdateRequest{
		PoolId: 10,
	}))
	if err != nil {
		t.Fatalf("CheckImageUpdate failed: %v", err)
	}
	if !res2.Msg.UpdateAvailable {
		t.Fatalf("expected update_available=true after bumping test tag")
	}
	if res2.Msg.Update == nil {
		t.Fatalf("expected update details to be returned")
	}
	if res2.Msg.Update.PoolId != 10 {
		t.Errorf("expected pool_id=10, got %d", res2.Msg.Update.PoolId)
	}
	if res2.Msg.Update.LatestDigest != bumpedDigest {
		t.Errorf("expected latest_digest=%s, got %s", bumpedDigest, res2.Msg.Update.LatestDigest)
	}
	if res2.Msg.Update.CurrentImage != testImage {
		t.Errorf("expected current_image=%s, got %s", testImage, res2.Msg.Update.CurrentImage)
	}
	if res2.Msg.Update.Status != "available" {
		t.Errorf("expected status=available, got %s", res2.Msg.Update.Status)
	}

	// Verify update surfaces on dashboard via ListImageUpdates
	listRes, err := client.ListImageUpdates(ctx, connect.NewRequest(&supervisorv1.ListImageUpdatesRequest{
		PoolId: 10,
	}))
	if err != nil {
		t.Fatalf("ListImageUpdates failed: %v", err)
	}
	if len(listRes.Msg.Updates) != 1 {
		t.Fatalf("expected 1 update notification, got %d", len(listRes.Msg.Updates))
	}
	if listRes.Msg.Updates[0].LatestDigest != bumpedDigest {
		t.Errorf("expected latest digest %s in list, got %s", bumpedDigest, listRes.Msg.Updates[0].LatestDigest)
	}

	// Phase 4: Pull image and update local inspector -> subsequent check resolves update
	pullRes, err := client.PullImage(ctx, connect.NewRequest(&supervisorv1.PullImageRequest{
		PoolId: 10,
	}))
	if err != nil {
		t.Fatalf("PullImage failed: %v", err)
	}
	if !pullRes.Msg.Success {
		t.Fatalf("expected PullImage success")
	}

	inspector.digests[testImage] = bumpedDigest

	res3, err := client.CheckImageUpdate(ctx, connect.NewRequest(&supervisorv1.CheckImageUpdateRequest{
		PoolId: 10,
	}))
	if err != nil {
		t.Fatalf("CheckImageUpdate failed: %v", err)
	}
	if res3.Msg.UpdateAvailable {
		t.Fatalf("expected update_available=false after pulling updated image")
	}

	// Notification should be cleared
	listRes2, err := client.ListImageUpdates(ctx, connect.NewRequest(&supervisorv1.ListImageUpdatesRequest{
		PoolId: 10,
	}))
	if err != nil {
		t.Fatalf("ListImageUpdates failed: %v", err)
	}
	if len(listRes2.Msg.Updates) != 0 {
		t.Errorf("expected 0 pending updates after resolution, got %d", len(listRes2.Msg.Updates))
	}
}

func TestCheckImageUpdate_PerPoolScoping(t *testing.T) {
	ctx := context.Background()

	pool1Image := "ghcr.io/noosxe/runner-aio:v1.0.0"
	pool2Image := "ghcr.io/noosxe/runner-aio:v2.0.0"

	mockDB := &mockImageUpdateDB{
		pools: map[int64]db.RunnerPool{
			1: {ID: 1, Name: "prod-pool", RunnerImage: pool1Image},
			2: {ID: 2, Name: "staging-pool", RunnerImage: pool2Image},
		},
	}
	mockPuller := &mockImagePuller{}

	inspector := &mockLocalInspector{
		digests: map[string]string{
			pool1Image: "sha256:v1-local",
			pool2Image: "sha256:v2-local",
		},
	}
	registry := &mockRegistryChecker{
		digests: map[string]string{
			pool1Image: "sha256:v1-local",
			pool2Image: "sha256:v2-new-bumped", // Only pool 2 has an update
		},
	}

	svc := NewImageUpdateService(mockDB, mockPuller,
		WithLocalInspector(inspector),
		WithRegistryChecker(registry),
	)

	mux := http.NewServeMux()
	path, handler := supervisorv1connect.NewImageUpdateServiceHandler(svc, BinaryConnectHandlerOptions()...)
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := supervisorv1connect.NewImageUpdateServiceClient(srv.Client(), srv.URL)

	// Check pool 1 (prod) -> no update
	p1Res, err := client.CheckImageUpdate(ctx, connect.NewRequest(&supervisorv1.CheckImageUpdateRequest{PoolId: 1}))
	if err != nil {
		t.Fatalf("check pool 1 failed: %v", err)
	}
	if p1Res.Msg.UpdateAvailable {
		t.Errorf("pool 1 should not have an update available")
	}

	// Check pool 2 (staging) -> update available!
	p2Res, err := client.CheckImageUpdate(ctx, connect.NewRequest(&supervisorv1.CheckImageUpdateRequest{PoolId: 2}))
	if err != nil {
		t.Fatalf("check pool 2 failed: %v", err)
	}
	if !p2Res.Msg.UpdateAvailable {
		t.Errorf("pool 2 should have an update available")
	}
	if p2Res.Msg.Update.LatestDigest != "sha256:v2-new-bumped" {
		t.Errorf("expected latest digest sha256:v2-new-bumped, got %s", p2Res.Msg.Update.LatestDigest)
	}

	// Verify scoping in ListImageUpdates
	listP1, err := client.ListImageUpdates(ctx, connect.NewRequest(&supervisorv1.ListImageUpdatesRequest{PoolId: 1}))
	if err != nil {
		t.Fatalf("list pool 1 failed: %v", err)
	}
	if len(listP1.Msg.Updates) != 0 {
		t.Errorf("expected 0 updates for pool 1, got %d", len(listP1.Msg.Updates))
	}

	listP2, err := client.ListImageUpdates(ctx, connect.NewRequest(&supervisorv1.ListImageUpdatesRequest{PoolId: 2}))
	if err != nil {
		t.Fatalf("list pool 2 failed: %v", err)
	}
	if len(listP2.Msg.Updates) != 1 {
		t.Errorf("expected 1 update for pool 2, got %d", len(listP2.Msg.Updates))
	}
}

func TestCheckImageUpdate_RegistryError(t *testing.T) {
	ctx := context.Background()

	mockDB := &mockImageUpdateDB{
		pools: map[int64]db.RunnerPool{
			1: {ID: 1, Name: "pool-err", RunnerImage: "ghcr.io/noosxe/unknown:latest"},
		},
	}
	mockPuller := &mockImagePuller{}

	registry := &mockRegistryChecker{
		digests: map[string]string{}, // empty -> will return error
	}

	svc := NewImageUpdateService(mockDB, mockPuller, WithRegistryChecker(registry))

	mux := http.NewServeMux()
	path, handler := supervisorv1connect.NewImageUpdateServiceHandler(svc, BinaryConnectHandlerOptions()...)
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := supervisorv1connect.NewImageUpdateServiceClient(srv.Client(), srv.URL)

	_, err := client.CheckImageUpdate(ctx, connect.NewRequest(&supervisorv1.CheckImageUpdateRequest{PoolId: 1}))
	if err == nil {
		t.Fatalf("expected error when registry check fails, got nil")
	}
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Errorf("expected CodeUnavailable, got %v", connect.CodeOf(err))
	}
}

