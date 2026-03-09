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
	EntityType string
	Filters    []FieldFilter // AND semantics
	TimeRange  *TimeRange
	Limit      int
	Reverse    bool // If true, return results in reverse chronological order (newest first)
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
	return s.fetchEntities(q.EntityType, result, q.Limit, q.Reverse)
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

	return s.fetchEntities(q.EntityType, keys, q.Limit, q.Reverse)
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
		// Range queries: scan the entire field prefix and filter
		prefix := kb.BuildPrefix(entityType, filter.Field)
		targetEncoded := index.EncodeValue(filter.Value)

		err := s.indexStore.ScanPrefix(prefix, func(key []byte) bool {
			// Extract the value portion for comparison
			valueStart := len(prefix)
			// Find the next separator after the value
			valuePart, ts, id := extractValueAndParse(key[valueStart:])

			if !timeInRange(ts, fromTS, toTS) {
				return true // Continue scanning
			}

			if compareOp(valuePart, targetEncoded, filter.Op) {
				keys[entityKey{Timestamp: ts, ID: id}] = struct{}{}
			}
			return true
		})
		if err != nil {
			return nil, err
		}
	}

	return keys, nil
}

// extractValueAndParse extracts value bytes and parses timestamp/ID from remainder.
func extractValueAndParse(data []byte) (value []byte, timestamp int64, entityID string) {
	// Find separators to extract components
	// Format: {value}/{timestamp}/{id}
	sep := byte(index.Separator)

	firstSep := -1
	secondSep := -1
	for i, b := range data {
		if b == sep {
			if firstSep == -1 {
				firstSep = i
			} else {
				secondSep = i
				break
			}
		}
	}

	if firstSep == -1 {
		return data, 0, ""
	}

	value = data[:firstSep]

	if secondSep == -1 || secondSep-firstSep-1 != 8 {
		return value, 0, ""
	}

	tsBytes := data[firstSep+1 : secondSep]
	timestamp = decodeTimestamp(tsBytes)
	entityID = string(data[secondSep+1:])

	return value, timestamp, entityID
}

func decodeTimestamp(b []byte) int64 {
	if len(b) != 8 {
		return 0
	}
	u := uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
	return int64(u ^ (1 << 63))
}

func timeInRange(ts, from, to int64) bool {
	return ts >= from && ts <= to
}

func compareOp(actual, target []byte, op Op) bool {
	cmp := compareBytes(actual, target)
	switch op {
	case OpLt:
		return cmp < 0
	case OpLte:
		return cmp <= 0
	case OpGt:
		return cmp > 0
	case OpGte:
		return cmp >= 0
	default:
		return false
	}
}

func compareBytes(a, b []byte) int {
	minLen := min(len(a), len(b))
	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
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
func (s *EntityStore) fetchEntities(entityType string, keys map[entityKey]struct{}, limit int, reverse bool) ([]*entity.Entity, error) {
	// Sort by timestamp
	sorted := make([]entityKey, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	if reverse {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Timestamp > sorted[j].Timestamp
		})
	} else {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Timestamp < sorted[j].Timestamp
		})
	}

	// Apply limit
	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}

	// Fetch entities
	entities := make([]*entity.Entity, 0, len(sorted))
	for _, key := range sorted {
		e, err := s.Get(entityType, key.ID)
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
