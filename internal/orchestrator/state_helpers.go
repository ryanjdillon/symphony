package orchestrator

import "time"

// ElapsedSeconds returns the time since the session started.
func (ls *LiveSession) ElapsedSeconds() float64 {
	return time.Since(ls.StartedAt).Seconds()
}
