package badger

import (
	"bytes"

	"github.com/dgraph-io/badger/v4"
)

// IndexStore is a BadgerDB implementation of store.IndexStore.
type IndexStore struct {
	db *badger.DB
}

// NewIndexStore creates a new BadgerDB-backed index store.
func NewIndexStore(db *badger.DB) *IndexStore {
	return &IndexStore{db: db}
}

// Set stores an index entry.
func (s *IndexStore) Set(key []byte, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// Delete removes an index entry.
func (s *IndexStore) Delete(key []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// Scan iterates over keys in the range [start, end) in lexicographic order.
func (s *IndexStore) Scan(start, end []byte, fn func(key []byte) bool) error {
	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(start); it.Valid(); it.Next() {
			key := it.Item().Key()
			if bytes.Compare(key, end) >= 0 {
				break
			}
			if !fn(key) {
				break
			}
		}
		return nil
	})
}

// ScanPrefix iterates over all keys with the given prefix.
func (s *IndexStore) ScanPrefix(prefix []byte, fn func(key []byte) bool) error {
	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if !fn(it.Item().Key()) {
				break
			}
		}
		return nil
	})
}

// Close is a no-op since the DB lifecycle is managed externally.
func (s *IndexStore) Close() error {
	return nil
}
