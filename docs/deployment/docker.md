# Docker Deployment

!!! tip "Binary deployment is preferred"
    Herald is designed to run as a native binary for direct access to Claude Code and your filesystem. Docker is available as an option, not a recommendation. See [Bare Metal](bare-metal.md) for the recommended approach.

## Dockerfile

```dockerfile
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o herald ./cmd/herald

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/herald /herald

EXPOSE 8420

VOLUME ["/data", "/config"]

HEALTHCHECK --interval=30s --timeout=3s CMD ["/herald", "health"]

ENTRYPOINT ["/herald"]
CMD ["serve", "--config", "/config/herald.yaml"]
```

Key properties:

- **Multi-stage build** — Build in `golang:1.26-alpine`, run from `scratch`
- **Zero CGO** — Static binary, no C dependencies
- **Non-root** — Runs as user 65534 (nobody)
- **~15MB image** — Just the binary and CA certs

## Docker Compose (Herald Only)

```yaml
services:
  herald:
    build: .
    ports:
      - "8420:8420"
    volumes:
      - herald-data:/data
      - ./configs/herald.example.yaml:/config/herald.yaml:ro
    environment:
      - HERALD_CLIENT_SECRET=${HERALD_CLIENT_SECRET:-}
    restart: unless-stopped

volumes:
  herald-data:
```

## Important: Host Networking

Herald needs `network_mode: host` because it must:

1. **Access Claude Code** — The `claude` binary must be available in the container's PATH
2. **Access your filesystem** — Project directories must be mounted
3. **Bind to localhost** — Herald listens on `127.0.0.1:8420`

!!! warning "Claude Code in Docker"
    Running Herald in Docker means Claude Code must also be accessible inside the container. You'll need to mount the `claude` binary and its config, or install it in the image. This is why native binary deployment is simpler.

## Volume Mounts

| Host Path | Container Path | Mode | Purpose |
|---|---|---|---|
| `~/.config/herald` | `/root/.config/herald` | rw | Config, database, work dir |
| `~/projects` | `/projects` | ro | Your project codebases |
| Claude binary | `/usr/local/bin/claude` | ro | Claude Code CLI |

## Environment Variables

```bash
# Optional — override auto-generated secret (for multi-instance deployments)
HERALD_CLIENT_SECRET=your-secret-here
```

The client secret is auto-generated on first run and persisted in `~/.config/herald/secret`. You only need the environment variable if you're running multiple Herald instances that must share the same secret.

## Building the Image

```bash
docker build -t herald .

# Or with docker compose
docker compose build
```

## With Traefik

See [Traefik Deployment](traefik.md) for the full Docker Compose setup with TLS and reverse proxy.
