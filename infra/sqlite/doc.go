// Package sqlite registers a database/sql driver that wraps modernc.org/sqlite
// and enables foreign key enforcement on every connection.
//
// Call [Register] once per process with a driver name, then open databases
// with [database/sql.Open] under that name. [NewDriver] returns the driver value
// for callers that register or use it themselves.
//
// # Usage
//
//	import (
//		"database/sql"
//
//		"github.com/go-sphere/sphere/infra/sqlite"
//	)
//
//	sqlite.Register("sqlite3_fk") // once per process; panics on a duplicate name
//
//	db, err := sql.Open("sqlite3_fk", "file:app.db")
//	if err != nil {
//		return err
//	}
//	defer db.Close()
//
// Each new connection runs PRAGMA foreign_keys = on. The driver delegates to
// the instance modernc.org/sqlite registered under "sqlite" (not a zero
// sqlite.Driver), so functions and collations registered through
// sqlite.RegisterScalarFunction and similar remain available.
package sqlite
