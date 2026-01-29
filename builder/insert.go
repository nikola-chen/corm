package builder

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
	"github.com/nikola-chen/corm/schema"
)

// InsertBuilder builds INSERT statements.
type InsertBuilder struct {
	exec      Executor
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

	fromSelect *SelectBuilder
	suffix     []clause.Expr
}

func newInsert(exec Executor, d dialect.Dialect, table string) *InsertBuilder {
	return &InsertBuilder{exec: exec, d: d, table: table}
}

// SuffixRaw appends a raw SQL suffix to the INSERT statement (e.g., ON CONFLICT ...).
// Do not concatenate untrusted user input into the SQL string; use args for parameter binding.
func (b *InsertBuilder) SuffixRaw(sql string, args ...any) *InsertBuilder {
	if b.err != nil {
		return b
	}
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return b
	}
	b.suffix = append(b.suffix, clause.Raw(sql, args...))
	return b
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

// FromSelect sets a SELECT statement to insert from.
func (b *InsertBuilder) FromSelect(sb *SelectBuilder) *InsertBuilder {
	b.fromSelect = sb
	return b
}

// Model adds a struct model to be inserted.
func (b *InsertBuilder) Model(dest any) *InsertBuilder {
	if b.err != nil {
		return b
	}
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
			IncludePrimaryKey: b.includePrimaryKey,
			IncludeAuto:       b.includeAuto,
			IncludeReadonly:   b.includeReadonly,
			IncludeZero:       b.includeZero,
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

// Map appends a single row from a map.
//
// If Columns(...) was called, Map follows the predefined column order and will
// attempt a case-insensitive key match (useful for JSON keys).
//
// If Columns(...) was not called, Map derives columns from map keys in sorted order
// to keep the generated SQL deterministic.
func (b *InsertBuilder) Map(values map[string]any) *InsertBuilder {
	if b.err != nil {
		return b
	}
	if len(values) == 0 {
		return b
	}

	if len(b.columns) > 0 {
		row := make([]any, 0, len(b.columns))
		var normValues map[string]any

		for _, col := range b.columns {
			v, ok := values[col]
			if !ok {
				if normValues == nil {
					normValues = make(map[string]any, len(values))
					for k, v := range values {
						normValues[strings.ToLower(k)] = v
					}
				}
				v, ok = normValues[strings.ToLower(col)]
			}
			if !ok {
				b.err = errors.New("corm: missing value for column: " + col)
				return b
			}
			row = append(row, v)
		}
		b.rows = append(b.rows, row)
		return b
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	row := make([]any, 0, len(keys))
	for _, k := range keys {
		if _, ok := quoteColumnStrict(b.d, k); !ok {
			b.err = errors.New("corm: invalid column identifier")
			return b
		}
		b.columns = append(b.columns, k)
		row = append(row, values[k])
	}
	b.rows = append(b.rows, row)
	return b
}

// MapLowerKeys appends a single row from a map whose keys are already normalized to lower-case.
//
// When Columns(...) was called, it avoids per-row key normalization work and is faster than Map.
func (b *InsertBuilder) MapLowerKeys(values map[string]any) *InsertBuilder {
	if b.err != nil {
		return b
	}
	if len(values) == 0 {
		return b
	}
	if len(b.columns) == 0 {
		return b.Map(values)
	}

	row := make([]any, 0, len(b.columns))
	for _, col := range b.columns {
		v, ok := values[strings.ToLower(col)]
		if !ok {
			b.err = errors.New("corm: missing value for column: " + col)
			return b
		}
		row = append(row, v)
	}
	b.rows = append(b.rows, row)
	return b
}

// Maps appends multiple rows from a slice of maps.
// It is equivalent to calling Map(row) for each element.
func (b *InsertBuilder) Maps(rows []map[string]any) *InsertBuilder {
	if b.err != nil {
		return b
	}
	for _, row := range rows {
		b.Map(row)
	}
	return b
}

// MapsLowerKeys appends multiple rows from a slice of maps whose keys are already normalized to lower-case.
// It is equivalent to calling MapLowerKeys(row) for each element.
func (b *InsertBuilder) MapsLowerKeys(rows []map[string]any) *InsertBuilder {
	if b.err != nil {
		return b
	}
	for _, row := range rows {
		b.MapLowerKeys(row)
	}
	return b
}

// Columns sets the columns to insert.
func (b *InsertBuilder) Columns(cols ...string) *InsertBuilder {
	if b.err != nil {
		return b
	}
	for _, c := range cols {
		if _, ok := quoteColumnStrict(b.d, c); !ok {
			b.err = errors.New("corm: invalid column identifier")
			return b
		}
		b.columns = append(b.columns, c)
	}
	return b
}

// Values adds a row of values to insert.
func (b *InsertBuilder) Values(values ...any) *InsertBuilder {
	if b.err != nil {
		return b
	}
	b.rows = append(b.rows, values)
	return b
}

// Returning adds a RETURNING clause.
func (b *InsertBuilder) Returning(cols ...string) *InsertBuilder {
	if b.err != nil {
		return b
	}
	for _, c := range cols {
		if _, ok := quoteColumnStrict(b.d, c); !ok {
			b.err = errors.New("corm: invalid column identifier")
			return b
		}
		b.returning = append(b.returning, c)
	}
	return b
}

// SQL generates the SQL query and arguments.
func (b *InsertBuilder) SQL() (string, []any, error) {
	if b.err != nil {
		return "", nil, b.err
	}
	if b.d == nil {
		return "", nil, errors.New("corm: nil dialect")
	}
	buf := getBuffer()
	defer putBuffer(buf)
	ab := newArgBuilder(b.d, 1)
	if err := b.appendSQL(buf, ab); err != nil {
		return "", nil, err
	}
	return buf.String(), ab.args, nil
}

func (b *InsertBuilder) appendSQL(buf *bytes.Buffer, ab *argBuilder) error {
	if strings.TrimSpace(b.table) == "" {
		return errors.New("corm: missing table for insert")
	}
	if len(b.columns) == 0 {
		return errors.New("corm: missing columns for insert")
	}

	buf.WriteString("INSERT INTO ")
	qTable, ok := quoteIdentStrict(b.d, b.table)
	if !ok {
		return errors.New("corm: invalid table identifier")
	}
	buf.WriteString(qTable)
	buf.WriteString(" (")
	for i, c := range b.columns {
		if i > 0 {
			buf.WriteString(", ")
		}
		qCol, ok := quoteColumnStrict(b.d, c)
		if !ok {
			return errors.New("corm: invalid column identifier")
		}
		buf.WriteString(qCol)
	}
	buf.WriteString(")")

	if b.fromSelect != nil {
		buf.WriteString(" ")
		if err := b.fromSelect.appendSQL(buf, ab); err != nil {
			return err
		}
	} else {
		if len(b.rows) == 0 {
			return errors.New("corm: missing values for insert")
		}
		buf.WriteString(" VALUES ")
		for r, row := range b.rows {
			if len(row) != len(b.columns) {
				// To avoid panic if we used buf.WriteString(ab.add(row[i])) with wrong index
				// although here we iterate range row, so panic is unlikely unless we iterate b.columns
				// But logical mismatch is an error.
				return errors.New("corm: insert values length mismatch columns")
			}
			if r > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString("(")
			for i := range b.columns { // Iterate columns to ensure we match count and order
				if i > 0 {
					buf.WriteString(", ")
				}
				if i >= len(row) {
					return errors.New("corm: insert values length mismatch columns")
				}
				buf.WriteString(ab.add(row[i]))
			}
			buf.WriteString(")")
		}
	}

	if len(b.returning) > 0 && b.d.SupportsReturning() {
		buf.WriteString(" RETURNING ")
		for i, c := range b.returning {
			if i > 0 {
				buf.WriteString(", ")
			}
			qCol, ok := quoteColumnStrict(b.d, c)
			if !ok {
				return errors.New("corm: invalid column identifier")
			}
			buf.WriteString(qCol)
		}
	}

	for _, s := range b.suffix {
		if strings.TrimSpace(s.SQL) == "" {
			continue
		}
		buf.WriteString(" ")
		if err := ab.appendExpr(buf, s); err != nil {
			return err
		}
	}

	return nil
}

func (b *InsertBuilder) Exec(ctx context.Context) (sql.Result, error) {
	if b.exec == nil {
		return nil, errors.New("corm: missing Executor for insert")
	}
	sqlStr, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return b.exec.ExecContext(ctx, sqlStr, args...)
}
