package provider

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockProvider provides a mock implementation of GitProvider for testing.
type MockProvider struct {
	RegistrationTokenFn   func(ctx context.Context, scope RegistrationScope, targetURL string) (string, error)
	ValidateCredentialsFn func(ctx context.Context) error
	ScalingModeFn         func() ScalingMode
	PollQueuedJobsFn      func(ctx context.Context, targetURL string) (int, error)
	DeregisterRunnerFn    func(ctx context.Context, scope RegistrationScope, targetURL, runnerName string) error
	GetRenovateTokenFn    func(ctx context.Context, targetURL string) (string, error)
}

// GetRegistrationToken delegates to RegistrationTokenFn if set, otherwise returns a default token.
func (m *MockProvider) GetRegistrationToken(ctx context.Context, scope RegistrationScope, targetURL string) (string, error) {
	if m.RegistrationTokenFn != nil {
		return m.RegistrationTokenFn(ctx, scope, targetURL)
	}
	return "mock-registration-token", nil
}

// ValidateCredentials delegates to ValidateCredentialsFn if set, otherwise returns nil.
func (m *MockProvider) ValidateCredentials(ctx context.Context) error {
	if m.ValidateCredentialsFn != nil {
		return m.ValidateCredentialsFn(ctx)
	}
	return nil
}

// ScalingMode delegates to ScalingModeFn if set, otherwise returns ScalingWebhook.
func (m *MockProvider) ScalingMode() ScalingMode {
	if m.ScalingModeFn != nil {
		return m.ScalingModeFn()
	}
	return ScalingWebhook
}

// PollQueuedJobs delegates to PollQueuedJobsFn if set, otherwise returns 0.
func (m *MockProvider) PollQueuedJobs(ctx context.Context, targetURL string) (int, error) {
	if m.PollQueuedJobsFn != nil {
		return m.PollQueuedJobsFn(ctx, targetURL)
	}
	return 0, nil
}

// DeregisterRunner delegates to DeregisterRunnerFn if set, otherwise returns nil.
func (m *MockProvider) DeregisterRunner(ctx context.Context, scope RegistrationScope, targetURL, runnerName string) error {
	if m.DeregisterRunnerFn != nil {
		return m.DeregisterRunnerFn(ctx, scope, targetURL, runnerName)
	}
	return nil
}

// GetRenovateToken delegates to GetRenovateTokenFn if set, otherwise returns a mock token.
func (m *MockProvider) GetRenovateToken(ctx context.Context, targetURL string) (string, error) {
	if m.GetRenovateTokenFn != nil {
		return m.GetRenovateTokenFn(ctx, targetURL)
	}
	return "mock-renovate-token", nil
}

var _ GitProvider = (*MockProvider)(nil)
var _ RunnerDeregistrar = (*MockProvider)(nil)
var _ RenovateTokenProvider = (*MockProvider)(nil)

// MockGitProvider is a testify/mock implementation of GitProvider, RunnerDeregistrar, and RenovateTokenProvider.
type MockGitProvider struct {
	mock.Mock
}

// NewMockGitProvider returns a new MockGitProvider.
func NewMockGitProvider() *MockGitProvider {
	return &MockGitProvider{}
}

func (m *MockGitProvider) GetRegistrationToken(ctx context.Context, scope RegistrationScope, targetURL string) (string, error) {
	args := m.Called(ctx, scope, targetURL)
	return args.String(0), args.Error(1)
}

func (m *MockGitProvider) ValidateCredentials(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGitProvider) ScalingMode() ScalingMode {
	args := m.Called()
	return args.Get(0).(ScalingMode)
}

func (m *MockGitProvider) PollQueuedJobs(ctx context.Context, targetURL string) (int, error) {
	args := m.Called(ctx, targetURL)
	return args.Int(0), args.Error(1)
}

func (m *MockGitProvider) DeregisterRunner(ctx context.Context, scope RegistrationScope, targetURL, runnerName string) error {
	args := m.Called(ctx, scope, targetURL, runnerName)
	return args.Error(0)
}

func (m *MockGitProvider) GetRenovateToken(ctx context.Context, targetURL string) (string, error) {
	args := m.Called(ctx, targetURL)
	return args.String(0), args.Error(1)
}

var _ GitProvider = (*MockGitProvider)(nil)
var _ RunnerDeregistrar = (*MockGitProvider)(nil)
var _ RenovateTokenProvider = (*MockGitProvider)(nil)
