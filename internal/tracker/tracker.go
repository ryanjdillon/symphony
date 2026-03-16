package tracker

import "context"

// Tracker fetches and normalizes issues from a project management system.
type Tracker interface {
	// FetchCandidates returns issues in active states eligible for dispatch.
	FetchCandidates(ctx context.Context) ([]Issue, error)

	// FetchIssueStates returns current states for the given issue IDs (reconciliation).
	FetchIssueStates(ctx context.Context, ids []string) (map[string]string, error)

	// FetchTerminalIssues returns issues in terminal states (startup cleanup).
	FetchTerminalIssues(ctx context.Context) ([]Issue, error)
}
