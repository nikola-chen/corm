package scan_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
	"testing"

	"github.com/nikola-chen/corm/scan"
)

const scanTestDriverName = "corm_scan_test_driver"

var registerScanTestDriverOnce sync.Once

type testDriver struct{}

func (d testDriver) Open(name string) (driver.Conn, error) {
	return &testConn{}, nil
}

type testConn struct{}

func (c *testConn) Prepare(query string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *testConn) Close() error                              { return nil }
func (c *testConn) Begin() (driver.Tx, error)                 { return nil, driver.ErrSkip }

func (c *testConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch query {
	case "struct_two_rows":
		return &testRows{
			cols: []string{"users.id", "`name`", "\"email\""},
			data: [][]driver.Value{
				{int64(1), "alice", nil},
				{int64(2), "bob", "bob@example.com"},
			},
		}, nil
	case "struct_dup_cols":
		return &testRows{
			cols: []string{"u.id", "o.id"},
			data: [][]driver.Value{
				{int64(1), int64(2)},
			},
		}, nil
	case "one_row":
		return &testRows{
			cols: []string{"id", "name"},
			data: [][]driver.Value{
				{int64(7), "neo"},
			},
		}, nil
	case "map_two_rows":
		return &testRows{
			cols: []string{"id", "score"},
			data: [][]driver.Value{
				{int64(1), int64(10)},
				{int64(2), int64(20)},
			},
		}, nil
	case "empty":
		return &testRows{
			cols: []string{"id"},
			data: nil,
		}, nil
	default:
		return &testRows{
			cols: []string{"id"},
			data: [][]driver.Value{{int64(1)}},
		}, nil
	}
}

var _ driver.QueryerContext = (*testConn)(nil)

type testRows struct {
	cols []string
	data [][]driver.Value
	i    int
}

func (r *testRows) Columns() []string { return r.cols }
func (r *testRows) Close() error      { return nil }

func (r *testRows) Next(dest []driver.Value) error {
	if r.i >= len(r.data) {
		return io.EOF
	}
	row := r.data[r.i]
	r.i++
	for i := range dest {
		dest[i] = row[i]
	}
	return nil
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	registerScanTestDriverOnce.Do(func() {
		sql.Register(scanTestDriverName, testDriver{})
	})
	db, err := sql.Open(scanTestDriverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type User struct {
	ID    int            `db:"id"`
	Name  string         `db:"name"`
	Email sql.NullString `db:"email"`
}

func TestScanAllStruct(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "struct_two_rows")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	var out []User
	if err := scan.ScanAll(rows, &out); err != nil {
		t.Fatalf("ScanAll: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out)=%d", len(out))
	}
	if out[0].ID != 1 || out[0].Name != "alice" || out[0].Email.Valid {
		t.Fatalf("row0=%+v", out[0])
	}
	if out[1].ID != 2 || out[1].Name != "bob" || !out[1].Email.Valid || out[1].Email.String != "bob@example.com" {
		t.Fatalf("row1=%+v", out[1])
	}
}

func TestScanAllStructPtrSlice(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "struct_two_rows")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	var out []*User
	if err := scan.ScanAll(rows, &out); err != nil {
		t.Fatalf("ScanAll: %v", err)
	}
	if len(out) != 2 || out[0] == nil || out[1] == nil {
		t.Fatalf("out=%v", out)
	}
	if out[0].ID != 1 || out[1].ID != 2 {
		t.Fatalf("ids=%d,%d", out[0].ID, out[1].ID)
	}
}

func TestScanOneStructAllocatesPtr(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "one_row")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	var u *struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	if err := scan.ScanOne(rows, &u); err != nil {
		t.Fatalf("ScanOne: %v", err)
	}
	if u == nil || u.ID != 7 || u.Name != "neo" {
		t.Fatalf("u=%+v", u)
	}
}

func TestScanAllMap(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "map_two_rows")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	var out []map[string]any
	if err := scan.ScanAll(rows, &out); err != nil {
		t.Fatalf("ScanAll: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out)=%d", len(out))
	}
	if out[0]["id"] != int64(1) || out[0]["score"] != int64(10) {
		t.Fatalf("row0=%v", out[0])
	}
}

func TestScanOneNoRows(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "empty")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	var u User
	err = scan.ScanOne(rows, &u)
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestScanAllStrictDuplicateColumns(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "struct_dup_cols")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	var out []User
	if err := scan.ScanAllStrict(rows, &out); err == nil {
		t.Fatalf("expected error")
	}
}
