package agent

import (
	"context"
	"time"

	"github.com/ryanjdillon/symphony/internal/tracker"
)

// Runner launches and manages a coding agent subprocess.
type Runner interface {
	// Name returns the agent identifier (e.g., "claude-code", "codex").
	Name() string

	// Start launches the agent process in the given workspace with the
	// provided prompt. Returns a Session for interacting with the running agent.
	Start(ctx context.Context, opts *StartOpts) (Session, error)
}

// StartOpts configures a new agent session.
type StartOpts struct {
	WorkspacePath string
	Prompt        string
	IssueContext  tracker.Issue
	Continuation  bool // true if this is a follow-up turn
	MaxTurns      int
	TurnTimeout   time.Duration
	StallTimeout  time.Duration
	Config        map[string]any
	Tools         []ToolHandler // tools available to the agent during this session
}

// Session represents a running agent interaction.
type Session interface {
	// Events returns a channel of streaming events from the agent.
	Events() <-chan Event

	// Wait blocks until the session completes and returns the outcome.
	Wait() Outcome

	// Stop terminates the agent process.
	Stop() error

	// SessionID returns the unique session identifier.
	SessionID() string
}

// Outcome represents the terminal result of an agent session.
type Outcome int

const (
	Succeeded Outcome = iota
	Failed
	TimedOut
	Stalled
	CanceledByReconciliation
)

var outcomeNames = [...]string{
	Succeeded:                "succeeded",
	Failed:                   "failed",
	TimedOut:                 "timed_out",
	Stalled:                  "stalled",
	CanceledByReconciliation: "canceled_by_reconciliation",
}

func (o Outcome) String() string {
	if int(o) < len(outcomeNames) {
		return outcomeNames[o]
	}
	return "unknown"
}
