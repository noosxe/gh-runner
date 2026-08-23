package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

// errNotImplemented is returned by the stub handlers below so callers and
// scripts never mistake a stub for a successful run. Later milestones
// replace the stubs and drop this sentinel.
var errNotImplemented = errors.New("not implemented yet")

// k is the shared koanf instance. RUN-6 layers only the parsed CLI flags
// into it; RUN-7 extends the loader with YAML/TOML files and environment
// variables while keeping flags the highest-precedence source.
var k = koanf.New(".")

// Persistent flag variables, bound in NewRootCommand and layered into k in
// bindFlagsToConfig.
var (
	flagConfig     string
	flagLogLevel   string
	flagDataDir    string
	flagDBPath     string
	flagPort       int
	flagDockerHost string
)

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

// bindFlagsToConfig layers the parsed persistent flags into the shared
// koanf instance and installs the process-wide logger.
func bindFlagsToConfig(cmd *cobra.Command, _ []string) error {
	if err := k.Load(posflag.Provider(cmd.Root().PersistentFlags(), ".", k), nil); err != nil {
		return fmt.Errorf("loading CLI flags into configuration: %w", err)
	}
	return initLogger(k.String("log-level"))
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
