package builder

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

// DeleteBuilder builds DELETE statements.
type DeleteBuilder struct {
	exec  executor
	d     dialect.Dialect
	table string
	where []clause.Expr
	err   error
}

func newDelete(exec executor, d dialect.Dialect, table string) *DeleteBuilder {
	return &DeleteBuilder{exec: exec, d: d, table: table}
}

// Where adds a WHERE condition.
// Do not concatenate untrusted user input into the SQL string; use args for parameter binding.
func (b *DeleteBuilder) Where(sql string, args ...any) *DeleteBuilder {
	b.where = append(b.where, clause.Raw(sql, args...))
	return b
}

func (b *DeleteBuilder) WhereRaw(sql string, args ...any) *DeleteBuilder {
	return b.Where(sql, args...)
}

func (b *DeleteBuilder) WhereEq(column string, value any) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	col, ok := quoteIdentStrict(b.d, column)
	if !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	return b.Where(col+" = ?", value)
}

// WhereIn adds a WHERE IN condition.
func (b *DeleteBuilder) WhereIn(column string, args ...any) *DeleteBuilder {
	return b.WhereExpr(clause.In(column, args...))
}

func (b *DeleteBuilder) WhereInIdent(column string, args ...any) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	col, ok := quoteIdentStrict(b.d, column)
	if !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	return b.WhereExpr(clause.In(col, args...))
}

func (b *DeleteBuilder) WhereLike(column string, value any) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	col, ok := quoteIdentStrict(b.d, column)
	if !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	return b.Where(col+" LIKE ?", value)
}

// WhereSubquery adds a condition with a subquery: "column op (subquery)".
func (b *DeleteBuilder) WhereSubquery(column, op string, sub *SelectBuilder) *DeleteBuilder {
	if b.err != nil {
		return b
	}
	sqlStr, args, err := sub.sqlRaw()
	if err != nil {
		b.err = err
		return b
	}
	return b.Where(column+" "+op+" ("+sqlStr+")", args...)
}

// WhereInSubquery adds a "column IN (subquery)" condition.
func (b *DeleteBuilder) WhereInSubquery(column string, sub *SelectBuilder) *DeleteBuilder {
	return b.WhereSubquery(column, "IN", sub)
}

// WhereExpr adds a clause.Expr as a WHERE condition.
func (b *DeleteBuilder) WhereExpr(e clause.Expr) *DeleteBuilder {
	if strings.TrimSpace(e.SQL) == "" {
		return b
	}
	b.where = append(b.where, e)
	return b
}

// SQL generates the SQL query and arguments.
func (b *DeleteBuilder) SQL() (string, []any, error) {
	sqlStr, args, err := b.sqlRaw()
	if err != nil {
		return "", nil, err
	}
	rewritten, _ := b.d.RewritePlaceholders(sqlStr, 1)
	return rewritten, args, nil
}

func (b *DeleteBuilder) sqlRaw() (string, []any, error) {
	if b.err != nil {
		return "", nil, b.err
	}
	if strings.TrimSpace(b.table) == "" {
		return "", nil, errors.New("corm: missing table for delete")
	}

	var args []any
	var buf bytes.Buffer
	buf.Grow(96)

	buf.WriteString("DELETE FROM ")
	buf.WriteString(quoteMaybe(b.d, b.table))

	if len(b.where) > 0 {
		buf.WriteString(" WHERE ")
		wrote := 0
		for _, w := range b.where {
			if strings.TrimSpace(w.SQL) == "" {
				continue
			}
			if wrote > 0 {
				buf.WriteString(" AND ")
			}
			buf.WriteString("(")
			buf.WriteString(w.SQL)
			buf.WriteString(")")
			args = append(args, w.Args...)
			wrote++
		}
		if wrote == 0 {
			buf.Truncate(buf.Len() - len(" WHERE "))
		}
	}

	return buf.String(), args, nil
}

func (b *DeleteBuilder) Exec(ctx context.Context) (sql.Result, error) {
	sqlStr, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return b.exec.ExecContext(ctx, sqlStr, args...)
}
