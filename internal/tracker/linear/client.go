package linear

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

	"github.com/ryanjdillon/symphony/internal/tracker"
)

const graphqlEndpoint = "https://api.linear.app/graphql"

// overrideEndpoint is used in tests to point at a local httptest server.
var overrideEndpoint string

func (c *Client) endpoint() string {
	if overrideEndpoint != "" {
		return overrideEndpoint
	}
	return graphqlEndpoint
}

// Client implements tracker.Tracker for Linear via GraphQL.
type Client struct {
	apiKey         string
	projectSlug    string
	activeStates   []string
	terminalStates []string
	httpClient     *http.Client
	logger         *slog.Logger
}

// NewClient creates a new Linear GraphQL client.
func NewClient(apiKey, projectSlug string, activeStates, terminalStates []string, logger *slog.Logger) *Client {
	return &Client{
		apiKey:         apiKey,
		projectSlug:    projectSlug,
		activeStates:   activeStates,
		terminalStates: terminalStates,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		logger:         logger.With("component", "linear"),
	}
}

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphqlError  `json:"errors,omitempty"`
}

type graphqlError struct {
	Message string `json:"message"`
}

type issuesData struct {
	Issues struct {
		Nodes []rawIssue `json:"nodes"`
	} `json:"issues"`
}

type rawIssue struct {
	ID          string  `json:"id"`
	Identifier  string  `json:"identifier"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Priority    int     `json:"priority"`
	State       struct {
		Name string `json:"name"`
	} `json:"state"`
	BranchName string `json:"branchName"`
	URL        string `json:"url"`
	Labels     struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Relations struct {
		Nodes []struct {
			RelatedIssue struct {
				ID string `json:"id"`
			} `json:"relatedIssue"`
		} `json:"nodes"`
	} `json:"relations"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type issueStatesData struct {
	Issues struct {
		Nodes []struct {
			ID    string `json:"id"`
			State struct {
				Name string `json:"name"`
			} `json:"state"`
		} `json:"nodes"`
	} `json:"issues"`
}

// FetchCandidates returns issues in active states eligible for dispatch.
func (c *Client) FetchCandidates(ctx context.Context) ([]tracker.Issue, error) {
	c.logger.Info("fetching candidate issues", "project", c.projectSlug, "states", c.activeStates)
	return c.fetchIssuesByStates(ctx, c.activeStates)
}

// FetchTerminalIssues returns issues in terminal states for startup cleanup.
func (c *Client) FetchTerminalIssues(ctx context.Context) ([]tracker.Issue, error) {
	c.logger.Info("fetching terminal issues", "project", c.projectSlug, "states", c.terminalStates)
	return c.fetchIssuesByStates(ctx, c.terminalStates)
}

// FetchIssueStates returns current states for the given issue IDs.
func (c *Client) FetchIssueStates(ctx context.Context, ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}

	c.logger.Info("fetching issue states", "count", len(ids))

	resp, err := c.doQuery(ctx, graphqlRequest{
		Query:     queryIssueStatesByIDs,
		Variables: map[string]any{"ids": ids},
	})
	if err != nil {
		return nil, fmt.Errorf("fetching issue states: %w", err)
	}

	var data issueStatesData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("decoding issue states response: %w", err)
	}

	result := make(map[string]string, len(data.Issues.Nodes))
	for _, node := range data.Issues.Nodes {
		result[node.ID] = node.State.Name
	}
	return result, nil
}

func (c *Client) fetchIssuesByStates(ctx context.Context, states []string) ([]tracker.Issue, error) {
	resp, err := c.doQuery(ctx, graphqlRequest{
		Query: queryIssuesByStates,
		Variables: map[string]any{
			"projectSlug": c.projectSlug,
			"states":      states,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("fetching issues by states: %w", err)
	}

	var data issuesData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("decoding issues response: %w", err)
	}

	issues := make([]tracker.Issue, 0, len(data.Issues.Nodes))
	for _, raw := range data.Issues.Nodes {
		issue, err := normalizeIssue(raw)
		if err != nil {
			c.logger.Warn("skipping malformed issue", "id", raw.ID, "error", err)
			continue
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (c *Client) doQuery(ctx context.Context, req graphqlRequest) (*graphqlResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", httpResp.StatusCode, truncate(string(respBody), 256))
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		messages := make([]string, len(gqlResp.Errors))
		for i, e := range gqlResp.Errors {
			messages[i] = e.Message
		}
		return nil, fmt.Errorf("graphql errors: %s", strings.Join(messages, "; "))
	}

	if gqlResp.Data == nil {
		return nil, fmt.Errorf("response contains no data")
	}

	return &gqlResp, nil
}

func normalizeIssue(raw rawIssue) (tracker.Issue, error) {
	createdAt, err := time.Parse(time.RFC3339, raw.CreatedAt)
	if err != nil {
		return tracker.Issue{}, fmt.Errorf("parsing createdAt %q: %w", raw.CreatedAt, err)
	}

	updatedAt, err := time.Parse(time.RFC3339, raw.UpdatedAt)
	if err != nil {
		return tracker.Issue{}, fmt.Errorf("parsing updatedAt %q: %w", raw.UpdatedAt, err)
	}

	labels := make([]string, 0, len(raw.Labels.Nodes))
	for _, l := range raw.Labels.Nodes {
		labels = append(labels, strings.ToLower(l.Name))
	}

	blockedBy := make([]string, 0, len(raw.Relations.Nodes))
	for _, r := range raw.Relations.Nodes {
		blockedBy = append(blockedBy, r.RelatedIssue.ID)
	}

	var priority *int
	if raw.Priority != 0 {
		p := raw.Priority
		priority = &p
	}

	var description string
	if raw.Description != nil {
		description = *raw.Description
	}

	return tracker.Issue{
		ID:          raw.ID,
		Identifier:  raw.Identifier,
		Title:       raw.Title,
		Description: description,
		Priority:    priority,
		State:       raw.State.Name,
		BranchName:  raw.BranchName,
		URL:         raw.URL,
		Labels:      labels,
		BlockedBy:   blockedBy,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
