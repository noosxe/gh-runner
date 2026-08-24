package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// do executes a request against the server's routing handler without
// binding a port and returns the recorder.
func do(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// body decodes a Report from a recorder body.
func body(t *testing.T, rec *httptest.ResponseRecorder) Report {
	t.Helper()
	var report Report
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decoding response body %q: %v", rec.Body.String(), err)
	}
	return report
}

// TestHealthzServesLiveness verifies GET /healthz answers 200 with the
// healthy aggregate once the liveness probe passes.
func TestHealthzServesLiveness(t *testing.T) {
	h := NewHealth()
	h.RegisterLiveness(fixedCheck{name: "db", status: StatusOK})
	s := New(Options{Port: 8080, Health: h})

	rec := do(t, s, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if report := body(t, rec); report.Status != "healthy" || report.Checks["db"] != "ok" {
		t.Errorf("report = %+v, want status healthy and db ok", report)
	}
}

// TestHealthzUnhealthyAnswers503 verifies a failing liveness probe answers
// 503 so orchestrators (compose healthcheck, load balancers) treat the
// process as a restart candidate.
func TestHealthzUnhealthyAnswers503(t *testing.T) {
	h := NewHealth()
	h.RegisterLiveness(fixedCheck{name: "db", status: StatusFail})
	s := New(Options{Port: 8080, Health: h})

	rec := do(t, s, "/healthz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /healthz = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if report := body(t, rec); report.Status != "unhealthy" {
		t.Errorf("status = %q, want unhealthy", report.Status)
	}
}

// TestReadyzDegradedPathViaFakeProbe is the RUN-10 acceptance test: the
// degraded path is exercised through a fake probe, and the body matches
// the OQ #19 example byte-for-byte.
func TestReadyzDegradedPathViaFakeProbe(t *testing.T) {
	h := daemonProbeSet(t, StatusDegraded)
	s := New(Options{Port: 8080, Health: h})

	rec := do(t, s, "/readyz")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /readyz = %d, want %d: degraded docker must stay ready (body %s)",
			rec.Code, http.StatusOK, rec.Body.String())
	}
	// encoding/json orders struct fields by declaration and map keys
	// alphabetically, so the wire format is deterministic.
	const want = `{"status":"ready","checks":{"auditor":"ok","db":"ok","docker":"degraded"}}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("GET /readyz body = %s, want %s", rec.Body.String(), want)
	}
}

// TestReadyzNotReadyAnswers503 verifies a failed readiness probe answers
// 503 so no work is routed to the instance.
func TestReadyzNotReadyAnswers503(t *testing.T) {
	h := NewHealth()
	h.RegisterReadiness(fixedCheck{name: "db", status: StatusFail})
	s := New(Options{Port: 8080, Health: h})

	rec := do(t, s, "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if report := body(t, rec); report.Status != "not_ready" {
		t.Errorf("status = %q, want not_ready", report.Status)
	}
}

// TestHealthEndpointsProbeStateIsLive verifies each request re-evaluates
// its probes: a status change between two GETs is reflected immediately,
// which is what real M2/M5 probes rely on.
func TestHealthEndpointsProbeStateIsLive(t *testing.T) {
	status := StatusOK
	h := NewHealth()
	h.RegisterLiveness(NewCheck("db", func(context.Context) Status { return status }))
	h.RegisterReadiness(NewCheck("db", func(context.Context) Status { return status }))
	s := New(Options{Port: 8080, Health: h})

	if rec := do(t, s, "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("first GET /healthz = %d, want %d", rec.Code, http.StatusOK)
	}

	status = StatusFail

	if rec := do(t, s, "/healthz"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("second GET /healthz = %d, want %d after probe flipped", rec.Code, http.StatusServiceUnavailable)
	}
	if rec := do(t, s, "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz = %d, want %d after probe flipped", rec.Code, http.StatusServiceUnavailable)
	}
}

// TestUnknownRouteAnswers404 verifies only the two health routes are
// mounted at this milestone (the ConnectRPC handlers arrive in M7).
func TestUnknownRouteAnswers404(t *testing.T) {
	s := New(Options{Port: 8080})
	if rec := do(t, s, "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /nope = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestDefaultHealthRegistryMounted verifies New builds a usable empty
// registry when none is supplied: endpoints answer without probes rather
// than panicking on a nil map.
func TestDefaultHealthRegistryMounted(t *testing.T) {
	s := New(Options{Port: 8080})

	if rec := do(t, s, "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("GET /healthz = %d, want %d with no probes", rec.Code, http.StatusOK)
	}
	if rec := do(t, s, "/readyz"); rec.Code != http.StatusOK {
		t.Errorf("GET /readyz = %d, want %d with no probes", rec.Code, http.StatusOK)
	}
}
