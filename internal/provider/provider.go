package provider

import "context"

// RegistrationScope defines the registration scope of a runner within the Git provider (docs/02 §3.2).
type RegistrationScope string

const (
	// ScopeRepo registers the runner for a specific repository.
	ScopeRepo RegistrationScope = "repo"
	// ScopeOrg registers the runner at the organization level.
	ScopeOrg RegistrationScope = "org"
	// ScopeGlobal registers the runner at the global / instance level.
	ScopeGlobal RegistrationScope = "global"
)

// ScalingMode declares how the supervisor scales runners for the provider.
type ScalingMode string

const (
	// ScalingWebhook indicates event-driven scaling via workflow_job webhooks (GitHub, Gitea).
	ScalingWebhook ScalingMode = "webhook"
	// ScalingPolling indicates polling-driven scaling via API queries (Forgejo).
	ScalingPolling ScalingMode = "polling"
)

// AuthMethod represents the supported authentication methods for auth profiles.
type AuthMethod string

const (
	// AuthMethodGitHubApp authenticates using a GitHub App (AppID + Private Key).
	AuthMethodGitHubApp AuthMethod = "github_app"
	// AuthMethodGiteaToken authenticates to Gitea via Personal Access Token.
	AuthMethodGiteaToken AuthMethod = "gitea_token"
	// AuthMethodForgejoToken authenticates to Forgejo via Personal Access Token.
	AuthMethodForgejoToken AuthMethod = "forgejo_token"
	// AuthMethodPAT authenticates using a generic Personal Access Token (e.g., GitHub PAT fallback).
	AuthMethodPAT AuthMethod = "pat"
)

// DiscoveredTarget represents an organization or repository discovered from a Git provider.
type DiscoveredTarget struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	HTMLURL     string `json:"html_url"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"is_private"`
	AvatarURL   string `json:"avatar_url"`
}

// GitProvider is the unified interface decoupling the supervisor from VCS APIs (docs/02 §3.2).
type GitProvider interface {
	// GetRegistrationToken retrieves a short-lived runner registration token for the target URL and scope.
	GetRegistrationToken(ctx context.Context, scope RegistrationScope, targetURL string) (string, error)

	// ValidateCredentials checks whether configured credentials are valid against the remote VCS API.
	ValidateCredentials(ctx context.Context) error

	// ScalingMode returns whether the provider scales via webhooks or polling.
	ScalingMode() ScalingMode

	// PollQueuedJobs queries the forge's API for queued jobs (used when ScalingMode() == ScalingPolling).
	PollQueuedJobs(ctx context.Context, targetURL string) (int, error)

	// DiscoverOrganizations discovers accessible organizations from the provider.
	DiscoverOrganizations(ctx context.Context) ([]DiscoveredTarget, error)

	// DiscoverRepositories discovers accessible repositories from the provider.
	DiscoverRepositories(ctx context.Context) ([]DiscoveredTarget, error)
}

// RenovateTokenProvider is optionally implemented by GitProviders that supply tokens for Renovate bot tasks.
type RenovateTokenProvider interface {
	// GetRenovateToken retrieves a short-lived token suitable for Renovate bot operations on the target URL.
	GetRenovateToken(ctx context.Context, targetURL string) (string, error)
}

// RunnerDeregistrar is optionally implemented by GitProviders that support API-driven runner deregistration (docs/03 §7).
type RunnerDeregistrar interface {
	// DeregisterRunner removes a registered runner from the Git provider via its API to prevent ghost runners.
	DeregisterRunner(ctx context.Context, scope RegistrationScope, targetURL, runnerName string) error
}
