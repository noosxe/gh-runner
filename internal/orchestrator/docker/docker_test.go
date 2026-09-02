package docker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/noosxe/gh-runner/internal/orchestrator"
	"github.com/noosxe/gh-runner/internal/orchestrator/docker"
)

type mockDockerAPI struct {
	pingFn            func(ctx context.Context) (types.Ping, error)
	closeFn           func() error
	containerCreateFn func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	containerStartFn  func(ctx context.Context, containerID string, options container.StartOptions) error
	containerStopFn   func(ctx context.Context, containerID string, options container.StopOptions) error
	containerRemoveFn func(ctx context.Context, containerID string, options container.RemoveOptions) error
	containerListFn   func(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	containersPruneFn func(ctx context.Context, pruneFilters filters.Args) (container.PruneReport, error)
}

func (m *mockDockerAPI) Ping(ctx context.Context) (types.Ping, error) {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return types.Ping{APIVersion: "1.47"}, nil
}

func (m *mockDockerAPI) Close() error {
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func (m *mockDockerAPI) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
	if m.containerCreateFn != nil {
		return m.containerCreateFn(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "mock-container-id"}, nil
}

func (m *mockDockerAPI) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if m.containerStartFn != nil {
		return m.containerStartFn(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerAPI) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.containerStopFn != nil {
		return m.containerStopFn(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerAPI) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if m.containerRemoveFn != nil {
		return m.containerRemoveFn(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerAPI) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	if m.containerListFn != nil {
		return m.containerListFn(ctx, options)
	}
	return []container.Summary{}, nil
}

func (m *mockDockerAPI) ContainersPrune(ctx context.Context, pruneFilters filters.Args) (container.PruneReport, error) {
	if m.containersPruneFn != nil {
		return m.containersPruneFn(ctx, pruneFilters)
	}
	return container.PruneReport{}, nil
}

func TestDockerClient_BootstrapAndPing(t *testing.T) {
	ctx := context.Background()

	// 1. Successful Ping
	mockAPI := &mockDockerAPI{}
	cli, err := docker.NewClient(ctx,
		docker.WithAPIClient(mockAPI),
		docker.WithHost("tcp://remote-docker:2376"),
		docker.WithDockerHostID("remote-host-1"),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	var _ orchestrator.ContainerProvider = cli

	if err := cli.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if cli.DockerHostID() != "remote-host-1" {
		t.Errorf("unexpected DockerHostID: %q", cli.DockerHostID())
	}

	// 2. Failed Ping (daemon unreachable)
	failingAPI := &mockDockerAPI{
		pingFn: func(ctx context.Context) (types.Ping, error) {
			return types.Ping{}, errors.New("connection refused")
		},
	}
	failingCli, err := docker.NewClient(ctx, docker.WithAPIClient(failingAPI))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	err = failingCli.Ping(ctx)
	if err == nil || !errors.Is(err, docker.ErrDaemonUnreachable) {
		t.Fatalf("expected ErrDaemonUnreachable, got: %v", err)
	}

	// 3. Close
	if err := cli.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
