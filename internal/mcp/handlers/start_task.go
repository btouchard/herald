package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/kolapsis/herald/internal/executor"
	"github.com/kolapsis/herald/internal/project"
	"github.com/kolapsis/herald/internal/task"
)

// StartTask returns a handler that creates and starts a Claude Code task.
// defaultTimeout and maxTimeout are expressed as time.Duration.
// maxPromptSize limits prompt length in bytes (0 = no limit).
func StartTask(tm *task.Manager, pm *project.Manager, defaultTimeout, maxTimeout time.Duration, maxPromptSize int) server.ToolHandlerFunc {
	defaultMinutes := int(defaultTimeout.Minutes())
	if defaultMinutes <= 0 {
		defaultMinutes = 30
	}
	maxMinutes := int(maxTimeout.Minutes())
	if maxMinutes <= 0 {
		maxMinutes = 120
	}

	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		prompt, _ := args["prompt"].(string)
		if prompt == "" {
			return mcp.NewToolResultError("prompt is required"), nil
		}

		if maxPromptSize > 0 && len(prompt) > maxPromptSize {
			return mcp.NewToolResultError(fmt.Sprintf("prompt too large: %d bytes (max %d)", len(prompt), maxPromptSize)), nil
		}

		projectName, _ := args["project"].(string)
		proj, err := pm.Resolve(projectName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Project error: %s", err)), nil
		}

		priority := task.PriorityNormal
		if p, ok := args["priority"].(string); ok && p != "" {
			priority = task.Priority(p)
		}

		timeoutMinutes := defaultMinutes
		if t, ok := args["timeout_minutes"].(float64); ok && t > 0 {
			timeoutMinutes = int(t)
		}
		if timeoutMinutes > maxMinutes {
			slog.Warn("timeout clamped to max",
				"requested", timeoutMinutes,
				"max", maxMinutes)
			timeoutMinutes = maxMinutes
		}

		sessionID, _ := args["session_id"].(string)
		gitBranch, _ := args["git_branch"].(string)
		dryRun, _ := args["dry_run"].(bool)

		// Create the task
		t := tm.Create(proj.Name, prompt, priority, timeoutMinutes)
		t.GitBranch = gitBranch
		t.DryRun = dryRun
		t.AllowedTools = proj.AllowedTools

		// Build executor request
		execReq := executor.Request{
			TaskID:         t.ID,
			Prompt:         prompt,
			ProjectPath:    proj.Path,
			SessionID:      sessionID,
			AllowedTools:   proj.AllowedTools,
			TimeoutMinutes: timeoutMinutes,
			DryRun:         dryRun,
		}

		// Start execution (enforces global + per-project concurrency limits)
		if err := tm.Start(ctx, t, execReq, proj.MaxConcurrentTasks); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Cannot start task: %s", err)), nil
		}

		// Build response
		var b strings.Builder
		fmt.Fprintf(&b, "Task started\n\n")
		fmt.Fprintf(&b, "- ID: %s\n", t.ID)
		fmt.Fprintf(&b, "- Project: %s\n", proj.Name)
		fmt.Fprintf(&b, "- Priority: %s\n", string(priority))
		if dryRun {
			b.WriteString("- Mode: dry run (plan only)\n")
		}
		if sessionID != "" {
			fmt.Fprintf(&b, "- Resuming session: %s\n", sessionID)
		}
		fmt.Fprintf(&b, "\nUse check_task with ID '%s' to monitor progress.", t.ID)

		return mcp.NewToolResultText(b.String()), nil
	}
}
