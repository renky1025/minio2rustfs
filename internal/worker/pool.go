package worker

import (
	"context"
	"sync"

	"minio2rustfs/internal/checkpoint"
	"minio2rustfs/internal/metrics"
	"minio2rustfs/internal/storage"

	"go.uber.org/zap"
)

// Pool manages a pool of workers
type Pool struct {
	size       int
	config     Config
	srcClient  storage.Client
	dstClient  storage.Client
	checkpoint checkpoint.Store
	metrics    *metrics.Collector
	logger     *zap.Logger
}

// NewPool creates a new worker pool
func NewPool(
	size int,
	config Config,
	srcClient storage.Client,
	dstClient storage.Client,
	checkpointStore checkpoint.Store,
	metricsCollector *metrics.Collector,
	logger *zap.Logger,
) *Pool {
	return &Pool{
		size:       size,
		config:     config,
		srcClient:  srcClient,
		dstClient:  dstClient,
		checkpoint: checkpointStore,
		metrics:    metricsCollector,
		logger:     logger,
	}
}

// Start starts the worker pool
func (p *Pool) Start(ctx context.Context, tasks <-chan Task, wg *sync.WaitGroup) {
	for i := 0; i < p.size; i++ {
		wg.Add(1)
		go p.worker(ctx, i, tasks, wg)
	}
}

func (p *Pool) worker(ctx context.Context, id int, tasks <-chan Task, wg *sync.WaitGroup) {
	defer wg.Done()

	logger := p.logger.With(zap.Int("worker_id", id))
	logger.Info("Worker started")

	processor := &TaskProcessor{
		config:     p.config,
		srcClient:  p.srcClient,
		dstClient:  p.dstClient,
		checkpoint: p.checkpoint,
		metrics:    p.metrics,
		logger:     logger,
	}

	for {
		select {
		case task, ok := <-tasks:
			if !ok {
				logger.Info("Worker finished - no more tasks")
				return
			}

			processor.Process(ctx, task)

		case <-ctx.Done():
			logger.Info("Worker stopped - context cancelled")
			return
		}
	}
}
