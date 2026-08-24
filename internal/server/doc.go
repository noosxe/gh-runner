// Package server hosts the supervisor's Echo v5 web server. RUN-10 bootstraps
// the instance with the GET /healthz (liveness) and GET /readyz (readiness)
// endpoints defined by OQ #19, backed by a registry of named Check probes
// that later milestones populate with real dependencies: the database in M2,
// Docker reachability and the auditor in M5, the control loop in M6. The
// ConnectRPC handlers, JWT cookie sessions, security middleware, and the
// embedded SPA arrive in M7 (RUN-44/RUN-50).
package server
