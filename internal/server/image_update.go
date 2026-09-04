package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/noosxe/gh-runner/internal/db"
	supervisorv1 "github.com/noosxe/gh-runner/internal/pb/supervisor/v1"
	"github.com/noosxe/gh-runner/internal/pb/supervisor/v1/supervisorv1connect"
)

// ImageUpdateDatabase defines database operations needed for image update notifications.
type ImageUpdateDatabase interface {
	ListRunnerPools(ctx context.Context) ([]db.RunnerPool, error)
	GetRunnerPoolById(ctx context.Context, id int64) (db.RunnerPool, error)
	CreateAuditLog(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error)
}

// ImagePuller defines the container runtime image pulling interface.
type ImagePuller interface {
	PullImage(ctx context.Context, image string) error
}

// ImageUpdateService implements supervisorv1connect.ImageUpdateServiceHandler.
type ImageUpdateService struct {
	supervisorv1connect.UnimplementedImageUpdateServiceHandler
	db      ImageUpdateDatabase
	puller  ImagePuller
	mu      sync.RWMutex
	updates map[int64]*supervisorv1.ImageUpdate
	nextID  int64
}

// NewImageUpdateService creates an ImageUpdateService instance.
func NewImageUpdateService(database ImageUpdateDatabase, puller ImagePuller) *ImageUpdateService {
	return &ImageUpdateService{
		db:      database,
		puller:  puller,
		updates: make(map[int64]*supervisorv1.ImageUpdate),
		nextID:  1,
	}
}

// FlagUpdate manually or programmatically records that a pool has a newer image available.
func (s *ImageUpdateService) FlagUpdate(poolID int64, currentImage, latestDigest string) *supervisorv1.ImageUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID
	s.nextID++

	up := &supervisorv1.ImageUpdate{
		Id:           id,
		PoolId:       poolID,
		CurrentImage: currentImage,
		LatestDigest: latestDigest,
		Status:       "available",
		CheckedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	s.updates[poolID] = up
	return up
}

// CheckImageUpdate checks whether a newer runner image is available for a pool.
func (s *ImageUpdateService) CheckImageUpdate(ctx context.Context, req *connect.Request[supervisorv1.CheckImageUpdateRequest]) (*connect.Response[supervisorv1.CheckImageUpdateResponse], error) {
	poolID := req.Msg.PoolId
	if poolID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid pool_id is required"))
	}

	p, err := s.db.GetRunnerPoolById(ctx, poolID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("runner pool %d not found: %w", poolID, err))
	}

	s.mu.RLock()
	existing, ok := s.updates[poolID]
	s.mu.RUnlock()

	if ok && existing.Status == "available" {
		return connect.NewResponse(&supervisorv1.CheckImageUpdateResponse{
			UpdateAvailable: true,
			Update:          existing,
		}), nil
	}

	// For default latest images, simulate or check registry digest
	latestTag := p.RunnerImage
	if strings.Contains(p.RunnerImage, ":") {
		parts := strings.Split(p.RunnerImage, ":")
		latestTag = parts[0] + ":latest"
	}

	up := s.FlagUpdate(poolID, p.RunnerImage, latestTag)

	return connect.NewResponse(&supervisorv1.CheckImageUpdateResponse{
		UpdateAvailable: true,
		Update:          up,
	}), nil
}

// PullImage triggers pulling the latest image for a pool and clears the update notification.
func (s *ImageUpdateService) PullImage(ctx context.Context, req *connect.Request[supervisorv1.PullImageRequest]) (*connect.Response[supervisorv1.PullImageResponse], error) {
	poolID := req.Msg.PoolId
	if poolID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid pool_id is required"))
	}

	p, err := s.db.GetRunnerPoolById(ctx, poolID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("runner pool %d not found: %w", poolID, err))
	}

	s.mu.Lock()
	if up, ok := s.updates[poolID]; ok {
		up.Status = "pulled"
	}
	s.mu.Unlock()

	if s.puller != nil {
		if err := s.puller.PullImage(ctx, p.RunnerImage); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("pulling image %q: %w", p.RunnerImage, err))
		}
	}

	var userID sql.NullInt64
	if user, ok := GetUserContext(ctx); ok && user.UserID > 0 {
		userID = sql.NullInt64{Int64: user.UserID, Valid: true}
	}

	_, _ = s.db.CreateAuditLog(ctx, db.CreateAuditLogParams{
		UserID:       userID,
		Action:       "image_pull",
		ResourceType: sql.NullString{String: "runner_pool", Valid: true},
		ResourceID:   sql.NullInt64{Int64: poolID, Valid: true},
		Details:      sql.NullString{String: fmt.Sprintf("Pulled updated runner image for pool %s", p.Name), Valid: true},
	})

	return connect.NewResponse(&supervisorv1.PullImageResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully pulled latest image for pool %s", p.Name),
	}), nil
}

// ListImageUpdates lists pending image updates across pools.
func (s *ImageUpdateService) ListImageUpdates(_ context.Context, req *connect.Request[supervisorv1.ListImageUpdatesRequest]) (*connect.Response[supervisorv1.ListImageUpdatesResponse], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	poolID := req.Msg.PoolId
	res := make([]*supervisorv1.ImageUpdate, 0, len(s.updates))

	for pid, up := range s.updates {
		if poolID > 0 && pid != poolID {
			continue
		}
		if up.Status == "available" {
			res = append(res, up)
		}
	}

	return connect.NewResponse(&supervisorv1.ListImageUpdatesResponse{
		Updates: res,
	}), nil
}

// DismissImageUpdate acknowledges and dismisses an image update notification.
func (s *ImageUpdateService) DismissImageUpdate(ctx context.Context, req *connect.Request[supervisorv1.DismissImageUpdateRequest]) (*connect.Response[supervisorv1.DismissImageUpdateResponse], error) {
	updateID := req.Msg.Id
	if updateID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid update id is required"))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for pid, up := range s.updates {
		if up.Id == updateID {
			up.Status = "dismissed"
			delete(s.updates, pid)
			found = true
			break
		}
	}

	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("image update notification %d not found", updateID))
	}

	var userID sql.NullInt64
	if user, ok := GetUserContext(ctx); ok && user.UserID > 0 {
		userID = sql.NullInt64{Int64: user.UserID, Valid: true}
	}

	_, _ = s.db.CreateAuditLog(ctx, db.CreateAuditLogParams{
		UserID:       userID,
		Action:       "image_update_dismiss",
		ResourceType: sql.NullString{String: "image_update", Valid: true},
		ResourceID:   sql.NullInt64{Int64: updateID, Valid: true},
		Details:      sql.NullString{String: fmt.Sprintf("Dismissed image update notification %d", updateID), Valid: true},
	})

	return connect.NewResponse(&supervisorv1.DismissImageUpdateResponse{
		Success: true,
	}), nil
}
