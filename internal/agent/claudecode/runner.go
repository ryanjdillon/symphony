package claudecode

import (
	"context"
	"log/slog"

	"github.com/ryanjdillon/symphony/internal/agent"
)

// Runner implements agent.Runner for Claude Code's app-server protocol.
type Runner struct {
	inner *agent.AppServerRunner
}

// NewRunner creates a Claude Code agent runner.
func NewRunner(command string, logger *slog.Logger) *Runner {
	return &Runner{
		inner: agent.NewAppServerRunner("claude-code", command, logger),
	}
}

func (r *Runner) Name() string { return r.inner.Name() }

func (r *Runner) Start(ctx context.Context, opts agent.StartOpts) (agent.Session, error) {
	return r.inner.Start(ctx, opts)
}
