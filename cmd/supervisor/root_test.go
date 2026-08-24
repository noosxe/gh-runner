package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
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

// TestBareInvocationRunsDaemon verifies that a bare `supervisor` delegates
// to the daemon handler. The daemon now blocks serving HTTP (RUN-10), so
// delegation is asserted structurally; the serving loop itself is
// exercised with a cancellable context in TestDaemonServesHealthEndpoints.
func TestBareInvocationRunsDaemon(t *testing.T) {
	root := NewRootCommand()
	if root.RunE == nil || reflect.ValueOf(root.RunE).Pointer() != reflect.ValueOf(runDaemon).Pointer() {
		t.Fatal("a bare invocation must delegate to the daemon handler")
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

// freePort asks the kernel for an unused TCP port by binding :0, reading
// the port back, and closing. The daemon rebinds it moments later; the
// tiny window is the standard trade-off of this pattern.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a free port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// waitFor polls url until an HTTP response arrives or the deadline passes.
func waitFor(t *testing.T, url string) *http.Response {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			return resp
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s never answered", url)
	return nil
}

// healthBody is the wire shape of /healthz and /readyz (OQ #19).
type healthBody struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// getHealth fetches and decodes url.
func getHealth(t *testing.T, url string) healthBody {
	t.Helper()
	resp := waitFor(t, url)
	defer func() { _ = resp.Body.Close() }()
	var body healthBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding %s body: %v", url, err)
	}
	return body
}

// TestDaemonServesHealthEndpoints is the RUN-10 acceptance test: the
// daemon boots the Echo server on SUPERVISOR_PORT (proving environment
// configuration reaches the daemon), answers curl-able health endpoints
// with the stub probe set, and shuts down cleanly on context cancellation.
func TestDaemonServesHealthEndpoints(t *testing.T) {
	validKeyEnv(t)
	port := freePort(t)
	t.Setenv("SUPERVISOR_DATA_DIR", "/env-data")
	t.Setenv("SUPERVISOR_PORT", strconv.Itoa(port))

	root := NewRootCommand()
	if err := bindFlagsToConfig(root, nil); err != nil {
		t.Fatalf("loading configuration: %v", err)
	}
	if cfg.DataDir != "/env-data" || cfg.Port != port {
		t.Fatalf("environment did not reach the daemon configuration: %+v", cfg)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runDaemonContext(ctx) }()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)

	if body := getHealth(t, base+"/healthz"); body.Status != "healthy" || body.Checks["db"] != "ok" {
		t.Errorf("GET /healthz = %+v, want status healthy with db ok", body)
	}
	want := map[string]string{"db": "ok", "docker": "ok", "auditor": "ok"}
	if body := getHealth(t, base+"/readyz"); body.Status != "ready" || fmt.Sprint(body.Checks) != fmt.Sprint(want) {
		t.Errorf("GET /readyz = %+v, want status ready with stub checks %v", body, want)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("daemon returned %v after shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not shut down after context cancellation")
	}
}
