# Symphony (Go) — Project Specification

> A Go implementation of the [OpenAI Symphony spec](https://github.com/openai/symphony/blob/main/SPEC.md)
> for orchestrating autonomous coding agents against Linear work queues.

## 1. Goals

1. **Implement the core Symphony orchestrator in Go**, conforming to the upstream SPEC.md contract.
2. **Support multiple coding agents** — Claude Code (primary), Amp, Gemini, Codex, Opencode, Ollama, and Pi — behind a unified agent runner interface.
3. **Linear as the initial issue tracker**, with an abstract tracker interface that supports future providers (GitHub Issues, Jira).
4. **First-class container/Kubernetes deployment** — runs in k3s (local) and AKS (work).
5. **Start lean** — core orchestrator only. Extensions (HTTP dashboard, SSH workers, `linear_graphql` tool) are tracked as future Linear issues and potentially implemented by Symphony itself.

## 2. Non-Goals (v1)

- Custom React frontend (v1 uses REST + WebSocket; React SPA is a future extension)
- SSH remote worker execution
- `linear_graphql` agent tool
- Multi-tracker support (architecture supports it; implementation is deferred)
- Persistent state across restarts (tracker is source of truth, per upstream spec)

---

## 3. Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                     Symphony (Go)                       │
│                                                         │
│  ┌──────────┐  ┌──────────────┐  ┌───────────────────┐ │
│  │ Workflow  │  │  Orchestrator│  │  Agent Runner     │ │
│  │ Loader   │──│  (scheduler) │──│  (subprocess mgr) │ │
│  └──────────┘  └──────┬───────┘  └───────────────────┘ │
│                       │                                 │
│  ┌──────────┐  ┌──────┴───────┐  ┌───────────────────┐ │
│  │ Config   │  │  Workspace   │  │  Tracker Client   │ │
│  │ Layer    │  │  Manager     │  │  (Linear GraphQL) │ │
│  └──────────┘  └──────────────┘  └───────────────────┘ │
│                                                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │  Status Surface (structured logs + optional HTTP) │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility |
|-----------|---------------|
| **Workflow Loader** | Parse `WORKFLOW.md` (YAML frontmatter + Markdown body), watch for changes, trigger hot-reload |
| **Config Layer** | Typed getters with defaults, `$ENV_VAR` resolution, validation |
| **Orchestrator** | Poll loop owner, single source of truth for scheduling, dispatch, retry, reconciliation |
| **Tracker Client** | Abstract interface; Linear GraphQL implementation fetches candidates, refreshes state, normalizes payloads |
| **Workspace Manager** | Maps issues → filesystem paths, lifecycle hooks, cleanup |
| **Agent Runner** | Abstract interface; implementations for each agent. Manages subprocess lifecycle, stdio JSON protocol, event streaming |
| **Status Surface** | Structured logging; optional HTTP API for observability |

---

## 4. Agent Runner Interface

The key differentiator from upstream: **agent-agnostic design**.

### 4.1 Interface Definition

```go
// AgentRunner launches and manages a coding agent subprocess.
type AgentRunner interface {
    // Name returns the agent identifier (e.g., "claude-code", "codex").
    Name() string

    // Start launches the agent process in the given workspace with the
    // provided prompt. Returns a Session for interacting with the running agent.
    Start(ctx context.Context, opts AgentStartOpts) (Session, error)
}

type AgentStartOpts struct {
    WorkspacePath string
    Prompt        string
    IssueContext  Issue
    Continuation  bool   // true if this is a follow-up turn
    MaxTurns      int
    TurnTimeout   time.Duration
    StallTimeout  time.Duration
    Config        map[string]any // agent-specific config from WORKFLOW.md
}

type Session interface {
    // Events returns a channel of streaming events from the agent.
    Events() <-chan AgentEvent

    // Wait blocks until the session completes and returns the outcome.
    Wait() RunOutcome

    // Stop terminates the agent process.
    Stop() error

    // SessionID returns the unique session identifier.
    SessionID() string
}

type AgentEvent struct {
    Type      string    // e.g., "turn/start", "turn/completed", "message"
    Timestamp time.Time
    Payload   json.RawMessage
    Tokens    TokenUsage
}

type RunOutcome int

const (
    Succeeded RunOutcome = iota
    Failed
    TimedOut
    Stalled
    CanceledByReconciliation
)
```

### 4.2 Agent Implementations (v1)

| Agent | Protocol | Notes |
|-------|----------|-------|
| **Claude Code** | `claude-code app-server` over stdio JSON | Primary. Same app-server protocol as Codex. |
| **Codex** | `codex app-server` over stdio JSON | Direct upstream compat. |
| **Amp** | TBD — research CLI/API interface | May need adapter. |
| **Gemini** | TBD — research CLI interface | May need adapter. |
| **Opencode** | TBD — research CLI interface | May need adapter. |
| **Ollama** | HTTP API at localhost:11434 | Different protocol; needs HTTP adapter wrapping stdio contract. |
| **Pi** | TBD — research interface | May need adapter. |

### 4.3 WORKFLOW.md Agent Selection

The `codex.command` field in WORKFLOW.md generalizes to an `agent` block:

```yaml
agent:
  kind: claude-code  # which AgentRunner to use
  command: claude-code app-server
  max_concurrent_agents: 10
  max_turns: 20
  # agent-specific config passed through as AgentStartOpts.Config
  config:
    model: claude-sonnet-4-6
    approval_policy: never
```

For backward compatibility with upstream WORKFLOW.md files, the presence of a `codex` block
implies `agent.kind: codex` and maps fields accordingly.

---

## 5. Tracker Interface

```go
// Tracker fetches and normalizes issues from a project management system.
type Tracker interface {
    // FetchCandidates returns issues in active states eligible for dispatch.
    FetchCandidates(ctx context.Context) ([]Issue, error)

    // FetchIssueStates returns current states for the given issue IDs (reconciliation).
    FetchIssueStates(ctx context.Context, ids []string) (map[string]string, error)

    // FetchTerminalIssues returns issues in terminal states (startup cleanup).
    FetchTerminalIssues(ctx context.Context) ([]Issue, error)
}

// Issue is the normalized domain model, tracker-agnostic.
type Issue struct {
    ID          string
    Identifier  string   // e.g., "SYM-123"
    Title       string
    Description string
    Priority    *int
    State       string
    BranchName  string
    URL         string
    Labels      []string
    BlockedBy   []string // issue IDs
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 5.1 Linear Implementation

- GraphQL client using `https://api.linear.app/graphql`
- Queries: `fetch_candidate_issues`, `fetch_issues_by_states`, `fetch_issue_states_by_ids`
- Normalizes labels to lowercase, parses blockers from inverse relations
- Handles: transport failures, non-200, GraphQL errors, malformed payloads
- Candidate-fetch failure → skip dispatch for this tick
- State-refresh failure → keep workers running, retry next tick

---

## 6. Orchestrator State Machine

### 6.1 Issue Lifecycle (internal)

```
Unclaimed ──→ Claimed ──→ Running ──→ RetryQueued ──→ (back to Claimed)
                                   └──→ Released (claim dropped)
```

### 6.2 In-Memory State

```go
type OrchestratorState struct {
    Running       map[string]*LiveSession  // issue ID → active session
    Claimed       map[string]struct{}       // reserved issue IDs
    RetryAttempts map[string]*RetryEntry   // issue ID → retry info
    Completed     map[string]struct{}       // bookkeeping
    TokenTotals   TokenUsage
    RateLimits    json.RawMessage          // latest snapshot
}
```

No persistent database. Restart recovery is tracker-driven per spec.

### 6.3 Poll Tick Sequence

1. **Reconcile** running issues (stall detection + state refresh)
2. **Validate** dispatch config
3. **Fetch** candidate issues from tracker
4. **Sort** by priority (ascending), then creation time (ascending)
5. **Dispatch** eligible issues while slots remain

### 6.4 Dispatch Eligibility

An issue is eligible when:
- All required fields present
- State is in `active_states`
- Not already claimed
- Global concurrency limit not reached
- Per-state concurrency limit not reached (if configured)
- Not blocked by non-terminal issues (for `Todo` state)

### 6.5 Retry and Backoff

| Scenario | Delay |
|----------|-------|
| Normal exit (continuation) | 1 second |
| Failure-driven | `10000 * 2^(attempt-1)` ms, capped at `max_retry_backoff_ms` |

---

## 7. Workspace Management

### 7.1 Path Convention

```
{workspace.root}/{sanitized_issue_identifier}/
```

Sanitization: replace non-`[a-zA-Z0-9_-]` with `_`.

### 7.2 Safety Invariants

1. Agent launches only within workspace path
2. Workspace path must be under configured root
3. Workspace key uses allowlisted characters only

### 7.3 Lifecycle Hooks

| Hook | When | Failure Behavior |
|------|------|-----------------|
| `after_create` | New workspace created | Fatal — abort |
| `before_run` | Before each agent attempt | Abort current attempt |
| `after_run` | After each attempt | Log and ignore |
| `before_remove` | Before workspace deletion | Log and ignore |

All hooks: configurable timeout (default 60s), run in shell with workspace as cwd.

---

## 8. Workflow Configuration

### 8.1 File Format

Same as upstream — YAML frontmatter + Markdown body in `WORKFLOW.md`:

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
  root: /tmp/workspaces
  hooks:
    after_create: "git clone --depth 1 $REPO_URL ."
    before_run: "git pull --rebase"
agent:
  kind: claude-code
  command: claude-code app-server
  max_concurrent_agents: 10
  max_turns: 20
  max_retry_backoff_ms: 300000
  config:
    model: claude-sonnet-4-6
    turn_timeout_ms: 3600000
    stall_timeout_ms: 300000
---

# Prompt Template

You are working on {{ issue.identifier }}: {{ issue.title }}

{{ issue.description }}
```

### 8.2 Template Rendering

- Variables: `{{ issue.* }}`, `{{ attempt }}`
- Strict mode: unknown variables or filters → fail the run attempt
- Empty body → minimal default prompt with issue identifier, title, body

### 8.3 Hot Reload

File watcher on `WORKFLOW.md`. Changes re-read and re-apply without restart:
- Affects future dispatch, retry scheduling, agent launches
- Does not interrupt running sessions

---

## 9. Reconciliation

### 9.1 Per-Tick Reconciliation

**Stall Detection:**
- Elapsed time since last event (or session start) vs `stall_timeout_ms`
- Exceeded → terminate worker, queue retry
- Disabled if timeout ≤ 0

**State Refresh:**
- Fetch current tracker states for all running issue IDs
- Terminal state → terminate + cleanup workspace
- Non-active state → terminate, no cleanup
- Active state → update snapshot

### 9.2 Startup Cleanup

On boot, query tracker for terminal-state issues and remove corresponding workspaces.

---

## 10. Observability

### 10.1 Structured Logging

Required context on every log line: `issue_id`, `issue_identifier`, `session_id`.

Format: `key=value` pairs. Use `slog` from Go stdlib.

### 10.2 HTTP API + WebSocket

Enabled via `--port` flag or `server.port` in WORKFLOW.md. Binds to loopback by default.

**REST Endpoints (upstream-compatible shapes):**

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/state` | Runtime summary JSON (running sessions, retry queue, token totals, rate limits) |
| `GET /api/v1/{identifier}` | Issue-specific debug details (workspace path, session state, event log, token counts) |
| `POST /api/v1/refresh` | Trigger immediate poll cycle (best-effort) |

**WebSocket Endpoint:**

| Endpoint | Description |
|----------|-------------|
| `GET /ws` | Real-time state updates via WebSocket |

The WebSocket protocol is simple JSON push — no Phoenix LiveView reimplementation:

```json
// Server → Client: state update (pushed on every orchestrator state change)
{
  "type": "state_update",
  "data": {
    "running": [
      {"issue_id": "...", "identifier": "SYM-123", "state": "In Progress",
       "session_id": "...", "turn_count": 3, "elapsed_s": 120}
    ],
    "retrying": [
      {"issue_id": "...", "identifier": "SYM-456", "attempt": 2,
       "due_at": "2026-03-16T12:00:00Z", "error": "stalled"}
    ],
    "tokens": {"input": 50000, "output": 12000, "total": 62000},
    "runtime_s": 3600,
    "rate_limits": {}
  }
}

// Client → Server: request immediate refresh
{"type": "refresh"}
```

State updates are broadcast whenever the orchestrator's in-memory state changes
(dispatch, completion, retry, reconciliation). Clients receive the full snapshot
each time — no diffing. This is simple, debuggable, and sufficient for the
expected number of concurrent dashboard viewers (1-3).

**Upstream frontend note:** The Phoenix LiveView dashboard uses a proprietary
binary WebSocket protocol that cannot be reused. Our REST endpoints match the
upstream `/api/v1/*` response shapes for tooling compatibility. A future React
frontend would consume both the REST API and the WebSocket for real-time updates.

---

## 11. Project Structure

```
symphony/
├── cmd/
│   └── symphony/
│       └── main.go              # CLI entrypoint
├── internal/
│   ├── config/
│   │   ├── config.go            # Typed config with defaults + $VAR resolution
│   │   └── workflow.go          # WORKFLOW.md parser (YAML frontmatter + markdown)
│   ├── orchestrator/
│   │   ├── orchestrator.go      # Poll loop, dispatch, retry, reconciliation
│   │   ├── state.go             # In-memory runtime state
│   │   └── scheduler.go         # Priority sorting, eligibility checks
│   ├── tracker/
│   │   ├── tracker.go           # Tracker interface
│   │   ├── issue.go             # Normalized domain model
│   │   └── linear/
│   │       ├── client.go        # Linear GraphQL client
│   │       └── queries.go       # GraphQL query definitions
│   ├── agent/
│   │   ├── runner.go            # AgentRunner + Session interfaces
│   │   ├── event.go             # AgentEvent, RunOutcome types
│   │   ├── claudecode/
│   │   │   └── runner.go        # Claude Code app-server implementation
│   │   ├── codex/
│   │   │   └── runner.go        # Codex app-server implementation
│   │   └── ollama/
│   │       └── runner.go        # Ollama HTTP adapter
│   ├── workspace/
│   │   ├── manager.go           # Workspace creation, cleanup, path sanitization
│   │   └── hooks.go             # Lifecycle hook execution
│   ├── template/
│   │   └── render.go            # Strict prompt template rendering
│   └── status/
│       ├── logger.go            # Structured slog setup
│       ├── server.go            # HTTP API server (REST + WebSocket)
│       └── ws.go                # WebSocket hub, broadcast, client handling
├── deploy/
│   ├── Dockerfile
│   ├── k8s/
│   │   ├── deployment.yaml
│   │   ├── configmap.yaml       # WORKFLOW.md mounted as ConfigMap
│   │   └── secret.yaml          # API keys
│   └── helm/                    # (future) Helm chart
├── SPEC.md                      # This file
├── WORKFLOW.md                   # Default workflow (user-provided)
├── go.mod
├── go.sum
├── LICENSE
└── Makefile
```

---

## 12. Deployment

### 12.1 Container Image

```dockerfile
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /symphony ./cmd/symphony

FROM alpine:3.21
RUN apk add --no-cache git openssh-client
COPY --from=build /symphony /usr/local/bin/symphony
ENTRYPOINT ["symphony"]
```

The agent binaries (claude-code, codex, etc.) must be available in the container. This requires either:
- A fat image with agents pre-installed
- A sidecar pattern where agents run in adjacent containers
- Volume-mounting agent binaries from the host

**Recommended approach:** Fat image per agent type, selected via WORKFLOW.md `agent.kind`.

### 12.2 Kubernetes — FluxCD GitOps

Aligns with existing k3s cluster patterns at `nixos/pc.ryanjdillon/k3s/`.

**Directory structure** (lives in the k3s repo, not this repo):

```
k3s/apps/symphony/
├── kustomization.yaml        # Kustomize entry point
├── app/
│   ├── namespace.yaml
│   ├── deployment.yaml       # Single replica
│   ├── configmap.yaml        # WORKFLOW.md content
│   ├── secret.sops.yaml      # LINEAR_API_KEY, ANTHROPIC_API_KEY (SOPS + age encrypted)
│   ├── pvc.yaml              # Workspace root persistence
│   ├── service.yaml          # ClusterIP for HTTP API + WebSocket
│   └── ingress.yaml          # Traefik IngressRoute (symphony.lan, symphony.tailnet.dillonteknisk.no)
```

**Key patterns (matching existing cluster conventions):**

- **Secrets:** SOPS-encrypted with age key, decrypted by FluxCD at reconciliation
- **Ingress:** Traefik IngressClass, cert-manager `letsencrypt` ClusterIssuer for TLS
- **Reloader:** Annotate with `reloader.stakater.com/auto: "true"` so ConfigMap/Secret
  changes trigger pod restart (hot-reload handles WORKFLOW.md changes, but secret
  rotation needs restart)
- **Scheduling:** No GPU needed — runs on general-compute nodes (no nodeSelector/tolerations)
- **Flux entry:** Added to `k3s/apps/kustomization.yaml` with `dependsOn: [infra]`
- **Reconciliation interval:** 10m (matches existing app pattern)

**Deployment spec highlights:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: symphony
  namespace: symphony
  annotations:
    reloader.stakater.com/auto: "true"
spec:
  replicas: 1                    # Single-authority orchestrator
  strategy:
    type: Recreate               # No rolling update — only one instance allowed
  template:
    spec:
      containers:
        - name: symphony
          image: ghcr.io/<user>/symphony:latest
          args: ["--workflow", "/etc/symphony/WORKFLOW.md", "--port", "8080"]
          ports:
            - containerPort: 8080   # REST API + WebSocket
          volumeMounts:
            - name: workflow
              mountPath: /etc/symphony
            - name: workspaces
              mountPath: /var/lib/symphony/workspaces
          env:
            - name: LINEAR_API_KEY
              valueFrom:
                secretKeyRef:
                  name: symphony-secrets
                  key: LINEAR_API_KEY
            - name: ANTHROPIC_API_KEY
              valueFrom:
                secretKeyRef:
                  name: symphony-secrets
                  key: ANTHROPIC_API_KEY
      volumes:
        - name: workflow
          configMap:
            name: symphony-workflow
        - name: workspaces
          persistentVolumeClaim:
            claimName: symphony-workspaces
```

### 12.3 Target Clusters

| Cluster | GitOps | Secrets | Ingress | Notes |
|---------|--------|---------|---------|-------|
| Local k3s (faria/solimar/laconchita) | FluxCD + Kustomize | SOPS + age | Traefik | Primary dev, general-compute nodes |
| AKS (work) | TBD — likely same FluxCD pattern | Azure Key Vault or SOPS | Azure Ingress / Traefik | Deferred, same manifests with overlays |

---

## 13. Dependencies (Go Modules)

| Module | Purpose |
|--------|---------|
| `gopkg.in/yaml.v3` | YAML frontmatter parsing |
| `github.com/fsnotify/fsnotify` | WORKFLOW.md file watching |
| `github.com/shurcooL/graphql` or raw HTTP | Linear GraphQL client |
| `log/slog` (stdlib) | Structured logging |
| `text/template` (stdlib) | Prompt template rendering |
| `os/exec` (stdlib) | Agent subprocess management |
| `encoding/json` (stdlib) | JSON protocol over stdio |
| `net/http` (stdlib) | HTTP API server |
| `github.com/gorilla/websocket` or `nhooyr.io/websocket` | WebSocket server |

Minimal dependency footprint. Prefer stdlib where possible.

---

## 14. Extension Roadmap

These are **not in scope for v1** but are documented here for future Linear issues.
Reference the upstream repo for implementation inspiration.

| Extension | Upstream Reference | Notes |
|-----------|-------------------|-------|
| React Frontend | `elixir/lib/symphony_web/live/dashboard_live.ex` for data model | SPA consuming REST + WebSocket; replaces need for LiveView |
| SSH Workers | SPEC.md §"Optional SSH Extension" | Remote agent execution on other nodes |
| `linear_graphql` Tool | SPEC.md §"Optional linear_graphql Client Tool" | Agent can query Linear directly during runs |
| GitHub Issues Tracker | N/A | New `Tracker` implementation |
| Jira Tracker | N/A | New `Tracker` implementation |
| Metrics/Prometheus | N/A | Export token usage, concurrency, retry counts via `/metrics` |
| Multi-workflow | N/A | Run multiple WORKFLOW.md files simultaneously |
| AKS Deployment | N/A | Kustomize overlays for Azure, Key Vault integration |

---

## 15. Conformance

This implementation targets **Core Conformance** from the upstream spec:

- [x] Workflow path resolution (explicit and default)
- [x] Dynamic reload without restart
- [x] Config defaults and `$VAR` resolution
- [x] Polling loop and dispatch validation
- [x] Workspace creation and reuse
- [x] Workspace hooks with timeout
- [x] Candidate selection and sorting
- [x] Concurrency control (global and per-state)
- [x] Retry queue with exponential backoff
- [x] Reconciliation stopping/cleanup
- [x] Structured logging with context
- [x] Operator-visible validation failures

Extension conformance items are deferred to the roadmap above.
