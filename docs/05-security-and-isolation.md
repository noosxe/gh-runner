# Security & Isolation Guardrails

GitHub and Gitea runners execute untrusted workflow code. Security is our absolute priority. The supervisor and dynamic runners incorporate strict isolation features to minimize the attack surface.

## 1. Non-Root Context

Runner containers spawned by the supervisor must execute jobs under a dedicated low-privilege system user (e.g., `runner` UID `1001`), as defined in the base `Dockerfile`. 
The `act_runner` process (for Gitea) and the `run.sh` process (for GitHub) must not be executed as `root`.

## 2. Credential Segregation & Token Generation

Since runner registration tokens expire quickly (typically 1 hour), a long-running supervisor cannot rely on static tokens. It must authenticate dynamically:

- **GitHub App / PAT**: The supervisor loads the private key or PAT. It requests an installation token, and then requests a fresh **Runner Registration Token** via the GitHub API.
- **Gitea PAT**: The supervisor calls Gitea's actions runner token API to retrieve a fresh Runner Registration Token.

**Strict Segregation**: Registration tokens are generated on-the-fly and passed strictly via environment variables to individual ephemeral containers. The master credentials (private keys, supervisor PATs) are **never** shared with or mounted into the ephemeral runner containers.

## 3. Resource Quotas & Saturation

Every runner pool allows specifying CPU and memory boundaries (e.g., `cpus: "2.0"`, `memory: "4g"`) directly in its configuration. The supervisor passes these constraints to the container engine, preventing a single rogue workflow from consuming all host system resources and causing a denial of service. 

Additionally, the `Total Allowed Runners` global limit acts as a circuit breaker, queuing internal provisioning if the host reaches maximum capacity.

## 4. Docker Socket Isolation (DooD Safety)

By default, the host Docker socket (`/var/run/docker.sock`) is **not** mounted into runner containers.
- If a repository strictly requires building Docker images or running containerized actions, the `allow_docker: true` flag must be explicitly set for that pool.
- Gitea's `act_runner` inherently relies on Docker-in-Docker (DooD) to run workflows. Configuring `allow_docker: true` is generally required for Gitea, but it exposes the host socket to the runner. Sibling container privileges must be carefully considered for untrusted repositories.

## 5. Configuration & Database Security

- **Encryption at Rest**: Sensitive credentials stored in the local SQLite/PG database must be encrypted at rest (e.g., using AES-256 with an encryption key provided via a supervisor environment variable).
- **Export/Import Sanitization**: When exporting configurations via YAML for GitOps workflows, raw credentials must be sanitized or redacted. The export file uses placeholders or reference keys to prevent accidental credential leakage into version control.
