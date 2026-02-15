package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/btouchard/herald/internal/task"
)

const (
	longPollInterval = 500 * time.Millisecond
	longPollMaxWait  = 30
)

// CheckTask returns a handler that reports a task's current status.
// When wait_seconds > 0 and the task is still running, it long-polls
// until the status changes or the timeout expires.
func CheckTask(tm *task.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		taskID, _ := args["task_id"].(string)
		if taskID == "" {
			return mcp.NewToolResultError("task_id is required"), nil
		}

		t, err := tm.Get(taskID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Task not found: %s", err)), nil
		}

		waitSeconds := 0
		if w, ok := args["wait_seconds"].(float64); ok && w > 0 {
			waitSeconds = int(w)
			if waitSeconds > longPollMaxWait {
				waitSeconds = longPollMaxWait
			}
		}

		snap := t.Snapshot()

		// Long-poll: if task is still active and wait requested, poll for changes
		if waitSeconds > 0 && !isTerminalStatus(snap.Status) {
			snap = waitForChange(ctx, t, snap, time.Duration(waitSeconds)*time.Second)
		}

		includeOutput, _ := args["include_output"].(bool)
		outputLines := 20
		if n, ok := args["output_lines"].(float64); ok && n > 0 {
			outputLines = int(n)
		}

		text := formatCheckResponse(snap, includeOutput, outputLines)
		return mcp.NewToolResultText(text), nil
	}
}

// waitForChange polls the task until its status changes, the task
// completes via Done channel, or the timeout expires. Progress-only
// changes do not trigger an early return — only status transitions
// (e.g. running→completed) do.
func waitForChange(ctx context.Context, t *task.Task, initial task.TaskSnapshot, timeout time.Duration) task.TaskSnapshot {
	deadline := time.After(timeout)
	ticker := time.NewTicker(longPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return t.Snapshot()
		case <-t.Done():
			return t.Snapshot()
		case <-deadline:
			return t.Snapshot()
		case <-ticker.C:
			snap := t.Snapshot()
			if snap.Status != initial.Status {
				return snap
			}
		}
	}
}

func isTerminalStatus(s task.Status) bool {
	return s == task.StatusCompleted || s == task.StatusFailed || s == task.StatusCancelled || s == task.StatusLinked
}

func formatCheckResponse(snap task.TaskSnapshot, includeOutput bool, outputLines int) string {
	var b strings.Builder

	if snap.Context != "" {
		fmt.Fprintf(&b, "Context: %s\n\n", snap.Context)
	}

	switch snap.Status {
	case task.StatusPending, task.StatusQueued:
		fmt.Fprintf(&b, "Status: %s\n", snap.Status)

	case task.StatusRunning:
		fmt.Fprintf(&b, "Status: running\n")
		fmt.Fprintf(&b, "Duration: %s\n", snap.FormatDuration())
		if snap.Progress != "" {
			fmt.Fprintf(&b, "Progress: %s\n", snap.Progress)
		}
		if snap.CostUSD > 0 {
			fmt.Fprintf(&b, "Cost so far: ~$%.2f\n", snap.CostUSD)
		}

	case task.StatusCompleted:
		fmt.Fprintf(&b, "Status: completed\n")
		fmt.Fprintf(&b, "Duration: %s\n", snap.FormatDuration())
		if snap.Model != "" {
			fmt.Fprintf(&b, "Model: %s\n", snap.Model)
		}
		if snap.CostUSD > 0 {
			fmt.Fprintf(&b, "Cost: $%.2f\n", snap.CostUSD)
		}
		if snap.Turns > 0 {
			fmt.Fprintf(&b, "Turns: %d\n", snap.Turns)
		}
		if snap.SessionID != "" {
			fmt.Fprintf(&b, "Session ID: %s (use to continue this conversation)\n", snap.SessionID)
		}
		b.WriteString("\nUse get_result for full output, get_diff for changes.")

	case task.StatusFailed:
		fmt.Fprintf(&b, "Status: failed\n")
		fmt.Fprintf(&b, "Duration: %s\n", snap.FormatDuration())
		if snap.Error != "" {
			fmt.Fprintf(&b, "Error: %s\n", snap.Error)
		}

	case task.StatusCancelled:
		fmt.Fprintf(&b, "Status: cancelled\n")
		fmt.Fprintf(&b, "Duration: %s\n", snap.FormatDuration())

	case task.StatusLinked:
		fmt.Fprintf(&b, "Status: linked (external Claude Code session)\n")
		fmt.Fprintf(&b, "Session ID: %s\n", snap.SessionID)
		if snap.Project != "" {
			fmt.Fprintf(&b, "Project: %s\n", snap.Project)
		}
		if snap.GitBranch != "" {
			fmt.Fprintf(&b, "Branch: %s\n", snap.GitBranch)
		}
		if snap.Output != "" {
			fmt.Fprintf(&b, "\nSummary:\n%s\n", snap.Output)
		}
		if snap.Progress != "" {
			fmt.Fprintf(&b, "\nCurrent task: %s\n", snap.Progress)
		}
		if len(snap.FilesModified) > 0 {
			fmt.Fprintf(&b, "\nFiles modified (%d):\n", len(snap.FilesModified))
			for _, f := range snap.FilesModified {
				fmt.Fprintf(&b, "  - %s\n", f)
			}
		}
		if snap.Turns > 0 {
			fmt.Fprintf(&b, "Turns: %d\n", snap.Turns)
		}
		fmt.Fprintf(&b, "\nUse start_task with session_id %q to resume this session.", snap.SessionID)
	}

	if includeOutput && snap.Output != "" {
		lines := lastNLines(snap.Output, outputLines)
		fmt.Fprintf(&b, "\n--- Last output ---\n%s", lines)
	}

	return b.String()
}

func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
