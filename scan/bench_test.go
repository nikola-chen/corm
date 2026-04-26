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

const benchDriverName = "corm_bench_scan_driver"

var registerBenchDriverOnce sync.Once

type benchDriver struct{}

func (d benchDriver) Open(name string) (driver.Conn, error) {
	return &benchConn{}, nil
}

type benchConn struct{}

func (c *benchConn) Prepare(query string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *benchConn) Close() error                              { return nil }
func (c *benchConn) Begin() (driver.Tx, error)                 { return nil, driver.ErrSkip }

func (c *benchConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return &benchRows{
		cols: []string{"id", "name", "email"},
		n:    100,
	}, nil
}

var _ driver.QueryerContext = (*benchConn)(nil)

type benchRows struct {
	cols []string
	n    int
	i    int
}

func (r *benchRows) Columns() []string { return r.cols }
func (r *benchRows) Close() error      { return nil }

func (r *benchRows) Next(dest []driver.Value) error {
	if r.i >= r.n {
		return io.EOF
	}
	dest[0] = int64(r.i + 1)
	dest[1] = "user_name"
	dest[2] = "user@example.com"
	r.i++
	return nil
}

type BenchScanUser struct {
	ID    int            `db:"id"`
	Name  string         `db:"name"`
	Email sql.NullString `db:"email"`
}

func openBenchDB(b *testing.B) *sql.DB {
	b.Helper()
	registerBenchDriverOnce.Do(func() {
		sql.Register(benchDriverName, benchDriver{})
	})
	db, err := sql.Open(benchDriverName, "")
	if err != nil {
		b.Fatalf("sql.Open: %v", err)
	}
	b.Cleanup(func() { _ = db.Close() })
	return db
}

func BenchmarkScanAllStruct(b *testing.B) {
	db := openBenchDB(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := db.QueryContext(context.Background(), "bench")
		if err != nil {
			b.Fatal(err)
		}
		var out []BenchScanUser
		if err := scan.ScanAll(rows, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScanOneStruct(b *testing.B) {
	db := openBenchDB(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := db.QueryContext(context.Background(), "bench")
		if err != nil {
			b.Fatal(err)
		}
		var u BenchScanUser
		if err := scan.ScanOne(rows, &u); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScanAllMap(b *testing.B) {
	db := openBenchDB(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := db.QueryContext(context.Background(), "bench")
		if err != nil {
			b.Fatal(err)
		}
		var out []map[string]any
		if err := scan.ScanAll(rows, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScanAllStructParallel(b *testing.B) {
	db := openBenchDB(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rows, err := db.QueryContext(context.Background(), "bench")
			if err != nil {
				b.Fatal(err)
			}
			var out []BenchScanUser
			if err := scan.ScanAll(rows, &out); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkIterStruct(b *testing.B) {
	db := openBenchDB(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := db.QueryContext(context.Background(), "bench")
		if err != nil {
			b.Fatal(err)
		}
		for range scan.Iter[BenchScanUser](rows) {
		}
	}
}
