# Index Migration

## Problem

Config changes to indexes don't affect existing data:
- **New index**: Existing entities not indexed
- **Removed index**: Orphan index entries remain

## Solution

### CLI Commands

```bash
# Reindex a specific entity type (backfills all configured indexes)
chrono reindex -config chrono.yaml -type sensor

# Reindex all entity types
chrono reindex -config chrono.yaml

# Cleanup orphaned index entries for a type
chrono cleanup -config chrono.yaml -type sensor
```

### Implementation

**Reindex** (`cmd/server/reindex.go` or separate `cmd/reindex/`):
1. Open BadgerDB in exclusive mode
2. Scan all KV entries for the entity type (`{type}:*`)
3. For each entity:
   - Delete existing index entries
   - Rebuild index entries from current config
4. Report progress: `Reindexed 1000/5000 entities`

**Cleanup** (`cmd/server/cleanup.go`):
1. Open BadgerDB in exclusive mode
2. Scan index entries for the entity type (`{type}/*`)
3. For each index entry, check if field is in current config
4. Delete entries for unconfigured fields
5. Report: `Deleted 2500 orphaned index entries`

### Storage Helpers

Add to `EntityStore`:

```go
// ScanEntities iterates all entities of a type
func (s *EntityStore) ScanEntities(entityType string, fn func(*entity.Entity) error) error

// DeleteIndexes removes all index entries for an entity
func (s *EntityStore) DeleteIndexes(e *entity.Entity) error

// RebuildIndexes creates index entries for an entity using current config
func (s *EntityStore) RebuildIndexes(e *entity.Entity) error
```

### Constraints

- Requires exclusive DB access (server must be stopped)
- No online reindexing (simplicity over complexity)
- Progress written to stdout for visibility
