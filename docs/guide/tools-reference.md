# Tools Reference

Herald exposes 10 MCP tools that Claude Chat discovers automatically. This page documents every parameter and response format.

## start_task

Launch a Claude Code task on a project. Returns immediately with a task ID. The task runs asynchronously.

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `prompt` | string | **Yes** | — | Task instructions for Claude Code |
| `project` | string | No | default project | Project name from configuration |
| `priority` | string | No | `"normal"` | `"low"`, `"normal"`, `"high"`, or `"urgent"` |
| `timeout_minutes` | number | No | `30` | Max execution time (clamped to `max_timeout`) |
| `template` | string | No | — | Template name (e.g., `review`, `test`, `fix`) |
| `session_id` | string | No | — | Session ID to resume (multi-turn conversations) |
| `git_branch` | string | No | auto-generated | Branch to create/use |
| `dry_run` | boolean | No | `false` | If true, plan without making changes |
| `model` | string | No | config default | Claude model to use (e.g., `claude-sonnet-4-5-20250929`, `claude-opus-4-6`) |

### Example Response

```
🚀 Task started

• ID: herald-a1b2c3d4
• Project: my-api
• Priority: normal

💡 Use check_task with task_id 'herald-a1b2c3d4' to monitor progress.
```

---

## check_task

Check the current status and progress of a task.

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `task_id` | string | **Yes** | — | Task ID from `start_task` |
| `wait_seconds` | number | No | `0` | Wait up to N seconds for status changes before responding (long-poll). Only returns early on status changes, not progress updates. |
| `include_output` | boolean | No | `false` | Include recent Claude Code output |
| `output_lines` | number | No | `20` | Number of output lines to include |

### Example Response (Running)

```
🔄 Running (2m 34s)

• Progress: Refactoring auth/middleware.go...
• Cost: $0.18 so far
```

### Example Response (Completed)

```
✅ Completed (4m 12s)

• Cost: $0.34
• Turns: 8
• Session: ses_abc123 (use to continue this conversation)

💡 Use get_result for the full output, or get_diff to see changes.
```

---

## get_result

Get the complete result of a finished task.

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `task_id` | string | **Yes** | — | Task ID from `start_task` |
| `format` | string | No | `"summary"` | `"summary"`, `"full"`, or `"json"` |

!!! info "Format options"
    - **summary** — Task metadata + truncated output (first 1000 chars)
    - **full** — Task metadata + complete untruncated output
    - **json** — Raw JSON serialization of the task

!!! note
    Only works for completed, failed, or cancelled tasks. Returns an error if the task is still running.

---

## list_tasks

List tasks with optional filters.

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `status` | string | No | `"all"` | `"all"`, `"pending"`, `"running"`, `"completed"`, `"failed"`, `"cancelled"`, `"linked"` |
| `project` | string | No | — | Filter by project name |
| `limit` | number | No | `20` | Maximum tasks to return |
| `since` | string | No | — | ISO 8601 datetime — only tasks after this time |

### Example Response

```
📋 Tasks (3 found)

✅ herald-a1b2c3d4 — completed
   Project: my-api | Priority: normal
   Duration: 4m 12s | Cost: $0.34

🔄 herald-e5f6a7b8 — running
   Project: my-api | Priority: high
   Running for 1m 45s...

❌ herald-c9d0e1f2 — failed
   Project: frontend | Priority: normal
   Duration: 0m 23s | Error: test suite failed
```

---

## cancel_task

Cancel a running or pending task.

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `task_id` | string | **Yes** | — | Task ID to cancel |
| `revert` | boolean | No | `false` | If true, revert Git changes made by this task |

### Example Response

```
🚫 Task herald-a1b2c3d4 cancelled.
```

---

## get_diff

Show the Git diff of changes made by a task or uncommitted changes in a project.

### Parameters

| Parameter | Type | Required | Description |
|---|---|---|---|
| `task_id` | string | One required | Diff the task's branch against the current branch |
| `project` | string | One required | Diff uncommitted changes against HEAD |

!!! note
    Provide either `task_id` or `project`, not both.

### Example Response

````
📝 Diff for task herald-a1b2c3d4 (branch: herald/a1b2c3d4-refactor-auth)

```diff
--- a/auth/middleware.go
+++ b/auth/middleware.go
@@ -12,8 +12,12 @@
-func CheckSession(r *http.Request) (*User, error) {
+func ValidateJWT(r *http.Request) (*User, error) {
+    token := r.Header.Get("Authorization")
```
````

---

## list_projects

List all configured projects with their Git status.

### Parameters

None.

### Example Response

```
📂 Projects (2 configured)

📦 my-api (default)
   Main backend API
   Path: /home/user/projects/my-api (main, clean)
   Concurrency: 1
   Tools: Read, Write, Edit, Bash(git *), Bash(go *), Bash(make *)

📦 frontend
   React frontend
   Path: /home/user/projects/frontend (develop, dirty)
   Concurrency: 2
   Tools: Read, Write, Edit, Bash(npm *)
```

---

## read_file

Read a file from a configured project.

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `path` | string | **Yes** | — | Relative path within the project |
| `project` | string | No | default project | Project name |
| `line_start` | number | No | — | Start reading from this line number (registered but not implemented in current version) |
| `line_end` | number | No | — | Stop reading at this line number (registered but not implemented in current version) |

!!! warning "Security"
    All paths are validated against the project root. Path traversal attempts (e.g., `../../etc/passwd`) are blocked. Files larger than 1MB are rejected.

### Example Response

````
📄 my-api/cmd/main.go (1.2 KB)

```go
package main

import (
    "log/slog"
    "os"
)

func main() {
    // ...
}
```
````

---

## herald_push

Push the current Claude Code session context to Herald for remote monitoring and continuation from another device. This is the **reverse flow** — instead of Claude Chat dispatching tasks, Claude Code pushes its session to Herald.

If a linked task with the same `session_id` already exists, it is updated instead of creating a duplicate.

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `session_id` | string | **Yes** | — | Current Claude Code session ID |
| `summary` | string | **Yes** | — | Summary of what has been done in this session so far |
| `project` | string | No | — | Project name or working directory path |
| `files_modified` | array | No | — | List of files created or modified during the session |
| `current_task` | string | No | — | What was being worked on (in progress or next step) |
| `git_branch` | string | No | — | Current git branch |
| `turns` | number | No | — | Number of conversation turns so far |

### Example Response (New)

```
Session pushed to Herald

- Task ID: herald-a1b2c3d4
- Session: ses_abc123
- Project: my-api
- Status: linked

You can now continue this session from Claude Chat:
  list_tasks to find it
  check_task for the full summary
  start_task with session_id "ses_abc123" to resume
```

### Example Response (Updated)

```
Session updated in Herald

- Task ID: herald-a1b2c3d4
- Session: ses_abc123
- Project: my-api
- Status: linked

You can now continue this session from Claude Chat:
  list_tasks to find it
  check_task for the full summary
  start_task with session_id "ses_abc123" to resume
```

---

## get_logs

View logs and activity history.

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `task_id` | string | No | — | Show detailed logs for a specific task |
| `level` | string | No | `"info"` | Minimum log level: `"debug"`, `"info"`, `"warn"`, `"error"` |
| `limit` | number | No | `20` | Maximum log entries to return |

### Example Response (Specific Task)

```
📋 Logs for herald-a1b2c3d4

• Status: ✅ completed
• Project: my-api
• Created: 2026-02-12 14:30:00
• Started: 2026-02-12 14:30:01
• Completed: 2026-02-12 14:34:12
• Duration: 4m 12s
• Session: ses_abc123
• Cost: $0.34
• Turns: 8
```

### Example Response (Recent Activity)

```
📋 Recent activity (5 tasks)

✅ herald-a1b2c3d4 — completed (my-api) — 2026-02-12 14:34
🔄 herald-e5f6a7b8 — running (my-api) — 2026-02-12 14:40
❌ herald-c9d0e1f2 — failed (frontend) — 2026-02-12 13:15
✅ herald-11223344 — completed (my-api) — 2026-02-12 12:00
🚫 herald-55667788 — cancelled (frontend) — 2026-02-12 11:30
```
