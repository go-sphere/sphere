package badgerdb_test

import (
	"context"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-sphere/sphere/cache/badgerdb"
)

// An in-memory BadgerDB keeps the example free of files; use NewDatabase with
// a Config.Path for persistent storage.
func ExampleNewDatabaseWithOptions() {
	ctx := context.Background()
	db, err := badgerdb.NewDatabaseWithOptions(
		badger.DefaultOptions("").WithInMemory(true).WithLogger(nil),
	)
	if err != nil {
		fmt.Println("open:", err)
		return
	}
	defer func() { _ = db.Close() }() // owned: closes the BadgerDB

	if err := db.SetWithTTL(ctx, "k", []byte("v"), time.Hour); err != nil {
		fmt.Println("set:", err)
		return
	}
	v, found, err := db.Get(ctx, "k")
	fmt.Println(string(v), found, err)
	// Output: v true <nil>
}
