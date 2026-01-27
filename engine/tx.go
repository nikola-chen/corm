package engine

import (
	"context"
	"database/sql"

	"corm/builder"
	"corm/dialect"
)

type Tx struct {
	tx     *sql.Tx
	dialect dialect.Dialect
	logger Logger
	cfg    Config
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
