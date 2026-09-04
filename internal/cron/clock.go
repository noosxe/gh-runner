package cron

import (
	"sync"
	"time"
)

// Clock provides an abstraction over time for the cron scheduler,
// allowing tests to use virtual clocks to simulate cron schedules without sleeping.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// RealClock implements Clock using standard library time in UTC.
type RealClock struct{}

// Now returns current time in UTC.
func (RealClock) Now() time.Time {
	return time.Now().UTC()
}

// After returns a channel that receives the current time after duration d.
func (RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

type virtualTimer struct {
	fireAt time.Time
	ch     chan time.Time
}

// VirtualClock is a thread-safe mock clock for testing that simulates the passage of time.
type VirtualClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*virtualTimer
}

// NewVirtualClock creates a VirtualClock set to the given initial UTC time.
func NewVirtualClock(initial time.Time) *VirtualClock {
	return &VirtualClock{
		now: initial.UTC(),
	}
}

// Now returns the current virtual time in UTC.
func (vc *VirtualClock) Now() time.Time {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	return vc.now
}

// After returns a channel that fires when the virtual clock advances to or past now + d.
func (vc *VirtualClock) After(d time.Duration) <-chan time.Time {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- vc.now
		return ch
	}

	vt := &virtualTimer{
		fireAt: vc.now.Add(d),
		ch:     ch,
	}
	vc.timers = append(vc.timers, vt)
	return ch
}

// Advance moves virtual time forward by d and fires any timers whose deadline has passed.
func (vc *VirtualClock) Advance(d time.Duration) {
	vc.mu.Lock()
	vc.now = vc.now.Add(d)
	vc.triggerTimersLocked()
	vc.mu.Unlock()
}

// Set explicitly sets virtual time to t (in UTC) and fires any timers whose deadline has passed.
func (vc *VirtualClock) Set(t time.Time) {
	vc.mu.Lock()
	vc.now = t.UTC()
	vc.triggerTimersLocked()
	vc.mu.Unlock()
}

func (vc *VirtualClock) triggerTimersLocked() {
	var remaining []*virtualTimer
	for _, vt := range vc.timers {
		if !vt.fireAt.After(vc.now) {
			select {
			case vt.ch <- vc.now:
			default:
			}
		} else {
			remaining = append(remaining, vt)
		}
	}
	vc.timers = remaining
}

// WaitersCount returns the number of active pending timers waiting to fire.
func (vc *VirtualClock) WaitersCount() int {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	return len(vc.timers)
}
