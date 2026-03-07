package badger

import (
	"github.com/cbodonnell/chrono/pkg/store"
	"github.com/dgraph-io/badger/v4"
)

// KVStore is a BadgerDB implementation of store.KVStore.
type KVStore struct {
	db *badger.DB
}

// NewKVStore creates a new BadgerDB-backed KV store.
func NewKVStore(db *badger.DB) *KVStore {
	return &KVStore{db: db}
}

// Get retrieves the value for a key.
func (s *KVStore) Get(key string) ([]byte, error) {
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
		return nil, store.ErrNotFound
	}
	return val, err
}

// Set stores a key-value pair.
func (s *KVStore) Set(key string, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
}

// Delete removes a key.
func (s *KVStore) Delete(key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// Close is a no-op since the DB lifecycle is managed externally.
func (s *KVStore) Close() error {
	return nil
}
