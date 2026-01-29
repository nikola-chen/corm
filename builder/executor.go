package builder

import (
	"context"
	"database/sql"
)

// Executor defines the interface for executing SQL queries.
// It is compatible with *sql.DB, *sql.Tx, and *sql.Conn.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}
