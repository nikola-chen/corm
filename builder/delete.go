package builder

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

// DeleteBuilder builds DELETE statements.
type DeleteBuilder struct {
	exec            Executor
	d               dialect.Dialect
	table           string
	where           whereBuilder
	allowEmptyWhere bool
	limit           *int
	err             error
}

const (
	deleteWhereExpr = iota
	deleteWhereSubquery
)

func newDelete(exec Executor, d dialect.Dialect, table string) *DeleteBuilder {
	table = strings.TrimSpace(table)
	b := &DeleteBuilder{exec: exec, d: d, table: table, where: whereBuilder{d: d}}
	if table != "" && d != nil {
		if _, ok := quoteIdentStrict(d, table); !ok {
			b.err = errors.New("corm: invalid table identifier")
		}
	}
	return b
}

func (b *DeleteBuilder) AllowEmptyWhere() *DeleteBuilder {
	b.allowEmptyWhere = true
	return b
}

// Where adds a WHERE condition.
// Do not concatenate untrusted user input into the SQL string; use args for parameter binding.
func (b *DeleteBuilder) Where(sql string, args ...any) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	b.where.Where(sql, args...)
	return b
}

func (b *DeleteBuilder) WhereEq(column string, value any) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereEq(column, value)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereIn adds a WHERE IN condition.
func (b *DeleteBuilder) WhereIn(column string, args ...any) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereIn(column, args...)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

func (b *DeleteBuilder) WhereLike(column string, value any) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereLike(column, value)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereMap adds conditions in the form of "column = ?" joined by AND.
// Keys are applied in sorted order to keep the generated SQL deterministic.
func (b *DeleteBuilder) WhereMap(conditions map[string]any) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereMap(conditions)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereSubquery adds a condition with a subquery: "column op (subquery)".
func (b *DeleteBuilder) WhereSubquery(column, op string, sub *SelectBuilder) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereSubquery(column, op, sub)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereInSubquery adds a "column IN (subquery)" condition.
func (b *DeleteBuilder) WhereInSubquery(column string, sub *SelectBuilder) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereSubquery(column, "IN", sub)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereExpr adds a clause.Expr as a WHERE condition.
func (b *DeleteBuilder) WhereExpr(e clause.Expr) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereExpr(e)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// SQL generates the SQL query and arguments.
func (b *DeleteBuilder) SQL() (string, []any, error) {
	if b.err != nil {
		return "", nil, b.err
	}
	if b.d == nil {
		return "", nil, errors.New("corm: nil dialect")
	}
	if strings.TrimSpace(b.table) == "" {
		return "", nil, errors.New("corm: missing table for delete")
	}

	buf := getBuffer()
	defer putBuffer(buf)
	ab := newArgBuilder(b.d, 1)

	buf.WriteString("DELETE FROM ")
	qTable, ok := quoteIdentStrict(b.d, b.table)
	if !ok {
		return "", nil, errors.New("corm: invalid table identifier")
	}
	buf.WriteString(qTable)

	if err := b.where.appendWhere(buf, ab); err != nil {
		return "", nil, err
	}
	if !b.allowEmptyWhere && len(b.where.items) == 0 {
		return "", nil, errors.New("corm: delete without where clause is not allowed, use AllowEmptyWhere() to override")
	}

	if b.limit != nil {
		if *b.limit < 0 {
			return "", nil, errors.New("corm: invalid limit")
		}
		if b.d.Name() != "mysql" {
			return "", nil, errors.New("corm: delete limit is only supported by mysql dialect")
		}
		buf.WriteString(" LIMIT ")
		buf.WriteString(ab.add(*b.limit))
	}

	return buf.String(), ab.args, nil
}

// Limit adds a LIMIT clause.
// Note: LIMIT on DELETE is not standard SQL and may not be supported by all dialects.
func (b *DeleteBuilder) Limit(limit int) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	b.limit = &limit
	return b
}

func (b *DeleteBuilder) Exec(ctx context.Context) (sql.Result, error) {
	if b.exec == nil {
		return nil, errors.New("corm: missing Executor for delete")
	}
	sqlStr, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return b.exec.ExecContext(ctx, sqlStr, args...)
}
