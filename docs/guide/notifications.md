# Notifications

Herald can notify you when tasks complete or fail. Useful when you start a task from your phone and want to know when it's done.

## ntfy

[ntfy](https://ntfy.sh) is a simple push notification service. Herald can send notifications to any ntfy topic — either the public ntfy.sh service or your own self-hosted instance.

### Setup with ntfy.sh (Public)

```yaml
notifications:
  ntfy:
    enabled: true
    server: "https://ntfy.sh"
    topic: "herald-yourname"
    events:
      - "task.completed"
      - "task.failed"
```

Then subscribe to the topic on your phone:

1. Install the [ntfy app](https://ntfy.sh) (Android/iOS)
2. Subscribe to `herald-yourname`
3. You'll get push notifications when tasks complete or fail

!!! warning "Public topics are public"
    Anyone who knows your topic name can subscribe. For private notifications, either use a unique/random topic name or self-host ntfy.

### Setup with Self-Hosted ntfy

```yaml
notifications:
  ntfy:
    enabled: true
    server: "https://ntfy.yourdomain.com"
    topic: "herald"
    token: "${HERALD_NTFY_TOKEN}"
    events:
      - "task.completed"
      - "task.failed"
```

!!! tip
    Use the `token` field for authentication with self-hosted ntfy instances that require it. Always set the token via environment variable.

## Webhooks

Herald can send HTTP webhooks with HMAC-signed payloads to any URL.

### Configuration

```yaml
notifications:
  webhooks:
    - name: "n8n"
      url: "https://n8n.example.com/webhook/herald"
      secret: "${HERALD_WEBHOOK_SECRET}"
      events:
        - "task.completed"
        - "task.failed"
```

### Payload Format

```json
{
  "event": "task.completed",
  "task_id": "herald-a1b2c3d4",
  "project": "my-api",
  "status": "completed",
  "duration_seconds": 252,
  "cost_usd": 0.34,
  "timestamp": "2026-02-12T14:34:12Z"
}
```

### Signature Verification

Webhooks are signed with HMAC-SHA256. The signature is in the `X-Herald-Signature` header:

```
X-Herald-Signature: sha256=<hex-encoded HMAC>
```

Verify the signature on your server by computing `HMAC-SHA256(secret, body)` and comparing.

## Event Types

| Event | Trigger |
|---|---|
| `task.completed` | Task finished successfully |
| `task.failed` | Task failed with an error |
| `task.cancelled` | Task was cancelled by user |
| `task.started` | Task began execution |
| `task.queued` | Task entered the queue |

## Multiple Notification Channels

You can enable ntfy and webhooks simultaneously, and configure multiple webhook endpoints:

```yaml
notifications:
  ntfy:
    enabled: true
    server: "https://ntfy.sh"
    topic: "herald-personal"
    events: ["task.completed", "task.failed"]

  webhooks:
    - name: "n8n-automation"
      url: "https://n8n.example.com/webhook/herald"
      secret: "${HERALD_N8N_SECRET}"
      events: ["task.completed", "task.failed"]

    - name: "monitoring"
      url: "https://monitoring.example.com/api/events"
      secret: "${HERALD_MONITOR_SECRET}"
      events: ["task.failed"]
```
