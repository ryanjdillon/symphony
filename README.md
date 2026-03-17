# Symphony

A Go implementation of the [OpenAI Symphony](https://github.com/openai/symphony) orchestrator for autonomous coding agents.

Symphony monitors a Linear project board, spawns isolated coding agents for each issue, and manages their lifecycle — retries, concurrency, workspace isolation, and reconciliation — so you manage *work*, not agents.

## How it works

```
Linear Board                    Symphony                         Agents
┌──────────┐     poll      ┌──────────────┐    spawn       ┌─────────────┐
│ Todo     │────────────>  │ Orchestrator │──────────────> │ Claude Code │
│ In Prog  │               │              │                │ Codex       │
│ Done     │  <────────────│  reconcile   │  <──────────── │ ...         │
└──────────┘   state sync  └──────────────┘   events/done  └─────────────┘
```

1. Polls Linear for issues in active states (Todo, In Progress, etc.)
2. Sorts by priority, checks eligibility (concurrency limits, blockers)
3. Creates an isolated workspace per issue (git clone via hooks)
4. Launches a coding agent subprocess with a rendered prompt
5. Streams events, tracks tokens, detects stalls
6. On completion: schedules continuation or retries with exponential backoff
7. Reconciles with Linear each tick — stops work on issues moved to terminal states

## Key features

- **Agent-agnostic** — pluggable `Runner` interface supports Claude Code, Codex, and future agents
- **Linear integration** — GraphQL client with candidate fetching, state reconciliation, blocker detection
- **Workspace isolation** — per-issue directories with lifecycle hooks (after_create, before_run, etc.)
- **Hot-reload config** — change `WORKFLOW.md` without restarting; running sessions are unaffected
- **Exponential backoff** — failed runs retry with `10s * 2^(attempt-1)`, capped at configurable max
- **Status API** — REST endpoints + WebSocket for real-time orchestrator state
- **Upstream compatible** — conforms to the [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md), including backward-compatible `codex` config block

## Quick start

### Prerequisites

- [Nix](https://nixos.org/) (for dev shell) or Go 1.25+, gofumpt, golangci-lint, just
- A [Linear API key](https://linear.app/settings/api)

### Setup

```bash
git clone https://github.com/ryanjdillon/symphony.git
cd symphony

# Enter dev shell (provides Go, linters, just)
nix develop

# Run quality gate
just all

# Set your Linear API key
export LINEAR_API_KEY=lin_api_...

# Edit WORKFLOW.md to point at your Linear project
# Then run:
just run
```

### Configuration

Symphony is configured via a single `WORKFLOW.md` file with YAML frontmatter and a Markdown prompt template:

```yaml
---
tracker:
  kind: linear
  project_slug: my-project
  api_key: $LINEAR_API_KEY
  active_states: ["Todo", "In Progress"]
  terminal_states: ["Done", "Closed", "Cancelled"]
polling:
  interval_ms: 30000
workspace:
  root: ~/code/symphony-workspaces
  hooks:
    after_create: "git clone --depth 1 $REPO_URL ."
agent:
  kind: claude-code
  command: claude-code app-server
  max_concurrent_agents: 5
  max_turns: 20
---

You are working on {{ .Issue.Identifier }}: {{ .Issue.Title }}

{{ .Issue.Description }}
```

See [SPEC.md](SPEC.md) for full configuration reference.

## Development

```bash
just              # list all commands
just all          # fmt + vet + lint + test + build
just test         # run tests
just lint         # golangci-lint
just build        # compile binary
just coverage     # test coverage report
just docker-build # build container image
```

### Project structure

```
cmd/symphony/         CLI entrypoint
internal/
  config/             WORKFLOW.md parser, typed config, hot-reload watcher
  orchestrator/       Poll loop, dispatch, retry, reconciliation, state machine
  tracker/            Tracker interface + Linear GraphQL implementation
  agent/              Runner interface + app-server protocol + agent wrappers
  workspace/          Workspace lifecycle, path sanitization, hook execution
  template/           Strict prompt template rendering
  status/             HTTP REST API, WebSocket hub, structured logging
deploy/               Dockerfile, k8s manifests
```

### Testing

44 tests across 7 packages covering config parsing, workspace management, template rendering, orchestrator scheduling/state, Linear client (mock HTTP), and HTTP API endpoints.

```bash
just test           # run all tests
just test-verbose   # verbose output
just coverage       # coverage report
```

## Deployment

### Docker

```bash
just docker-build
docker run -e LINEAR_API_KEY=lin_api_... -v ./WORKFLOW.md:/etc/symphony/WORKFLOW.md symphony --workflow /etc/symphony/WORKFLOW.md
```

### Kubernetes

Symphony runs as a single-replica Deployment (single-authority orchestrator). See [SPEC.md §12.2](SPEC.md) for FluxCD/Kustomize manifests matching k3s cluster conventions.

## Roadmap

| Feature | Status |
|---------|--------|
| Core orchestrator | Done |
| Linear tracker | Done |
| Claude Code runner | Done |
| Codex runner | Done |
| REST API + WebSocket | Done |
| React frontend | Planned |
| SSH remote workers | Planned |
| GitHub Issues tracker | Planned |
| Jira tracker | Planned |
| Prometheus metrics | Planned |
| Multi-workflow | Planned |

## Documentation

- [SPEC.md](SPEC.md) — full project specification
- [AGENTS.md](AGENTS.md) — guide for coding agents working in this codebase
- [WORKFLOW.md](WORKFLOW.md) — example workflow configuration

## License

[Apache 2.0](LICENSE)
