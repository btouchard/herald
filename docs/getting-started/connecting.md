# Connecting from Claude Chat

Once Herald is running and accessible via HTTPS (either through ngrok or a reverse proxy), connect it to Claude Chat using Custom Connectors.

## Step-by-Step

### 1. Open Custom Connectors

In Claude Chat (web or mobile):

**Settings** → **Custom Connectors** → **Add Connector**

### 2. Configure the Connector

| Field | Value |
|---|---|
| **Server URL** | Your Herald MCP URL (see below) |
| **Name** | Herald (or whatever you prefer) |

The Server URL depends on your setup:

- **ngrok tunnel**: Copy the URL from the startup banner (e.g. `https://abc123.ngrok-free.app/mcp`)
- **Custom domain**: `https://herald.yourdomain.com/mcp`

### 3. Authenticate

Claude Chat will initiate the OAuth 2.1 flow:

1. You'll be redirected to Herald's authorization page
2. Herald auto-approves the connection (no login page in current version)
3. Claude Chat receives an access token via PKCE

### 4. Verify the Connection

Once connected, Claude Chat automatically discovers Herald's 10 tools. Test it:

> *"Use list_projects to show my configured projects."*

You should see your projects listed with their Git status.

## What Claude Chat Can Do Now

With Herald connected, Claude Chat gains these capabilities:

| Tool | What You Can Say |
|---|---|
| `start_task` | *"Refactor the auth middleware in my-api"* |
| `check_task` | *"How's that task going?"* |
| `get_result` | *"Show me the full result"* |
| `list_tasks` | *"What tasks ran today?"* |
| `cancel_task` | *"Cancel that task"* |
| `get_diff` | *"Show me the diff"* |
| `list_projects` | *"What projects do I have?"* |
| `read_file` | *"Show me the main.go file in my-api"* |
| `herald_push` | *(Called by Claude Code)* — *"Push this session to Herald"* |
| `get_logs` | *"Show me the logs for that task"* |

!!! tip "Natural language works"
    You don't need to name tools explicitly. Just describe what you want and Claude Chat will pick the right tool. *"Add pagination to the user list endpoint and run the tests"* triggers `start_task` automatically.

## Troubleshooting

### "Connection refused"

- Verify Herald is running: `curl http://127.0.0.1:8420/health`
- Check your reverse proxy is forwarding to port 8420
- Ensure TLS is properly configured on your reverse proxy

### "Authorization failed"

- Verify `client_id` and `client_secret` match between Herald config and the connector
- Check that `redirect_uris` includes all required URIs — especially `https://claude.ai/api/mcp/auth_callback` (this is the URI Claude.ai actually sends during Custom Connector authentication)
- If you upgraded from v0.1.0, add the missing redirect URI to your config:
  ```yaml
  auth:
    redirect_uris:
      - "https://claude.ai/oauth/callback"
      - "https://claude.ai/api/oauth/callback"
      - "https://claude.ai/api/mcp/auth_callback"  # required since v0.1.1
  ```
- Look at Herald's logs for auth errors: `./bin/herald serve` with `log_level: debug`

### "No tools discovered"

- Confirm the connector URL ends with `/mcp`
- Check Herald logs for incoming MCP requests
- Try disconnecting and reconnecting the connector

## What's Next

- [Workflow](../guide/workflow.md) — Learn the start → check → result loop
- [Tools Reference](../guide/tools-reference.md) — All 10 tools in detail
