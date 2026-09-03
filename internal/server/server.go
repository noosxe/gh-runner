package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/labstack/echo/v5"
	"github.com/noosxe/gh-runner/internal/pb/supervisor/v1/supervisorv1connect"
	"github.com/noosxe/gh-runner/web"
)

// DisabledJSONCodec explicitly overrides the default "json" codec in Connect,
// enforcing that JSON payloads are rejected because binary protocol is mandatory (docs/06 §1).
type DisabledJSONCodec struct{}

func (d DisabledJSONCodec) Name() string { return "json" }
func (d DisabledJSONCodec) Marshal(any) ([]byte, error) {
	return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("binary protocol mandatory: JSON transport is disabled per docs/06 §1"))
}
func (d DisabledJSONCodec) Unmarshal([]byte, any) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("binary protocol mandatory: JSON transport is disabled per docs/06 §1"))
}

// BinaryProtocolInterceptor inspects Connect requests to ensure no JSON content-type is permitted.
func BinaryProtocolInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			ct := req.Header().Get("Content-Type")
			if strings.Contains(ct, "json") {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("binary protocol mandatory: JSON transport is disabled per docs/06 §1"))
			}
			return next(ctx, req)
		}
	}
}

// BinaryConnectHandlerOptions returns the standard Connect HandlerOptions enforcing
// mandatory binary protocol transport (docs/06 §1).
func BinaryConnectHandlerOptions() []connect.HandlerOption {
	return []connect.HandlerOption{
		connect.WithCodec(DisabledJSONCodec{}),
		connect.WithInterceptors(connect.UnaryInterceptorFunc(BinaryProtocolInterceptor())),
	}
}

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

	// StaticFS is the filesystem containing built SPA assets (index.html, js, css).
	// If nil, defaults to the embedded web.Dist() filesystem (docs/06 §2, RUN-44).
	StaticFS fs.FS

	// AuthDB is the database interface used for administrator authentication,
	// sessions, and audit logs. If nil, AuthService is not automatically mounted.
	AuthDB AuthDatabase

	// PoolDB is the database interface used for runner pools and audit logs.
	// If nil, PoolService is not automatically mounted.
	PoolDB PoolDatabase

	// PoolStats provides live runtime runner counts and reload capabilities (RUN-46).
	PoolStats PoolStatsProvider

	// JWTSigningSecret is the 256-bit HMAC key derived from SUPERVISOR_DB_ENCRYPTION_KEY
	// used to cryptographically sign session JWT tokens (docs/05 §5, keys.LabelJWTSigning).
	JWTSigningSecret []byte

	// IsSecureCookie sets the Secure attribute on the session cookie. Defaults to false
	// for local development/testing without TLS, set to true behind HTTPS.
	IsSecureCookie bool
}

// Server is the supervisor's HTTP server: an Echo v5 instance (docs/06 §1)
// that serves health endpoints, ConnectRPC services (binary transport mandatory),
// and the embedded SPA.
type Server struct {
	echo      *echo.Echo
	http      *http.Server
	health    *Health
	staticFS  fs.FS
	authDB    AuthDatabase
	jwtSecret []byte
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
		echo:      e,
		health:    opts.Health,
		staticFS:  opts.StaticFS,
		authDB:    opts.AuthDB,
		jwtSecret: opts.JWTSigningSecret,
	}
	if s.health == nil {
		s.health = NewHealth()
	}
	if s.staticFS == nil {
		if dist, err := web.Dist(); err == nil {
			s.staticFS = dist
		}
	}

	// Middleware: enforce binary protocol on Connect RPC routes
	e.Use(s.enforceBinaryTransportMiddleware)

	e.GET("/healthz", s.handleHealthz)
	e.GET("/readyz", s.handleReadyz)

	// Mount AuthService if database and secret are provided (RUN-45)
	if s.authDB != nil && len(s.jwtSecret) > 0 {
		authSvc := NewAuthService(s.authDB, s.jwtSecret, opts.IsSecureCookie)
		path, handler := supervisorv1connect.NewAuthServiceHandler(authSvc, s.ConnectHandlerOptions()...)
		s.MountConnectHandler(path, handler)
	}

	// Mount PoolService if pool database is provided (RUN-46)
	if opts.PoolDB != nil {
		poolSvc := NewPoolService(opts.PoolDB, opts.PoolStats)
		path, handler := supervisorv1connect.NewPoolServiceHandler(poolSvc, s.ConnectHandlerOptions()...)
		s.MountConnectHandler(path, handler)
	}

	// SPA fallback routes (catch-all GET/HEAD, falls back to index.html)
	e.GET("/*", s.serveSPA)
	e.HEAD("/*", s.serveSPA)

	s.http = &http.Server{
		Addr:    fmt.Sprintf(":%d", opts.Port),
		Handler: e,
	}
	return s
}

// ConnectHandlerOptions returns the standard Connect HandlerOptions enforcing binary transport
// and (if configured) authentication interception on protected RPCs.
func (s *Server) ConnectHandlerOptions() []connect.HandlerOption {
	opts := BinaryConnectHandlerOptions()
	if s.authDB != nil && len(s.jwtSecret) > 0 {
		opts = append(opts, connect.WithInterceptors(NewAuthInterceptor(s.authDB, s.jwtSecret)))
	}
	return opts
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

// Echo returns the underlying Echo instance.
func (s *Server) Echo() *echo.Echo {
	return s.echo
}

// MountConnectHandler registers a ConnectRPC service handler on Echo.
func (s *Server) MountConnectHandler(pattern string, handler http.Handler) {
	prefix := strings.TrimSuffix(pattern, "/")
	wrapped := echo.WrapHandler(handler)
	s.echo.Any(prefix, wrapped)
	s.echo.Any(prefix+"/*", wrapped)
}

func (s *Server) serveSPA(c *echo.Context) error {
	if s.staticFS == nil {
		return c.String(http.StatusNotFound, "frontend assets not found")
	}

	reqPath := strings.TrimPrefix(c.Request().URL.Path, "/")
	// Never serve SPA index.html for API / Connect RPC endpoints
	if strings.HasPrefix(reqPath, "supervisor.v1.") || strings.HasPrefix(reqPath, "healthz") || strings.HasPrefix(reqPath, "readyz") {
		return c.String(http.StatusNotFound, "not found")
	}

	if reqPath == "" {
		reqPath = "index.html"
	}

	// If the file exists in staticFS, serve it directly
	if f, err := s.staticFS.Open(reqPath); err == nil {
		_ = f.Close()
		return c.FileFS(reqPath, s.staticFS)
	}

	// Fallback to index.html for client-side routing (TanStack Router)
	return c.FileFS("index.html", s.staticFS)
}

func (s *Server) enforceBinaryTransportMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		path := c.Request().URL.Path
		if strings.HasPrefix(path, "/supervisor.v1.") {
			ct := c.Request().Header.Get("Content-Type")
			if strings.Contains(ct, "json") {
				return c.JSON(http.StatusUnsupportedMediaType, map[string]string{
					"code":    "invalid_argument",
					"message": "binary protocol mandatory: JSON transport is disabled per docs/06 §1",
				})
			}
		}
		return next(c)
	}
}
