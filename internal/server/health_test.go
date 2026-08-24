package server

import (
	"context"
	"testing"
)

// fixedCheck is a fake probe with a settable status, satisfying the RUN-10
// acceptance criterion "degraded path testable via fake probe".
type fixedCheck struct {
	name   string
	status Status
}

func (c fixedCheck) Name() string                   { return c.name }
func (c fixedCheck) Check(_ context.Context) Status { return c.status }

// daemonProbeSet mirrors the stub registration the daemon installs
// (cmd/supervisor registerHealthChecks): db liveness+readiness, docker and
// auditor readiness, so the OQ #19 example is exercised verbatim.
func daemonProbeSet(t *testing.T, docker Status) *Health {
	t.Helper()
	h := NewHealth()
	db := fixedCheck{name: "db", status: StatusOK}
	h.RegisterLiveness(db)
	h.RegisterReadiness(db)
	h.RegisterReadiness(fixedCheck{name: "docker", status: docker})
	h.RegisterReadiness(fixedCheck{name: "auditor", status: StatusOK})
	return h
}

// TestLivenessHealthy verifies the liveness aggregate: a passing db check
// reports the process healthy.
func TestLivenessHealthy(t *testing.T) {
	h := daemonProbeSet(t, StatusOK)
	report := h.LivenessReport(context.Background())
	if report.Status != statusHealthy {
		t.Errorf("liveness status = %q, want %q", report.Status, statusHealthy)
	}
	if report.Checks["db"] != string(StatusOK) {
		t.Errorf("db check = %q, want %q", report.Checks["db"], StatusOK)
	}
}

// TestLivenessFailsWhenDBDown verifies the liveness contract: the database
// is required, so a failing db probe makes the process unhealthy (a
// restart candidate), even while every other check passes.
func TestLivenessFailsWhenDBDown(t *testing.T) {
	h := daemonProbeSet(t, StatusOK)
	h.RegisterLiveness(fixedCheck{name: "db", status: StatusFail})

	report := h.LivenessReport(context.Background())
	if report.Status != statusUnhealthy {
		t.Errorf("liveness status = %q, want %q", report.Status, statusUnhealthy)
	}
	if report.Checks["db"] != string(StatusFail) {
		t.Errorf("db check = %q, want %q", report.Checks["db"], StatusFail)
	}
}

// TestLivenessToleratesDegraded verifies that a degraded check does not
// fail liveness: degraded means "operating without a dependency", and the
// process itself remains a valid running instance.
func TestLivenessToleratesDegraded(t *testing.T) {
	h := NewHealth()
	h.RegisterLiveness(fixedCheck{name: "db", status: StatusDegraded})
	if report := h.LivenessReport(context.Background()); report.Status != statusHealthy {
		t.Errorf("liveness status = %q, want %q (degraded tolerated)", report.Status, statusHealthy)
	}
}

// TestReadinessDegradedDockerStillReady pins the OQ #19 example verbatim:
// with Docker unreachable the supervisor reports ready with the
// degradation visible per check.
func TestReadinessDegradedDockerStillReady(t *testing.T) {
	h := daemonProbeSet(t, StatusDegraded)

	report := h.ReadinessReport(context.Background())
	if report.Status != statusReady {
		t.Errorf("readiness status = %q, want %q (degraded docker must stay ready)", report.Status, statusReady)
	}
	want := map[string]string{"db": "ok", "docker": "degraded", "auditor": "ok"}
	for name, status := range want {
		if report.Checks[name] != status {
			t.Errorf("check %q = %q, want %q", name, report.Checks[name], status)
		}
	}
	if len(report.Checks) != len(want) {
		t.Errorf("checks = %v, want exactly %v", report.Checks, want)
	}
}

// TestReadinessFailsWhenDBDown verifies the readiness contract: a failed
// required check (the database) flips readiness to not_ready so no work is
// routed to this instance.
func TestReadinessFailsWhenDBDown(t *testing.T) {
	h := daemonProbeSet(t, StatusOK)
	h.RegisterReadiness(fixedCheck{name: "db", status: StatusFail})

	if report := h.ReadinessReport(context.Background()); report.Status != statusNotReady {
		t.Errorf("readiness status = %q, want %q", report.Status, statusNotReady)
	}
}

// TestReadinessFailsWhenDockerDown verifies that a *failed* docker check
// (as opposed to degraded) does fail readiness: the distinction is the
// whole point of the degraded status.
func TestReadinessFailsWhenDockerDown(t *testing.T) {
	h := daemonProbeSet(t, StatusFail)

	if report := h.ReadinessReport(context.Background()); report.Status != statusNotReady {
		t.Errorf("readiness status = %q, want %q", report.Status, statusNotReady)
	}
}

// TestRegisterReplacesSameName verifies re-registration semantics: a
// component replacing its probe upgrades in place instead of leaving a
// duplicate entry.
func TestRegisterReplacesSameName(t *testing.T) {
	h := NewHealth()
	h.RegisterReadiness(fixedCheck{name: "db", status: StatusFail})
	h.RegisterReadiness(fixedCheck{name: "db", status: StatusOK})

	report := h.ReadinessReport(context.Background())
	if report.Status != statusReady || len(report.Checks) != 1 {
		t.Errorf("re-registration should replace, got status=%q checks=%v", report.Status, report.Checks)
	}
}

// TestProbeReceivesRequestContext verifies the endpoint's context reaches
// the probes, so real probes (M2 db ping, M5 docker ping) can honor
// cancellation and deadlines.
func TestProbeReceivesRequestContext(t *testing.T) {
	type ctxKey struct{}
	got := make(chan context.Context, 1)
	h := NewHealth()
	h.RegisterLiveness(NewCheck("ctx", func(ctx context.Context) Status {
		got <- ctx
		return StatusOK
	}))

	want := context.WithValue(context.Background(), ctxKey{}, "sentinel")
	h.LivenessReport(want)

	select {
	case c := <-got:
		if c.Value(ctxKey{}) != "sentinel" {
			t.Error("probe did not receive the caller's context")
		}
	default:
		t.Fatal("probe was never invoked")
	}
}

// TestEmptyRegistryIsHealthy verifies the zero-probe registry answers with
// an empty checks object ({} — not null) and a satisfied status, keeping
// the JSON shape stable before any milestone registers probes.
func TestEmptyRegistryIsHealthy(t *testing.T) {
	h := NewHealth()

	liveness := h.LivenessReport(context.Background())
	if liveness.Status != statusHealthy || liveness.Checks == nil || len(liveness.Checks) != 0 {
		t.Errorf("empty liveness report = %+v, want status=%q checks={}", liveness, statusHealthy)
	}
	readiness := h.ReadinessReport(context.Background())
	if readiness.Status != statusReady || readiness.Checks == nil || len(readiness.Checks) != 0 {
		t.Errorf("empty readiness report = %+v, want status=%q checks={}", readiness, statusReady)
	}
}

// TestNewCheckAdaptsFunction verifies the NewCheck adapter mirrors the
// interface-implementing types.
func TestNewCheckAdaptsFunction(t *testing.T) {
	c := NewCheck("fn", func(context.Context) Status { return StatusDegraded })
	if c.Name() != "fn" {
		t.Errorf("Name() = %q, want %q", c.Name(), "fn")
	}
	if s := c.Check(context.Background()); s != StatusDegraded {
		t.Errorf("Check() = %q, want %q", s, StatusDegraded)
	}
}
