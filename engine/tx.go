package engine

import (
	"context"
	"database/sql"
	"fmt"
	"unicode"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

// Tx wraps a database transaction.
//
// Tx is not safe for concurrent use by multiple goroutines.
type Tx struct {
	tx           *sql.Tx
	dialect      dialect.Dialect
	logger       Logger
	cfg          Config
	savepointSeq int
}

func (e *Engine) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := e.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx, dialect: e.dialect, logger: e.logger, cfg: e.cfg}, nil
}

func (t *Tx) Commit() error { return t.tx.Commit() }

func (t *Tx) Rollback() error { return t.tx.Rollback() }

// Transaction executes a function within a nested transaction (using SAVEPOINT).
//
// Tx is not safe for concurrent use. Do not call Transaction concurrently on the same Tx.
func (t *Tx) Transaction(ctx context.Context, fn func(*Tx) error) (err error) {
	t.savepointSeq++
	name := fmt.Sprintf("sp_%d", t.savepointSeq)

	if !isValidSavepointName(name) {
		return fmt.Errorf("corm: invalid savepoint name: %s", name)
	}

	// Note: Not all databases support SAVEPOINT.
	// We assume standard SQL behavior (Postgres, MySQL).
	if _, err := t.tx.ExecContext(ctx, "SAVEPOINT "+name); err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := t.rollbackTo(ctx, name); rbErr != nil && t.logger != nil {
				t.logger.Printf("corm: rollback to savepoint failed name=%s err=%v", name, rbErr)
			}
			panic(p)
		} else if err != nil {
			if rbErr := t.rollbackTo(ctx, name); rbErr != nil && t.logger != nil {
				t.logger.Printf("corm: rollback to savepoint failed name=%s err=%v", name, rbErr)
			}
		} else {
			err = t.release(ctx, name)
		}
	}()

	return fn(t)
}

// isValidSavepointName validates that name is a valid SQL identifier.
// Savepoint names must start with a letter or underscore and contain only
// letters, digits, and underscores. Max length is 128 characters.
func isValidSavepointName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return false
			}
		}
	}
	return true
}

func (t *Tx) rollbackTo(ctx context.Context, name string) error {
	if !isValidSavepointName(name) {
		return fmt.Errorf("corm: invalid savepoint name: %s", name)
	}
	_, err := t.tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+name)
	return err
}

func (t *Tx) release(ctx context.Context, name string) error {
	// MSSQL doesn't support RELEASE SAVEPOINT, but others do.
	// If we wanted to support MSSQL, we would check dialect.
	// For now, standard RELEASE.
	if !isValidSavepointName(name) {
		return fmt.Errorf("corm: invalid savepoint name: %s", name)
	}
	_, err := t.tx.ExecContext(ctx, "RELEASE SAVEPOINT "+name)
	return err
}

func (t *Tx) Select(columns ...string) *builder.SelectBuilder {
	return builder.NewAPI(t.dialect, t.executor()).Select(columns...)
}

func (t *Tx) SelectExpr(columns ...clause.Expr) *builder.SelectBuilder {
	return builder.NewAPI(t.dialect, t.executor()).Select().SelectExpr(columns...)
}

func (t *Tx) Insert(table string) *builder.InsertBuilder {
	return builder.NewAPI(t.dialect, t.executor()).Insert(table)
}

func (t *Tx) Update(table string) *builder.UpdateBuilder {
	return builder.NewAPI(t.dialect, t.executor()).Update(table)
}

func (t *Tx) Delete(table string) *builder.DeleteBuilder {
	return builder.NewAPI(t.dialect, t.executor()).Delete(table)
}

// RawExec executes a raw SQL query and returns the result.
func (t *Tx) RawExec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, sqlStr, args...)
}

// RawQuery executes a raw SQL query and returns the rows.
func (t *Tx) RawQuery(ctx context.Context, sqlStr string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, sqlStr, args...)
}

// RawQueryFunc executes a raw SQL query and calls fn with the resulting rows.
// It ensures rows are properly closed after fn returns.
func (t *Tx) RawQueryFunc(ctx context.Context, sqlStr string, fn func(*sql.Rows) error, args ...any) error {
	rows, err := t.tx.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return fn(rows)
}

// Builder returns a builder.API that is pre-bound to this Tx's dialect and executor.
func (t *Tx) Builder() *builder.API {
	return builder.NewAPI(t.dialect, t.executor())
}
