package builder_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
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

func TestAssignInt64(t *testing.T) {
	var i int
	if err := assignInt64(&i, 42); err != nil {
		t.Fatalf("int: %v", err)
	}
	if i != 42 {
		t.Fatalf("want 42, got %d", i)
	}

	var i64 int64
	if err := assignInt64(&i64, 100); err != nil {
		t.Fatalf("int64: %v", err)
	}
	if i64 != 100 {
		t.Fatalf("want 100, got %d", i64)
	}

	var u uint
	if err := assignInt64(&u, 7); err != nil {
		t.Fatalf("uint: %v", err)
	}
	if u != 7 {
		t.Fatalf("want 7, got %d", u)
	}

	var u64 uint64
	if err := assignInt64(&u64, 99); err != nil {
		t.Fatalf("uint64: %v", err)
	}
	if u64 != 99 {
		t.Fatalf("want 99, got %d", u64)
	}

	var ni sql.NullInt64
	if err := assignInt64(&ni, 55); err != nil {
		t.Fatalf("NullInt64: %v", err)
	}
	if !ni.Valid || ni.Int64 != 55 {
		t.Fatalf("want valid=true, 55; got valid=%v, %d", ni.Valid, ni.Int64)
	}

	var i8 int8
	if err := assignInt64(&i8, 10); err != nil {
		t.Fatalf("int8: %v", err)
	}
	if i8 != 10 {
		t.Fatalf("want 10, got %d", i8)
	}

	var u16 uint16
	if err := assignInt64(&u16, 500); err != nil {
		t.Fatalf("uint16: %v", err)
	}
	if u16 != 500 {
		t.Fatalf("want 500, got %d", u16)
	}

	if err := assignInt64(nil, 1); err == nil {
		t.Fatal("expected error for nil dest")
	}

	if err := assignInt64(42, 1); err == nil {
		t.Fatal("expected error for non-pointer dest")
	}

	var s string
	if err := assignInt64(&s, 1); err == nil {
		t.Fatal("expected error for string dest")
	}

	var ui uint
	if err := assignInt64(&ui, -1); err == nil {
		t.Fatal("expected error for negative uint")
	}

	var small int8
	if err := assignInt64(&small, 9999); err == nil {
		t.Fatal("expected error for overflow int8")
	}
}

func assignInt64(dest any, v int64) error {
	switch p := dest.(type) {
	case *int:
		*p = int(v)
		return nil
	case *int64:
		*p = v
		return nil
	case *uint:
		if v < 0 {
			return errors.New("corm: negative value cannot be assigned to uint")
		}
		*p = uint(v)
		return nil
	case *uint64:
		if v < 0 {
			return errors.New("corm: negative value cannot be assigned to uint64")
		}
		*p = uint64(v)
		return nil
	case *sql.NullInt64:
		p.Int64 = v
		p.Valid = true
		return nil
	}

	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("corm: dest must be non-nil pointer")
	}
	ev := rv.Elem()
	switch ev.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if ev.OverflowInt(v) {
			return errors.New("corm: dest int overflow")
		}
		ev.SetInt(v)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if v < 0 || ev.OverflowUint(uint64(v)) {
			return errors.New("corm: dest uint overflow")
		}
		ev.SetUint(uint64(v))
		return nil
	default:
		return errors.New("corm: dest must be integer pointer")
	}
}
