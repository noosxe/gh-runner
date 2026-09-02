package provider

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// MaxBackoffCap is the 15-minute maximum backoff cap per docs/open-questions.md #5.
	MaxBackoffCap = 15 * time.Minute
	// PersistentAlertThreshold is the 10-minute threshold after which rate limits raise UI alert events.
	PersistentAlertThreshold = 10 * time.Minute
	// DefaultBaseDelay is the default starting exponential backoff delay.
	DefaultBaseDelay = 1 * time.Second
	// DefaultMaxRetries is the maximum number of retry attempts per request.
	DefaultMaxRetries = 4
)

var (
	// ErrContextCanceled is returned when context expires while backing off.
	ErrContextCanceled = errors.New("rate-limit backoff canceled by context")
)

// RateLimitAlert represents an active or triggered rate-limit alert event.
type RateLimitAlert struct {
	Provider  string        `json:"provider"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration"`
	Message   string        `json:"message"`
}

// AlertHandler receives persistent rate-limit notifications.
type AlertHandler func(alert RateLimitAlert)

// RateLimitTracker manages rate limit state and persistent alert triggers across providers.
type RateLimitTracker struct {
	mu           sync.RWMutex
	startedAt    map[string]time.Time
	alertRaised  map[string]bool
	alertHandler AlertHandler
	nowFn        func() time.Time
}

// NewRateLimitTracker creates a new rate-limit state tracker.
func NewRateLimitTracker(handler AlertHandler) *RateLimitTracker {
	return &RateLimitTracker{
		startedAt:    make(map[string]time.Time),
		alertRaised:  make(map[string]bool),
		alertHandler: handler,
		nowFn:        time.Now,
	}
}

// SetNowFn allows overriding the current time function (useful for tests).
func (t *RateLimitTracker) SetNowFn(fn func() time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if fn != nil {
		t.nowFn = fn
	}
}

// RecordRateLimit records that a provider is currently rate-limited and triggers alerts if >10m.
func (t *RateLimitTracker) RecordRateLimit(providerName string) (time.Duration, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.nowFn()
	start, exists := t.startedAt[providerName]
	if !exists {
		start = now
		t.startedAt[providerName] = start
		t.alertRaised[providerName] = false
	}

	duration := now.Sub(start)
	raisedAlert := false
	if duration >= PersistentAlertThreshold && !t.alertRaised[providerName] {
		t.alertRaised[providerName] = true
		raisedAlert = true
		if t.alertHandler != nil {
			go t.alertHandler(RateLimitAlert{
				Provider:  providerName,
				StartedAt: start,
				Duration:  duration,
				Message:   fmt.Sprintf("Provider %q rate-limited for >10 minutes (%v)", providerName, duration.Round(time.Second)),
			})
		}
	}

	return duration, raisedAlert
}

// RecordSuccess clears the rate-limited state for a provider upon a successful request.
func (t *RateLimitTracker) RecordSuccess(providerName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.startedAt, providerName)
	delete(t.alertRaised, providerName)
}

// IsRateLimited returns true if the provider is actively tracked as rate-limited.
func (t *RateLimitTracker) IsRateLimited(providerName string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.startedAt[providerName]
	return exists
}

// DefaultTracker is the global default rate-limit tracker.
var DefaultTracker = NewRateLimitTracker(nil)

// RateLimitTransport wraps an http.RoundTripper with rate-limit and backoff retry logic.
type RateLimitTransport struct {
	Base         http.RoundTripper
	ProviderName string
	BaseDelay    time.Duration
	MaxBackoff   time.Duration
	MaxRetries   int
	Tracker      *RateLimitTracker
	SleepFn      func(ctx context.Context, d time.Duration) error
	NowFn        func() time.Time
}

// NewRateLimitTransport creates a new RateLimitTransport for a given provider.
func NewRateLimitTransport(providerName string, base http.RoundTripper) *RateLimitTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &RateLimitTransport{
		Base:         base,
		ProviderName: providerName,
		BaseDelay:    DefaultBaseDelay,
		MaxBackoff:   MaxBackoffCap,
		MaxRetries:   DefaultMaxRetries,
		Tracker:      DefaultTracker,
		SleepFn: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
		NowFn: time.Now,
	}
}

// RoundTrip executes an HTTP request with automatic rate-limit detection and exponential/header backoff.
func (t *RateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		// Reset body if replaying request with body
		if attempt > 0 && req.GetBody != nil {
			body, getErr := req.GetBody()
			if getErr != nil {
				return nil, fmt.Errorf("resetting request body for retry: %w", getErr)
			}
			req.Body = body
		}

		resp, err = t.Base.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if !t.isRateLimited(resp) {
			if t.Tracker != nil {
				t.Tracker.RecordSuccess(t.ProviderName)
			}
			return resp, nil
		}

		// Rate limit detected
		if t.Tracker != nil {
			t.Tracker.RecordRateLimit(t.ProviderName)
		}

		if attempt == t.MaxRetries {
			// Return response without retrying further on max retries reached (no process crash)
			return resp, nil
		}

		delay := t.calculateDelay(resp, attempt)
		_ = resp.Body.Close()

		if sleepErr := t.SleepFn(req.Context(), delay); sleepErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrContextCanceled, sleepErr)
		}
	}

	return resp, err
}

func (t *RateLimitTransport) isRateLimited(resp *http.Response) bool {
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if resp.StatusCode == http.StatusForbidden {
		// GitHub returns 403 Forbidden with X-RateLimit-Remaining: 0 on rate limit
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return true
		}
		if resp.Header.Get("Retry-After") != "" {
			return true
		}
	}
	return false
}

func (t *RateLimitTransport) calculateDelay(resp *http.Response, attempt int) time.Duration {
	now := t.NowFn()

	// 1. Honor Retry-After header (integer seconds or HTTP-Date)
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds > 0 {
			return t.clampDelay(time.Duration(seconds) * time.Second)
		}
		if targetTime, err := http.ParseTime(retryAfter); err == nil {
			diff := targetTime.Sub(now)
			if diff > 0 {
				return t.clampDelay(diff)
			}
		}
	}

	// 2. Honor GitHub X-RateLimit-Reset (Unix timestamp in seconds)
	if resetHeader := resp.Header.Get("X-RateLimit-Reset"); resetHeader != "" {
		if resetUnix, err := strconv.ParseInt(strings.TrimSpace(resetHeader), 10, 64); err == nil && resetUnix > 0 {
			targetTime := time.Unix(resetUnix, 0)
			diff := targetTime.Sub(now)
			if diff > 0 {
				return t.clampDelay(diff)
			}
		}
	}

	// 3. Exponential backoff with jitter for Gitea / Forgejo and generic 429s
	base := t.BaseDelay
	if base <= 0 {
		base = DefaultBaseDelay
	}

	multiplier := 1 << attempt
	backoff := base * time.Duration(multiplier)

	// Add random jitter between 0% and 25%
	jitterFrac := cryptoRandFloat() * 0.25
	jitter := time.Duration(float64(backoff) * jitterFrac)
	backoff += jitter

	return t.clampDelay(backoff)
}

func (t *RateLimitTransport) clampDelay(d time.Duration) time.Duration {
	maxCap := t.MaxBackoff
	if maxCap <= 0 {
		maxCap = MaxBackoffCap
	}
	if d > maxCap {
		return maxCap
	}
	if d < 100*time.Millisecond {
		return 100 * time.Millisecond
	}
	return d
}

func cryptoRandFloat() float64 {
	nBig, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return 0.1
	}
	return float64(nBig.Int64()) / 10000.0
}
