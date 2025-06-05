package badgerdb

import (
	"context"
	"errors"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type Config struct {
	Path string `json:"path"`
}

type Database struct {
	db *badger.DB
}

func NewDatabase(config *Config) (*Database, error) {
	db, err := badger.Open(badger.DefaultOptions(config.Path))
	if err != nil {
		return nil, err
	}
	return &Database{
		db: db,
	}, nil
}

func NewDatabaseWithBadger(db *badger.DB) *Database {
	return &Database{
		db: db,
	}
}

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

func (d *Database) Sync() error {
	return d.db.Sync()
}

func (d *Database) Close() error {
	return d.db.Close()
}
