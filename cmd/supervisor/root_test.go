package main

import (
	"strings"
	"testing"
)

// validKeyEnv returns t.Setenv for a strong placeholder encryption key so
// the loader's required-key validation passes and tests can focus on the
// behavior under test.
func validKeyEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SUPERVISOR_DB_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
}

// TestUsageListsAllSubcommands mirrors the acceptance criterion:
// `supervisor --help` lists every planned subcommand.
func TestUsageListsAllSubcommands(t *testing.T) {
	usage := NewRootCommand().UsageString()
	for _, name := range []string{"daemon", "import", "reset-password", "backup"} {
		if !strings.Contains(usage, name) {
			t.Errorf("usage output does not mention subcommand %q:\n%s", name, usage)
		}
	}
}

// TestBareInvocationRunsDaemon verifies that a bare `supervisor` (with a
// valid configuration) delegates to the daemon handler.
func TestBareInvocationRunsDaemon(t *testing.T) {
	validKeyEnv(t)
	root := NewRootCommand()
	root.SetArgs([]string{})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "daemon: not implemented") {
		t.Fatalf("bare invocation should delegate to the daemon stub, got %v", err)
	}
}

// TestDaemonRefusesToStartWithoutEncryptionKey mirrors the RUN-7
// acceptance criterion: the supervisor refuses to boot with an
// actionable error when the encryption key is not provided.
func TestDaemonRefusesToStartWithoutEncryptionKey(t *testing.T) {
	// Explicitly empty beats any ambient value; an empty key is "missing".
	t.Setenv("SUPERVISOR_DB_ENCRYPTION_KEY", "")
	root := NewRootCommand()
	root.SetArgs([]string{"daemon"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "SUPERVISOR_DB_ENCRYPTION_KEY") {
		t.Fatalf("daemon without an encryption key should refuse to start, got %v", err)
	}
}

// TestImportRequiresConfigFlag verifies that import refuses to run without
// an explicit --config path.
func TestImportRequiresConfigFlag(t *testing.T) {
	validKeyEnv(t)
	root := NewRootCommand()
	root.SetArgs([]string{"import"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--config") {
		t.Fatalf("import without --config should fail, got %v", err)
	}
}

// TestEnvironmentReachesDaemon verifies that SUPERVISOR_* variables flow
// through the config stack into the running command.
func TestEnvironmentReachesDaemon(t *testing.T) {
	validKeyEnv(t)
	t.Setenv("SUPERVISOR_DATA_DIR", "/env-data")
	root := NewRootCommand()
	root.SetArgs([]string{"daemon"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "daemon: not implemented") {
		t.Fatalf("daemon with valid env config should reach its stub, got %v", err)
	}
	if cfg == nil || cfg.DataDir != "/env-data" {
		t.Fatalf("configuration was not loaded from the environment: %+v", cfg)
	}
}

// TestInvalidLogLevelRejected verifies that bindFlagsToConfig rejects a
// bogus --log-level before any command handler runs.
func TestInvalidLogLevelRejected(t *testing.T) {
	root := NewRootCommand()
	root.SetArgs([]string{"--log-level", "bogus", "backup"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "log-level") {
		t.Fatalf("invalid log level should be rejected, got %v", err)
	}
}
