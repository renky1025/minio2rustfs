package storage

import (
	"context"
	"io"
	"time"
)

// Client defines the interface for S3-compatible storage operations
type Client interface {
	// Object operations
	GetObject(ctx context.Context, bucket, key string) (Object, error)
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, opts PutOptions) error
	HeadObject(ctx context.Context, bucket, key string) (ObjectInfo, error)
	ListObjects(ctx context.Context, bucket, prefix string) (<-chan ObjectInfo, <-chan error)

	// Multipart operations
	NewMultipartUpload(ctx context.Context, bucket, key string, opts PutOptions) (string, error)
	UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (string, error)
	CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) error
	AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error
}

// Object represents an object stream
type Object interface {
	io.ReadCloser
	Stat() (ObjectInfo, error)
}

// ObjectInfo contains object metadata
type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	LastModified time.Time
	ContentType  string // Add ContentType field
	Metadata     map[string]string
}

// PutOptions contains options for put operations
type PutOptions struct {
	ContentType string
	Metadata    map[string]string
}

// CompletedPart represents a completed multipart upload part
type CompletedPart struct {
	PartNumber int
	ETag       string
}

// Config contains client configuration
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Secure    bool
}
