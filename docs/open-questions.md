# Open Questions & Clarifications

Before proceeding with the implementation of the AIO Supervisor, the following architectural and product questions need clarification from the product/engineering team:

## 1. Gitea & Forgejo Webhooks & Scale-to-Zero
- Phase 3 proposes Webhook integrations (`workflow_job.queued`) for real-time scaling and scale-to-zero for GitHub. Does Gitea or Forgejo provide an equivalent reliable webhook payload for job queuing that we can ingest? If not, do we rely strictly on the periodic auditor loop for Gitea/Forgejo pools?

## 2. YAML vs. Database Reconciliation
- We mention both YAML import/export and the SQLite DB. We need to define the exact source of truth on boot. If a user mounts a static `config.yml` to the container, does it overwrite the SQLite state? Does it run in a "read-only GitOps mode" where the UI disables editing?

## 3. Environment Variable Specification
- We need to define the formal list of environment variables the Supervisor container itself requires or supports to boot (e.g., `PORT`, `DB_PATH`, `DB_ENCRYPTION_KEY`, `LOG_LEVEL`).

## 4. Data Retention & Pruning Strategy
- We mention a 30-day retention window in the requirements, but we haven't defined the background cron worker or specific query responsible for purging old `job_history` rows and their associated log files to prevent disk exhaustion.

## 5. Error Handling & Rate Limiting
- How should the supervisor handle GitHub/Gitea/Forgejo API rate limits (e.g., backoff strategies) or Docker daemon unreachability without crashing the process or spamming logs?

---

## Authentication & Session Management

## 6. Session Token Mechanism
- `AuthService.Login` returns a `token` and `GetSession` validates it, but the docs never specify: JWT vs opaque session tokens? Token expiry duration and refresh strategy? Cookie-based vs `Authorization` header transport? How does ConnectRPC binary mode carry authentication context per-request (interceptors, metadata)?

## 7. Multi-Admin & RBAC Model
- The `admin_users` table supports multiple rows and `GetSessionResponse` includes `is_admin`, implying multiple roles exist. Do we support multiple admin accounts? Are there non-admin viewer roles? If yes, what permissions does each role have? If not, should we simplify the schema and proto to remove `is_admin`?

## 8. Password Reset / Recovery Flow
- There's no external auth (no email, no OAuth for the admin UI). If the local administrator forgets their password, what's the recovery path? Options: CLI reset command (`supervisor reset-password`), direct DB manipulation, or environment-variable-based recovery key?

---

## Database & Schema

## 9. Missing `scope` Column on `runner_pools`
- The `GitProvider` interface defines `RegistrationScope` with values `repo`, `org`, and `global`, but the `runner_pools` table has no `scope` column. Org-level and global-level runner pools cannot be persisted or distinguished. Should we add a `scope TEXT NOT NULL CHECK(scope IN ('repo', 'org', 'global'))` column?

## 10. Global Settings Keys & Defaults
- The `app_settings` table is a generic key/value store. The product requirements define `Total Allowed Runners` and `Total Idle Warm Pool` as global settings (Onboarding Step 3). We need to formalize: what are all expected keys, their value types, validation constraints, and default values? Should this be documented in the schema migration as seed data?

## 11. Session / Token Storage Table
- If admin login produces session tokens, there's no `sessions` table in the schema for tracking active sessions, token expiry, or revocation. Is session state managed in-memory only (lost on restart)? If so, is that acceptable for production use?

## 12. Audit Log Table
- For a security-sensitive system managing runner infrastructure and storing encrypted credentials, should we add an `audit_log` table to track admin actions (pool CRUD, credential changes, manual runner terminations, login attempts)? This has compliance and debugging implications.

---

## RPC Protocol Gaps

## 13. Auth Profile Management RPCs
- The `auth_profiles` table stores GitHub App, Gitea PAT, and Forgejo PAT credentials, and the Onboarding Step 2 requires creating/selecting them. However, `08-rpc-protocols.md` defines no CRUD service for auth profiles (e.g., `AuthProfileService` with `Create`, `List`, `Update`, `Delete`). These RPCs need to be defined.

## 14. Streaming Log RPC
- Product requirements specify "Streaming Logs: Live logs of individual active runners." ConnectRPC supports `server-streaming` RPCs. We need to define an endpoint like `StreamRunnerLogs(StreamRunnerLogsRequest) returns (stream LogChunk)`. This also requires deciding the log capture mechanism (Docker log driver API, volume mounts, etc. — see Question 20).

## 15. Onboarding State RPC
- The 5-step setup wizard needs backend state to determine whether setup is complete (show wizard vs. dashboard). Should there be a `GetOnboardingStatus` RPC? What state does it check — presence of an admin user, at least one auth profile, at least one pool?

## 16. Image Update Management RPCs
- Docs 01 and 03 describe periodic image update checks and admin notifications. We need RPCs to: list available image updates, trigger a manual pull, configure the automatic update schedule, and dismiss update notifications.

## 17. Missing Resource Fields in Pool Proto
- The `Pool` proto message is missing `cpu_limit`, `memory_limit`, `max_runner_lifetime_seconds`, and `auth_profile_id` fields, even though these exist in the DB schema and YAML config. The frontend cannot configure resource limits without these fields.

## 18. Renovate Trigger & Status RPCs
- Renovate has a DB table and cron scheduling, but no RPCs to manually trigger a Renovate run, view the last run status, or check upcoming scheduled runs. Should we add these to a `RenovateService` or extend `PoolService`?

---

## Architecture & Operations

## 19. Supervisor Health Check Endpoints
- No document defines HTTP health/readiness endpoints (e.g., `GET /healthz`, `GET /readyz`) for the supervisor container. These are essential for Docker Compose `healthcheck` directives, load balancer integrations, and production monitoring. What should each check validate (DB connectivity, Docker socket reachability, active control loop)?

## 20. Log Capture Mechanism for Runner Containers
- The dashboard promises streaming logs and the schema has `log_retention_path`, but the docs never define *how* runner container stdout/stderr is captured by the supervisor. Options: Docker container logs API (`docker logs`), volume-mounted log files, Docker logging driver configuration? This decision affects the streaming RPC design, retention, and disk usage.

## 21. Backup & Disaster Recovery
- The SQLite database holds all critical state (encrypted credentials, pool configs, job history). No doc covers: recommended volume mount for the DB file, backup strategies (periodic SQLite `.backup` command?), or recovery procedures. What happens if the DB is corrupted?

## 22. Network Topology & DNS
- The architecture shows a compose stack but doesn't specify: Docker network mode (bridge/host/custom), DNS resolution between supervisor and ephemeral runners, or how the supervisor discovers the container engine endpoint if it's not `/var/run/docker.sock` (e.g., remote Docker host, TCP socket).

## 23. Container Naming Strategy
- The lifecycle doc defines container labels for tracking but doesn't specify the naming pattern for spawned containers (e.g., `ghrs-<pool>-<short-hash>`). Clear naming is important for log readability, manual debugging (`docker ps`), and preventing name collisions.

## 24. Graceful Shutdown Timeout
- The shutdown sequence diagram shows the supervisor waiting for active runners to finish, but doesn't define: what's the maximum wait duration? What happens if a runner is stuck and never exits? Is there a configurable `shutdown_timeout_seconds` with a force-kill fallback?

---

## Security

## 25. Web UI TLS / HTTPS Strategy
- The security doc covers credential encryption and container hardening but doesn't address transport security for the Web UI. Does the supervisor terminate TLS itself (self-signed cert generation)? Is a reverse proxy (nginx, Caddy, Traefik) expected in front? Should the docs recommend a deployment pattern?

## 26. CORS & HTTP Security Headers
- The SPA frontend is served by the Go backend. No document defines CORS policy, Content Security Policy (CSP), or other HTTP security headers (`X-Frame-Options`, `Strict-Transport-Security`). These are standard hardening measures for web applications.

## 27. Supervisor Docker Socket Permissions
- The security doc covers Docker socket isolation for *runner* containers (`allow_docker` flag), but doesn't detail the supervisor's own socket access model. What user/group does the supervisor process run as? Is the socket mount read-write (it must be to spawn containers)? What mitigations apply to the supervisor's elevated host access?

---

## Testing Strategy

## 28. Testing Framework & Strategy
- `AGENTS.md` references `tests/integration/` and `tests/unit/` directories, but no design doc covers: Go test framework conventions (`testing` + `testify`?), mocking strategy for Docker Engine SDK and GitHub/Gitea/Forgejo API clients, shell script testing (BATS?), frontend testing (Vitest + React Testing Library?), E2E/integration approach, or CI pipeline design for the supervisor.

---

## Frontend / UI Design

## 29. UI/UX Wireframes & Component Spec
- Product requirements describe rich dashboards, a multi-step wizard, analytics graphs, and streaming logs, but there are no wireframes, component hierarchies, or page-level design specs. Should a design doc (e.g., `09-frontend-design.md`) be created before implementation to align on layout, navigation, and interaction patterns?

## 30. Real-time Dashboard Refresh Strategy
- The dashboard displays live pool states, active runner counts, CPU/memory stats, and streaming logs. No doc specifies the data refresh mechanism: polling with TanStack Query `refetchInterval`? WebSocket/SSE push? ConnectRPC server streaming? What's the acceptable latency for pool state updates?
