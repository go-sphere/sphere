package badgerdb

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
)

func newTestDB(t *testing.T) *Database {
	t.Helper()
	db, err := NewDatabase(Config{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestKeys(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	if err := db.MultiSet(ctx, map[string][]byte{
		"a:1": []byte("1"),
		"a:2": []byte("2"),
		"b:1": []byte("3"),
	}); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}

	got, err := db.Keys(ctx, "a:")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	slices.Sort(got)
	want := []string{"a:1", "a:2"}
	if !slices.Equal(got, want) {
		t.Fatalf("Keys mismatch: got=%v want=%v", got, want)
	}

	all, err := db.Keys(ctx, "")
	if err != nil {
		t.Fatalf("Keys empty prefix: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("Keys empty prefix len: got=%d want=3", len(all))
	}
}

// TestCloseOwnership pins the ownership rule: Close releases only a BadgerDB
// instance this Database opened. Getting it wrong is silent in one direction
// (a leak) and destructive in the other (a shared instance closed out from
// under its owner), so both branches are asserted.
func TestCloseOwnership(t *testing.T) {
	ctx := context.Background()

	t.Run("injected", func(t *testing.T) {
		db, err := badger.Open(badger.DefaultOptions(t.TempDir()))
		if err != nil {
			t.Fatalf("open badger: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })

		d := NewDatabaseWithBadger(db)
		if err := d.Set(ctx, "k", []byte("v")); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := d.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		if err := db.View(func(txn *badger.Txn) error {
			_, err := txn.Get([]byte("k"))
			return err
		}); err != nil {
			t.Fatalf("injected instance must stay open after Close: %v", err)
		}
	})

	t.Run("owned", func(t *testing.T) {
		d, err := NewDatabaseWithOptions(badger.DefaultOptions(t.TempDir()))
		if err != nil {
			t.Fatalf("NewDatabaseWithOptions: %v", err)
		}
		if err := d.Set(ctx, "k", []byte("v")); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := d.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		if _, _, err := d.Get(ctx, "k"); !errors.Is(err, badger.ErrDBClosed) {
			t.Fatalf("owned instance must be closed: got err=%v", err)
		}
	})
}

// The long keys exceed Badger's transaction budget if deleted together.
func TestDelAllBatches(t *testing.T) {
	ctx := t.Context()
	db := newTestDB(t)

	const keys = 2037
	valMap := make(map[string][]byte, 500)
	prefix := strings.Repeat("k", 8*1024)
	for i := range keys {
		valMap[fmt.Sprintf("%s_%06d", prefix, i)] = []byte("v")
		if len(valMap) == 500 {
			if err := db.MultiSet(ctx, valMap); err != nil {
				t.Fatalf("MultiSet: %v", err)
			}
			clear(valMap)
		}
	}
	if err := db.MultiSet(ctx, valMap); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}

	if err := db.DelAll(ctx); err != nil {
		t.Fatalf("DelAll: %v", err)
	}
	got, err := db.Keys(ctx, "")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Keys after DelAll: got %d keys, want 0", len(got))
	}
}

// TestBatchLargerThanOneTxn pins that the batch methods split a batch that
// exceeds badger's transaction size limit instead of failing with
// ErrTxnTooBig. A 1 MiB memtable keeps the limit low enough for a quick test;
// the value threshold must then stay below badger's derived max batch size.
func TestBatchLargerThanOneTxn(t *testing.T) {
	ctx := context.Background()
	db, err := NewDatabaseWithOptions(badger.DefaultOptions("").
		WithInMemory(true).
		WithMemTableSize(1 << 20).
		WithValueThreshold(1 << 10).
		WithLogger(nil))
	if err != nil {
		t.Fatalf("NewDatabaseWithOptions: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const n = 20000
	valMap := make(map[string][]byte, n)
	keys := make([]string, 0, n)
	for i := range n {
		key := fmt.Sprintf("k%05d", i)
		valMap[key] = []byte("v")
		keys = append(keys, key)
	}
	// Precondition: the batch really is too big for one transaction.
	txnErr := db.db.Update(func(txn *badger.Txn) error {
		for _, key := range keys {
			if err := txn.Set([]byte(key), []byte("v")); err != nil {
				return err
			}
		}
		return nil
	})
	if !errors.Is(txnErr, badger.ErrTxnTooBig) {
		t.Fatalf("single-transaction write = %v, want ErrTxnTooBig", txnErr)
	}

	assertCount := func(step string, want int) {
		t.Helper()
		got, err := db.Keys(ctx, "")
		if err != nil {
			t.Fatalf("%s: Keys: %v", step, err)
		}
		if len(got) != want {
			t.Fatalf("%s: %d keys, want %d", step, len(got), want)
		}
	}

	if err := db.MultiSet(ctx, valMap); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}
	assertCount("MultiSet", n)
	if err := db.MultiDel(ctx, keys); err != nil {
		t.Fatalf("MultiDel: %v", err)
	}
	assertCount("MultiDel", 0)
	if err := db.MultiSetWithTTL(ctx, valMap, time.Hour); err != nil {
		t.Fatalf("MultiSetWithTTL: %v", err)
	}
	assertCount("MultiSetWithTTL", n)
}

// TestEmptyKeyRejected pins the documented driver limitation: badger rejects
// the empty key, so "" cannot be stored or read.
func TestEmptyKeyRejected(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	if err := db.Set(ctx, "", []byte("v")); !errors.Is(err, badger.ErrEmptyKey) {
		t.Fatalf("Set(\"\") = %v, want ErrEmptyKey", err)
	}
	if _, _, err := db.Get(ctx, ""); !errors.Is(err, badger.ErrEmptyKey) {
		t.Fatalf("Get(\"\") = %v, want ErrEmptyKey", err)
	}
}
