package store

import (
	"bytes"

	"github.com/dgraph-io/badger/v4"
)

// BadgerIndexStore is a BadgerDB implementation of store.IndexStore.
type BadgerIndexStore struct {
	db *badger.DB
}

var _ IndexStore = (*BadgerIndexStore)(nil)

// NewBadgerIndexStore creates a new BadgerDB-backed index store.
func NewBadgerIndexStore(db *badger.DB) *BadgerIndexStore {
	return &BadgerIndexStore{db: db}
}

// Set stores an index entry.
func (s *BadgerIndexStore) Set(key []byte, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// Delete removes an index entry.
func (s *BadgerIndexStore) Delete(key []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// Scan iterates over keys in the range [start, end) in lexicographic order.
func (s *BadgerIndexStore) Scan(start, end []byte, fn func(key []byte) bool) error {
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
func (s *BadgerIndexStore) ScanPrefix(prefix []byte, fn func(key []byte) bool) error {
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

// Close closes the badger db connection if it's not already closed.
func (s *BadgerIndexStore) Close() error {
	if s.db.IsClosed() {
		return nil
	}
	return s.db.Close()
}
