# Symphony

Autonomous coding agent orchestrator — polls Linear for work, spawns isolated agents (Claude Code, Codex, etc.), manages retries, concurrency, and reconciliation. Go implementation of the [OpenAI Symphony spec](https://github.com/openai/symphony).

Symphony monitors a project board, spawns isolated coding agents for each issue, and manages their lifecycle so you manage *work*, not agents.

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
- **Multi-workflow** — run multiple `WORKFLOW.md` files simultaneously, each targeting different projects or agents; `--workflow-dir` watches a directory for dynamic add/remove
- **SSH remote workers** — execute agent sessions on remote hosts via SSH with least-loaded host selection, per-host concurrency limits, health tracking, and continuation affinity
- **Agent tools** — extensible `ToolHandler` interface; includes `linear_graphql` tool for agents to query Linear during sessions (mutation guard, single-operation validation)
- **Linear integration** — GraphQL client with candidate fetching, state reconciliation, blocker detection via `inverseRelations`
- **Workspace isolation** — per-issue directories with lifecycle hooks (after_create, before_run, after_run, before_remove)
- **Hot-reload config** — change `WORKFLOW.md` without restarting; running sessions are unaffected
- **Exponential backoff** — failed runs retry with `10s * 2^(attempt-1)`, capped at configurable max
- **Status API** — REST endpoints + WebSocket for real-time orchestrator state
- **React dashboard** — real-time UI showing running sessions, retry queue, token usage, with WebSocket auto-reconnect; embedded in the Go binary
- **OpenTelemetry** — metrics (counters, gauges, histograms) with OTLP gRPC exporter; includes workflow, state, host, and reason attributes; no-op when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset
- **CI/CD** — GitHub Actions with conventional commit enforcement (commitlint), release-please for semantic versioning, Docker image published to GHCR on tag
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

Symphony is configured via `WORKFLOW.md` files with YAML frontmatter and a Markdown prompt template:

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
worker:
  ssh_hosts: ["dev-server-1", "dev-server-2"]
  max_concurrent_agents_per_host: 3
---

You are working on {{ .Issue.Identifier }}: {{ .Issue.Title }}

{{ .Issue.Description }}
```

Run multiple workflows simultaneously:

```bash
symphony --workflow backend.md --workflow frontend.md
# Or watch a directory:
symphony --workflow-dir ./workflows/
```

See [SPEC.md](SPEC.md) for full configuration reference.

## Development

```bash
just              # list all commands
just all          # fmt + vet + lint + test + test-frontend + build
just ci           # same checks as GitHub Actions
just test         # Go tests
just test-frontend # Vitest frontend tests
just lint         # golangci-lint
just build        # frontend + Go binary with embedded dashboard
just coverage     # test coverage report
just docker-build # build container image
just run          # build and run with local WORKFLOW.md
just dev-frontend # start Vite dev server with API proxy
```

### Project structure

```
cmd/symphony/         CLI entrypoint, multi-workflow support
internal/
  config/             WORKFLOW.md parser, typed config, hot-reload watcher
  orchestrator/       Poll loop, dispatch, retry, reconciliation, multi-orchestrator
  tracker/            Tracker interface + Linear GraphQL client
  agent/              Runner/Session interfaces, app-server protocol, SSH runner
  agent/tools/        Agent tool implementations (linear_graphql)
  worker/             SSH host manager (selection, health, affinity)
  workspace/          Workspace lifecycle, path sanitization, hook execution
  template/           Strict prompt template rendering
  status/             HTTP REST API, WebSocket hub, embedded frontend
  telemetry/          OpenTelemetry metrics provider and instruments
frontend/             React + TypeScript + Tailwind v4 dashboard (Vite)
deploy/               Multi-stage Dockerfile, k8s manifests
```

### Testing

117 tests (78 Go + 39 frontend) covering:
- Config parsing, codex backward compat, env var resolution, validation
- Workspace sanitization, creation, removal, hooks, timeouts
- Template rendering (strict mode, defaults, attempts)
- Orchestrator scheduling, state machine, retry backoff
- Linear client (mock HTTP, GraphQL errors, normalization)
- HTTP API endpoints (state, issue detail, refresh)
- Tool handler helpers, linear_graphql (validation, mutation guard, mock HTTP)
- SSH host manager (selection, capacity, health, affinity, failback)
- Frontend components, hooks, API client, WebSocket

```bash
just test            # Go tests
just test-frontend   # Vitest
just coverage        # Go coverage report
```

## Deployment

### Docker

```bash
just docker-build
docker run -e LINEAR_API_KEY=lin_api_... \
  -v ./WORKFLOW.md:/etc/symphony/WORKFLOW.md \
  symphony --workflow /etc/symphony/WORKFLOW.md --port 8080
```

### Kubernetes

Symphony runs as a single-replica Deployment (single-authority orchestrator) with FluxCD GitOps:

- **ConfigMap** for WORKFLOW.md (editable via git)
- **SOPS-encrypted Secret** for API keys
- **PVC** for workspace persistence
- **Traefik IngressRoute** for dashboard access
- **OTEL** env vars pointing at the cluster's collector

See [SPEC.md §12.2](SPEC.md) for full k8s manifest reference.

### CI/CD

Pushing to `main` triggers:
1. **release-please** creates/updates a release PR with auto-generated changelog
2. Merging the release PR creates a **GitHub release** and **version tag**
3. The tag triggers a **Docker build** → pushed to `ghcr.io/ryanjdillon/symphony`

PRs require passing: commitlint, lint, Go tests, frontend tests, build.

## Roadmap

| Feature | Status |
|---------|--------|
| Core orchestrator | Done |
| Linear tracker | Done |
| Claude Code + Codex runners | Done |
| REST API + WebSocket | Done |
| React dashboard | Done |
| Agent tools (linear_graphql) | Done |
| SSH remote workers | Done |
| Multi-workflow | Done |
| OpenTelemetry metrics | Done |
| CI/CD + semantic versioning | Done |
| k3s deployment (FluxCD) | Done |
| Claude Pro/Max OAuth auth | Planned |
| Workflow prompt catalog | Planned |
| Global prompt preamble | Planned |
| GitHub Issues tracker | Backlog |
| Jira tracker | Backlog |
| AKS deployment | Backlog |

## Documentation

- [SPEC.md](SPEC.md) — full project specification
- [AGENTS.md](AGENTS.md) — guide for coding agents working in this codebase
- [WORKFLOW.md](WORKFLOW.md) — example workflow configuration

## License

[Apache 2.0](LICENSE)
