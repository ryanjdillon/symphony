package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLinearGraphQL_Spec(t *testing.T) {
	tool := NewLinearGraphQL("test-key", false, slog.Default())
	spec := tool.Spec()

	if spec.Name != "linear_graphql" {
		t.Errorf("name = %q, want %q", spec.Name, "linear_graphql")
	}
	if spec.Description == "" {
		t.Error("description should not be empty")
	}
	if len(spec.InputSchema) == 0 {
		t.Error("input schema should not be empty")
	}
}

func TestLinearGraphQL_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing auth header")
		}
		resp := map[string]any{
			"data": map[string]any{
				"viewer": map[string]string{"name": "Test User"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newTestTool(srv.URL, "test-key", false)

	args, _ := json.Marshal(map[string]string{"query": "{ viewer { name } }"})
	result := tool.Execute(context.Background(), args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Output)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestLinearGraphQL_Execute_GraphQLErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"errors": []map[string]string{{"message": "field not found"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newTestTool(srv.URL, "test-key", false)

	args, _ := json.Marshal(map[string]string{"query": "{ invalid }"})
	result := tool.Execute(context.Background(), args)

	if result.Success {
		t.Error("expected failure for GraphQL errors")
	}
}

func TestLinearGraphQL_Execute_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer srv.Close()

	tool := newTestTool(srv.URL, "bad-key", false)

	args, _ := json.Marshal(map[string]string{"query": "{ viewer { name } }"})
	result := tool.Execute(context.Background(), args)

	if result.Success {
		t.Error("expected failure for 401")
	}
}

func TestLinearGraphQL_MissingQuery(t *testing.T) {
	tool := NewLinearGraphQL("key", false, slog.Default())

	args, _ := json.Marshal(map[string]string{})
	result := tool.Execute(context.Background(), args)

	if result.Success {
		t.Error("expected failure for missing query")
	}
}

func TestLinearGraphQL_EmptyArguments(t *testing.T) {
	tool := NewLinearGraphQL("key", false, slog.Default())

	result := tool.Execute(context.Background(), nil)

	if result.Success {
		t.Error("expected failure for nil arguments")
	}
}

func TestLinearGraphQL_RawStringQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{"data": map[string]any{"ok": true}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newTestTool(srv.URL, "key", false)

	// Pass query as raw string, not object
	args, _ := json.Marshal("{ viewer { name } }")
	result := tool.Execute(context.Background(), args)

	if !result.Success {
		t.Fatalf("expected success for raw string query, got: %s", result.Output)
	}
}

func TestLinearGraphQL_MutationBlocked(t *testing.T) {
	tool := NewLinearGraphQL("key", false, slog.Default())

	args, _ := json.Marshal(map[string]string{"query": "mutation { issueUpdate(id: \"x\") { success } }"})
	result := tool.Execute(context.Background(), args)

	if result.Success {
		t.Error("expected failure for blocked mutation")
	}
	if result.Output == "" {
		t.Error("expected error message")
	}
}

func TestLinearGraphQL_MutationAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{"data": map[string]any{"issueUpdate": map[string]bool{"success": true}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newTestTool(srv.URL, "key", true)

	args, _ := json.Marshal(map[string]string{"query": "mutation { issueUpdate(id: \"x\") { success } }"})
	result := tool.Execute(context.Background(), args)

	if !result.Success {
		t.Fatalf("expected success when mutations allowed, got: %s", result.Output)
	}
}

func TestLinearGraphQL_MultipleOperationsRejected(t *testing.T) {
	tool := NewLinearGraphQL("key", false, slog.Default())

	args, _ := json.Marshal(map[string]string{"query": "query A { viewer { name } } query B { viewer { id } }"})
	result := tool.Execute(context.Background(), args)

	if result.Success {
		t.Error("expected failure for multiple operations")
	}
}

func TestCountOperations(t *testing.T) {
	tests := []struct {
		query string
		want  int
	}{
		{"{ viewer { name } }", 1},
		{"query { viewer { name } }", 1},
		{"query GetViewer { viewer { name } }", 1},
		{"mutation { update { success } }", 1},
		{"query A { a } query B { b }", 2},
		{"", 0},
	}

	for _, tt := range tests {
		got := countOperations(tt.query)
		if got != tt.want {
			t.Errorf("countOperations(%q) = %d, want %d", tt.query, got, tt.want)
		}
	}
}

func TestIsMutation(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"mutation { update { success } }", true},
		{"mutation UpdateIssue { update { success } }", true},
		{"query { viewer { name } }", false},
		{"{ viewer { name } }", false},
	}

	for _, tt := range tests {
		got := isMutation(tt.query)
		if got != tt.want {
			t.Errorf("isMutation(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

// newTestTool creates a LinearGraphQL tool pointing at a test server.
func newTestTool(serverURL, apiKey string, allowMutations bool) *LinearGraphQL {
	tool := NewLinearGraphQL(apiKey, allowMutations, slog.Default())
	tool.endpoint = serverURL
	return tool
}
