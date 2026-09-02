package docker_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
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
	eventsFn          func(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
	networkListFn     func(ctx context.Context, options network.ListOptions) ([]network.Summary, error)
	networkCreateFn   func(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error)
	networkConnectFn  func(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error
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

func (m *mockDockerAPI) Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
	if m.eventsFn != nil {
		return m.eventsFn(ctx, options)
	}
	return nil, nil
}

func (m *mockDockerAPI) NetworkList(ctx context.Context, options network.ListOptions) ([]network.Summary, error) {
	if m.networkListFn != nil {
		return m.networkListFn(ctx, options)
	}
	return []network.Summary{}, nil
}

func (m *mockDockerAPI) NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error) {
	if m.networkCreateFn != nil {
		return m.networkCreateFn(ctx, name, options)
	}
	return network.CreateResponse{ID: "mock-net-id"}, nil
}

func (m *mockDockerAPI) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	if m.networkConnectFn != nil {
		return m.networkConnectFn(ctx, networkID, containerID, config)
	}
	return nil
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

func TestDockerClient_SpawnRunner(t *testing.T) {
	ctx := context.Background()

	var createdConfig *container.Config
	var createdHostConfig *container.HostConfig
	var createdNetConfig *network.NetworkingConfig
	var createdName string
	var startedID string

	mockAPI := &mockDockerAPI{
		containerCreateFn: func(ctx context.Context, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig, platform *v1.Platform, name string) (container.CreateResponse, error) {
			createdConfig = cfg
			createdHostConfig = hostCfg
			createdNetConfig = netCfg
			createdName = name
			return container.CreateResponse{ID: "c-123456"}, nil
		},
		containerStartFn: func(ctx context.Context, id string, options container.StartOptions) error {
			startedID = id
			return nil
		},
	}

	cli, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	config := orchestrator.RunnerConfig{
		PoolName:    "arm64-pool",
		RepoURL:     "https://github.com/my-org/my-repo",
		Token:       "runner-ephemeral-tok",
		Labels:      []string{"self-hosted", "arm64"},
		WorkDir:     "_custom_work",
		CPULimit:    "1.5",
		MemoryLimit: "2g",
		AllowDocker: true,
	}

	id, err := cli.SpawnRunner(ctx, config)
	if err != nil {
		t.Fatalf("SpawnRunner failed: %v", err)
	}
	if id != "c-123456" || startedID != "c-123456" {
		t.Fatalf("expected container c-123456 created and started, got %q / %q", id, startedID)
	}

	// Verify Naming
	if !strings.HasPrefix(createdName, "ghrs-arm64-pool-") {
		t.Errorf("unexpected container name: %q", createdName)
	}

	// Verify Labels
	if createdConfig.Labels[orchestrator.LabelManaged] != "true" {
		t.Errorf("missing managed label")
	}
	if createdConfig.Labels[orchestrator.LabelPoolName] != "arm64-pool" {
		t.Errorf("unexpected pool label: %v", createdConfig.Labels[orchestrator.LabelPoolName])
	}
	if createdConfig.Labels[orchestrator.LabelTaskType] != orchestrator.TaskTypeRunner {
		t.Errorf("unexpected task-type label: %v", createdConfig.Labels[orchestrator.LabelTaskType])
	}
	if createdConfig.Labels[orchestrator.LabelSpawnedAt] == "" {
		t.Errorf("missing spawned-at label")
	}

	// Verify Limits
	if createdHostConfig.NanoCPUs != 1500000000 {
		t.Errorf("expected 1.5e9 NanoCPUs, got %d", createdHostConfig.NanoCPUs)
	}
	if createdHostConfig.Memory != 2*1024*1024*1024 {
		t.Errorf("expected 2GiB memory, got %d", createdHostConfig.Memory)
	}

	// Verify Docker socket mount
	foundDockerSock := false
	for _, bind := range createdHostConfig.Binds {
		if bind == "/var/run/docker.sock:/var/run/docker.sock" {
			foundDockerSock = true
			break
		}
	}
	if !foundDockerSock {
		t.Errorf("expected /var/run/docker.sock mount in binds: %+v", createdHostConfig.Binds)
	}

	// Verify Environment Variables
	envMap := make(map[string]string)
	for _, e := range createdConfig.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	if envMap["RUNNER_TOKEN"] != "runner-ephemeral-tok" {
		t.Errorf("unexpected token in env: %v", envMap["RUNNER_TOKEN"])
	}
	if envMap["GITHUB_REPOSITORY_URL"] != "https://github.com/my-org/my-repo" {
		t.Errorf("unexpected repo URL in env: %v", envMap["GITHUB_REPOSITORY_URL"])
	}
	if envMap["RUNNER_WORKDIR"] != "_custom_work" {
		t.Errorf("unexpected workdir: %v", envMap["RUNNER_WORKDIR"])
	}
	if envMap["RUNNER_EPHEMERAL"] != "true" {
		t.Errorf("missing RUNNER_EPHEMERAL=true")
	}

	// Verify Network Attachment
	if createdNetConfig == nil || createdNetConfig.EndpointsConfig[orchestrator.DefaultNetworkName] == nil {
		t.Errorf("expected container attached to %q network: %+v", orchestrator.DefaultNetworkName, createdNetConfig)
	}
}

func TestDockerClient_SpawnTask(t *testing.T) {
	ctx := context.Background()

	var createdConfig *container.Config
	mockAPI := &mockDockerAPI{
		containerCreateFn: func(ctx context.Context, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig, platform *v1.Platform, name string) (container.CreateResponse, error) {
			createdConfig = cfg
			return container.CreateResponse{ID: "task-999"}, nil
		},
	}

	cli, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	config := orchestrator.RunnerConfig{
		PoolName: "renovate-pool",
		RepoURL:  "https://github.com/org/repo",
		Token:    "renovate-short-lived-tok",
		Image:    "renovate/renovate:latest",
	}

	id, err := cli.SpawnTask(ctx, config)
	if err != nil {
		t.Fatalf("SpawnTask failed: %v", err)
	}
	if id != "task-999" {
		t.Fatalf("unexpected id: %q", id)
	}

	if createdConfig.Labels[orchestrator.LabelTaskType] != orchestrator.TaskTypeJob {
		t.Errorf("expected task-type job, got %v", createdConfig.Labels[orchestrator.LabelTaskType])
	}
	if createdConfig.Image != "renovate/renovate:latest" {
		t.Errorf("unexpected image: %q", createdConfig.Image)
	}

	envMap := make(map[string]string)
	for _, e := range createdConfig.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	if envMap["RENOVATE_TOKEN"] != "renovate-short-lived-tok" {
		t.Errorf("unexpected renovate token: %v", envMap["RENOVATE_TOKEN"])
	}
}

func TestDockerClient_SpawnFailureCleanup(t *testing.T) {
	ctx := context.Background()

	var removedID string
	var forceRemoved bool

	mockAPI := &mockDockerAPI{
		containerCreateFn: func(ctx context.Context, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig, platform *v1.Platform, name string) (container.CreateResponse, error) {
			return container.CreateResponse{ID: "doomed-container"}, nil
		},
		containerStartFn: func(ctx context.Context, id string, options container.StartOptions) error {
			return errors.New("cannot start: port conflict")
		},
		containerRemoveFn: func(ctx context.Context, id string, options container.RemoveOptions) error {
			removedID = id
			forceRemoved = options.Force
			return nil
		},
	}

	cli, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = cli.SpawnRunner(ctx, orchestrator.RunnerConfig{PoolName: "test-pool"})
	if err == nil || !strings.Contains(err.Error(), "starting container") {
		t.Fatalf("expected start container error, got: %v", err)
	}
	if removedID != "doomed-container" || !forceRemoved {
		t.Errorf("expected doomed-container to be force removed, got %q (force=%v)", removedID, forceRemoved)
	}
}

func TestParseLimits(t *testing.T) {
	// 1. CPU Limits
	nano, err := docker.ParseCPULimit("2.5")
	if err != nil || nano != 2500000000 {
		t.Errorf("expected 2.5e9 nano cpus, got %d (err=%v)", nano, err)
	}
	nano, err = docker.ParseCPULimit("")
	if err != nil || nano != 0 {
		t.Errorf("expected 0 for empty cpu, got %d", nano)
	}
	_, err = docker.ParseCPULimit("-1")
	if err == nil {
		t.Errorf("expected error for negative cpu limit")
	}
	_, err = docker.ParseCPULimit("invalid")
	if err == nil {
		t.Errorf("expected error for invalid cpu limit")
	}

	// 2. Memory Limits
	memTests := []struct {
		input    string
		expected int64
	}{
		{"4g", 4 * 1024 * 1024 * 1024},
		{"512m", 512 * 1024 * 1024},
		{"1024k", 1024 * 1024},
		{"1000b", 1000},
		{"", 0},
	}
	for _, tc := range memTests {
		bytes, err := docker.ParseMemoryLimit(tc.input)
		if err != nil {
			t.Errorf("ParseMemoryLimit(%q) failed: %v", tc.input, err)
		}
		if bytes != tc.expected {
			t.Errorf("ParseMemoryLimit(%q) = %d, expected %d", tc.input, bytes, tc.expected)
		}
	}

	_, err = docker.ParseMemoryLimit("bad-mem")
	if err == nil {
		t.Errorf("expected error for bad memory limit")
	}
	_, err = docker.ParseMemoryLimit("-100m")
	if err == nil {
		t.Errorf("expected error for negative memory limit")
	}
}

func TestDockerClient_AuditRunners(t *testing.T) {
	ctx := context.Background()

	var filterUsed filters.Args
	mockAPI := &mockDockerAPI{
		containerListFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
			filterUsed = options.Filters
			return []container.Summary{
				{
					ID:    "cnt-1",
					Names: []string{"/ghrs-pool-a-abcdef"},
					State: "running",
					Labels: map[string]string{
						orchestrator.LabelManaged:   "true",
						orchestrator.LabelPoolName:  "pool-a",
						orchestrator.LabelID:        "ghrs-pool-a-abcdef",
						orchestrator.LabelSpawnedAt: "2026-09-03T12:00:00Z",
					},
					NetworkSettings: &container.NetworkSettingsSummary{
						Networks: map[string]*network.EndpointSettings{
							"bridge": {IPAddress: "172.17.0.2"},
						},
					},
				},
			}, nil
		},
	}

	cli, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	statuses, err := cli.AuditRunners(ctx)
	if err != nil {
		t.Fatalf("AuditRunners failed: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("expected 1 runner status, got %d", len(statuses))
	}
	s := statuses[0]
	if s.ID != "cnt-1" || s.Name != "ghrs-pool-a-abcdef" || s.PoolName != "pool-a" || s.State != "running" || s.IPAddress != "172.17.0.2" {
		t.Errorf("unexpected mapped runner status: %+v", s)
	}

	// Verify label filter
	labels := filterUsed.Get("label")
	if len(labels) == 0 || labels[0] != orchestrator.LabelManaged+"=true" {
		t.Errorf("expected managed label filter, got %+v", labels)
	}
}

func TestDockerClient_TerminateRunner(t *testing.T) {
	ctx := context.Background()

	var stoppedID string
	var removedID string
	var forceRemoved bool

	mockAPI := &mockDockerAPI{
		containerStopFn: func(ctx context.Context, containerID string, options container.StopOptions) error {
			stoppedID = containerID
			return nil
		},
		containerRemoveFn: func(ctx context.Context, containerID string, options container.RemoveOptions) error {
			removedID = containerID
			forceRemoved = options.Force
			return nil
		},
	}

	cli, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if err := cli.TerminateRunner(ctx, "cnt-to-terminate"); err != nil {
		t.Fatalf("TerminateRunner failed: %v", err)
	}
	if stoppedID != "cnt-to-terminate" || removedID != "cnt-to-terminate" || !forceRemoved {
		t.Errorf("terminate failed: stopped=%q removed=%q force=%v", stoppedID, removedID, forceRemoved)
	}
}

func TestDockerClient_PruneExitedContainers(t *testing.T) {
	ctx := context.Background()

	var pruneFiltersUsed filters.Args
	mockAPI := &mockDockerAPI{
		containersPruneFn: func(ctx context.Context, pruneFilters filters.Args) (container.PruneReport, error) {
			pruneFiltersUsed = pruneFilters
			return container.PruneReport{
				ContainersDeleted: []string{"c-dead-1", "c-dead-2"},
				SpaceReclaimed:    1024,
			}, nil
		},
	}

	cli, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if err := cli.PruneExitedContainers(ctx); err != nil {
		t.Fatalf("PruneExitedContainers failed: %v", err)
	}

	labels := pruneFiltersUsed.Get("label")
	if len(labels) == 0 || labels[0] != orchestrator.LabelManaged+"=true" {
		t.Errorf("expected managed label in prune filters: %+v", labels)
	}
}

func TestDockerClient_Events(t *testing.T) {
	ctx := context.Background()

	msgChan := make(chan events.Message, 1)
	errChan := make(chan error, 1)

	mockAPI := &mockDockerAPI{
		eventsFn: func(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
			return msgChan, errChan
		},
	}

	cli, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	outMsg, outErr := cli.Events(ctx, events.ListOptions{})
	if outMsg == nil || outErr == nil {
		t.Fatalf("expected non-nil event channels")
	}
}

func TestDockerClient_EnsureNetwork(t *testing.T) {
	ctx := context.Background()

	// 1. Network already exists
	listCalled := false
	createCalled := false
	mockAPIExisting := &mockDockerAPI{
		networkListFn: func(ctx context.Context, options network.ListOptions) ([]network.Summary, error) {
			listCalled = true
			return []network.Summary{
				{
					ID:   "net-existing-123",
					Name: orchestrator.DefaultNetworkName,
				},
			}, nil
		},
		networkCreateFn: func(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error) {
			createCalled = true
			return network.CreateResponse{ID: "should-not-be-called"}, nil
		},
	}

	cli, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPIExisting))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	id, err := cli.EnsureNetwork(ctx, "")
	if err != nil {
		t.Fatalf("EnsureNetwork failed: %v", err)
	}
	if id != "net-existing-123" {
		t.Errorf("expected existing network id net-existing-123, got %q", id)
	}
	if !listCalled || createCalled {
		t.Errorf("expected listCalled=true and createCalled=false, got list=%v create=%v", listCalled, createCalled)
	}

	// 2. Network does not exist -> created with driver bridge and managed label
	var createdNetName string
	var createdOptions network.CreateOptions
	mockAPINew := &mockDockerAPI{
		networkListFn: func(ctx context.Context, options network.ListOptions) ([]network.Summary, error) {
			return []network.Summary{}, nil
		},
		networkCreateFn: func(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error) {
			createdNetName = name
			createdOptions = options
			return network.CreateResponse{ID: "net-created-456"}, nil
		},
	}

	cliNew, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPINew))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	id, err = cliNew.EnsureNetwork(ctx, "custom-net")
	if err != nil {
		t.Fatalf("EnsureNetwork failed: %v", err)
	}
	if id != "net-created-456" {
		t.Errorf("expected created network id net-created-456, got %q", id)
	}
	if createdNetName != "custom-net" {
		t.Errorf("expected network name custom-net, got %q", createdNetName)
	}
	if createdOptions.Driver != "bridge" {
		t.Errorf("expected bridge driver, got %q", createdOptions.Driver)
	}
	if createdOptions.Labels[orchestrator.LabelManaged] != "true" {
		t.Errorf("expected managed label on created network: %+v", createdOptions.Labels)
	}
}
