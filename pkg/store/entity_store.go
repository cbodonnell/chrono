package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"sort"

	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/index"
)

// EntityStore is the main entry point for entity storage and retrieval.
type EntityStore struct {
	kv              KVStore
	indexStore      IndexStore
	registry        *index.Registry
	serializer      Serializer
	keyBuilder      *index.KeyBuilder
	retentionWorker *RetentionWorker
}

// NewEntityStore creates a new EntityStore.
func NewEntityStore(kv KVStore, indexStore IndexStore, registry *index.Registry, serializer Serializer) (*EntityStore, error) {
	es := &EntityStore{
		kv:         kv,
		indexStore: indexStore,
		registry:   registry,
		serializer: serializer,
		keyBuilder: index.NewKeyBuilder(),
	}

	// TODO: figure out if this should happen here or not.
	// It's convenient, but potentially unexpected for an index rebuild to occur.
	// Sync indexes (checks for config changes and reindexes if needed)
	if err := es.syncIndexes(); err != nil {
		return nil, fmt.Errorf("failed to sync indexes: %v", err)
	}

	// Start retention worker if any entity has TTL configured
	if registry.HasRetention() {
		es.retentionWorker = NewRetentionWorker(es, registry, DefaultRetentionConfig())
		es.retentionWorker.Start()
	}

	return es, nil
}

// Write stores an entity version and updates all configured indexes.
// Each write creates a new version - no overwrites occur.
func (s *EntityStore) Write(e *entity.Entity) error {
	// Use versioned KV key: {type}:{id}:{timestamp_hex}
	kvKey := buildVersionedKVKey(e.Type, e.ID, e.Timestamp)

	// Serialize full entity → KV (append-only, no deletion of old versions)
	blob, err := s.serializer.Marshal(e)
	if err != nil {
		return fmt.Errorf("serialize entity: %w", err)
	}
	if err := s.kv.Set(kvKey, blob); err != nil {
		return fmt.Errorf("kv set: %w", err)
	}

	// Write the _all index entry for time-series queries
	allKey := s.keyBuilder.BuildAllKey(e.Type, e.Timestamp, e.ID)
	if err := s.indexStore.Set(allKey, nil); err != nil {
		return fmt.Errorf("index set _all: %w", err)
	}

	// Write the _by_id index entry for version lookups
	byIDKey := s.keyBuilder.BuildByIDKey(e.Type, e.ID, e.Timestamp)
	if err := s.indexStore.Set(byIDKey, nil); err != nil {
		return fmt.Errorf("index set _by_id: %w", err)
	}

	// Look up index config for this entity type
	cfg := s.registry.Get(e.Type)
	if cfg == nil {
		return nil // No indexes configured, just store in KV
	}

	// Write one index entry per indexed field
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

// Get retrieves the latest version of an entity by type and ID.
// Uses reverse scan of _by_id index to find the most recent timestamp.
func (s *EntityStore) Get(entityType, entityID string) (*entity.Entity, error) {
	// Build range for reverse scan of _by_id index
	start := s.keyBuilder.BuildByIDRangeStart(entityType, entityID, 0)
	end := s.keyBuilder.BuildByIDRangeEnd(entityType, entityID, math.MaxInt64)

	var latestTimestamp int64 = -1
	err := s.indexStore.ReverseScan(start, end, func(key []byte) bool {
		_, _, ts := index.ParseByIDIndexKey(key)
		latestTimestamp = ts
		return false // Stop after first result (newest)
	})
	if err != nil {
		return nil, fmt.Errorf("scan _by_id index: %w", err)
	}

	if latestTimestamp < 0 {
		return nil, ErrNotFound
	}

	return s.GetVersion(entityType, entityID, latestTimestamp)
}

// GetVersion retrieves a specific version of an entity by type, ID, and timestamp.
func (s *EntityStore) GetVersion(entityType, entityID string, timestamp int64) (*entity.Entity, error) {
	kvKey := buildVersionedKVKey(entityType, entityID, timestamp)
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

// HistoryOptions configures a GetHistory query.
type HistoryOptions struct {
	TimeRange *TimeRange
	Limit     int
	Reverse   bool // true = newest first (descending timestamp)
}

// GetHistory retrieves all versions of an entity.
func (s *EntityStore) GetHistory(entityType, entityID string, opts *HistoryOptions) ([]*entity.Entity, error) {
	var fromTS int64 = 0
	var toTS int64 = math.MaxInt64

	if opts != nil && opts.TimeRange != nil {
		fromTS = opts.TimeRange.From
		toTS = opts.TimeRange.To
	}

	start := s.keyBuilder.BuildByIDRangeStart(entityType, entityID, fromTS)
	end := s.keyBuilder.BuildByIDRangeEnd(entityType, entityID, toTS)

	// Collect timestamps from _by_id index
	var timestamps []int64
	scanFn := func(key []byte) bool {
		_, _, ts := index.ParseByIDIndexKey(key)
		timestamps = append(timestamps, ts)
		// Apply limit during scan if set
		if opts != nil && opts.Limit > 0 && len(timestamps) >= opts.Limit {
			return false
		}
		return true
	}

	var err error
	if opts != nil && opts.Reverse {
		err = s.indexStore.ReverseScan(start, end, scanFn)
	} else {
		err = s.indexStore.Scan(start, end, scanFn)
	}
	if err != nil {
		return nil, fmt.Errorf("scan _by_id index: %w", err)
	}

	// Fetch entities
	entities := make([]*entity.Entity, 0, len(timestamps))
	for _, ts := range timestamps {
		e, err := s.GetVersion(entityType, entityID, ts)
		if err != nil {
			if err == ErrNotFound {
				continue // Version was deleted
			}
			return nil, err
		}
		entities = append(entities, e)
	}

	return entities, nil
}

// Delete removes a specific version of an entity and all its index entries.
func (s *EntityStore) Delete(e *entity.Entity) error {
	// Use versioned KV key
	kvKey := buildVersionedKVKey(e.Type, e.ID, e.Timestamp)
	if err := s.kv.Delete(kvKey); err != nil && err != ErrNotFound {
		return fmt.Errorf("kv delete: %w", err)
	}
	return s.deleteIndexes(e)
}

// DeleteEntity removes ALL versions of an entity.
func (s *EntityStore) DeleteEntity(entityType, entityID string) error {
	// Get all versions
	versions, err := s.GetHistory(entityType, entityID, nil)
	if err != nil {
		return fmt.Errorf("get history: %w", err)
	}

	// Delete each version
	for _, e := range versions {
		if err := s.Delete(e); err != nil {
			return fmt.Errorf("delete version: %w", err)
		}
	}

	return nil
}

// deleteIndexes removes all index entries for a specific entity version.
func (s *EntityStore) deleteIndexes(e *entity.Entity) error {
	// Delete the _all index entry
	allKey := s.keyBuilder.BuildAllKey(e.Type, e.Timestamp, e.ID)
	if err := s.indexStore.Delete(allKey); err != nil {
		return fmt.Errorf("index delete _all: %w", err)
	}

	// Delete the _by_id index entry
	byIDKey := s.keyBuilder.BuildByIDKey(e.Type, e.ID, e.Timestamp)
	if err := s.indexStore.Delete(byIDKey); err != nil {
		return fmt.Errorf("index delete _by_id: %w", err)
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

// buildVersionedKVKey constructs a versioned KV key for entity storage.
// Format: "{type}:{id}:{timestamp_hex}" e.g. "gamestate:game-123:800f3adc12345678"
func buildVersionedKVKey(entityType, entityID string, timestamp int64) string {
	// XOR with sign bit to make the timestamp sortable (same as encoding.EncodeTimestamp)
	u := uint64(timestamp) ^ (1 << 63)
	return fmt.Sprintf("%s:%s:%016x", entityType, entityID, u)
}

// syncIndexes checks index configurations and rebuilds indexes if configurations have changed.
// This should be called after creating the EntityStore and before using it.
// Returns an error if reindexing fails.
func (s *EntityStore) syncIndexes() error {
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
			log.Printf("reindex: failed to unmarshal entity %s: %v", key, err)
			return true // Continue with other entities
		}

		// Write the _all index entry
		allKey := s.keyBuilder.BuildAllKey(e.Type, e.Timestamp, e.ID)
		if err := s.indexStore.Set(allKey, nil); err != nil {
			log.Printf("reindex: failed to write _all index for %s/%s: %v", e.Type, e.ID, err)
			return true // Continue with other entities
		}

		// Write the _by_id index entry
		byIDKey := s.keyBuilder.BuildByIDKey(e.Type, e.ID, e.Timestamp)
		if err := s.indexStore.Set(byIDKey, nil); err != nil {
			log.Printf("reindex: failed to write _by_id index for %s/%s: %v", e.Type, e.ID, err)
			return true // Continue with other entities
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
				if err := s.indexStore.Set(k, nil); err != nil {
					log.Printf("reindex: failed to write field index %s for %s/%s: %v", idxField.Name, e.Type, e.ID, err)
				}
			}
		}

		return true
	}); err != nil {
		return fmt.Errorf("scan entities for reindex: %w", err)
	}

	return nil
}

// versionKey identifies a specific entity version.
type versionKey struct {
	EntityID  string
	Timestamp int64
}

// DeleteExpiredBatch deletes up to limit expired entity versions (timestamps before cutoffNS).
// Returns the number of versions deleted.
func (s *EntityStore) DeleteExpiredBatch(entityType string, cutoffNS int64, limit int) (int, error) {
	// Build range for _all index: from beginning of time to cutoff
	startKey := s.keyBuilder.BuildAllRangeStart(entityType, 0)
	endKey := s.keyBuilder.BuildAllRangeEnd(entityType, cutoffNS)

	// Collect versions to delete (entity ID + timestamp pairs, up to limit)
	var versions []versionKey
	err := s.indexStore.Scan(startKey, endKey, func(key []byte) bool {
		_, ts, entityID := index.ParseAllIndexKey(key)
		if entityID != "" {
			versions = append(versions, versionKey{EntityID: entityID, Timestamp: ts})
		}
		return len(versions) < limit
	})
	if err != nil {
		return 0, fmt.Errorf("scan expired versions: %w", err)
	}

	// Delete each version (this handles KV + all index cleanup)
	deleted := 0
	for _, v := range versions {
		e, err := s.GetVersion(entityType, v.EntityID, v.Timestamp)
		if err != nil {
			if err == ErrNotFound {
				continue // Already deleted
			}
			return deleted, fmt.Errorf("get version %s/%s@%d: %w", entityType, v.EntityID, v.Timestamp, err)
		}

		if err := s.Delete(e); err != nil {
			return deleted, fmt.Errorf("delete version %s/%s@%d: %w", entityType, v.EntityID, v.Timestamp, err)
		}
		deleted++
	}

	return deleted, nil
}

// Close releases resources held by the store.
func (s *EntityStore) Close(ctx context.Context) error {
	if s.retentionWorker != nil {
		if err := s.retentionWorker.Stop(ctx); err != nil {
			return err
		}
	}

	if err := s.kv.Close(); err != nil {
		return err
	}
	if err := s.indexStore.Close(); err != nil {
		return err
	}

	return nil
}
