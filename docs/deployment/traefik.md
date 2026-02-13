# Traefik Deployment

Traefik is the recommended reverse proxy for Herald. It handles TLS termination with automatic Let's Encrypt certificates and forwards traffic to Herald on localhost.

## Architecture

```
Internet ──► Traefik (:443, TLS) ──► Herald (127.0.0.1:8420)
```

Herald binds to localhost only. Traefik handles all external traffic.

## Docker Compose

```yaml
services:
  traefik:
    image: traefik:v3
    command:
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--entrypoints.web.http.redirections.entryPoint.to=websecure"
      - "--certificatesresolvers.le.acme.email=you@example.com"
      - "--certificatesresolvers.le.acme.storage=/letsencrypt/acme.json"
      - "--certificatesresolvers.le.acme.httpchallenge.entrypoint=web"
      - "--providers.docker=true"
      - "--providers.docker.exposedByDefault=false"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "./letsencrypt:/letsencrypt"
    restart: unless-stopped

  herald:
    build: .
    network_mode: host
    volumes:
      - "~/.config/herald:/root/.config/herald"
      - "~/projects:/root/projects:ro"
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.herald.rule=Host(`herald.yourdomain.com`)"
      - "traefik.http.routers.herald.entrypoints=websecure"
      - "traefik.http.routers.herald.tls.certresolver=le"
      - "traefik.http.services.herald.loadbalancer.server.port=8420"
    restart: unless-stopped
```

!!! note "`network_mode: host`"
    Herald needs `network_mode: host` in Docker to access Claude Code and the local filesystem. This means Traefik and Herald share the host network.

## Traefik Static Configuration (File-Based)

If you're not using Docker labels, use Traefik's file configuration:

### traefik.yml

```yaml
entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
  websecure:
    address: ":443"

certificatesResolvers:
  le:
    acme:
      email: you@example.com
      storage: /etc/traefik/acme.json
      httpChallenge:
        entryPoint: web

providers:
  file:
    filename: /etc/traefik/dynamic.yml
```

### dynamic.yml

```yaml
http:
  routers:
    herald:
      rule: "Host(`herald.yourdomain.com`)"
      entryPoints:
        - websecure
      tls:
        certResolver: le
      service: herald

  services:
    herald:
      loadBalancer:
        servers:
          - url: "http://127.0.0.1:8420"
```

## DNS Setup

Point your domain to your server:

```
herald.yourdomain.com → A record → your.server.ip
```

Traefik will automatically obtain a Let's Encrypt certificate on first request.

## Verify

```bash
# Check TLS
curl -v https://herald.yourdomain.com/health

# Check MCP endpoint
curl -v https://herald.yourdomain.com/mcp
```

The MCP endpoint should return a 401 (unauthorized) since you haven't authenticated. That's correct — it means Herald is running and auth is enforced.
