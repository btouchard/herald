# Custom Executor Guide

Herald's executor architecture is pluggable. While the default executor is `claude-code` (Claude Code CLI), you can implement adapters for any CLI tool — Codex, Gemini CLI, Aider, or your own wrapper.

## Architecture

```
executor/
├── executor.go       # Interface + Capabilities + Registry
├── registry.go       # Register/Get/Available
├── kill.go           # GracefulKill (shared POSIX utility)
└── claude/
    ├── claude.go     # Claude Code implementation (init() auto-registers as "claude-code")
    └── stream.go     # stream-json output parsing
```

The registry pattern uses Go `init()` functions for zero-config registration. Each executor lives in its own sub-package under `internal/executor/`.

## Implementing an Executor

### 1. Create a sub-package

```
internal/executor/mybackend/
└── mybackend.go
```

### 2. Implement the interface

```go
package mybackend

import (
    "context"

    "github.com/btouchard/herald/internal/executor"
)

const name = "my-backend"

func init() {
    executor.Register(name, factory)
}

func factory(cfg map[string]any) (executor.Executor, error) {
    binaryPath, _ := cfg["binary_path"].(string)
    if binaryPath == "" {
        binaryPath = "my-cli" // default binary name
    }
    return &Executor{binaryPath: binaryPath}, nil
}

type Executor struct {
    binaryPath string
}

func (e *Executor) Capabilities() executor.Capabilities {
    return executor.Capabilities{
        Name:             name,
        Version:          "0.1.0",
        SupportsSession:  false, // set true if your CLI supports session resumption
        SupportsModel:    false, // set true if your CLI supports model selection
        SupportsToolList: false, // set true if your CLI supports tool restrictions
        SupportsDryRun:   false, // set true if your CLI supports dry-run/plan mode
        SupportsStreaming: true,  // set true if you stream progress events
    }
}

func (e *Executor) Execute(ctx context.Context, req executor.Request, onProgress executor.ProgressFunc) (*executor.Result, error) {
    // Build and run your CLI command here.
    // Use req.Prompt, req.ProjectPath, req.TimeoutMinutes, etc.
    // Call onProgress("progress", "message") to report status.
    // Return an executor.Result with output, cost, turns, etc.

    return &executor.Result{
        Output:   "task completed",
        ExitCode: 0,
    }, nil
}
```

### 3. Register via blank import

In `cmd/herald/main.go`, add a blank import so the `init()` function runs:

```go
import (
    _ "github.com/btouchard/herald/internal/executor/claude"
    _ "github.com/btouchard/herald/internal/executor/mybackend"
)
```

### 4. Select via config

In `herald.yaml`:

```yaml
execution:
  executor: "my-backend"
```

If omitted, defaults to `"claude-code"`.

## Capabilities

The `Capabilities` struct tells MCP handlers what your executor supports. When a user requests a feature that your executor doesn't support (e.g., session resumption), Herald shows a warning in the response — not an error.

| Field | Description |
|---|---|
| `SupportsSession` | Can resume previous conversations via `session_id` |
| `SupportsModel` | Can override the model per task |
| `SupportsToolList` | Can restrict which tools the CLI uses |
| `SupportsDryRun` | Can run in plan-only mode without making changes |
| `SupportsStreaming` | Emits progress events during execution |
| `Name` | Display name shown in MCP responses |
| `Version` | Executor version string |

## Request and Result

### Request fields

| Field | Description | Capability required |
|---|---|---|
| `TaskID` | Unique task identifier | Always available |
| `Prompt` | User prompt text | Always available |
| `ProjectPath` | Absolute path to project directory | Always available |
| `TimeoutMinutes` | Maximum execution time | Always available |
| `Env` | Environment variables to set | Always available |
| `SessionID` | Session to resume | `SupportsSession` |
| `Model` | Model override | `SupportsModel` |
| `AllowedTools` | Tool restrictions | `SupportsToolList` |
| `DryRun` | Plan-only mode | `SupportsDryRun` |

Fields that require capabilities your executor doesn't support are silently ignored. Herald warns the user in the MCP response.

### Result fields

| Field | Description |
|---|---|
| `SessionID` | Session ID for future resumption (if supported) |
| `Output` | Full text output from the CLI |
| `CostUSD` | Estimated cost in USD (0 if not available) |
| `Turns` | Number of conversation turns |
| `Duration` | Execution duration |
| `ExitCode` | Process exit code (0 = success) |

## Progress reporting

Call `onProgress` during execution to update task status in real-time:

```go
onProgress("progress", "Reading project files...")
onProgress("progress", "Generating code changes...")
onProgress("result", "Task completed successfully")
```

Progress messages appear in `check_task` responses and MCP push notifications.

## Factory configuration

The factory receives a `map[string]any` built from Herald's config. The Claude Code executor uses these keys:

- `claude_path` — path to the CLI binary
- `work_dir` — working directory for task files
- `env` — environment variables map

You can define your own keys. They are passed through from the execution config.

## Testing

Test your executor with mock commands. See `internal/executor/claude/claude_test.go` for patterns:

```go
func TestMyExecutor_Execute_Success(t *testing.T) {
    exec := &Executor{binaryPath: "/usr/bin/echo"}
    result, err := exec.Execute(context.Background(), executor.Request{
        TaskID:      "herald-test01",
        Prompt:      "hello",
        ProjectPath: t.TempDir(),
    }, func(eventType, message string) {})

    require.NoError(t, err)
    assert.Equal(t, 0, result.ExitCode)
}
```
