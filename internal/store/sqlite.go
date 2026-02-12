package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const timeFormat = time.RFC3339

// SQLiteStore implements Store using modernc.org/sqlite (pure Go, zero CGO).
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database and runs migrations.
// The database file is created with 0600 permissions and its parent directory with 0700.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	// Pre-create the file with restrictive permissions if it doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("creating database file: %w", err)
		}
		_ = f.Close()
	}

	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite handles one writer at a time
	db.SetMaxIdleConns(1)

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

func (s *SQLiteStore) migrate() error {
	// Ensure schema_version table exists
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("creating schema_version table: %w", err)
	}

	var current int
	row := s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}

	for i := current; i < len(migrations); i++ {
		slog.Info("applying migration", "version", i+1)
		if _, err := s.db.Exec(migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := s.db.Exec("INSERT INTO schema_version (version) VALUES (?)", i+1); err != nil {
			return fmt.Errorf("recording migration %d: %w", i+1, err)
		}
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- Tasks ---

func (s *SQLiteStore) CreateTask(t *TaskRecord) error {
	_, err := s.db.Exec(`INSERT INTO tasks (id, project, prompt, status, priority, session_id, pid,
		git_branch, output, progress, error, cost_usd, turns, timeout_minutes, dry_run,
		created_at, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Project, t.Prompt, t.Status, t.Priority, t.SessionID, t.PID,
		t.GitBranch, t.Output, t.Progress, t.Error, t.CostUSD, t.Turns,
		t.TimeoutMinutes, boolToInt(t.DryRun),
		formatTime(t.CreatedAt), formatTime(t.StartedAt), formatTime(t.CompletedAt))
	if err != nil {
		return fmt.Errorf("inserting task: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetTask(id string) (*TaskRecord, error) {
	row := s.db.QueryRow(`SELECT id, project, prompt, status, priority, session_id, pid,
		git_branch, output, progress, error, cost_usd, turns, timeout_minutes, dry_run,
		created_at, started_at, completed_at
		FROM tasks WHERE id = ?`, id)
	return scanTask(row)
}

func (s *SQLiteStore) UpdateTask(t *TaskRecord) error {
	_, err := s.db.Exec(`UPDATE tasks SET
		status = ?, priority = ?, session_id = ?, pid = ?,
		git_branch = ?, output = ?, progress = ?, error = ?,
		cost_usd = ?, turns = ?, timeout_minutes = ?, dry_run = ?,
		started_at = ?, completed_at = ?
		WHERE id = ?`,
		t.Status, t.Priority, t.SessionID, t.PID,
		t.GitBranch, t.Output, t.Progress, t.Error,
		t.CostUSD, t.Turns, t.TimeoutMinutes, boolToInt(t.DryRun),
		formatTime(t.StartedAt), formatTime(t.CompletedAt),
		t.ID)
	if err != nil {
		return fmt.Errorf("updating task: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListTasks(f TaskFilter) ([]TaskRecord, error) {
	query := "SELECT id, project, prompt, status, priority, session_id, pid, git_branch, output, progress, error, cost_usd, turns, timeout_minutes, dry_run, created_at, started_at, completed_at FROM tasks WHERE 1=1"
	var args []interface{}

	if f.Status != "" && f.Status != "all" {
		query += " AND status = ?"
		args = append(args, f.Status)
	}
	if f.Project != "" {
		query += " AND project = ?"
		args = append(args, f.Project)
	}
	if !f.Since.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, formatTime(f.Since))
	}

	query += " ORDER BY created_at DESC"

	if f.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, f.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tasks []TaskRecord
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, rows.Err()
}

// --- Task Events ---

func (s *SQLiteStore) AddEvent(e *TaskEvent) error {
	_, err := s.db.Exec(`INSERT INTO task_events (task_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`,
		e.TaskID, e.EventType, e.Message, formatTime(e.CreatedAt))
	if err != nil {
		return fmt.Errorf("adding event: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetEvents(taskID string, limit int) ([]TaskEvent, error) {
	query := "SELECT id, task_id, event_type, message, created_at FROM task_events WHERE task_id = ? ORDER BY created_at DESC"
	var args []interface{}
	args = append(args, taskID)

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []TaskEvent
	for rows.Next() {
		var e TaskEvent
		var createdAt string
		if err := rows.Scan(&e.ID, &e.TaskID, &e.EventType, &e.Message, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		e.CreatedAt = parseTime(createdAt)
		events = append(events, e)
	}
	return events, rows.Err()
}

// --- OAuth Tokens ---

func (s *SQLiteStore) StoreToken(t *TokenRecord) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO oauth_tokens (token_hash, token_type, client_id, scope, expires_at, revoked, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.TokenHash, t.TokenType, t.ClientID, t.Scope, formatTime(t.ExpiresAt), boolToInt(t.Revoked), formatTime(t.CreatedAt))
	if err != nil {
		return fmt.Errorf("storing token: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetToken(tokenHash string) (*TokenRecord, error) {
	var t TokenRecord
	var expiresAt, createdAt string
	var revoked int

	err := s.db.QueryRow(`SELECT token_hash, token_type, client_id, scope, expires_at, revoked, created_at FROM oauth_tokens WHERE token_hash = ?`, tokenHash).
		Scan(&t.TokenHash, &t.TokenType, &t.ClientID, &t.Scope, &expiresAt, &revoked, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("token not found: %w", err)
	}

	t.ExpiresAt = parseTime(expiresAt)
	t.CreatedAt = parseTime(createdAt)
	t.Revoked = revoked != 0

	if t.Revoked {
		return nil, fmt.Errorf("token revoked")
	}
	if time.Now().After(t.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	return &t, nil
}

func (s *SQLiteStore) RevokeToken(tokenHash string) error {
	_, err := s.db.Exec("UPDATE oauth_tokens SET revoked = 1 WHERE token_hash = ?", tokenHash)
	if err != nil {
		return fmt.Errorf("revoking token: %w", err)
	}
	return nil
}

// --- OAuth Authorization Codes ---

func (s *SQLiteStore) StoreAuthCode(c *AuthCodeRecord) error {
	_, err := s.db.Exec(`INSERT INTO oauth_codes (code_hash, client_id, redirect_uri, code_challenge, scope, expires_at, used) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.CodeHash, c.ClientID, c.RedirectURI, c.CodeChallenge, c.Scope, formatTime(c.ExpiresAt), boolToInt(c.Used))
	if err != nil {
		return fmt.Errorf("storing auth code: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ConsumeAuthCode(codeHash string) (*AuthCodeRecord, error) {
	var c AuthCodeRecord
	var expiresAt, createdAt string
	var used int

	err := s.db.QueryRow(`SELECT code_hash, client_id, redirect_uri, code_challenge, scope, expires_at, used, created_at FROM oauth_codes WHERE code_hash = ?`, codeHash).
		Scan(&c.CodeHash, &c.ClientID, &c.RedirectURI, &c.CodeChallenge, &c.Scope, &expiresAt, &used, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("authorization code not found: %w", err)
	}

	c.ExpiresAt = parseTime(expiresAt)
	c.CreatedAt = parseTime(createdAt)
	c.Used = used != 0

	if c.Used {
		return nil, fmt.Errorf("authorization code already used")
	}
	if time.Now().After(c.ExpiresAt) {
		return nil, fmt.Errorf("authorization code expired")
	}

	// Mark as used
	if _, err := s.db.Exec("UPDATE oauth_codes SET used = 1 WHERE code_hash = ?", codeHash); err != nil {
		return nil, fmt.Errorf("marking code used: %w", err)
	}

	return &c, nil
}

// --- Maintenance ---

func (s *SQLiteStore) Cleanup() error {
	now := formatTime(time.Now())

	if _, err := s.db.Exec("DELETE FROM oauth_codes WHERE expires_at < ? OR used = 1", now); err != nil {
		return fmt.Errorf("cleaning codes: %w", err)
	}
	if _, err := s.db.Exec("DELETE FROM oauth_tokens WHERE expires_at < ? OR revoked = 1", now); err != nil {
		return fmt.Errorf("cleaning tokens: %w", err)
	}

	return nil
}

// --- Helpers ---

func scanTask(row *sql.Row) (*TaskRecord, error) {
	var t TaskRecord
	var dryRun int
	var createdAt, startedAt, completedAt string

	err := row.Scan(&t.ID, &t.Project, &t.Prompt, &t.Status, &t.Priority,
		&t.SessionID, &t.PID, &t.GitBranch, &t.Output, &t.Progress, &t.Error,
		&t.CostUSD, &t.Turns, &t.TimeoutMinutes, &dryRun,
		&createdAt, &startedAt, &completedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning task: %w", err)
	}

	t.DryRun = dryRun != 0
	t.CreatedAt = parseTime(createdAt)
	t.StartedAt = parseTime(startedAt)
	t.CompletedAt = parseTime(completedAt)

	return &t, nil
}

func scanTaskRows(rows *sql.Rows) (*TaskRecord, error) {
	var t TaskRecord
	var dryRun int
	var createdAt, startedAt, completedAt string

	err := rows.Scan(&t.ID, &t.Project, &t.Prompt, &t.Status, &t.Priority,
		&t.SessionID, &t.PID, &t.GitBranch, &t.Output, &t.Progress, &t.Error,
		&t.CostUSD, &t.Turns, &t.TimeoutMinutes, &dryRun,
		&createdAt, &startedAt, &completedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning task: %w", err)
	}

	t.DryRun = dryRun != 0
	t.CreatedAt = parseTime(createdAt)
	t.StartedAt = parseTime(startedAt)
	t.CompletedAt = parseTime(completedAt)

	return &t, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(timeFormat)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(timeFormat, s)
	return t
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
