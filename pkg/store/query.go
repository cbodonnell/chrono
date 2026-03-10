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
	EntityType  string
	Filters     []FieldFilter
	TimeRange   *TimeRange
	Limit       int
	Reverse     bool // If true, return results in reverse chronological order (newest first)
	AllVersions bool // If false (default), only return entities whose latest version matches filters; if true, return all matching historical versions
	MatchAny    bool // If true, use OR semantics (match any filter); if false (default), use AND semantics (match all filters)
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

// Less returns true if this key should sort before other.
func (k entityKey) Less(other entityKey) bool {
	if k.Timestamp != other.Timestamp {
		return k.Timestamp < other.Timestamp
	}
	return k.ID < other.ID
}

// Query executes a query and returns matching entities.
// TODO: figure out multi-tenancy within the database
func (s *EntityStore) Query(q *Query) ([]*entity.Entity, error) {
	if q.AllVersions {
		return s.queryAllVersions(q)
	}
	return s.queryLatestOnly(q)
}

// queryLatestOnly queries the _latest indexes for current entity state.
func (s *EntityStore) queryLatestOnly(q *Query) ([]*entity.Entity, error) {
	var entityIDs []string

	if len(q.Filters) == 0 {
		// No filters - scan _latest_all index
		ids, err := s.scanLatestAll(q.EntityType)
		if err != nil {
			return nil, err
		}
		entityIDs = ids
	} else {
		// Scan _latest indexes for each filter
		var resultSets []map[string]struct{}
		for _, filter := range q.Filters {
			ids, err := s.scanLatestFilter(q.EntityType, filter)
			if err != nil {
				return nil, err
			}
			resultSets = append(resultSets, ids)
		}

		// Combine result sets (intersect for AND, union for OR)
		result := combineResultSets(resultSets, q.MatchAny)
		if result == nil {
			return nil, nil
		}

		entityIDs = make([]string, 0, len(result))
		for id := range result {
			entityIDs = append(entityIDs, id)
		}
	}

	// Fetch latest version of each entity
	return s.fetchLatestEntities(q.EntityType, entityIDs, q.TimeRange, q.Limit, q.Reverse)
}

// queryAllVersions queries all versions using time-series indexes.
func (s *EntityStore) queryAllVersions(q *Query) ([]*entity.Entity, error) {
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

	// Combine result sets (intersect for AND, union for OR)
	result := combineResultSets(resultSets, q.MatchAny)
	if result == nil {
		return nil, nil
	}

	// Fetch entities from KV store
	return s.fetchVersionedEntities(q.EntityType, result, q.Limit, q.Reverse)
}

// queryAll queries using the _all index for time-series access (all versions).
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

	return s.fetchVersionedEntities(q.EntityType, keys, q.Limit, q.Reverse)
}

// scanLatestAll scans the _latest_all index for all entity IDs.
func (s *EntityStore) scanLatestAll(entityType string) ([]string, error) {
	kb := index.NewKeyBuilder()
	prefix := kb.BuildLatestAllPrefix(entityType)

	var ids []string
	err := s.indexStore.ScanPrefix(prefix, func(key []byte) bool {
		_, id := index.ParseLatestAllIndexKey(key)
		if id != "" {
			ids = append(ids, id)
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// scanLatestFilter scans the _latest index for a single filter.
func (s *EntityStore) scanLatestFilter(entityType string, filter FieldFilter) (map[string]struct{}, error) {
	ids := make(map[string]struct{})
	kb := index.NewKeyBuilder()

	switch filter.Op {
	case OpEq, OpContains:
		// Exact match: scan prefix with value
		prefix := kb.BuildLatestValuePrefix(entityType, filter.Field, filter.Value)
		err := s.indexStore.ScanPrefix(prefix, func(key []byte) bool {
			_, _, id := index.ParseLatestIndexKey(key)
			if id != "" {
				ids[id] = struct{}{}
			}
			return true
		})
		if err != nil {
			return nil, err
		}

	case OpLt, OpLte, OpGt, OpGte:
		// Range queries
		opStr := opToString(filter.Op)
		start := kb.BuildLatestComparisonRangeStart(entityType, filter.Field, filter.Value, opStr)
		end := kb.BuildLatestComparisonRangeEnd(entityType, filter.Field, filter.Value, opStr)

		err := s.indexStore.Scan(start, end, func(key []byte) bool {
			_, _, id := index.ParseLatestIndexKey(key)
			if id != "" {
				ids[id] = struct{}{}
			}
			return true
		})
		if err != nil {
			return nil, err
		}
	}

	return ids, nil
}

// intersectSets returns the intersection of two sets.
func intersectSets[K comparable](a, b map[K]struct{}) map[K]struct{} {
	result := make(map[K]struct{})
	for key := range a {
		if _, ok := b[key]; ok {
			result[key] = struct{}{}
		}
	}
	return result
}

// unionSets returns the union of two sets.
func unionSets[K comparable](a, b map[K]struct{}) map[K]struct{} {
	result := make(map[K]struct{}, len(a)+len(b))
	for key := range a {
		result[key] = struct{}{}
	}
	for key := range b {
		result[key] = struct{}{}
	}
	return result
}

// sortSlice sorts a slice using the provided less function, optionally in reverse order.
func sortSlice[T any](items []T, less func(a, b T) bool, reverse bool) {
	sort.Slice(items, func(i, j int) bool {
		if reverse {
			return less(items[j], items[i])
		}
		return less(items[i], items[j])
	})
}

// combineResultSets combines multiple result sets using intersection (AND) or union (OR).
// Returns nil if the result is empty during intersection.
func combineResultSets[K comparable](resultSets []map[K]struct{}, matchAny bool) map[K]struct{} {
	if len(resultSets) == 0 {
		return nil
	}

	// Sort by size for efficiency (smallest first for intersection)
	sort.Slice(resultSets, func(i, j int) bool {
		return len(resultSets[i]) < len(resultSets[j])
	})

	result := resultSets[0]
	for i := 1; i < len(resultSets); i++ {
		if matchAny {
			result = unionSets(result, resultSets[i])
		} else {
			result = intersectSets(result, resultSets[i])
			if len(result) == 0 {
				return nil
			}
		}
	}
	return result
}

// fetchLatestEntities fetches the latest version of each entity.
func (s *EntityStore) fetchLatestEntities(entityType string, entityIDs []string, timeRange *TimeRange, limit int, reverse bool) ([]*entity.Entity, error) {
	entities := make([]*entity.Entity, 0, len(entityIDs))
	for _, id := range entityIDs {
		e, err := s.Get(entityType, id)
		if err != nil {
			if err == ErrNotFound {
				continue
			}
			return nil, err
		}

		// Filter by time range if specified
		if timeRange != nil {
			if e.Timestamp < timeRange.From || e.Timestamp > timeRange.To {
				continue
			}
		}

		entities = append(entities, e)
	}

	// Sort by timestamp
	sortSlice(entities, func(a, b *entity.Entity) bool {
		return a.Timestamp < b.Timestamp
	}, reverse)

	// Apply limit
	if limit > 0 && len(entities) > limit {
		entities = entities[:limit]
	}

	return entities, nil
}

// fetchVersionedEntities fetches specific versions of entities.
func (s *EntityStore) fetchVersionedEntities(entityType string, keys map[entityKey]struct{}, limit int, reverse bool) ([]*entity.Entity, error) {
	// Sort by timestamp
	sorted := make([]entityKey, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	sortSlice(sorted, entityKey.Less, reverse)

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
				continue
			}
			return nil, err
		}
		entities = append(entities, e)
	}

	return entities, nil
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
	case OpEq, OpContains:
		// Exact match or array contains: scan range with value
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


