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
- **Logging**: The native standard library `log/slog` is mandated for structured logging. 
  - Each backend module must instantiate a proper logger instance configured with relevant attributes (e.g., `slog.String("module", "orchestrator")`).
  - Standard log levels are `info`, `warn`, and `error`.
  - `debug` and `trace` logs should only be emitted if the application is explicitly started in debug mode.
  - The primary sink is `stdout` (standard for containerized applications), but the architecture must allow plugging in other sinks.

## 2. Frontend Stack (Web UI)

The Web Control Interface is a Single Page Application (SPA) bundled and served by the Go backend.

- **Framework**: **React** with **TypeScript**, built via **Vite**.
- **Routing**: **TanStack Router** (`@tanstack/react-router`) handles all client-side navigation.
- **Data Fetching & RPC**: **TanStack Query** (`@tanstack/react-query`) is deeply integrated with the ConnectRPC client clients for fetching, caching, and mutating data.
- **Ecosystem**: The use of other available `@tanstack` libraries is highly encouraged where applicable.
- **Styling**: **TailwindCSS** is strictly mandated. No manual CSS files or classes should be written. 
- **Theming**: The UI must support both Light and Dark themes, with automatic selection based on the user's system preferences.
