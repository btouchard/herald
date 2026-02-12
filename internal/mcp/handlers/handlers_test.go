package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kolapsis/herald/internal/task"
)

// --- CheckTask tests ---

func TestCheckTask_WhenTaskExists_ReturnsStatus(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	// Create a task via manager
	tsk := tm.Create("test", "do something", task.PriorityNormal, 30)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "pending")
}

func TestCheckTask_WhenMissingTaskID_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "task_id is required")
}

func TestCheckTask_WhenTaskNotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": "herald-nonexist",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "not found")
}

func TestCheckTask_WhenCompleted_ShowsCostAndTurns(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	tsk := tm.Create("test", "do something", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetCost(0.42)
	tsk.SetTurns(5)
	tsk.SetSessionID("ses_abc")
	tsk.SetStatus(task.StatusCompleted)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "completed")
	assert.Contains(t, text, "$0.42")
	assert.Contains(t, text, "5")
	assert.Contains(t, text, "ses_abc")
}

func TestCheckTask_WhenIncludeOutput_ShowsOutput(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	tsk := tm.Create("test", "do something", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.AppendOutput("line 1\nline 2\nline 3\n")
	tsk.SetStatus(task.StatusCompleted)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id":        tsk.ID,
		"include_output": true,
		"output_lines":   float64(2),
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Last output")
}

// --- CheckTask long-polling tests ---

func TestCheckTask_WhenWaitZero_ReturnsImmediately(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	tsk := tm.Create("test", "do something", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)

	start := time.Now()
	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id":      tsk.ID,
		"wait_seconds": float64(0),
	}))
	elapsed := time.Since(start)

	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "running")
	assert.Less(t, elapsed, 500*time.Millisecond, "wait_seconds=0 should return immediately")
}

func TestCheckTask_WhenWaitOnCompletedTask_ReturnsImmediately(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	tsk := tm.Create("test", "do something", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetStatus(task.StatusCompleted)

	start := time.Now()
	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id":      tsk.ID,
		"wait_seconds": float64(5),
	}))
	elapsed := time.Since(start)

	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "completed")
	assert.Less(t, elapsed, 500*time.Millisecond, "completed task should not wait")
}

func TestCheckTask_WhenWaitAndTaskCompletesDuringWait_ReturnsOnCompletion(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	tsk := tm.Create("test", "do something", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)

	// Complete the task after 300ms
	go func() {
		time.Sleep(300 * time.Millisecond)
		tsk.SetStatus(task.StatusCompleted)
	}()

	start := time.Now()
	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id":      tsk.ID,
		"wait_seconds": float64(5),
	}))
	elapsed := time.Since(start)

	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "completed")
	assert.Less(t, elapsed, 2*time.Second, "should return soon after task completes")
	assert.Greater(t, elapsed, 200*time.Millisecond, "should have waited for completion")
}

func TestCheckTask_WhenWaitAndProgressChanges_ReturnsOnChange(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	tsk := tm.Create("test", "do something", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetProgress("step 1")

	// Change progress after 300ms
	go func() {
		time.Sleep(300 * time.Millisecond)
		tsk.SetProgress("step 2")
	}()

	start := time.Now()
	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id":      tsk.ID,
		"wait_seconds": float64(5),
	}))
	elapsed := time.Since(start)

	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "step 2")
	assert.Less(t, elapsed, 2*time.Second, "should return soon after progress changes")
}

func TestCheckTask_WhenWaitExceedsMax_ClampsToMax(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CheckTask(tm)

	tsk := tm.Create("test", "do something", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)

	// Request 60s wait (exceeds max of 30), should be clamped.
	// We cancel the context after 1s to avoid actually waiting 30s in tests.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result, err := handler(ctx, makeReq(map[string]any{
		"task_id":      tsk.ID,
		"wait_seconds": float64(60),
	}))

	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "running")
}

// --- GetResult tests ---

func TestGetResult_WhenTaskCompleted_ReturnsSummary(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	tsk := tm.Create("test", "fix the bug", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetCost(0.34)
	tsk.AppendOutput("Fixed the auth bug by updating token validation.")
	tsk.SetStatus(task.StatusCompleted)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "completed")
	assert.Contains(t, text, "$0.34")
	assert.Contains(t, text, "Fixed the auth bug")
}

func TestGetResult_WhenMissingTaskID_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "task_id is required")
}

func TestGetResult_WhenTaskStillRunning_InformsUser(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	tsk := tm.Create("test", "work", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "still running")
}

func TestGetResult_WhenFormatFull_ReturnsFullOutput(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	tsk := tm.Create("test", "fix", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.AppendOutput("Full output content here.")
	tsk.SetStatus(task.StatusCompleted)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
		"format":  "full",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Full output content here.")
	assert.Contains(t, text, "Full output")
}

func TestGetResult_WhenFormatJSON_ReturnsJSON(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetResult(tm)

	tsk := tm.Create("test", "fix", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetStatus(task.StatusCompleted)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
		"format":  "json",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, `"ID"`)
	assert.Contains(t, text, `"Status"`)
}

// --- CancelTask tests ---

func TestCancelTask_WhenRunning_CancelsSuccessfully(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CancelTask(tm)

	tsk := tm.Create("test", "work", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "cancelled")
}

func TestCancelTask_WhenMissingTaskID_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CancelTask(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "task_id is required")
}

func TestCancelTask_WhenTaskNotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := CancelTask(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": "herald-nonexist",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Failed to cancel")
}

// --- ListTasks tests ---

func TestListTasks_WhenNoTasks_ReturnsEmptyMessage(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := ListTasks(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "No tasks found")
}

func TestListTasks_WhenTasksExist_ListsThem(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := ListTasks(tm)

	tm.Create("test", "task 1", task.PriorityNormal, 30)
	tm.Create("test", "task 2", task.PriorityHigh, 30)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "2 found")
	assert.Contains(t, text, "test")
}

func TestListTasks_WhenFilterByStatus_ReturnsMatching(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := ListTasks(tm)

	tsk := tm.Create("test", "task 1", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetStatus(task.StatusCompleted)
	tm.Create("test", "task 2", task.PriorityNormal, 30) // stays pending

	result, err := handler(context.Background(), makeReq(map[string]any{
		"status": "pending",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "1 found")
}

func TestListTasks_WhenLimitSet_RespectsIt(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := ListTasks(tm)

	for range 5 {
		tm.Create("test", "task", task.PriorityNormal, 30)
	}

	result, err := handler(context.Background(), makeReq(map[string]any{
		"limit": float64(2),
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "2 found")
}

// --- GetLogs tests ---

func TestGetLogs_WhenTaskExists_ShowsLogs(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetLogs(tm)

	tsk := tm.Create("test", "work", task.PriorityNormal, 30)
	tsk.SetStatus(task.StatusRunning)
	tsk.SetProgress("fixing bugs")
	tsk.SetCost(0.15)
	tsk.SetStatus(task.StatusCompleted)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": tsk.ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, tsk.ID)
	assert.Contains(t, text, "completed")
	assert.Contains(t, text, "fixing bugs")
}

func TestGetLogs_WhenTaskNotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetLogs(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"task_id": "herald-nonexist",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "not found")
}

func TestGetLogs_WhenNoTaskID_ShowsRecentActivity(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetLogs(tm)

	tm.Create("test", "task 1", task.PriorityNormal, 30)
	time.Sleep(10 * time.Millisecond) // ensure ordering
	tm.Create("test", "task 2", task.PriorityNormal, 30)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Recent activity")
	assert.Contains(t, text, "2 tasks")
}

func TestGetLogs_WhenNoActivity_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := GetLogs(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "No activity")
}

// --- Helper tests ---

func TestLastNLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		n      int
		expect string
	}{
		{"fewer lines than n", "a\nb", 5, "a\nb"},
		{"exact n", "a\nb\nc", 3, "a\nb\nc"},
		{"more lines than n", "a\nb\nc\nd\ne", 2, "d\ne"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expect, lastNLines(tt.input, tt.n))
		})
	}
}

func TestTruncateSummary(t *testing.T) {
	t.Parallel()

	short := "short text"
	assert.Equal(t, short, truncateSummary(short, 100))

	long := string(make([]byte, 200))
	result := truncateSummary(long, 50)
	assert.Len(t, result, 50+len("\n\n[... output truncated, use format='full' for complete output]"))
}
