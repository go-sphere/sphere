// Package badgerdb is a persistent cache.ByteCache driver on BadgerDB.
//
// Use [NewDatabase] to open a database at a path, [NewDatabaseWithOptions] for
// custom badger.Options (including in-memory mode), or [NewDatabaseWithBadger]
// to wrap a *badger.DB you already manage. Values are []byte only; wrap with
// cache.NewJsonCache for typed values.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/cache/badgerdb"
//
//	db, err := badgerdb.NewDatabase(badgerdb.Config{Path: "./data/cache"})
//	if err != nil {
//		return err
//	}
//	defer db.Close() // closes the BadgerDB opened by NewDatabase
//
//	if err := db.SetWithTTL(ctx, "k", []byte("v"), time.Hour); err != nil {
//		return err
//	}
//	v, found, err := db.Get(ctx, "k")
//
// DelAll enumerates keys and deletes them in batches; it does not call DropAll
// (unsafe against concurrent reads). The scan+delete is not atomic. GetDel
// retries badger.ErrConflict until success, miss, or ctx.Err(). Keys and
// GetDel honour ctx; most other methods ignore it. TTLs are rounded up to whole
// seconds. NewDatabase and NewDatabaseWithOptions own the DB;
// NewDatabaseWithBadger does not.
package badgerdb
