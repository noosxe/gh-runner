package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/noosxe/gh-runner/internal/db"
	"github.com/noosxe/gh-runner/internal/server"
)

// daemonShutdownTimeout bounds the HTTP drain window on SIGTERM/SIGINT:
// in-flight health requests are trivially short, and later milestones
// (M6, RUN-41) layer their own component shutdowns under the same budget.
const daemonShutdownTimeout = 10 * time.Second

func newDaemonCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the supervisor daemon (default when no subcommand is given)",
		RunE:  runDaemon,
	}
}

// runDaemon boots the supervisor daemon and blocks until SIGINT/SIGTERM.
// Today that is the Echo v5 HTTP server with the /healthz and /readyz
// endpoints (RUN-10); later milestones grow the same process: database and
// migrations (M2), providers and the Docker orchestrator with real health
// probes (M4/M5), the pool control loop (M6), and the ConnectRPC API plus
// embedded SPA (M7). Configuration loading and validation (RUN-7) already
// ran in the root command's PersistentPreRunE.
func runDaemon(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runDaemonContext(ctx)
}

// runDaemonContext is the daemon body, parameterized by its shutdown
// context so tests drive boot and shutdown deterministically; runDaemon
// derives it from process signals.
func runDaemonContext(ctx context.Context) error {
	logger.Info("supervisor daemon starting",
		"version", version,
		"data_dir", cfg.DataDir,
		"db_path", cfg.DBPath,
		"port", cfg.Port,
	)

	database, err := db.Open(db.Options{Path: cfg.DBPath})
	if err != nil {
		return fmt.Errorf("daemon: database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.Error("closing database", "err", err)
		}
	}()

	health := server.NewHealth()
	registerHealthChecks(health, database)
	srv := server.New(server.Options{Port: cfg.Port, Health: health})

	// Start blocks, so serve from a goroutine and surface fatal errors
	// (port already bound, permission denied) through the select below.
	serveErr := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("daemon: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutdown signal received, draining http server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), daemonShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("daemon: graceful shutdown: %w", err)
	}
	logger.Info("supervisor daemon stopped")
	return nil
}

// stubCheck is a placeholder probe reporting a fixed status: it exists so
// the health endpoints answer with the full probe set from day one, while
// the milestones that own each dependency swap in real probes.
type stubCheck struct {
	name   string
	status server.Status
}

func (c stubCheck) Name() string                          { return c.name }
func (c stubCheck) Check(_ context.Context) server.Status { return c.status }

// registerHealthChecks wires the daemon's probe set (OQ #19):
// the database backs both liveness and readiness via a real SQLite ping (RUN-12);
// Docker and the auditor / control loop back readiness only, because the
// supervisor stays ready to serve while a degraded Docker socket blocks only
// pool reconciliation. Docker and auditor remain optimistic stubs until their
// owners land:
//
//   - db: SQLite ping via internal/db (M2, RUN-12)
//   - docker: Docker daemon ping, unreachable = degraded (M5, RUN-30/RUN-35)
//   - auditor: audit + control-loop heartbeat (M5/M6, RUN-32/RUN-37)
func registerHealthChecks(h *server.Health, database *db.DB) {
	dbCheck := server.NewCheck("db", func(ctx context.Context) server.Status {
		if database == nil || database.Ping(ctx) != nil {
			return server.StatusFail
		}
		return server.StatusOK
	})
	h.RegisterLiveness(dbCheck)
	h.RegisterReadiness(dbCheck)
	h.RegisterReadiness(stubCheck{name: "docker", status: server.StatusOK})
	h.RegisterReadiness(stubCheck{name: "auditor", status: server.StatusOK})
}
