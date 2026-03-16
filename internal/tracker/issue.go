package tracker

import "time"

// Issue is the normalized domain model, tracker-agnostic.
type Issue struct {
	ID          string
	Identifier  string // e.g., "SYM-123"
	Title       string
	Description string
	Priority    *int
	State       string
	BranchName  string
	URL         string
	Labels      []string
	BlockedBy   []string // issue IDs
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
