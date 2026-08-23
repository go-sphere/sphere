// Package badgerdb is a persistent cache.ByteCache driver on BadgerDB.
//
// []byte only. DelAll enumerates keys and MultiDel; it does not call DropAll
// (unsafe against concurrent reads). The scan+delete is not atomic. GetDel
// retries ErrConflict until success, miss, or ctx.Err(). Keys honours ctx
// per key; most other methods ignore ctx. NewDatabase and
// NewDatabaseWithOptions own the DB; NewDatabaseWithBadger does not.
package badgerdb

import (
	"context"
	"errors"
	"runtime"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-sphere/sphere/cache"
)

// Config holds configuration options for BadgerDB.
type Config struct {
	Path string `json:"path"`
}

// Database is a BadgerDB-backed cache implementation that provides persistent key-value storage.
// It implements the ByteCache interface using BadgerDB as the underlying storage engine.
type Database struct {
	db *badger.DB
	// owned reports whether this Database opened the underlying BadgerDB
	// instance and is therefore responsible for closing it. Injected instances
	// are not owned.
	owned bool
}

// NewDatabase creates a new BadgerDB cache with the specified configuration.
// It opens a BadgerDB instance at the configured path with default options.
// The instance is owned by this Database, so Close closes it.
func NewDatabase(conf Config) (*Database, error) {
	db, err := badger.Open(badger.DefaultOptions(conf.Path))
	if err != nil {
		return nil, err
	}
	return &Database{
		db:    db,
		owned: true,
	}, nil
}

// NewDatabaseWithBadger creates a new Database wrapper around an existing BadgerDB instance.
// This allows for advanced configuration and sharing of BadgerDB instances.
// The instance is injected, not owned: Close does not close it, so the caller
// keeps ownership and must close the *badger.DB itself.
func NewDatabaseWithBadger(db *badger.DB) *Database {
	return &Database{
		db:    db,
		owned: false,
	}
}

// NewDatabaseWithOptions creates a new BadgerDB cache with custom BadgerDB options.
// This provides full control over BadgerDB configuration such as compression, encryption, etc.
// The instance is opened here and owned by this Database, so Close closes it.
func NewDatabaseWithOptions(opts badger.Options) (*Database, error) {
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &Database{
		db:    db,
		owned: true,
	}, nil
}

func (d *Database) Set(ctx context.Context, key string, val []byte) error {
	return d.db.Update(func(txn *badger.Txn) error {
		return txn.SetEntry(badger.NewEntry([]byte(key), val))
	})
}

func (d *Database) SetWithTTL(ctx context.Context, key string, val []byte, expiration time.Duration) error {
	if expiration < 0 {
		return cache.ErrInvalidTTL
	}
	return d.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), val)
		if expiration > 0 {
			entry = entry.WithTTL(expiration)
		}
		// expiration == 0: never expire; skip WithTTL so no ExpiresAt is set.
		return txn.SetEntry(entry)
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
	if expiration < 0 {
		return cache.ErrInvalidTTL
	}
	return d.db.Update(func(txn *badger.Txn) error {
		for k, v := range valMap {
			entry := badger.NewEntry([]byte(k), v)
			if expiration > 0 {
				entry = entry.WithTTL(expiration)
			}
			// expiration == 0: never expire; skip WithTTL so no ExpiresAt is set.
			if err := txn.SetEntry(entry); err != nil {
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

// GetDel reads and deletes the key in one transaction, so an entry is returned
// as found to at most one caller. BadgerDB runs Update transactions under
// optimistic concurrency control, and GetDel is the only method here whose
// read set overlaps its write set: concurrent callers on the same key make all
// but one fail with ErrConflict. That is a retryable outcome rather than a
// caller-visible failure, so retry until it clears — the losing transaction
// committed nothing, and once the winner's delete lands the retry simply
// reports the key as missing.
func (d *Database) GetDel(ctx context.Context, key string) ([]byte, bool, error) {
	for {
		var val []byte
		err := d.db.Update(func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(key))
			if err != nil {
				return err
			}
			val, err = item.ValueCopy(nil)
			if err != nil {
				return err
			}
			return txn.Delete([]byte(key))
		})
		switch {
		case err == nil:
			return val, true, nil
		case errors.Is(err, badger.ErrKeyNotFound):
			return nil, false, nil
		case errors.Is(err, badger.ErrConflict):
			if cerr := ctx.Err(); cerr != nil {
				return nil, false, cerr
			}
			runtime.Gosched()
		default:
			return nil, false, err
		}
	}
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

// DelAll removes every entry by enumerating the keyspace and a single
// MultiDel, mirroring nscache.NSCache.DelAll.
//
// It deliberately avoids badger's DropAll, which is documented as safe against
// concurrent writes but not against concurrent reads — the caller is expected to
// guarantee no reads are in flight, or badger may panic. That guarantee cannot
// be offered here: DelAll reaches callers through Evictor, a required member of
// the Cache interface, so anyone holding a cache.ByteCache can invoke it without
// knowing which driver is underneath, let alone quiescing readers first.
//
// Like the nscache equivalent, this is not atomic: a key written between the
// scan and the delete survives.
func (d *Database) DelAll(ctx context.Context) error {
	keys, err := d.Keys(ctx, "")
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return d.MultiDel(ctx, keys)
}

// Keys returns every key whose name starts with prefix. It uses a read-only
// transaction with value prefetching disabled so the iterator only walks the
// LSM keyspace.
func (d *Database) Keys(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	err := d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		if prefix != "" {
			opts.Prefix = []byte(prefix)
		}
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			if err := ctx.Err(); err != nil {
				return err
			}
			keys = append(keys, string(it.Item().KeyCopy(nil)))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
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

// Close releases resources owned by this Database. When the BadgerDB instance
// was injected (not owned), Close is a no-op and leaves it open for its owner
// to close; only an instance opened by this Database is closed here.
func (d *Database) Close() error {
	if d.owned {
		return d.db.Close()
	}
	return nil
}

// Sync flushes in-memory writes to disk.
func (d *Database) Sync() error {
	return d.db.Sync()
}
