package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/kolapsis/herald/internal/mcp/handlers"
)

func registerTools(s *server.MCPServer, deps *Deps) {
	// list_projects — List configured projects with Git status
	s.AddTool(
		mcp.NewTool("list_projects",
			mcp.WithDescription("List all configured projects with their Git status and description."),
		),
		handlers.ListProjects(deps.Projects),
	)

	// start_task — Launch a Claude Code task
	s.AddTool(
		mcp.NewTool("start_task",
			mcp.WithDescription("Start a Claude Code task on a project. Returns immediately with a task ID. The task runs asynchronously — use check_task to monitor progress."),
			mcp.WithString("prompt",
				mcp.Required(),
				mcp.Description("The task instructions for Claude Code"),
			),
			mcp.WithString("project",
				mcp.Description("Project name from configuration. If omitted, uses default project."),
			),
			mcp.WithString("priority",
				mcp.Description("Task priority in the execution queue"),
				mcp.Enum("low", "normal", "high", "urgent"),
			),
			mcp.WithString("template",
				mcp.Description("Optional template name to use (e.g., 'review', 'test', 'fix')"),
			),
			mcp.WithString("session_id",
				mcp.Description("Claude Code session ID to resume (for multi-turn conversations)"),
			),
			mcp.WithNumber("timeout_minutes",
				mcp.Description("Maximum execution time in minutes (default: 30)"),
			),
			mcp.WithString("git_branch",
				mcp.Description("Git branch to create/use. Auto-generated if not specified."),
			),
			mcp.WithBoolean("dry_run",
				mcp.Description("If true, Claude Code plans but doesn't execute changes"),
			),
		),
		handlers.StartTask(deps.Tasks, deps.Projects, deps.Execution.DefaultTimeout, deps.Execution.MaxTimeout, deps.Execution.MaxPromptSize, deps.Store),
	)

	// check_task — Check task status
	s.AddTool(
		mcp.NewTool("check_task",
			mcp.WithDescription("Check the current status and progress of a running task. Supports long-polling with wait_seconds to reduce polling overhead."),
			mcp.WithString("task_id",
				mcp.Required(),
				mcp.Description("The task ID returned by start_task"),
			),
			mcp.WithNumber("wait_seconds",
				mcp.Description("Wait up to N seconds for status changes before responding (long-poll). 0 for immediate response."),
			),
			mcp.WithBoolean("include_output",
				mcp.Description("Include the last N lines of Claude Code output"),
			),
			mcp.WithNumber("output_lines",
				mcp.Description("Number of output lines to include (default: 20)"),
			),
		),
		handlers.CheckTask(deps.Tasks),
	)

	// get_result — Get full task result
	s.AddTool(
		mcp.NewTool("get_result",
			mcp.WithDescription("Get the complete result of a finished task."),
			mcp.WithString("task_id",
				mcp.Required(),
				mcp.Description("The task ID returned by start_task"),
			),
			mcp.WithString("format",
				mcp.Description("Output format: summary (key changes), full (complete output), json (raw)"),
				mcp.Enum("summary", "full", "json"),
			),
		),
		handlers.GetResult(deps.Tasks),
	)

	// list_tasks — List tasks
	s.AddTool(
		mcp.NewTool("list_tasks",
			mcp.WithDescription("List tasks with optional filters."),
			mcp.WithString("status",
				mcp.Description("Filter by status"),
				mcp.Enum("all", "pending", "running", "completed", "failed", "cancelled", "linked"),
			),
			mcp.WithString("project",
				mcp.Description("Filter by project name"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of tasks to return (default: 10)"),
			),
			mcp.WithString("since",
				mcp.Description("ISO 8601 datetime — only tasks after this time"),
			),
		),
		handlers.ListTasks(deps.Tasks),
	)

	// cancel_task — Cancel a running task
	s.AddTool(
		mcp.NewTool("cancel_task",
			mcp.WithDescription("Cancel a running or pending task."),
			mcp.WithString("task_id",
				mcp.Required(),
				mcp.Description("The task ID to cancel"),
			),
			mcp.WithBoolean("revert",
				mcp.Description("If true, git revert changes made by this task"),
			),
		),
		handlers.CancelTask(deps.Tasks),
	)

	// get_diff — Get Git diff for a task or project
	s.AddTool(
		mcp.NewTool("get_diff",
			mcp.WithDescription("Show Git diff of changes. Use task_id to diff a task's branch against current branch, or project to diff uncommitted changes."),
			mcp.WithString("task_id",
				mcp.Description("Task ID — diffs the task branch against the current branch"),
			),
			mcp.WithString("project",
				mcp.Description("Project name — diffs uncommitted changes against HEAD"),
			),
		),
		handlers.GetDiff(deps.Tasks, deps.Projects),
	)

	// read_file — Read a file from a project
	s.AddTool(
		mcp.NewTool("read_file",
			mcp.WithDescription("Read a file from a configured project (path-safe)."),
			mcp.WithString("project",
				mcp.Description("Project name. If omitted, uses default project."),
			),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Relative path within the project"),
			),
			mcp.WithNumber("line_start",
				mcp.Description("Start reading from this line number"),
			),
			mcp.WithNumber("line_end",
				mcp.Description("Stop reading at this line number"),
			),
		),
		handlers.ReadFile(deps.Projects),
	)

	// herald_push — Push Claude Code session context to Herald
	s.AddTool(
		mcp.NewTool("herald_push",
			mcp.WithDescription("Push the current Claude Code session context to Herald for remote monitoring and continuation. Call this when the user wants to continue working from another device."),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Current Claude Code session ID"),
			),
			mcp.WithString("summary",
				mcp.Required(),
				mcp.Description("Summary of what has been done in this session so far"),
			),
			mcp.WithString("project",
				mcp.Description("Project name or working directory path"),
			),
			mcp.WithArray("files_modified",
				mcp.Description("List of files created or modified during the session"),
				mcp.WithStringItems(),
			),
			mcp.WithString("current_task",
				mcp.Description("What was being worked on (in progress or next step)"),
			),
			mcp.WithString("git_branch",
				mcp.Description("Current git branch"),
			),
			mcp.WithNumber("turns",
				mcp.Description("Number of conversation turns so far"),
			),
		),
		handlers.HeraldPush(deps.Tasks),
	)

	// get_logs — Get logs and activity history
	s.AddTool(
		mcp.NewTool("get_logs",
			mcp.WithDescription("Get logs and activity history."),
			mcp.WithString("task_id",
				mcp.Description("Specific task ID. If omitted, shows recent activity."),
			),
			mcp.WithString("level",
				mcp.Description("Minimum log level to show"),
				mcp.Enum("debug", "info", "warn", "error"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of log entries to return (default: 50)"),
			),
		),
		handlers.GetLogs(deps.Tasks),
	)
}
