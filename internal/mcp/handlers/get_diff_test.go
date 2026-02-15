package handlers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/btouchard/herald/internal/config"
	"github.com/btouchard/herald/internal/project"
	"github.com/btouchard/herald/internal/task"
)

// initGitRepo creates a temporary git repo with an initial commit.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...) //nolint:gosec // test helper with hardcoded args
		cmd.Dir = dir
		require.NoError(t, cmd.Run(), "failed to run: %v", args)
	}

	// Create a file and make an initial commit
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0600))
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	return dir
}

func newDiffTestDeps(repoPath string) (*task.Manager, *project.Manager) {
	pm := project.NewManager(map[string]config.Project{
		"test-repo": {
			Path:        repoPath,
			Description: "Test repo",
			Default:     true,
		},
	})
	tm := task.NewManager(&mockExecutor{}, 3, 2*time.Hour)
	return tm, pm
}

func TestGetDiff_WhenMissingBothParams_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, pm := newTestDeps()
	handler := GetDiff(tm, pm)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "task_id or project is required")
}

func TestGetDiff_WhenTaskNotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, pm := newTestDeps()
	handler := GetDiff(tm, pm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": "herald-nonexist",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Task not found")
}

func TestGetDiff_WhenProjectNotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, pm := newTestDeps()
	handler := GetDiff(tm, pm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"project": "nonexistent-project",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Project error")
}

func TestGetDiff_WhenProjectHasNoChanges_ReturnsNoChanges(t *testing.T) {
	t.Parallel()
	repoPath := initGitRepo(t)
	_, pm := newDiffTestDeps(repoPath)
	tm := task.NewManager(&mockExecutor{}, 3, 2*time.Hour)
	handler := GetDiff(tm, pm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"project": "test-repo",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "No changes detected")
}

func TestGetDiff_WhenProjectHasChanges_ReturnsDiff(t *testing.T) {
	t.Parallel()
	repoPath := initGitRepo(t)
	_, pm := newDiffTestDeps(repoPath)
	tm := task.NewManager(&mockExecutor{}, 3, 2*time.Hour)
	handler := GetDiff(tm, pm)

	// Make an uncommitted change
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0600))

	result, err := handler(context.Background(), makeReq(map[string]any{
		"project": "test-repo",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "```diff")
	assert.Contains(t, text, "main.go")
}

func TestGetDiff_WhenTaskWithProjectNotInManager_ReturnsProjectError(t *testing.T) {
	t.Parallel()

	pm := project.NewManager(map[string]config.Project{})
	tm := task.NewManager(&mockExecutor{}, 3, 2*time.Hour)
	handler := GetDiff(tm, pm)

	tsk := tm.Create("unknown-project", "work", "", task.PriorityNormal, 30)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Project not found")
}

func TestGetDiff_WhenProjectNotGitRepo_ReturnsError(t *testing.T) {
	t.Parallel()
	notRepo := t.TempDir() // plain directory, not a git repo
	pm := project.NewManager(map[string]config.Project{
		"not-a-repo": {
			Path:    notRepo,
			Default: true,
		},
	})
	tm := task.NewManager(&mockExecutor{}, 3, 2*time.Hour)
	handler := GetDiff(tm, pm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"project": "not-a-repo",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "not a git repository")
}

func TestGetDiff_WhenTaskHasProject_UsesTaskProject(t *testing.T) {
	t.Parallel()
	repoPath := initGitRepo(t)
	tm, pm := newDiffTestDeps(repoPath)
	handler := GetDiff(tm, pm)

	tsk := tm.Create("test-repo", "some task", "", task.PriorityNormal, 30)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "No changes detected")
}

// --- ReadFile handler tests ---

type readFileDeps struct {
	tm      *task.Manager
	pm      *project.Manager
	handler func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
	root    string
}

func newReadFileDeps(t *testing.T) readFileDeps {
	t.Helper()
	root := t.TempDir()

	// Create test files
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "handler.go"), []byte("package src\n"), 0600))

	pm := project.NewManager(map[string]config.Project{
		"test": {
			Path:    root,
			Default: true,
		},
	})

	me := &mockExecutor{}
	tm := task.NewManager(me, 3, 2*time.Hour)
	handler := ReadFile(pm)

	return readFileDeps{tm: tm, pm: pm, handler: handler, root: root}
}

func TestReadFile_WhenValidFile_ReturnsContent(t *testing.T) {
	t.Parallel()
	deps := newReadFileDeps(t)

	result, err := deps.handler(context.Background(), makeReq(map[string]any{
		"path": "main.go",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "main.go")
	assert.Contains(t, text, "package main")
	assert.Contains(t, text, "```")
}

func TestReadFile_WhenNestedFile_ReturnsContent(t *testing.T) {
	t.Parallel()
	deps := newReadFileDeps(t)

	result, err := deps.handler(context.Background(), makeReq(map[string]any{
		"path": "src/handler.go",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "package src")
}

func TestReadFile_WhenMissingPath_ReturnsError(t *testing.T) {
	t.Parallel()
	deps := newReadFileDeps(t)

	result, err := deps.handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "path is required")
}

func TestReadFile_WhenFileNotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	deps := newReadFileDeps(t)

	result, err := deps.handler(context.Background(), makeReq(map[string]any{
		"path": "nonexistent.go",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "File not found")
}

func TestReadFile_WhenPathTraversal_ReturnsError(t *testing.T) {
	t.Parallel()
	deps := newReadFileDeps(t)

	result, err := deps.handler(context.Background(), makeReq(map[string]any{
		"path": "../../../etc/passwd",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Access denied")
}

func TestReadFile_WhenAbsolutePath_ReturnsError(t *testing.T) {
	t.Parallel()
	deps := newReadFileDeps(t)

	result, err := deps.handler(context.Background(), makeReq(map[string]any{
		"path": "/etc/passwd",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Access denied")
}

func TestReadFile_WhenDirectory_ReturnsError(t *testing.T) {
	t.Parallel()
	deps := newReadFileDeps(t)

	result, err := deps.handler(context.Background(), makeReq(map[string]any{
		"path": "src",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "is a directory")
}

func TestReadFile_WhenFileTooLarge_ReturnsError(t *testing.T) {
	t.Parallel()
	deps := newReadFileDeps(t)

	// Create a file larger than 1MB
	bigContent := make([]byte, 1024*1024+1)
	require.NoError(t, os.WriteFile(filepath.Join(deps.root, "big.bin"), bigContent, 0600))

	result, err := deps.handler(context.Background(), makeReq(map[string]any{
		"path": "big.bin",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "File too large")
}

func TestReadFile_WhenInvalidProject_ReturnsError(t *testing.T) {
	t.Parallel()

	pm := project.NewManager(map[string]config.Project{})
	handler := ReadFile(pm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"path":    "file.go",
		"project": "nonexistent",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Project error")
}

// --- Additional coverage for formatEstimate ---

func TestFormatEstimate_WhenUnderOneMinute_ReturnsSeconds(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "45s", formatEstimate(45*time.Second))
}

func TestFormatEstimate_WhenOverOneMinute_ReturnsMinutes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "3m", formatEstimate(3*time.Minute))
}

func TestFormatEstimate_WhenExactlyOneMinute_ReturnsMinutes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "1m", formatEstimate(1*time.Minute))
}

// --- Additional statusIcon coverage ---

func TestStatusIcon_AllStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status task.Status
		icon   string
	}{
		{task.StatusPending, "⏳"},
		{task.StatusQueued, "📥"},
		{task.StatusRunning, "🔄"},
		{task.StatusCompleted, "✅"},
		{task.StatusFailed, "❌"},
		{task.StatusCancelled, "🚫"},
		{task.StatusLinked, "🔗"},
		{task.Status("unknown"), "❓"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.icon, statusIcon(tt.status))
		})
	}
}

// --- Additional GetResult coverage ---

func TestGetResult_WhenTaskNotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": "herald-nonexist",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Task not found")
}

func TestGetResult_WhenTaskFailed_ShowsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	tsk := tm.Create("test", "fix", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetError("claude exited with code 1")
	tsk.SetStatus(task.StatusFailed)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "failed")
	assert.Contains(t, text, "claude exited with code 1")
}

func TestGetResult_WhenTaskCancelled_ShowsCancelled(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	tsk := tm.Create("test", "work", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetStatus(task.StatusCancelled)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "cancelled")
}

func TestGetResult_WhenFormatFull_WithCostAndError_ShowsAll(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	tsk := tm.Create("test", "fix", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetCost(0.50)
	tsk.SetError("partial failure")
	tsk.AppendOutput("Some output before failure")
	tsk.SetStatus(task.StatusFailed)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
		"format":  "full",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "$0.50")
	assert.Contains(t, text, "partial failure")
	assert.Contains(t, text, "Some output before failure")
	assert.Contains(t, text, "Full output")
}

func TestGetResult_WhenTaskPending_InformsUserStillPending(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	tsk := tm.Create("test", "work", "", task.PriorityNormal, 30)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "still pending")
}

func TestGetResult_WhenCompletedWithSessionID_ShowsSessionID(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	tsk := tm.Create("test", "fix", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetSessionID("ses_completed")
	tsk.SetTurns(8)
	tsk.SetStatus(task.StatusCompleted)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "ses_completed")
	assert.Contains(t, text, "8")
}

// --- Additional ListTasks coverage ---

func TestListTasks_WhenFilterByProject_ReturnsMatching(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := ListTasks(tm)

	tm.Create("project-a", "task 1", "", task.PriorityNormal, 30)
	tm.Create("project-b", "task 2", "", task.PriorityNormal, 30)
	tm.Create("project-a", "task 3", "", task.PriorityNormal, 30)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"project": "project-a",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "2 found")
}

func TestListTasks_WhenRunningTask_ShowsDurationAndProgress(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := ListTasks(tm)

	tsk := tm.Create("test", "work", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetProgress("fixing auth bug")

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "running")
	assert.Contains(t, text, "fixing auth bug")
}

func TestListTasks_WhenCompletedTask_ShowsCost(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := ListTasks(tm)

	tsk := tm.Create("test", "work", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetCost(0.75)
	tsk.SetStatus(task.StatusCompleted)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "$0.75")
}

func TestListTasks_WhenFailedTaskWithError_ShowsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := ListTasks(tm)

	tsk := tm.Create("test", "work", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetError("timeout exceeded")
	tsk.SetStatus(task.StatusFailed)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "timeout exceeded")
}

// --- Additional GetLogs coverage ---

func TestGetLogs_WhenTaskHasSessionAndCost_ShowsAll(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetLogs(tm)

	tsk := tm.Create("test", "work", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetSessionID("ses_logs")
	tsk.SetCost(0.42)
	tsk.SetTurns(5)
	tsk.SetError("some warning")
	tsk.SetStatus(task.StatusFailed)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "ses_logs")
	assert.Contains(t, text, "$0.42")
	assert.Contains(t, text, "5")
	assert.Contains(t, text, "some warning")
}

func TestGetLogs_WhenLimitProvided_RespectsIt(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetLogs(tm)

	for range 5 {
		tm.Create("test", "task", "", task.PriorityNormal, 30)
	}

	result, err := handler(context.Background(), makeReq(map[string]any{
		"limit": float64(2),
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "2 tasks")
}

// --- Additional CheckTask coverage ---

func TestCheckTask_WhenRunning_ShowsProgressAndCost(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	tsk := tm.Create("test", "do work", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetProgress("editing files")
	tsk.SetCost(0.15)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "running")
	assert.Contains(t, text, "editing files")
	assert.Contains(t, text, "$0.15")
}

func TestCheckTask_WhenFailed_ShowsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	tsk := tm.Create("test", "do work", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetError("claude crashed")
	tsk.SetStatus(task.StatusFailed)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "failed")
	assert.Contains(t, text, "claude crashed")
}

func TestCheckTask_WhenCancelled_ShowsCancelled(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	tsk := tm.Create("test", "do work", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetStatus(task.StatusCancelled)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "cancelled")
}

// --- Additional CancelTask coverage ---

func TestCancelTask_WhenAlreadyCompleted_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CancelTask(tm)

	tsk := tm.Create("test", "work", "", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetStatus(task.StatusCompleted)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Failed to cancel")
}

// --- StartTask additional coverage ---

func TestStartTask_WhenMissingPrompt_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, pm := newTestDeps()
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", nil)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "prompt is required")
}

func TestStartTask_WhenDryRun_ShowsDryRunMode(t *testing.T) {
	t.Parallel()
	tm, pm := newTestDeps()
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", nil)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt":  "plan the refactoring",
		"dry_run": true,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "dry run")
}

func TestStartTask_WhenSessionID_ShowsResuming(t *testing.T) {
	t.Parallel()
	tm, pm := newTestDeps()
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", nil)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt":     "continue",
		"session_id": "ses_resume123",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Resuming session")
	assert.Contains(t, text, "ses_resume123")
}
