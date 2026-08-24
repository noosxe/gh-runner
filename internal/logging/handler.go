package logging

import (
	"context"
	"log/slog"
	"sync"
)

// swapHandler is a fixed slog.Handler whose delegate can be replaced by
// Setup. Backend modules capture their loggers at package init — long
// before configuration is loaded — so the process-wide root stays put and
// only its delegate is swapped: every For-derived logger (and the slog
// default) reconfigures without a single call site changing.
type swapHandler struct {
	mu       sync.RWMutex
	delegate slog.Handler
}

// swap installs a new delegate, draining the bootstrap buffer (if the old
// delegate was one) through the new handler so pre-Setup records reach
// the configured sink.
func (s *swapHandler) swap(handler slog.Handler) {
	s.mu.Lock()
	old := s.delegate
	s.delegate = handler
	s.mu.Unlock()
	if buffered, ok := old.(*bufferedHandler); ok {
		buffered.drain(handler)
	}
}

// current returns the active delegate.
func (s *swapHandler) current() slog.Handler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.delegate
}

func (s *swapHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return s.current().Enabled(ctx, level)
}

func (s *swapHandler) Handle(ctx context.Context, record slog.Record) error {
	return s.current().Handle(ctx, record)
}

// WithAttrs returns a handler that defers binding the attributes until
// Handle time (see deferredHandler): binding them onto the current
// delegate now would freeze the pre-Setup handler forever.
func (s *swapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return s
	}
	return deferredHandler{parent: s, ops: []bindOp{{attrs: cloneAttrs(attrs)}}}
}

// WithGroup returns a handler that defers the group nesting until Handle
// time, for the same reason as WithAttrs.
func (s *swapHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return s
	}
	return deferredHandler{parent: s, ops: []bindOp{{group: name}}}
}

// bindOp is one deferred WithGroup (group non-empty) or WithAttrs step.
type bindOp struct {
	group string
	attrs []slog.Attr
}

// deferredHandler replays its WithGroup/WithAttrs chain onto the swap
// handler's current delegate at each call. That per-record rebinding is
// the price of swap-awareness; it costs one small handler allocation per
// emitted record, negligible for the supervisor's log volume, and keeps
// attribute/group semantics identical to a directly bound handler.
type deferredHandler struct {
	parent *swapHandler
	ops    []bindOp
}

func (d deferredHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Attributes and groups never change the level, so the bare delegate
	// decides.
	return d.parent.current().Enabled(ctx, level)
}

func (d deferredHandler) Handle(ctx context.Context, record slog.Record) error {
	handler := d.parent.current()
	for _, op := range d.ops {
		if op.group != "" {
			handler = handler.WithGroup(op.group)
		} else {
			handler = handler.WithAttrs(op.attrs)
		}
	}
	return handler.Handle(ctx, record)
}

func (d deferredHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return d
	}
	ops := make([]bindOp, len(d.ops), len(d.ops)+1)
	copy(ops, d.ops)
	ops = append(ops, bindOp{attrs: cloneAttrs(attrs)})
	d.ops = ops
	return d
}

func (d deferredHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return d
	}
	ops := make([]bindOp, len(d.ops), len(d.ops)+1)
	copy(ops, d.ops)
	ops = append(ops, bindOp{group: name})
	d.ops = ops
	return d
}

// cloneAttrs copies an attribute slice so later mutations of the caller's
// slice (e.g. append reuse) cannot leak into deferred handlers.
func cloneAttrs(attrs []slog.Attr) []slog.Attr {
	out := make([]slog.Attr, len(attrs))
	copy(out, attrs)
	return out
}
