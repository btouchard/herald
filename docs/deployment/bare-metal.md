# Bare Metal Deployment

Running Herald as a native binary is the recommended deployment method. It provides direct access to Claude Code and your filesystem with zero containerization overhead.

## Installation

```bash
# Build from source
git clone https://github.com/kolapsis/herald.git
cd herald
make build

# Copy binary to a standard location
sudo cp bin/herald /usr/local/bin/herald
```

## Configuration

```bash
# Create config directory
mkdir -p ~/.config/herald

# Copy and edit config
cp configs/herald.example.yaml ~/.config/herald/herald.yaml
```

Edit `~/.config/herald/herald.yaml` with your domain and projects. The client secret is auto-generated on first run. See [Configuration](../getting-started/configuration.md) for the full reference.

## systemd Service

Create `/etc/systemd/system/herald.service`:

```ini
[Unit]
Description=Herald MCP Server
After=network.target

[Service]
Type=simple
User=youruser
Group=youruser
ExecStart=/usr/local/bin/herald serve
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/home/youruser/.config/herald
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

!!! tip "Secret management"
    The client secret is auto-generated and stored in `~/.config/herald/secret`. To rotate it, run `herald rotate-secret` and restart the service.

### Enable and Start

```bash
sudo systemctl daemon-reload
sudo systemctl enable herald
sudo systemctl start herald

# Check status
sudo systemctl status herald

# View logs
journalctl -u herald -f
```

## Log Rotation

Herald logs to stdout by default. With systemd, logs go to the journal. For file-based logging, you can redirect:

```ini
StandardOutput=append:/var/log/herald/herald.log
StandardError=append:/var/log/herald/error.log
```

Then configure logrotate at `/etc/logrotate.d/herald`:

```
/var/log/herald/*.log {
    daily
    missingok
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 youruser youruser
    postrotate
        systemctl reload herald 2>/dev/null || true
    endscript
}
```

## Verify

```bash
# Health check
curl http://127.0.0.1:8420/health

# Check systemd status
systemctl status herald

# Watch logs
journalctl -u herald -f --no-pager
```

## Reverse Proxy

You still need a reverse proxy for TLS. See:

- [Traefik](traefik.md) — Docker-based or standalone
- **Caddy** — `caddy reverse-proxy --from herald.yourdomain.com --to 127.0.0.1:8420`

## Updating

```bash
cd herald
git pull
make build
sudo cp bin/herald /usr/local/bin/herald
sudo systemctl restart herald
```
