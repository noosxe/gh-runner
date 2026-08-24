package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// newTestSwap mirrors the process root before its first Setup: a swap
// handler over a fresh bootstrap buffer.
func newTestSwap() *swapHandler {
	return &swapHandler{delegate: newBootstrapBuffer()}
}

// debugJSONHandler builds the handler Setup would install for explicit
// debug mode (floor lowered to trace) writing to buf.
func debugJSONHandler(t *testing.T, buf *bytes.Buffer) slog.Handler {
	t.Helper()
	handler, err := newHandler(Options{Writer: buf}, LevelTrace)
	if err != nil {
		t.Fatalf("building debug handler failed: %v", err)
	}
	return handler
}

// TestBootstrapRecordsSurviveSetup verifies the bootstrap contract:
// records emitted before Setup — including the config loader's debug
// ones — are buffered and replayed through the configured sink, with
// their module attributes and original order intact.
func TestBootstrapRecordsSurviveSetup(t *testing.T) {
	swap := newTestSwap()
	logger := slog.New(swap).With(slog.String("module", "config"))

	logger.Log(context.Background(), LevelTrace, "early trace")
	logger.Debug("early debug")
	logger.Info("early info")

	var buf bytes.Buffer
	swap.swap(debugJSONHandler(t, &buf))

	records := decode(t, buf.String())
	if len(records) != 3 {
		t.Fatalf("want 3 drained records, got %d:\n%s", len(records), buf.String())
	}
	want := []struct{ msg, level string }{
		{"early trace", "TRACE"},
		{"early debug", "DEBUG"},
		{"early info", "INFO"},
	}
	for i, w := range want {
		if records[i]["msg"] != w.msg || records[i]["level"] != w.level {
			t.Errorf("drained record %d = %v, want msg=%q level=%q", i, records[i], w.msg, w.level)
		}
		if records[i]["module"] != "config" {
			t.Errorf("drained record %d lost its module attribute: %v", i, records[i])
		}
	}
}

// TestBootstrapDrainAppliesLevelFilter verifies the drain respects the
// configured level: at info, buffered debug and trace records are
// dropped, not replayed.
func TestBootstrapDrainAppliesLevelFilter(t *testing.T) {
	swap := newTestSwap()
	logger := slog.New(swap).With(slog.String("module", "config"))
	logger.Debug("early debug")
	logger.Info("early info")

	var buf bytes.Buffer
	handler, err := newHandler(Options{Writer: &buf}, slog.LevelInfo)
	if err != nil {
		t.Fatalf("building info handler failed: %v", err)
	}
	swap.swap(handler)

	found := messages(decode(t, buf.String()))
	if !found["early info"] {
		t.Errorf("info record missing from the drain:\n%s", buf.String())
	}
	if found["early debug"] {
		t.Error("debug record leaked through the info-level drain")
	}
}

// TestBootstrapGroupSemantics verifies attribute scoping of pre-Setup
// derived loggers: attrs added after WithGroup nest inside the group in
// the drained records, exactly as with a directly bound handler.
func TestBootstrapGroupSemantics(t *testing.T) {
	swap := newTestSwap()
	grouped := slog.New(swap).With(slog.String("module", "config")).
		WithGroup("layers").With("file", "supervisor.yaml")
	grouped.Info("layered")

	var buf bytes.Buffer
	swap.swap(debugJSONHandler(t, &buf))

	record := decode(t, buf.String())[0]
	layers, ok := record["layers"].(map[string]any)
	if !ok || layers["file"] != "supervisor.yaml" {
		t.Errorf("grouped attributes misplaced in drained record: %v", record)
	}
	if record["module"] != "config" {
		t.Errorf("module attribute lost: %v", record)
	}
}

// TestBootstrapBufferCapsRecords verifies the buffer keeps only the
// newest records under overflow.
func TestBootstrapBufferCapsRecords(t *testing.T) {
	buffered := &bufferedHandler{cap: 2}
	logger := slog.New(buffered)
	for i := range 4 {
		logger.Info("record", "i", i)
	}

	var buf bytes.Buffer
	buffered.drain(debugJSONHandler(t, &buf))

	records := decode(t, buf.String())
	if len(records) != 2 {
		t.Fatalf("want 2 retained records, got %d:\n%s", len(records), buf.String())
	}
	if records[0]["i"] != float64(2) || records[1]["i"] != float64(3) {
		t.Errorf("buffer did not keep the newest records: %v", records)
	}
}

// TestSwapWithoutBufferIsPlain verifies swapping between configured
// handlers never buffers: post-Setup records go straight to the active
// handler.
func TestSwapWithoutBufferIsPlain(t *testing.T) {
	swap := newTestSwap()
	var bootstrap bytes.Buffer
	swap.swap(debugJSONHandler(t, &bootstrap))
	logger := slog.New(swap).With(slog.String("module", "late"))

	var reconfigured bytes.Buffer
	swap.swap(debugJSONHandler(t, &reconfigured))
	logger.Info("after both swaps")

	if strings.Contains(bootstrap.String(), "after both swaps") {
		t.Errorf("record leaked to a retired handler:\n%s", bootstrap.String())
	}
	if !strings.Contains(reconfigured.String(), "after both swaps") {
		t.Errorf("record missing from the active handler:\n%s", reconfigured.String())
	}
}
