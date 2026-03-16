# Symphony (Go) — Agent Guide

> Read this file at session start. It contains everything you need to work in this codebase.

## Build & Test

```bash
# All-in-one quality gate (format + lint + vet + test)
just all

# Individual targets
just fmt        # gofumpt formatting
just lint       # golangci-lint
just vet        # go vet
just test       # go test ./...
just build      # compile binary
just docker-build  # build container image
```

**Always run `just all` before submitting work.** If it doesn't pass, your change isn't ready.

## Architecture

Symphony follows strict dependency layering. Each layer imports only from layers to its left:

```
Types → Config → Tracker → Workspace → Agent → Template → Orchestrator → Status → CLI
```

| Layer | Package | Imports From |
|-------|---------|-------------|
| Types | `internal/tracker` (Issue type) | stdlib only |
| Config | `internal/config` | stdlib, yaml, fsnotify |
| Tracker | `internal/tracker`, `internal/tracker/linear` | Config, Types |
| Workspace | `internal/workspace` | Config, Types |
| Agent | `internal/agent`, `internal/agent/*` | Types |
| Template | `internal/template` | Types |
| Orchestrator | `internal/orchestrator` | All internal packages |
| Status | `internal/status` | Orchestrator, Types |
| CLI | `cmd/symphony` | All internal packages |

**Violations are caught by `justlint`.** Do not introduce circular imports.

## Coding Conventions

- **Error handling**: Wrap errors with `fmt.Errorf("context: %w", err)`. Never swallow errors silently.
- **Logging**: Use `log/slog` with structured fields. Always include `issue_id`, `issue_identifier`, `session_id` when available.
- **Context**: All long-running or I/O operations take `context.Context` as first parameter.
- **Interfaces**: Define interfaces in the package that _uses_ them, not the package that _implements_ them. Exception: `tracker.Tracker` and `agent.Runner` are defined in their own packages because multiple implementations exist.
- **Testing**: Table-driven tests. Use `testify` only if already imported; prefer stdlib `testing`.
- **Naming**: Follow Go conventions — `NewClient` not `CreateClient`, `ErrNotFound` not `NotFoundError`.
- **Imports**: Group as: stdlib, external, internal. Use blank line separators.

## Common Pitfalls

- **Workspace paths**: Always validate that resolved paths are under `workspace.root`. Use `filepath.Rel` to check. Path traversal is a security issue.
- **Environment variable resolution**: The `$VAR_NAME` syntax in WORKFLOW.md is NOT shell expansion. It's our own resolution via `os.Getenv`. Don't shell-expand config values.
- **Hot reload**: Config changes must not affect running sessions. Only future dispatches pick up new config.
- **Concurrency**: The orchestrator state is the single source of truth. Access it through the orchestrator's methods, never directly. The orchestrator runs on a single goroutine poll loop; workers run in separate goroutines.
- **Agent subprocess cleanup**: Always send SIGTERM first, wait 5s grace, then SIGKILL. Orphan processes are unacceptable.

## Directory Structure

```
cmd/symphony/          CLI entrypoint
internal/config/       Workflow parser, typed config, file watcher
internal/orchestrator/ Poll loop, dispatch, retry, reconciliation
internal/tracker/      Tracker interface + Linear implementation
internal/agent/        Agent runner interface + implementations
internal/workspace/    Workspace lifecycle + hooks
internal/template/     Prompt template rendering
internal/status/       HTTP API, WebSocket, structured logging
deploy/                Dockerfile, k8s manifests
```

## Key Files

- `SPEC.md` — Full project specification. If behavior changes, update the spec.
- `WORKFLOW.md` — Example workflow configuration.
- `justfile` — All build/test/lint targets.
- `AGENTS.md` — This file. Update when you discover a new pitfall.
