package server

import (
	"context"
	"sync"
)

// Per-check outcomes (the values of the "checks" object in the endpoint
// bodies, OQ #19).
const (
	// StatusOK reports a fully healthy dependency.
	StatusOK Status = "ok"
	// StatusDegraded reports an unreachable dependency the supervisor can
	// still operate without — today only the Docker socket: pools cannot be
	// reconciled while it is down, but the API and database keep serving,
	// so readiness survives (OQ #19).
	StatusDegraded Status = "degraded"
	// StatusFail reports a required dependency that is unavailable and
	// fails the enclosing endpoint.
	StatusFail Status = "fail"
)

// Aggregate statuses (the top-level "status" field of the endpoint bodies).
const (
	statusHealthy   = "healthy"   // /healthz: every liveness check passed
	statusUnhealthy = "unhealthy" // /healthz: a liveness check failed
	statusReady     = "ready"     // /readyz: serving, degraded checks allowed
	statusNotReady  = "not_ready" // /readyz: a readiness check failed
)

// Status is the outcome of a single health check probe.
type Status string

// Check is a named health probe registered on a Health registry. Probes run
// on every request to /healthz or /readyz and must be safe for concurrent
// use. The ctx carries the request's cancellation and deadline so slow
// dependencies (a hung Docker socket, a stalled database) do not pin the
// endpoint open; probes that need richer diagnostics than a Status log
// through their own module logger.
//
// The first real probes arrive with their owning milestones (RUN-10 stubs
// the interface): db ping in M2 (RUN-12), Docker daemon reachability in M5
// (RUN-30, unreachable = degraded), and the auditor/control-loop heartbeat
// in M5/M6 (RUN-32/RUN-37).
type Check interface {
	// Name is the probe's key in the "checks" object of the report.
	Name() string
	// Check evaluates the dependency and reports its Status.
	Check(ctx context.Context) Status
}

// NewCheck adapts a name and a probe function into a Check.
func NewCheck(name string, probe func(ctx context.Context) Status) Check {
	return funcCheck{name: name, probe: probe}
}

// funcCheck is the Check implementation behind NewCheck.
type funcCheck struct {
	name  string
	probe func(ctx context.Context) Status
}

func (c funcCheck) Name() string                     { return c.name }
func (c funcCheck) Check(ctx context.Context) Status { return c.probe(ctx) }

// Report is the JSON body served by /healthz and /readyz (OQ #19):
//
//	{ "status": "ready", "checks": { "db": "ok", "docker": "degraded", "auditor": "ok" } }
type Report struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Health is the probe registry backing the health endpoints. It holds two
// independent probe sets: liveness checks (/healthz: process alive + DB
// accessible) and readiness checks (/readyz: DB connectivity + auditor and
// control loop running, with Docker degraded tolerated). Components
// register their probes at boot; the same Check may appear in both sets.
type Health struct {
	mu        sync.RWMutex
	liveness  map[string]Check
	readiness map[string]Check
}

// NewHealth returns an empty registry.
func NewHealth() *Health {
	return &Health{
		liveness:  make(map[string]Check),
		readiness: make(map[string]Check),
	}
}

// RegisterLiveness adds c to the liveness set of /healthz. Registering a
// name again replaces the earlier probe, so re-registration at runtime is
// an upgrade, not a duplicate.
func (h *Health) RegisterLiveness(c Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.liveness[c.Name()] = c
}

// RegisterReadiness adds c to the readiness set of /readyz. Registering a
// name again replaces the earlier probe.
func (h *Health) RegisterReadiness(c Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.readiness[c.Name()] = c
}

// LivenessReport evaluates every liveness probe and aggregates the result:
// any failed check makes the process "unhealthy" (a liveness failure means
// the supervisor itself needs a restart); degraded checks stay healthy —
// liveness asks "is the process able to run at all", not "is everything
// perfect".
func (h *Health) LivenessReport(ctx context.Context) Report {
	checks := runChecks(ctx, h.snapshot(h.liveness))
	return newReport(aggregate(checks, statusHealthy, statusUnhealthy), checks)
}

// ReadinessReport evaluates every readiness probe and aggregates the
// result: failed checks make the supervisor "not_ready", while degraded
// checks — by design only the Docker socket (OQ #19) — keep it "ready"
// with the degradation visible per check.
func (h *Health) ReadinessReport(ctx context.Context) Report {
	checks := runChecks(ctx, h.snapshot(h.readiness))
	return newReport(aggregate(checks, statusReady, statusNotReady), checks)
}

// snapshot copies one probe set under the read lock so probes run outside
// it: a probe that (re-)registers synchronously must not deadlock against
// RegisterLiveness/RegisterReadiness.
func (h *Health) snapshot(set map[string]Check) map[string]Check {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]Check, len(set))
	for name, c := range set {
		out[name] = c
	}
	return out
}

// runChecks evaluates every probe and collects its Status.
func runChecks(ctx context.Context, set map[string]Check) map[string]Status {
	checks := make(map[string]Status, len(set))
	for name, c := range set {
		checks[name] = c.Check(ctx)
	}
	return checks
}

// aggregate folds per-check statuses into the endpoint's top-level status.
// okStatus is the aggregate when every check passed (degraded counts as
// passing); failedStatus is the aggregate when any check failed.
func aggregate(checks map[string]Status, okStatus, failedStatus string) string {
	for _, s := range checks {
		if s == StatusFail {
			return failedStatus
		}
	}
	return okStatus
}

// newReport renders a Report with string-valued checks (never a nil map,
// so JSON emits {} rather than null when no probes are registered).
func newReport(status string, checks map[string]Status) Report {
	out := make(map[string]string, len(checks))
	for name, s := range checks {
		out[name] = string(s)
	}
	return Report{Status: status, Checks: out}
}
