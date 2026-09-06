# Custom Self-Hosted GitHub Actions Runner & Supervisor

[![Go CI](https://github.com/noosxe/gh-runner/actions/workflows/go.yml/badge.svg)](https://github.com/noosxe/gh-runner/actions/workflows/go.yml)
[![Web CI](https://github.com/noosxe/gh-runner/actions/workflows/web.yml/badge.svg)](https://github.com/noosxe/gh-runner/actions/workflows/web.yml)
[![Lint](https://github.com/noosxe/gh-runner/actions/workflows/lint.yml/badge.svg)](https://github.com/noosxe/gh-runner/actions/workflows/lint.yml)
[![Runner Multi-Arch Build](https://github.com/noosxe/gh-runner/actions/workflows/build.yml/badge.svg)](https://github.com/noosxe/gh-runner/actions/workflows/build.yml)
[![Supervisor Multi-Arch Build](https://github.com/noosxe/gh-runner/actions/workflows/supervisor-build.yml/badge.svg)](https://github.com/noosxe/gh-runner/actions/workflows/supervisor-build.yml)

A lightweight, secure, and self-contained self-hosted runner and orchestrator stack for **GitHub Actions**, **Gitea Actions**, and **Forgejo Actions**. Packaged as production-ready, multi-architecture Docker containers designed to run natively on **ARM64** (Apple Silicon, AWS Graviton, Raspberry Pi) and **AMD64** hosts without emulation overhead.

---

## ✨ Features

- **All-in-One Multi-Provider Supervisor:** Database-driven daemon that automatically provisions, monitors, scales, and maintains dynamic pools of ephemeral runner containers across GitHub, Gitea, and Forgejo repositories.
- **Embedded Web UI & 5-Step Onboarding Wizard:** Single-Page Application (React 19, TypeScript, TanStack Router & Query, TailwindCSS) embedded directly into the Go supervisor binary via `go:embed`. Features a zero-config 5-step onboarding wizard, dark/light theme switching, and live pool management.
- **Dynamic Ephemeral Scaling:** Automatically manages warm standby containers (`min_idle_runners`) ready for immediate job dispatch, auto-scales up to concurrency limits (`max_concurrency`), and aggressively prunes completed or failed containers within seconds.
- **Real-Time Streaming Logs & Interactive Terminal:** Unbuffered ConnectRPC server-sent streaming (`StreamRunnerLogs`, `StreamSystemMetrics`) pushing real-time container output directly to an embedded xterm.js terminal emulator with auto-scroll and quick-copy.
- **Dependency Automation via Renovate:** Built-in scheduled Renovate task runner for autonomous dependency updates, configured via cron expressions with isolated ephemeral container execution.
- **Single Master Key, Derived Secrets:** The single required `SUPERVISOR_DB_ENCRYPTION_KEY` expands via HKDF-SHA256 into two distinct, deterministic secrets — an AES-256 database encryption key for credentials at rest and a HMAC secret for JWT session tokens.
- **Embedded SQLite & Auto-Migrations:** Pure-Go SQLite persistence via `modernc.org/sqlite` (strictly CGO-free) with automated Goose migrations on boot, rolling snapshot backups (`SUPERVISOR_BACKUP_INTERVAL_HOURS`), and strict corruption detection.
- **Unified Multi-Provider Runner Image (`runner-aio`):** Multi-stage container image bundling GitHub Actions runner, Gitea `act_runner`, and Forgejo `forgejo-runner` with automatic provider detection, non-root user execution (`UID 1001`), and active signal traps for clean deregistration.
- **Automated Health Probes:** Serves `GET /healthz` (liveness: process and SQLite accessible) and `GET /readyz` (readiness: database, audit loop, and Docker daemon reachability; reports `degraded` during Docker outages while remaining responsive).
- **Reverse-Proxy TLS & Hardened Cookies:** Designed for TLS termination via external reverse proxies (Caddy, Traefik, Nginx) with unbuffered HTTP/2 streaming support and configurable `SUPERVISOR_SECURE_COOKIE` enforcing `Secure; HttpOnly; SameSite=Strict` attributes.
- **Comprehensive Automated Test Suites:** Extensive test coverage across Go unit, race detection (`go test -race`), and testify test suites (`mockery`-backed Docker and GitProvider clients), runner script test harnesses, and 20 frontend Vitest test suites.

---

## 🚀 Quickstart: Supervisor Stack (Recommended)

Deploy the complete supervisor daemon and web control interface using Docker Compose:

### 1. Configure the Environment
Clone this repository (or download `docker-compose.yml` and `.env.example`):

```bash
git clone https://github.com/noosxe/gh-runner.git
cd gh-runner
cp .env.example .env
```

Generate a 256-bit encryption key (minimum 32 bytes) and set it in your `.env` file:
```bash
# Generate encryption key
openssl rand -base64 32
```
Add the output to `.env`:
```env
SUPERVISOR_DB_ENCRYPTION_KEY=your_generated_base64_key_here
SUPERVISOR_PORT=8090
```

### 2. Launch the Supervisor Stack
Start the containerized supervisor daemon:
```bash
docker compose up -d
```

Verify that the supervisor is healthy:
```bash
docker compose ps
curl -s http://localhost:8090/healthz
# Expected output: {"status":"healthy","checks":{"db":"ok"}}
```

### 3. Complete the Onboarding Wizard
Navigate to `http://localhost:8090` in your browser. The system will automatically direct you to the 5-step onboarding wizard:
1. **Create Master Administrator:** Set administrative credentials for the dashboard.
2. **Connect Git Provider:** Connect GitHub (GitHub App or PAT), Gitea (PAT), or Forgejo (PAT).
3. **Global Scaling Safeguards:** Establish total allowed runner limits, idle warm pool quotas, and history retention.
4. **Initial Runner Pool Setup:** Configure your repository URL, warm standby targets, concurrency limits, and optional Renovate automation.
5. **Review & Launch:** Review configuration and launch your supervisor!

---

## 📦 Standalone Runner Deployment (Optional)

If you only need a single static runner for a specific GitHub repository without the supervisor daemon or dynamic autoscaling:

```bash
# Set runner credentials in .env
GITHUB_REPOSITORY_URL=https://github.com/owner/repo
RUNNER_TOKEN=your_short_lived_registration_token

# Spin up standalone runner using the runner-standalone compose profile
docker compose --profile runner-standalone up -d runner
```

To monitor runner registration logs:
```bash
docker compose logs -f runner
```

---

## ⚙️ Configuration & Environment Variables

### Supervisor Daemon (`gh-runner-supervisor`)

The supervisor daemon layers configuration in increasing precedence: **built-in defaults → configuration file (`--config`) → environment variables → CLI flags**.

| Variable | Type | Required | Default | Description |
| :--- | :---: | :---: | :--- | :--- |
| `SUPERVISOR_DB_ENCRYPTION_KEY` | String | **Yes** | — | Master key (min 32 bytes) used for AES-256 database encryption and HKDF JWT secret derivation. |
| `SUPERVISOR_PORT` | Int | No | `8090` | HTTP port for the ConnectRPC API and embedded web control interface. |
| `SUPERVISOR_DATA_DIR` | String | No | `/data` | Data directory for SQLite database, snapshot backups, and gzipped execution logs. |
| `SUPERVISOR_DB_PATH` | String | No | `/data/supervisor.db` | Explicit file path for the SQLite database. |
| `SUPERVISOR_LOG_LEVEL` | String | No | `info` | Logging verbosity: `debug`, `info`, `warn`, `error`. Logs format as structured JSON. |
| `SUPERVISOR_DOCKER_HOST` | String | No | `unix:///var/run/docker.sock` | Docker daemon endpoint for orchestrating runner containers. |
| `SUPERVISOR_BACKUP_INTERVAL_HOURS` | Int | No | `6` | Frequency of automated rolling SQLite snapshot backups in hours. |
| `SUPERVISOR_BACKUP_RETENTION_COUNT` | Int | No | `7` | Number of automated SQLite backups retained before pruning. |
| `SUPERVISOR_CONFIG` | String | No | — | Path to an optional YAML or TOML configuration file. |
| `SUPERVISOR_SECURE_COOKIE` | Bool | No | `false` | Enables the `Secure` attribute on auth cookies. Set to `true` when behind HTTPS. |

### Standalone Runner Container (`runner-aio`)

| Variable | Type | Required | Default | Description |
| :--- | :---: | :---: | :--- | :--- |
| `GITHUB_REPOSITORY_URL` | String | **Yes** | — | Target repository URL (e.g., `https://github.com/owner/repo`). |
| `RUNNER_TOKEN` | String | **Yes** | — | Temporary runner registration token from repository settings (1 hour expiry). |
| `RUNNER_NAME` | String | No | *container-id* | Runner display name in the provider dashboard. |
| `RUNNER_LABELS` | String | No | `self-hosted,linux,arm64` | Comma-separated list of runner labels for workflow targeting. |
| `RUNNER_WORKDIR` | String | No | `_work` | Workspace directory path inside the runner container. |

---

## 🔒 Security Architecture & Hardening

### Supervisor Docker Socket Access (Accepted Risk Callout)

> [!CAUTION]
> **Elevated Docker Socket Privileges:**
> The `supervisor` container executes as `root` inside the container and mounts the host Docker socket (`/var/run/docker.sock`) read-write. This elevated access is **strictly required by design** so that the supervisor daemon can communicate with the Docker Engine SDK to dynamically create, inspect, attach logs to, and destroy ephemeral runner containers on the host.

**Operational Hardening Recommendations:**
- **Host Isolation:** Deploy the supervisor on a dedicated virtual machine or host instance isolated from shared production application workloads.
- **Network Boundaries:** Do not expose the supervisor port (`8090`) directly to the public internet without an authenticating reverse proxy and firewall rules.
- **Docker Socket Proxies:** In high-compliance environments, front the Docker socket with a capability-filtering socket proxy restricting container creation parameters.

### Credential & Token Segregation
- **Zero Master Token Leakage:** Master credentials (GitHub App private keys, provider PATs) are encrypted at rest with AES-256 and are **never** mounted, passed as environment variables, or serialized into spawned runner containers.
- **Ephemeral Job Tokens:** Spawner routines fetch short-lived, single-use runner registration tokens from Git provider APIs immediately before container creation.

### Non-Root Runner Execution
- Ephemeral runner containers run under a dedicated, low-privilege system user (`runner`, `UID 1001`, `GID 1001`). Workloads inside the runner cannot write to host filesystem paths outside their designated volume bounds.

### Reverse-Proxy & TLS Hardening
The supervisor daemon intentionally serves unencrypted HTTP on loopback/internal networks, delegating TLS termination to reverse proxies (such as Caddy, Traefik, or Nginx).

- For complete reverse proxy configurations, HTTP/2 setup, and unbuffered ConnectRPC streaming directives, consult the comprehensive guide: **[docs/10-reverse-proxy-tls.md](docs/10-reverse-proxy-tls.md)**.
- When running behind TLS, ensure `SUPERVISOR_SECURE_COOKIE=true` is set to protect session tokens against plaintext interception.

---

## 🛠️ CI/CD Pipelines

Automated GitHub Actions workflows ensure continuous verification and multi-architecture publishing:

- **Go CI (`go.yml`):** Runs `go build`, `go vet`, unit tests, and the Go data race detector (`go test -race`) on native AMD64 (`ubuntu-latest`) and ARM64 (`ubuntu-24.04-arm`) runners on every PR and push to `main`.
- **Web CI (`web.yml`):** Automated static analysis (`oxlint`), code formatting check (`oxfmt`), Vitest unit test suite execution, and production Vite compilation for frontend changes.
- **Lint CI (`lint.yml`):** Runs `shellcheck` across all runner lifecycle scripts and `hadolint` across both `Dockerfile` and `Dockerfile.supervisor`.
- **Runner Multi-Arch Release (`build.yml`):** Native AMD64 and ARM64 parallel matrix build creating and publishing multi-arch manifests for `ghcr.io/<owner>/runner-aio` upon git tag release (`v*`).
- **Supervisor Multi-Arch Release (`supervisor-build.yml`):** Native multi-stage AMD64 and ARM64 build compiling the supervisor daemon and embedded UI into `ghcr.io/<owner>/gh-runner-supervisor` upon git tag release (`v*`).

---

## 🗺️ Roadmap & Future Enhancements

The core supervisor daemon, web control interface, multi-provider engine, and test suites are complete. Active research is focused on post-MVP enhancements:

- **Multi-Host Clustering:** Support for distributed Docker hosts over mutual-TLS (mTLS) TCP sockets to schedule runner pools across heterogeneous node clusters.
- **Rootless & Socket-Proxy Isolation:** Alternative supervisor orchestration backends utilizing rootless Podman / Docker or gVisor runtimes to eliminate root socket mounts.
- **Enterprise SSO / OIDC:** Federated single sign-on integration supporting OpenID Connect (OIDC), Okta, Keycloak, and GitHub OAuth for supervisor administrative access.

---

## 📄 License

Distributed under the Apache 2.0 License. See `LICENSE` for more information.
