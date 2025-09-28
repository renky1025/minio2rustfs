package checkpoint

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite
type SQLiteStore struct {
	db      *sql.DB
	closed  bool
	writeMu sync.Mutex
}

// NewSQLiteStore creates a new SQLite checkpoint store
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Configure SQLite for concurrent access
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=2000&_foreign_keys=on&_busy_timeout=60000", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings for concurrent access
	db.SetMaxOpenConns(50)                  // 增加并发连接数
	db.SetMaxIdleConns(10)                  // 增加空闲连接数
	db.SetConnMaxLifetime(10 * time.Minute) // 增加连接生命周期

	store := &SQLiteStore{
		db:     db,
		closed: false,
	}
	if err := store.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		bucket TEXT NOT NULL,
		key TEXT NOT NULL,
		size INTEGER NOT NULL,
		etag TEXT NOT NULL,
		status TEXT NOT NULL,
		attempts INTEGER DEFAULT 0,
		last_error TEXT,
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (bucket, key)
	);
	
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_updated_at ON tasks(updated_at);
	`

	_, err := s.db.Exec(query)
	return err
}

// GetTask retrieves a task record with retry mechanism
func (s *SQLiteStore) GetTask(bucket, key string) (*TaskRecord, error) {
	// Check if store is closed
	if s.closed {
		return nil, fmt.Errorf("database store is closed")
	}

	// Check if database is still open
	if err := s.db.Ping(); err != nil {
		return nil, fmt.Errorf("database connection is not available: %w", err)
	}

	var result *TaskRecord
	err := s.retryOnBusy(func() error {
		var err error
		result, err = s.getTaskInternal(bucket, key)
		return err
	})
	return result, err
}

// getTaskInternal performs the actual get operation
func (s *SQLiteStore) getTaskInternal(bucket, key string) (*TaskRecord, error) {
	query := `
	SELECT bucket, key, size, etag, status, attempts, last_error, updated_at
	FROM tasks WHERE bucket = ? AND key = ?
	`

	row := s.db.QueryRow(query, bucket, key)

	var record TaskRecord
	var lastError sql.NullString

	err := row.Scan(
		&record.Bucket,
		&record.Key,
		&record.Size,
		&record.ETag,
		&record.Status,
		&record.Attempts,
		&lastError,
		&record.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if lastError.Valid {
		record.LastError = lastError.String
	}

	return &record, nil
}

// SaveTask saves or updates a task record with retry mechanism
func (s *SQLiteStore) SaveTask(record *TaskRecord) error {
	// Check if store is closed
	if s.closed {
		return fmt.Errorf("database store is closed")
	}

	// Check if database is still open
	if err := s.db.Ping(); err != nil {
		return fmt.Errorf("database connection is not available: %w", err)
	}

	// Serialize writes to avoid SQLITE_BUSY from multiple concurrent writers
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	return s.retryOnBusy(func() error {
		return s.saveTaskWithTransaction(record)
	})
}

// saveTaskWithTransaction performs the actual save operation in a transaction
func (s *SQLiteStore) saveTaskWithTransaction(record *TaskRecord) error {
	record.UpdatedAt = time.Now()

	// Use a transaction for better concurrency
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // This will be ignored if Commit() succeeds

	// Use UPSERT to avoid DELETE+INSERT of REPLACE which increases lock contention
	query := `
    INSERT INTO tasks 
    (bucket, key, size, etag, status, attempts, last_error, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(bucket, key) DO UPDATE SET
        size = excluded.size,
        etag = excluded.etag,
        status = excluded.status,
        attempts = excluded.attempts,
        last_error = excluded.last_error,
        updated_at = excluded.updated_at
    `

	_, err = tx.Exec(query,
		record.Bucket,
		record.Key,
		record.Size,
		record.ETag,
		record.Status,
		record.Attempts,
		record.LastError,
		record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to execute insert: %w", err)
	}

	return tx.Commit()
}

// retryOnBusy retries the operation if SQLite is busy
func (s *SQLiteStore) retryOnBusy(operation func() error) error {
	maxRetries := 10                   // 增加重试次数
	baseDelay := 50 * time.Millisecond // 增加基本延迟

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		// Check if this is a busy error
		if isSQLiteBusyError(err) {
			if attempt < maxRetries-1 {
				// Wait with exponential backoff + jitter
				delay := baseDelay * time.Duration(1<<uint(attempt))
				// 添加随机抖动来减少竞争
				jitter := time.Duration(attempt*10) * time.Millisecond
				time.Sleep(delay + jitter)
				continue
			}
		}

		// Return the error if it's not a busy error or we've exhausted retries
		return err
	}

	return nil
}

// isSQLiteBusyError checks if the error is a SQLite busy error
func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	errorStr := err.Error()
	return strings.Contains(errorStr, "database is locked") ||
		strings.Contains(errorStr, "SQLITE_BUSY") ||
		strings.Contains(errorStr, "database is closed")
}

// ListPendingTasks returns all pending tasks
func (s *SQLiteStore) ListPendingTasks() ([]*TaskRecord, error) {
	return s.listTasksByStatus(StatusPending)
}

// ListFailedTasks returns all failed tasks
func (s *SQLiteStore) ListFailedTasks() ([]*TaskRecord, error) {
	return s.listTasksByStatus(StatusFailed)
}

func (s *SQLiteStore) listTasksByStatus(status TaskStatus) ([]*TaskRecord, error) {
	query := `
	SELECT bucket, key, size, etag, status, attempts, last_error, updated_at
	FROM tasks WHERE status = ?
	ORDER BY updated_at ASC
	`

	rows, err := s.db.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*TaskRecord

	for rows.Next() {
		var record TaskRecord
		var lastError sql.NullString

		err := rows.Scan(
			&record.Bucket,
			&record.Key,
			&record.Size,
			&record.ETag,
			&record.Status,
			&record.Attempts,
			&lastError,
			&record.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if lastError.Valid {
			record.LastError = lastError.String
		}

		records = append(records, &record)
	}

	return records, rows.Err()
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	s.closed = true
	return s.db.Close()
}
