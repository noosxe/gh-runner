package logging

import (
	"context"
	"log/slog"
	"sync"
)

// bootstrapBufferCapacity bounds the pre-Setup record queue. Overflow
// drops the oldest records: the buffer only covers the bootstrap window
// (configuration loading), which emits a handful of records.
const bootstrapBufferCapacity = 256

// bufferedHandler is the swap handler's delegate until the first Setup:
// module loggers already exist at package-init time, but the sink and
// level are unknown until configuration is loaded. Records are queued
// here and drained through the configured handler when Setup swaps it
// in, so nothing logged before bootstrap (e.g. the config loader's own
// debug records) is lost. Enabled reports true for every level — the
// drain applies the configured filter, which keeps debug and trace
// records recoverable by running in explicit debug mode.
type bufferedHandler struct {
	mu      sync.Mutex
	cap     int
	records []slog.Record
}

// newBootstrapBuffer builds the pre-Setup delegate for the process root.
func newBootstrapBuffer() *bufferedHandler {
	return &bufferedHandler{cap: bootstrapBufferCapacity}
}

func (b *bufferedHandler) Enabled(context.Context, slog.Level) bool { return true }

func (b *bufferedHandler) Handle(_ context.Context, record slog.Record) error {
	capacity := b.cap
	if capacity <= 0 {
		capacity = bootstrapBufferCapacity
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.records = append(b.records, record)
	if len(b.records) > capacity {
		// Keep the newest records; the buffer is best-effort.
		b.records = b.records[len(b.records)-capacity:]
	}
	return nil
}

func (b *bufferedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return b
	}
	return &bufferedBound{sink: b, ops: []bindOp{{attrs: cloneAttrs(attrs)}}}
}

func (b *bufferedHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return b
	}
	return &bufferedBound{sink: b, ops: []bindOp{{group: name}}}
}

// drain replays queued records through target, which applies its own
// level filter — debug and trace records queued during bootstrap surface
// only when Setup configured an explicit-debug-mode handler. Best
// effort: records racing in after the drain snapshot stay queued (and
// are dropped with the handler), and handler errors have no earlier sink
// to report to.
func (b *bufferedHandler) drain(target slog.Handler) {
	b.mu.Lock()
	pending := b.records
	b.records = nil
	b.mu.Unlock()

	ctx := context.Background()
	for _, record := range pending {
		if target.Enabled(ctx, record.Level) {
			_ = target.Handle(ctx, record)
		}
	}
}

// bufferedBound is a handler derived from bufferedHandler via
// WithAttrs/WithGroup: it folds its op chain into each record before
// queueing, mirroring how a directly bound handler would carry them.
type bufferedBound struct {
	sink *bufferedHandler
	ops  []bindOp
}

func (bb *bufferedBound) Enabled(context.Context, slog.Level) bool { return true }

func (bb *bufferedBound) Handle(ctx context.Context, record slog.Record) error {
	if attrs := replayOps(bb.ops); len(attrs) > 0 {
		record.AddAttrs(attrs...)
	}
	return bb.sink.Handle(ctx, record)
}

func (bb *bufferedBound) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return bb
	}
	ops := make([]bindOp, len(bb.ops), len(bb.ops)+1)
	copy(ops, bb.ops)
	ops = append(ops, bindOp{attrs: cloneAttrs(attrs)})
	return &bufferedBound{sink: bb.sink, ops: ops}
}

func (bb *bufferedBound) WithGroup(name string) slog.Handler {
	if name == "" {
		return bb
	}
	ops := make([]bindOp, len(bb.ops), len(bb.ops)+1)
	copy(ops, bb.ops)
	ops = append(ops, bindOp{group: name})
	return &bufferedBound{sink: bb.sink, ops: ops}
}

// replayOps folds a WithGroup/WithAttrs chain into a single attribute
// slice: attributes following a WithGroup nest inside it, exactly as a
// directly bound handler would scope them.
func replayOps(ops []bindOp) []slog.Attr {
	var out []slog.Attr
	var groups []string
	for _, op := range ops {
		if op.group != "" {
			groups = append(groups, op.group)
			continue
		}
		attrs := op.attrs
		// Wrap the attributes in the pending groups, innermost last.
		for i := len(groups) - 1; i >= 0; i-- {
			attrs = []slog.Attr{{Key: groups[i], Value: slog.GroupValue(attrs...)}}
		}
		out = append(out, attrs...)
	}
	return out
}
