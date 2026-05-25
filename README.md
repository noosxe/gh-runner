# Custom Self-Hosted GitHub Actions Runner

A lightweight, secure, and self-contained self-hosted GitHub Actions Runner packaged as a multi-architecture Docker container. Designed to run natively on **ARM64** (Apple Silicon, AWS Graviton, Raspberry Pi) and **AMD64** platforms, with seamless local orchestration via Docker Compose.

---

## ✨ Features

- **Native Architecture Execution:** Built and executed natively on both ARM64 and AMD64 hosts without CPU emulation overhead.
- **Graceful Lifecycle Management:** Intercepts system termination signals (`SIGTERM`/`SIGINT`) to cleanly de-register the runner from the GitHub repository before container teardown, preventing "offline ghost runners" in your dashboard.
- **Secure Non-Root Isolation:** The runner agent and jobs execute under a dedicated, low-privilege `runner` system user rather than `root`.
- **Fast Build Times:** Pre-bakes dotnet runtimes and core operating system dependencies into the container layer to minimize boot latency.

---

## 🚀 Quickstart

Follow these steps to instantly deploy a local runner:

### 1. Copy Environment Configuration
Create a local `.env` file from the provided template:
```bash
cp .env.example .env
```

### 2. Configure Settings
Go to your GitHub private repository ➡️ **Settings** ➡️ **Actions** ➡️ **Runners** ➡️ **New self-hosted runner**. 

Update your local `.env` file with your details:
```env
# The full URL of the repository you want to attach the runner to
GITHUB_REPOSITORY_URL=https://github.com/owner/repo

# A fresh, short-lived runner registration token from the settings page
RUNNER_TOKEN=your_registration_token_here

# Optional configuration
RUNNER_NAME=local-docker-runner
RUNNER_LABELS=self-hosted,linux,arm64
```

### 3. Start the Runner
Launch the runner in detached mode using Docker Compose:
```bash
docker compose up -d
```

To monitor the registration and execution logs:
```bash
docker compose logs -f
```

### 4. Stop and De-Register
To cleanly stop the runner, simply run:
```bash
docker compose down
```
*Note: This command triggers the registration cleanup hook, automatically removing the runner from the GitHub Repository Actions UI instantly.*

---

## 🛠️ CI/CD Build Pipeline

The project integrates an automated **Parallel Native Matrix & Manifest Merger** workflow:
- **Pull Requests and Push to main:** Triggers a native dry-run compilation on parallel AMD64 and ARM64 GitHub runners to ensure code and Docker layer compatibility.
- **Releases (Tag Push `v*`):** 
  1. Compiles the containers on native runners and publishes them by content-digest to the GitHub Container Registry (GHCR).
  2. Runs a downstream coordination job that merges the digests into a unified multi-architecture manifest list under the version tag (e.g. `v1.0.0`) and the `latest` tag.
