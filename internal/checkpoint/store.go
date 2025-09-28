package checkpoint

import (
	"time"
)

// TaskStatus represents the status of a migration task
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
)

// TaskRecord represents a task record in the checkpoint store
type TaskRecord struct {
	Bucket    string     `json:"bucket"`
	Key       string     `json:"key"`
	Size      int64      `json:"size"`
	ETag      string     `json:"etag"`
	Status    TaskStatus `json:"status"`
	Attempts  int        `json:"attempts"`
	LastError string     `json:"last_error,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Store defines the interface for checkpoint persistence
type Store interface {
	// Task operations
	GetTask(bucket, key string) (*TaskRecord, error)
	SaveTask(record *TaskRecord) error
	ListPendingTasks() ([]*TaskRecord, error)
	ListFailedTasks() ([]*TaskRecord, error)

	// Cleanup
	Close() error
}
