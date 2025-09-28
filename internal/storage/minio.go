package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOClient implements the Client interface using minio-go
type MinIOClient struct {
	client *minio.Client
}

// NewMinIOClient creates a new MinIO client
func NewMinIOClient(cfg Config) (*MinIOClient, error) {
	// Clean and validate endpoint
	endpoint, err := cleanEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
	})
	if err != nil {
		return nil, err
	}

	return &MinIOClient{client: client}, nil
}

// cleanEndpoint removes protocol and path from endpoint URL to get host:port format
func cleanEndpoint(endpoint string) (string, error) {
	if endpoint == "" {
		return "", fmt.Errorf("endpoint cannot be empty")
	}

	// If endpoint doesn't have protocol, add http:// for parsing
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		// Check if it's already in host:port format
		if strings.Contains(endpoint, "/") {
			return "", fmt.Errorf("endpoint contains path but no protocol")
		}
		return endpoint, nil
	}

	// Parse URL to extract host and port
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	// Check if path is not empty (indicating a full URL with path)
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		return "", fmt.Errorf("endpoint URL cannot have paths, only host:port is allowed (got path: %s)", parsedURL.Path)
	}

	// Return host:port format
	return parsedURL.Host, nil
}

// GetObject retrieves an object
func (c *MinIOClient) GetObject(ctx context.Context, bucket, key string) (Object, error) {
	obj, err := c.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &minioObject{obj}, nil
}

// PutObject uploads an object
func (c *MinIOClient) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, opts PutOptions) error {
	putOpts := minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		UserMetadata: opts.Metadata,
	}

	_, err := c.client.PutObject(ctx, bucket, key, reader, size, putOpts)
	return err
}

// HeadObject gets object metadata
func (c *MinIOClient) HeadObject(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	info, err := c.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, err
	}

	return ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		ETag:         info.ETag,
		LastModified: info.LastModified,
		ContentType:  info.ContentType, // Add ContentType field
		Metadata:     info.UserMetadata,
	}, nil
}

// ListObjects lists objects with prefix
func (c *MinIOClient) ListObjects(ctx context.Context, bucket, prefix string) (<-chan ObjectInfo, <-chan error) {
	objCh := make(chan ObjectInfo)
	errCh := make(chan error, 1)

	go func() {
		defer close(objCh)
		defer close(errCh)

		for obj := range c.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: true,
		}) {
			if obj.Err != nil {
				errCh <- obj.Err
				return
			}

			select {
			case objCh <- ObjectInfo{
				Key:          obj.Key,
				Size:         obj.Size,
				ETag:         obj.ETag,
				LastModified: obj.LastModified,
				ContentType:  obj.ContentType, // Add ContentType field
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return objCh, errCh
}

// NewMultipartUpload initiates a multipart upload
func (c *MinIOClient) NewMultipartUpload(ctx context.Context, bucket, key string, opts PutOptions) (string, error) {
	putOpts := minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		UserMetadata: opts.Metadata,
	}

	// Use direct core API for multipart uploads
	core := &minio.Core{Client: c.client}
	return core.NewMultipartUpload(ctx, bucket, key, putOpts)
}

// UploadPart uploads a part
func (c *MinIOClient) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	// Use direct core API for multipart uploads
	core := &minio.Core{Client: c.client}
	part, err := core.PutObjectPart(ctx, bucket, key, uploadID, partNumber, reader, size, minio.PutObjectPartOptions{})
	if err != nil {
		return "", err
	}
	return part.ETag, nil
}

// CompleteMultipartUpload completes a multipart upload
func (c *MinIOClient) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) error {
	minioParts := make([]minio.CompletePart, len(parts))
	for i, part := range parts {
		minioParts[i] = minio.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		}
	}

	// Use direct core API for multipart uploads
	core := &minio.Core{Client: c.client}
	_, err := core.CompleteMultipartUpload(ctx, bucket, key, uploadID, minioParts, minio.PutObjectOptions{})
	return err
}

// AbortMultipartUpload aborts a multipart upload
func (c *MinIOClient) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	// Use direct core API for multipart uploads
	core := &minio.Core{Client: c.client}
	return core.AbortMultipartUpload(ctx, bucket, key, uploadID)
}

// minioObject wraps minio.Object to implement our Object interface
type minioObject struct {
	*minio.Object
}

func (o *minioObject) Stat() (ObjectInfo, error) {
	info, err := o.Object.Stat()
	if err != nil {
		return ObjectInfo{}, err
	}

	return ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		ETag:         info.ETag,
		LastModified: info.LastModified,
		ContentType:  info.ContentType, // Add ContentType field
		Metadata:     info.UserMetadata,
	}, nil
}
