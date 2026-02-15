package task

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/btouchard/herald/internal/executor"
)

// mockExecutor simulates Claude Code execution for testing.
type mockExecutor struct {
	delay  time.Duration
	output string
	cost   float64
	turns  int
	err    error
}

func (m *mockExecutor) Execute(ctx context.Context, req executor.Request, onProgress executor.ProgressFunc) (*executor.Result, error) {
	if onProgress != nil {
		onProgress("started", fmt.Sprintf("PID %d", 12345))
		onProgress("progress", "Working on it...")
	}

	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if m.err != nil {
		return &executor.Result{
			Output:  m.output,
			CostUSD: m.cost,
		}, m.err
	}

	return &executor.Result{
		SessionID: "ses_test123",
		Output:    m.output,
		CostUSD:   m.cost,
		Turns:     m.turns,
		Duration:  m.delay,
	}, nil
}

func TestManager_Create_ReturnsTask(t *testing.T) {
	t.Parallel()

	m := NewManager(&mockExecutor{}, 3, 2*time.Hour)

	task := m.Create("proj", "do something", "", PriorityNormal, 30)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "proj", task.Project)
	assert.Equal(t, StatusPending, task.Status)
}

func TestManager_Get_ReturnsTask(t *testing.T) {
	t.Parallel()

	m := NewManager(&mockExecutor{}, 3, 2*time.Hour)
	created := m.Create("proj", "do something", "", PriorityNormal, 30)

	found, err := m.Get(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestManager_Get_ReturnsErrorForUnknown(t *testing.T) {
	t.Parallel()

	m := NewManager(&mockExecutor{}, 3, 2*time.Hour)

	_, err := m.Get("herald-nonexist")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_StartAndComplete(t *testing.T) {
	t.Parallel()

	mock := &mockExecutor{
		delay:  50 * time.Millisecond,
		output: "Done fixing the bug.",
		cost:   0.25,
		turns:  3,
	}
	m := NewManager(mock, 3, 2*time.Hour)
	ctx := context.Background()

	task := m.Create("proj", "fix bug", "", PriorityNormal, 30)
	err := m.Start(ctx, task, executor.Request{
		TaskID:      task.ID,
		Prompt:      "fix bug",
		ProjectPath: "/tmp",
	}, 0)
	require.NoError(t, err)

	select {
	case <-task.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("task did not complete in time")
	}

	snap := task.Snapshot()
	assert.Equal(t, StatusCompleted, snap.Status)
	assert.Equal(t, "ses_test123", snap.SessionID)
	assert.InDelta(t, 0.25, snap.CostUSD, 0.001)
	assert.Equal(t, 3, snap.Turns)
	assert.Contains(t, snap.Output, "Done fixing the bug.")
}

func TestManager_StartAndFail(t *testing.T) {
	t.Parallel()

	mock := &mockExecutor{
		delay: 10 * time.Millisecond,
		err:   fmt.Errorf("claude exited with code 1"),
	}
	m := NewManager(mock, 3, 2*time.Hour)
	ctx := context.Background()

	task := m.Create("proj", "bad task", "", PriorityNormal, 30)
	err := m.Start(ctx, task, executor.Request{
		TaskID: task.ID,
		Prompt: "bad task",
	}, 0)
	require.NoError(t, err)

	<-task.Done()

	snap := task.Snapshot()
	assert.Equal(t, StatusFailed, snap.Status)
	assert.Contains(t, snap.Error, "claude exited")
}

func TestManager_Cancel(t *testing.T) {
	t.Parallel()

	mock := &mockExecutor{
		delay: 10 * time.Second,
	}
	m := NewManager(mock, 3, 2*time.Hour)
	ctx := context.Background()

	task := m.Create("proj", "long task", "", PriorityNormal, 30)
	err := m.Start(ctx, task, executor.Request{
		TaskID: task.ID,
		Prompt: "long task",
	}, 0)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	err = m.Cancel(task.ID)
	require.NoError(t, err)

	select {
	case <-task.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled task did not complete in time")
	}

	snap := task.Snapshot()
	assert.Equal(t, StatusCancelled, snap.Status)
}

func TestManager_Cancel_ErrorOnTerminalTask(t *testing.T) {
	t.Parallel()

	mock := &mockExecutor{delay: 10 * time.Millisecond}
	m := NewManager(mock, 3, 2*time.Hour)
	ctx := context.Background()

	task := m.Create("proj", "quick", "", PriorityNormal, 30)
	require.NoError(t, m.Start(ctx, task, executor.Request{TaskID: task.ID, Prompt: "quick"}, 0))
	<-task.Done()

	err := m.Cancel(task.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already")
}

func TestManager_List_FiltersbyStatus(t *testing.T) {
	t.Parallel()

	m := NewManager(&mockExecutor{}, 3, 2*time.Hour)
	t1 := m.Create("proj", "one", "", PriorityNormal, 30)
	t2 := m.Create("proj", "two", "", PriorityNormal, 30)
	t1.SetStatus(StatusCompleted)
	t2.SetStatus(StatusFailed)

	completed := m.List(Filter{Status: "completed"})
	assert.Len(t, completed, 1)
	assert.Equal(t, t1.ID, completed[0].ID)

	all := m.List(Filter{Status: "all"})
	assert.Len(t, all, 2)
}

func TestManager_List_FiltersByProject(t *testing.T) {
	t.Parallel()

	m := NewManager(&mockExecutor{}, 3, 2*time.Hour)
	m.Create("alpha", "task1", "", PriorityNormal, 30)
	m.Create("beta", "task2", "", PriorityNormal, 30)
	m.Create("alpha", "task3", "", PriorityNormal, 30)

	result := m.List(Filter{Project: "alpha"})
	assert.Len(t, result, 2)
}

func TestManager_List_RespectsLimit(t *testing.T) {
	t.Parallel()

	m := NewManager(&mockExecutor{}, 3, 2*time.Hour)
	for range 5 {
		m.Create("proj", "task", "", PriorityNormal, 30)
	}

	result := m.List(Filter{Limit: 3})
	assert.Len(t, result, 3)
}

func TestManager_RunningCount(t *testing.T) {
	t.Parallel()

	mock := &mockExecutor{delay: 5 * time.Second}
	m := NewManager(mock, 3, 2*time.Hour)
	ctx := context.Background()

	assert.Equal(t, 0, m.RunningCount())

	task := m.Create("proj", "long", "", PriorityNormal, 30)
	require.NoError(t, m.Start(ctx, task, executor.Request{TaskID: task.ID, Prompt: "long"}, 0))

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, m.RunningCount())

	_ = m.Cancel(task.ID)
	<-task.Done()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, m.RunningCount())
}

func TestManager_Start_WhenGlobalLimitReached_ReturnsError(t *testing.T) {
	t.Parallel()

	mock := &mockExecutor{delay: 10 * time.Second}
	m := NewManager(mock, 2, 2*time.Hour) // global limit = 2
	ctx := context.Background()

	// Start 2 tasks (at limit)
	t1 := m.Create("proj", "task1", "", PriorityNormal, 30)
	require.NoError(t, m.Start(ctx, t1, executor.Request{TaskID: t1.ID, Prompt: "task1"}, 0))

	t2 := m.Create("proj", "task2", "", PriorityNormal, 30)
	require.NoError(t, m.Start(ctx, t2, executor.Request{TaskID: t2.ID, Prompt: "task2"}, 0))

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 2, m.RunningCount())

	// Third task should be rejected
	t3 := m.Create("proj", "task3", "", PriorityNormal, 30)
	err := m.Start(ctx, t3, executor.Request{TaskID: t3.ID, Prompt: "task3"}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "global concurrency limit reached")
	assert.Contains(t, err.Error(), "2/2")

	// Cleanup
	_ = m.Cancel(t1.ID)
	_ = m.Cancel(t2.ID)
	<-t1.Done()
	<-t2.Done()
}

func TestManager_Start_WhenProjectLimitReached_ReturnsError(t *testing.T) {
	t.Parallel()

	mock := &mockExecutor{delay: 10 * time.Second}
	m := NewManager(mock, 10, 2*time.Hour) // high global limit
	ctx := context.Background()

	// Start 1 task on "alpha" (per-project limit will be 1)
	t1 := m.Create("alpha", "task1", "", PriorityNormal, 30)
	require.NoError(t, m.Start(ctx, t1, executor.Request{TaskID: t1.ID, Prompt: "task1"}, 1))

	time.Sleep(50 * time.Millisecond)

	// Second task on same project should be rejected
	t2 := m.Create("alpha", "task2", "", PriorityNormal, 30)
	err := m.Start(ctx, t2, executor.Request{TaskID: t2.ID, Prompt: "task2"}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project")
	assert.Contains(t, err.Error(), "alpha")

	// Different project should still work
	t3 := m.Create("beta", "task3", "", PriorityNormal, 30)
	require.NoError(t, m.Start(ctx, t3, executor.Request{TaskID: t3.ID, Prompt: "task3"}, 1))

	// Cleanup
	_ = m.Cancel(t1.ID)
	_ = m.Cancel(t3.ID)
	<-t1.Done()
	<-t3.Done()
}

func TestManager_Start_WhenTaskCompletes_FreesSlot(t *testing.T) {
	t.Parallel()

	mock := &mockExecutor{delay: 50 * time.Millisecond}
	m := NewManager(mock, 1, 2*time.Hour) // global limit = 1
	ctx := context.Background()

	// Start and wait for completion
	t1 := m.Create("proj", "task1", "", PriorityNormal, 30)
	require.NoError(t, m.Start(ctx, t1, executor.Request{TaskID: t1.ID, Prompt: "task1"}, 0))
	<-t1.Done()

	assert.Equal(t, 0, m.RunningCount())

	// Should be able to start another task now
	t2 := m.Create("proj", "task2", "", PriorityNormal, 30)
	require.NoError(t, m.Start(ctx, t2, executor.Request{TaskID: t2.ID, Prompt: "task2"}, 0))
	<-t2.Done()

	assert.Equal(t, StatusCompleted, t2.Snapshot().Status)
}
