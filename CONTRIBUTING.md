# Contributing to Herald

Thank you for considering contributing to Herald! This document explains how to get started.

## Development Setup

**Prerequisites**: Go 1.26+, [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) (optional, for integration testing).

```bash
git clone https://github.com/kolapsis/herald.git
cd herald
make build
make test
```

## Project Structure

```
cmd/herald/         Entry point and CLI wiring
internal/
  config/           Configuration loading (YAML + env vars)
  mcp/              MCP server, tool handlers, middleware
  executor/         Claude Code process execution
  task/             Task lifecycle, queue, goroutine pool
  store/            SQLite persistence
  auth/             OAuth 2.1 server (PKCE, JWT)
  project/          Project management
  git/              Git operations (branch, stash, diff)
```

Each `internal/` package is self-contained. Dependencies between packages are wired in `cmd/herald/main.go`.

## Making Changes

1. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feat/your-feature
   ```

2. **Write code and tests together**. Tests live alongside the code they test (`*_test.go` in the same package).

3. **Run the checks before committing**:
   ```bash
   make lint    # golangci-lint
   make test    # all tests + race detector
   ```

4. **Commit** using [Conventional Commits](https://www.conventionalcommits.org/):
   ```
   feat(executor): add timeout configuration per task
   fix(store): handle concurrent writes to task table
   refactor(mcp): simplify handler registration
   test(git): add branch conflict edge cases
   docs: update configuration reference
   chore: bump mcp-go to v0.28
   ```

5. **Open a Pull Request** against `main`.

## Code Conventions

### Go Idioms (Go 1.26)

- Use `errors.AsType[E]` instead of `errors.As` for type-safe error matching
- Use `log/slog` for all logging — no external logging libraries
- Always check errors — never discard with `_`
- Wrap errors with context: `fmt.Errorf("failed to ...: %w", err)`
- Use `context.Context` as the first parameter for cancellable operations

### Architecture Rules

- **Zero CGO** — everything must compile with `CGO_ENABLED=0`
- **No ORM** — use raw SQL with `modernc.org/sqlite`
- **Small interfaces** — 1 to 3 methods, defined where they are consumed
- **No utility packages** — no `utils/`, `helpers/`, `common/`

### Testing

- Test names follow `TestFunction_WhenCondition_ExpectedBehavior`
- Use `t.Parallel()` for independent tests
- Use table-driven tests for multiple cases
- Use `testify/assert` and `testify/require` for assertions
- Integration tests use build tag `//go:build integration`

### Security

Herald exposes Claude Code to the network. Every contribution must consider:

- **Path traversal** — use `SafePath()` for any filesystem access
- **Input validation** — validate all user-provided parameters
- **Timeouts** — every operation must have a deadline
- **No `--dangerously-skip-permissions`** — tool restrictions are enforced per project

## What We're Looking For

- Bug fixes with regression tests
- New notification backends (Slack, Discord, email)
- Performance improvements with benchmarks
- Documentation improvements
- Security hardening

## What to Avoid

- Adding dependencies without discussion — open an issue first
- Large refactors without prior agreement
- Changes that require CGO
- Breaking the MCP protocol contract

## Questions?

Open an [issue](https://github.com/kolapsis/herald/issues) or start a [discussion](https://github.com/kolapsis/herald/discussions).
