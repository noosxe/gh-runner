// Package github implements the GitProvider interface for GitHub:
// GitHub App authentication (private key → RS256 JWT → installation token → runner
// registration token) and PAT fallback. Introduced in RUN-25 per docs/02 §3.2 and docs/05 §2.
//
// # Required GitHub App Permissions & Scopes
//
// When configuring a GitHub App for the supervisor, the following permissions are required:
//
// Runner Provisioning:
//   - Administration: read (Required to create runner registration tokens for repository/organization pools)
//   - Metadata: read (Required for repository metadata discovery)
//
// Renovate Bot Tasks (optional, if Renovate is enabled for the pool):
//   - Contents: write (To create and update branches and commits)
//   - Pull requests: write (To open and manage dependency update PRs)
//   - Workflows: write (To update GitHub Actions workflow files when upgrading actions)
package github
