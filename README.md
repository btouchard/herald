# Herald

**Bridge Claude Chat to Claude Code. Command your workstation from your phone.**

[![Go 1.26+](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Status: Alpha](https://img.shields.io/badge/Status-Alpha-orange)]()

:fr: [Version française](README_FR.md)

---

Claude Chat and Claude Code live in two separate worlds. One runs in your browser and on your phone. The other runs in your terminal and writes actual code. They don't talk to each other.

Herald fixes that. It's a self-hosted MCP server that connects Claude Chat to Claude Code using Anthropic's official [Custom Connectors](https://docs.anthropic.com/en/docs/claude-code/mcp) protocol. You stay in Claude Chat — Herald dispatches the work to Claude Code running on your machine.

```
  📱 Claude Chat (phone / web)
       │
       ▼ MCP over HTTPS
  🖥️  Herald (your workstation)
       │
       ▼ spawns & manages
  ⚡ Claude Code (executes tasks)
```

## The Workflow

You're on your phone. You open Claude Chat and say:

> "Refactor the auth middleware in my-api to use JWT instead of session cookies."

Here's what happens:

```
You (Claude Chat)          Herald                     Claude Code
─────────────────          ──────                     ───────────
"Refactor auth..."    ──►  start_task
                           → creates branch
                           → spawns Claude Code  ──►  reads codebase
                                                      refactors auth
                                                      runs tests
                                                      commits changes
                      ◄──  task_id: herald-a1b2c3d4

"How's it going?"     ──►  check_task
                      ◄──  ✅ Completed (4m 12s)
                           4 files changed (+127/-23)

"Show me the diff"    ──►  get_diff
                      ◄──  auth/middleware.go
                           +func ValidateJWT(...)
                           -func CheckSession(...)
                           ...
```

All from your phone. Your workstation did the heavy lifting.

## Features

- **Native MCP bridge** — Uses Anthropic's official Custom Connectors. Not a hack, not a wrapper.
- **Async task execution** — Start tasks, check progress, get results. No long-polling, no timeouts.
- **Git branch isolation** — Each task gets its own branch. Your main branch stays clean.
- **Session resumption** — Multi-turn Claude Code conversations. Continue where you left off.
- **Multi-project support** — Configure multiple projects with per-project security policies.
- **Per-project allowed tools** — Control exactly which tools Claude Code can use per project.
- **OAuth 2.1 + PKCE** — Proper auth. Not a shared API key.
- **SQLite persistence** — Tasks survive server restarts. History is searchable.
- **Push notifications** — Get notified via [ntfy](https://ntfy.sh) when tasks complete or fail.
- **Single binary** — One Go binary, ~15MB. No Docker required, no runtime dependencies.
- **Zero CGO** — Cross-compiles to any platform Go supports.
- **6 dependencies** — chi, mcp-go, modernc/sqlite, uuid, yaml, testify. That's it.

## Quick Start

### Prerequisites

- **Go 1.26+**
- **Claude Code CLI** installed and authenticated (`claude --version`)
- **Anthropic account** with Custom Connectors access
- **A domain with HTTPS** (Traefik, Caddy, or any reverse proxy for TLS)

### Build

```bash
git clone https://github.com/kolapsis/herald.git
cd herald
make build
```

This produces `bin/herald` — a statically-linked binary with zero CGO.

### Configure

```bash
mkdir -p ~/.config/herald
cp configs/herald.example.yaml ~/.config/herald/herald.yaml
```

Edit `~/.config/herald/herald.yaml`:

```yaml
server:
  host: "127.0.0.1"
  port: 8420
  public_url: "https://herald.yourdomain.com"

auth:
  client_id: "herald-claude-chat"
  client_secret: "${HERALD_CLIENT_SECRET}"

projects:
  my-api:
    path: "/home/you/projects/my-api"
    description: "Main backend API"
    default: true
    allowed_tools:
      - "Read"
      - "Write"
      - "Edit"
      - "Bash(git *)"
      - "Bash(go *)"
      - "Bash(make *)"
    git:
      auto_branch: true
      auto_stash: true
      branch_prefix: "herald/"
```

Set the required secret:

```bash
export HERALD_CLIENT_SECRET="$(openssl rand -hex 32)"
```

### Run

```bash
./bin/herald serve
# herald is ready addr=127.0.0.1:8420
```

### Connect from Claude Chat

1. Go to **Claude Chat** → **Settings** → **Custom Connectors**
2. Add a new MCP connector:
   - **URL**: `https://herald.yourdomain.com/mcp`
   - **Auth**: OAuth 2.1 (Herald handles the flow)
3. Claude Chat will discover Herald's 9 tools automatically
4. Start talking to Claude — it can now dispatch tasks to your machine

## Configuration Reference

<details>
<summary>Full herald.yaml with all options</summary>

```yaml
server:
  host: "127.0.0.1"          # Always localhost — reverse proxy handles external
  port: 8420
  public_url: "https://herald.yourdomain.com"
  log_level: "info"           # debug, info, warn, error
  # log_file: "/var/log/herald.log"

auth:
  client_id: "herald-claude-chat"
  client_secret: "${HERALD_CLIENT_SECRET}"
  admin_password_hash: "${HERALD_ADMIN_PASSWORD_HASH}"
  access_token_ttl: 1h
  refresh_token_ttl: 720h    # 30 days

  # API tokens for REST API / curl / automation
  # api_tokens:
  #   - name: "local"
  #     token_hash: "${HERALD_API_TOKEN_HASH}"
  #     scope: "*"

database:
  path: "~/.config/herald/herald.db"
  retention_days: 90

execution:
  claude_path: "claude"       # Path to Claude Code binary
  default_timeout: 30m
  max_timeout: 2h
  work_dir: "~/.config/herald/work"
  max_concurrent: 3           # Max parallel Claude Code instances
  env:
    CLAUDE_CODE_ENTRYPOINT: "herald"
    CLAUDE_CODE_DISABLE_AUTO_UPDATE: "1"

notifications:
  ntfy:
    enabled: false
    server: "https://ntfy.sh"
    topic: "herald"
    # token: "${HERALD_NTFY_TOKEN}"
    events:
      - "task.completed"
      - "task.failed"

  # webhooks:
  #   - name: "n8n"
  #     url: "https://n8n.example.com/webhook/herald"
  #     secret: "${HERALD_WEBHOOK_SECRET}"
  #     events: ["task.completed", "task.failed"]

projects:
  my-api:
    path: "/home/you/projects/my-api"
    description: "Main backend API"
    default: true
    allowed_tools:
      - "Read"
      - "Write"
      - "Edit"
      - "Bash(git *)"
      - "Bash(go *)"
      - "Bash(make *)"
    max_concurrent_tasks: 1
    git:
      auto_branch: true
      auto_stash: true
      auto_commit: true
      branch_prefix: "herald/"

rate_limit:
  requests_per_minute: 60
  burst: 10

dashboard:
  enabled: true
```

</details>

## MCP Tools

Herald exposes 9 tools through the MCP protocol. Claude Chat discovers and uses them automatically.

| Tool | Description |
|---|---|
| `start_task` | Launch a Claude Code task. Returns a task ID immediately. Supports priority, timeout, dry run, session resumption, and Git branch options. |
| `check_task` | Check status and progress of a running task. Optionally include recent output lines. |
| `get_result` | Get the full result of a completed task. Formats: `summary`, `full`, or `json`. |
| `list_tasks` | List tasks with filters (status, project, time range, limit). |
| `cancel_task` | Cancel a running or queued task. Optionally revert Git changes. |
| `get_diff` | Show Git diff for a task's branch or a project's uncommitted changes. |
| `list_projects` | List configured projects with their Git status and description. |
| `read_file` | Read a file from a project directory. Path-safe — cannot escape the project root. |
| `get_logs` | View logs and activity history. Filter by task, level, or count. |

## Architecture

```
Claude Chat (mobile/web)
  → HTTPS (MCP Streamable HTTP + OAuth 2.1)
  → Traefik / Caddy (reverse proxy, TLS termination)
  → Herald (Go binary, port 8420)
    ├── MCP Handler (/mcp)
    ├── OAuth 2.1 Server (PKCE, token rotation)
    ├── Task Manager (goroutine pool, priority queue)
    ├── Claude Code Executor (os/exec, stream-json parsing)
    ├── SQLite (task persistence, auth tokens)
    └── Notification Hub (ntfy, webhooks)
```

### Design Principles

- **Single binary** — Everything embedded. HTML dashboard via `go:embed`. No external runtime.
- **Async-first** — Each task is a goroutine. Start/check/result polling pattern.
- **Stateless MCP, stateful backend** — MCP requests are independent. State lives in SQLite + memory.
- **Fail-safe** — If Herald crashes, running Claude Code processes continue. Results persist on disk.

### Tech Stack

| Component | Choice | Why |
|---|---|---|
| Language | Go 1.26 | Single binary, cross-compilation, goroutines |
| MCP | [mcp-go](https://github.com/mark3labs/mcp-go) | Streamable HTTP, official protocol support |
| HTTP Router | [chi](https://github.com/go-chi/chi) | Lightweight, stdlib-compatible |
| Database | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | Pure Go SQLite, zero CGO |
| Logging | `log/slog` | Go stdlib, structured, multi-handler |
| Config | `gopkg.in/yaml.v3` | Standard YAML parsing |

6 direct dependencies. No ORM, no logging framework, no build toolchain.

## Security

Herald exposes Claude Code over the network. Security is not optional.

- **Localhost only** — Herald binds to `127.0.0.1`. A reverse proxy (Traefik, Caddy) handles TLS and external access.
- **OAuth 2.1 + PKCE** — Every MCP request requires a valid Bearer token. No shared keys.
- **Short-lived tokens** — Access tokens expire in 1 hour. Refresh tokens rotate on each use.
- **Path traversal protection** — `read_file` resolves paths and verifies they stay within the project root. Symlink escapes are blocked.
- **Per-project tool restrictions** — Each project defines exactly which tools Claude Code can use. No blanket permissions.
- **Rate limiting** — 60 requests/minute per token by default.
- **Task timeouts** — Every task has a deadline (default 30 min). No infinite processes.
- **No prompt injection** — Herald passes prompts to Claude Code unmodified. No enrichment, no system prompt injection, no rewriting.
- **Audit trail** — Every action is logged with timestamp and identity.

## Deployment with Traefik

Herald is designed to sit behind a reverse proxy. Here's a minimal `docker-compose.yml`:

```yaml
services:
  traefik:
    image: traefik:v3
    command:
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.le.acme.email=you@example.com"
      - "--certificatesresolvers.le.acme.storage=/letsencrypt/acme.json"
      - "--certificatesresolvers.le.acme.httpchallenge.entrypoint=web"
    ports:
      - "443:443"
    volumes:
      - "./letsencrypt:/letsencrypt"

  herald:
    build: .
    network_mode: host     # Needs access to Claude Code on host
    volumes:
      - "~/.config/herald:/root/.config/herald"
      - "~/projects:/root/projects:ro"
    environment:
      - HERALD_CLIENT_SECRET
    labels:
      - "traefik.http.routers.herald.rule=Host(`herald.yourdomain.com`)"
      - "traefik.http.routers.herald.tls.certresolver=le"
      - "traefik.http.services.herald.loadbalancer.server.port=8420"
```

> **Note**: Running Herald as a native binary (not in Docker) is recommended for the best experience, since it needs direct access to Claude Code and your project files.

## Roadmap

| Version | Status | Focus |
|---|---|---|
| **v0.1** | :white_check_mark: Done | Core MCP server, async task execution, Git integration, OAuth 2.1, SQLite persistence |
| **v0.2** | :arrows_counterclockwise: In progress | Shared memory — bidirectional context between Claude Chat and Claude Code |
| **v0.3** | :clipboard: Planned | Real-time dashboard (embedded web UI with SSE) |
| **v1.0** | :rocket: Future | Stable API, managed hosting option, plugin system |

## Contributing

Herald is in early alpha. Contributions are welcome.

1. Fork the repo
2. Create a branch (`feat/your-feature` or `fix/your-fix`)
3. Write tests for non-trivial changes
4. Run `make lint && make test`
5. Open a PR

Please follow [Conventional Commits](https://www.conventionalcommits.org/) for commit messages.

## License

[MIT](LICENSE) — Kolapsis
