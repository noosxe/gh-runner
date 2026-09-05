package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerimage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
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

func (m *MockDockerAPIClient) Ping(ctx context.Context) (types.Ping, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.Ping), args.Error(1)
}

func (m *MockDockerAPIClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDockerAPIClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
	args := m.Called(ctx, config, hostConfig, networkingConfig, platform, containerName)
	return args.Get(0).(container.CreateResponse), args.Error(1)
}

func (m *MockDockerAPIClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerAPIClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerAPIClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerAPIClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	args := m.Called(ctx, options)
	if res := args.Get(0); res != nil {
		return res.([]container.Summary), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockDockerAPIClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, containerID, options)
	if res := args.Get(0); res != nil {
		return res.(io.ReadCloser), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockDockerAPIClient) ContainersPrune(ctx context.Context, pruneFilters filters.Args) (container.PruneReport, error) {
	args := m.Called(ctx, pruneFilters)
	return args.Get(0).(container.PruneReport), args.Error(1)
}

func (m *MockDockerAPIClient) Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
	args := m.Called(ctx, options)
	var msgChan <-chan events.Message
	if res := args.Get(0); res != nil {
		msgChan = res.(<-chan events.Message)
	}
	var errChan <-chan error
	if res := args.Get(1); res != nil {
		errChan = res.(<-chan error)
	}
	return msgChan, errChan
}

func (m *MockDockerAPIClient) NetworkList(ctx context.Context, options network.ListOptions) ([]network.Summary, error) {
	args := m.Called(ctx, options)
	if res := args.Get(0); res != nil {
		return res.([]network.Summary), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockDockerAPIClient) NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error) {
	args := m.Called(ctx, name, options)
	return args.Get(0).(network.CreateResponse), args.Error(1)
}

func (m *MockDockerAPIClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	args := m.Called(ctx, networkID, containerID, config)
	return args.Error(0)
}

func (m *MockDockerAPIClient) ImageInspectWithRaw(ctx context.Context, imageID string) (dockerimage.InspectResponse, []byte, error) {
	args := m.Called(ctx, imageID)
	var raw []byte
	if b := args.Get(1); b != nil {
		raw = b.([]byte)
	}
	return args.Get(0).(dockerimage.InspectResponse), raw, args.Error(2)
}

func (m *MockDockerAPIClient) ImagePull(ctx context.Context, refStr string, options dockerimage.PullOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, refStr, options)
	if res := args.Get(0); res != nil {
		return res.(io.ReadCloser), args.Error(1)
	}
	return nil, args.Error(1)
}

var _ APIClient = (*MockDockerAPIClient)(nil)
