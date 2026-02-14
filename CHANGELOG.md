# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Roadmap

- Model templates per task type (e.g. review→opus, test→sonnet, fix→sonnet) with config-driven defaults

## [0.1.1] — 2026-02-14

### Added

- Optional ngrok tunnel integration (`tunnel.enabled` in config) for instant HTTPS exposure without reverse proxy setup
- Dual-serve architecture: Herald serves both local (`127.0.0.1`) and tunnel (ngrok) listeners with the same router
- Environment variable `HERALD_NGROK_AUTHTOKEN` for secure token management
- Interactive setup wizard in install script (port, exposure method, project configuration)
- `--local` flag for install script to install from local `bin/` directory
- Config backup with timestamp when setup wizard overwrites an existing config

### Fixed

- Graceful shutdown: SSE connections no longer block server shutdown for 10+ seconds
- OAuth redirect URI: added `https://claude.ai/api/mcp/auth_callback` to default allowed redirect URIs
- Startup banner now displays the full MCP URL (e.g. `https://xxx.ngrok-free.app/mcp`)
- Tunnel error messages: common ngrok errors now show actionable remediation steps
- Executor pipe draining: fixed race condition where stdout/stderr pipes were not fully drained before `Wait()`

### Removed

- Dashboard references (moved to long-term roadmap)

## [0.1.0] — 2026-02-12

### Added

- MCP server with Streamable HTTP transport
- 10 MCP tools: `start_task`, `check_task`, `get_result`, `list_tasks`, `cancel_task`, `get_diff`, `list_projects`, `read_file`, `herald_push`, `get_logs`
- Async task execution with Claude Code CLI
- OAuth 2.1 + PKCE authentication with auto-generated secrets
- `herald rotate-secret` and `herald health` subcommands
- Git branch isolation per task
- Session resumption (multi-turn Claude Code conversations)
- `herald_push` — bidirectional bridge allowing Claude Code to push session context for remote continuation
- Linked task type for sessions pushed via `herald_push`
- Configurable model per task (`model` parameter on `start_task`)
- Long-polling on `check_task` (returns on status change)
- Multi-project support with per-project allowed tools
- SQLite persistence (pure Go, zero CGO)
- MCP push notifications (server-initiated, via SSE)
- Per-token and per-IP rate limiting (token bucket algorithm)
- `docker-compose.yml` for minimal deployment
- Structured logging with `log/slog`
- README in English and French

### Security

- Validate `redirect_uri` against configured allowlist (prevents open redirect)
- Enforce mandatory PKCE S256 on all OAuth flows
- Enforce `max_concurrent` task limit
- Validate and clamp `timeout_minutes` against `max_timeout`
- Constant-time secret comparison with SHA-256 hashing at rest
