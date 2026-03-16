package orchestrator

import (
	"testing"
	"time"

	"github.com/ryanjdillon/symphony/internal/agent"
)

func TestState_ClaimAndRelease(t *testing.T) {
	s := newState()

	s.claim("issue-1")
	if !s.isClaimed("issue-1") {
		t.Error("issue-1 should be claimed")
	}
	if s.isClaimed("issue-2") {
		t.Error("issue-2 should not be claimed")
	}

	s.release("issue-1")
	if s.isClaimed("issue-1") {
		t.Error("issue-1 should be released")
	}
}

func TestState_RunningCount(t *testing.T) {
	s := newState()

	if s.runningCount() != 0 {
		t.Errorf("expected 0 running, got %d", s.runningCount())
	}

	s.addRunning(&LiveSession{IssueID: "1", IssueState: "Todo"})
	s.addRunning(&LiveSession{IssueID: "2", IssueState: "In Progress"})
	s.addRunning(&LiveSession{IssueID: "3", IssueState: "Todo"})

	if s.runningCount() != 3 {
		t.Errorf("expected 3 running, got %d", s.runningCount())
	}
	if s.runningCountByState("Todo") != 2 {
		t.Errorf("expected 2 running in Todo, got %d", s.runningCountByState("Todo"))
	}
	if s.runningCountByState("In Progress") != 1 {
		t.Errorf("expected 1 running in In Progress, got %d", s.runningCountByState("In Progress"))
	}

	s.removeRunning("1")
	if s.runningCount() != 2 {
		t.Errorf("expected 2 running after remove, got %d", s.runningCount())
	}
}

func TestState_MarkCompleted(t *testing.T) {
	s := newState()

	s.claim("issue-1")
	s.addRunning(&LiveSession{IssueID: "issue-1"})
	s.addRetry(&RetryEntry{IssueID: "issue-1", Attempt: 1})

	s.markCompleted("issue-1")

	if s.isClaimed("issue-1") {
		t.Error("completed issue should not be claimed")
	}
	if _, ok := s.getRunning("issue-1"); ok {
		t.Error("completed issue should not be running")
	}
	if retries := s.dueRetries(time.Now()); len(retries) > 0 {
		t.Error("completed issue should not have retries")
	}
}

func TestState_DueRetries(t *testing.T) {
	s := newState()

	past := time.Now().Add(-1 * time.Minute)
	future := time.Now().Add(1 * time.Hour)

	s.addRetry(&RetryEntry{IssueID: "due", DueAt: past})
	s.addRetry(&RetryEntry{IssueID: "not-due", DueAt: future})

	due := s.dueRetries(time.Now())
	if len(due) != 1 {
		t.Fatalf("expected 1 due retry, got %d", len(due))
	}
	if due[0].IssueID != "due" {
		t.Errorf("expected due retry for 'due', got %q", due[0].IssueID)
	}
}

func TestState_AddTokens(t *testing.T) {
	s := newState()

	s.addTokens(agent.TokenUsage{Input: 100, Output: 50, Total: 150})
	s.addTokens(agent.TokenUsage{Input: 200, Output: 100, Total: 300})

	snap := s.Snapshot()
	if snap.TokenTotals.Input != 300 {
		t.Errorf("expected 300 input tokens, got %d", snap.TokenTotals.Input)
	}
	if snap.TokenTotals.Total != 450 {
		t.Errorf("expected 450 total tokens, got %d", snap.TokenTotals.Total)
	}
}

func TestState_Snapshot(t *testing.T) {
	s := newState()

	s.addRunning(&LiveSession{
		IssueID:         "1",
		IssueIdentifier: "SYM-1",
		IssueState:      "Todo",
		SessionID:       "sess-1",
		StartedAt:       time.Now(),
		LastEventAt:     time.Now(),
		TurnCount:       3,
	})
	s.addRetry(&RetryEntry{
		IssueID:         "2",
		IssueIdentifier: "SYM-2",
		Attempt:         2,
		DueAt:           time.Now().Add(1 * time.Minute),
		LastError:       "stalled",
	})

	snap := s.Snapshot()
	if len(snap.Running) != 1 {
		t.Errorf("expected 1 running in snapshot, got %d", len(snap.Running))
	}
	if len(snap.Retrying) != 1 {
		t.Errorf("expected 1 retrying in snapshot, got %d", len(snap.Retrying))
	}
	if snap.RuntimeSecs <= 0 {
		t.Error("expected positive runtime seconds")
	}
}

func TestState_RunningIssueIDs(t *testing.T) {
	s := newState()
	s.addRunning(&LiveSession{IssueID: "a"})
	s.addRunning(&LiveSession{IssueID: "b"})

	ids := s.runningIssueIDs()
	if len(ids) != 2 {
		t.Errorf("expected 2 running IDs, got %d", len(ids))
	}
}
