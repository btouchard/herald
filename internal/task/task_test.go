package task

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateID_HasCorrectFormat(t *testing.T) {
	t.Parallel()

	id := GenerateID()
	assert.True(t, strings.HasPrefix(id, "herald-"), "ID should start with herald-")
	assert.Len(t, id, 15, "ID should be 15 chars: herald- + 8 hex")
}

func TestGenerateID_IsUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)
	for range 100 {
		id := GenerateID()
		assert.False(t, seen[id], "duplicate ID generated: %s", id)
		seen[id] = true
	}
}

func TestNew_SetsDefaults(t *testing.T) {
	t.Parallel()

	task := New("my-project", "fix the bug", "", "", 0, 0)

	assert.Equal(t, "my-project", task.Project)
	assert.Equal(t, "fix the bug", task.Prompt)
	assert.Equal(t, StatusPending, task.Status)
	assert.Equal(t, PriorityNormal, task.Priority)
	assert.Equal(t, 30, task.TimeoutMinutes)
	assert.NotNil(t, task.done)
}

func TestNew_RespectsExplicitValues(t *testing.T) {
	t.Parallel()

	task := New("proj", "do it", "", PriorityUrgent, 60, 0)

	assert.Equal(t, PriorityUrgent, task.Priority)
	assert.Equal(t, 60, task.TimeoutMinutes)
}

func TestSetStatus_UpdatesTimestamps(t *testing.T) {
	t.Parallel()

	task := New("p", "x", "", PriorityNormal, 30, 0)

	task.SetStatus(StatusRunning)
	assert.Equal(t, StatusRunning, task.Status)
	assert.False(t, task.StartedAt.IsZero(), "StartedAt should be set")

	task.SetStatus(StatusCompleted)
	assert.Equal(t, StatusCompleted, task.Status)
	assert.False(t, task.CompletedAt.IsZero(), "CompletedAt should be set")
}

func TestSetStatus_ClosesDoneChannel(t *testing.T) {
	t.Parallel()

	task := New("p", "x", "", PriorityNormal, 30, 0)

	select {
	case <-task.Done():
		t.Fatal("done channel should not be closed yet")
	default:
	}

	task.SetStatus(StatusCompleted)

	select {
	case <-task.Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("done channel should be closed after completion")
	}
}

func TestIsTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status   Status
		terminal bool
	}{
		{StatusPending, false},
		{StatusQueued, false},
		{StatusRunning, false},
		{StatusCompleted, true},
		{StatusFailed, true},
		{StatusCancelled, true},
	}

	for _, tt := range tests {
		task := New("p", "x", "", PriorityNormal, 30, 0)
		task.Status = tt.status
		assert.Equal(t, tt.terminal, task.IsTerminal(), "status %s", tt.status)
	}
}

func TestSnapshot_ReturnsCopy(t *testing.T) {
	t.Parallel()

	task := New("proj", "prompt", "", PriorityHigh, 15, 0)
	task.SetStatus(StatusRunning)
	task.SetProgress("working...")
	task.SetCost(0.42)
	task.SetTurns(3)

	snap := task.Snapshot()

	assert.Equal(t, task.ID, snap.ID)
	assert.Equal(t, "proj", snap.Project)
	assert.Equal(t, StatusRunning, snap.Status)
	assert.Equal(t, "working...", snap.Progress)
	assert.InDelta(t, 0.42, snap.CostUSD, 0.001)
	assert.Equal(t, 3, snap.Turns)
}

func TestPriorityWeight(t *testing.T) {
	t.Parallel()

	assert.Greater(t, PriorityUrgent.Weight(), PriorityHigh.Weight())
	assert.Greater(t, PriorityHigh.Weight(), PriorityNormal.Weight())
	assert.Greater(t, PriorityNormal.Weight(), PriorityLow.Weight())
}

func TestSnapshot_FormatDuration(t *testing.T) {
	t.Parallel()

	snap := TaskSnapshot{
		StartedAt:   time.Now().Add(-3*time.Minute - 45*time.Second),
		CompletedAt: time.Now(),
	}

	assert.Equal(t, "3m 45s", snap.FormatDuration())
}

func TestSnapshot_FormatDuration_WhenShort(t *testing.T) {
	t.Parallel()

	snap := TaskSnapshot{
		StartedAt:   time.Now().Add(-12 * time.Second),
		CompletedAt: time.Now(),
	}

	assert.Equal(t, "12s", snap.FormatDuration())
}

func TestAppendOutput_Accumulates(t *testing.T) {
	t.Parallel()

	task := New("p", "x", "", PriorityNormal, 30, 0)
	task.AppendOutput("hello ")
	task.AppendOutput("world")

	snap := task.Snapshot()
	assert.Equal(t, "hello world", snap.Output)
}

func TestAppendOutput_WhenBounded_TruncatesOldContent(t *testing.T) {
	t.Parallel()

	task := New("p", "x", "", PriorityNormal, 30, 10) // 10 bytes max
	task.AppendOutput("12345")
	task.AppendOutput("67890")
	task.AppendOutput("ABCDE")

	snap := task.Snapshot()
	assert.Equal(t, "67890ABCDE", snap.Output) // only last 10 bytes
	assert.Equal(t, 15, task.OutputTotalBytes())
}

func TestSetStatus_DoubleClose_DoesNotPanic(t *testing.T) {
	t.Parallel()

	task := New("p", "x", "", PriorityNormal, 30, 0)
	require.NotPanics(t, func() {
		task.SetStatus(StatusCompleted)
		task.SetStatus(StatusCompleted)
	})
}

func TestNew_WithContext_StoresContext(t *testing.T) {
	t.Parallel()

	task := New("proj", "do task", "fixing auth bug from mobile", PriorityNormal, 30, 0)

	assert.Equal(t, "fixing auth bug from mobile", task.Context)
}

func TestNew_WithEmptyContext_AllowsEmpty(t *testing.T) {
	t.Parallel()

	task := New("proj", "do task", "", PriorityNormal, 30, 0)

	assert.Equal(t, "", task.Context)
}

func TestSnapshot_IncludesContext(t *testing.T) {
	t.Parallel()

	task := New("proj", "prompt", "working on feature X", PriorityHigh, 15, 0)
	task.SetStatus(StatusRunning)

	snap := task.Snapshot()

	assert.Equal(t, "working on feature X", snap.Context)
}
