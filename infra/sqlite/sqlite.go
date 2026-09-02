// Package sqlite registers a database/sql driver around the already
// registered modernc.org/sqlite driver instance (not a zero sqlite.Driver),
// so RegisterScalarFunction and similar still work.
//
// Open runs PRAGMA foreign_keys = on. sql.Register panics on a duplicate
// name — Register(name) is not idempotent.
package sqlite

import (
	"database/sql"
	"database/sql/driver"
	"fmt"

	"modernc.org/sqlite"
)

// Driver wraps the modernc.org/sqlite driver to provide additional functionality.
// It automatically enables foreign key constraints on connection establishment.
//
// It delegates to the driver instance modernc.org/sqlite registers under the
// name "sqlite" rather than embedding a fresh sqlite.Driver value. That package
// keeps every registration — scalar and aggregate functions, collations,
// connection hooks, virtual table modules — on that one instance, so a zero
// value carries none of them: anything registered through
// sqlite.RegisterScalarFunction and friends was silently absent here. Queries
// then failed with "no such function" on this driver while working on "sqlite"
// in the same process, and a missing custom collation was worse still, quietly
// falling back to byte order and changing how rows sort and compare.
type Driver struct {
	base driver.Driver
}

// NewDriver creates a new SQLite driver instance with enhanced functionality.
// The returned driver automatically enables foreign key constraints for all connections.
func NewDriver() Driver {
	return Driver{base: baseDriver()}
}

// baseDriver returns the driver modernc.org/sqlite registered with database/sql,
// which is the instance its package-level registration functions mutate.
func baseDriver() driver.Driver {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		// Only reachable if modernc.org/sqlite is not registered, which cannot
		// happen while it is imported.
		return &sqlite.Driver{}
	}
	base := db.Driver()
	_ = db.Close()
	return base
}

// Open establishes a connection to the SQLite database and enables foreign key constraints.
// It wraps the underlying driver's Open method and executes "PRAGMA foreign_keys = on;"
// to ensure referential integrity is enforced. Returns an error if connection fails.
func (d Driver) Open(name string) (driver.Conn, error) {
	base := d.base
	if base == nil {
		base = baseDriver()
	}
	conn, err := base.Open(name)
	if err != nil {
		return nil, err
	}
	c, ok := conn.(interface {
		Exec(stmt string, args []driver.Value) (driver.Result, error)
	})
	if !ok {
		_ = conn.Close()
		return nil, fmt.Errorf("sqlite: connection does not support Exec, cannot enable foreign keys")
	}
	if _, e := c.Exec("PRAGMA foreign_keys = on;", nil); e != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", e)
	}
	return conn, nil
}

// Register registers the enhanced SQLite driver with the sql package using the specified name.
// This allows the driver to be used with sql.Open() calls. The driver automatically
// enables foreign key constraints for all connections.
func Register(name string) {
	sql.Register(name, NewDriver())
}
