package store

import (
	"bytes"
	"sort"
	"sync"
)

// MemoryIndexStore is an in-memory implementation of store.IndexStore using a sorted slice.
type MemoryIndexStore struct {
	mu   sync.RWMutex
	keys [][]byte
}

var _ IndexStore = (*MemoryIndexStore)(nil)

// NewMemoryIndexStore creates a new in-memory index store.
func NewMemoryIndexStore() *MemoryIndexStore {
	return &MemoryIndexStore{
		keys: make([][]byte, 0),
	}
}

// Set stores an index entry.
func (s *MemoryIndexStore) Set(key []byte, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find insertion point
	idx := sort.Search(len(s.keys), func(i int) bool {
		return bytes.Compare(s.keys[i], key) >= 0
	})

	// Check if key already exists
	if idx < len(s.keys) && bytes.Equal(s.keys[idx], key) {
		return nil // Already exists
	}

	// Insert at position
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)

	s.keys = append(s.keys, nil)
	copy(s.keys[idx+1:], s.keys[idx:])
	s.keys[idx] = keyCopy

	return nil
}

// Delete removes an index entry.
func (s *MemoryIndexStore) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := sort.Search(len(s.keys), func(i int) bool {
		return bytes.Compare(s.keys[i], key) >= 0
	})

	if idx < len(s.keys) && bytes.Equal(s.keys[idx], key) {
		s.keys = append(s.keys[:idx], s.keys[idx+1:]...)
	}

	return nil
}

// Scan iterates over keys in the range [start, end) in lexicographic order.
func (s *MemoryIndexStore) Scan(start, end []byte, fn func(key []byte) bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find start position
	startIdx := sort.Search(len(s.keys), func(i int) bool {
		return bytes.Compare(s.keys[i], start) >= 0
	})

	for i := startIdx; i < len(s.keys); i++ {
		if bytes.Compare(s.keys[i], end) >= 0 {
			break
		}
		if !fn(s.keys[i]) {
			break
		}
	}

	return nil
}

// ReverseScan iterates over keys in the range [start, end) in reverse lexicographic order.
func (s *MemoryIndexStore) ReverseScan(start, end []byte, fn func(key []byte) bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find the position just before end (exclusive upper bound)
	endIdx := sort.Search(len(s.keys), func(i int) bool {
		return bytes.Compare(s.keys[i], end) >= 0
	}) - 1

	for i := endIdx; i >= 0; i-- {
		if bytes.Compare(s.keys[i], start) < 0 {
			break
		}
		if !fn(s.keys[i]) {
			break
		}
	}

	return nil
}

// ScanPrefix iterates over all keys with the given prefix.
func (s *MemoryIndexStore) ScanPrefix(prefix []byte, fn func(key []byte) bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find start position
	startIdx := sort.Search(len(s.keys), func(i int) bool {
		return bytes.Compare(s.keys[i], prefix) >= 0
	})

	for i := startIdx; i < len(s.keys); i++ {
		if !bytes.HasPrefix(s.keys[i], prefix) {
			break
		}
		if !fn(s.keys[i]) {
			break
		}
	}

	return nil
}

// Close releases resources.
func (s *MemoryIndexStore) Close() error {
	return nil
}
