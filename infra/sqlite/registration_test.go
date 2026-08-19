package sqlite

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	msqlite "modernc.org/sqlite"
)

// TestPackageLevelRegistrationsApply pins that a function registered through
// modernc.org/sqlite's package-level API is visible on this driver. Embedding a
// zero sqlite.Driver silently dropped every such registration, so a query using
// one failed with "no such function" here while working on the "sqlite" driver
// in the same process.
func TestPackageLevelRegistrationsApply(t *testing.T) {
	const fn = "sphere_test_answer"
	if err := msqlite.RegisterScalarFunction(fn, 0, func(ctx *msqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		return int64(42), nil
	}); err != nil {
		t.Fatalf("register scalar function: %v", err)
	}

	Register("sphere-sqlite-test")
	db, err := sql.Open("sphere-sqlite-test", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	var got int64
	if err := db.QueryRow("SELECT " + fn + "()").Scan(&got); err != nil {
		t.Fatalf("query: %v", err)
	}
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}

	var fk int64
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, want 1", fk)
	}
}
