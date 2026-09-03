package sqlite

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	msqlite "modernc.org/sqlite"
)

type fakeDriver struct {
	conn driver.Conn
	err  error
}

func (d fakeDriver) Open(string) (driver.Conn, error) {
	return d.conn, d.err
}

type fakeConn struct {
	closed bool
}

func (*fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

func (*fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

type fakeExecConn struct {
	*fakeConn
	stmt    string
	execErr error
}

func (c *fakeExecConn) Exec(stmt string, _ []driver.Value) (driver.Result, error) {
	c.stmt = stmt
	return nil, c.execErr
}

func TestDriverOpenReturnsNilConnectionOnBaseError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("open failed")
	conn, err := (Driver{base: fakeDriver{conn: &fakeConn{}, err: wantErr}}).Open("test")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Open() error = %v, want %v", err, wantErr)
	}
	if conn != nil {
		t.Fatalf("Open() connection = %v, want nil", conn)
	}
}

func TestDriverOpenClosesConnectionWithoutExec(t *testing.T) {
	t.Parallel()

	baseConn := &fakeConn{}
	conn, err := (Driver{base: fakeDriver{conn: baseConn}}).Open("test")
	if err == nil || !strings.Contains(err.Error(), "does not support Exec") {
		t.Fatalf("Open() error = %v, want unsupported Exec error", err)
	}
	if conn != nil {
		t.Fatalf("Open() connection = %v, want nil", conn)
	}
	if !baseConn.closed {
		t.Fatal("Open() did not close connection without Exec support")
	}
}

func TestDriverOpenClosesConnectionWhenEnablingForeignKeysFails(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("exec failed")
	baseConn := &fakeConn{}
	execConn := &fakeExecConn{fakeConn: baseConn, execErr: wantErr}
	conn, err := (Driver{base: fakeDriver{conn: execConn}}).Open("test")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Open() error = %v, want %v", err, wantErr)
	}
	if strings.Contains(err.Error(), "enable enable") {
		t.Fatalf("Open() error contains duplicate word: %v", err)
	}
	if conn != nil {
		t.Fatalf("Open() connection = %v, want nil", conn)
	}
	if !baseConn.closed {
		t.Fatal("Open() did not close connection after Exec failed")
	}
}

func TestDriverOpenEnablesForeignKeys(t *testing.T) {
	t.Parallel()

	execConn := &fakeExecConn{fakeConn: &fakeConn{}}
	conn, err := (Driver{base: fakeDriver{conn: execConn}}).Open("test")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if conn != execConn {
		t.Fatalf("Open() connection = %v, want %v", conn, execConn)
	}
	if execConn.stmt != "PRAGMA foreign_keys = on;" {
		t.Fatalf("Exec statement = %q, want PRAGMA foreign_keys = on;", execConn.stmt)
	}
}

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
