package codex

import (
	"context"
	"log/slog"

	"github.com/ryanjdillon/symphony/internal/agent"
)

// Runner implements agent.Runner for Codex's app-server protocol.
type Runner struct {
	inner *agent.AppServerRunner
}

// NewRunner creates a Codex agent runner.
func NewRunner(command string, logger *slog.Logger) *Runner {
	return &Runner{
		inner: agent.NewAppServerRunner("codex", command, logger),
	}
}

func (r *Runner) Name() string { return r.inner.Name() }

func (r *Runner) Start(ctx context.Context, opts *agent.StartOpts) (agent.Session, error) {
	return r.inner.Start(ctx, opts)
}
