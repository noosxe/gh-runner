package main

import (
	"strings"
	"testing"
)

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

// TestBareInvocationRunsDaemon verifies that a bare `supervisor` delegates
// to the daemon handler.
func TestBareInvocationRunsDaemon(t *testing.T) {
	root := NewRootCommand()
	root.SetArgs([]string{})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "daemon: not implemented") {
		t.Fatalf("bare invocation should delegate to the daemon stub, got %v", err)
	}
}

// TestImportRequiresConfigFlag verifies that import refuses to run without
// an explicit --config path.
func TestImportRequiresConfigFlag(t *testing.T) {
	root := NewRootCommand()
	root.SetArgs([]string{"import"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--config") {
		t.Fatalf("import without --config should fail, got %v", err)
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
