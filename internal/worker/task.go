package worker

// Task represents a migration task
type Task struct {
	Bucket      string            `json:"bucket"`
	Key         string            `json:"key"`
	Size        int64             `json:"size"`
	ETag        string            `json:"etag"`
	ContentType string            `json:"content_type"` // Add ContentType field
	Metadata    map[string]string `json:"metadata"`
}

// Config contains worker configuration
type Config struct {
	MultipartThreshold int64
	PartSize           int64
	Retries            int
	RetryBackoffMs     int
	SkipExisting       bool
}
