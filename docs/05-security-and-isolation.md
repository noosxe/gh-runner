# Security & Isolation Guardrails

GitHub, Gitea, and Forgejo runners execute untrusted workflow code. Security is our absolute priority. The supervisor and dynamic runners incorporate strict isolation features to minimize the attack surface.

## 1. Non-Root Context

Runner containers spawned by the supervisor must execute jobs under a dedicated low-privilege system user (e.g., `runner` UID `1001`), as defined in the base `Dockerfile`. 
The `act_runner` process (for Gitea), `forgejo-runner` process (for Forgejo), and the `run.sh` process (for GitHub) must not be executed as `root`.

## 2. Credential Segregation & Token Generation

Since runner registration tokens expire quickly (typically 1 hour), a long-running supervisor cannot rely on static tokens. It must authenticate dynamically:

- **GitHub App / PAT**: The supervisor loads the private key or PAT. It requests an installation token, and then requests a fresh **Runner Registration Token** via the GitHub API. If spawning a Renovate task container, it simply passes the short-lived installation token itself.
- **Gitea / Forgejo PAT**: The supervisor calls Gitea's / Forgejo's actions runner token API to retrieve a fresh Runner Registration Token.

**Strict Segregation**: Tokens are generated on-the-fly and passed strictly via environment variables (`RUNNER_TOKEN` or `RENOVATE_TOKEN`) to individual ephemeral containers. The master credentials (private keys, supervisor PATs) are **never** shared with or mounted into the ephemeral containers.

### GitHub App Scopes
To support the AIO Supervisor *and* Renovate Bot, the GitHub App must be provisioned with the following permissions:
- **Runner Registration**: `Administration: read`, `Metadata: read`
- **Renovate Execution**: `Contents: write`, `Pull requests: write`, `Workflows: write` (if updating actions)

## 3. Resource Quotas & Saturation

Every runner pool allows specifying CPU and memory boundaries (e.g., `cpus: "2.0"`, `memory: "4g"`) directly in its configuration. The supervisor passes these constraints to the container engine, preventing a single rogue workflow from consuming all host system resources and causing a denial of service. 

Additionally, the `Total Allowed Runners` global limit acts as a circuit breaker, queuing internal provisioning if the host reaches maximum capacity.

## 4. Docker Socket Isolation (DooD Safety)

By default, the host Docker socket (`/var/run/docker.sock`) is **not** mounted into runner containers.
- If a repository strictly requires building Docker images or running containerized actions, the `allow_docker: true` flag must be explicitly set for that pool.
- Gitea's `act_runner` and Forgejo's `forgejo-runner` inherently rely on Docker-in-Docker (DooD) to run workflows. Configuring `allow_docker: true` is strictly required for Gitea and Forgejo. The Web UI enforces this dependency and will prevent users from saving a Gitea or Forgejo pool configuration without Docker access enabled. However, because this exposes the host socket to the runner, sibling container privileges must be carefully considered for untrusted repositories.

> **Current Scope & Trust Boundary**: Gitea and Forgejo integrations are designed for **private/internal** instances only. The mandatory `allow_docker: true` requirement for these providers is an accepted trade-off within this trust boundary — untrusted third-party workflow code is not expected to run on these pools. Public GitHub is the only provider expected to handle untrusted workflow code, and Docker socket access remains opt-in for GitHub pools.
>
> **Future Mitigation** *(deferred)*: Rootless Docker, Podman support, and Sysbox runtime integration are tracked as future enhancements to reduce socket exposure for all providers.

## 5. Configuration & Database Security

- **Encryption at Rest & Secret Derivation**: Sensitive credentials stored in the local SQLite database must be encrypted at rest using AES-256. The AES key and the JWT signing secret are both deterministically derived from the single required `SUPERVISOR_DB_ENCRYPTION_KEY` master key via HKDF-SHA256, each under its own context label, so the two secrets are cryptographically independent while operators manage exactly one secret (implemented in `internal/keys`, RUN-9).
- **Local Administrator Hashing**: The initial local administrator password must be securely hashed (using algorithms like bcrypt or argon2) prior to storage in the SQLite database to protect against offline attacks.
- **Export/Import Sanitization**: When exporting configurations via YAML for GitOps workflows, raw credentials must be sanitized or redacted. The export file uses placeholders or reference keys to prevent accidental credential leakage into version control.
- **Session Tokens**: Admin session tokens are JWTs transported exclusively via `HttpOnly` secure cookies with `SameSite=Strict` policy. Raw tokens are never exposed to client-side JavaScript, mitigating XSS-based token theft. Session state is tracked in the database for audit and forced revocation capabilities.
