package handlers

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/btouchard/herald/internal/task"
)

func TestHeraldPush_WhenAllFields_CreatesLinkedTask(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := HeraldPush(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"session_id":     "ses_abc123",
		"summary":        "Refactored auth middleware, added rate limiting",
		"project":        "herald",
		"files_modified": []interface{}{"internal/auth/oauth.go", "internal/mcp/middleware/auth.go"},
		"current_task":   "Writing tests for rate limiter",
		"git_branch":     "feat/rate-limit",
		"turns":          float64(12),
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Session pushed to Herald")
	assert.Contains(t, text, "ses_abc123")
	assert.Contains(t, text, "herald")
	assert.Contains(t, text, "linked")
	assert.Contains(t, text, "herald-") // task ID prefix

	// Verify task was created with correct type and status
	tasks := tm.List(task.Filter{})
	require.Len(t, tasks, 1)
	assert.Equal(t, task.TypeLinked, tasks[0].Type)
	assert.Equal(t, task.StatusLinked, tasks[0].Status)
	assert.Equal(t, "ses_abc123", tasks[0].SessionID)
	assert.Equal(t, "herald", tasks[0].Project)
	assert.Equal(t, "feat/rate-limit", tasks[0].GitBranch)
	assert.Equal(t, "Writing tests for rate limiter", tasks[0].Progress)
	assert.Equal(t, 12, tasks[0].Turns)
	assert.Contains(t, tasks[0].Output, "Refactored auth middleware")
	assert.Equal(t, []string{"internal/auth/oauth.go", "internal/mcp/middleware/auth.go"}, tasks[0].FilesModified)
}

func TestHeraldPush_WhenMinimalFields_CreatesLinkedTask(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := HeraldPush(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"session_id": "ses_minimal",
		"summary":    "Quick fix",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Session pushed to Herald")
	assert.Contains(t, text, "ses_minimal")

	tasks := tm.List(task.Filter{})
	require.Len(t, tasks, 1)
	assert.Equal(t, task.TypeLinked, tasks[0].Type)
	assert.Equal(t, task.StatusLinked, tasks[0].Status)
	assert.Equal(t, "ses_minimal", tasks[0].SessionID)
	assert.Contains(t, tasks[0].Output, "Quick fix")
}

func TestHeraldPush_WhenSameSessionID_UpdatesExistingTask(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := HeraldPush(tm)

	// First push
	result1, err := handler(context.Background(), makeReq(map[string]any{
		"session_id": "ses_dedup",
		"summary":    "Initial work",
		"project":    "my-api",
		"turns":      float64(5),
	}))
	require.NoError(t, err)
	text1 := result1.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text1, "Session pushed to Herald")

	// Second push with same session_id
	result2, err := handler(context.Background(), makeReq(map[string]any{
		"session_id":   "ses_dedup",
		"summary":      "Updated work with more progress",
		"project":      "my-api",
		"current_task": "Finishing up tests",
		"turns":        float64(10),
	}))
	require.NoError(t, err)
	text2 := result2.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text2, "Session updated in Herald")

	// Should still be only 1 task, not 2
	tasks := tm.List(task.Filter{})
	require.Len(t, tasks, 1)
	assert.Equal(t, task.StatusLinked, tasks[0].Status)
	assert.Equal(t, "ses_dedup", tasks[0].SessionID)
	assert.Contains(t, tasks[0].Output, "Updated work with more progress")
	assert.Equal(t, "Finishing up tests", tasks[0].Progress)
	assert.Equal(t, 10, tasks[0].Turns)
}

func TestHeraldPush_WhenMissingSessionID_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := HeraldPush(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"summary": "some work",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "session_id is required")
}

func TestHeraldPush_WhenMissingSummary_ReturnsError(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()
	handler := HeraldPush(tm)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"session_id": "ses_nosummary",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "summary is required")
}

func TestCheckTask_WhenLinkedTask_ShowsLinkedInfo(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()

	// Create linked task via herald_push
	pushHandler := HeraldPush(tm)
	_, err := pushHandler(context.Background(), makeReq(map[string]any{
		"session_id":     "ses_check",
		"summary":        "Implemented new feature",
		"project":        "herald",
		"current_task":   "Adding tests",
		"git_branch":     "feat/new-thing",
		"files_modified": []interface{}{"main.go", "handler.go"},
		"turns":          float64(8),
	}))
	require.NoError(t, err)

	// Find the task ID
	tasks := tm.List(task.Filter{})
	require.Len(t, tasks, 1)

	// Check the linked task
	checkHandler := CheckTask(tm)
	result, err := checkHandler(context.Background(), makeReq(map[string]any{
		"task_id": tasks[0].ID,
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "linked")
	assert.Contains(t, text, "ses_check")
	assert.Contains(t, text, "herald")
	assert.Contains(t, text, "feat/new-thing")
	assert.Contains(t, text, "Implemented new feature")
	assert.Contains(t, text, "Adding tests")
	assert.Contains(t, text, "main.go")
	assert.Contains(t, text, "handler.go")
	assert.Contains(t, text, "8")
}

func TestListTasks_WhenLinkedTaskExists_ShowsWithLinkIndicator(t *testing.T) {
	t.Parallel()
	tm, _ := newTestDeps()

	// Create a regular task
	tm.Create("test", "regular task", "", task.PriorityNormal, 30)

	// Create a linked task
	pushHandler := HeraldPush(tm)
	_, err := pushHandler(context.Background(), makeReq(map[string]any{
		"session_id": "ses_list",
		"summary":    "Linked session work",
		"project":    "test",
	}))
	require.NoError(t, err)

	listHandler := ListTasks(tm)
	result, err := listHandler(context.Background(), makeReq(map[string]any{}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "2 found")
	assert.Contains(t, text, "linked")
	assert.Contains(t, text, "ses_list")
	assert.Contains(t, text, "start_task to resume")
}
