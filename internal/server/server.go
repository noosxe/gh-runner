package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
)

// Options configures New.
type Options struct {
	// Port is the TCP port the HTTP server binds on (config.Port /
	// SUPERVISOR_PORT; 1..65535, enforced by internal/config validation).
	Port int

	// Health is the probe registry served by /healthz and /readyz. nil
	// creates an empty registry that components populate as they land:
	// the DB probe in M2, Docker and auditor probes in M5, the control
	// loop in M6 (OQ #19, RUN-10).
	Health *Health
}

// Server is the supervisor's HTTP server: an Echo v5 instance (docs/06 §1)
// that today serves only the health endpoints. M7 (RUN-44) mounts the
// ConnectRPC handlers and the embedded SPA on this same instance.
type Server struct {
	echo   *echo.Echo
	http   *http.Server
	health *Health
}

// New builds the server and its routes. Construction is infallible: routes
// are static, and bind failures surface from Start.
func New(opts Options) *Server {
	// Echo v5 logs through native slog; route it through this package's
	// module logger so its records carry module="server" like every other
	// supervisor component (RUN-8).
	e := echo.New()
	e.Logger = logger

	s := &Server{
		echo:   e,
		health: opts.Health,
	}
	if s.health == nil {
		s.health = NewHealth()
	}

	e.GET("/healthz", s.handleHealthz)
	e.GET("/readyz", s.handleReadyz)

	s.http = &http.Server{
		Addr:    fmt.Sprintf(":%d", opts.Port),
		Handler: e,
	}
	return s
}

// Health exposes the probe registry so the daemon (and later milestones)
// register their checks without holding a second reference.
func (s *Server) Health() *Health { return s.health }

// Handler exposes the routing http.Handler. Tests drive the endpoints
// through it without binding a port.
func (s *Server) Handler() http.Handler { return s.echo }

// Start binds the configured port and serves until Shutdown is called. It
// blocks; callers run it in its own goroutine. A clean shutdown yields a
// nil return, anything else (port in use, permission denied) the error.
func (s *Server) Start() error {
	logger.Info("http server listening", "addr", s.http.Addr)
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully drains in-flight requests. The ctx bounds the drain
// window; after it expires Shutdown returns ctx.Err() and the caller
// decides whether to force-quit.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		return err
	}
	logger.Info("http server stopped")
	return nil
}

// handleHealthz serves GET /healthz: liveness. The response itself proves
// the process is alive; the registered liveness probes (DB once M2 lands)
// decide the body. Unhealthy answers with 503 so orchestrators restart the
// supervisor rather than routing to it.
func (s *Server) handleHealthz(c *echo.Context) error {
	report := s.health.LivenessReport(c.Request().Context())
	return c.JSON(reportHTTPStatus(report.Status, statusHealthy), report)
}

// handleReadyz serves GET /readyz: readiness. Failed probes answer 503
// (do not route work here yet); degraded probes — the Docker socket being
// down (OQ #19) — keep the endpoint ready and merely flag the check.
func (s *Server) handleReadyz(c *echo.Context) error {
	report := s.health.ReadinessReport(c.Request().Context())
	return c.JSON(reportHTTPStatus(report.Status, statusReady), report)
}

// reportHTTPStatus maps an aggregate status to its response code: 200 when
// the endpoint is satisfied, 503 otherwise.
func reportHTTPStatus(status, okStatus string) int {
	if status != okStatus {
		return http.StatusServiceUnavailable
	}
	return http.StatusOK
}
