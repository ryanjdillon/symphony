package authcheck

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestChecker_HealthyCommand(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := New("echo ok", logger, WithTimeout(5*time.Second))

	s := c.Check(context.Background())
	if !s.Healthy {
		t.Errorf("expected healthy, got error: %s", s.Error)
	}
	if s.Output != "ok" {
		t.Errorf("expected output 'ok', got %q", s.Output)
	}
}

func TestChecker_FailingCommand(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := New("false", logger, WithTimeout(5*time.Second))

	s := c.Check(context.Background())
	if s.Healthy {
		t.Error("expected unhealthy for failing command")
	}
	if s.Error == "" {
		t.Error("expected error message")
	}
}

func TestChecker_AuthFailureInOutput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := New("echo not authenticated", logger, WithTimeout(5*time.Second))

	s := c.Check(context.Background())
	if s.Healthy {
		t.Error("expected unhealthy when output contains auth failure keywords")
	}
}

func TestChecker_OnChangeCallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	var callbackStatus Status
	called := false

	c := New("false", logger,
		WithTimeout(5*time.Second),
		WithOnChange(func(s Status) {
			called = true
			callbackStatus = s
		}),
	)

	// First check: transitions from default healthy to unhealthy
	c.Check(context.Background())
	if !called {
		t.Error("expected onChange callback to be called on state change")
	}
	if callbackStatus.Healthy {
		t.Error("expected callback status to be unhealthy")
	}
}

func TestChecker_OnChangeNotCalledWhenSame(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	callCount := 0
	c := New("echo ok", logger,
		WithTimeout(5*time.Second),
		WithOnChange(func(_ Status) {
			callCount++
		}),
	)

	// Both checks healthy — no state change, no callback
	c.Check(context.Background())
	c.Check(context.Background())
	if callCount != 0 {
		t.Errorf("expected 0 callbacks (no state change), got %d", callCount)
	}
}

func TestChecker_StatusReturnsLatest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := New("echo ok", logger, WithTimeout(5*time.Second))

	// Before any check, status should reflect default
	s := c.Status()
	if s.CheckedAt != (time.Time{}) {
		t.Error("expected zero time before first check")
	}

	c.Check(context.Background())
	s = c.Status()
	if s.CheckedAt.IsZero() {
		t.Error("expected non-zero checked_at after check")
	}
	if !s.Healthy {
		t.Error("expected healthy status")
	}
}

func TestChecker_Timeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := New("sleep 10", logger, WithTimeout(100*time.Millisecond))

	s := c.Check(context.Background())
	if s.Healthy {
		t.Error("expected unhealthy for timed-out command")
	}
}

func TestContainsAuthFailure(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Everything is OK", false},
		{"Not Authenticated", true},
		{"error: token expired", true},
		{"OAuth error: invalid grant", true},
		{"all checks passed", false},
		{"Login Required to continue", true},
	}

	for _, tt := range tests {
		got := containsAuthFailure(tt.input)
		if got != tt.want {
			t.Errorf("containsAuthFailure(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
