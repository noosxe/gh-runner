package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/noosxe/gh-runner/internal/db"
	supervisorv1 "github.com/noosxe/gh-runner/internal/pb/supervisor/v1"
	"github.com/noosxe/gh-runner/internal/pb/supervisor/v1/supervisorv1connect"
)

// OnboardingDatabase defines the database queries required by OnboardingService.
// *db.DB satisfies this interface.
type OnboardingDatabase interface {
	CountAdminUsers(ctx context.Context) (int64, error)
	CountAuthProfiles(ctx context.Context) (int64, error)
	CountRunnerPools(ctx context.Context) (int64, error)
	ListAppSettings(ctx context.Context) ([]db.AppSetting, error)
	GetAppSetting(ctx context.Context, key string) (db.AppSetting, error)
	SetAppSetting(ctx context.Context, arg db.SetAppSettingParams) (db.AppSetting, error)
	CreateAuditLog(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error)
}

// OnboardingService implements supervisorv1connect.OnboardingServiceHandler.
type OnboardingService struct {
	supervisorv1connect.UnimplementedOnboardingServiceHandler
	db OnboardingDatabase
}

// NewOnboardingService constructs an OnboardingService instance.
func NewOnboardingService(database OnboardingDatabase) *OnboardingService {
	return &OnboardingService{db: database}
}

// GetOnboardingStatus returns the overall setup progress (publicly accessible).
func (s *OnboardingService) GetOnboardingStatus(ctx context.Context, _ *connect.Request[supervisorv1.GetOnboardingStatusRequest]) (*connect.Response[supervisorv1.GetOnboardingStatusResponse], error) {
	adminCount, err := s.db.CountAdminUsers(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("counting admin users: %w", err))
	}

	profileCount, err := s.db.CountAuthProfiles(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("counting auth profiles: %w", err))
	}

	poolCount, err := s.db.CountRunnerPools(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("counting runner pools: %w", err))
	}

	adminCreated := adminCount > 0
	profileExists := profileCount > 0
	poolExists := poolCount > 0
	setupComplete := adminCreated && profileExists && poolExists

	return connect.NewResponse(&supervisorv1.GetOnboardingStatusResponse{
		AdminCreated:      adminCreated,
		AuthProfileExists: profileExists,
		PoolExists:        poolExists,
		SetupComplete:     setupComplete,
	}), nil
}

// GetAppSettings returns all application global configuration settings.
func (s *OnboardingService) GetAppSettings(ctx context.Context, _ *connect.Request[supervisorv1.GetAppSettingsRequest]) (*connect.Response[supervisorv1.GetAppSettingsResponse], error) {
	settings, err := s.db.ListAppSettings(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("listing app settings: %w", err))
	}

	resp := &supervisorv1.GetAppSettingsResponse{
		Settings: make([]*supervisorv1.AppSettingEntry, 0, len(settings)),
	}
	for _, item := range settings {
		resp.Settings = append(resp.Settings, &supervisorv1.AppSettingEntry{
			Key:       item.Key,
			Value:     item.Value,
			UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
		})
	}

	return connect.NewResponse(resp), nil
}

// SetAppSetting updates or sets a specific application setting key/value pair.
func (s *OnboardingService) SetAppSetting(ctx context.Context, req *connect.Request[supervisorv1.SetAppSettingRequest]) (*connect.Response[supervisorv1.SetAppSettingResponse], error) {
	key := strings.TrimSpace(req.Msg.Key)
	if key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("app setting key must not be empty"))
	}

	setting, err := s.db.SetAppSetting(ctx, db.SetAppSettingParams{
		Key:   key,
		Value: req.Msg.Value,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("setting app setting %q: %w", key, err))
	}

	var userID sql.NullInt64
	if user, ok := GetUserContext(ctx); ok && user.UserID > 0 {
		userID = sql.NullInt64{Int64: user.UserID, Valid: true}
	}

	_, _ = s.db.CreateAuditLog(ctx, db.CreateAuditLogParams{
		UserID:       userID,
		Action:       "app_setting_update",
		ResourceType: sql.NullString{String: "app_setting", Valid: true},
		Details:      sql.NullString{String: fmt.Sprintf("Updated app setting %s", key), Valid: true},
	})

	return connect.NewResponse(&supervisorv1.SetAppSettingResponse{
		Key:   setting.Key,
		Value: setting.Value,
	}), nil
}
