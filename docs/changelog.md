# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- Push notifications via ntfy
- Structured logging with `log/slog`
- README in English and French
