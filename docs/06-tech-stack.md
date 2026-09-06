# Technology Stack

This document outlines the mandated technology stack for the AIO Supervisor backend and frontend.

## 1. Backend Stack (Go)

The backend daemon is engineered to be modular, extensible, and free of heavy CGO dependencies to ensure seamless cross-compilation for ARM64 and AMD64 architectures.

- **CLI Framework**: `github.com/spf13/cobra` is used for the application's CLI entrypoints and command definitions.
- **Configuration Management**: `github.com/knadh/koanf` handles all configuration parsing. It must support `yaml` and `toml` file formats, CLI flags, and environment variables.
- **Web Server**: **Echo v5** (`github.com/labstack/echo/v5`) is the primary HTTP framework.
- **RPC Framework**: **ConnectRPC** (`connectrpc.com/connect`) is used for all backend-to-frontend communication. **Binary mode is mandatory** (no JSON transport). All protocols (protobufs) are defined in the design phase.
- **Database Engine**: Embedded **SQLite** using the pure-Go driver `modernc.org/sqlite` to strictly avoid CGO requirements.
- **Database Tooling**: 
  - **SQLc** (`github.com/sqlc-dev/sqlc`) generates type-safe Go code from SQL schema and query definitions.
  - **Goose** (`github.com/pressly/goose/v3`) handles database migrations. Migrations are executed automatically during application startup, and any errors are strictly logged.
- **Container Orchestration SDK**: **Moby Client SDK** (`github.com/moby/moby/client` and `github.com/moby/moby/api`). The supervisor interacts with the local container runtime strictly via the decoupled client module and API types package, avoiding monolithic daemon dependencies (`docker/docker`) and eliminating downstream CVE surfaces.
- **Logging**: The native standard library `log/slog` is mandated for structured logging. 
  - Each backend module must instantiate a proper logger instance configured with relevant attributes (e.g., `slog.String("module", "orchestrator")`).
  - Standard log levels are `info`, `warn`, and `error`.
  - `debug` and `trace` logs should only be emitted if the application is explicitly started in debug mode.
  - The primary sink is `stdout` (standard for containerized applications), but the architecture must allow plugging in other sinks.

## 2. Frontend Stack (Web UI)

The Web Control Interface is a Single Page Application (SPA) bundled and served by the Go backend.

- **Framework**: **React** with **TypeScript**, built via **Vite**.
- **Package Manager**: **pnpm** is strictly mandated across the repository for all Node/frontend dependency management.
- **Linting & Formatting**: **oxlint** and **oxfmt** (from the Rust-based Oxc project) replace legacy ESLint and Prettier, providing sub-second static analysis and deterministic formatting.
- **Routing**: **TanStack Router** (`@tanstack/react-router`) handles all client-side navigation.
- **Data Fetching & RPC**: **TanStack Query** (`@tanstack/react-query`) is deeply integrated with the ConnectRPC client clients for fetching, caching, and mutating data.
- **Ecosystem**: The use of other available `@tanstack` libraries is highly encouraged where applicable.
- **Styling**: **TailwindCSS** is strictly mandated. No manual CSS files or classes should be written. 
- **Theming**: The UI must support both Light and Dark themes, with automatic selection based on the user's system preferences.

## 3. Container & Process Management

- **Init System**: **s6-overlay** is mandated as the init system and process manager inside the Supervisor Docker container. Even though the primary workload is a single Go process, utilizing s6-overlay provides robust signal handling, zombie process reaping, and standardized startup/shutdown initialization phases, ensuring the container architecture adheres to production best practices.

## 4. Multi-Stage Container Build Strategy

The Supervisor is packaged as a lightweight, multi-stage Docker image to keep the final footprint minimal and avoid shipping build toolchains to production.

- **Stage 1: Frontend Build (`node:24-alpine`)**
  - Installs Node dependencies via `pnpm install --frozen-lockfile` and builds the Vite/React/TypeScript web application (`pnpm run build`).
- **Stage 2: Backend Build (`golang:1.26-alpine`)**
  - The static output directory from the frontend build stage is copied into this stage.
  - The frontend assets are embedded directly into the Go application using the native `go:embed` directive.
  - The Go compiler builds the final standalone binary.
- **Stage 3: Final Runtime (`alpine:latest`)**
  - Uses a fresh, minimal Alpine Linux base image.
  - Installs the `s6-overlay` init system.
  - Copies the final Go binary from Stage 2 into the image.
  - Exposes the necessary ports and volumes (e.g., for the SQLite database).
