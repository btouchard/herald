package task

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// Type distinguishes how a task was created.
type Type string

const (
	TypeDispatched Type = "dispatched" // launched via Chat → Herald → Code
	TypeLinked     Type = "linked"     // pushed from Code → Herald
)

// Status represents the lifecycle state of a task.
type Status string

const (
	StatusPending   Status = "pending"
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusLinked    Status = "linked" // external session registered, not managed by Herald
)

// Priority determines task ordering in the execution queue.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// PriorityWeight returns the numeric weight for queue ordering (higher = first).
func (p Priority) Weight() int {
	switch p {
	case PriorityUrgent:
		return 4
	case PriorityHigh:
		return 3
	case PriorityNormal:
		return 2
	case PriorityLow:
		return 1
	default:
		return 2
	}
}

// Task represents a Claude Code execution unit.
type Task struct {
	mu sync.RWMutex

	ID             string
	Type           Type
	Project        string
	Prompt         string
	Status         Status
	Priority       Priority
	SessionID      string
	MCPSessionID   string // MCP client session for push notifications (runtime-only)
	PID            int
	GitBranch      string

	output        []byte
	maxOutputSize int
	outputTotal   int
	Progress      string
	Error         string

	CostUSD      float64
	Turns        int
	FilesModified []string
	LinesAdded   int
	LinesRemoved int

	TimeoutMinutes int
	DryRun         bool
	AllowedTools   []string

	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time

	done chan struct{}
}

// GenerateID creates a new task ID in the format herald-{8 hex chars}.
func GenerateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("herald-%x", b)
}

// New creates a new Task with the given parameters.
// maxOutputSize limits the in-memory output buffer (0 = unlimited).
func New(project, prompt string, priority Priority, timeoutMinutes, maxOutputSize int) *Task {
	if priority == "" {
		priority = PriorityNormal
	}
	if timeoutMinutes <= 0 {
		timeoutMinutes = 30
	}

	return &Task{
		ID:             GenerateID(),
		Type:           TypeDispatched,
		Project:        project,
		Prompt:         prompt,
		Status:         StatusPending,
		Priority:       priority,
		TimeoutMinutes: timeoutMinutes,
		maxOutputSize:  maxOutputSize,
		CreatedAt:      time.Now(),
		done:           make(chan struct{}),
	}
}

// NewLinked creates a linked task from an external Claude Code session.
// The task starts in StatusLinked and is not meant for execution by Herald.
func NewLinked(sessionID, project, summary, currentTask, gitBranch string, turns int, filesModified []string) *Task {
	t := &Task{
		ID:            GenerateID(),
		Type:          TypeLinked,
		Status:        StatusLinked,
		SessionID:     sessionID,
		Project:       project,
		GitBranch:     gitBranch,
		Progress:      currentTask,
		Turns:         turns,
		FilesModified: filesModified,
		CreatedAt:     time.Now(),
		done:          make(chan struct{}),
	}
	t.output = []byte(summary)
	t.outputTotal = len(summary)
	return t
}

// Done returns a channel that is closed when the task reaches a terminal state.
func (t *Task) Done() <-chan struct{} {
	return t.done
}

// IsTerminal returns true if the task is in a final state.
func (t *Task) IsTerminal() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status == StatusCompleted || t.Status == StatusFailed || t.Status == StatusCancelled || t.Status == StatusLinked
}

// SetStatus updates the task status and timestamps.
func (t *Task) SetStatus(s Status) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Status = s
	switch s {
	case StatusRunning:
		t.StartedAt = time.Now()
	case StatusCompleted, StatusFailed, StatusCancelled:
		t.CompletedAt = time.Now()
		select {
		case <-t.done:
		default:
			close(t.done)
		}
	}
}

// SetOutput replaces the output buffer contents.
func (t *Task) SetOutput(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.output = []byte(text)
	t.outputTotal = len(text)
}

// SetLinkedFields updates mutable fields on a linked task.
func (t *Task) SetLinkedFields(project, gitBranch, progress string, turns int, filesModified []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Project = project
	t.GitBranch = gitBranch
	t.Progress = progress
	t.Turns = turns
	t.FilesModified = filesModified
}

// SetProgress updates the last progress message.
func (t *Task) SetProgress(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Progress = msg
}

// SetError records an error message.
func (t *Task) SetError(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Error = msg
}

// SetSessionID stores the Claude Code session ID.
func (t *Task) SetSessionID(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.SessionID = id
}

// SetPID stores the process ID.
func (t *Task) SetPID(pid int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.PID = pid
}

// SetCost updates the accumulated cost.
func (t *Task) SetCost(usd float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.CostUSD = usd
}

// SetTurns updates the turn count.
func (t *Task) SetTurns(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Turns = n
}

// AppendOutput appends text to the bounded output buffer.
// When maxOutputSize > 0, only the last maxOutputSize bytes are kept in memory.
func (t *Task) AppendOutput(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	data := []byte(text)
	t.outputTotal += len(data)

	t.output = append(t.output, data...)

	if t.maxOutputSize > 0 && len(t.output) > t.maxOutputSize {
		excess := len(t.output) - t.maxOutputSize
		t.output = t.output[excess:]
	}
}

// Output returns the current output buffer contents.
func (t *Task) Output() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return string(t.output)
}

// OutputTotalBytes returns the total bytes written (before truncation).
func (t *Task) OutputTotalBytes() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.outputTotal
}

// Snapshot returns a read-consistent copy of key fields.
func (t *Task) Snapshot() TaskSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return TaskSnapshot{
		ID:             t.ID,
		Type:           t.Type,
		Project:        t.Project,
		Prompt:         t.Prompt,
		Status:         t.Status,
		Priority:       t.Priority,
		SessionID:      t.SessionID,
		MCPSessionID:   t.MCPSessionID,
		GitBranch:      t.GitBranch,
		Output:         string(t.output),
		Progress:       t.Progress,
		Error:          t.Error,
		CostUSD:        t.CostUSD,
		Turns:          t.Turns,
		FilesModified:  t.FilesModified,
		LinesAdded:     t.LinesAdded,
		LinesRemoved:   t.LinesRemoved,
		TimeoutMinutes: t.TimeoutMinutes,
		DryRun:         t.DryRun,
		CreatedAt:      t.CreatedAt,
		StartedAt:      t.StartedAt,
		CompletedAt:    t.CompletedAt,
	}
}

// TaskSnapshot is a read-only copy of a Task's state at a point in time.
type TaskSnapshot struct {
	ID             string
	Type           Type
	Project        string
	Prompt         string
	Status         Status
	Priority       Priority
	SessionID      string
	MCPSessionID   string
	GitBranch      string
	Output         string
	Progress       string
	Error          string
	CostUSD        float64
	Turns          int
	FilesModified  []string
	LinesAdded     int
	LinesRemoved   int
	TimeoutMinutes int
	DryRun         bool
	CreatedAt      time.Time
	StartedAt      time.Time
	CompletedAt    time.Time
}

// Duration returns the elapsed time from start to completion (or now if still running).
func (s TaskSnapshot) Duration() time.Duration {
	if s.StartedAt.IsZero() {
		return 0
	}
	end := s.CompletedAt
	if end.IsZero() {
		end = time.Now()
	}
	return end.Sub(s.StartedAt)
}

// FormatDuration returns a human-readable duration string.
func (s TaskSnapshot) FormatDuration() string {
	d := s.Duration()
	if d < time.Second {
		return "< 1s"
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
