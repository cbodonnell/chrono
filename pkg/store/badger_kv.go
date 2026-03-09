package store

import (
	"github.com/dgraph-io/badger/v4"
)

// BadgerKVStore is a BadgerDB implementation of store.KVStore.
type BadgerKVStore struct {
	db *badger.DB
}

var _ KVStore = (*BadgerKVStore)(nil)

// NewBadgerKVStore creates a new BadgerDB-backed KV store.
func NewBadgerKVStore(db *badger.DB) *BadgerKVStore {
	return &BadgerKVStore{db: db}
}

// Get retrieves the value for a key.
func (s *BadgerKVStore) Get(key string) ([]byte, error) {
	var val []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		val, err = item.ValueCopy(nil)
		return err
	})
	if err == badger.ErrKeyNotFound {
		return nil, ErrNotFound
	}
	return val, err
}

// Set stores a key-value pair.
func (s *BadgerKVStore) Set(key string, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
}

// Delete removes a key.
func (s *BadgerKVStore) Delete(key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// ScanPrefix iterates over all keys with the given prefix.
func (s *BadgerKVStore) ScanPrefix(prefix string, fn func(key string, value []byte) bool) error {
	prefixBytes := []byte(prefix)
	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefixBytes
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			item := it.Item()
			key := string(item.Key())
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			if !fn(key, val) {
				break
			}
		}
		return nil
	})
}

// Close closes the badger db connection if it's not already closed.
func (s *BadgerKVStore) Close() error {
	if s.db.IsClosed() {
		return nil
	}
	return s.db.Close()
}
