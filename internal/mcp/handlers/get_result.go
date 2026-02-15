package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/btouchard/herald/internal/task"
)

// GetResult returns a handler that provides the full result of a completed task.
func GetResult(tm *task.Manager) server.ToolHandlerFunc {
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

		snap := t.Snapshot()

		if snap.Status == task.StatusRunning || snap.Status == task.StatusPending || snap.Status == task.StatusQueued {
			return mcp.NewToolResultText(
				fmt.Sprintf("Task %s is still %s. Use check_task to monitor progress.", taskID, snap.Status),
			), nil
		}

		format := "summary"
		if f, ok := args["format"].(string); ok && f != "" {
			format = f
		}

		switch format {
		case "json":
			return formatJSON(snap)
		case "full":
			return formatFull(snap)
		default:
			return formatSummary(snap)
		}
	}
}

func formatSummary(snap task.TaskSnapshot) (*mcp.CallToolResult, error) {
	var b strings.Builder

	switch snap.Status {
	case task.StatusCompleted:
		b.WriteString("Task completed\n\n")
	case task.StatusFailed:
		b.WriteString("Task failed\n\n")
	case task.StatusCancelled:
		b.WriteString("Task cancelled\n\n")
	}

	fmt.Fprintf(&b, "- ID: %s\n", snap.ID)
	if snap.Context != "" {
		fmt.Fprintf(&b, "- Context: %s\n", snap.Context)
	}
	fmt.Fprintf(&b, "- Project: %s\n", snap.Project)
	if snap.Model != "" {
		fmt.Fprintf(&b, "- Model: %s\n", snap.Model)
	}
	fmt.Fprintf(&b, "- Duration: %s\n", snap.FormatDuration())

	if snap.CostUSD > 0 {
		fmt.Fprintf(&b, "- Cost: $%.2f\n", snap.CostUSD)
	}
	if snap.Turns > 0 {
		fmt.Fprintf(&b, "- Turns: %d\n", snap.Turns)
	}

	if snap.Error != "" {
		fmt.Fprintf(&b, "- Error: %s\n", snap.Error)
	}

	if snap.Output != "" {
		summary := truncateSummary(snap.Output, 1000)
		fmt.Fprintf(&b, "\nSummary:\n%s\n", summary)
	}

	if snap.SessionID != "" {
		fmt.Fprintf(&b, "\nSession ID: %s — use in start_task to continue.", snap.SessionID)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func formatFull(snap task.TaskSnapshot) (*mcp.CallToolResult, error) {
	var b strings.Builder

	fmt.Fprintf(&b, "Task %s — %s\n", snap.ID, snap.Status)
	if snap.Context != "" {
		fmt.Fprintf(&b, "Context: %s\n", snap.Context)
	}
	fmt.Fprintf(&b, "Project: %s | Duration: %s", snap.Project, snap.FormatDuration())
	if snap.CostUSD > 0 {
		fmt.Fprintf(&b, " | Cost: $%.2f", snap.CostUSD)
	}
	b.WriteString("\n\n")

	if snap.Error != "" {
		fmt.Fprintf(&b, "Error: %s\n\n", snap.Error)
	}

	if snap.Output != "" {
		fmt.Fprintf(&b, "--- Full output ---\n%s\n", snap.Output)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func formatJSON(snap task.TaskSnapshot) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("JSON encoding error: %s", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func truncateSummary(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n\n[... output truncated, use format='full' for complete output]"
}
