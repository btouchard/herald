# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Removed

- Dashboard references (moved to long-term roadmap)

### Added

- Optional ngrok tunnel integration (`tunnel.enabled` in config) for instant HTTPS exposure without reverse proxy setup
- Dual-serve architecture: Herald serves both local (`127.0.0.1`) and tunnel (ngrok) listeners with the same router
- Environment variable `HERALD_NGROK_AUTHTOKEN` for secure token management
- Configurable model per task (`model` parameter on `start_task`, defaults to Sonnet for cost efficiency)
- Auto-generated secrets (zero-config auth setup)
- `herald rotate-secret` subcommand
- `herald health` subcommand for Docker HEALTHCHECK
- `docker-compose.yml` for minimal deployment
- Long-polling on `check_task` (returns only on status change, not progress updates)
- MCP push notifications for task lifecycle events
- `herald_push` MCP tool — bidirectional bridge allowing Claude Code to push session context to Herald for remote monitoring and continuation from another device
- New task type `linked` for sessions pushed from Claude Code via `herald_push`
- Deduplication: pushing the same `session_id` updates the existing linked task instead of creating a duplicate
- `list_tasks` shows linked sessions with `linked` status filter
- SQLite migration: `type` column on tasks table to distinguish regular tasks from linked sessions

### Roadmap

- Model templates per task type (e.g. review→opus, test→sonnet, fix→sonnet) with config-driven defaults

### Security

- **C1**: Validate `redirect_uri` against configured allowlist in OAuth authorization flow — prevents open redirect attacks
- **C2**: Enforce mandatory PKCE S256 on all OAuth flows — `code_challenge` and `code_verifier` are now required, not optional
- **C3**: Implement per-token and per-IP rate limiting middleware — token bucket algorithm, pure Go, no external deps
- **C4**: Enforce `max_concurrent` task limit — `Start()` now checks running count before spawning goroutines
- **C5**: Validate and clamp `timeout_minutes` against `max_timeout` — defense-in-depth at handler and task manager layers
- **C6**: Use `crypto/subtle.ConstantTimeCompare` for all secret comparisons — client secret hashed at rest with SHA-256, PKCE verification also constant-time

### Fixed

- `oauthError()` was passing `nil` as `*http.Request` to `http.Redirect` — would panic on OAuth error redirects (found during C2 fix)

## [0.1.0] — 2026-02-12

### Added

- MCP server with Streamable HTTP transport
- 9 MCP tools: `start_task`, `check_task`, `get_result`, `list_tasks`, `cancel_task`, `get_diff`, `list_projects`, `read_file`, `get_logs`
- Async task execution with Claude Code CLI
- OAuth 2.1 + PKCE authentication
- Git branch isolation per task
- Session resumption (multi-turn Claude Code conversations)
- Multi-project support with per-project allowed tools
- SQLite persistence (pure Go, zero CGO)
- MCP push notifications (server-initiated, via SSE)
- Structured logging with `log/slog`
- README in English and French
