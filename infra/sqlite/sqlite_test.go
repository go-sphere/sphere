package sqlite

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
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
