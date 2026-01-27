package builder

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
	"github.com/nikola-chen/corm/schema"
)

type setItem struct {
	column string
	value  any
}

// UpdateBuilder builds UPDATE statements.
type UpdateBuilder struct {
	exec  executor
	d     dialect.Dialect
	table string
	sets  []setItem
	where []clause.Expr
	err   error

	includePrimaryKey bool
	includeAuto       bool
	includeReadonly   bool
	includeZero       bool
}

func newUpdate(exec executor, d dialect.Dialect, table string) *UpdateBuilder {
	return &UpdateBuilder{exec: exec, d: d, table: table}
}

// IncludePrimaryKey includes primary key fields in the UPDATE statement.
func (b *UpdateBuilder) IncludePrimaryKey() *UpdateBuilder {
	b.includePrimaryKey = true
	return b
}

// IncludeAuto includes auto-increment/generated fields in the UPDATE statement.
func (b *UpdateBuilder) IncludeAuto() *UpdateBuilder {
	b.includeAuto = true
	return b
}

// IncludeReadonly includes read-only fields in the UPDATE statement.
func (b *UpdateBuilder) IncludeReadonly() *UpdateBuilder {
	b.includeReadonly = true
	return b
}

// IncludeZero includes zero-value fields in the UPDATE statement.
func (b *UpdateBuilder) IncludeZero() *UpdateBuilder {
	b.includeZero = true
	return b
}

// SetStruct sets columns and values from a struct.
func (b *UpdateBuilder) SetStruct(dest any) *UpdateBuilder {
	s, err := schema.Parse(dest)
	if err != nil {
		b.err = err
		return b
	}
	if b.table == "" {
		b.table = s.Table
	}

	cols, vals, err := s.ColumnsAndValues(dest, schema.ExtractOptions{
		IncludePrimaryKey: b.includePrimaryKey,
		IncludeAuto:       b.includeAuto,
		IncludeReadonly:   b.includeReadonly,
		IncludeZero:       b.includeZero,
	})
	if err != nil {
		b.err = err
		return b
	}
	for i := range cols {
		b.Set(cols[i], vals[i])
	}
	return b
}

// Set sets a column to a value.
func (b *UpdateBuilder) Set(column string, value any) *UpdateBuilder {
	b.sets = append(b.sets, setItem{column: column, value: value})
	return b
}

// SetMap sets columns and values from a map.
func (b *UpdateBuilder) SetMap(values map[string]any) *UpdateBuilder {
	for k, v := range values {
		b.sets = append(b.sets, setItem{column: k, value: v})
	}
	return b
}

// Where adds a WHERE condition.
// Do not concatenate untrusted user input into the SQL string; use args for parameter binding.
func (b *UpdateBuilder) Where(sql string, args ...any) *UpdateBuilder {
	b.where = append(b.where, clause.Raw(sql, args...))
	return b
}

// WhereIn adds a WHERE IN condition.
func (b *UpdateBuilder) WhereIn(column string, args ...any) *UpdateBuilder {
	return b.WhereExpr(clause.In(column, args...))
}


// WhereExpr adds a clause.Expr as a WHERE condition.
func (b *UpdateBuilder) WhereExpr(e clause.Expr) *UpdateBuilder {
	if strings.TrimSpace(e.SQL) == "" {
		return b
	}
	b.where = append(b.where, e)
	return b
}

// SQL generates the SQL query and arguments.
func (b *UpdateBuilder) SQL() (string, []any, error) {
	if b.err != nil {
		return "", nil, b.err
	}
	if strings.TrimSpace(b.table) == "" {
		return "", nil, errors.New("corm: missing table for update")
	}
	if len(b.sets) == 0 {
		return "", nil, errors.New("corm: missing set values for update")
	}

	var args []any
	var buf bytes.Buffer
	buf.Grow(128)
	argIndex := 1

	buf.WriteString("UPDATE ")
	buf.WriteString(quoteMaybe(b.d, b.table))
	buf.WriteString(" SET ")

	for i, s := range b.sets {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(quoteMaybe(b.d, s.column))
		buf.WriteString(" = ")
		buf.WriteString(b.d.Placeholder(argIndex))
		argIndex++
		args = append(args, s.value)
	}

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

func (b *UpdateBuilder) Exec(ctx context.Context) (sql.Result, error) {
	sqlStr, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return b.exec.ExecContext(ctx, sqlStr, args...)
}
