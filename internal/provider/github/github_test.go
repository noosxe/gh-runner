package github_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/noosxe/gh-runner/internal/db"
	"github.com/noosxe/gh-runner/internal/provider"
	"github.com/noosxe/gh-runner/internal/provider/github"
)

func setupMockGitHubServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// 1. /app endpoint
	mux.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":12345,"name":"test-runner-app"}`))
	})

	// 2. /user endpoint (PAT)
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-pat-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"test-user"}`))
	})

	// 3. /repos/{owner}/{repo}/installation
	mux.HandleFunc("/repos/my-org/my-repo/installation", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7777}`))
	})

	// 4. /orgs/{owner}/installation
	mux.HandleFunc("/orgs/my-org/installation", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7777}`))
	})

	// 5. /app/installations/{id}/access_tokens
	mux.HandleFunc("/app/installations/7777/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		resp := map[string]any{
			"token":      "ghs_installation_access_token_xyz",
			"expires_at": time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	// 6. /repos/{owner}/{repo}/actions/runners/registration-token
	mux.HandleFunc("/repos/my-org/my-repo/actions/runners/registration-token", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer ghs_installation_access_token_xyz" && auth != "Bearer valid-pat-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Unauthorized runner token access"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"gh_repo_runner_token_abc123","expires_at":"2030-01-01T00:00:00Z"}`))
	})

	// 7. /orgs/{owner}/actions/runners/registration-token
	mux.HandleFunc("/orgs/my-org/actions/runners/registration-token", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer ghs_installation_access_token_xyz" && auth != "Bearer valid-pat-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Unauthorized runner token access"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"gh_org_runner_token_xyz789","expires_at":"2030-01-01T00:00:00Z"}`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(func() { server.Close() })
	return server
}

func TestGitHubAppProvider(t *testing.T) {
	pkcs1PEM, _, _ := generateTestKeyPEMs(t)
	server := setupMockGitHubServer(t)
	ctx := context.Background()

	client, err := github.NewAppProvider(12345, pkcs1PEM, github.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewAppProvider failed: %v", err)
	}

	// 1. Credentials validation
	if err := client.ValidateCredentials(ctx); err != nil {
		t.Fatalf("ValidateCredentials failed: %v", err)
	}

	// 2. Scaling mode & queued jobs
	if client.ScalingMode() != provider.ScalingWebhook {
		t.Errorf("expected scaling mode webhook, got %v", client.ScalingMode())
	}
	if queued, err := client.PollQueuedJobs(ctx, "https://github.com/my-org/my-repo"); err != nil || queued != 0 {
		t.Errorf("expected 0 queued jobs, got %d, err: %v", queued, err)
	}

	// 3. Repo-scoped registration token
	tok, err := client.GetRegistrationToken(ctx, provider.ScopeRepo, "https://github.com/my-org/my-repo.git")
	if err != nil {
		t.Fatalf("GetRegistrationToken (repo) failed: %v", err)
	}
	if tok != "gh_repo_runner_token_abc123" {
		t.Errorf("unexpected repo token: %q", tok)
	}

	// 4. Org-scoped registration token
	orgTok, err := client.GetRegistrationToken(ctx, provider.ScopeOrg, "https://github.com/my-org")
	if err != nil {
		t.Fatalf("GetRegistrationToken (org) failed: %v", err)
	}
	if orgTok != "gh_org_runner_token_xyz789" {
		t.Errorf("unexpected org token: %q", orgTok)
	}

	// 5. Renovate token retrieval
	renovateTok, err := client.GetRenovateToken(ctx, "https://github.com/my-org/my-repo")
	if err != nil {
		t.Fatalf("GetRenovateToken failed: %v", err)
	}
	if renovateTok != "ghs_installation_access_token_xyz" {
		t.Errorf("unexpected renovate token: %q", renovateTok)
	}
}

func TestGitHubPATProvider(t *testing.T) {
	server := setupMockGitHubServer(t)
	ctx := context.Background()

	client, err := github.NewPATProvider("valid-pat-secret", github.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewPATProvider failed: %v", err)
	}

	// 1. Credentials validation
	if err := client.ValidateCredentials(ctx); err != nil {
		t.Fatalf("ValidateCredentials failed: %v", err)
	}

	// 2. Repo-scoped registration token
	tok, err := client.GetRegistrationToken(ctx, provider.ScopeRepo, "https://github.com/my-org/my-repo")
	if err != nil {
		t.Fatalf("GetRegistrationToken (repo) failed: %v", err)
	}
	if tok != "gh_repo_runner_token_abc123" {
		t.Errorf("unexpected repo token: %q", tok)
	}

	// 3. Renovate token (returns PAT)
	renovateTok, err := client.GetRenovateToken(ctx, "https://github.com/my-org/my-repo")
	if err != nil {
		t.Fatalf("GetRenovateToken failed: %v", err)
	}
	if renovateTok != "valid-pat-secret" {
		t.Errorf("unexpected renovate token: %q", renovateTok)
	}

	// 4. Invalid PAT
	badClient, err := github.NewPATProvider("invalid-pat", github.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewPATProvider failed: %v", err)
	}
	if err := badClient.ValidateCredentials(ctx); err == nil {
		t.Fatal("expected error with bad PAT")
	}
}

func TestRegistryIntegration(t *testing.T) {
	pkcs1PEM, _, _ := generateTestKeyPEMs(t)
	ctx := context.Background()

	// Test building AuthMethodGitHubApp via DefaultRegistry
	p1, err := provider.DefaultRegistry.Build(ctx, db.DecryptedAuthProfile{
		AuthProfile: db.AuthProfile{
			AuthMethod: string(provider.AuthMethodGitHubApp),
			AppID:      sql.NullInt64{Int64: 9876, Valid: true},
		},
		PrivateKey: pkcs1PEM,
	})
	if err != nil {
		t.Fatalf("failed to build GitHub App provider from registry: %v", err)
	}
	if p1.ScalingMode() != provider.ScalingWebhook {
		t.Errorf("expected webhook scaling mode")
	}

	// Test building AuthMethodPAT via DefaultRegistry
	p2, err := provider.DefaultRegistry.Build(ctx, db.DecryptedAuthProfile{
		AuthProfile: db.AuthProfile{
			AuthMethod: string(provider.AuthMethodPAT),
		},
		Token: "ghp_some_valid_pat",
	})
	if err != nil {
		t.Fatalf("failed to build GitHub PAT provider from registry: %v", err)
	}
	if p2.ScalingMode() != provider.ScalingWebhook {
		t.Errorf("expected webhook scaling mode")
	}
}
