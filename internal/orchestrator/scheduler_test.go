package orchestrator

import (
	"testing"
	"time"

	"github.com/ryanjdillon/symphony/internal/tracker"
)

func intPtr(i int) *int { return &i }

func TestSortCandidates(t *testing.T) {
	now := time.Now()
	issues := []tracker.Issue{
		{ID: "3", Priority: intPtr(3), CreatedAt: now},
		{ID: "1", Priority: intPtr(1), CreatedAt: now},
		{ID: "2", Priority: intPtr(2), CreatedAt: now},
		{ID: "nil", Priority: nil, CreatedAt: now},
	}

	sortCandidates(issues)

	want := []string{"1", "2", "3", "nil"}
	for i, id := range want {
		if issues[i].ID != id {
			t.Errorf("position %d: got ID %q, want %q", i, issues[i].ID, id)
		}
	}
}

func TestSortCandidates_SamePriority_ByCreatedAt(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	issues := []tracker.Issue{
		{ID: "newer", Priority: intPtr(1), CreatedAt: t2},
		{ID: "older", Priority: intPtr(1), CreatedAt: t1},
	}

	sortCandidates(issues)

	if issues[0].ID != "older" {
		t.Errorf("expected older first, got %q", issues[0].ID)
	}
}

func TestIsEligible(t *testing.T) {
	state := newState()
	active := map[string]struct{}{"Todo": {}, "In Progress": {}}
	terminal := map[string]struct{}{"Done": {}}

	issue := tracker.Issue{
		ID:         "id1",
		Identifier: "SYM-1",
		Title:      "Test",
		State:      "Todo",
	}

	if !isEligible(issue, state, active, terminal, 10, nil, nil) {
		t.Error("expected eligible issue to pass")
	}

	// Missing title
	badIssue := tracker.Issue{ID: "id2", Identifier: "SYM-2", State: "Todo"}
	if isEligible(badIssue, state, active, terminal, 10, nil, nil) {
		t.Error("expected issue without title to be ineligible")
	}

	// Wrong state
	wrongState := tracker.Issue{ID: "id3", Identifier: "SYM-3", Title: "X", State: "Done"}
	if isEligible(wrongState, state, active, terminal, 10, nil, nil) {
		t.Error("expected terminal state issue to be ineligible")
	}

	// Already claimed
	state.claim("id1")
	if isEligible(issue, state, active, terminal, 10, nil, nil) {
		t.Error("expected claimed issue to be ineligible")
	}
}

func TestRetryDelay(t *testing.T) {
	// Continuation retry
	if d := retryDelay(0, 300000); d != 1*time.Second {
		t.Errorf("attempt 0: got %v, want 1s", d)
	}

	// First failure: 10s
	if d := retryDelay(1, 300000); d != 10*time.Second {
		t.Errorf("attempt 1: got %v, want 10s", d)
	}

	// Second failure: 20s
	if d := retryDelay(2, 300000); d != 20*time.Second {
		t.Errorf("attempt 2: got %v, want 20s", d)
	}

	// Capped at max
	if d := retryDelay(100, 300000); d != 300*time.Second {
		t.Errorf("attempt 100: got %v, want 300s (capped)", d)
	}
}
