# Custom Self-Hosted GitHub Actions Runner

A lightweight, secure, and self-contained self-hosted GitHub Actions Runner packaged as a multi-architecture Docker container. Designed to run natively on **ARM64** (Apple Silicon, AWS Graviton, Raspberry Pi) and **AMD64** platforms, with seamless local orchestration via Docker Compose.

---

## ✨ Features

- **Native Architecture Execution:** Built and executed natively on both ARM64 and AMD64 hosts without CPU emulation overhead.
- **Graceful Lifecycle Management:** Intercepts system termination signals (`SIGTERM`/`SIGINT`) to cleanly de-register the runner from the GitHub repository before container teardown, preventing "offline ghost runners" in your dashboard.
- **Secure Non-Root Isolation:** The runner agent and jobs execute under a dedicated, low-privilege `runner` system user rather than `root`.
- **Fast Build Times:** Pre-bakes dotnet runtimes and core operating system dependencies into the container layer to minimize boot latency.
- **Layered Supervisor Configuration:** The `supervisor` daemon merges built-in defaults, an optional YAML/TOML settings file, `SUPERVISOR_*` environment variables, and CLI flags (in increasing precedence), with typed validation that refuses to start without a strong `SUPERVISOR_DB_ENCRYPTION_KEY`.
- **Single Master Key, Derived Secrets:** The one required `SUPERVISOR_DB_ENCRYPTION_KEY` is expanded via HKDF-SHA256 (`internal/keys`) into two independent runtime secrets — the AES-256 database encryption key and the JWT signing secret — each under its own context label, so there is no separate `JWT_SECRET` to configure or leak, and both remain stable across restarts.
- **Health Endpoints:** The daemon serves `GET /healthz` (liveness: process alive + DB accessible) and `GET /readyz` (readiness: DB + auditor/control loop; Docker unreachable reports `degraded` but stays ready) on `SUPERVISOR_PORT`, answering JSON with per-check detail — e.g. `{ "status": "ready", "checks": { "db": "ok", "docker": "degraded", "auditor": "ok" } }` — for compose healthchecks, load balancers, and monitoring probes.
- **Embedded SQLite & Auto-Migration Runner:** Pure-Go SQLite persistence via `modernc.org/sqlite` (strictly CGO-free for seamless ARM64/AMD64 cross-compilation) with automatic Goose migrations at startup, strict migration error logging, and startup refusal on corrupted databases directing administrators to backup snapshots (OQ #21).
- **Unified Multi-Provider Runner Image (`runner-aio`):** A single multi-stage, multi-architecture (`amd64` and `arm64`) container image packaging GitHub Actions runner, Gitea `act_runner`, and Forgejo `forgejo-runner` with version-pinned downloads, SHA256 checksum verification, automatic provider detection (`RUNNER_PROVIDER`, `GITEA_INSTANCE_URL`, `FORGEJO_INSTANCE_URL`), ephemeral one-job execution semantics, non-root hardening (`UID/GID 1001`), and automatic interrupt de-registration traps. Published to GHCR as `ghcr.io/noosxe/runner-aio:latest`.
- **Git Provider Integration & Dynamic Token Engine:** Swappable `GitProvider` interface supporting GitHub (App auth chain with RS256 JWTs and PAT fallback), Gitea (PAT API), and Forgejo (PAT API with queued-job polling). Includes shared rate-limit backoff middleware honoring `Retry-After` and `X-RateLimit-Reset` headers, a 15-minute maximum backoff cap, persistent 10-minute alert dispatching, and strict token segregation ensuring master credentials never leak into runner container specs.

---

## 🚀 Usage Guide

This self-hosted runner can be deployed either by pulling the pre-built multi-architecture container from the **GitHub Container Registry (GHCR)** (recommended) or by compiling it locally from source.

### Option A: Using the Pre-Built Image (Recommended)

To run the runner without needing to download or build the source code locally, create a `compose.yml` file and a `.env` file in a directory of your choice.

#### 1. Create a `compose.yml` File
Save the following configuration as `compose.yml`:

```yaml
services:
  runner:
    image: ghcr.io/noosxe/runner-aio:latest
    container_name: runner-aio
    # Use standard init process to reap zombie processes spawned by runner jobs
    init: true
    restart: unless-stopped
    environment:
      - GITHUB_REPOSITORY_URL=${GITHUB_REPOSITORY_URL}
      - RUNNER_TOKEN=${RUNNER_TOKEN}
      - RUNNER_NAME=${RUNNER_NAME:-runner-aio}
      - RUNNER_LABELS=${RUNNER_LABELS:-self-hosted,linux,arm64}
      - RUNNER_WORKDIR=${RUNNER_WORKDIR:-_work}
    # Mount volumes to support Docker-outside-of-Docker (DooD) operations
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    # Optional: If your jobs need to communicate with Docker outside the container,
    # uncomment below and match "999" to your host's docker group GID (run `getent group docker`).
    # group_add:
    #   - "999"
```

#### 2. Create the Environment Configuration
Create a `.env` file in the same directory:

```env
# The full URL of the GitHub repository
GITHUB_REPOSITORY_URL=https://github.com/owner/repo

# A fresh, short-lived runner registration token from repository settings:
# Settings -> Actions -> Runners -> New self-hosted runner
RUNNER_TOKEN=your_registration_token_here

# Optional Configurations
RUNNER_NAME=prod-gh-runner
RUNNER_LABELS=self-hosted,linux,arm64
RUNNER_WORKDIR=_work
```

#### 3. Spin Up the Runner
Deploy the container in the background:
```bash
docker compose up -d
```

To monitor the registration and execution logs:
```bash
docker compose logs -f
```

---

### Option B: Local Build and Execution (From Source)

If you have cloned this repository and want to build the container locally:

#### 1. Setup Your Environment File
Duplicate the provided example configuration:
```bash
cp .env.example .env
```
Open `.env` and fill in your `GITHUB_REPOSITORY_URL` and `RUNNER_TOKEN`.

#### 2. Start the Runner via Local Build
Build and run the container locally:
```bash
docker compose up --build -d
```

To monitor logs or tear it down:
```bash
docker compose logs -f
docker compose down
```

---

## ⚙️ Configuration & Environment Variables

The runner container is highly customizable via environment variables defined in your `.env` file:

| Variable | Type | Required | Default | Description |
| :--- | :--- | :---: | :--- | :--- |
| `GITHUB_REPOSITORY_URL` | String | **Yes** | — | The full repository URL to register the runner to (e.g., `https://github.com/owner/repo`). |
| `RUNNER_TOKEN` | String | **Yes** | — | The temporary registration token obtained from GitHub runner settings (expires after 1 hour). |
| `RUNNER_NAME` | String | No | *container-hostname* | The name displayed for this runner on the GitHub Actions dashboard. |
| `RUNNER_LABELS` | String | No | `self-hosted,linux,arm64` | A comma-separated list of custom labels to tag the runner with. |
| `RUNNER_WORKDIR` | String | No | `_work` | The internal working directory where workflow jobs will run. |

### Supervisor Daemon Configuration

The `supervisor` daemon layers its configuration, lowest to highest precedence:

1. built-in defaults,
2. an optional settings file (`--config` / `SUPERVISOR_CONFIG`; YAML or TOML, keys spelled like the flags below, e.g. `data-dir: /data`),
3. `SUPERVISOR_*` environment variables,
4. CLI flags (only flags you actually pass override lower layers).

| Variable | Type | Required | Default | Description |
| :--- | :--- | :---: | :--- | :--- |
| `SUPERVISOR_DB_ENCRYPTION_KEY` | String | **Yes** | — | Master key encrypting credentials in the database (AES-256) and deriving the JWT signing secret. Must be at least 32 bytes (`openssl rand -base64 32`); the daemon refuses to start without it. |
| `SUPERVISOR_PORT` | Int | No | `8080` | HTTP port for the API and web control interface. |
| `SUPERVISOR_DB_PATH` | String | No | `<data-dir>/supervisor.db` | Path to the SQLite database file. |
| `SUPERVISOR_LOG_LEVEL` | String | No | `info` | One of `debug`, `info`, `warn`, `error`. Selecting `debug` is explicit debug mode: it also unlocks trace output. Logs are structured JSON on stdout, one record per module (`module` field). |
| `SUPERVISOR_DOCKER_HOST` | String | No | `unix:///var/run/docker.sock` | Docker daemon endpoint used to launch runner containers. |
| `SUPERVISOR_DATA_DIR` | String | No | `/data` | Data directory holding the database, backups, and runner logs. |
| `SUPERVISOR_BACKUP_INTERVAL_HOURS` | Int | No | `6` | Hours between automated SQLite snapshot backups. |
| `SUPERVISOR_BACKUP_RETENTION_COUNT` | Int | No | `7` | Number of snapshot backups to retain. |
| `SUPERVISOR_CONFIG` | String | No | — | Path to a YAML/TOML settings file (overridden by `--config`). |

---

## 🔒 Security & Execution Features

### Docker-Outside-of-Docker (DooD)
This runner is configured to support Docker-outside-of-Docker execution. This allows workflow actions inside the runner to invoke sibling containers on the host machine.
* The container mounts `/var/run/docker.sock` from the host.
* **Important Permission Note:** To allow the non-root `runner` user inside the container to talk to the host's Docker socket, you may need to uncomment the `group_add` section in `compose.yml` and provide your host machine's `docker` group ID:
  ```bash
  # Find host docker GID
  getent group docker | cut -d: -f3
  ```

### Graceful Lifecycle Management
* On startup, the container registers with the GitHub API using the provided token.
* Upon termination (`docker compose down`, `docker stop`, `SIGTERM`/`SIGINT`), a trap triggers a cleanup routine that automatically de-registers the runner from the repository. This guarantees that your runner dashboard does not get cluttered with offline zombie runners.

### Docker Orchestrator Core
* **Ephemeral Runner Lifecycle:** Spawns and manages ephemeral runner and task containers with deterministic naming (`ghrs-<pool-slug>-<6-hex>`), supervisor tracking labels (`com.github-runner-supervisor.*`), CPU/memory limits, and environment segregation.
* **Supervisor-Managed Bridge Network:** Connects runners to a dedicated `ghrs-supervisor` bridge network isolated from supervisor host networks.
* **Event-Driven Container Reaping:** Subscribes to Docker Engine `die` and `destroy` events with deduplication to ensure low-latency (<2s) cleanup without duplicate provisioning races.
* **Exit Log Capture & Compression:** Captures container stdout and stderr via the Docker Logs API on exit, structures them into JSONL with timestamps and stream tagging, and streams them directly into gzipped files (`DATA_DIR/logs/<runner-id>.log.jsonl.gz`) before container removal.
* **Degraded-Mode Handling:** Gracefully handles Docker daemon disconnections by pausing container spawning and rate-limiting log errors. Surfaces reachability via `/readyz` (`checks.docker: "degraded"`) while keeping the supervisor alive, and automatically recovers when the daemon returns.

> [!WARNING]
> Keep your `RUNNER_TOKEN` confidential. It is short-lived, but it grants the ability to register runners capable of executing arbitrary code inside your host environment.

---

## 🛠️ CI/CD Pipelines

### Go CI (`go.yml`)

Builds, vets, and tests the Go module (`cmd/`, `internal/`) on every PR and push to `main` touching Go inputs:
- **Native AMD64 + ARM64 matrix:** `go build ./...`, `go vet ./...`, and `go test ./...` run on native runners (`ubuntu-latest` / `ubuntu-24.04-arm`) with `CGO_ENABLED=0`, matching the pure-Go stack mandate (docs/06 §1) and the local `nix develop` Makefile targets.
- **Toolchain pinned to `go.mod`:** CI resolves the Go version from `go-version-file: go.mod` so it never drifts from the module definition.

### Lint (`lint.yml`)

Automated shell and Docker linter checks on PRs and pushes to `main` touching image inputs:
- **ShellCheck:** validates `src/*.sh` — identical to the mandated local check `nix develop --command shellcheck src/*.sh`.
- **Hadolint:** validates the runner `Dockerfile`; error-severity findings fail the job, style warnings surface as annotations.

### Multi-Arch Image Build (`build.yml`)

The **Parallel Native Matrix & Manifest Merger** workflow:
- **Pull Requests and Push to main:** Triggers a native dry-run compilation on parallel AMD64 and ARM64 GitHub runners to ensure code and Docker layer compatibility (only when image inputs change).
- **Releases (Tag Push `v*`):** 
  1. Compiles the containers on native runners and publishes them by content-digest to the GitHub Container Registry (GHCR) as `ghcr.io/<owner>/runner-aio`.
  2. Runs a downstream coordination job that merges the digests into a unified multi-architecture manifest list under the version tag (e.g., `v1.0.0`) and the `latest` tag.

---

## 🗺️ Roadmap

The repository is actively developing the following advanced runner solutions:

- **GitHub, Gitea & Forgejo Actions Runner AIO Supervisor** `*[Design Phase]*`  
  A database-driven and GUI-configured containerized manager/coordinator daemon that runs on the host and automatically provisions, schedules, and maintains dynamic pools of ephemeral runner containers across multiple GitHub, Gitea, and Forgejo repositories. High-level requirements and architecture specifications can be found in the [docs/](docs/) directory (see [01-product-requirements.md](docs/01-product-requirements.md) and [02-architecture-design.md](docs/02-architecture-design.md)).
