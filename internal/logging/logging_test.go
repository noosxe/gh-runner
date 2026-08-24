package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// decode parses newline-delimited JSON log output into records.
func decode(t *testing.T, out string) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line is not valid JSON: %v\n%s", err, line)
		}
		records = append(records, record)
	}
	return records
}

// messages collects the msg values of decoded records.
func messages(records []map[string]any) map[string]bool {
	found := make(map[string]bool, len(records))
	for _, record := range records {
		found[record["msg"].(string)] = true
	}
	return found
}

// recordingHandler is a pluggable sink for tests: it records every record
// in a store shared across WithAttrs derivatives, because the deferred
// handler binding calls WithAttrs per record and discards the derived
// handler.
type recordingHandler struct {
	store *recordStore
	min   slog.Level
	pre   []slog.Attr
}

type recordStore struct {
	mu      sync.Mutex
	records []slog.Record
}

func (r *recordingHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= r.min
}

func (r *recordingHandler) Handle(_ context.Context, record slog.Record) error {
	if len(r.pre) > 0 {
		record.AddAttrs(r.pre...)
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.records = append(r.store.records, record)
	return nil
}

func (r *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := &recordingHandler{store: r.store, min: r.min}
	next.pre = append(append([]slog.Attr(nil), r.pre...), attrs...)
	return next
}

func (r *recordingHandler) WithGroup(string) slog.Handler { return r }

// TestParseLevel verifies the LOG_LEVEL contract parsing, including the
// deliberate rejection of "trace": trace is not a selectable level, it is
// only reachable through explicit debug mode (docs/06 §1).
func TestParseLevel(t *testing.T) {
	cases := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{in: "debug", want: slog.LevelDebug},
		{in: "DEBUG", want: slog.LevelDebug},
		{in: " info ", want: slog.LevelInfo},
		{in: "", want: slog.LevelInfo},
		{in: "warn", want: slog.LevelWarn},
		{in: "Warning", want: slog.LevelWarn},
		{in: "error", want: slog.LevelError},
		{in: "trace", wantErr: true},
		{in: "bogus", wantErr: true},
	}
	for _, tc := range cases {
		got, err := ParseLevel(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseLevel(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseLevel(%q) failed: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestSetupGatesDebugAndTrace verifies the standard-mode filter: info,
// warn, and error records pass while debug and trace records are
// suppressed.
func TestSetupGatesDebugAndTrace(t *testing.T) {
	var buf bytes.Buffer
	if err := Setup(Options{Level: "info", Writer: &buf}); err != nil {
		t.Fatalf("Setup(info) failed: %v", err)
	}

	logger := For("gating")
	logger.Log(context.Background(), LevelTrace, "trace beacon")
	logger.Debug("debug beacon")
	logger.Info("info beacon")
	logger.Warn("warn beacon")
	logger.Error("error beacon")

	found := messages(decode(t, buf.String()))
	for _, msg := range []string{"info beacon", "warn beacon", "error beacon"} {
		if !found[msg] {
			t.Errorf("record %q missing at level info; output:\n%s", msg, buf.String())
		}
	}
	for _, msg := range []string{"trace beacon", "debug beacon"} {
		if found[msg] {
			t.Errorf("record %q must be suppressed at level info", msg)
		}
	}
}

// TestDebugModeUnlocksDebugAndTrace verifies explicit debug mode: selecting
// the debug level lowers the filter floor to trace, and trace records are
// rendered with the stable TRACE label.
func TestDebugModeUnlocksDebugAndTrace(t *testing.T) {
	var buf bytes.Buffer
	if err := Setup(Options{Level: "debug", Writer: &buf}); err != nil {
		t.Fatalf("Setup(debug) failed: %v", err)
	}

	logger := For("debugmode")
	logger.Debug("debug beacon")
	logger.Log(context.Background(), LevelTrace, "trace beacon")

	levels := make(map[string]string)
	for _, record := range decode(t, buf.String()) {
		levels[record["msg"].(string)] = record["level"].(string)
	}
	if got, ok := levels["debug beacon"]; !ok || got != "DEBUG" {
		t.Errorf("debug record missing or mislabeled: got %q, want DEBUG", got)
	}
	if got, ok := levels["trace beacon"]; !ok || got != "TRACE" {
		t.Errorf("trace record missing or mislabeled: got %q, want TRACE", got)
	}
}

// TestForTagsModuleAttribute verifies the mandated per-module pattern: a
// logger from For carries module=<name> on every record, and slog
// attribute/group semantics survive the deferred binding.
func TestForTagsModuleAttribute(t *testing.T) {
	var buf bytes.Buffer
	if err := Setup(Options{Level: "info", Writer: &buf}); err != nil {
		t.Fatalf("Setup(info) failed: %v", err)
	}

	logger := For("orchestrator")
	logger.Info("pool reconciled", "runners", 2)

	records := decode(t, buf.String())
	if len(records) != 1 {
		t.Fatalf("want exactly one record, got %d:\n%s", len(records), buf.String())
	}
	record := records[0]
	if record["module"] != "orchestrator" {
		t.Errorf("record module = %v, want orchestrator", record["module"])
	}
	if record["level"] != "INFO" || record["runners"] != float64(2) {
		t.Errorf("record fields wrong: %v", record)
	}

	// Attribute scoping: attrs added after WithGroup must land inside the
	// group, exactly as with a directly bound handler.
	grouped := logger.WithGroup("docker").With("container", "runner-1")
	var groupBuf bytes.Buffer
	if err := Setup(Options{Level: "info", Writer: &groupBuf}); err != nil {
		t.Fatalf("re-Setup(info) failed: %v", err)
	}
	grouped.Info("spawned")
	record = decode(t, groupBuf.String())[0]
	docker, ok := record["docker"].(map[string]any)
	if !ok || docker["container"] != "runner-1" || docker["module"] != nil {
		t.Errorf("grouped attributes misplaced: %v", record)
	}
}

// TestLoggerCapturedBeforeSetupFollowsReconfiguration verifies the swap
// architecture: a module logger captured before (an earlier) Setup picks
// up the handler and level of the latest Setup.
func TestLoggerCapturedBeforeSetupFollowsReconfiguration(t *testing.T) {
	var first, second bytes.Buffer
	if err := Setup(Options{Level: "info", Writer: &first}); err != nil {
		t.Fatalf("Setup(info) failed: %v", err)
	}
	logger := For("swap")

	if err := Setup(Options{Level: "debug", Writer: &second}); err != nil {
		t.Fatalf("Setup(debug) failed: %v", err)
	}

	logger.Debug("after reconfiguration")

	if strings.Contains(first.String(), "after reconfiguration") {
		t.Errorf("record leaked to the retired handler:\n%s", first.String())
	}
	if !strings.Contains(second.String(), "after reconfiguration") {
		t.Errorf("record missing from the active handler:\n%s", second.String())
	}
}

// TestDefaultSinkIsHonoredBySlogDefault verifies bare slog calls (used by
// dependencies and simple call sites) route through the same configured
// sink after Setup.
func TestDefaultSinkIsHonoredBySlogDefault(t *testing.T) {
	var buf bytes.Buffer
	if err := Setup(Options{Level: "info", Writer: &buf}); err != nil {
		t.Fatalf("Setup(info) failed: %v", err)
	}
	slog.Info("through the default logger")
	if !strings.Contains(buf.String(), "through the default logger") {
		t.Errorf("slog default did not reach the configured sink:\n%s", buf.String())
	}
}

// TestPluggableHandler verifies the sink seam: a caller-provided handler
// receives every record (from For-derived loggers and the slog default)
// and owns its own level filtering regardless of the configured level.
func TestPluggableHandler(t *testing.T) {
	store := &recordStore{}
	custom := &recordingHandler{store: store, min: slog.LevelWarn}
	if err := Setup(Options{Level: "debug", Handler: custom}); err != nil {
		t.Fatalf("Setup(custom handler) failed: %v", err)
	}

	logger := For("custom")
	logger.Info("suppressed by the custom handler's own filter")
	logger.Warn("through the custom handler")
	slog.Error("through the slog default")

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.records) != 2 {
		t.Fatalf("want 2 records in the custom sink, got %d", len(store.records))
	}
	var moduleAttr string
	store.records[0].Attrs(func(a slog.Attr) bool {
		if a.Key == "module" {
			moduleAttr = a.Value.String()
		}
		return true
	})
	if moduleAttr != "custom" {
		t.Errorf("custom sink record module = %q, want custom", moduleAttr)
	}
}

// TestSetupRejectsInvalidOptions verifies configuration errors surface
// before any handler is installed.
func TestSetupRejectsInvalidOptions(t *testing.T) {
	if err := Setup(Options{Level: "bogus"}); err == nil || !strings.Contains(err.Error(), "log level") {
		t.Errorf("Setup with bogus level should fail with a log level error, got %v", err)
	}
	if err := Setup(Options{Level: "info", Format: "xml"}); err == nil || !strings.Contains(err.Error(), "log format") {
		t.Errorf("Setup with bogus format should fail with a log format error, got %v", err)
	}
}

// TestTextFormat verifies the text encoding option of the built-in
// handler, including the TRACE rename.
func TestTextFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := Setup(Options{Level: "debug", Format: FormatText, Writer: &buf}); err != nil {
		t.Fatalf("Setup(text) failed: %v", err)
	}
	For("text").Log(context.Background(), LevelTrace, "trace in text")
	if !strings.Contains(buf.String(), "level=TRACE") || !strings.Contains(buf.String(), "module=text") {
		t.Errorf("text output missing expected fields:\n%s", buf.String())
	}
}
