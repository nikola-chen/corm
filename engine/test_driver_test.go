package engine

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
	"testing"

	"github.com/nikola-chen/corm/dialect"
)

const engineTestDriverName = "corm_engine_test_driver"

var registerEngineTestDriverOnce sync.Once

type engineTestDriver struct{}

func (d engineTestDriver) Open(name string) (driver.Conn, error) {
	return &engineTestConn{dbName: name}, nil
}

type engineTestConn struct {
	dbName string
}

func (c *engineTestConn) Prepare(query string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *engineTestConn) Close() error                              { return nil }
func (c *engineTestConn) Begin() (driver.Tx, error)                 { return &engineTestTx{}, nil }

func (c *engineTestConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return &engineTestResult{}, nil
}

func (c *engineTestConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return &engineTestRows{
		cols: []string{"id", "name"},
		data: [][]driver.Value{{int64(1), "test"}},
	}, nil
}

var _ driver.ExecerContext = (*engineTestConn)(nil)
var _ driver.QueryerContext = (*engineTestConn)(nil)
var _ driver.ConnBeginTx = (*engineTestConn)(nil)

func (c *engineTestConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return &engineTestTx{}, nil
}

type engineTestTx struct{}

func (tx *engineTestTx) Commit() error   { return nil }
func (tx *engineTestTx) Rollback() error { return nil }

type engineTestResult struct{}

func (r *engineTestResult) LastInsertId() (int64, error) { return 0, nil }
func (r *engineTestResult) RowsAffected() (int64, error) { return 1, nil }

type engineTestRows struct {
	cols []string
	data [][]driver.Value
	i    int
}

func (r *engineTestRows) Columns() []string { return r.cols }
func (r *engineTestRows) Close() error      { return nil }

func (r *engineTestRows) Next(dest []driver.Value) error {
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

func openEngineTestDB(t *testing.T) *sql.DB {
	t.Helper()
	registerEngineTestDriverOnce.Do(func() {
		sql.Register(engineTestDriverName, engineTestDriver{})
		dialect.Register(engineTestDriverName, dialect.MustGet("mysql"))
	})
	db, err := sql.Open(engineTestDriverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
