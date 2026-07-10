# Open Questions & Clarifications

Before proceeding with the implementation of the AIO Supervisor, the following architectural and product questions need clarification from the product/engineering team:

## 1. Authentication & Control Plane SSO
- **Gitea SSO**: The requirements mention GitHub OAuth and GitHub App integration for logging into the supervisor's web dashboard. If a user only intends to manage Gitea runners, will the web UI support Gitea OAuth for SSO? How should multi-tenant authorization be handled for Gitea-only deployments?
- **Local Admin Account**: Is there a need for a local "admin" fallback account (e.g., using basic auth) if the external OAuth provider is unreachable or misconfigured?

## 2. Gitea Webhooks & Scale-to-Zero
- Phase 3 proposes Webhook integrations (`workflow_job.queued`) for real-time scaling and scale-to-zero for GitHub. Does Gitea provide an equivalent reliable webhook payload for job queuing that we can ingest? If not, do we rely strictly on the periodic auditor loop for Gitea pools?

## 3. Database Selection & Embedding
- The architecture mentions `SQLite / PG driver`. For an "All-In-One" containerized deployment with zero external dependencies, is SQLite the intended default embedded database? If so, we need to ensure the database file is placed in a persistent volume (e.g., `/data/supervisor.db`) to survive supervisor container restarts.

## 4. Container Engine Compatibility (Podman)
- The documentation frequently references the Docker socket (`/var/run/docker.sock`) and Docker SDK. Should the supervisor explicitly support Podman (which has Docker-compatible socket APIs) or contain workarounds for Podman-specific behaviors (e.g., rootless podman networking and UID mapping)?

## 5. Docker-in-Docker (DooD) for Gitea `act_runner`
- Gitea's `act_runner` natively spawns sibling Docker containers to execute workflow steps. If the `allow_docker` config is set to `false`, `act_runner` will likely fail to execute standard jobs. Should we fail-fast in the UI/API when a user configures a Gitea pool without `allow_docker: true`, or is there a rootless/containerless execution mode for Gitea actions that we intend to support?

## 6. Runner Version Management
- The GitHub Runner and Gitea `act_runner` binaries are currently downloaded during the Docker build stage. How do we plan to handle binary updates? Will users need to pull a new version of our AIO image, or should the supervisor orchestrate binary updates dynamically?
