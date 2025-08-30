package badgerdb

import (
	"context"
	"errors"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// Config holds configuration options for BadgerDB.
type Config struct {
	Path string `json:"path"`
}

// Database is a BadgerDB-backed cache implementation that provides persistent key-value storage.
// It implements the ByteCache interface using BadgerDB as the underlying storage engine.
type Database struct {
	db *badger.DB
}

// NewDatabase creates a new BadgerDB cache with the specified configuration.
// It opens a BadgerDB instance at the configured path with default options.
func NewDatabase(config *Config) (*Database, error) {
	db, err := badger.Open(badger.DefaultOptions(config.Path))
	if err != nil {
		return nil, err
	}
	return &Database{
		db: db,
	}, nil
}

// NewDatabaseWithBadger creates a new Database wrapper around an existing BadgerDB instance.
// This allows for advanced configuration and sharing of BadgerDB instances.
func NewDatabaseWithBadger(db *badger.DB) *Database {
	return &Database{
		db: db,
	}
}

// NewDatabaseWithOptions creates a new BadgerDB cache with custom BadgerDB options.
// This provides full control over BadgerDB configuration such as compression, encryption, etc.
func NewDatabaseWithOptions(opts badger.Options) (*Database, error) {
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &Database{
		db: db,
	}, nil
}

func (d *Database) Set(ctx context.Context, key string, val []byte) error {
	return d.db.Update(func(txn *badger.Txn) error {
		return txn.SetEntry(badger.NewEntry([]byte(key), val))
	})
}

func (d *Database) SetWithTTL(ctx context.Context, key string, val []byte, expiration time.Duration) error {
	return d.db.Update(func(txn *badger.Txn) error {
		return txn.SetEntry(badger.NewEntry([]byte(key), val).WithTTL(expiration))
	})
}

func (d *Database) MultiSet(ctx context.Context, valMap map[string][]byte) error {
	return d.db.Update(func(txn *badger.Txn) error {
		for k, v := range valMap {
			err := txn.SetEntry(badger.NewEntry([]byte(k), v))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *Database) MultiSetWithTTL(ctx context.Context, valMap map[string][]byte, expiration time.Duration) error {
	return d.db.Update(func(txn *badger.Txn) error {
		for k, v := range valMap {
			err := txn.SetEntry(badger.NewEntry([]byte(k), v).WithTTL(expiration))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *Database) Get(ctx context.Context, key string) ([]byte, bool, error) {
	var val []byte
	err := d.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		val, err = item.ValueCopy(nil)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return val, true, nil
}

func (d *Database) MultiGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	res := make(map[string][]byte)
	err := d.db.View(func(txn *badger.Txn) error {
		for _, key := range keys {
			item, err := txn.Get([]byte(key))
			if err != nil {
				if errors.Is(err, badger.ErrKeyNotFound) {
					continue // Key not found, skip it
				}
				return err
			}
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			res[key] = val
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (d *Database) Del(ctx context.Context, key string) error {
	return d.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

func (d *Database) MultiDel(ctx context.Context, keys []string) error {
	return d.db.Update(func(txn *badger.Txn) error {
		for _, key := range keys {
			err := txn.Delete([]byte(key))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *Database) DelAll(ctx context.Context) error {
	return d.db.DropAll()
}

func (d *Database) Exists(ctx context.Context, key string) (bool, error) {
	err := d.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key))
		return err
	})
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) Sync() error {
	return d.db.Sync()
}
