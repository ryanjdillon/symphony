package status

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ryanjdillon/symphony/internal/agent"
	"github.com/ryanjdillon/symphony/internal/orchestrator"
)

func testSnapshot() orchestrator.StateSnapshot {
	return orchestrator.StateSnapshot{
		Running: []orchestrator.LiveSession{
			{
				IssueID:         "id-1",
				IssueIdentifier: "SYM-1",
				IssueState:      "In Progress",
				SessionID:       "sess-1",
				StartedAt:       time.Now().Add(-2 * time.Minute),
				LastEventAt:     time.Now(),
				TurnCount:       3,
				Tokens:          agent.TokenUsage{Input: 1000, Output: 500, Total: 1500},
			},
		},
		Retrying: []orchestrator.RetryEntry{
			{
				IssueID:         "id-2",
				IssueIdentifier: "SYM-2",
				Attempt:         2,
				DueAt:           time.Now().Add(30 * time.Second),
				LastError:       "stalled",
			},
		},
		TokenTotals: agent.TokenUsage{Input: 5000, Output: 2000, Total: 7000},
		RuntimeSecs: 3600.5,
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	refreshCalled := false
	return NewServer(
		testSnapshot,
		func() { refreshCalled = true; _ = refreshCalled },
		slog.Default(),
	)
}

func TestHandleState(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/state", http.NoBody)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp stateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Running) != 1 {
		t.Errorf("expected 1 running, got %d", len(resp.Running))
	}
	if resp.Running[0].Identifier != "SYM-1" {
		t.Errorf("running[0].identifier = %q, want SYM-1", resp.Running[0].Identifier)
	}
	if len(resp.Retrying) != 1 {
		t.Errorf("expected 1 retrying, got %d", len(resp.Retrying))
	}
	if resp.Tokens.Total != 7000 {
		t.Errorf("tokens.total = %d, want 7000", resp.Tokens.Total)
	}
}

func TestHandleIssue_Found(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/SYM-1", http.NoBody)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp["identifier"] != "SYM-1" {
		t.Errorf("identifier = %v, want SYM-1", resp["identifier"])
	}
}

func TestHandleIssue_NotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/SYM-999", http.NoBody)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleIssue_FoundInRetrying(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/SYM-2", http.NoBody)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["identifier"] != "SYM-2" {
		t.Errorf("identifier = %v, want SYM-2", resp["identifier"])
	}
}

func TestHandleRefresh(t *testing.T) {
	refreshCalled := false
	srv := NewServer(
		testSnapshot,
		func() { refreshCalled = true },
		slog.Default(),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/refresh", http.NoBody)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rec.Code)
	}
	if !refreshCalled {
		t.Error("refresh callback should have been called")
	}
}
