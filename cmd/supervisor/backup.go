package main

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

// newBackupCommand creates the `supervisor backup` subcommand: an on-demand
// SQLite snapshot alongside the automated periodic ones (open questions
// #21).
func newBackupCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Take an on-demand snapshot of the SQLite database",
		Long: `Writes a consistent SQLite backup to
<data-dir>/backups/supervisor-<timestamp>.db, the same location and format
used by the automated periodic snapshots.`,
		RunE: runBackup,
	}
}

// runBackup is a stub. Later milestones snapshot the SQLite database via
// its online backup API and prune old snapshots per the retention settings.
func runBackup(cmd *cobra.Command, args []string) error {
	slog.Info("backup: snapshotting not implemented yet", "data_dir", k.String("data-dir"))
	return fmt.Errorf("backup: %w", errNotImplemented)
}
