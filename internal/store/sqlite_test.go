package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSQLiteStore_Migration_CreatesTablesAndVersion(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	var version int
	err := s.db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, len(migrations), version)
}

func TestSQLiteStore_CreateAndGetTask(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	task := &TaskRecord{
		ID:             "herald-abc12345",
		Project:        "my-api",
		Prompt:         "fix the bug",
		Status:         "pending",
		Priority:       "normal",
		TimeoutMinutes: 30,
		CreatedAt:      now,
	}

	require.NoError(t, s.CreateTask(task))

	got, err := s.GetTask("herald-abc12345")
	require.NoError(t, err)
	assert.Equal(t, "herald-abc12345", got.ID)
	assert.Equal(t, "my-api", got.Project)
	assert.Equal(t, "fix the bug", got.Prompt)
	assert.Equal(t, "pending", got.Status)
	assert.Equal(t, "normal", got.Priority)
	assert.Equal(t, 30, got.TimeoutMinutes)
}

func TestSQLiteStore_UpdateTask(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	task := &TaskRecord{
		ID:        "herald-upd00001",
		Project:   "proj",
		Prompt:    "do stuff",
		Status:    "pending",
		Priority:  "normal",
		CreatedAt: now,
	}
	require.NoError(t, s.CreateTask(task))

	task.Status = "running"
	task.SessionID = "ses_abc"
	task.PID = 12345
	task.StartedAt = now.Add(time.Second)
	require.NoError(t, s.UpdateTask(task))

	got, err := s.GetTask("herald-upd00001")
	require.NoError(t, err)
	assert.Equal(t, "running", got.Status)
	assert.Equal(t, "ses_abc", got.SessionID)
	assert.Equal(t, 12345, got.PID)
}

func TestSQLiteStore_ListTasks_FilterByStatus(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	require.NoError(t, s.CreateTask(&TaskRecord{ID: "herald-s001", Project: "p", Prompt: "a", Status: "completed", Priority: "normal", CreatedAt: now}))
	require.NoError(t, s.CreateTask(&TaskRecord{ID: "herald-s002", Project: "p", Prompt: "b", Status: "failed", Priority: "normal", CreatedAt: now.Add(time.Second)}))
	require.NoError(t, s.CreateTask(&TaskRecord{ID: "herald-s003", Project: "p", Prompt: "c", Status: "completed", Priority: "normal", CreatedAt: now.Add(2 * time.Second)}))

	completed, err := s.ListTasks(TaskFilter{Status: "completed"})
	require.NoError(t, err)
	assert.Len(t, completed, 2)

	all, err := s.ListTasks(TaskFilter{Status: "all"})
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestSQLiteStore_ListTasks_FilterByProject(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	require.NoError(t, s.CreateTask(&TaskRecord{ID: "herald-p001", Project: "alpha", Prompt: "a", Status: "pending", Priority: "normal", CreatedAt: now}))
	require.NoError(t, s.CreateTask(&TaskRecord{ID: "herald-p002", Project: "beta", Prompt: "b", Status: "pending", Priority: "normal", CreatedAt: now}))

	alpha, err := s.ListTasks(TaskFilter{Project: "alpha"})
	require.NoError(t, err)
	assert.Len(t, alpha, 1)
	assert.Equal(t, "herald-p001", alpha[0].ID)
}

func TestSQLiteStore_ListTasks_Limit(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	for i := range 5 {
		require.NoError(t, s.CreateTask(&TaskRecord{
			ID: fmt.Sprintf("herald-l%03d", i), Project: "p", Prompt: "x", Status: "pending", Priority: "normal",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}))
	}

	limited, err := s.ListTasks(TaskFilter{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, limited, 3)
}

func TestSQLiteStore_ListTasks_OrderByCreatedAtDesc(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	require.NoError(t, s.CreateTask(&TaskRecord{ID: "herald-old", Project: "p", Prompt: "old", Status: "pending", Priority: "normal", CreatedAt: now}))
	require.NoError(t, s.CreateTask(&TaskRecord{ID: "herald-new", Project: "p", Prompt: "new", Status: "pending", Priority: "normal", CreatedAt: now.Add(time.Minute)}))

	tasks, err := s.ListTasks(TaskFilter{})
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	assert.Equal(t, "herald-new", tasks[0].ID, "newest task should be first")
}

func TestSQLiteStore_AddAndGetEvents(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	require.NoError(t, s.CreateTask(&TaskRecord{ID: "herald-ev001", Project: "p", Prompt: "x", Status: "running", Priority: "normal", CreatedAt: now}))

	require.NoError(t, s.AddEvent(&TaskEvent{TaskID: "herald-ev001", EventType: "started", Message: "PID 12345", CreatedAt: now}))
	require.NoError(t, s.AddEvent(&TaskEvent{TaskID: "herald-ev001", EventType: "progress", Message: "Working...", CreatedAt: now.Add(time.Second)}))
	require.NoError(t, s.AddEvent(&TaskEvent{TaskID: "herald-ev001", EventType: "completed", Message: "Done", CreatedAt: now.Add(2 * time.Second)}))

	events, err := s.GetEvents("herald-ev001", 0)
	require.NoError(t, err)
	assert.Len(t, events, 3)
	assert.Equal(t, "completed", events[0].EventType, "newest event should be first")

	limited, err := s.GetEvents("herald-ev001", 2)
	require.NoError(t, err)
	assert.Len(t, limited, 2)
}

func TestSQLiteStore_StoreAndGetToken(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	token := &TokenRecord{
		TokenHash: "hash-abc123",
		TokenType: "access",
		ClientID:  "test-client",
		Scope:     "tasks",
		ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Second),
		CreatedAt: time.Now().Truncate(time.Second),
	}

	require.NoError(t, s.StoreToken(token))

	got, err := s.GetToken("hash-abc123")
	require.NoError(t, err)
	assert.Equal(t, "access", got.TokenType)
	assert.Equal(t, "test-client", got.ClientID)
	assert.Equal(t, "tasks", got.Scope)
}

func TestSQLiteStore_GetToken_RejectsRevoked(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	token := &TokenRecord{
		TokenHash: "hash-revoke",
		TokenType: "refresh",
		ClientID:  "c",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	require.NoError(t, s.StoreToken(token))
	require.NoError(t, s.RevokeToken("hash-revoke"))

	_, err := s.GetToken("hash-revoke")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestSQLiteStore_GetToken_RejectsExpired(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	token := &TokenRecord{
		TokenHash: "hash-expired",
		TokenType: "access",
		ClientID:  "c",
		ExpiresAt: time.Now().Add(-time.Hour),
		CreatedAt: time.Now(),
	}
	require.NoError(t, s.StoreToken(token))

	_, err := s.GetToken("hash-expired")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestSQLiteStore_StoreAndConsumeAuthCode(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	code := &AuthCodeRecord{
		CodeHash:      "code-hash-123",
		ClientID:      "test-client",
		RedirectURI:   "https://callback.test/cb",
		CodeChallenge: "challenge-value",
		Scope:         "tasks",
		ExpiresAt:     time.Now().Add(10 * time.Minute).Truncate(time.Second),
	}
	require.NoError(t, s.StoreAuthCode(code))

	got, err := s.ConsumeAuthCode("code-hash-123")
	require.NoError(t, err)
	assert.Equal(t, "test-client", got.ClientID)
	assert.Equal(t, "challenge-value", got.CodeChallenge)

	// Second consume should fail
	_, err = s.ConsumeAuthCode("code-hash-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already used")
}

func TestSQLiteStore_ConsumeAuthCode_RejectsExpired(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	code := &AuthCodeRecord{
		CodeHash:  "code-expired",
		ClientID:  "c",
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	require.NoError(t, s.StoreAuthCode(code))

	_, err := s.ConsumeAuthCode("code-expired")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestSQLiteStore_Cleanup_RemovesExpiredAndRevoked(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	// Store expired token
	require.NoError(t, s.StoreToken(&TokenRecord{
		TokenHash: "hash-old",
		TokenType: "access",
		ClientID:  "c",
		ExpiresAt: time.Now().Add(-time.Hour),
		CreatedAt: time.Now(),
	}))

	// Store valid token
	require.NoError(t, s.StoreToken(&TokenRecord{
		TokenHash: "hash-valid",
		TokenType: "access",
		ClientID:  "c",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}))

	// Store used code
	require.NoError(t, s.StoreAuthCode(&AuthCodeRecord{
		CodeHash:  "code-used",
		ClientID:  "c",
		ExpiresAt: time.Now().Add(time.Hour),
		Used:      false,
	}))
	_, err := s.ConsumeAuthCode("code-used")
	require.NoError(t, err)

	require.NoError(t, s.Cleanup())

	// Expired token should be gone
	_, err = s.GetToken("hash-old")
	require.Error(t, err)

	// Valid token should remain
	_, err = s.GetToken("hash-valid")
	require.NoError(t, err)
}

func TestSQLiteStore_GetTask_NotFound(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	_, err := s.GetTask("herald-nonexist")
	require.Error(t, err)
}

func TestSQLiteStore_GetAverageTaskDuration_WhenNoHistory_ReturnsZero(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	avg, count, err := s.GetAverageTaskDuration("my-project")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), avg)
	assert.Equal(t, 0, count)
}

func TestNewSQLiteStore_SetsFilePermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "test.db")

	s, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// Check directory permissions
	dirInfo, err := os.Stat(filepath.Join(dir, "subdir"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm(), "directory should be 0700")

	// Check file permissions
	fileInfo, err := os.Stat(dbPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fileInfo.Mode().Perm(), "database file should be 0600")
}

func TestNewSQLiteStore_FixesLoosePermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "loose.db")

	// Create file with overly permissive mode
	require.NoError(t, os.WriteFile(dbPath, nil, 0600))

	s, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	info, err := os.Stat(dbPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "permissions should be tightened to 0600")
}

func TestSQLiteStore_GetAverageTaskDuration_WhenHistory_ReturnsAverage(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)

	// Task 1: 60s duration
	require.NoError(t, s.CreateTask(&TaskRecord{
		ID: "herald-d001", Project: "proj", Prompt: "a", Status: "completed", Priority: "normal",
		CreatedAt: now, StartedAt: now, CompletedAt: now.Add(60 * time.Second),
	}))

	// Task 2: 120s duration
	require.NoError(t, s.CreateTask(&TaskRecord{
		ID: "herald-d002", Project: "proj", Prompt: "b", Status: "completed", Priority: "normal",
		CreatedAt: now.Add(time.Minute), StartedAt: now.Add(time.Minute), CompletedAt: now.Add(time.Minute + 120*time.Second),
	}))

	// Task 3: different project (should not be counted)
	require.NoError(t, s.CreateTask(&TaskRecord{
		ID: "herald-d003", Project: "other", Prompt: "c", Status: "completed", Priority: "normal",
		CreatedAt: now, StartedAt: now, CompletedAt: now.Add(300 * time.Second),
	}))

	// Task 4: failed task (should not be counted)
	require.NoError(t, s.CreateTask(&TaskRecord{
		ID: "herald-d004", Project: "proj", Prompt: "d", Status: "failed", Priority: "normal",
		CreatedAt: now, StartedAt: now, CompletedAt: now.Add(10 * time.Second),
	}))

	avg, count, err := s.GetAverageTaskDuration("proj")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	// Average of 60s and 120s = 90s
	assert.InDelta(t, 90*time.Second, avg, float64(2*time.Second), "average should be ~90s")
}

func TestSQLiteStore_Context_PersistsAcrossRoundtrip(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	task := &TaskRecord{
		ID:             "herald-ctx00001",
		Project:        "my-api",
		Prompt:         "fix auth bug",
		Context:        "fixing login issue from mobile app",
		Status:         "pending",
		Priority:       "high",
		TimeoutMinutes: 30,
		CreatedAt:      now,
	}

	require.NoError(t, s.CreateTask(task))

	got, err := s.GetTask("herald-ctx00001")
	require.NoError(t, err)
	assert.Equal(t, "fixing login issue from mobile app", got.Context)
}

func TestSQLiteStore_Context_AllowsEmptyContext(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	task := &TaskRecord{
		ID:             "herald-ctx00002",
		Project:        "proj",
		Prompt:         "do task",
		Context:        "",
		Status:         "pending",
		Priority:       "normal",
		TimeoutMinutes: 30,
		CreatedAt:      now,
	}

	require.NoError(t, s.CreateTask(task))

	got, err := s.GetTask("herald-ctx00002")
	require.NoError(t, err)
	assert.Equal(t, "", got.Context)
}

func TestSQLiteStore_ListTasks_IncludesContext(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	require.NoError(t, s.CreateTask(&TaskRecord{
		ID:        "herald-ctx00003",
		Project:   "p",
		Prompt:    "a",
		Context:   "working on feature X",
		Status:    "completed",
		Priority:  "normal",
		CreatedAt: now,
	}))

	tasks, err := s.ListTasks(TaskFilter{})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, "working on feature X", tasks[0].Context)
}
