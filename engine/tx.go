package engine

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/dialect"
)

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
func (t *Tx) Transaction(ctx context.Context, fn func(*Tx) error) (err error) {
	t.savepointSeq++
	name := fmt.Sprintf("sp_%d", t.savepointSeq)

	// Note: Not all databases support SAVEPOINT.
	// We assume standard SQL behavior (Postgres, MySQL, SQLite).
	if _, err := t.tx.ExecContext(ctx, "SAVEPOINT "+name); err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = t.rollbackTo(ctx, name)
			panic(p)
		} else if err != nil {
			_ = t.rollbackTo(ctx, name)
		} else {
			_ = t.release(ctx, name)
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

func (t *Tx) SelectColumns(columns []string) *builder.SelectBuilder {
	return builder.Select(t.executor(), t.dialect, columns...)
}

func (t *Tx) InsertInto(table string) *builder.InsertBuilder {
	return builder.InsertInto(t.executor(), t.dialect, table)
}

func (t *Tx) Update(table string) *builder.UpdateBuilder {
	return builder.Update(t.executor(), t.dialect, table)
}

func (t *Tx) DeleteFrom(table string) *builder.DeleteBuilder {
	return builder.DeleteFrom(t.executor(), t.dialect, table)
}
