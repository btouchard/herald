# Architecture Overview

## System Diagram

```
Claude Chat (mobile/web)
  → HTTPS (MCP Streamable HTTP + OAuth 2.1)
  → Traefik / Caddy (TLS termination)
  → Herald (Go binary, port 8420)
    ├── MCP Handler (/mcp)
    ├── OAuth 2.1 Server (PKCE, token rotation)
    ├── Task Manager (goroutine pool, priority queue)
    ├── Claude Code Executor (os/exec, stream-json parsing)
    ├── SQLite (persistence)
    └── Notification Hub (ntfy, webhooks)
```

## Design Principles

### Single Binary

Everything is embedded in one Go executable (~15MB). The dashboard HTML/CSS/JS is compiled in via `go:embed`. No Docker, no runtime dependencies, no node_modules.

### Async-First

Every Claude Code task runs in its own goroutine, managed by a bounded worker pool. The MCP protocol follows a start/poll/result pattern — the client starts a task and checks back later.

### Stateless MCP, Stateful Backend

Each MCP request is independent. Herald doesn't rely on MCP session state. The actual state (tasks, tokens, history) lives in SQLite and in-memory structures.

### Fail-Safe

If Herald crashes, running Claude Code processes continue (they're independent OS processes). Results are persisted to disk. On restart, Herald recovers state from SQLite.

### Minimal Complexity

No premature abstractions. If an interface has only one implementation, it might not need to be an interface yet. Herald is a tool, not a framework.

## Component Architecture

```
cmd/herald (wiring)
  └── internal/mcp        → internal/task, internal/project, internal/auth
  └── internal/task        → internal/executor, internal/store, internal/notify
  └── internal/executor    → (os/exec, nothing internal)
  └── internal/store       → (modernc.org/sqlite, nothing internal)
  └── internal/notify      → (net/http, nothing internal)
  └── internal/api         → internal/task, internal/project
  └── internal/dashboard   → (go:embed, nothing internal)
```

Each `internal/` package is autonomous and communicates with others through interfaces. Dependency injection happens in `cmd/herald/main.go` only.

### Key Components

| Component | Package | Responsibility |
|---|---|---|
| **MCP Server** | `internal/mcp` | Handles MCP Streamable HTTP requests, registers tools |
| **Task Manager** | `internal/task` | Task lifecycle, priority queue, goroutine pool |
| **Executor** | `internal/executor` | Spawns Claude Code via `os/exec`, parses stream-json output |
| **Store** | `internal/store` | SQLite persistence — tasks, tokens, audit log |
| **Auth** | `internal/auth` | OAuth 2.1 server with PKCE, JWT tokens, token rotation |
| **Notify** | `internal/notify` | Notification hub — ntfy, webhooks, SSE |
| **Project** | `internal/project` | Project configuration, validation, Git status |
| **Dashboard** | `internal/dashboard` | Embedded web UI served via `go:embed` |
| **API** | `internal/api` | REST API for dashboard and automation |
| **Config** | `internal/config` | YAML loading, env var expansion, defaults |

### Key Interfaces

```go
// store.Store — persistence layer
type Store interface {
    CreateTask(ctx context.Context, task *Task) error
    GetTask(ctx context.Context, id string) (*Task, error)
    UpdateTask(ctx context.Context, task *Task) error
    ListTasks(ctx context.Context, filter Filter) ([]*Task, error)
}

// executor.Executor — task execution
type Executor interface {
    Execute(ctx context.Context, req Request) (*Result, error)
}

// notify.Notifier — notification delivery
type Notifier interface {
    Notify(ctx context.Context, event Event) error
}
```

Interfaces are defined by their consumers, not their implementors. Small (1-3 methods).

## Tech Stack

| Component | Choice | Rationale |
|---|---|---|
| Language | Go 1.26 | Single binary, cross-compilation, goroutines, `log/slog` |
| MCP | [mcp-go](https://github.com/mark3labs/mcp-go) | Streamable HTTP, official MCP protocol support |
| Router | [chi](https://github.com/go-chi/chi) | Lightweight, stdlib `net/http` compatible |
| Database | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | Pure Go SQLite, zero CGO |
| IDs | [google/uuid](https://github.com/google/uuid) | UUID generation |
| Config | [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) | Standard YAML parsing |
| Testing | [testify](https://github.com/stretchr/testify) | Assertions (assert/require) |
| Logging | `log/slog` (stdlib) | Structured logging, no external dependency |

6 direct dependencies. No ORM. No logging framework. No build toolchain.

## Request Flow

### MCP Tool Call

```
1. Claude Chat sends MCP request to /mcp
2. Traefik terminates TLS, forwards to 127.0.0.1:8420
3. Auth middleware validates Bearer token (JWT)
4. Rate limiter checks per-token quota
5. MCP server routes to tool handler (e.g., start_task)
6. Handler validates parameters
7. Task Manager creates task, enqueues
8. Worker pool picks up task, spawns Claude Code
9. Executor streams output, updates progress
10. On completion, result persisted to SQLite
11. Notification Hub fires events (ntfy, webhooks)
```

### Claude Code Execution

```
1. Prompt written to /tmp/herald/{task_id}/prompt.md
2. Executor runs: cat prompt.md | claude -p --output-format stream-json
3. Stream-json output parsed line by line
4. Progress events update task state in memory
5. Result event triggers completion
6. Exit code checked, task marked completed or failed
```

!!! note "Long prompts"
    Prompts are always piped via stdin, never passed as CLI arguments. This avoids argument length limits and keeps prompts out of `ps` output.
