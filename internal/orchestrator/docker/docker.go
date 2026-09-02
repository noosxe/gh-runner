package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

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
// Full implementation delivered in RUN-31.
func (c *Client) SpawnRunner(ctx context.Context, config orchestrator.RunnerConfig) (string, error) {
	return "", errors.New("SpawnRunner not yet implemented")
}

// SpawnTask creates and starts a one-off task container (e.g. Renovate bot).
// Full implementation delivered in RUN-31.
func (c *Client) SpawnTask(ctx context.Context, config orchestrator.RunnerConfig) (string, error) {
	return "", errors.New("SpawnTask not yet implemented")
}

// TerminateRunner gracefully stops and removes a runner container.
// Full implementation delivered in RUN-32.
func (c *Client) TerminateRunner(ctx context.Context, containerID string) error {
	return errors.New("TerminateRunner not yet implemented")
}

// AuditRunners inspects all active and exited supervisor-managed runner containers.
// Full implementation delivered in RUN-33.
func (c *Client) AuditRunners(ctx context.Context) ([]orchestrator.RunnerStatus, error) {
	return nil, errors.New("AuditRunners not yet implemented")
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
