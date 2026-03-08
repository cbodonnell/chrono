package store

import (
	"context"
	"log"
	"time"

	"github.com/cbodonnell/chrono/pkg/index"
)

// RetentionConfig holds configuration for the retention worker.
type RetentionConfig struct {
	Interval   time.Duration // How often to run cleanup (default: 1h)
	BatchSize  int           // Max entities to delete per batch (default: 1000)
	BatchDelay time.Duration // Delay between batches to avoid blocking writes (default: 100ms)
}

// DefaultRetentionConfig returns the default retention configuration.
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		Interval:   time.Hour,
		BatchSize:  1000,
		BatchDelay: 100 * time.Millisecond,
	}
}

// RetentionWorker runs periodic cleanup of expired entities.
type RetentionWorker struct {
	store    *EntityStore
	registry *index.Registry
	config   RetentionConfig
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewRetentionWorker creates a new retention worker.
func NewRetentionWorker(store *EntityStore, registry *index.Registry, config RetentionConfig) *RetentionWorker {
	return &RetentionWorker{
		store:    store,
		registry: registry,
		config:   config,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the background cleanup loop.
func (w *RetentionWorker) Start() {
	go w.run()
}

// Stop gracefully stops the retention worker.
// It waits for the current cleanup cycle to complete or until ctx is cancelled.
func (w *RetentionWorker) Stop(ctx context.Context) error {
	close(w.stopCh)

	select {
	case <-w.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *RetentionWorker) run() {
	defer close(w.doneCh)

	// Run cleanup immediately on startup
	w.runOnce()

	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.runOnce()
		}
	}
}

func (w *RetentionWorker) runOnce() {
	for _, entityType := range w.registry.EntityTypes() {
		// Check for stop signal between entity types
		select {
		case <-w.stopCh:
			return
		default:
		}

		cfg := w.registry.Get(entityType)
		if cfg == nil || cfg.TTL == 0 {
			continue // No TTL configured, skip
		}

		w.cleanupEntityType(entityType, cfg.TTL)
	}
}

func (w *RetentionWorker) cleanupEntityType(entityType string, ttl time.Duration) {
	cutoffNS := time.Now().Add(-ttl).UnixNano()
	totalDeleted := 0

	for {
		// Check for stop signal between batches
		select {
		case <-w.stopCh:
			return
		default:
		}

		deleted, err := w.store.DeleteExpiredBatch(entityType, cutoffNS, w.config.BatchSize)
		if err != nil {
			log.Printf("retention: error cleaning up %s: %v", entityType, err)
			return
		}

		totalDeleted += deleted

		if deleted < w.config.BatchSize {
			// No more entities to delete
			break
		}

		// Sleep between batches to avoid blocking writes
		time.Sleep(w.config.BatchDelay)
	}

	if totalDeleted > 0 {
		log.Printf("retention: deleted %d expired entities of type %s", totalDeleted, entityType)
	}
}

// HasRetention returns true if any entity type has TTL configured.
func HasRetention(registry *index.Registry) bool {
	for _, entityType := range registry.EntityTypes() {
		cfg := registry.Get(entityType)
		if cfg != nil && cfg.TTL > 0 {
			return true
		}
	}
	return false
}
