package sqlite

import (
	"database/sql"
	"database/sql/driver"
	"fmt"

	"modernc.org/sqlite"
)

// Driver wraps the modernc.org/sqlite.Driver to provide additional functionality.
// It automatically enables foreign key constraints on connection establishment.
type Driver struct {
	sqlite.Driver
}

// NewDriver creates a new SQLite driver instance with enhanced functionality.
// The returned driver automatically enables foreign key constraints for all connections.
func NewDriver() Driver {
	return Driver{
		Driver: sqlite.Driver{},
	}
}

// Open establishes a connection to the SQLite database and enables foreign key constraints.
// It wraps the underlying driver's Open method and executes "PRAGMA foreign_keys = on;"
// to ensure referential integrity is enforced. Returns an error if connection fails.
func (d Driver) Open(name string) (driver.Conn, error) {
	conn, err := d.Driver.Open(name)
	if err != nil {
		return conn, err
	}
	c := conn.(interface {
		Exec(stmt string, args []driver.Value) (driver.Result, error)
	})
	if _, e := c.Exec("PRAGMA foreign_keys = on;", nil); e != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to enable enable foreign keys: %w", e)
	}
	return conn, nil
}

// Register registers the enhanced SQLite driver with the sql package using the specified name.
// This allows the driver to be used with sql.Open() calls. The driver automatically
// enables foreign key constraints for all connections.
func Register(name string) {
	sql.Register(name, NewDriver())
}
