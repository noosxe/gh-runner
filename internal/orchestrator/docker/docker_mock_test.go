package docker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/noosxe/gh-runner/internal/orchestrator"
	"github.com/noosxe/gh-runner/internal/orchestrator/docker"
)

func TestDockerClient_WithMockDockerAPIClient(t *testing.T) {
	ctx := context.Background()

	t.Run("Bootstrap and Ping Success", func(t *testing.T) {
		mockAPI := docker.NewMockDockerAPIClient()
		mockAPI.On("Ping", mock.Anything).Return(types.Ping{APIVersion: "1.47"}, nil).Once()

		client, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
		require.NoError(t, err)
		require.NotNil(t, client)
		assert.False(t, client.IsDegraded())

		err = client.Ping(ctx)
		require.NoError(t, err)

		mockAPI.AssertExpectations(t)
	})

	t.Run("SpawnRunner Success and Label Verification", func(t *testing.T) {
		mockAPI := docker.NewMockDockerAPIClient()
		mockAPI.On("NetworkList", mock.Anything, mock.AnythingOfType("network.ListOptions")).Return([]network.Summary{
			{ID: "net-default-id", Name: orchestrator.DefaultNetworkName},
		}, nil).Once()

		expectedContainerID := "cnt-testify-123456"
		mockAPI.On("ContainerCreate",
			mock.Anything,
			mock.MatchedBy(func(cfg *container.Config) bool {
				return cfg.Labels[orchestrator.LabelManaged] == "true" &&
					cfg.Labels[orchestrator.LabelPoolName] == "ci-pool" &&
					cfg.Image == "ghcr.io/actions/runner:latest"
			}),
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.AnythingOfType("string"),
		).Return(container.CreateResponse{ID: expectedContainerID}, nil).Once()

		mockAPI.On("ContainerStart",
			mock.Anything,
			expectedContainerID,
			mock.AnythingOfType("container.StartOptions"),
		).Return(nil).Once()

		client, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
		require.NoError(t, err)

		cfg := orchestrator.RunnerConfig{
			Name:     "ghrs-ci-pool-abc12345",
			PoolName: "ci-pool",
			Image:    "ghcr.io/actions/runner:latest",
			RepoURL:  "https://github.com/org/repo",
			Token:    "mock-runner-token",
		}

		id, err := client.SpawnRunner(ctx, cfg)
		require.NoError(t, err)
		assert.Equal(t, expectedContainerID, id)

		mockAPI.AssertExpectations(t)
	})

	t.Run("SpawnRunner Create Failure Cleans Up", func(t *testing.T) {
		mockAPI := docker.NewMockDockerAPIClient()
		mockAPI.On("NetworkList", mock.Anything, mock.AnythingOfType("network.ListOptions")).Return([]network.Summary{
			{ID: "net-default-id", Name: orchestrator.DefaultNetworkName},
		}, nil).Once()

		createErr := errors.New("out of disk space")
		mockAPI.On("ContainerCreate",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(container.CreateResponse{}, createErr).Once()

		client, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
		require.NoError(t, err)

		cfg := orchestrator.RunnerConfig{
			Name:     "ghrs-ci-pool-fail",
			PoolName: "ci-pool",
			Image:    "ghcr.io/actions/runner:latest",
		}

		id, err := client.SpawnRunner(ctx, cfg)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, createErr))
		assert.Empty(t, id)

		mockAPI.AssertExpectations(t)
	})

	t.Run("TerminateRunner Stops and Removes Container", func(t *testing.T) {
		mockAPI := docker.NewMockDockerAPIClient()
		targetID := "cnt-terminate-999"
		mockAPI.On("ContainerStop", mock.Anything, targetID, mock.AnythingOfType("container.StopOptions")).Return(nil).Once()
		mockAPI.On("ContainerRemove", mock.Anything, targetID, mock.MatchedBy(func(opts container.RemoveOptions) bool {
			return opts.Force
		})).Return(nil).Once()

		client, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
		require.NoError(t, err)

		err = client.TerminateRunner(ctx, targetID)
		assert.NoError(t, err)

		mockAPI.AssertExpectations(t)
	})

	t.Run("PruneExitedContainers Calls Prune", func(t *testing.T) {
		mockAPI := docker.NewMockDockerAPIClient()
		mockAPI.On("ContainersPrune", mock.Anything, mock.MatchedBy(func(args filters.Args) bool {
			return args.ExactMatch("label", orchestrator.LabelManaged+"=true")
		})).Return(container.PruneReport{
			ContainersDeleted: []string{"dead-c1", "dead-c2"},
			SpaceReclaimed:    1024 * 1024 * 50,
		}, nil).Once()

		client, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
		require.NoError(t, err)

		err = client.PruneExitedContainers(ctx)
		assert.NoError(t, err)

		mockAPI.AssertExpectations(t)
	})

	t.Run("EnsureNetwork Idempotency", func(t *testing.T) {
		mockAPI := docker.NewMockDockerAPIClient()
		// Case 1: Network already exists
		mockAPI.On("NetworkList", mock.Anything, mock.AnythingOfType("network.ListOptions")).Return([]network.Summary{
			{ID: "net-existing-id", Name: "gh-runner-net"},
		}, nil).Once()

		client, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
		require.NoError(t, err)

		netID, err := client.EnsureNetwork(ctx, "gh-runner-net")
		require.NoError(t, err)
		assert.Equal(t, "net-existing-id", netID)

		mockAPI.AssertExpectations(t)
	})

	t.Run("AuditRunners Parses Status Correctly", func(t *testing.T) {
		mockAPI := docker.NewMockDockerAPIClient()
		spawnedTime := time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
		mockAPI.On("ContainerList", mock.Anything, mock.AnythingOfType("container.ListOptions")).Return([]container.Summary{
			{
				ID:    "cnt-audited-1",
				Names: []string{"/ghrs-pool1-0001"},
				State: "running",
				Labels: map[string]string{
					orchestrator.LabelManaged:   "true",
					orchestrator.LabelPoolName:  "pool1",
					orchestrator.LabelSpawnedAt: spawnedTime,
				},
				NetworkSettings: &container.NetworkSettingsSummary{
					Networks: map[string]*network.EndpointSettings{
						"bridge": {IPAddress: "172.17.0.2"},
					},
				},
			},
		}, nil).Once()

		client, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
		require.NoError(t, err)

		statuses, err := client.AuditRunners(ctx)
		require.NoError(t, err)
		require.Len(t, statuses, 1)
		assert.Equal(t, "cnt-audited-1", statuses[0].ID)
		assert.Equal(t, "ghrs-pool1-0001", statuses[0].Name)
		assert.Equal(t, "pool1", statuses[0].PoolName)
		assert.Equal(t, "running", statuses[0].State)
		assert.Equal(t, "172.17.0.2", statuses[0].IPAddress)

		mockAPI.AssertExpectations(t)
	})

	t.Run("Close Closes Underlying Client", func(t *testing.T) {
		mockAPI := docker.NewMockDockerAPIClient()
		mockAPI.On("Close").Return(nil).Once()

		client, err := docker.NewClient(ctx, docker.WithAPIClient(mockAPI))
		require.NoError(t, err)

		err = client.Close()
		assert.NoError(t, err)

		mockAPI.AssertExpectations(t)
	})
}
