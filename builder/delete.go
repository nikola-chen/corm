package builder

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"

	"corm/clause"
	"corm/dialect"
)

// DeleteBuilder builds DELETE statements.
type DeleteBuilder struct {
	exec  executor
	d     dialect.Dialect
	table string
	where []clause.Expr
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

// WhereIn adds a WHERE IN condition.
func (b *DeleteBuilder) WhereIn(column string, args ...any) *DeleteBuilder {
	return b.WhereExpr(clause.In(column, args...))
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
	if strings.TrimSpace(b.table) == "" {
		return "", nil, errors.New("corm: missing table for delete")
	}

	var args []any
	var buf bytes.Buffer
	buf.Grow(96)
	argIndex := 1

	buf.WriteString("DELETE FROM ")
	buf.WriteString(quoteMaybe(b.d, b.table))

	if len(b.where) > 0 {
		buf.WriteString(" WHERE ")
		for i, w := range b.where {
			if i > 0 {
				buf.WriteString(" AND ")
			}
			buf.WriteString("(")
			part, next := b.d.RewritePlaceholders(w.SQL, argIndex)
			buf.WriteString(part)
			buf.WriteString(")")
			args = append(args, w.Args...)
			argIndex = next
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
