# Security Model

Herald exposes Claude Code to the network. This page describes the threat model and every security measure in place.

## Threat Model

### What Herald Exposes

- Remote execution of Claude Code on your workstation
- File reading within configured projects
- Git operations within configured projects

### Attack Surface

| Surface | Exposure | Mitigation |
|---|---|---|
| Network listener | HTTPS endpoint | Binds to `127.0.0.1` only; reverse proxy handles TLS |
| Authentication | OAuth 2.1 endpoint | PKCE mandatory, constant-time secret comparison |
| MCP tools | 10 tools callable remotely | OAuth required, per-token rate limiting |
| Filesystem | `read_file` tool | Path traversal protection, project root sandboxing |
| Execution | Claude Code spawning | Per-project tool restrictions, timeouts, concurrency limits |
| Prompts | User-provided instructions | Passed unmodified — no injection possible from Herald side |

## Security Layers

### 1. Network Isolation

Herald **always** binds to `127.0.0.1`. It never listens on `0.0.0.0`.

External access is handled by a reverse proxy (Traefik or Caddy) that:

- Terminates TLS with a valid certificate
- Forwards traffic to localhost:8420
- Can add additional IP restrictions, geofencing, etc.

### 2. OAuth 2.1 with Mandatory PKCE

Every MCP request requires a valid Bearer token obtained through the OAuth 2.1 flow.

```
Claude Chat                          Herald
───────────                          ──────
1. Generate code_verifier
   + code_challenge (S256)
2. GET /oauth/authorize ──────────►  Validate redirect_uri
                                     against allowlist
                         ◄──────────  Authorization page

3. User authorizes
4. Redirect with auth code ────────► Validate code_challenge
                                     (constant-time)
                         ◄──────────  Access token + refresh token

5. MCP requests with
   Bearer token ──────────────────►  Validate JWT signature
                                     Check expiry
                                     Rate limit check
```

**Key properties:**

- **PKCE S256 is mandatory** — `code_challenge` and `code_verifier` are required on every flow
- **Redirect URI allowlist** — Only pre-configured URIs are accepted (prevents open redirect)
- **Constant-time comparison** — Client secret and PKCE verifier use `crypto/subtle.ConstantTimeCompare`
- **Client secret hashed at rest** — Stored as SHA-256 hash, never in plaintext

### 3. Token Security

| Token | Lifetime | Rotation |
|---|---|---|
| Access token | 1 hour | Expires, must refresh |
| Refresh token | 30 days | Rotated on every use |
| Auth code | Single use | Consumed on exchange |

Refresh token rotation means each refresh token can only be used once. If a token is replayed, it's rejected.

### 4. Path Traversal Protection

All file operations validate that the resolved path stays within the project root:

```go
func SafePath(projectRoot, requestedPath string) (string, error) {
    // Reject absolute paths
    // Resolve .. and symlinks
    // Verify result is under projectRoot
}
```

**Protected against:**

- `../../etc/passwd` — relative traversal
- `/etc/passwd` — absolute paths
- Symlink escapes — resolved before validation
- URL-encoded sequences — handled by path resolution

The `read_file` tool enforces a 1MB file size limit.

### 5. Execution Sandboxing

**Per-project tool restrictions:**

```yaml
allowed_tools:
  - "Read"
  - "Write"
  - "Edit"
  - "Bash(git *)"      # Only git commands
  - "Bash(go *)"       # Only go commands
```

Claude Code runs with explicit `--allowedTools` flags. No `--dangerously-skip-permissions` anywhere.

**Timeouts:**

- Every task has a deadline (default: 30 minutes, max: configurable)
- `timeout_minutes` parameter is clamped to `max_timeout` — users cannot request unbounded execution
- Timeout enforced at both the handler and task manager layers

**Concurrency limits:**

- Global `max_concurrent` limits total running tasks
- Per-project `max_concurrent_tasks` limits per-project parallelism
- Enforced before spawning goroutines — exceeding tasks are queued

### 6. Rate Limiting

Per-token rate limiting using the token bucket algorithm:

```yaml
rate_limit:
  requests_per_minute: 200
  burst: 100
```

- Applied to all MCP and API endpoints
- Per-token tracking (not just per-IP)
- Pure Go implementation, no external dependencies

### 7. Prompt Integrity

Herald **never** modifies, enriches, or injects into user prompts. The prompt is passed to Claude Code exactly as received. This means:

- No system prompt injection from Herald
- No hidden instructions
- No prompt wrapping or templating (unless explicitly requested via `template` parameter)

### 8. Audit Trail

Every action is logged with structured data:

```
task_id, action, user_identity, timestamp, result
```

Logged actions include: task creation, task start, task completion/failure, file reads, cancellations.

### 9. Secret Management

- Client secret is auto-generated on first run and stored in `~/.config/herald/secret` (mode 0600)
- Can be overridden via `HERALD_CLIENT_SECRET` environment variable
- Rotate with `herald rotate-secret` (invalidates all sessions)
- Client secret is hashed (SHA-256) before use in memory
- OAuth tokens are JWTs signed with a key derived from the client secret

## Hardening Checklist

!!! warning "Before exposing Herald to the internet"

    - [ ] Herald binds to `127.0.0.1` (never `0.0.0.0`)
    - [ ] Reverse proxy configured with valid TLS certificate
    - [ ] `client_secret` set via environment variable
    - [ ] `redirect_uris` restricted to Claude's callback URLs
    - [ ] Per-project `allowed_tools` configured (no blanket Bash)
    - [ ] `max_timeout` set to a reasonable value
    - [ ] `max_concurrent` set to prevent resource exhaustion
    - [ ] Rate limiting enabled
    - [ ] Firewall rules restrict access to reverse proxy ports only
