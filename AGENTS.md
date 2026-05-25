# Agent Guidelines (AGENTS.md)

Welcome, AI Agent! This document acts as your compass for contributing to this repository. It defines our project context, security protocols, structural expectations, and operational guidelines to ensure that all agentic contributions remain **secure**, **robust**, and **maintainable**.

---

## 🎯 1. Project Context & Objectives
This repository is dedicated to building a custom, lightweight, and secure self-hosted **GitHub Actions Runner** packaged as a Docker image.

### High-Level Goals:
- **First-Class ARM64 Support:** The image must run flawlessly on ARM64 (e.g., Apple Silicon, AWS Graviton, Raspberry Pi 4/5) and support multi-architecture builds (specifically ARM64 and AMD64) as we evolve.
- **Docker Compose Orchestration:** Provide a seamless, plug-and-play developer experience via `docker-compose.yml` to instantly register and spin up runners.
- **Graceful Lifecycle Management:** Ensure runners register on startup and cleanly deregister from GitHub upon termination (`SIGTERM`/`SIGINT`) to prevent "offline ghost runners" from cluttering the GitHub dashboard.

---

## 🔒 2. Security First: Critical Agentic Guardrails
GitHub runners execute untrusted workflow code. Security is our absolute priority. Any code or configuration you create must adhere to the following:

### A. Credential & Token Safety
* **Zero-Leak Policy:** **NEVER** hardcode Personal Access Tokens (PATs), Runner Registration Tokens, or any sensitive API keys in the Dockerfile, scripts, workflow files, or documentation.
* **Environment Templates:** Store configuration templates in `.env.example`. Ensure any real `.env` files are added to `.gitignore`.
* **Token Lifetime:** Design scripts to prioritize short-lived runner registration tokens rather than highly privileged personal access tokens (PATs) where possible.

### B. Container Hardening
* **Non-Root Execution:** The runner process inside the container should execute as a dedicated, non-root user (e.g., `runner:runner` with a specified UID/GID). Avoid running as `root` unless specifically required for Docker-in-Docker (DinD), and even then, explore rootless Docker options.
* **Minimal Base Image:** Use lightweight, minimal, and secure base images (such as Alpine Linux or a minimal Debian-slim distro) to reduce the container's attack surface and speed up downloads.
* **Least Privilege:** Document minimal Docker configurations (e.g., dropping capabilities with `cap_drop`) for the runner container to minimize host system exposure.

---

## 🏗️ 3. Proposed Repository Architecture
When creating new files, structure the repository logically as follows:

```text
.
├── .github/
│   └── workflows/
│       ├── build.yml         # CI/CD: Multi-arch Docker build & push (via Buildx)
│       └── lint.yml          # CI/CD: Automated shell and Docker linter checks
├── src/
│   ├── entrypoint.sh         # Main orchestration script: setup, registration, execution, and cleanup
│   └── register.sh           # Helper script for registering the runner with GitHub APIs
├── tests/
│   ├── integration/          # Integration tests using local mock APIs or running Docker
│   └── unit/                 # Script unit tests (e.g., using BATS or ShellSpec)
├── Dockerfile                # Multi-stage, multi-arch Dockerfile (arm64 & amd64)
├── docker-compose.yml        # Standard compose file for instant local deployment
├── .env.example              # Template file for environment variable settings
├── README.md                 # User instructions, quickstart, and configuration guides
└── AGENTS.md                 # This file
```

---

## 🛠️ 4. Coding Standards & Best Practices

### A. Shell Scripting (`src/entrypoint.sh`, etc.)
* **Strict Mode:** Always start shell scripts with standard bash security switches:
  ```bash
  #!/bin/bash
  set -euo pipefail
  ```
* **Robust Error Handling:** Wrap crucial commands (like runner registration) with error messages and graceful exits.
* **Graceful Deregistration:** Implement active traps to handle termination signals (`SIGTERM`, `SIGINT`) and trigger deregistration commands to tell GitHub the runner is shutting down.
  ```bash
  trap 'cleanup' SIGTERM SIGINT
  ```
* **Tooling:** Lint shell scripts with `shellcheck` and format them with `shfmt`.

### B. Dockerfile Best Practices
* **Multi-Stage Builds:** Use multi-stage builds to keep the final runner image extremely slim.
* **Caching Optimization:** Order directives to leverage Docker layer caching (e.g., copy package lists/dependencies before application scripts).
* **Hadolint Compliance:** Ensure Dockerfile statements conform to `hadolint` rules (e.g., pin package versions, clean package caches in the same `RUN` layer).

---

## ⚙️ 5. Standard Environment Configurations
The runner container must recognize and parse these configuration variables:

| Variable | Type | Required | Description | Default |
|----------|------|----------|-------------|---------|
| `GITHUB_REPOSITORY_URL` | String | **Yes** | The full URL of the private GitHub repository (e.g., `https://github.com/owner/repo`). | - |
| `RUNNER_TOKEN` | String | **Yes** | The pre-generated runner registration token for the repository. | - |
| `RUNNER_NAME` | String | No | Custom name for the runner inside the GitHub dashboard. | `hostname` |
| `RUNNER_LABELS` | String | No | Comma-separated list of custom labels. | `self-hosted,linux,arm64` |
| `RUNNER_WORKDIR` | String | No | Work directory inside the container. | `_work` |

---

## 🤖 6. AI Agent Workflow & Instructions
When you are tasked with extending or fixing this repository:
1. **Plan Before Coding:** Discuss structural changes or design tradeoffs with the user first.
2. **Maintain Quality:** Write robust, clear inline comments explaining *why* certain workarounds (e.g., specific environment flags for ARM64 compatibility) are utilized.
3. **Verify Locally:** Test code by initiating building and running test containers using the `docker` and `docker compose` tools.
4. **Respect existing guides:** Do not violate rules defined in this file. If you need to update this document, explain the rationale clearly to the user.
