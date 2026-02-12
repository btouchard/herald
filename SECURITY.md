# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | Yes       |

## Reporting a Vulnerability

**Please do NOT open a public issue for security vulnerabilities.**

Instead, report vulnerabilities privately:

1. **Email**: Send details to **benjamin@kolapsis.com**
2. **GitHub**: Use [GitHub Security Advisories](https://github.com/kolapsis/herald/security/advisories/new) (preferred)

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Acknowledgment**: within 48 hours
- **Initial assessment**: within 5 business days
- **Fix timeline**: depends on severity, typically within 30 days

## Security Model

Herald exposes Claude Code to the network. The security model is designed around defense in depth:

### Network

- Herald binds to `127.0.0.1` only — never `0.0.0.0`
- TLS termination is handled by a reverse proxy (Traefik, Caddy, nginx)
- Configuration validation rejects attempts to bind to all interfaces

### Authentication

- OAuth 2.1 with PKCE is required for all MCP requests
- Access tokens expire after 1 hour (configurable)
- Refresh tokens expire after 30 days, rotated on each use
- API tokens use SHA-256 hashed storage

### Filesystem

- Path traversal protection on all file operations (`SafePath`)
- Symlink resolution prevents escape from project directories
- Absolute paths in requests are rejected
- SQLite database created with `0600` permissions, directory with `0700`

### Execution

- Per-project tool restrictions via `allowed_tools`
- `--dangerously-skip-permissions` is never used
- Every task has a mandatory timeout (default: 30 minutes)
- Prompt size is bounded to prevent resource exhaustion
- Output buffer is bounded to prevent memory exhaustion
- User prompts are passed to Claude Code unmodified — no injection or enrichment

### Rate Limiting

- 60 requests per minute per token (configurable)
- Burst allowance for legitimate usage patterns

### Audit

- Every action is logged with timestamp and identity
- Task creation, execution, cancellation, and file access are all recorded

## Security Headers

All HTTP responses include:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Cache-Control: no-store`
