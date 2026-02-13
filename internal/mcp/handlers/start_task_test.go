package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/btouchard/herald/internal/executor"
	"github.com/btouchard/herald/internal/project"
	"github.com/btouchard/herald/internal/task"
	"github.com/btouchard/herald/internal/config"
)

type mockExecutor struct{}

func (m *mockExecutor) Execute(_ context.Context, _ executor.Request, _ executor.ProgressFunc) (*executor.Result, error) {
	return &executor.Result{Output: "done"}, nil
}

type mockEstimator struct {
	avgDuration time.Duration
	count       int
	err         error
}

func (m *mockEstimator) GetAverageTaskDuration(_ string) (time.Duration, int, error) {
	return m.avgDuration, m.count, m.err
}

func newTestDeps() (*task.Manager, *project.Manager) {
	pm := project.NewManager(map[string]config.Project{
		"test": {
			Path:        "/tmp",
			Description: "Test project",
			Default:     true,
		},
	})
	tm := task.NewManager(&mockExecutor{}, 3, 2*time.Hour)
	return tm, pm
}

func makeReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func TestStartTask_WhenNormalTimeout_AcceptsIt(t *testing.T) {
	t.Parallel()

	tm, pm := newTestDeps()
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", nil)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt":          "do something",
		"timeout_minutes": float64(45),
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Task started")

	// Verify the task was created with the requested timeout
	tasks := tm.List(task.Filter{})
	require.Len(t, tasks, 1)
	assert.Equal(t, 45, tasks[0].TimeoutMinutes)
}

func TestStartTask_WhenTimeoutExceedsMax_ClampsToMax(t *testing.T) {
	t.Parallel()

	tm, pm := newTestDeps()
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", nil) // max = 120 min

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt":          "do something",
		"timeout_minutes": float64(999999),
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Task started")

	tasks := tm.List(task.Filter{})
	require.Len(t, tasks, 1)
	assert.Equal(t, 120, tasks[0].TimeoutMinutes)
}

func TestStartTask_WhenTimeoutZeroOrNegative_UsesDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout float64
	}{
		{"zero", 0},
		{"negative", -10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tm, pm := newTestDeps()
			handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", nil)

			result, err := handler(context.Background(), makeReq(map[string]any{
				"prompt":          "do something",
				"timeout_minutes": tt.timeout,
			}))
			require.NoError(t, err)

			text := result.Content[0].(mcp.TextContent).Text
			assert.Contains(t, text, "Task started")

			tasks := tm.List(task.Filter{})
			require.Len(t, tasks, 1)
			assert.Equal(t, 30, tasks[0].TimeoutMinutes)
		})
	}
}

func TestStartTask_WhenNoTimeoutProvided_UsesDefault(t *testing.T) {
	t.Parallel()

	tm, pm := newTestDeps()
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", nil)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt": "do something",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Task started")

	tasks := tm.List(task.Filter{})
	require.Len(t, tasks, 1)
	assert.Equal(t, 30, tasks[0].TimeoutMinutes)
}

func TestStartTask_WhenPromptTooLarge_RejectsWithError(t *testing.T) {
	t.Parallel()

	tm, pm := newTestDeps()
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 100, "claude-sonnet-4-5-20250929", nil) // max 100 bytes

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt": string(make([]byte, 200)), // 200 bytes > 100 limit
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "prompt too large")
}

func TestStartTask_WhenEstimatorHasHistory_IncludesEstimate(t *testing.T) {
	t.Parallel()

	tm, pm := newTestDeps()
	est := &mockEstimator{avgDuration: 3 * time.Minute, count: 12}
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", est)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt": "do something",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Estimated duration: ~3m")
	assert.Contains(t, text, "12 previous tasks")
	assert.Contains(t, text, "Suggested first check")
}

func TestStartTask_WhenEstimatorNoHistory_ShowsUnknown(t *testing.T) {
	t.Parallel()

	tm, pm := newTestDeps()
	est := &mockEstimator{avgDuration: 0, count: 0}
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", est)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt": "do something",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "unknown (no task history")
}

func TestStartTask_WhenNilEstimator_SkipsEstimate(t *testing.T) {
	t.Parallel()

	tm, pm := newTestDeps()
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", nil)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt": "do something",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Task started")
	assert.NotContains(t, text, "Estimated duration")
}

func TestStartTask_WhenExplicitModel_UsesIt(t *testing.T) {
	t.Parallel()

	tm, pm := newTestDeps()
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", nil)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt": "do something",
		"model":  "claude-opus-4-6",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Task started")
	assert.Contains(t, text, "claude-opus-4-6")

	tasks := tm.List(task.Filter{})
	require.Len(t, tasks, 1)
	assert.Equal(t, "claude-opus-4-6", tasks[0].Model)
}

func TestStartTask_WhenNoModel_UsesConfigDefault(t *testing.T) {
	t.Parallel()

	tm, pm := newTestDeps()
	handler := StartTask(tm, pm, 30*time.Minute, 2*time.Hour, 102400, "claude-sonnet-4-5-20250929", nil)

	result, err := handler(context.Background(), makeReq(map[string]any{
		"prompt": "do something",
	}))
	require.NoError(t, err)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "claude-sonnet-4-5-20250929")

	tasks := tm.List(task.Filter{})
	require.Len(t, tasks, 1)
	assert.Equal(t, "claude-sonnet-4-5-20250929", tasks[0].Model)
}
