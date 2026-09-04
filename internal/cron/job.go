package cron

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
)

// MissedFirePolicy controls how the scheduler handles jobs whose scheduled fire times
// were missed while the supervisor was down or restarting (docs/03 §5).
type MissedFirePolicy int

const (
	// MissedFirePolicyRunImmediately triggers the task immediately upon registration/restart
	// if an expected scheduled fire was missed.
	MissedFirePolicyRunImmediately MissedFirePolicy = iota
	// MissedFirePolicySkip logs that an expected fire was missed and skips straight to the
	// next future scheduled run without executing.
	MissedFirePolicySkip
)

// TaskFunc defines the executable payload for a scheduled cron job.
type TaskFunc func(ctx context.Context) error

// JobConfig defines configuration for registering a scheduled job.
type JobConfig struct {
	// PoolID is the unique pool identifier associated with this scheduled job.
	PoolID int64
	// Schedule is a standard 5-field cron expression (e.g., "0 2 * * *").
	Schedule string
	// LastRun is the timestamp when this job was last executed (zero if never run).
	LastRun time.Time
	// MissedFirePolicy specifies behavior when a scheduled run was missed during downtime.
	MissedFirePolicy MissedFirePolicy
	// Task is the function to execute when the schedule triggers.
	Task TaskFunc
}

// JobInfo exposes runtime status and scheduling metadata for a registered job.
type JobInfo struct {
	PoolID           int64            `json:"pool_id"`
	Schedule         string           `json:"schedule"`
	NextRun          time.Time        `json:"next_run"`
	LastRun          time.Time        `json:"last_run"`
	IsRunning        bool             `json:"is_running"`
	MissedFirePolicy MissedFirePolicy `json:"missed_fire_policy"`
}

// jobEntry represents an internal registered job inside the scheduler.
type jobEntry struct {
	poolID           int64
	expr             string
	schedule         cron.Schedule
	task             TaskFunc
	lastRun          time.Time
	nextRun          time.Time
	running          bool
	missedFirePolicy MissedFirePolicy
	pendingImmediate bool
}
