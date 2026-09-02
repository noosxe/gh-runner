package orchestrator

import (
	"context"
	"sync"
)

// MockContainerProvider is a mock implementation of ContainerProvider for testing.
type MockContainerProvider struct {
	mu sync.RWMutex

	SpawnRunnerFn           func(ctx context.Context, config RunnerConfig) (string, error)
	SpawnTaskFn             func(ctx context.Context, config RunnerConfig) (string, error)
	TerminateRunnerFn       func(ctx context.Context, containerID string) error
	AuditRunnersFn          func(ctx context.Context) ([]RunnerStatus, error)
	PruneExitedContainersFn func(ctx context.Context) error
	EnsureNetworkFn         func(ctx context.Context, name string) (string, error)
	PingFn                  func(ctx context.Context) error
	CloseFn                 func() error

	SpawnedRunners []RunnerConfig
	SpawnedTasks   []RunnerConfig
	TerminatedIDs  []string
}

// NewMockContainerProvider returns an initialized MockContainerProvider.
func NewMockContainerProvider() *MockContainerProvider {
	return &MockContainerProvider{}
}

// SpawnRunner delegates to SpawnRunnerFn or records and returns a mock container ID.
func (m *MockContainerProvider) SpawnRunner(ctx context.Context, config RunnerConfig) (string, error) {
	m.mu.Lock()
	m.SpawnedRunners = append(m.SpawnedRunners, config)
	fn := m.SpawnRunnerFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, config)
	}
	return "mock-container-runner-" + config.Name, nil
}

// SpawnTask delegates to SpawnTaskFn or records and returns a mock task ID.
func (m *MockContainerProvider) SpawnTask(ctx context.Context, config RunnerConfig) (string, error) {
	m.mu.Lock()
	m.SpawnedTasks = append(m.SpawnedTasks, config)
	fn := m.SpawnTaskFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, config)
	}
	return "mock-container-task-" + config.Name, nil
}

// TerminateRunner delegates to TerminateRunnerFn or records the termination.
func (m *MockContainerProvider) TerminateRunner(ctx context.Context, containerID string) error {
	m.mu.Lock()
	m.TerminatedIDs = append(m.TerminatedIDs, containerID)
	fn := m.TerminateRunnerFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, containerID)
	}
	return nil
}

// AuditRunners delegates to AuditRunnersFn or returns an empty list.
func (m *MockContainerProvider) AuditRunners(ctx context.Context) ([]RunnerStatus, error) {
	m.mu.RLock()
	fn := m.AuditRunnersFn
	m.mu.RUnlock()

	if fn != nil {
		return fn(ctx)
	}
	return []RunnerStatus{}, nil
}

// PruneExitedContainers delegates to PruneExitedContainersFn or returns nil.
func (m *MockContainerProvider) PruneExitedContainers(ctx context.Context) error {
	m.mu.RLock()
	fn := m.PruneExitedContainersFn
	m.mu.RUnlock()

	if fn != nil {
		return fn(ctx)
	}
	return nil
}

// EnsureNetwork delegates to EnsureNetworkFn or returns a mock network ID.
func (m *MockContainerProvider) EnsureNetwork(ctx context.Context, name string) (string, error) {
	m.mu.RLock()
	fn := m.EnsureNetworkFn
	m.mu.RUnlock()

	if fn != nil {
		return fn(ctx, name)
	}
	if name == "" {
		name = DefaultNetworkName
	}
	return "mock-network-id-" + name, nil
}

// Ping delegates to PingFn or returns nil.
func (m *MockContainerProvider) Ping(ctx context.Context) error {
	m.mu.RLock()
	fn := m.PingFn
	m.mu.RUnlock()

	if fn != nil {
		return fn(ctx)
	}
	return nil
}

// Close delegates to CloseFn or returns nil.
func (m *MockContainerProvider) Close() error {
	m.mu.RLock()
	fn := m.CloseFn
	m.mu.RUnlock()

	if fn != nil {
		return fn()
	}
	return nil
}

var _ ContainerProvider = (*MockContainerProvider)(nil)
