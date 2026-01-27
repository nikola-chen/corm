package builder

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"

	"corm/dialect"
	"corm/schema"
)

// InsertBuilder builds INSERT statements.
type InsertBuilder struct {
	exec      executor
	d         dialect.Dialect
	table     string
	columns   []string
	rows      [][]any
	returning []string
	err       error

	includePrimaryKey bool
	includeAuto       bool
	includeReadonly   bool
	includeZero       bool
}

func newInsert(exec executor, d dialect.Dialect, table string) *InsertBuilder {
	return &InsertBuilder{exec: exec, d: d, table: table}
}

// IncludePrimaryKey includes primary key fields in the INSERT statement.
func (b *InsertBuilder) IncludePrimaryKey() *InsertBuilder {
	b.includePrimaryKey = true
	return b
}

// IncludeAuto includes auto-increment/generated fields in the INSERT statement.
func (b *InsertBuilder) IncludeAuto() *InsertBuilder {
	b.includeAuto = true
	return b
}

// IncludeReadonly includes read-only fields in the INSERT statement.
func (b *InsertBuilder) IncludeReadonly() *InsertBuilder {
	b.includeReadonly = true
	return b
}

// IncludeZero includes zero-value fields in the INSERT statement.
func (b *InsertBuilder) IncludeZero() *InsertBuilder {
	b.includeZero = true
	return b
}

// Model adds a struct model to be inserted.
func (b *InsertBuilder) Model(dest any) *InsertBuilder {
	s, err := schema.Parse(dest)
	if err != nil {
		b.err = err
		return b
	}
	if b.table == "" {
		b.table = s.Table
	}

	if len(b.columns) > 0 {
		cols, vals, err := s.ColumnsAndValues(dest, schema.ExtractOptions{
			IncludePrimaryKey: true,
			IncludeAuto:       true,
			IncludeReadonly:   true,
			IncludeZero:       true,
		})
		if err != nil {
			b.err = err
			return b
		}
		byCol := make(map[string]any, len(cols))
		for i := range cols {
			byCol[strings.ToLower(cols[i])] = vals[i]
		}
		row := make([]any, 0, len(b.columns))
		for _, col := range b.columns {
			v, ok := byCol[strings.ToLower(col)]
			if !ok {
				b.err = errors.New("corm: unknown column in Model: " + col)
				return b
			}
			row = append(row, v)
		}
		b.rows = append(b.rows, row)
		return b
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
	b.columns = append(b.columns, cols...)
	b.rows = append(b.rows, vals)
	return b
}

// Columns sets the columns to insert.
func (b *InsertBuilder) Columns(cols ...string) *InsertBuilder {
	b.columns = append(b.columns, cols...)
	return b
}

// Values adds a row of values to insert.
func (b *InsertBuilder) Values(values ...any) *InsertBuilder {
	b.rows = append(b.rows, values)
	return b
}

// Returning adds a RETURNING clause.
func (b *InsertBuilder) Returning(cols ...string) *InsertBuilder {
	b.returning = append(b.returning, cols...)
	return b
}

// SQL generates the SQL query and arguments.
func (b *InsertBuilder) SQL() (string, []any, error) {
	if b.err != nil {
		return "", nil, b.err
	}
	if strings.TrimSpace(b.table) == "" {
		return "", nil, errors.New("corm: missing table for insert")
	}
	if len(b.rows) == 0 {
		return "", nil, errors.New("corm: missing values for insert")
	}
	if len(b.columns) == 0 {
		return "", nil, errors.New("corm: missing columns for insert")
	}

	var args []any
	var buf bytes.Buffer
	buf.Grow(128)
	argIndex := 1

	buf.WriteString("INSERT INTO ")
	buf.WriteString(quoteMaybe(b.d, b.table))
	buf.WriteString(" (")
	for i, c := range b.columns {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(quoteMaybe(b.d, c))
	}
	buf.WriteString(") VALUES ")

	for r, row := range b.rows {
		if len(row) != len(b.columns) {
			return "", nil, errors.New("corm: insert values length mismatch columns")
		}
		if r > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString("(")
		for i := range row {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(b.d.Placeholder(argIndex))
			argIndex++
		}
		buf.WriteString(")")
		args = append(args, row...)
	}

	if len(b.returning) > 0 && b.d.SupportsReturning() {
		buf.WriteString(" RETURNING ")
		for i, c := range b.returning {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(quoteMaybe(b.d, c))
		}
	}

	return buf.String(), args, nil
}

func (b *InsertBuilder) Exec(ctx context.Context) (sql.Result, error) {
	sqlStr, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return b.exec.ExecContext(ctx, sqlStr, args...)
}
