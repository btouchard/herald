# Configuration

Herald is configured via a single YAML file at `~/.config/herald/herald.yaml`.

## Quick Setup

```bash
mkdir -p ~/.config/herald
cp configs/herald.example.yaml ~/.config/herald/herald.yaml
```

Generate a client secret:

```bash
export HERALD_CLIENT_SECRET="$(openssl rand -hex 32)"
```

## Full Reference

### Server

```yaml
server:
  host: "127.0.0.1"          # Always localhost — reverse proxy handles external
  port: 8420
  public_url: "https://herald.yourdomain.com"
  log_level: "info"           # debug, info, warn, error
```

!!! warning "Always bind to localhost"
    Herald **must** bind to `127.0.0.1`. Never use `0.0.0.0`. Use a reverse proxy like [Traefik](../deployment/traefik.md) for external HTTPS access.

### Auth

```yaml
auth:
  client_id: "herald-claude-chat"
  client_secret: "${HERALD_CLIENT_SECRET}"
  admin_password_hash: "${HERALD_ADMIN_PASSWORD_HASH}"
  access_token_ttl: 1h
  refresh_token_ttl: 720h     # 30 days
  redirect_uris:
    - "https://claude.ai/oauth/callback"
    - "https://claude.ai/api/oauth/callback"
```

| Field | Default | Description |
|---|---|---|
| `client_id` | — | OAuth client ID (shared with Claude Chat connector setup) |
| `client_secret` | — | OAuth client secret. **Must be set via env var.** |
| `admin_password_hash` | — | SHA-256 hash for dashboard admin access |
| `access_token_ttl` | `1h` | Access token lifetime |
| `refresh_token_ttl` | `720h` | Refresh token lifetime (30 days) |
| `redirect_uris` | — | Allowed OAuth redirect URIs (exact match) |

!!! tip "Never hardcode secrets"
    Use environment variable substitution (`${VAR}`) for all secrets. Herald expands `${VAR}` references in YAML values at load time.

### Database

```yaml
database:
  path: "~/.config/herald/herald.db"
  retention_days: 90
```

Herald uses SQLite (pure Go, zero CGO) for persistence. Tasks, tokens, and audit logs are stored here.

### Execution

```yaml
execution:
  claude_path: "claude"
  default_timeout: 30m
  max_timeout: 2h
  work_dir: "~/.config/herald/work"
  max_concurrent: 3
  env:
    CLAUDE_CODE_ENTRYPOINT: "herald"
    CLAUDE_CODE_DISABLE_AUTO_UPDATE: "1"
```

| Field | Default | Description |
|---|---|---|
| `claude_path` | `"claude"` | Path to Claude Code binary |
| `default_timeout` | `30m` | Default task timeout |
| `max_timeout` | `2h` | Maximum allowed timeout (clamps user requests) |
| `work_dir` | `~/.config/herald/work` | Temp directory for prompts and outputs |
| `max_concurrent` | `3` | Global concurrent task limit |
| `env` | — | Environment variables passed to Claude Code |

### Notifications

```yaml
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
```

See [Notifications](../guide/notifications.md) for detailed setup.

### Projects

```yaml
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
```

| Field | Required | Description |
|---|---|---|
| `path` | Yes | Absolute path to the project directory |
| `description` | No | Human-readable description (shown in `list_projects`) |
| `default` | No | If `true`, this project is used when no project is specified |
| `allowed_tools` | Yes | Claude Code tools this project can use |
| `max_concurrent_tasks` | No | Per-project concurrency limit |
| `git.auto_branch` | No | Create a new branch for each task |
| `git.auto_stash` | No | Stash uncommitted changes before switching branches |
| `git.auto_commit` | No | Auto-commit changes when task completes |
| `git.branch_prefix` | No | Prefix for auto-created branches (e.g., `herald/`) |

See [Multi-Project](../guide/multi-project.md) for advanced setups.

### Rate Limiting

```yaml
rate_limit:
  requests_per_minute: 60
  burst: 10
```

Per-token rate limiting using the token bucket algorithm. Applied to all MCP and API endpoints.

### Dashboard

```yaml
dashboard:
  enabled: true
```

Enables the embedded web dashboard at `/dashboard`. Uses SSE for real-time task updates.

## Environment Variable Substitution

Any value in `herald.yaml` can reference an environment variable:

```yaml
auth:
  client_secret: "${HERALD_CLIENT_SECRET}"
```

Herald expands `${VAR}` at load time. If the variable is not set, the value remains as the literal string `${VAR}`.

## What's Next

- [Connect from Claude Chat](connecting.md) — Add the Custom Connector
- [Workflow](../guide/workflow.md) — Start using Herald
