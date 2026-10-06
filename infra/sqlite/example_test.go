package sqlite_test

import (
	"database/sql"
	"fmt"

	"github.com/go-sphere/sphere/infra/sqlite"
)

func ExampleRegister() {
	// Register once per process; a duplicate name panics.
	sqlite.Register("sqlite_fk_example")

	db, err := sql.Open("sqlite_fk_example", ":memory:")
	if err != nil {
		fmt.Println("open:", err)
		return
	}
	defer func() { _ = db.Close() }()

	var enabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		fmt.Println("query:", err)
		return
	}
	fmt.Println("foreign_keys:", enabled)
	// Output: foreign_keys: 1
}
