package store

import "errors"

var (
	ErrNotFound = errors.New("key not found")
)

// KVStore is the interface for key-value storage of full entities.
// Keys follow the schema: {entity_type}:{entity_id}
type KVStore interface {
	// Get retrieves the value for a key. Returns ErrNotFound if key doesn't exist.
	Get(key string) ([]byte, error)

	// Set stores a key-value pair.
	Set(key string, value []byte) error

	// Delete removes a key.
	Delete(key string) error

	// ScanPrefix iterates over all keys with the given prefix.
	// The callback receives each key and value; return false to stop iteration.
	ScanPrefix(prefix string, fn func(key string, value []byte) bool) error

	// Close releases any resources held by the store.
	Close() error
}

// IndexStore is the interface for index storage with ordered key scans.
// Keys follow the schema: {entity_type}/{field_name}/{field_value_bytes}/{timestamp_unix_ns}/{entity_id}
type IndexStore interface {
	// Set stores an index entry (value is typically empty).
	Set(key []byte, value []byte) error

	// Delete removes an index entry.
	Delete(key []byte) error

	// Scan iterates over keys in the range [start, end) in lexicographic order.
	// The callback receives each key; return false to stop iteration.
	Scan(start, end []byte, fn func(key []byte) bool) error

	// ScanPrefix iterates over all keys with the given prefix.
	// The callback receives each key; return false to stop iteration.
	ScanPrefix(prefix []byte, fn func(key []byte) bool) error

	// Close releases any resources held by the store.
	Close() error
}
