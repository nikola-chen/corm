package builder_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/dialect"
)

type testResult struct{ id int64 }

func (r testResult) LastInsertId() (int64, error) { return r.id, nil }
func (r testResult) RowsAffected() (int64, error) { return 1, nil }

type fakeExec struct{ id int64 }

func (f fakeExec) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return testResult{id: f.id}, nil
}
func (f fakeExec) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, errors.New("unexpected QueryContext")
}

func TestInsertExecAndReturnID_MySQL(t *testing.T) {
	qb := builder.NewAPI(dialect.MustGet("mysql"), fakeExec{id: 7})
	b := qb.Insert("users").Columns("name").Values("alice")
	id, err := b.ExecAndReturnID(context.Background(), "id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 7 {
		t.Fatalf("want 7, got %d", id)
	}
}

var registerOnce sync.Once

func registerTestDriver() {
	registerOnce.Do(func() {
		sql.Register("corm_insertid_test", testDriver{})
	})
}

type testDriver struct{}

func (testDriver) Open(name string) (driver.Conn, error) { return testConn{}, nil }

type testConn struct{}

func (testConn) Prepare(query string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (testConn) Close() error                              { return nil }
func (testConn) Begin() (driver.Tx, error)                 { return nil, errors.New("not supported") }

func (testConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return &testRows{}, nil
}

type testRows struct{ done bool }

func (testRows) Columns() []string { return []string{"id"} }
func (r *testRows) Close() error   { return nil }
func (r *testRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = int64(42)
	return nil
}

func TestInsertExecAndReturnID_Postgres(t *testing.T) {
	registerTestDriver()
	db, err := sql.Open("corm_insertid_test", "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	qb := builder.NewAPI(dialect.MustGet("postgres"), db)
	id, err := qb.Insert("users").Columns("name").Values("alice").ExecAndReturnID(context.Background(), "id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Fatalf("want 42, got %d", id)
	}
}
