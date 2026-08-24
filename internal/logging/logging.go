// Package logging implements the supervisor's structured logging bootstrap
// (RUN-8, docs/06 §1): the native log/slog stack with a stdout sink by
// default, pluggable handlers for other sinks, per-module loggers derived
// via For (the slog.String("module", ...) pattern mandated for every
// backend module), and debug/trace records gated behind explicit debug
// mode. Records emitted before the sink is configured are buffered and
// replayed once Setup installs it.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// LevelTrace is the trace level, one step below slog's Debug. slog has no
// built-in trace level; records emitted at LevelTrace only ever pass the
// filter when the supervisor runs in explicit debug mode (see Setup).
var LevelTrace = slog.Level(-8)

// levelNames renames levels for the built-in handlers so trace records
// render as "TRACE" instead of slog's synthesized "DEBUG-4", keeping output
// greppable and machine-parsable.
var levelNames = map[slog.Level]string{
	LevelTrace:      "TRACE",
	slog.LevelDebug: "DEBUG",
	slog.LevelInfo:  "INFO",
	slog.LevelWarn:  "WARN",
	slog.LevelError: "ERROR",
}

// Format selects the encoding of the built-in handler (docs/06 §1:
// structured logging for containerized consumption, so JSON is the
// default). Ignored when Options.Handler is set.
type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

// Options configures Setup.
type Options struct {
	// Level is the supervisor's LOG_LEVEL contract value: debug, info,
	// warn, or error (case-insensitive; "warning" accepted). Empty
	// defaults to info. Selecting debug is "explicit debug mode" — the
	// filter floor drops to LevelTrace so debug and trace records pass
	// (docs/06 §1); otherwise both stay suppressed.
	Level string

	// Format picks the built-in handler's encoding; empty defaults to
	// FormatJSON. Ignored when Handler is set.
	Format Format

	// Writer is the sink of the built-in handler. nil means os.Stdout —
	// the primary sink, standard for containerized applications. Ignored
	// when Handler is set.
	Writer io.Writer

	// Handler, when non-nil, replaces the built-in handler entirely. This
	// is the pluggable-sink seam of docs/06 §1: future file, syslog, or
	// OTLP sinks are dropped in here without touching any call site.
	Handler slog.Handler
}

// ParseLevel converts a LOG_LEVEL contract value into a slog.Level. It is
// case-insensitive and accepts "warning" as "warn", mirroring the config
// normalizer; an empty value parses as the info default so zero-value
// Options behave sensibly.
func ParseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q: want debug, info, warn, or error", value)
	}
}

// root is the fixed process-wide handler every For-derived logger — and,
// after Setup, the slog default — logs through. Until the first Setup its
// delegate is a bounded bootstrap buffer: module loggers already exist at
// package-init time, but the sink and level are unknown until
// configuration is loaded, so early records are queued and drained
// through the configured handler by Setup (see swapHandler and
// bufferedHandler).
var root = &swapHandler{delegate: newBootstrapBuffer()}

// Setup bootstraps process-wide logging: it builds (or accepts via
// Options.Handler) a handler honoring the configured level, installs it
// behind every logger this package hands out, and makes it the slog
// default so bare slog calls follow suit. The first call also drains the
// bootstrap buffer through the new handler, replaying records emitted
// before configuration was loaded. It is safe to call repeatedly (tests
// and reconfiguration); the last call wins.
func Setup(opts Options) error {
	level, err := ParseLevel(opts.Level)
	if err != nil {
		return err
	}

	// Explicit debug mode: the contract's standard levels are info, warn,
	// and error; debug and trace records are only emitted when the
	// operator explicitly selects debug, and that selection lowers the
	// filter floor past Debug down to Trace.
	min := level
	if level == slog.LevelDebug {
		min = LevelTrace
	}

	var handler slog.Handler
	if opts.Handler != nil {
		handler = opts.Handler
	} else {
		handler, err = newHandler(opts, min)
		if err != nil {
			return err
		}
	}

	root.swap(handler)
	slog.SetDefault(slog.New(root))
	return nil
}

// newHandler builds the standard sink: a JSON (or text) slog handler over
// Options.Writer with level filtering and level renaming applied.
func newHandler(opts Options, min slog.Level) (slog.Handler, error) {
	writer := opts.Writer
	if writer == nil {
		writer = os.Stdout
	}

	handlerOpts := &slog.HandlerOptions{
		Level:       min,
		ReplaceAttr: renameLevels,
	}

	switch opts.Format {
	case "", FormatJSON:
		return slog.NewJSONHandler(writer, handlerOpts), nil
	case FormatText:
		return slog.NewTextHandler(writer, handlerOpts), nil
	default:
		return nil, fmt.Errorf("invalid log format %q: want %q or %q", opts.Format, FormatJSON, FormatText)
	}
}

// renameLevels rewrites the level attribute of the built-in handlers so
// custom levels carry stable names (TRACE for LevelTrace) instead of
// slog's "DEBUG-4" synthesis.
func renameLevels(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key != slog.LevelKey || attr.Value.Kind() != slog.KindAny {
		return attr
	}
	if level, ok := attr.Value.Any().(slog.Level); ok {
		if name, known := levelNames[level]; known {
			attr.Value = slog.StringValue(name)
		}
	}
	return attr
}

// For returns the logger of a backend module, tagged with
// slog.String("module", name) as mandated by docs/06 §1, e.g.
// logging.For("orchestrator"). It is cheap and safe to call at package
// init time, before Setup: the returned logger follows whatever handler
// Setup installs later.
func For(module string) *slog.Logger {
	return slog.New(root).With(slog.String("module", module))
}
