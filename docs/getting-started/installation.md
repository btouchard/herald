# Installation

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| **Claude Code CLI** | Latest | [Install docs](https://docs.anthropic.com/en/docs/claude-code) |
| **Anthropic account** | — | With Claude Code access |
| **HTTPS exposure** | — | Via ngrok (built-in) **or** a domain + reverse proxy (Traefik/Caddy) |

!!! tip "No Docker required"
    Herald is a single Go binary. Docker is available as an [option](../deployment/docker.md), but running the binary directly gives you native access to Claude Code and your filesystem.

## Install from Release

The quickest way to install Herald. Detects your OS and architecture, downloads the latest release, verifies the checksum, and installs to `/usr/local/bin`.

```bash
curl -fsSL https://raw.githubusercontent.com/btouchard/herald/main/install.sh | sh
```

After installation, the script launches an **interactive setup wizard** that walks you through port, exposure method (ngrok / custom domain / local-only), and project configuration. It generates a ready-to-use `herald.yaml`.

To install to a custom directory:

```bash
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/btouchard/herald/main/install.sh | sh
```

## Build from Source

Requires **Go 1.26+** ([download](https://go.dev/dl/)).

```bash
git clone https://github.com/btouchard/herald.git
cd herald
make build
```

The binary is at `./bin/herald`. It's statically linked (~15MB) and has zero runtime dependencies.

To install from your local build and run the setup wizard:

```bash
./install.sh --local
```

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
```

## Verify Installation

```bash
# Check the binary runs
./bin/herald version

# Verify Claude Code is available
claude --version
```

## What's Next

1. [Configure Herald](configuration.md) — Set up `herald.yaml` with your domain and projects
2. [Connect from Claude Chat](connecting.md) — Add the Custom Connector
3. Start coding from your phone
