package linear

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	c := NewClient("test-key", "test-project", []string{"Todo", "In Progress"}, []string{"Done"}, slog.Default())
	c.httpClient = &http.Client{}
	return c
}

func TestFetchCandidates(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or wrong auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing content-type header")
		}

		resp := map[string]any{
			"data": map[string]any{
				"issues": map[string]any{
					"nodes": []map[string]any{
						{
							"id":          "issue-1",
							"identifier":  "SYM-1",
							"title":       "Test Issue",
							"description": "A test issue",
							"priority":    2,
							"state":       map[string]string{"name": "Todo"},
							"branchName":  "sym-1-test",
							"url":         "https://linear.app/test/SYM-1",
							"labels":      map[string]any{"nodes": []map[string]string{{"name": "Bug"}}},
							"inverseRelations": map[string]any{"nodes": []any{}},
							"createdAt":   "2026-03-16T10:00:00Z",
							"updatedAt":   "2026-03-16T12:00:00Z",
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	// Override the endpoint for testing
	origEndpoint := graphqlEndpoint
	defer func() { overrideEndpoint = "" }()
	overrideEndpoint = srv.URL

	issues, err := client.FetchCandidates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	issue := issues[0]
	if issue.ID != "issue-1" {
		t.Errorf("issue.ID = %q, want %q", issue.ID, "issue-1")
	}
	if issue.Identifier != "SYM-1" {
		t.Errorf("issue.Identifier = %q, want %q", issue.Identifier, "SYM-1")
	}
	if issue.State != "Todo" {
		t.Errorf("issue.State = %q, want %q", issue.State, "Todo")
	}
	if len(issue.Labels) != 1 || issue.Labels[0] != "bug" {
		t.Errorf("issue.Labels = %v, want [bug] (lowercase)", issue.Labels)
	}

	_ = origEndpoint
}

func TestFetchIssueStates(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"issues": map[string]any{
					"nodes": []map[string]any{
						{"id": "id-1", "state": map[string]string{"name": "Done"}},
						{"id": "id-2", "state": map[string]string{"name": "In Progress"}},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	overrideEndpoint = srv.URL
	defer func() { overrideEndpoint = "" }()

	states, err := client.FetchIssueStates(context.Background(), []string{"id-1", "id-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if states["id-1"] != "Done" {
		t.Errorf("states[id-1] = %q, want %q", states["id-1"], "Done")
	}
	if states["id-2"] != "In Progress" {
		t.Errorf("states[id-2] = %q, want %q", states["id-2"], "In Progress")
	}
}

func TestFetchIssueStates_EmptyIDs(t *testing.T) {
	client := NewClient("key", "slug", nil, nil, slog.Default())
	states, err := client.FetchIssueStates(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(states) != 0 {
		t.Errorf("expected empty map, got %v", states)
	}
}

func TestGraphQLErrors(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"errors": []map[string]string{
				{"message": "something went wrong"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	overrideEndpoint = srv.URL
	defer func() { overrideEndpoint = "" }()

	_, err := client.FetchCandidates(context.Background())
	if err == nil {
		t.Fatal("expected error for GraphQL errors response")
	}
}

func TestNon200Status(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	})
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	overrideEndpoint = srv.URL
	defer func() { overrideEndpoint = "" }()

	_, err := client.FetchCandidates(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestNormalizeIssue_MalformedTimestamp(t *testing.T) {
	raw := &rawIssue{
		ID:         "bad",
		Identifier: "SYM-X",
		CreatedAt:  "not-a-date",
		UpdatedAt:  "2026-01-01T00:00:00Z",
	}
	_, err := normalizeIssue(raw)
	if err == nil {
		t.Error("expected error for malformed createdAt")
	}
}

func TestNormalizeIssue_NilPriority(t *testing.T) {
	raw := &rawIssue{
		ID:         "id",
		Identifier: "SYM-1",
		Priority:   0,
		CreatedAt:  "2026-01-01T00:00:00Z",
		UpdatedAt:  "2026-01-01T00:00:00Z",
	}
	issue, err := normalizeIssue(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Priority != nil {
		t.Errorf("expected nil priority for 0, got %v", *issue.Priority)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(short, 10) = %q", got)
	}
	if got := truncate("this is a long string", 10); got != "this is a ..." {
		t.Errorf("truncate(long, 10) = %q", got)
	}
}
