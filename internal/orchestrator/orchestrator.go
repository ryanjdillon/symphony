package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/ryanjdillon/symphony/internal/agent"
	"github.com/ryanjdillon/symphony/internal/config"
	"github.com/ryanjdillon/symphony/internal/template"
	"github.com/ryanjdillon/symphony/internal/tracker"
	"github.com/ryanjdillon/symphony/internal/workspace"
)

// StateChangeFunc is called whenever the orchestrator state changes.
// Used by the status surface to push WebSocket updates.
type StateChangeFunc func(StateSnapshot)

// Orchestrator owns the poll loop and is the single source of truth for scheduling.
type Orchestrator struct {
	cfg           *config.Config
	tracker       tracker.Tracker
	workspaceMgr  *workspace.Manager
	agentRunner   agent.Runner
	state         *State
	logger        *slog.Logger
	onStateChange StateChangeFunc
}

// New creates a new Orchestrator.
func New(
	cfg *config.Config,
	trk tracker.Tracker,
	wsMgr *workspace.Manager,
	runner agent.Runner,
	logger *slog.Logger,
	onStateChange StateChangeFunc,
) *Orchestrator {
	return &Orchestrator{
		cfg:           cfg,
		tracker:       trk,
		workspaceMgr:  wsMgr,
		agentRunner:   runner,
		state:         newState(),
		logger:        logger,
		onStateChange: onStateChange,
	}
}

// Snapshot returns the current state for the status surface.
func (o *Orchestrator) Snapshot() StateSnapshot {
	return o.state.Snapshot()
}

// Run starts the poll loop. Blocks until context is cancelled.
func (o *Orchestrator) Run(ctx context.Context) error {
	o.logger.Info("orchestrator starting")

	if err := o.startupCleanup(ctx); err != nil {
		o.logger.Warn("startup cleanup failed", "error", err)
	}

	interval := time.Duration(o.cfg.Polling.IntervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.logger.Info("orchestrator shutting down")
			o.stopAllRunning()
			return ctx.Err()
		case <-ticker.C:
			o.tick(ctx)
		}
	}
}

// TriggerRefresh runs a poll tick immediately (best-effort, for the API).
func (o *Orchestrator) TriggerRefresh(ctx context.Context) {
	go o.tick(ctx)
}

// UpdateConfig swaps the live config (called by the file watcher).
func (o *Orchestrator) UpdateConfig(cfg *config.Config) {
	o.cfg = cfg
	o.logger.Info("config reloaded")
}

func (o *Orchestrator) tick(ctx context.Context) {
	o.reconcile(ctx)
	o.processRetries(ctx)

	if err := o.cfg.Validate(); err != nil {
		o.logger.Error("dispatch config invalid, skipping", "error", err)
		return
	}

	candidates, err := o.tracker.FetchCandidates(ctx)
	if err != nil {
		o.logger.Error("failed to fetch candidates, skipping dispatch", "error", err)
		return
	}

	sortCandidates(candidates)

	activeStates := toSet(o.cfg.Tracker.ActiveStates)
	terminalStates := toSet(o.cfg.Tracker.TerminalStates)

	// Fetch states for blockers
	allIssueStates := make(map[string]string)
	for _, c := range candidates {
		allIssueStates[c.ID] = c.State
	}

	for _, issue := range candidates {
		if !isEligible(issue, o.state, activeStates, terminalStates,
			o.cfg.Agent.MaxConcurrentAgents, o.cfg.Agent.MaxConcurrentAgentsByState, allIssueStates) {
			continue
		}
		o.dispatch(ctx, issue)
	}
}

func (o *Orchestrator) dispatch(ctx context.Context, issue tracker.Issue) {
	logger := o.logger.With("issue_id", issue.ID, "issue_identifier", issue.Identifier)
	logger.Info("dispatching issue", "state", issue.State)

	o.state.claim(issue.ID)

	wsPath, created, err := o.workspaceMgr.EnsureWorkspace(issue.Identifier)
	if err != nil {
		logger.Error("failed to create workspace", "error", err)
		o.state.release(issue.ID)
		return
	}

	if created {
		if hook := o.cfg.Workspace.Hooks.AfterCreate; hook != "" {
			hookTimeout := time.Duration(o.cfg.Workspace.Hooks.HookTimeoutMs) * time.Millisecond
			if err := workspace.RunHook(ctx, "after_create", hook, wsPath, hookTimeout, logger); err != nil {
				logger.Error("after_create hook failed", "error", err)
				o.state.release(issue.ID)
				return
			}
		}
	}

	go o.runWorker(ctx, issue, wsPath, 0)
}

func (o *Orchestrator) runWorker(ctx context.Context, issue tracker.Issue, wsPath string, attempt int) {
	logger := o.logger.With("issue_id", issue.ID, "issue_identifier", issue.Identifier, "attempt", attempt)

	if hook := o.cfg.Workspace.Hooks.BeforeRun; hook != "" {
		hookTimeout := time.Duration(o.cfg.Workspace.Hooks.HookTimeoutMs) * time.Millisecond
		if err := workspace.RunHook(ctx, "before_run", hook, wsPath, hookTimeout, logger); err != nil {
			logger.Error("before_run hook failed, aborting attempt", "error", err)
			o.scheduleRetry(issue, attempt, err.Error())
			return
		}
	}

	var attemptPtr *int
	if attempt > 0 {
		attemptPtr = &attempt
	}

	prompt, err := template.Render(o.cfg.PromptTemplate, issue, attemptPtr)
	if err != nil {
		logger.Error("template render failed", "error", err)
		o.scheduleRetry(issue, attempt, err.Error())
		return
	}

	turnTimeout := time.Duration(o.cfg.Agent.TurnTimeoutMs()) * time.Millisecond
	stallTimeout := time.Duration(o.cfg.Agent.StallTimeoutMs()) * time.Millisecond

	session, err := o.agentRunner.Start(ctx, agent.StartOpts{
		WorkspacePath: wsPath,
		Prompt:        prompt,
		IssueContext:  issue,
		Continuation:  attempt > 0,
		MaxTurns:      o.cfg.Agent.MaxTurns,
		TurnTimeout:   turnTimeout,
		StallTimeout:  stallTimeout,
		Config:        o.cfg.Agent.Config,
	})
	if err != nil {
		logger.Error("failed to start agent", "error", err)
		o.scheduleRetry(issue, attempt, err.Error())
		return
	}

	ls := &LiveSession{
		IssueID:         issue.ID,
		IssueIdentifier: issue.Identifier,
		IssueState:      issue.State,
		SessionID:       session.SessionID(),
		Session:         session,
		StartedAt:       time.Now(),
		LastEventAt:     time.Now(),
	}
	o.state.addRunning(ls)
	o.notifyStateChange()

	logger = logger.With("session_id", session.SessionID())
	logger.Info("agent session started")

	for event := range session.Events() {
		ls.LastEventAt = time.Now()
		ls.Tokens = event.Tokens
		if event.Type == "turn/start" {
			ls.TurnCount++
		}
	}

	outcome := session.Wait()
	o.state.removeRunning(issue.ID)
	o.state.addTokens(ls.Tokens)

	logger.Info("agent session ended", "outcome", outcome.String(), "turns", ls.TurnCount)

	if hook := o.cfg.Workspace.Hooks.AfterRun; hook != "" {
		hookTimeout := time.Duration(o.cfg.Workspace.Hooks.HookTimeoutMs) * time.Millisecond
		if err := workspace.RunHook(ctx, "after_run", hook, wsPath, hookTimeout, logger); err != nil {
			logger.Warn("after_run hook failed", "error", err)
		}
	}

	switch outcome {
	case agent.Succeeded:
		// Continuation retry: re-check tracker state after 1s
		o.state.addRetry(&RetryEntry{
			IssueID:         issue.ID,
			IssueIdentifier: issue.Identifier,
			Attempt:         attempt + 1,
			DueAt:           time.Now().Add(1 * time.Second),
		})
	case agent.Failed, agent.TimedOut, agent.Stalled:
		o.scheduleRetry(issue, attempt, outcome.String())
	case agent.CanceledByReconciliation:
		// Released by reconciliation, no retry
	}

	o.notifyStateChange()
}

func (o *Orchestrator) scheduleRetry(issue tracker.Issue, attempt int, lastErr string) {
	delay := retryDelay(attempt, o.cfg.Agent.MaxRetryBackoffMs)
	o.state.addRetry(&RetryEntry{
		IssueID:         issue.ID,
		IssueIdentifier: issue.Identifier,
		Attempt:         attempt + 1,
		DueAt:           time.Now().Add(delay),
		LastError:       lastErr,
	})
	o.logger.Info("retry scheduled",
		"issue_id", issue.ID,
		"issue_identifier", issue.Identifier,
		"attempt", attempt+1,
		"due_in", delay,
	)
	o.notifyStateChange()
}

func (o *Orchestrator) processRetries(ctx context.Context) {
	due := o.state.dueRetries(time.Now())
	for _, entry := range due {
		logger := o.logger.With("issue_id", entry.IssueID, "issue_identifier", entry.IssueIdentifier)

		// Refresh issue state from tracker before retrying
		states, err := o.tracker.FetchIssueStates(ctx, []string{entry.IssueID})
		if err != nil {
			logger.Warn("failed to refresh issue state for retry", "error", err)
			continue
		}

		currentState, ok := states[entry.IssueID]
		if !ok {
			logger.Warn("issue not found in tracker, releasing")
			o.state.release(entry.IssueID)
			continue
		}

		terminalStates := toSet(o.cfg.Tracker.TerminalStates)
		if _, terminal := terminalStates[currentState]; terminal {
			logger.Info("issue now terminal, releasing", "state", currentState)
			o.state.markCompleted(entry.IssueID)
			if err := o.workspaceMgr.RemoveWorkspace(entry.IssueIdentifier); err != nil {
				logger.Warn("workspace cleanup failed", "error", err)
			}
			continue
		}

		activeStates := toSet(o.cfg.Tracker.ActiveStates)
		if _, active := activeStates[currentState]; !active {
			logger.Info("issue no longer active, releasing", "state", currentState)
			o.state.release(entry.IssueID)
			continue
		}

		wsPath := o.workspaceMgr.WorkspacePath(entry.IssueIdentifier)
		issue := tracker.Issue{
			ID:         entry.IssueID,
			Identifier: entry.IssueIdentifier,
			State:      currentState,
		}
		go o.runWorker(ctx, issue, wsPath, entry.Attempt)
	}
}

func (o *Orchestrator) reconcile(ctx context.Context) {
	runningIDs := o.state.runningIssueIDs()
	if len(runningIDs) == 0 {
		return
	}

	stallTimeout := time.Duration(o.cfg.Agent.StallTimeoutMs()) * time.Millisecond

	// Stall detection
	if stallTimeout > 0 {
		now := time.Now()
		for _, id := range runningIDs {
			ls, ok := o.state.getRunning(id)
			if !ok {
				continue
			}
			elapsed := now.Sub(ls.LastEventAt)
			if elapsed > stallTimeout {
				o.logger.Warn("session stalled, terminating",
					"issue_id", id,
					"issue_identifier", ls.IssueIdentifier,
					"session_id", ls.SessionID,
					"stalled_for", elapsed,
				)
				if err := ls.Session.Stop(); err != nil {
					o.logger.Warn("failed to stop stalled session", "error", err)
				}
			}
		}
	}

	// State refresh
	states, err := o.tracker.FetchIssueStates(ctx, runningIDs)
	if err != nil {
		o.logger.Warn("reconciliation state refresh failed, keeping workers", "error", err)
		return
	}

	terminalStates := toSet(o.cfg.Tracker.TerminalStates)
	activeStates := toSet(o.cfg.Tracker.ActiveStates)

	for _, id := range runningIDs {
		ls, ok := o.state.getRunning(id)
		if !ok {
			continue
		}

		currentState, ok := states[id]
		if !ok {
			continue
		}

		logger := o.logger.With("issue_id", id, "issue_identifier", ls.IssueIdentifier)

		if _, terminal := terminalStates[currentState]; terminal {
			logger.Info("issue now terminal during run, stopping", "state", currentState)
			if err := ls.Session.Stop(); err != nil {
				logger.Warn("failed to stop session", "error", err)
			}
			o.state.markCompleted(id)
			if err := o.workspaceMgr.RemoveWorkspace(ls.IssueIdentifier); err != nil {
				logger.Warn("workspace cleanup failed", "error", err)
			}
			o.notifyStateChange()
			continue
		}

		if _, active := activeStates[currentState]; !active {
			logger.Info("issue no longer active during run, stopping", "state", currentState)
			if err := ls.Session.Stop(); err != nil {
				logger.Warn("failed to stop session", "error", err)
			}
			o.state.release(id)
			o.notifyStateChange()
			continue
		}

		ls.IssueState = currentState
	}
}

func (o *Orchestrator) startupCleanup(ctx context.Context) error {
	issues, err := o.tracker.FetchTerminalIssues(ctx)
	if err != nil {
		return fmt.Errorf("fetch terminal issues: %w", err)
	}

	identifiers := make([]string, 0, len(issues))
	for _, issue := range issues {
		identifiers = append(identifiers, issue.Identifier)
	}

	if len(identifiers) > 0 {
		o.logger.Info("cleaning up terminal workspaces", "count", len(identifiers))
		return o.workspaceMgr.CleanupTerminal(identifiers)
	}
	return nil
}

func (o *Orchestrator) stopAllRunning() {
	for _, id := range o.state.runningIssueIDs() {
		if ls, ok := o.state.getRunning(id); ok {
			o.logger.Info("stopping session on shutdown",
				"issue_id", id,
				"issue_identifier", ls.IssueIdentifier,
				"session_id", ls.SessionID,
			)
			if err := ls.Session.Stop(); err != nil {
				o.logger.Warn("failed to stop session", "error", err)
			}
		}
	}
}

func (o *Orchestrator) notifyStateChange() {
	if o.onStateChange != nil {
		o.onStateChange(o.state.Snapshot())
	}
}

// retryDelay calculates exponential backoff: 10000 * 2^(attempt-1) ms, capped at maxMs.
func retryDelay(attempt, maxMs int) time.Duration {
	if attempt <= 0 {
		return 1 * time.Second // continuation retry
	}
	delayMs := 10000.0 * math.Pow(2, float64(attempt-1))
	if delayMs > float64(maxMs) {
		delayMs = float64(maxMs)
	}
	return time.Duration(delayMs) * time.Millisecond
}

func toSet(slice []string) map[string]struct{} {
	m := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		m[s] = struct{}{}
	}
	return m
}
