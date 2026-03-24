package orchestrator

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ryanjdillon/symphony/internal/agent"
)

// LiveSession tracks metadata for an active coding agent subprocess.
type LiveSession struct {
	IssueID         string
	IssueIdentifier string
	IssueState      string
	SessionID       string
	Host            string // SSH host; empty for local execution
	Session         agent.Session
	StartedAt       time.Time
	LastEventAt     time.Time
	TurnCount       int
	Tokens          agent.TokenUsage
}

// RetryEntry represents a scheduled retry for a failed or continuation run.
type RetryEntry struct {
	IssueID         string
	IssueIdentifier string
	Attempt         int
	DueAt           time.Time
	LastError       string
	Host            string // SSH host for continuation affinity
}

// State holds the orchestrator's in-memory runtime state.
// Access must go through the Orchestrator methods; this struct
// is exported only for the status surface to snapshot.
type State struct {
	mu            sync.RWMutex
	Running       map[string]*LiveSession // issue ID → active session
	Claimed       map[string]struct{}     // reserved issue IDs
	RetryAttempts map[string]*RetryEntry  // issue ID → retry info
	Completed     map[string]struct{}     // bookkeeping
	TokenTotals   agent.TokenUsage
	RateLimits    json.RawMessage
	StartedAt     time.Time
}

func newState() *State {
	return &State{
		Running:       make(map[string]*LiveSession),
		Claimed:       make(map[string]struct{}),
		RetryAttempts: make(map[string]*RetryEntry),
		Completed:     make(map[string]struct{}),
		StartedAt:     time.Now(),
	}
}

// Snapshot returns a read-only copy of the state for the status surface.
type StateSnapshot struct {
	Running     []LiveSession
	Retrying    []RetryEntry
	TokenTotals agent.TokenUsage
	RateLimits  json.RawMessage
	RuntimeSecs float64
}

func (s *State) Snapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	running := make([]LiveSession, 0, len(s.Running))
	for _, ls := range s.Running {
		running = append(running, *ls)
	}

	retrying := make([]RetryEntry, 0, len(s.RetryAttempts))
	for _, re := range s.RetryAttempts {
		retrying = append(retrying, *re)
	}

	return StateSnapshot{
		Running:     running,
		Retrying:    retrying,
		TokenTotals: s.TokenTotals,
		RateLimits:  s.RateLimits,
		RuntimeSecs: time.Since(s.StartedAt).Seconds(),
	}
}

func (s *State) claim(issueID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Claimed[issueID] = struct{}{}
}

func (s *State) release(issueID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Claimed, issueID)
	delete(s.Running, issueID)
	delete(s.RetryAttempts, issueID)
}

func (s *State) isClaimed(issueID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.Claimed[issueID]
	return ok
}

func (s *State) addRunning(ls *LiveSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Running[ls.IssueID] = ls
	delete(s.RetryAttempts, ls.IssueID)
}

func (s *State) removeRunning(issueID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Running, issueID)
}

func (s *State) addRetry(entry *RetryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RetryAttempts[entry.IssueID] = entry
}

func (s *State) markCompleted(issueID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Completed[issueID] = struct{}{}
	delete(s.Claimed, issueID)
	delete(s.Running, issueID)
	delete(s.RetryAttempts, issueID)
}

func (s *State) addTokens(usage agent.TokenUsage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TokenTotals.Input += usage.Input
	s.TokenTotals.Output += usage.Output
	s.TokenTotals.Total += usage.Total
}

func (s *State) runningCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Running)
}

func (s *State) runningCountByState(state string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, ls := range s.Running {
		if ls.IssueState == state {
			count++
		}
	}
	return count
}

func (s *State) runningIssueIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.Running))
	for id := range s.Running {
		ids = append(ids, id)
	}
	return ids
}

func (s *State) getRunning(issueID string) (*LiveSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ls, ok := s.Running[issueID]
	return ls, ok
}

func (s *State) dueRetries(now time.Time) []*RetryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var due []*RetryEntry
	for _, entry := range s.RetryAttempts {
		if !now.Before(entry.DueAt) {
			due = append(due, entry)
		}
	}
	return due
}
