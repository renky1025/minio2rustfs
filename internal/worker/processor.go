package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"minio2rustfs/internal/checkpoint"
	"minio2rustfs/internal/metrics"
	"minio2rustfs/internal/storage"

	"go.uber.org/zap"
)

// TaskProcessor handles individual task processing
type TaskProcessor struct {
	config     Config
	srcClient  storage.Client
	dstClient  storage.Client
	checkpoint checkpoint.Store
	metrics    *metrics.Collector
	logger     *zap.Logger
}

// Process processes a single migration task
func (p *TaskProcessor) Process(ctx context.Context, task Task) {
	startTime := time.Now()

	// Check if task is already completed
	if record, err := p.checkpoint.GetTask(task.Bucket, task.Key); err == nil && record != nil {
		if record.Status == checkpoint.StatusCompleted && p.config.SkipExisting {
			p.logger.Debug("Skipping completed task", zap.String("key", task.Key))
			p.metrics.IncSkippedWithBytes(task.Size) // Use new method with bytes
			return
		}
	}

	// Check if object exists in destination with same size/etag
	if p.config.SkipExisting && p.objectExistsAndMatches(ctx, task) {
		p.logger.Debug("Skipping existing object", zap.String("key", task.Key))
		p.markCompleted(task)
		p.metrics.IncSkippedWithBytes(task.Size) // Use new method with bytes
		return
	}

	// Process with retry logic
	var lastErr error
	for attempt := 1; attempt <= p.config.Retries; attempt++ {
		err := p.processTask(ctx, task)
		if err == nil {
			// Mark as completed and update metrics
			p.markCompleted(task)
			p.metrics.IncSuccessWithBytes(task.Size) // Use new method with bytes
			p.metrics.AddBytes(task.Size)
			p.metrics.ObserveDuration(time.Since(startTime))
			p.logger.Info("Task completed successfully",
				zap.String("key", task.Key),
				zap.Int64("size", task.Size),
				zap.Duration("duration", time.Since(startTime)),
			)
			return
		}

		lastErr = err
		p.logger.Warn("Task attempt failed",
			zap.String("key", task.Key),
			zap.Int("attempt", attempt),
			zap.Error(err),
		)

		if !p.isRetriableError(err) {
			break
		}

		if attempt < p.config.Retries {
			backoff := p.calculateBackoff(attempt)
			time.Sleep(backoff)
		}
	}

	// Mark as failed
	p.markFailed(task, lastErr)
	p.metrics.IncFailed()
	p.logger.Error("Task failed after all retries",
		zap.String("key", task.Key),
		zap.Error(lastErr),
	)
}

func (p *TaskProcessor) processTask(ctx context.Context, task Task) error {
	// Get source object
	srcObj, err := p.srcClient.GetObject(ctx, task.Bucket, task.Key)
	if err != nil {
		return fmt.Errorf("failed to get source object: %w", err)
	}
	defer srcObj.Close()

	// Choose upload strategy based on size
	if task.Size < p.config.MultipartThreshold {
		return p.uploadSingle(ctx, task, srcObj)
	}

	return p.uploadMultipart(ctx, task, srcObj)
}

func (p *TaskProcessor) uploadSingle(ctx context.Context, task Task, reader io.Reader) error {
	// Use original content-type if available, otherwise fallback to application/octet-stream
	contentType := task.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	opts := storage.PutOptions{
		ContentType: contentType,
		Metadata:    task.Metadata,
	}

	return p.dstClient.PutObject(ctx, task.Bucket, task.Key, reader, task.Size, opts)
}

func (p *TaskProcessor) uploadMultipart(ctx context.Context, task Task, reader io.Reader) error {
	// Use original content-type if available, otherwise fallback to application/octet-stream
	contentType := task.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	opts := storage.PutOptions{
		ContentType: contentType,
		Metadata:    task.Metadata,
	}

	// Initiate multipart upload
	uploadID, err := p.dstClient.NewMultipartUpload(ctx, task.Bucket, task.Key, opts)
	if err != nil {
		return fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	// Calculate number of parts
	partCount := int(math.Ceil(float64(task.Size) / float64(p.config.PartSize)))
	parts := make([]storage.CompletedPart, 0, partCount)

	// Upload parts
	for partNum := 1; partNum <= partCount; partNum++ {
		partSize := p.config.PartSize
		if int64(partNum-1)*p.config.PartSize+partSize > task.Size {
			partSize = task.Size - int64(partNum-1)*p.config.PartSize
		}

		// Read part data
		partData := make([]byte, partSize)
		n, err := io.ReadFull(reader, partData)
		if err != nil && err != io.ErrUnexpectedEOF {
			p.dstClient.AbortMultipartUpload(ctx, task.Bucket, task.Key, uploadID)
			return fmt.Errorf("failed to read part %d: %w", partNum, err)
		}
		partData = partData[:n]

		// Upload part
		etag, err := p.dstClient.UploadPart(ctx, task.Bucket, task.Key, uploadID, partNum,
			bytes.NewReader(partData), int64(len(partData)))
		if err != nil {
			p.dstClient.AbortMultipartUpload(ctx, task.Bucket, task.Key, uploadID)
			return fmt.Errorf("failed to upload part %d: %w", partNum, err)
		}

		parts = append(parts, storage.CompletedPart{
			PartNumber: partNum,
			ETag:       etag,
		})
	}

	// Complete multipart upload
	return p.dstClient.CompleteMultipartUpload(ctx, task.Bucket, task.Key, uploadID, parts)
}

func (p *TaskProcessor) objectExistsAndMatches(ctx context.Context, task Task) bool {
	info, err := p.dstClient.HeadObject(ctx, task.Bucket, task.Key)
	if err != nil {
		return false
	}

	return info.Size == task.Size && info.ETag == task.ETag
}

func (p *TaskProcessor) markCompleted(task Task) {
	record := &checkpoint.TaskRecord{
		Bucket: task.Bucket,
		Key:    task.Key,
		Size:   task.Size,
		ETag:   task.ETag,
		Status: checkpoint.StatusCompleted,
	}

	if err := p.checkpoint.SaveTask(record); err != nil {
		p.logger.Error("Failed to save completed task",
			zap.String("bucket", task.Bucket),
			zap.String("key", task.Key),
			zap.Error(err))
	}
}

func (p *TaskProcessor) markFailed(task Task, err error) {
	record := &checkpoint.TaskRecord{
		Bucket:    task.Bucket,
		Key:       task.Key,
		Size:      task.Size,
		ETag:      task.ETag,
		Status:    checkpoint.StatusFailed,
		LastError: err.Error(),
	}

	if saveErr := p.checkpoint.SaveTask(record); saveErr != nil {
		// Check if this is a database closed error
		if strings.Contains(saveErr.Error(), "database is closed") {
			p.logger.Warn("Cannot save failed task - database is closed",
				zap.String("bucket", task.Bucket),
				zap.String("key", task.Key),
				zap.String("original_error", err.Error()))
		} else {
			p.logger.Error("Failed to save failed task",
				zap.String("bucket", task.Bucket),
				zap.String("key", task.Key),
				zap.Error(saveErr))
		}
	}
}

func (p *TaskProcessor) isRetriableError(err error) bool {
	// More sophisticated error classification
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	// Check for network-related errors
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "temporary") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "dns") ||
		// HTTP 5xx server errors
		strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "internal server error") ||
		strings.Contains(errStr, "bad gateway") ||
		strings.Contains(errStr, "service unavailable") ||
		strings.Contains(errStr, "gateway timeout")
}

func (p *TaskProcessor) calculateBackoff(attempt int) time.Duration {
	base := time.Duration(p.config.RetryBackoffMs) * time.Millisecond
	return base * time.Duration(math.Pow(2, float64(attempt-1)))
}
