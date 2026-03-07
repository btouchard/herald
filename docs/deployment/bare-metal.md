# Bare Metal Deployment

Running Herald as a native binary is the recommended deployment method. It provides direct access to Claude Code and your filesystem with zero containerization overhead.

## Installation

```bash
# Build from source
git clone https://github.com/btouchard/herald.git
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

## Running Herald

### Standalone (Recommended)

The simplest and most reliable way to run Herald is in a terminal multiplexer:

```bash
# Using tmux
tmux new-session -d -s herald 'herald serve'

# Using screen
screen -dmS herald herald serve

# Or simply in the foreground
herald serve
```

### systemd User Service (Recommended)

Claude Code relies on user-level authentication tokens stored in your home directory. A **user-level systemd service** (`~/.config/systemd/user/`) inherits your environment, so Herald can access Claude Code credentials without any extra configuration.

Create `~/.config/systemd/user/herald.service`:

```ini
[Unit]
Description=Herald MCP Bridge
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/herald serve
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
```

Enable and start:

```bash
# Enable linger so the service runs even when you're not logged in
loginctl enable-linger $USER

# Reload, enable and start
systemctl --user daemon-reload
systemctl --user enable --now herald

# Check status
systemctl --user status herald

# View logs
journalctl --user -u herald -f
```

!!! important "Linger is required"
    Without `loginctl enable-linger`, user services stop when you log out. Enable it once and your Herald service will persist across sessions and reboots.

!!! tip "Secret management"
    The client secret is auto-generated and stored in `~/.config/herald/secret`. To rotate it, run `herald rotate-secret` and restart the service.

### systemd System Service (Alternative — requires API key)

A system-level service (`/etc/systemd/system/`) runs outside any user session. Claude Code's interactive OAuth tokens are **not available** in this context, so you must provide an `ANTHROPIC_API_KEY` instead.

!!! warning "API billing"
    Using an API key means Claude Code usage is billed through the Anthropic API, not through your Pro/Max subscription.

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
EnvironmentFile=/etc/herald/env

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/home/youruser/.config/herald
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Create `/etc/herald/env` with restricted permissions:

```bash
sudo mkdir -p /etc/herald
echo 'ANTHROPIC_API_KEY=sk-ant-...' | sudo tee /etc/herald/env
sudo chmod 600 /etc/herald/env
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now herald

# Check status
sudo systemctl status herald

# View logs
journalctl -u herald -f
```

## Log Rotation

Herald logs to stdout by default. With systemd, logs go to the journal automatically.

For the **user service**, view logs with:

```bash
journalctl --user -u herald -f
```

For the **system service**, view logs with:

```bash
journalctl -u herald -f
```

## Verify

```bash
# Health check
curl http://127.0.0.1:8420/health

# User service
systemctl --user status herald
journalctl --user -u herald -f --no-pager

# System service
sudo systemctl status herald
journalctl -u herald -f --no-pager
```

## HTTPS Exposure

Herald needs HTTPS to work with Claude Chat Custom Connectors. You have two options:

### Option A: ngrok tunnel (simplest)

Enable the built-in ngrok tunnel — no DNS, no certificates, no reverse proxy:

```yaml
tunnel:
  enabled: true
  provider: "ngrok"
  authtoken: "your-token"  # or set HERALD_NGROK_AUTHTOKEN env var
```

The tunnel URL appears in the startup banner. See [Configuration](../getting-started/configuration.md#tunnel) for details.

### Option B: Reverse proxy

Use a reverse proxy for TLS termination:

- [Traefik](traefik.md) — Docker-based or standalone
- **Caddy** — `caddy reverse-proxy --from herald.yourdomain.com --to 127.0.0.1:8420`

## Updating

```bash
cd herald
git pull
make build
sudo cp bin/herald /usr/local/bin/herald

# User service
systemctl --user restart herald

# Or system service
sudo systemctl restart herald
```
