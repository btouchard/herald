package task

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kolapsis/herald/internal/executor"
)

// Manager handles task lifecycle: creation, execution, cancellation.
type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*Task

	executor      executor.Executor
	maxConcurrent int
	cancelFuncs   map[string]context.CancelFunc
}

// NewManager creates a new task Manager.
func NewManager(exec executor.Executor, maxConcurrent int) *Manager {
	if maxConcurrent < 1 {
		maxConcurrent = 3
	}
	return &Manager{
		tasks:         make(map[string]*Task),
		executor:      exec,
		maxConcurrent: maxConcurrent,
		cancelFuncs:   make(map[string]context.CancelFunc),
	}
}

// Create makes a new task and stores it.
func (m *Manager) Create(project, prompt string, priority Priority, timeoutMinutes int) *Task {
	t := New(project, prompt, priority, timeoutMinutes)

	m.mu.Lock()
	m.tasks[t.ID] = t
	m.mu.Unlock()

	slog.Info("task created",
		"task_id", t.ID,
		"project", project,
		"priority", string(priority))

	return t
}

// Get returns a task by ID.
func (m *Manager) Get(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %q not found", id)
	}
	return t, nil
}

// List returns tasks matching the given filter.
func (m *Manager) List(filter Filter) []TaskSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []TaskSnapshot
	for _, t := range m.tasks {
		snap := t.Snapshot()

		if filter.Status != "" && filter.Status != "all" && snap.Status != Status(filter.Status) {
			continue
		}
		if filter.Project != "" && snap.Project != filter.Project {
			continue
		}
		if !filter.Since.IsZero() && snap.CreatedAt.Before(filter.Since) {
			continue
		}

		results = append(results, snap)
	}

	// Sort by creation time (newest first)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].CreatedAt.After(results[i].CreatedAt) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results
}

// Filter specifies criteria for listing tasks.
type Filter struct {
	Status  string
	Project string
	Limit   int
	Since   time.Time
}

// Start begins executing a task asynchronously.
// Returns an error if the global or per-project concurrency limit is reached.
// Uses background context so tasks survive after the MCP request completes.
func (m *Manager) Start(_ context.Context, t *Task, req executor.Request, maxPerProject int) error {
	m.mu.RLock()
	globalRunning := 0
	projectRunning := 0
	for _, existing := range m.tasks {
		existing.mu.RLock()
		if existing.Status == StatusRunning {
			globalRunning++
			if existing.Project == t.Project {
				projectRunning++
			}
		}
		existing.mu.RUnlock()
	}
	m.mu.RUnlock()

	if globalRunning >= m.maxConcurrent {
		return fmt.Errorf("global concurrency limit reached (%d/%d)", globalRunning, m.maxConcurrent)
	}
	if maxPerProject > 0 && projectRunning >= maxPerProject {
		return fmt.Errorf("project %q concurrency limit reached (%d/%d)", t.Project, projectRunning, maxPerProject)
	}

	timeout := time.Duration(t.TimeoutMinutes) * time.Minute
	taskCtx, cancel := context.WithTimeout(context.Background(), timeout)

	m.mu.Lock()
	m.cancelFuncs[t.ID] = cancel
	m.mu.Unlock()

	t.SetStatus(StatusRunning)

	go m.run(taskCtx, cancel, t, req)
	return nil
}

func (m *Manager) run(ctx context.Context, cancel context.CancelFunc, t *Task, req executor.Request) {
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("task panicked",
				"task_id", t.ID,
				"panic", r)
			t.SetError(fmt.Sprintf("internal panic: %v", r))
			t.SetStatus(StatusFailed)
		}
	}()

	onProgress := func(eventType, message string) {
		t.SetProgress(message)
		if eventType == "started" {
			var pid int
			_, _ = fmt.Sscanf(message, "PID %d", &pid)
			if pid > 0 {
				t.SetPID(pid)
			}
		}
	}

	result, err := m.executor.Execute(ctx, req, onProgress)

	if result != nil {
		t.SetCost(result.CostUSD)
		t.SetTurns(result.Turns)
		t.SetSessionID(result.SessionID)
		t.AppendOutput(result.Output)
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.SetError("task timed out")
			t.SetStatus(StatusFailed)
			slog.Warn("task timed out", "task_id", t.ID)
			return
		}
		if ctx.Err() == context.Canceled {
			t.SetStatus(StatusCancelled)
			return
		}
		t.SetError(err.Error())
		t.SetStatus(StatusFailed)
		return
	}

	t.SetStatus(StatusCompleted)
}

// Cancel stops a running task.
func (m *Manager) Cancel(id string) error {
	m.mu.RLock()
	t, ok := m.tasks[id]
	cancelFn := m.cancelFuncs[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("task %q not found", id)
	}

	if t.IsTerminal() {
		return fmt.Errorf("task %q is already %s", id, t.Status)
	}

	slog.Info("cancelling task", "task_id", id)

	if cancelFn != nil {
		cancelFn()
	}

	t.mu.RLock()
	pid := t.PID
	t.mu.RUnlock()

	if pid > 0 {
		go executor.GracefulKill(pid)
	}

	t.SetStatus(StatusCancelled)
	return nil
}

// RunningCount returns the number of currently running tasks.
func (m *Manager) RunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, t := range m.tasks {
		t.mu.RLock()
		if t.Status == StatusRunning {
			count++
		}
		t.mu.RUnlock()
	}
	return count
}
