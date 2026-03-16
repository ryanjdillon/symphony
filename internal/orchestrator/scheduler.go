package orchestrator

import (
	"sort"

	"github.com/ryanjdillon/symphony/internal/tracker"
)

// sortCandidates sorts issues by priority (ascending, nil last) then creation time (ascending).
func sortCandidates(issues []tracker.Issue) {
	sort.Slice(issues, func(i, j int) bool {
		pi, pj := issues[i].Priority, issues[j].Priority
		switch {
		case pi != nil && pj != nil:
			if *pi != *pj {
				return *pi < *pj
			}
		case pi != nil:
			return true
		case pj != nil:
			return false
		}
		return issues[i].CreatedAt.Before(issues[j].CreatedAt)
	})
}

// isEligible checks whether an issue can be dispatched given current state and config.
func isEligible(
	issue *tracker.Issue,
	state *State,
	activeStates map[string]struct{},
	terminalStates map[string]struct{},
	maxConcurrent int,
	maxByState map[string]int,
	allIssueStates map[string]string,
) bool {
	if issue.ID == "" || issue.Identifier == "" || issue.Title == "" {
		return false
	}

	if _, ok := activeStates[issue.State]; !ok {
		return false
	}

	if state.isClaimed(issue.ID) {
		return false
	}

	if state.runningCount() >= maxConcurrent {
		return false
	}

	if maxCap, ok := maxByState[issue.State]; ok {
		if state.runningCountByState(issue.State) >= maxCap {
			return false
		}
	}

	if issue.State == "Todo" && isBlocked(issue, terminalStates, allIssueStates) {
		return false
	}

	return true
}

// isBlocked returns true if any blocker is in a non-terminal state.
func isBlocked(issue *tracker.Issue, terminalStates map[string]struct{}, allIssueStates map[string]string) bool {
	for _, blockerID := range issue.BlockedBy {
		blockerState, ok := allIssueStates[blockerID]
		if !ok {
			return true // unknown state = assume blocked
		}
		if _, terminal := terminalStates[blockerState]; !terminal {
			return true
		}
	}
	return false
}
