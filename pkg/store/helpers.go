package store

import (
	"fmt"

	"github.com/cbodonnell/chrono/pkg/config"
	"github.com/dgraph-io/badger/v4"
)

func NewEmbeddedStore(cfg *config.Config) (*EntityStore, error) {
	// Build index registry from config
	registry, err := cfg.BuildRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to build registry: %w", err)
	}

	// Open BadgerDB
	opts := badger.DefaultOptions(cfg.Storage.DataDir)
	opts.Logger = nil // Disable badger's verbose logging
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Create storage backends
	kv := NewBadgerKVStore(db)
	idx := NewBadgerIndexStore(db)

	// Create entity store
	es, err := NewEntityStore(kv, idx, registry, NewMsgpackSerializer())
	if err != nil {
		return nil, fmt.Errorf("failed to create entity store: %w", err)
	}

	return es, nil
}

func NewMemoryStore(cfg *config.Config) (*EntityStore, error) {
	// Build index registry from config
	registry, err := cfg.BuildRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to build registry: %w", err)
	}

	// Create storage backends
	kv := NewMemoryKVStore()
	idx := NewMemoryIndexStore()

	// Create entity store
	es, err := NewEntityStore(kv, idx, registry, NewMsgpackSerializer())
	if err != nil {
		return nil, fmt.Errorf("failed to create entity store: %w", err)
	}

	return es, nil
}
