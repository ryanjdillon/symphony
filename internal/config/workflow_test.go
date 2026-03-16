package config

import (
	"testing"
)

func TestParseWorkflow(t *testing.T) {
	input := []byte(`---
tracker:
  kind: linear
  project_slug: my-project
  api_key: $LINEAR_API_KEY
  active_states: ["Todo", "In Progress"]
polling:
  interval_ms: 5000
agent:
  kind: claude-code
  command: claude-code app-server
  max_concurrent_agents: 5
---
You are working on {{ .Issue.Identifier }}: {{ .Issue.Title }}
`)

	cfg, body, err := parseWorkflow(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Tracker.Kind != "linear" {
		t.Errorf("tracker.kind = %q, want %q", cfg.Tracker.Kind, "linear")
	}
	if cfg.Tracker.ProjectSlug != "my-project" {
		t.Errorf("tracker.project_slug = %q, want %q", cfg.Tracker.ProjectSlug, "my-project")
	}
	if cfg.Polling.IntervalMs != 5000 {
		t.Errorf("polling.interval_ms = %d, want %d", cfg.Polling.IntervalMs, 5000)
	}
	if cfg.Agent.Kind != "claude-code" {
		t.Errorf("agent.kind = %q, want %q", cfg.Agent.Kind, "claude-code")
	}
	if cfg.Agent.MaxConcurrentAgents != 5 {
		t.Errorf("agent.max_concurrent_agents = %d, want %d", cfg.Agent.MaxConcurrentAgents, 5)
	}
	if body == "" {
		t.Error("prompt body should not be empty")
	}
}

func TestParseWorkflow_CodexCompat(t *testing.T) {
	input := []byte(`---
tracker:
  kind: linear
  project_slug: test
  api_key: test-key
codex:
  command: codex app-server
  approval_policy: never
---
prompt body
`)

	cfg, _, err := parseWorkflow(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Agent.Kind != "codex" {
		t.Errorf("agent.kind = %q, want %q after codex compat", cfg.Agent.Kind, "codex")
	}
	if cfg.Agent.Command != "codex app-server" {
		t.Errorf("agent.command = %q, want %q", cfg.Agent.Command, "codex app-server")
	}
	if cfg.Agent.Config["approval_policy"] != "never" {
		t.Errorf("agent.config[approval_policy] = %v, want %q", cfg.Agent.Config["approval_policy"], "never")
	}
}

func TestParseWorkflow_MissingDelimiter(t *testing.T) {
	input := []byte(`no frontmatter here`)
	_, _, err := parseWorkflow(input)
	if err == nil {
		t.Error("expected error for missing frontmatter delimiter")
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	if len(cfg.Tracker.ActiveStates) != 2 {
		t.Errorf("expected 2 default active states, got %d", len(cfg.Tracker.ActiveStates))
	}
	if cfg.Polling.IntervalMs != 30000 {
		t.Errorf("polling.interval_ms = %d, want 30000", cfg.Polling.IntervalMs)
	}
	if cfg.Agent.MaxConcurrentAgents != 10 {
		t.Errorf("agent.max_concurrent_agents = %d, want 10", cfg.Agent.MaxConcurrentAgents)
	}
	if cfg.Agent.MaxTurns != 20 {
		t.Errorf("agent.max_turns = %d, want 20", cfg.Agent.MaxTurns)
	}
}

func TestResolveEnvVars(t *testing.T) {
	t.Setenv("TEST_SYMPHONY_KEY", "secret123")

	if got := ResolveEnvVars("$TEST_SYMPHONY_KEY"); got != "secret123" {
		t.Errorf("ResolveEnvVars($TEST_SYMPHONY_KEY) = %q, want %q", got, "secret123")
	}

	if got := ResolveEnvVars("plain-value"); got != "plain-value" {
		t.Errorf("ResolveEnvVars(plain-value) = %q, want %q", got, "plain-value")
	}

	if got := ResolveEnvVars("$NONEXISTENT_VAR_XYZ"); got != "$NONEXISTENT_VAR_XYZ" {
		t.Errorf("ResolveEnvVars($NONEXISTENT) = %q, want original", got)
	}
}

func TestValidate(t *testing.T) {
	cfg := &Config{
		Tracker: TrackerConfig{Kind: "linear", ProjectSlug: "test", APIKey: "key"},
		Agent:   AgentConfig{Command: "test-agent"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}

	cfg.Tracker.Kind = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for missing tracker.kind")
	}
}
