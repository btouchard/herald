# The Self-Hosting Session

This page documents the build journey of Herald — a project that was itself built using the workflow it enables.

## The Premise

Herald was built from a phone, using Claude Chat connected to Claude Code through exactly the kind of bridge Herald provides. The self-hosting moment — where Herald was used to build Herald — is the ultimate proof of concept.

## Build Log

### v0.1.0 — Core Foundation

The initial version shipped with:

- **MCP server** with Streamable HTTP transport
- **9 MCP tools**: `start_task`, `check_task`, `get_result`, `list_tasks`, `cancel_task`, `get_diff`, `list_projects`, `read_file`, `get_logs`
- **Async task execution** with Claude Code CLI
- **OAuth 2.1 + PKCE** authentication
- **Git branch isolation** per task
- **Session resumption** (multi-turn Claude Code conversations)
- **Multi-project support** with per-project allowed tools
- **SQLite persistence** (pure Go, zero CGO)
- **Push notifications** via ntfy
- **Structured logging** with `log/slog`

### Security Hardening

After the initial build, a security audit identified and fixed 6 issues:

| Fix | Description |
|---|---|
| **C1** | Validate `redirect_uri` against configured allowlist — prevents open redirect attacks |
| **C2** | Enforce mandatory PKCE S256 on all OAuth flows — `code_challenge` and `code_verifier` now required |
| **C3** | Implement per-token and per-IP rate limiting — token bucket algorithm, pure Go |
| **C4** | Enforce `max_concurrent` task limit — checks running count before spawning goroutines |
| **C5** | Validate and clamp `timeout_minutes` against `max_timeout` — defense-in-depth at handler and task manager layers |
| **C6** | Use `crypto/subtle.ConstantTimeCompare` for all secret comparisons — client secret hashed at rest with SHA-256 |

Each fix was a task executed through the Herald workflow: start task, check progress, get result, review diff, commit.

## The Stack

The entire project was built with these constraints:

- **Zero CGO** — Every dependency is pure Go
- **6 dependencies** — chi, mcp-go, modernc/sqlite, uuid, yaml, testify
- **Single binary** — ~15MB, statically linked
- **Go 1.26** — Using the latest Go idioms (`errors.AsType`, `new(expr)`, `slog`)

## What This Proves

Herald isn't a theoretical tool. It's a working system that was used to build itself. The workflow — typing a prompt on your phone and having code appear on your workstation — is real, tested, and production-ready.
