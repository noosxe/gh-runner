package cron

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// ErrInvalidSchedule indicates that the provided cron expression is malformed or unsupported.
var ErrInvalidSchedule = errors.New("invalid cron schedule expression")

// standardParser parses standard 5-field cron expressions (minute, hour, dom, month, dow)
// as well as standard descriptors (@yearly, @monthly, @weekly, @daily, @hourly).
var standardParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// ParseSchedule parses a standard 5-field cron expression into a cron.Schedule.
func ParseSchedule(expr string) (cron.Schedule, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: schedule cannot be empty", ErrInvalidSchedule)
	}

	sched, err := standardParser.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSchedule, err)
	}
	return sched, nil
}

// ComputeNextRun parses a standard cron schedule expression and returns the next execution time
// relative to the given reference time, guaranteed to be in UTC.
func ComputeNextRun(expr string, from time.Time) (time.Time, error) {
	sched, err := ParseSchedule(expr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(from.UTC()).UTC(), nil
}
