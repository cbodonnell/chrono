package store

import (
	"math"
	"sort"

	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/index"
)

// Op represents a filter operation.
type Op uint8

const (
	OpEq       Op = iota // Equal
	OpLt                 // Less than
	OpLte                // Less than or equal
	OpGt                 // Greater than
	OpGte                // Greater than or equal
	OpContains           // Array contains element
)

// Query defines a query against the entity store.
type Query struct {
	EntityType     string
	Filters        []FieldFilter // AND semantics
	TimeRange      *TimeRange
	Limit          int
	Reverse        bool // If true, return results in reverse chronological order (newest first)
	IncludeHistory bool // If true, return all versions; if false (default), return only latest per entity ID
}

// FieldFilter specifies a filter on a single field.
type FieldFilter struct {
	Field string
	Op    Op
	Value entity.Value
}

// TimeRange specifies a time range for queries.
type TimeRange struct {
	From int64 // Unix nanoseconds (inclusive)
	To   int64 // Unix nanoseconds (inclusive)
}

// entityKey uniquely identifies an entity by timestamp and ID.
type entityKey struct {
	Timestamp int64
	ID        string
}

// Query executes a query and returns matching entities.
// TODO: figure out multi-tenancy within the database
func (s *EntityStore) Query(q *Query) ([]*entity.Entity, error) {
	// If no filters, use the _all index for time-series query
	if len(q.Filters) == 0 {
		return s.queryAll(q)
	}

	// Execute each filter and collect entity keys
	var resultSets []map[entityKey]struct{}

	for _, filter := range q.Filters {
		keys, err := s.scanFilter(q.EntityType, filter, q.TimeRange)
		if err != nil {
			return nil, err
		}
		resultSets = append(resultSets, keys)
	}

	// Intersect result sets (smallest first for efficiency)
	sort.Slice(resultSets, func(i, j int) bool {
		return len(resultSets[i]) < len(resultSets[j])
	})

	result := resultSets[0]
	for i := 1; i < len(resultSets); i++ {
		result = intersect(result, resultSets[i])
		if len(result) == 0 {
			return nil, nil
		}
	}

	// Fetch entities from KV store
	return s.fetchEntities(q.EntityType, result, q.Limit, q.Reverse, q.IncludeHistory)
}

// queryAll queries using the _all index for time-series access.
func (s *EntityStore) queryAll(q *Query) ([]*entity.Entity, error) {
	keys := make(map[entityKey]struct{})

	var start, end []byte
	kb := index.NewKeyBuilder()

	if q.TimeRange != nil {
		start = kb.BuildAllRangeStart(q.EntityType, q.TimeRange.From)
		end = kb.BuildAllRangeEnd(q.EntityType, q.TimeRange.To)
	} else {
		start = kb.BuildAllRangeStart(q.EntityType, math.MinInt64)
		end = kb.BuildAllRangeEnd(q.EntityType, math.MaxInt64)
	}

	scanFn := func(key []byte) bool {
		_, ts, id := index.ParseAllIndexKey(key)
		keys[entityKey{Timestamp: ts, ID: id}] = struct{}{}
		return true
	}

	var err error
	if q.Reverse {
		err = s.indexStore.ReverseScan(start, end, scanFn)
	} else {
		err = s.indexStore.Scan(start, end, scanFn)
	}
	if err != nil {
		return nil, err
	}

	return s.fetchEntities(q.EntityType, keys, q.Limit, q.Reverse, q.IncludeHistory)
}

// scanFilter scans the index for a single filter.
func (s *EntityStore) scanFilter(entityType string, filter FieldFilter, timeRange *TimeRange) (map[entityKey]struct{}, error) {
	keys := make(map[entityKey]struct{})
	kb := index.NewKeyBuilder()

	fromTS := int64(math.MinInt64)
	toTS := int64(math.MaxInt64)
	if timeRange != nil {
		fromTS = timeRange.From
		toTS = timeRange.To
	}

	switch filter.Op {
	case OpEq:
		// Exact match: scan prefix with value
		start := kb.BuildRangeStart(entityType, filter.Field, filter.Value, fromTS)
		end := kb.BuildRangeEnd(entityType, filter.Field, filter.Value, toTS)

		err := s.indexStore.Scan(start, end, func(key []byte) bool {
			_, _, ts, id := index.ParseIndexKey(key)
			keys[entityKey{Timestamp: ts, ID: id}] = struct{}{}
			return true
		})
		if err != nil {
			return nil, err
		}

	case OpContains:
		// For array contains, same as OpEq on the element value
		start := kb.BuildRangeStart(entityType, filter.Field, filter.Value, fromTS)
		end := kb.BuildRangeEnd(entityType, filter.Field, filter.Value, toTS)

		err := s.indexStore.Scan(start, end, func(key []byte) bool {
			_, _, ts, id := index.ParseIndexKey(key)
			keys[entityKey{Timestamp: ts, ID: id}] = struct{}{}
			return true
		})
		if err != nil {
			return nil, err
		}

	case OpLt, OpLte, OpGt, OpGte:
		// Range queries: use bounded scan based on operator
		opStr := opToString(filter.Op)
		start := kb.BuildComparisonRangeStart(entityType, filter.Field, filter.Value, opStr)
		end := kb.BuildComparisonRangeEnd(entityType, filter.Field, filter.Value, opStr)

		err := s.indexStore.Scan(start, end, func(key []byte) bool {
			_, _, ts, id := index.ParseIndexKey(key)
			if !timeInRange(ts, fromTS, toTS) {
				return true // Continue scanning
			}
			keys[entityKey{Timestamp: ts, ID: id}] = struct{}{}
			return true
		})
		if err != nil {
			return nil, err
		}
	}

	return keys, nil
}

func timeInRange(ts, from, to int64) bool {
	return ts >= from && ts <= to
}

func opToString(op Op) string {
	switch op {
	case OpLt:
		return "lt"
	case OpLte:
		return "lte"
	case OpGt:
		return "gt"
	case OpGte:
		return "gte"
	default:
		return ""
	}
}

// intersect returns the intersection of two entity key sets.
func intersect(a, b map[entityKey]struct{}) map[entityKey]struct{} {
	result := make(map[entityKey]struct{})
	for key := range a {
		if _, ok := b[key]; ok {
			result[key] = struct{}{}
		}
	}
	return result
}

// fetchEntities fetches entities from KV store and returns them sorted by timestamp.
// When includeHistory is false, only the latest version of each entity ID is returned.
func (s *EntityStore) fetchEntities(entityType string, keys map[entityKey]struct{}, limit int, reverse bool, includeHistory bool) ([]*entity.Entity, error) {
	// If including history, fetch all matched versions
	if includeHistory {
		// Sort by timestamp
		sorted := make([]entityKey, 0, len(keys))
		for key := range keys {
			sorted = append(sorted, key)
		}
		if reverse {
			sort.Slice(sorted, func(i, j int) bool {
				if sorted[i].Timestamp != sorted[j].Timestamp {
					return sorted[i].Timestamp > sorted[j].Timestamp
				}
				return sorted[i].ID < sorted[j].ID
			})
		} else {
			sort.Slice(sorted, func(i, j int) bool {
				if sorted[i].Timestamp != sorted[j].Timestamp {
					return sorted[i].Timestamp < sorted[j].Timestamp
				}
				return sorted[i].ID < sorted[j].ID
			})
		}

		// Apply limit
		if limit > 0 && len(sorted) > limit {
			sorted = sorted[:limit]
		}

		// Fetch entities
		entities := make([]*entity.Entity, 0, len(sorted))
		for _, key := range sorted {
			e, err := s.GetVersion(entityType, key.ID, key.Timestamp)
			if err != nil {
				if err == ErrNotFound {
					continue // Entity was deleted
				}
				return nil, err
			}
			entities = append(entities, e)
		}

		return entities, nil
	}

	// Not including history - get unique entity IDs and fetch their LATEST versions
	seenIDs := make(map[string]bool)
	for key := range keys {
		seenIDs[key.ID] = true
	}

	// Fetch the latest version of each entity
	entities := make([]*entity.Entity, 0, len(seenIDs))
	for entityID := range seenIDs {
		e, err := s.Get(entityType, entityID)
		if err != nil {
			if err == ErrNotFound {
				continue // Entity was deleted
			}
			return nil, err
		}
		entities = append(entities, e)
	}

	// Sort by timestamp
	if reverse {
		sort.Slice(entities, func(i, j int) bool {
			return entities[i].Timestamp > entities[j].Timestamp
		})
	} else {
		sort.Slice(entities, func(i, j int) bool {
			return entities[i].Timestamp < entities[j].Timestamp
		})
	}

	// Apply limit
	if limit > 0 && len(entities) > limit {
		entities = entities[:limit]
	}

	return entities, nil
}
