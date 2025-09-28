package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"minio2rustfs/internal/checkpoint"
	"minio2rustfs/internal/config"
	"minio2rustfs/internal/metrics"
	"minio2rustfs/internal/progress"
	"minio2rustfs/internal/storage"
	"minio2rustfs/internal/worker"

	"go.uber.org/zap"
)

// Migrator represents the main migration application
type Migrator struct {
	cfg        *config.Config
	logger     *zap.Logger
	srcClient  storage.Client
	dstClient  storage.Client
	checkpoint checkpoint.Store
	metrics    *metrics.Collector
	workers    *worker.Pool
}

// New creates a new migrator instance
func New(cfg *config.Config, logger *zap.Logger) (*Migrator, error) {
	// Create source client
	srcClient, err := storage.NewMinIOClient(storage.Config{
		Endpoint:  cfg.Source.Endpoint,
		AccessKey: cfg.Source.AccessKey,
		SecretKey: cfg.Source.SecretKey,
		Secure:    cfg.Source.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create source client: %w", err)
	}

	// Create destination client
	dstClient, err := storage.NewMinIOClient(storage.Config{
		Endpoint:  cfg.Target.Endpoint,
		AccessKey: cfg.Target.AccessKey,
		SecretKey: cfg.Target.SecretKey,
		Secure:    cfg.Target.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create destination client: %w", err)
	}

	// Create checkpoint store
	checkpointStore, err := checkpoint.NewSQLiteStore(cfg.Migration.Checkpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create checkpoint store: %w", err)
	}

	// Create metrics collector
	metricsCollector := metrics.New()

	// Create worker pool
	workerPool := worker.NewPool(cfg.Migration.Concurrency, worker.Config{
		MultipartThreshold: cfg.Migration.MultipartThreshold,
		PartSize:           cfg.Migration.PartSize,
		Retries:            cfg.Migration.Retries,
		RetryBackoffMs:     cfg.Migration.RetryBackoffMs,
		SkipExisting:       cfg.Migration.SkipExisting,
	}, srcClient, dstClient, checkpointStore, metricsCollector, logger)

	return &Migrator{
		cfg:        cfg,
		logger:     logger,
		srcClient:  srcClient,
		dstClient:  dstClient,
		checkpoint: checkpointStore,
		metrics:    metricsCollector,
		workers:    workerPool,
	}, nil
}

// Run executes the migration process
func (m *Migrator) Run(ctx context.Context) error {
	m.logger.Info("Starting migration",
		zap.String("bucket", m.cfg.Migration.Bucket),
		zap.String("prefix", m.cfg.Migration.Prefix),
		zap.String("object", m.cfg.Migration.Object),
		zap.Int("concurrency", m.cfg.Migration.Concurrency),
		zap.Bool("dry_run", m.cfg.Migration.DryRun),
	)

	// Start metrics server in a goroutine with error handling
	go func() {
		if err := m.metrics.StartServer(":8080"); err != nil {
			m.logger.Error("Failed to start metrics server", zap.Error(err))
		}
	}()

	// Create task channel
	tasks := make(chan worker.Task, m.cfg.Migration.Concurrency*2)

	// Create progress display if enabled and supported and not in dry-run mode
	var progressDisplay *progress.Display
	if m.cfg.Migration.ShowProgress && !m.cfg.Migration.DryRun && progress.IsTerminalSupported() {
		progressTracker := m.metrics.GetProgressTracker()
		progressDisplay = progress.NewDisplay(progressTracker, 2*time.Second) // 增加更新间隔
		m.logger.Info("Progress display enabled")
	} else {
		if m.cfg.Migration.DryRun {
			m.logger.Info("Progress display disabled (dry-run mode)")
		} else if !m.cfg.Migration.ShowProgress {
			m.logger.Info("Progress display disabled (disabled in config)")
		} else {
			m.logger.Info("Progress display disabled (unsupported terminal)")
		}
	}

	// Start worker pool
	var wg sync.WaitGroup
	m.workers.Start(ctx, tasks, &wg)

	// List and enqueue objects
	lister := &ObjectLister{
		client: m.srcClient,
		logger: m.logger,
	}

	// First pass: count objects and total size for progress tracking
	if progressDisplay != nil {
		m.logger.Info("Counting objects for progress tracking...")
		totalObjects, totalBytes, err := lister.CountObjects(ctx, m.cfg.Migration.Bucket, m.cfg.Migration.Prefix, m.cfg.Migration.Object)
		if err != nil {
			m.logger.Warn("Failed to count objects, progress tracking may be inaccurate", zap.Error(err))
		} else {
			m.metrics.SetTotalCounts(totalObjects, totalBytes)
			m.logger.Info("Object counting completed",
				zap.Int64("total_objects", totalObjects),
				zap.String("total_size", progress.FormatBytes(totalBytes)),
			)
			// Start progress display
			progressDisplay.Start()
			// Note: We'll stop it after workers complete
		}
	}

	if err := lister.ListAndEnqueue(ctx, m.cfg.Migration.Bucket, m.cfg.Migration.Prefix, m.cfg.Migration.Object, tasks, m.cfg.Migration.DryRun); err != nil {
		close(tasks)
		return fmt.Errorf("failed to list objects: %w", err)
	}

	close(tasks)
	wg.Wait()

	// Stop progress display if it was started
	if progressDisplay != nil {
		progressDisplay.Stop()
	}

	m.logger.Info("Migration completed")
	return nil
}

// Close cleans up resources
func (m *Migrator) Close() error {
	if m.checkpoint != nil {
		m.checkpoint.Close()
	}
	return nil
}
