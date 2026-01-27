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

// SelectBuilder builds SELECT statements.
type SelectBuilder struct {
	exec    executor
	d       dialect.Dialect
	columns []string
	table   string
	joins   []string
	where   []clause.Expr
	groupBy []string
	having  []clause.Expr
	orderBy []string
	limit   *int
	offset  *int
}

func newSelect(exec executor, d dialect.Dialect, columns []string) *SelectBuilder {
	return &SelectBuilder{exec: exec, d: d, columns: columns}
}

// From sets the table to select from.
func (b *SelectBuilder) From(table string) *SelectBuilder {
	b.table = table
	return b
}

// Where adds a WHERE condition.
// It supports "id = ?", 1 or "name = ?", "alice".
// Do not concatenate untrusted user input into the SQL string; use args for parameter binding.
func (b *SelectBuilder) Where(sql string, args ...any) *SelectBuilder {
	b.where = append(b.where, clause.Raw(sql, args...))
	return b
}

// WhereIn adds a WHERE IN condition.
// It automatically handles slice arguments.
// Example: WhereIn("id", []int{1, 2, 3})
func (b *SelectBuilder) WhereIn(column string, args ...any) *SelectBuilder {
	return b.WhereExpr(clause.In(column, args...))
}

// WhereExpr adds a clause.Expr as a WHERE condition.
func (b *SelectBuilder) WhereExpr(e clause.Expr) *SelectBuilder {
	if strings.TrimSpace(e.SQL) == "" {
		return b
	}
	b.where = append(b.where, e)
	return b
}

// Join adds a JOIN clause.
// Do not concatenate untrusted user input into the joinSQL string.
func (b *SelectBuilder) Join(joinSQL string) *SelectBuilder {
	if strings.TrimSpace(joinSQL) == "" {
		return b
	}
	b.joins = append(b.joins, joinSQL)
	return b
}

// GroupBy adds a GROUP BY clause.
func (b *SelectBuilder) GroupBy(columns ...string) *SelectBuilder {
	b.groupBy = append(b.groupBy, columns...)
	return b
}

// Having adds a HAVING condition.
func (b *SelectBuilder) Having(sql string, args ...any) *SelectBuilder {
	b.having = append(b.having, clause.Raw(sql, args...))
	return b
}

// OrderBy adds an ORDER BY clause.
// dir must be "ASC" or "DESC".
// Do not pass untrusted user input as column/dir.
func (b *SelectBuilder) OrderBy(column, dir string) *SelectBuilder {
	dir = strings.TrimSpace(strings.ToUpper(dir))
	switch dir {
	case "ASC", "DESC":
	default:
		dir = "ASC"
	}
	b.orderBy = append(b.orderBy, quoteMaybe(b.d, column)+" "+dir)
	return b
}

// OrderByAsc adds an ORDER BY ASC clause.
func (b *SelectBuilder) OrderByAsc(column string) *SelectBuilder {
	return b.OrderBy(column, "ASC")
}

// OrderByDesc adds an ORDER BY DESC clause.
func (b *SelectBuilder) OrderByDesc(column string) *SelectBuilder {
	return b.OrderBy(column, "DESC")
}

// Limit sets the LIMIT.
func (b *SelectBuilder) Limit(limit int) *SelectBuilder {
	if limit < 0 {
		limit = 0
	}
	b.limit = &limit
	return b
}

// LimitOffset sets LIMIT and OFFSET in SQL order.
func (b *SelectBuilder) LimitOffset(limit, offset int) *SelectBuilder {
	return b.Limit(limit).Offset(offset)
}

// Offset sets the OFFSET.
func (b *SelectBuilder) Offset(offset int) *SelectBuilder {
	if offset < 0 {
		offset = 0
	}
	b.offset = &offset
	return b
}

// SQL generates the SQL query and arguments.
func (b *SelectBuilder) SQL() (string, []any, error) {
	if strings.TrimSpace(b.table) == "" {
		return "", nil, errors.New("corm: missing table for select")
	}

	var buf bytes.Buffer
	buf.Grow(128)
	argIndex := 1

	buf.WriteString("SELECT ")
	if len(b.columns) == 0 {
		buf.WriteString("*")
	} else {
		for i, c := range b.columns {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(quoteMaybe(b.d, c))
		}
	}
	buf.WriteString(" FROM ")
	buf.WriteString(quoteMaybe(b.d, b.table))

	for _, j := range b.joins {
		buf.WriteString(" ")
		buf.WriteString(j)
	}

	var args []any
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

	if len(b.groupBy) > 0 {
		buf.WriteString(" GROUP BY ")
		for i, c := range b.groupBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(quoteMaybe(b.d, c))
		}
	}

	if len(b.having) > 0 {
		buf.WriteString(" HAVING ")
		for i, h := range b.having {
			if i > 0 {
				buf.WriteString(" AND ")
			}
			buf.WriteString("(")
			part, next := b.d.RewritePlaceholders(h.SQL, argIndex)
			buf.WriteString(part)
			buf.WriteString(")")
			args = append(args, h.Args...)
			argIndex = next
		}
	}

	if len(b.orderBy) > 0 {
		buf.WriteString(" ORDER BY ")
		for i, o := range b.orderBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(o)
		}
	}

	if b.limit != nil {
		buf.WriteString(" LIMIT ")
		buf.WriteString(b.d.Placeholder(argIndex))
		args = append(args, *b.limit)
		argIndex++
	}
	if b.offset != nil {
		buf.WriteString(" OFFSET ")
		buf.WriteString(b.d.Placeholder(argIndex))
		args = append(args, *b.offset)
		argIndex++
	}

	return buf.String(), args, nil
}

func (b *SelectBuilder) Query(ctx context.Context) (*sql.Rows, error) {
	sqlStr, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return b.exec.QueryContext(ctx, sqlStr, args...)
}
