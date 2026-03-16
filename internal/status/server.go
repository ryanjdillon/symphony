package status

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ryanjdillon/symphony/internal/orchestrator"
)

// SnapshotFunc returns the current orchestrator state.
type SnapshotFunc func() orchestrator.StateSnapshot

// RefreshFunc triggers an immediate poll cycle.
type RefreshFunc func()

// Server provides the HTTP API and WebSocket endpoint.
type Server struct {
	snapshot SnapshotFunc
	refresh  RefreshFunc
	hub      *Hub
	logger   *slog.Logger
	mux      *http.ServeMux
}

// NewServer creates a new status server.
func NewServer(snapshot SnapshotFunc, refresh RefreshFunc, logger *slog.Logger) *Server {
	s := &Server{
		snapshot: snapshot,
		refresh:  refresh,
		hub:      NewHub(),
		logger:   logger,
		mux:      http.NewServeMux(),
	}

	s.mux.HandleFunc("GET /api/v1/state", s.handleState)
	s.mux.HandleFunc("GET /api/v1/{identifier}", s.handleIssue)
	s.mux.HandleFunc("POST /api/v1/refresh", s.handleRefresh)
	s.mux.HandleFunc("GET /ws", s.hub.HandleWebSocket)

	return s
}

// Hub returns the WebSocket hub for broadcasting state changes.
func (s *Server) Hub() *Hub {
	return s.hub
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Start begins serving on the given address and starts the WebSocket hub.
func (s *Server) Start(addr string) error {
	go s.hub.Run()
	s.logger.Info("status server starting", "addr", addr)
	return http.ListenAndServe(addr, s)
}

func (s *Server) handleState(w http.ResponseWriter, _ *http.Request) {
	snap := s.snapshot()
	writeJSON(w, statePayload(snap))
}

func (s *Server) handleIssue(w http.ResponseWriter, r *http.Request) {
	identifier := r.PathValue("identifier")

	snap := s.snapshot()

	for _, ls := range snap.Running {
		if strings.EqualFold(ls.IssueIdentifier, identifier) {
			writeJSON(w, issuePayload(ls))
			return
		}
	}

	for _, re := range snap.Retrying {
		if strings.EqualFold(re.IssueIdentifier, identifier) {
			writeJSON(w, retryPayload(re))
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) handleRefresh(w http.ResponseWriter, _ *http.Request) {
	s.refresh()
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"refresh_triggered"}`))
}

type stateResponse struct {
	Running     []runningEntry `json:"running"`
	Retrying    []retryingEntry `json:"retrying"`
	Tokens      tokenResponse  `json:"tokens"`
	RuntimeSecs float64        `json:"runtime_s"`
}

type runningEntry struct {
	IssueID    string  `json:"issue_id"`
	Identifier string  `json:"identifier"`
	State      string  `json:"state"`
	SessionID  string  `json:"session_id"`
	TurnCount  int     `json:"turn_count"`
	ElapsedS   float64 `json:"elapsed_s"`
}

type retryingEntry struct {
	IssueID    string `json:"issue_id"`
	Identifier string `json:"identifier"`
	Attempt    int    `json:"attempt"`
	DueAt      string `json:"due_at"`
	Error      string `json:"error"`
}

type tokenResponse struct {
	Input  int64 `json:"input"`
	Output int64 `json:"output"`
	Total  int64 `json:"total"`
}

func statePayload(snap orchestrator.StateSnapshot) stateResponse {
	running := make([]runningEntry, 0, len(snap.Running))
	for _, ls := range snap.Running {
		running = append(running, runningEntry{
			IssueID:    ls.IssueID,
			Identifier: ls.IssueIdentifier,
			State:      ls.IssueState,
			SessionID:  ls.SessionID,
			TurnCount:  ls.TurnCount,
			ElapsedS:   ls.ElapsedSeconds(),
		})
	}

	retrying := make([]retryingEntry, 0, len(snap.Retrying))
	for _, re := range snap.Retrying {
		retrying = append(retrying, retryingEntry{
			IssueID:    re.IssueID,
			Identifier: re.IssueIdentifier,
			Attempt:    re.Attempt,
			DueAt:      re.DueAt.Format("2006-01-02T15:04:05Z"),
			Error:      re.LastError,
		})
	}

	return stateResponse{
		Running:  running,
		Retrying: retrying,
		Tokens: tokenResponse{
			Input:  snap.TokenTotals.Input,
			Output: snap.TokenTotals.Output,
			Total:  snap.TokenTotals.Total,
		},
		RuntimeSecs: snap.RuntimeSecs,
	}
}

func issuePayload(ls orchestrator.LiveSession) map[string]any {
	return map[string]any{
		"issue_id":    ls.IssueID,
		"identifier":  ls.IssueIdentifier,
		"state":       ls.IssueState,
		"session_id":  ls.SessionID,
		"turn_count":  ls.TurnCount,
		"elapsed_s":   ls.ElapsedSeconds(),
		"started_at":  ls.StartedAt.Format("2006-01-02T15:04:05Z"),
		"last_event":  ls.LastEventAt.Format("2006-01-02T15:04:05Z"),
		"tokens": map[string]int64{
			"input":  ls.Tokens.Input,
			"output": ls.Tokens.Output,
			"total":  ls.Tokens.Total,
		},
	}
}

func retryPayload(re orchestrator.RetryEntry) map[string]any {
	return map[string]any{
		"issue_id":   re.IssueID,
		"identifier": re.IssueIdentifier,
		"attempt":    re.Attempt,
		"due_at":     re.DueAt.Format("2006-01-02T15:04:05Z"),
		"error":      re.LastError,
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
