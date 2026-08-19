package badgerdb

import (
	"context"
	"errors"
	"slices"
	"testing"

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
