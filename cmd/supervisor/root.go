package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/noosxe/gh-runner/internal/config"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

// errNotImplemented is returned by the stub handlers below so callers and
// scripts never mistake a stub for a successful run. Later milestones
// replace the stubs and drop this sentinel.
var errNotImplemented = errors.New("not implemented yet")

// cfg holds the fully merged and validated configuration once the root
// command's PersistentPreRunE has run. Every subcommand reads its
// settings from here instead of touching flags or the environment.
var cfg *config.Config

// NewRootCommand builds the `supervisor` root command with all subcommands
// and persistent flags attached.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "supervisor",
		Short: "AIO Supervisor for ephemeral GitHub/Gitea/Forgejo runner containers",
		Long: `supervisor manages dynamic pools of ephemeral GitHub, Gitea, and Forgejo
Actions runner containers and serves the embedded web control interface.

Run it with no subcommand to start the daemon.`,
		Version:           version,
		SilenceUsage:      true, // runtime errors should not repeat the usage text
		PersistentPreRunE: bindFlagsToConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			// The daemon is the default: a bare `supervisor` (optionally
			// with persistent flags) starts it.
			return runDaemon(cmd, args)
		},
	}

	// Flag values are bound to throwaway locals: everything flows through
	// the typed config.Config produced by internal/config, which loads
	// these flags as the highest-precedence layer (RUN-7).
	var flagConfig, flagLogLevel, flagDataDir, flagDBPath, flagDockerHost string
	var flagPort int
	f := root.PersistentFlags()
	f.StringVarP(&flagConfig, "config", "c", "", "path to the configuration file (YAML or TOML)")
	f.StringVar(&flagLogLevel, "log-level", "info", "log level (debug, info, warn, error)")
	f.StringVar(&flagDataDir, "data-dir", "/data", "data directory holding the database, backups, and runner logs")
	f.StringVar(&flagDBPath, "db-path", "", "path to the SQLite database file (defaults to <data-dir>/supervisor.db)")
	f.IntVar(&flagPort, "port", 8080, "HTTP port for the API and web control interface")
	f.StringVar(&flagDockerHost, "docker-host", "", "Docker daemon endpoint (defaults to the local Docker socket)")

	root.AddCommand(
		newDaemonCommand(),
		newImportCommand(),
		newResetPasswordCommand(),
		newBackupCommand(),
	)
	return root
}

// Execute runs the root command. Cobra prints errors itself; main only
// needs the error value to decide the exit code.
func Execute() error {
	return NewRootCommand().Execute()
}

// bindFlagsToConfig loads the full configuration stack (defaults, config
// file, SUPERVISOR_* environment, CLI flags) via internal/config, which
// also validates the result, and installs the process-wide logger.
func bindFlagsToConfig(cmd *cobra.Command, _ []string) error {
	loaded, err := config.Load(config.Options{Flags: cmd.Root().PersistentFlags()})
	if err != nil {
		return err
	}
	cfg = loaded
	return initLogger(cfg.LogLevel)
}

// initLogger installs the default slog logger. RUN-8 replaces this with
// per-module loggers and debug gating; the CLI skeleton only needs level
// selection.
func initLogger(level string) error {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return fmt.Errorf("invalid --log-level %q (want debug, info, warn, or error)", level)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
	return nil
}
