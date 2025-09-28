package app

import (
	"context"
	"fmt"

	"minio2rustfs/internal/storage"
	"minio2rustfs/internal/worker"

	"go.uber.org/zap"
)

// ObjectLister handles listing objects for migration
type ObjectLister struct {
	client storage.Client
	logger *zap.Logger
}

// ListAndEnqueue lists objects and enqueues them as tasks
func (l *ObjectLister) ListAndEnqueue(ctx context.Context, bucket, prefix, objectKey string, tasks chan<- worker.Task, dryRun bool) error {
	if objectKey != "" {
		// Single object mode
		return l.enqueueSingleObject(ctx, bucket, objectKey, tasks, dryRun)
	}

	// List objects with prefix
	return l.enqueueObjects(ctx, bucket, prefix, tasks, dryRun)
}

// CountObjects counts the total number of objects and bytes
func (l *ObjectLister) CountObjects(ctx context.Context, bucket, prefix, objectKey string) (int64, int64, error) {
	if objectKey != "" {
		// Single object mode
		info, err := l.client.HeadObject(ctx, bucket, objectKey)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get object info for %s: %w", objectKey, err)
		}
		return 1, info.Size, nil
	}

	// Count objects with prefix
	return l.countObjects(ctx, bucket, prefix)
}

func (l *ObjectLister) countObjects(ctx context.Context, bucket, prefix string) (int64, int64, error) {
	objCh, errCh := l.client.ListObjects(ctx, bucket, prefix)

	var totalObjects int64
	var totalSize int64

	for {
		select {
		case obj, ok := <-objCh:
			if !ok {
				return totalObjects, totalSize, nil
			}

			totalObjects++
			totalSize += obj.Size

		case err := <-errCh:
			if err != nil {
				return totalObjects, totalSize, fmt.Errorf("error counting objects: %w", err)
			}

		case <-ctx.Done():
			return totalObjects, totalSize, ctx.Err()
		}
	}
}

func (l *ObjectLister) enqueueSingleObject(ctx context.Context, bucket, key string, tasks chan<- worker.Task, dryRun bool) error {
	info, err := l.client.HeadObject(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("failed to get object info for %s: %w", key, err)
	}

	task := worker.Task{
		Bucket:      bucket,
		Key:         key,
		Size:        info.Size,
		ETag:        info.ETag,
		ContentType: info.ContentType, // Add ContentType field
		Metadata:    info.Metadata,
	}

	if dryRun {
		l.logger.Info("Would migrate object",
			zap.String("bucket", bucket),
			zap.String("key", key),
			zap.Int64("size", info.Size),
		)
		return nil
	}

	select {
	case tasks <- task:
		l.logger.Debug("Enqueued object", zap.String("key", key))
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (l *ObjectLister) enqueueObjects(ctx context.Context, bucket, prefix string, tasks chan<- worker.Task, dryRun bool) error {
	objCh, errCh := l.client.ListObjects(ctx, bucket, prefix)

	var totalObjects int64
	var totalSize int64

	for {
		select {
		case obj, ok := <-objCh:
			if !ok {
				l.logger.Info("Finished listing objects",
					zap.Int64("total_objects", totalObjects),
					zap.Int64("total_size_bytes", totalSize),
				)
				return nil
			}

			totalObjects++
			totalSize += obj.Size

			task := worker.Task{
				Bucket:      bucket,
				Key:         obj.Key,
				Size:        obj.Size,
				ETag:        obj.ETag,
				ContentType: obj.ContentType, // Add ContentType field
				Metadata:    obj.Metadata,
			}

			if dryRun {
				l.logger.Info("Would migrate object",
					zap.String("bucket", bucket),
					zap.String("key", obj.Key),
					zap.Int64("size", obj.Size),
				)
				continue
			}

			select {
			case tasks <- task:
				l.logger.Debug("Enqueued object", zap.String("key", obj.Key))
			case <-ctx.Done():
				return ctx.Err()
			}

		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("error listing objects: %w", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
