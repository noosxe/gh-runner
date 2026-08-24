package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// newImportCommand creates the `supervisor import` subcommand: an explicit
// and destructive re-import of pool configuration from a YAML file into
// the database (docs/02-architecture-design.md §4).
func newImportCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "import",
		Short: "Import pool configuration from a YAML file into the database",
		Long: `Import merges or overwrites database state from the YAML file given via
--config. This is a destructive operation; once implemented it asks for
confirmation before writing.`,
		RunE: runImport,
	}
}

// runImport is a stub. Later milestones parse the YAML file, diff it
// against database state, and apply it behind an interactive confirmation.
func runImport(cmd *cobra.Command, args []string) error {
	configPath := cfg.ConfigFile
	if configPath == "" {
		return errors.New("import: the --config flag is required (path to a YAML configuration file)")
	}
	logger.Info("importing configuration", "config", configPath)
	return fmt.Errorf("import: %w", errNotImplemented)
}
