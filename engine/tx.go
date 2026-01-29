package engine

import (
	"context"
	"database/sql"
	"fmt"

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

func (t *Tx) rollbackTo(ctx context.Context, name string) error {
	_, err := t.tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+name)
	return err
}

func (t *Tx) release(ctx context.Context, name string) error {
	// MSSQL doesn't support RELEASE SAVEPOINT, but others do.
	// If we wanted to support MSSQL, we would check dialect.
	// For now, standard RELEASE.
	_, err := t.tx.ExecContext(ctx, "RELEASE SAVEPOINT "+name)
	return err
}

func (t *Tx) Select(columns ...string) *builder.SelectBuilder {
	return builder.Select(t.executor(), t.dialect, columns...)
}

func (t *Tx) SelectExpr(columns ...clause.Expr) *builder.SelectBuilder {
	return builder.Select(t.executor(), t.dialect).SelectExpr(columns...)
}

func (t *Tx) Insert(table string) *builder.InsertBuilder {
	return builder.Insert(t.executor(), t.dialect, table)
}

func (t *Tx) Update(table string) *builder.UpdateBuilder {
	return builder.Update(t.executor(), t.dialect, table)
}

func (t *Tx) Delete(table string) *builder.DeleteBuilder {
	return builder.Delete(t.executor(), t.dialect, table)
}

// Builder returns a builder.API that is pre-bound to this Tx's dialect and executor.
func (t *Tx) Builder() *builder.API {
	return builder.NewAPI(t.dialect, t.executor())
}
