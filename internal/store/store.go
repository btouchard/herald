package store

import (
	"time"
)

// Store is the persistence interface for Herald.
// Defined at the consumer side per Go conventions.
type Store interface {
	// Tasks
	CreateTask(t *TaskRecord) error
	GetTask(id string) (*TaskRecord, error)
	UpdateTask(t *TaskRecord) error
	ListTasks(f TaskFilter) ([]TaskRecord, error)
	GetLinkedTaskBySessionID(sessionID string) (*TaskRecord, error)

	// Task events
	AddEvent(e *TaskEvent) error
	GetEvents(taskID string, limit int) ([]TaskEvent, error)

	// OAuth tokens
	StoreToken(t *TokenRecord) error
	GetToken(tokenHash string) (*TokenRecord, error)
	RevokeToken(tokenHash string) error

	// OAuth authorization codes
	StoreAuthCode(c *AuthCodeRecord) error
	ConsumeAuthCode(codeHash string) (*AuthCodeRecord, error)

	// Analytics
	GetAverageTaskDuration(project string) (time.Duration, int, error)

	// Maintenance
	Cleanup() error
	Close() error
}

// TaskRecord represents a persisted task.
type TaskRecord struct {
	ID             string
	Type           string
	Project        string
	Prompt         string
	Status         string
	Priority       string
	SessionID      string
	PID            int
	GitBranch      string
	Output         string
	Progress       string
	Error          string
	CostUSD        float64
	Turns          int
	TimeoutMinutes int
	DryRun         bool
	CreatedAt      time.Time
	StartedAt      time.Time
	CompletedAt    time.Time
}

// TaskFilter specifies criteria for listing tasks.
type TaskFilter struct {
	Status  string
	Project string
	Limit   int
	Since   time.Time
}

// TaskEvent represents a timestamped event for audit trail.
type TaskEvent struct {
	ID        int64
	TaskID    string
	EventType string
	Message   string
	CreatedAt time.Time
}

// TokenRecord represents a persisted OAuth token.
type TokenRecord struct {
	TokenHash string
	TokenType string
	ClientID  string
	Scope     string
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}

// AuthCodeRecord represents a persisted authorization code.
type AuthCodeRecord struct {
	CodeHash      string
	ClientID      string
	RedirectURI   string
	CodeChallenge string
	Scope         string
	ExpiresAt     time.Time
	Used          bool
	CreatedAt     time.Time
}
