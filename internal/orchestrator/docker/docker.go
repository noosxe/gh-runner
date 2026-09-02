package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/noosxe/gh-runner/internal/orchestrator"
)

var (
	// ErrNilClient is returned if a nil docker client is provided.
	ErrNilClient = errors.New("docker client cannot be nil")
	// ErrDaemonUnreachable is returned when pinging the Docker daemon fails.
	ErrDaemonUnreachable = errors.New("docker daemon unreachable")
)

// APIClient abstracts the Docker Engine SDK methods utilized by the orchestrator.
type APIClient interface {
	Ping(ctx context.Context) (types.Ping, error)
	Close() error
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainersPrune(ctx context.Context, pruneFilters filters.Args) (container.PruneReport, error)
}

// Client implements orchestrator.ContainerProvider using the Docker Engine API.
type Client struct {
	docker       APIClient
	host         string
	dockerHostID string // Groundwork for multi-host Docker pools (OQ #22)
	mu           sync.RWMutex
}

// Option configures the Docker orchestrator Client.
type Option func(*options)

type options struct {
	host         string
	certPath     string
	verifyTLS    bool
	dockerHostID string
	apiClient    APIClient
}

// WithHost configures a custom Docker host endpoint (e.g. unix:///var/run/docker.sock or tcp://remote:2376).
func WithHost(host string) Option {
	return func(o *options) {
		o.host = host
	}
}

// WithTLS configures TLS client authentication for a remote Docker daemon.
func WithTLS(certPath string, verify bool) Option {
	return func(o *options) {
		o.certPath = certPath
		o.verifyTLS = verify
	}
}

// WithDockerHostID sets the identifier for this Docker host instance (OQ #22).
func WithDockerHostID(id string) Option {
	return func(o *options) {
		o.dockerHostID = id
	}
}

// WithAPIClient injects a custom or mock Docker API client.
func WithAPIClient(apiClient APIClient) Option {
	return func(o *options) {
		o.apiClient = apiClient
	}
}

// NewClient initializes a new Docker orchestrator client.
// Honors standard Docker environment variables (DOCKER_HOST, DOCKER_TLS_VERIFY, DOCKER_CERT_PATH)
// unless overridden by options.
func NewClient(ctx context.Context, opts ...Option) (*Client, error) {
	cfg := options{
		host:         os.Getenv("DOCKER_HOST"),
		certPath:     os.Getenv("DOCKER_CERT_PATH"),
		verifyTLS:    os.Getenv("DOCKER_TLS_VERIFY") != "",
		dockerHostID: "default",
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.apiClient != nil {
		return &Client{
			docker:       cfg.apiClient,
			host:         cfg.host,
			dockerHostID: cfg.dockerHostID,
		}, nil
	}

	var clientOpts []client.Opt
	clientOpts = append(clientOpts, client.FromEnv)

	if cfg.host != "" {
		clientOpts = append(clientOpts, client.WithHost(cfg.host))
	}
	if cfg.certPath != "" {
		caCert := fmt.Sprintf("%s/ca.pem", strings.TrimRight(cfg.certPath, "/"))
		cert := fmt.Sprintf("%s/cert.pem", strings.TrimRight(cfg.certPath, "/"))
		key := fmt.Sprintf("%s/key.pem", strings.TrimRight(cfg.certPath, "/"))
		clientOpts = append(clientOpts, client.WithTLSClientConfig(caCert, cert, key))
	}

	cli, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("initializing Docker SDK client: %w", err)
	}

	return &Client{
		docker:       cli,
		host:         cfg.host,
		dockerHostID: cfg.dockerHostID,
	}, nil
}

// Ping verifies Docker daemon reachability.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	docker := c.docker
	c.mu.RUnlock()

	if docker == nil {
		return ErrNilClient
	}

	_, err := docker.Ping(ctx)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDaemonUnreachable, err)
	}
	return nil
}

// Close releases the Docker client connections.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.docker != nil {
		return c.docker.Close()
	}
	return nil
}

// SpawnRunner creates and starts an ephemeral runner container.
func (c *Client) SpawnRunner(ctx context.Context, config orchestrator.RunnerConfig) (string, error) {
	return c.spawn(ctx, config, orchestrator.TaskTypeRunner)
}

// SpawnTask creates and starts a one-off task container (e.g. Renovate bot).
func (c *Client) SpawnTask(ctx context.Context, config orchestrator.RunnerConfig) (string, error) {
	return c.spawn(ctx, config, orchestrator.TaskTypeJob)
}

func (c *Client) spawn(ctx context.Context, config orchestrator.RunnerConfig, taskType string) (string, error) {
	c.mu.RLock()
	docker := c.docker
	c.mu.RUnlock()

	if docker == nil {
		return "", ErrNilClient
	}

	containerName := config.Name
	if containerName == "" {
		containerName = orchestrator.GenerateContainerName(config.PoolName)
	}

	image := config.Image
	if image == "" {
		image = orchestrator.DefaultRunnerImage
	}

	labels := map[string]string{
		orchestrator.LabelManaged:   "true",
		orchestrator.LabelPoolName:  config.PoolName,
		orchestrator.LabelID:        containerName,
		orchestrator.LabelSpawnedAt: time.Now().UTC().Format(time.RFC3339),
		orchestrator.LabelTaskType:  taskType,
	}

	env := config.Env
	if len(env) == 0 {
		if taskType == orchestrator.TaskTypeRunner {
			workDir := config.WorkDir
			if workDir == "" {
				workDir = "_work"
			}
			env = []string{
				"RUNNER_NAME=" + containerName,
				"RUNNER_TOKEN=" + config.Token,
				"RUNNER_WORKDIR=" + workDir,
				"RUNNER_LABELS=" + strings.Join(config.Labels, ","),
				"RUNNER_EPHEMERAL=true",
			}
			if config.RepoURL != "" {
				if strings.Contains(config.RepoURL, "gitea") {
					env = append(env, "GITEA_INSTANCE_URL="+config.RepoURL)
				} else if strings.Contains(config.RepoURL, "forgejo") {
					env = append(env, "FORGEJO_INSTANCE_URL="+config.RepoURL)
				} else {
					env = append(env, "GITHUB_REPOSITORY_URL="+config.RepoURL)
				}
			}
		} else {
			env = []string{
				"RENOVATE_TOKEN=" + config.Token,
				"RENOVATE_ENDPOINT=" + config.RepoURL,
			}
		}
	}

	hostConfig := &container.HostConfig{}

	if config.AllowDocker {
		hostConfig.Binds = append(hostConfig.Binds, "/var/run/docker.sock:/var/run/docker.sock")
	}

	if config.CPULimit != "" {
		nanoCPUs, err := ParseCPULimit(config.CPULimit)
		if err != nil {
			return "", err
		}
		hostConfig.NanoCPUs = nanoCPUs
	}

	if config.MemoryLimit != "" {
		memBytes, err := ParseMemoryLimit(config.MemoryLimit)
		if err != nil {
			return "", err
		}
		hostConfig.Memory = memBytes
	}

	containerConfig := &container.Config{
		Image:  image,
		Env:    env,
		Labels: labels,
	}

	createResp, err := docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("creating container %q: %w", containerName, err)
	}

	if err := docker.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		_ = docker.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("starting container %q (%s): %w", containerName, createResp.ID, err)
	}

	return createResp.ID, nil
}

// ParseCPULimit converts CPU limits like "2.0", "0.5", "1" to NanoCPUs (int64).
func ParseCPULimit(cpu string) (int64, error) {
	trimmed := strings.TrimSpace(cpu)
	if trimmed == "" {
		return 0, nil
	}
	val, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cpu limit %q: %w", cpu, err)
	}
	if val <= 0 {
		return 0, fmt.Errorf("cpu limit must be positive: %q", cpu)
	}
	return int64(val * 1e9), nil
}

// ParseMemoryLimit parses strings like "4g", "512m", "1024k" to byte count (int64).
func ParseMemoryLimit(mem string) (int64, error) {
	s := strings.TrimSpace(strings.ToLower(mem))
	if s == "" {
		return 0, nil
	}
	multiplier := int64(1)
	switch {
	case strings.HasSuffix(s, "g") || strings.HasSuffix(s, "gb"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "b"), "g")
	case strings.HasSuffix(s, "m") || strings.HasSuffix(s, "mb"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "b"), "m")
	case strings.HasSuffix(s, "k") || strings.HasSuffix(s, "kb"):
		multiplier = 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "b"), "k")
	case strings.HasSuffix(s, "b"):
		s = strings.TrimSuffix(s, "b")
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory limit %q: %w", mem, err)
	}
	if val <= 0 {
		return 0, fmt.Errorf("memory limit must be positive: %q", mem)
	}
	return int64(val * float64(multiplier)), nil
}

// TerminateRunner gracefully stops and removes a runner container.
func (c *Client) TerminateRunner(ctx context.Context, containerID string) error {
	c.mu.RLock()
	docker := c.docker
	c.mu.RUnlock()

	if docker == nil {
		return ErrNilClient
	}

	timeoutSec := 10
	stopOpts := container.StopOptions{
		Timeout: &timeoutSec,
	}

	// Graceful stop with fallback
	if err := docker.ContainerStop(ctx, containerID, stopOpts); err != nil {
		if !cerrdefs.IsNotFound(err) {
			_ = docker.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
			return fmt.Errorf("stopping container %s: %w", containerID, err)
		}
	}

	// Remove container
	if err := docker.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return fmt.Errorf("removing container %s: %w", containerID, err)
		}
	}

	return nil
}

// AuditRunners inspects all active and exited supervisor-managed runner containers
// by querying the Docker daemon for the com.github-runner-supervisor.managed=true label.
func (c *Client) AuditRunners(ctx context.Context) ([]orchestrator.RunnerStatus, error) {
	c.mu.RLock()
	docker := c.docker
	c.mu.RUnlock()

	if docker == nil {
		return nil, ErrNilClient
	}

	filterArgs := filters.NewArgs()
	filterArgs.Add("label", orchestrator.LabelManaged+"=true")

	containers, err := docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("listing supervisor containers: %w", err)
	}

	statuses := make([]orchestrator.RunnerStatus, 0, len(containers))
	for _, cnt := range containers {
		name := ""
		if len(cnt.Names) > 0 {
			name = strings.TrimPrefix(cnt.Names[0], "/")
		}

		poolName := cnt.Labels[orchestrator.LabelPoolName]

		var spawnedAt time.Time
		if rawSpawned := cnt.Labels[orchestrator.LabelSpawnedAt]; rawSpawned != "" {
			if t, err := time.Parse(time.RFC3339, rawSpawned); err == nil {
				spawnedAt = t
			}
		}
		if spawnedAt.IsZero() {
			spawnedAt = time.Unix(cnt.Created, 0)
		}

		ipAddress := ""
		if cnt.NetworkSettings != nil && len(cnt.NetworkSettings.Networks) > 0 {
			for _, net := range cnt.NetworkSettings.Networks {
				if net.IPAddress != "" {
					ipAddress = net.IPAddress
					break
				}
			}
		}

		statuses = append(statuses, orchestrator.RunnerStatus{
			ID:        cnt.ID,
			Name:      name,
			PoolName:  poolName,
			State:     cnt.State,
			IPAddress: ipAddress,
			SpawnedAt: spawnedAt,
		})
	}

	return statuses, nil
}

// PruneExitedContainers removes containers that have finished executing.
// Full implementation delivered in RUN-33.
func (c *Client) PruneExitedContainers(ctx context.Context) error {
	return errors.New("PruneExitedContainers not yet implemented")
}

// DockerHostID returns the configured Docker host identifier.
func (c *Client) DockerHostID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dockerHostID
}

var _ orchestrator.ContainerProvider = (*Client)(nil)
