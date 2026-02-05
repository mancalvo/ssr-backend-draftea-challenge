package services

import (
	"context"
	"time"

	"github.com/mancalvo/ssr-backend-draftea-challenge/pkg/logger"
)

const (
	// DefaultCleanupInterval is the default interval between cleanup runs.
	DefaultCleanupInterval = 15 * time.Minute

	// DefaultCleanupBatchSize is the default number of records to delete per batch.
	DefaultCleanupBatchSize = 500
)

// CleanupWorkerConfig holds configuration for the cleanup worker.
type CleanupWorkerConfig struct {
	Interval  time.Duration
	BatchSize int
}

// StartCleanupWorker starts a background goroutine that periodically cleans up expired records.
// It respects context cancellation for graceful shutdown.
func StartCleanupWorker(ctx context.Context, svc Service, cfg CleanupWorkerConfig) {
	if cfg.Interval == 0 {
		cfg.Interval = DefaultCleanupInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultCleanupBatchSize
	}

	go runCleanupLoop(ctx, svc, cfg)
}

func runCleanupLoop(ctx context.Context, svc Service, cfg CleanupWorkerConfig) {
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	logger.Info("idempotency cleanup worker started",
		"interval", cfg.Interval.String(),
		"batch_size", cfg.BatchSize,
	)

	for {
		select {
		case <-ctx.Done():
			logger.Info("idempotency cleanup worker stopped")
			return
		case <-ticker.C:
			runCleanupBatch(ctx, svc, cfg.BatchSize)
		}
	}
}

func runCleanupBatch(ctx context.Context, svc Service, batchSize int) {
	deleted, err := svc.Cleanup(ctx, batchSize)
	if err != nil {
		logger.Error("idempotency cleanup failed", "error", err)
		return
	}

	if deleted > 0 {
		logger.Info("idempotency cleanup completed", "deleted", deleted)
	}
}
