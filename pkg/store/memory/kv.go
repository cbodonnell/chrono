package memory

import (
	"sync"

	"github.com/cbodonnell/chrono/pkg/store"
)

// KVStore is an in-memory implementation of store.KVStore.
type KVStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewKVStore creates a new in-memory KV store.
func NewKVStore() *KVStore {
	return &KVStore{
		data: make(map[string][]byte),
	}
}

// Get retrieves the value for a key.
func (s *KVStore) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.data[key]
	if !ok {
		return nil, store.ErrNotFound
	}

	// Return a copy to prevent mutation
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// Set stores a key-value pair.
func (s *KVStore) Set(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy to prevent mutation
	stored := make([]byte, len(value))
	copy(stored, value)
	s.data[key] = stored
	return nil
}

// Delete removes a key.
func (s *KVStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// Close releases resources.
func (s *KVStore) Close() error {
	return nil
}
