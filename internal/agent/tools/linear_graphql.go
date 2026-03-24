package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ryanjdillon/symphony/internal/agent"
)

const linearGraphQLEndpoint = "https://api.linear.app/graphql"

// LinearGraphQL implements agent.ToolHandler for executing raw GraphQL queries
// against Linear using the orchestrator's configured API key.
type LinearGraphQL struct {
	apiKey         string
	allowMutations bool
	endpoint       string // overridable for testing; defaults to linearGraphQLEndpoint
	httpClient     *http.Client
	logger         *slog.Logger
}

// NewLinearGraphQL creates a new linear_graphql tool handler.
func NewLinearGraphQL(apiKey string, allowMutations bool, logger *slog.Logger) *LinearGraphQL {
	return &LinearGraphQL{
		apiKey:         apiKey,
		allowMutations: allowMutations,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		logger:         logger.With("tool", "linear_graphql"),
	}
}

var linearGraphQLInputSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"query": {
			"type": "string",
			"description": "A single GraphQL query or mutation document"
		},
		"variables": {
			"type": "object",
			"description": "Optional GraphQL variables"
		}
	},
	"required": ["query"]
}`)

func (t *LinearGraphQL) Spec() agent.ToolSpec {
	return agent.ToolSpec{
		Name:        "linear_graphql",
		Description: "Execute a raw GraphQL query against Linear using Symphony's configured auth.",
		InputSchema: linearGraphQLInputSchema,
	}
}

type linearGraphQLArgs struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

func (t *LinearGraphQL) Execute(ctx context.Context, arguments json.RawMessage) agent.ToolResult {
	args, err := t.parseArguments(arguments)
	if err != nil {
		return agent.FailureResult(err.Error())
	}

	if err := t.validate(args.Query); err != nil {
		return agent.FailureResult(err.Error())
	}

	t.logger.Info("executing query", "query_length", len(args.Query))

	result, err := t.executeQuery(ctx, args.Query, args.Variables)
	if err != nil {
		t.logger.Warn("query execution failed", "error", err)
		return agent.FailureResult(err.Error())
	}

	return agent.SuccessResult(result)
}

func (t *LinearGraphQL) parseArguments(arguments json.RawMessage) (*linearGraphQLArgs, error) {
	if len(arguments) == 0 {
		return nil, fmt.Errorf("missing arguments")
	}

	// Try parsing as a structured object first
	var args linearGraphQLArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		// Try parsing as a raw query string
		var rawQuery string
		if err2 := json.Unmarshal(arguments, &rawQuery); err2 == nil && rawQuery != "" {
			return &linearGraphQLArgs{Query: rawQuery}, nil
		}
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Query == "" {
		return nil, fmt.Errorf("query is required and must be a non-empty string")
	}

	return &args, nil
}

func (t *LinearGraphQL) validate(query string) error {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return fmt.Errorf("query is empty")
	}

	// Reject multiple operations (look for multiple top-level query/mutation/subscription keywords)
	opCount := countOperations(trimmed)
	if opCount > 1 {
		return fmt.Errorf("query contains %d operations; only single-operation documents are allowed", opCount)
	}

	// Block mutations unless explicitly allowed
	if !t.allowMutations && isMutation(trimmed) {
		return fmt.Errorf("mutations are not allowed; set agent.config.allow_linear_mutations to enable")
	}

	return nil
}

// countOperations counts top-level query/mutation/subscription keywords.
// This is a simple heuristic — not a full GraphQL parser.
func countOperations(query string) int {
	count := 0
	lower := strings.ToLower(query)
	for _, keyword := range []string{"query ", "query{", "mutation ", "mutation{", "subscription ", "subscription{"} {
		count += strings.Count(lower, keyword)
	}
	// Anonymous queries (just `{`) — if no named ops found and starts with `{`, count as 1
	if count == 0 && strings.HasPrefix(strings.TrimSpace(lower), "{") {
		count = 1
	}
	return count
}

// isMutation checks if the query is a mutation operation.
func isMutation(query string) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	return strings.HasPrefix(lower, "mutation")
}

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

func (t *LinearGraphQL) executeQuery(ctx context.Context, query string, variables map[string]any) (any, error) {
	reqBody := graphqlRequest{
		Query:     query,
		Variables: variables,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	ep := t.endpoint
	if ep == "" {
		ep = linearGraphQLEndpoint
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("linear API returned status %d: %s", resp.StatusCode, truncateBytes(respBody, 256))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Check for GraphQL errors
	if errors, ok := result["errors"]; ok {
		return nil, fmt.Errorf("GraphQL errors: %v", errors)
	}

	return result, nil
}

func truncateBytes(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "..."
}
