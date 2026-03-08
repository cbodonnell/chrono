package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

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

const configHashKeyPrefix = "_meta:index_config:"

// SyncIndexes checks index configurations and rebuilds indexes if configurations have changed.
// This should be called after creating the EntityStore and before using it.
// Returns an error if reindexing fails.
func (s *EntityStore) SyncIndexes() error {
	for _, entityType := range s.registry.EntityTypes() {
		cfg := s.registry.Get(entityType)
		if cfg == nil {
			continue
		}

		currentHash := s.hashConfig(cfg)
		storedHash, err := s.getStoredConfigHash(entityType)
		if err != nil && err != ErrNotFound {
			return fmt.Errorf("get stored config hash for %s: %w", entityType, err)
		}

		if currentHash != storedHash {
			if err := s.Reindex(entityType); err != nil {
				return fmt.Errorf("reindex %s: %w", entityType, err)
			}
			if err := s.storeConfigHash(entityType, currentHash); err != nil {
				return fmt.Errorf("store config hash for %s: %w", entityType, err)
			}
		}
	}
	return nil
}

// hashConfig computes a deterministic hash of an EntityTypeConfig.
func (s *EntityStore) hashConfig(cfg *index.EntityTypeConfig) string {
	h := sha256.New()

	// Sort indexes by name for deterministic ordering
	indexes := make([]index.FieldIndex, len(cfg.Indexes))
	copy(indexes, cfg.Indexes)
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].Name < indexes[j].Name
	})

	for _, idx := range indexes {
		h.Write([]byte(idx.Name))
		h.Write([]byte{byte(idx.Type)})
	}

	h.Write([]byte(cfg.TTL.String()))

	return hex.EncodeToString(h.Sum(nil))
}

// getStoredConfigHash retrieves the stored config hash for an entity type.
func (s *EntityStore) getStoredConfigHash(entityType string) (string, error) {
	key := configHashKeyPrefix + entityType
	val, err := s.kv.Get(key)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// storeConfigHash stores the config hash for an entity type.
func (s *EntityStore) storeConfigHash(entityType string, hash string) error {
	key := configHashKeyPrefix + entityType
	return s.kv.Set(key, []byte(hash))
}

// Reindex rebuilds all indexes for a given entity type.
// This clears existing indexes and re-indexes all entities from the KV store.
func (s *EntityStore) Reindex(entityType string) error {
	// 1. Delete all existing indexes for this entity type
	prefix := []byte(entityType + "/")
	var keysToDelete [][]byte
	if err := s.indexStore.ScanPrefix(prefix, func(key []byte) bool {
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		keysToDelete = append(keysToDelete, keyCopy)
		return true
	}); err != nil {
		return fmt.Errorf("scan indexes for deletion: %w", err)
	}

	for _, key := range keysToDelete {
		if err := s.indexStore.Delete(key); err != nil {
			return fmt.Errorf("delete index key: %w", err)
		}
	}

	// 2. Scan KV store for all entities of this type and re-index them
	kvPrefix := entityType + ":"
	if err := s.kv.ScanPrefix(kvPrefix, func(key string, value []byte) bool {
		var e entity.Entity
		if err := s.serializer.Unmarshal(value, &e); err != nil {
			return true // Skip malformed entities
		}

		// Write the _all index entry
		allKey := s.keyBuilder.BuildAllKey(e.Type, e.Timestamp, e.ID)
		if err := s.indexStore.Set(allKey, nil); err != nil {
			return true // Continue on error
		}

		// Write field indexes based on current config
		cfg := s.registry.Get(e.Type)
		if cfg == nil {
			return true
		}

		for _, idxField := range cfg.Indexes {
			val, ok := idxField.Path.Extract(e.Fields)
			if !ok {
				continue
			}

			keys := index.BuildIndexKeys(e.Type, idxField.Name, val, e.Timestamp, e.ID)
			for _, k := range keys {
				s.indexStore.Set(k, nil)
			}
		}

		return true
	}); err != nil {
		return fmt.Errorf("scan entities for reindex: %w", err)
	}

	return nil
}

// DeleteExpiredBatch deletes up to limit expired entities (timestamps before cutoffNS).
// Returns the number of entities deleted.
func (s *EntityStore) DeleteExpiredBatch(entityType string, cutoffNS int64, limit int) (int, error) {
	// Build range for _all index: from beginning of time to cutoff
	startKey := s.keyBuilder.BuildAllRangeStart(entityType, 0)
	endKey := s.keyBuilder.BuildAllRangeEnd(entityType, cutoffNS)

	// Collect entity IDs to delete (up to limit)
	var entityIDs []string
	err := s.indexStore.Scan(startKey, endKey, func(key []byte) bool {
		_, _, entityID := index.ParseAllIndexKey(key)
		if entityID != "" {
			entityIDs = append(entityIDs, entityID)
		}
		return len(entityIDs) < limit
	})
	if err != nil {
		return 0, fmt.Errorf("scan expired entities: %w", err)
	}

	// Delete each entity (this handles KV + all index cleanup)
	deleted := 0
	for _, entityID := range entityIDs {
		e, err := s.Get(entityType, entityID)
		if err != nil {
			if err == ErrNotFound {
				continue // Already deleted
			}
			return deleted, fmt.Errorf("get entity %s/%s: %w", entityType, entityID, err)
		}

		if err := s.Delete(e); err != nil {
			return deleted, fmt.Errorf("delete entity %s/%s: %w", entityType, entityID, err)
		}
		deleted++
	}

	return deleted, nil
}

// Close releases resources held by the store.
func (s *EntityStore) Close() error {
	if err := s.kv.Close(); err != nil {
		return err
	}
	return s.indexStore.Close()
}
