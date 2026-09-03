package orchestrator

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrDaemonDegraded is returned when container spawning is paused due to an unreachable Docker daemon (OQ #5).
	ErrDaemonDegraded = errors.New("docker daemon is unreachable: container spawning paused in degraded state")
)

const (
	// PersistentDockerDownThreshold is the duration after which an unreachable
	// Docker daemon triggers an alert event for the UI (docs/open-questions.md OQ #5).
	PersistentDockerDownThreshold = 5 * time.Minute

	// DefaultLogCooldown is the rate-limit interval to prevent log spam when
	// the daemon is repeatedly unreachable.
	DefaultLogCooldown = 30 * time.Second

	// DefaultMonitorInterval is the default periodic health check interval.
	DefaultMonitorInterval = 5 * time.Second
)

// DockerAlert represents an alert emitted when the Docker daemon has been unreachable
// for greater than 5 minutes (OQ #5).
type DockerAlert struct {
	HostID    string        `json:"host_id"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration"`
	Message   string        `json:"message"`
}

// DockerHealthTracker monitors daemon reachability, tracks degraded mode,
// rate-limits error logging to prevent log spam, and emits alerts when down > 5m.
type DockerHealthTracker struct {
	mu           sync.RWMutex
	hostID       string
	downSince    time.Time
	isDown       bool
	alertRaised  bool
	lastLogged   time.Time
	logCooldown  time.Duration
	alertHandler func(DockerAlert)
	nowFn        func() time.Time
}

// NewDockerHealthTracker creates a health tracker for a Docker host.
func NewDockerHealthTracker(hostID string, alertHandler func(DockerAlert)) *DockerHealthTracker {
	if hostID == "" {
		hostID = "local"
	}
	return &DockerHealthTracker{
		hostID:       hostID,
		logCooldown:  DefaultLogCooldown,
		alertHandler: alertHandler,
		nowFn:        time.Now,
	}
}

// SetNowFn overrides the time source (useful for testing).
func (t *DockerHealthTracker) SetNowFn(fn func() time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if fn != nil {
		t.nowFn = fn
	}
}

// SetLogCooldown overrides the rate-limit log cooldown.
func (t *DockerHealthTracker) SetLogCooldown(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.logCooldown = d
}

// RecordFailure records a failed ping or operation against the daemon.
// Returns (downDuration, shouldLog, raisedAlert).
func (t *DockerHealthTracker) RecordFailure(err error) (time.Duration, bool, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.nowFn()
	if !t.isDown {
		t.isDown = true
		t.downSince = now
		t.alertRaised = false
	}

	downDuration := now.Sub(t.downSince)
	shouldLog := false
	if t.lastLogged.IsZero() || now.Sub(t.lastLogged) >= t.logCooldown {
		t.lastLogged = now
		shouldLog = true
	}

	raisedAlert := false
	if downDuration >= PersistentDockerDownThreshold && !t.alertRaised {
		t.alertRaised = true
		raisedAlert = true
		if t.alertHandler != nil {
			go t.alertHandler(DockerAlert{
				HostID:    t.hostID,
				StartedAt: t.downSince,
				Duration:  downDuration,
				Message:   fmt.Sprintf("Docker daemon %q unreachable for >5 minutes (%v)", t.hostID, downDuration.Round(time.Second)),
			})
		}
	}

	return downDuration, shouldLog, raisedAlert
}

// RecordSuccess marks the daemon as healthy and resets the degraded state.
// Returns true if the daemon just transitioned from degraded to healthy (recovery).
func (t *DockerHealthTracker) RecordSuccess() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	recovered := t.isDown
	t.isDown = false
	t.downSince = time.Time{}
	t.alertRaised = false
	return recovered
}

// IsDegraded reports whether the daemon is currently degraded/unreachable.
func (t *DockerHealthTracker) IsDegraded() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.isDown
}

// DownDuration returns how long the daemon has been continually down.
func (t *DockerHealthTracker) DownDuration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.isDown {
		return 0
	}
	return t.nowFn().Sub(t.downSince)
}

// CanSpawn checks if container spawning is allowed. Returns ErrDaemonDegraded if degraded.
func (t *DockerHealthTracker) CanSpawn() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.isDown {
		return ErrDaemonDegraded
	}
	return nil
}

// Status returns "degraded" if down, otherwise "ok".
func (t *DockerHealthTracker) Status() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.isDown {
		return "degraded"
	}
	return "ok"
}
