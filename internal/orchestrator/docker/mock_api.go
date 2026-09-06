package docker

import (
	"context"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/mock"
)

// MockDockerAPIClient is a testify/mock implementation of APIClient.
type MockDockerAPIClient struct {
	mock.Mock
}

// NewMockDockerAPIClient returns a new MockDockerAPIClient.
func NewMockDockerAPIClient() *MockDockerAPIClient {
	return &MockDockerAPIClient{}
}

func (m *MockDockerAPIClient) Ping(ctx context.Context, options client.PingOptions) (client.PingResult, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(client.PingResult), args.Error(1)
}

func (m *MockDockerAPIClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDockerAPIClient) ContainerCreate(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(client.ContainerCreateResult), args.Error(1)
}

func (m *MockDockerAPIClient) ContainerStart(ctx context.Context, containerID string, options client.ContainerStartOptions) (client.ContainerStartResult, error) {
	args := m.Called(ctx, containerID, options)
	if res := args.Get(0); res != nil {
		return res.(client.ContainerStartResult), args.Error(1)
	}
	return client.ContainerStartResult{}, args.Error(1)
}

func (m *MockDockerAPIClient) ContainerStop(ctx context.Context, containerID string, options client.ContainerStopOptions) (client.ContainerStopResult, error) {
	args := m.Called(ctx, containerID, options)
	if res := args.Get(0); res != nil {
		return res.(client.ContainerStopResult), args.Error(1)
	}
	return client.ContainerStopResult{}, args.Error(1)
}

func (m *MockDockerAPIClient) ContainerRemove(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	args := m.Called(ctx, containerID, options)
	if res := args.Get(0); res != nil {
		return res.(client.ContainerRemoveResult), args.Error(1)
	}
	return client.ContainerRemoveResult{}, args.Error(1)
}

func (m *MockDockerAPIClient) ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error) {
	args := m.Called(ctx, options)
	if res := args.Get(0); res != nil {
		return res.(client.ContainerListResult), args.Error(1)
	}
	return client.ContainerListResult{}, args.Error(1)
}

func (m *MockDockerAPIClient) ContainerLogs(ctx context.Context, containerID string, options client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
	args := m.Called(ctx, containerID, options)
	if res := args.Get(0); res != nil {
		return res.(client.ContainerLogsResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockDockerAPIClient) ContainerPrune(ctx context.Context, opts client.ContainerPruneOptions) (client.ContainerPruneResult, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).(client.ContainerPruneResult), args.Error(1)
}

func (m *MockDockerAPIClient) Events(ctx context.Context, options client.EventsListOptions) client.EventsResult {
	args := m.Called(ctx, options)
	if res := args.Get(0); res != nil {
		return res.(client.EventsResult)
	}
	return client.EventsResult{}
}

func (m *MockDockerAPIClient) NetworkList(ctx context.Context, options client.NetworkListOptions) (client.NetworkListResult, error) {
	args := m.Called(ctx, options)
	if res := args.Get(0); res != nil {
		return res.(client.NetworkListResult), args.Error(1)
	}
	return client.NetworkListResult{}, args.Error(1)
}

func (m *MockDockerAPIClient) NetworkCreate(ctx context.Context, name string, options client.NetworkCreateOptions) (client.NetworkCreateResult, error) {
	args := m.Called(ctx, name, options)
	return args.Get(0).(client.NetworkCreateResult), args.Error(1)
}

func (m *MockDockerAPIClient) NetworkConnect(ctx context.Context, networkID string, options client.NetworkConnectOptions) (client.NetworkConnectResult, error) {
	args := m.Called(ctx, networkID, options)
	if res := args.Get(0); res != nil {
		return res.(client.NetworkConnectResult), args.Error(1)
	}
	return client.NetworkConnectResult{}, args.Error(1)
}

func (m *MockDockerAPIClient) ImageInspect(ctx context.Context, imageID string, inspectOpts ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	args := m.Called(ctx, imageID, inspectOpts)
	if res := args.Get(0); res != nil {
		return res.(client.ImageInspectResult), args.Error(1)
	}
	return client.ImageInspectResult{}, args.Error(1)
}

func (m *MockDockerAPIClient) ImagePull(ctx context.Context, refStr string, options client.ImagePullOptions) (client.ImagePullResponse, error) {
	args := m.Called(ctx, refStr, options)
	if res := args.Get(0); res != nil {
		return res.(client.ImagePullResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

var _ APIClient = (*MockDockerAPIClient)(nil)
