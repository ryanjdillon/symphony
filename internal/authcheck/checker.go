package authcheck

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Status represents the result of an auth health check.
type Status struct {
	Healthy   bool      `json:"healthy"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

// Checker periodically probes OAuth auth health by running a command
// (typically "claude doctor") and tracking whether it succeeds.
type Checker struct {
	command  string
	args     []string
	interval time.Duration
	timeout  time.Duration
	logger   *slog.Logger
	onChange func(Status)

	mu     sync.RWMutex
	status Status
}

// Option configures a Checker.
type Option func(*Checker)

// WithInterval sets the check interval (default: 1 hour).
func WithInterval(d time.Duration) Option {
	return func(c *Checker) { c.interval = d }
}

// WithTimeout sets the command execution timeout (default: 30 seconds).
func WithTimeout(d time.Duration) Option {
	return func(c *Checker) { c.timeout = d }
}

// WithOnChange registers a callback invoked when health status changes.
func WithOnChange(fn func(Status)) Option {
	return func(c *Checker) { c.onChange = fn }
}

// New creates a Checker that runs the given command to probe auth health.
// The command string is split on whitespace (e.g. "claude doctor").
func New(command string, logger *slog.Logger, opts ...Option) *Checker {
	parts := strings.Fields(command)
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	c := &Checker{
		command:  parts[0],
		args:     args,
		interval: 1 * time.Hour,
		timeout:  30 * time.Second,
		logger:   logger.With("component", "authcheck"),
		status: Status{
			Healthy:   true, // assume healthy until first check
			CheckedAt: time.Time{},
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Status returns the most recent check result.
func (c *Checker) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// Check runs the probe command once and updates the status.
func (c *Checker) Check(ctx context.Context) Status {
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, c.command, c.args...)
	output, err := cmd.CombinedOutput()

	s := Status{
		CheckedAt: time.Now(),
		Output:    strings.TrimSpace(string(output)),
	}

	if err != nil {
		s.Healthy = false
		s.Error = fmt.Sprintf("command failed: %v", err)
		c.logger.Warn("auth check failed",
			"command", c.command,
			"error", err,
			"output", s.Output,
		)
	} else {
		// claude doctor exits 0 on success; also check for auth-related
		// failure keywords in the output as a safety net.
		if containsAuthFailure(s.Output) {
			s.Healthy = false
			s.Error = "auth failure detected in command output"
			c.logger.Warn("auth check detected failure in output",
				"output", s.Output,
			)
		} else {
			s.Healthy = true
			c.logger.Debug("auth check passed")
		}
	}

	c.mu.Lock()
	prev := c.status
	c.status = s
	c.mu.Unlock()

	if c.onChange != nil && prev.Healthy != s.Healthy {
		c.onChange(s)
	}

	return s
}

// Run starts the periodic check loop. Blocks until ctx is cancelled.
// Runs an initial check immediately.
func (c *Checker) Run(ctx context.Context) {
	c.logger.Info("auth checker starting",
		"command", c.command,
		"interval", c.interval,
	)

	// Initial check
	c.Check(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("auth checker stopping")
			return
		case <-ticker.C:
			c.Check(ctx)
		}
	}
}

// containsAuthFailure checks command output for signs of auth problems.
func containsAuthFailure(output string) bool {
	lower := strings.ToLower(output)
	patterns := []string{
		"not authenticated",
		"authentication failed",
		"auth error",
		"token expired",
		"oauth error",
		"login required",
		"unauthorized",
		"invalid token",
		"credential",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
