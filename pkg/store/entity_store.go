package store

import (
	"fmt"

	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/index"
)

// EntityStore is the main entry point for entity storage and retrieval.
type EntityStore struct {
	kv         KVStore
	indexStore IndexStore
	registry   *index.Registry
	serializer Serializer
	keyBuilder *index.KeyBuilder
}

// NewEntityStore creates a new EntityStore.
func NewEntityStore(kv KVStore, indexStore IndexStore, registry *index.Registry, serializer Serializer) *EntityStore {
	return &EntityStore{
		kv:         kv,
		indexStore: indexStore,
		registry:   registry,
		serializer: serializer,
		keyBuilder: index.NewKeyBuilder(),
	}
}

// Write stores an entity and updates all configured indexes.
func (s *EntityStore) Write(e *entity.Entity) error {
	kvKey := fmt.Sprintf("%s:%s", e.Type, e.ID)

	// 1. Check if entity already exists and delete old indexes
	if existing, err := s.kv.Get(kvKey); err == nil {
		var oldEntity entity.Entity
		if err := s.serializer.Unmarshal(existing, &oldEntity); err != nil {
			return fmt.Errorf("deserialize old entity: %w", err)
		}
		if err := s.deleteIndexes(&oldEntity); err != nil {
			return fmt.Errorf("delete old indexes: %w", err)
		}
	} else if err != ErrNotFound {
		return fmt.Errorf("kv get: %w", err)
	}

	// 2. Serialize full entity → KV
	blob, err := s.serializer.Marshal(e)
	if err != nil {
		return fmt.Errorf("serialize entity: %w", err)
	}
	if err := s.kv.Set(kvKey, blob); err != nil {
		return fmt.Errorf("kv set: %w", err)
	}

	// 3. Write the _all index entry for time-series queries
	allKey := s.keyBuilder.BuildAllKey(e.Type, e.Timestamp, e.ID)
	if err := s.indexStore.Set(allKey, nil); err != nil {
		return fmt.Errorf("index set _all: %w", err)
	}

	// 4. Look up index config for this entity type
	cfg := s.registry.Get(e.Type)
	if cfg == nil {
		return nil // No indexes configured, just store in KV
	}

	// 5. Write one index entry per indexed field
	for _, idxField := range cfg.Indexes {
		val, ok := idxField.Path.Extract(e.Fields)
		if !ok {
			continue
		}

		keys := index.BuildIndexKeys(e.Type, idxField.Name, val, e.Timestamp, e.ID)
		for _, key := range keys {
			if err := s.indexStore.Set(key, nil); err != nil {
				return fmt.Errorf("index set %s: %w", idxField.Name, err)
			}
		}
	}

	return nil
}

// Get retrieves an entity by type and ID.
func (s *EntityStore) Get(entityType, entityID string) (*entity.Entity, error) {
	kvKey := fmt.Sprintf("%s:%s", entityType, entityID)
	blob, err := s.kv.Get(kvKey)
	if err != nil {
		return nil, err
	}

	var e entity.Entity
	if err := s.serializer.Unmarshal(blob, &e); err != nil {
		return nil, fmt.Errorf("deserialize entity: %w", err)
	}

	return &e, nil
}

// Delete removes an entity and all its index entries.
func (s *EntityStore) Delete(e *entity.Entity) error {
	kvKey := fmt.Sprintf("%s:%s", e.Type, e.ID)
	if err := s.kv.Delete(kvKey); err != nil && err != ErrNotFound {
		return fmt.Errorf("kv delete: %w", err)
	}
	return s.deleteIndexes(e)
}

// deleteIndexes removes all index entries for an entity.
func (s *EntityStore) deleteIndexes(e *entity.Entity) error {
	// Delete the _all index entry
	allKey := s.keyBuilder.BuildAllKey(e.Type, e.Timestamp, e.ID)
	if err := s.indexStore.Delete(allKey); err != nil {
		return fmt.Errorf("index delete _all: %w", err)
	}

	// Delete field index entries
	cfg := s.registry.Get(e.Type)
	if cfg == nil {
		return nil
	}

	for _, idxField := range cfg.Indexes {
		val, ok := idxField.Path.Extract(e.Fields)
		if !ok {
			continue
		}

		keys := index.BuildIndexKeys(e.Type, idxField.Name, val, e.Timestamp, e.ID)
		for _, key := range keys {
			if err := s.indexStore.Delete(key); err != nil {
				return fmt.Errorf("index delete %s: %w", idxField.Name, err)
			}
		}
	}

	return nil
}

// Close releases resources held by the store.
func (s *EntityStore) Close() error {
	if err := s.kv.Close(); err != nil {
		return err
	}
	return s.indexStore.Close()
}
