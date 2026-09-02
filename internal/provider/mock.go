package provider

import "context"

// MockProvider provides a mock implementation of GitProvider for testing.
type MockProvider struct {
	RegistrationTokenFn   func(ctx context.Context, scope RegistrationScope, targetURL string) (string, error)
	ValidateCredentialsFn func(ctx context.Context) error
	ScalingModeFn         func() ScalingMode
	PollQueuedJobsFn      func(ctx context.Context, targetURL string) (int, error)
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
