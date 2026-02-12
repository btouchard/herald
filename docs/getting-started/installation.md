# Installation

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| **Go** | 1.26+ | [Download](https://go.dev/dl/) |
| **Claude Code CLI** | Latest | [Install docs](https://docs.anthropic.com/en/docs/claude-code) |
| **Anthropic account** | — | With Claude Code access |
| **HTTPS domain** | — | Required for Claude Chat Custom Connectors |
| **Reverse proxy** | — | Traefik or Caddy recommended for TLS termination |

!!! tip "No Docker required"
    Herald is a single Go binary. Docker is available as an [option](../deployment/docker.md), but running the binary directly gives you native access to Claude Code and your filesystem.

## Build from Source

```bash
git clone https://github.com/kolapsis/herald.git
cd herald
make build
```

The binary is at `./bin/herald`. It's statically linked (~15MB) and has zero runtime dependencies.

!!! note "Zero CGO"
    Herald compiles with `CGO_ENABLED=0`. No C toolchain needed. Cross-compiles to any Go-supported platform.

### Cross-compilation

```bash
# Linux AMD64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/herald ./cmd/herald

# Linux ARM64 (Raspberry Pi, etc.)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/herald ./cmd/herald

# macOS ARM64 (Apple Silicon)
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/herald ./cmd/herald

# Windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/herald.exe ./cmd/herald
```

## Verify Installation

```bash
# Check the binary runs
./bin/herald --help

# Verify Claude Code is available
claude --version
```

## What's Next

1. [Configure Herald](configuration.md) — Set up `herald.yaml` with your domain and projects
2. [Connect from Claude Chat](connecting.md) — Add the Custom Connector
3. Start coding from your phone
