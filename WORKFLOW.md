---
tracker:
  kind: linear
  project_slug: symphony-beta
  api_key: $LINEAR_API_KEY
  active_states:
    - Todo
    - In Progress
  terminal_states:
    - Done
    - Closed
    - Cancelled
    - Canceled
polling:
  interval_ms: 30000
workspace:
  root: ~/code/symphony-workspaces
  hooks:
    after_create: |
      git clone --depth 1 https://github.com/ryanjdillon/symphony .
agent:
  kind: claude-code
  command: claude-code app-server
  max_concurrent_agents: 5
  max_turns: 20
  max_retry_backoff_ms: 300000
  config:
    turn_timeout_ms: 3600000
    stall_timeout_ms: 300000
---

You are working on {{ .Issue.Identifier }}: {{ .Issue.Title }}

{{ .Issue.Description }}

## Instructions

- Read AGENTS.md before starting work
- Run `just all` before completing to verify your changes pass the quality gate
- Create focused, atomic changes that address only the issue at hand
