package cron

import (
	"errors"
	"testing"
	"time"
)

func TestParseSchedule_Valid(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{name: "standard 5-field daily at 2am", expr: "0 2 * * *"},
		{name: "every 15 minutes", expr: "*/15 * * * *"},
		{name: "hourly", expr: "0 * * * *"},
		{name: "specific weekdays", expr: "30 9 * * 1-5"},
		{name: "descriptor daily", expr: "@daily"},
		{name: "descriptor hourly", expr: "@hourly"},
		{name: "with surrounding whitespace", expr: "  0 3 * * *  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched, err := ParseSchedule(tt.expr)
			if err != nil {
				t.Fatalf("ParseSchedule(%q) unexpected error: %v", tt.expr, err)
			}
			if sched == nil {
				t.Fatalf("ParseSchedule(%q) returned nil schedule", tt.expr)
			}
		})
	}
}

func TestParseSchedule_Invalid(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{name: "empty string", expr: ""},
		{name: "whitespace only", expr: "   "},
		{name: "invalid text", expr: "everyday"},
		{name: "too few fields", expr: "* * *"},
		{name: "out of bounds minute", expr: "65 * * * *"},
		{name: "out of bounds hour", expr: "0 25 * * *"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched, err := ParseSchedule(tt.expr)
			if err == nil {
				t.Fatalf("ParseSchedule(%q) expected error, got nil", tt.expr)
			}
			if !errors.Is(err, ErrInvalidSchedule) {
				t.Fatalf("expected ErrInvalidSchedule, got %v", err)
			}
			if sched != nil {
				t.Fatalf("expected nil schedule on error, got %v", sched)
			}
		})
	}
}

func TestComputeNextRun(t *testing.T) {
	ref := time.Date(2026, 9, 4, 10, 15, 0, 0, time.UTC)

	t.Run("daily at 2am next run is tomorrow", func(t *testing.T) {
		next, err := ComputeNextRun("0 2 * * *", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC)
		if !next.Equal(expected) {
			t.Fatalf("expected %v, got %v", expected, next)
		}
		if next.Location() != time.UTC {
			t.Fatalf("expected UTC location, got %v", next.Location())
		}
	})

	t.Run("every 30 minutes next run is 10:30", func(t *testing.T) {
		next, err := ComputeNextRun("*/30 * * * *", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC)
		if !next.Equal(expected) {
			t.Fatalf("expected %v, got %v", expected, next)
		}
	})

	t.Run("non-UTC reference time is converted to UTC", func(t *testing.T) {
		loc := time.FixedZone("TEST", 4*3600) // UTC+4
		// 14:15 UTC+4 is 10:15 UTC
		refNonUTC := time.Date(2026, 9, 4, 14, 15, 0, 0, loc)
		next, err := ComputeNextRun("*/30 * * * *", refNonUTC)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC)
		if !next.Equal(expected) {
			t.Fatalf("expected %v, got %v", expected, next)
		}
		if next.Location() != time.UTC {
			t.Fatalf("expected UTC location, got %v", next.Location())
		}
	})

	t.Run("invalid expression returns error", func(t *testing.T) {
		_, err := ComputeNextRun("invalid", ref)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
