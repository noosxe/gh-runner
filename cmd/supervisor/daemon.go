package main

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

// newDaemonCommand creates the `supervisor daemon` subcommand: the default,
// long-running process that reconciles runner pools and serves the web
// control interface (docs/03-lifecycle-and-orchestration.md).
func newDaemonCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the supervisor daemon (default when no subcommand is given)",
		RunE:  runDaemon,
	}
}

// runDaemon is a stub. Later milestones boot the real daemon here: database
// migrations, provider and orchestrator wiring, the pool reconciler loop,
// and the Echo web server. Configuration loading and validation (RUN-7)
// already ran in the root command's PersistentPreRunE.
func runDaemon(cmd *cobra.Command, args []string) error {
	slog.Info("supervisor daemon starting", "version", version, "data_dir", cfg.DataDir, "db_path", cfg.DBPath)
	return fmt.Errorf("daemon: %w", errNotImplemented)
}
