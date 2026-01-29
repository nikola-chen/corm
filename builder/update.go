package builder

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
	"github.com/nikola-chen/corm/schema"
)

type setItem struct {
	column string
	value  any
}

const (
	updateWhereExpr = iota
	updateWhereSubquery
)

// UpdateBuilder builds UPDATE statements.
type UpdateBuilder struct {
	exec  Executor
	d     dialect.Dialect
	table string
	sets  []setItem
	where whereBuilder
	err   error

	includePrimaryKey bool
	includeAuto       bool
	includeReadonly   bool
	includeZero       bool

	allowEmptyWhere bool
	limit           *int

	batch *batchUpdateBuilder
}

func newUpdate(exec Executor, d dialect.Dialect, table string) *UpdateBuilder {
	table = strings.TrimSpace(table)
	b := &UpdateBuilder{exec: exec, d: d, table: table, where: whereBuilder{d: d}}
	if table != "" && d != nil {
		if _, ok := quoteIdentStrict(d, table); !ok {
			b.err = errors.New("corm: invalid table identifier")
		}
	}
	return b
}

// AllowEmptyWhere allows UPDATE without a WHERE clause.
func (b *UpdateBuilder) AllowEmptyWhere() *UpdateBuilder {
	b.allowEmptyWhere = true
	return b
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

func (b *UpdateBuilder) Model(dest any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	s, err := schema.Parse(dest)
	if err != nil {
		b.err = err
		return b
	}
	if b.table == "" {
		if strings.TrimSpace(s.Table) == "" {
			b.err = errors.New("corm: missing table for update: model has no table name")
			return b
		}
		if b.d != nil {
			if _, ok := quoteIdentStrict(b.d, s.Table); !ok {
				b.err = errors.New("corm: invalid table identifier from model")
				return b
			}
		}
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
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.err = errors.New("corm: cannot use Set on batch update, use Models/Maps")
		return b
	}
	if _, ok := quoteColumnStrict(b.d, column); !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	b.sets = append(b.sets, setItem{column: column, value: value})
	return b
}

// Increment increments a column by the specified amount.
// Example: Increment("count", 1) generates "count = count + ?"
func (b *UpdateBuilder) Increment(column string, amount any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.err = errors.New("corm: cannot use Increment on batch update, use Models/Maps")
		return b
	}
	col, ok := quoteColumnStrict(b.d, column)
	if !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	b.sets = append(b.sets, setItem{column: column, value: clause.Raw(col+" + ?", amount)})
	return b
}

// Decrement decrements a column by the specified amount.
// Example: Decrement("count", 1) generates "count = count - ?"
func (b *UpdateBuilder) Decrement(column string, amount any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.err = errors.New("corm: cannot use Decrement on batch update, use Models/Maps")
		return b
	}
	col, ok := quoteColumnStrict(b.d, column)
	if !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	b.sets = append(b.sets, setItem{column: column, value: clause.Raw(col+" - ?", amount)})
	return b
}

// Map adds SET items from a map.
// Keys are applied in sorted order to keep the generated SQL deterministic.
func (b *UpdateBuilder) Map(values map[string]any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.err = errors.New("corm: cannot use Map(map) on batch update, use Maps([]map)")
		return b
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if _, ok := quoteColumnStrict(b.d, k); !ok {
			b.err = errors.New("corm: invalid column identifier")
			return b
		}
		b.sets = append(b.sets, setItem{column: k, value: values[k]})
	}
	return b
}

// Where adds a WHERE condition.
// Do not concatenate untrusted user input into the SQL string; use args for parameter binding.
func (b *UpdateBuilder) Where(sql string, args ...any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.batch.where.Where(sql, args...)
		return b
	}
	b.where.Where(sql, args...)
	return b
}

func (b *UpdateBuilder) WhereEq(column string, value any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.batch.where.WhereEq(column, value)
		if b.batch.where.err != nil {
			b.err = b.batch.where.err
		}
		return b
	}
	b.where.WhereEq(column, value)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereIn adds a WHERE IN condition.
func (b *UpdateBuilder) WhereIn(column string, args ...any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.batch.where.WhereIn(column, args...)
		if b.batch.where.err != nil {
			b.err = b.batch.where.err
		}
		return b
	}
	b.where.WhereIn(column, args...)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

func (b *UpdateBuilder) WhereLike(column string, value any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.batch.where.WhereLike(column, value)
		if b.batch.where.err != nil {
			b.err = b.batch.where.err
		}
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
func (b *UpdateBuilder) WhereMap(conditions map[string]any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.batch.where.WhereMap(conditions)
		if b.batch.where.err != nil {
			b.err = b.batch.where.err
		}
		return b
	}
	b.where.WhereMap(conditions)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereSubquery adds a condition with a subquery: "column op (subquery)".
func (b *UpdateBuilder) WhereSubquery(column, op string, sub *SelectBuilder) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.batch.where.WhereSubquery(column, op, sub)
		if b.batch.where.err != nil {
			b.err = b.batch.where.err
		}
		return b
	}
	b.where.WhereSubquery(column, op, sub)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereInSubquery adds a "column IN (subquery)" condition.
func (b *UpdateBuilder) WhereInSubquery(column string, sub *SelectBuilder) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.batch.where.WhereSubquery(column, "IN", sub)
		if b.batch.where.err != nil {
			b.err = b.batch.where.err
		}
		return b
	}
	b.where.WhereSubquery(column, "IN", sub)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereExpr adds a clause.Expr as a WHERE condition.
func (b *UpdateBuilder) WhereExpr(e clause.Expr) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch != nil {
		b.batch.where.WhereExpr(e)
		if b.batch.where.err != nil {
			b.err = b.batch.where.err
		}
		return b
	}
	b.where.WhereExpr(e)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

func (b *UpdateBuilder) Key(column string) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch == nil {
		if len(b.sets) > 0 || len(b.where.items) > 0 {
			b.err = errors.New("corm: cannot switch to batch update after using Set/Where")
			return b
		}
		b.batch = newBatchUpdate(b.exec, b.d, b.table)
	}
	b.batch.Key(column)
	return b
}

func (b *UpdateBuilder) Models(models any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch == nil {
		if len(b.sets) > 0 || len(b.where.items) > 0 {
			b.err = errors.New("corm: cannot switch to batch update after using Set/Where")
			return b
		}
		b.batch = newBatchUpdate(b.exec, b.d, b.table)
	}
	b.batch.includePrimaryKey = b.includePrimaryKey
	b.batch.includeAuto = b.includeAuto
	b.batch.includeReadonly = b.includeReadonly
	b.batch.includeZero = b.includeZero
	b.batch.Models(models)
	return b
}

func (b *UpdateBuilder) Maps(rows []map[string]any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch == nil {
		if len(b.sets) > 0 || len(b.where.items) > 0 {
			b.err = errors.New("corm: cannot switch to batch update after using Set/Where")
			return b
		}
		b.batch = newBatchUpdate(b.exec, b.d, b.table)
	}
	b.batch.includePrimaryKey = b.includePrimaryKey
	b.batch.includeAuto = b.includeAuto
	b.batch.includeReadonly = b.includeReadonly
	b.batch.includeZero = b.includeZero
	b.batch.Maps(rows)
	return b
}

func (b *UpdateBuilder) MapsLowerKeys(rows []map[string]any) *UpdateBuilder {
	if b.err != nil {
		return b
	}
	if b.batch == nil {
		if len(b.sets) > 0 || len(b.where.items) > 0 {
			b.err = errors.New("corm: cannot switch to batch update after using Set/Where")
			return b
		}
		b.batch = newBatchUpdate(b.exec, b.d, b.table)
	}
	b.batch.includePrimaryKey = b.includePrimaryKey
	b.batch.includeAuto = b.includeAuto
	b.batch.includeReadonly = b.includeReadonly
	b.batch.includeZero = b.includeZero
	b.batch.MapsLowerKeys(rows)
	return b
}

// SQL generates the SQL query and arguments.
func (b *UpdateBuilder) SQL() (string, []any, error) {
	if b.err != nil {
		return "", nil, b.err
	}
	if b.batch != nil {
		return b.batch.SQL()
	}
	if b.d == nil {
		return "", nil, errors.New("corm: nil dialect")
	}
	if strings.TrimSpace(b.table) == "" {
		return "", nil, errors.New("corm: missing table for update")
	}
	if len(b.sets) == 0 {
		return "", nil, errors.New("corm: missing set values for update")
	}

	buf := getBuffer()
	defer putBuffer(buf)
	ab := newArgBuilder(b.d, 1)

	buf.WriteString("UPDATE ")
	qTable, ok := quoteIdentStrict(b.d, b.table)
	if !ok {
		return "", nil, errors.New("corm: invalid table identifier")
	}
	buf.WriteString(qTable)
	buf.WriteString(" SET ")

	for i, s := range b.sets {
		if i > 0 {
			buf.WriteString(", ")
		}
		col, ok := quoteColumnStrict(b.d, s.column)
		if !ok {
			return "", nil, errors.New("corm: invalid column identifier")
		}
		buf.WriteString(col)
		buf.WriteString(" = ")

		if e, ok := s.value.(clause.Expr); ok {
			if err := ab.appendExpr(buf, e); err != nil {
				return "", nil, err
			}
		} else {
			buf.WriteString(ab.add(s.value))
		}
	}

	if err := b.where.appendWhere(buf, ab); err != nil {
		return "", nil, err
	}

	if !b.allowEmptyWhere && len(b.where.items) == 0 {
		return "", nil, errors.New("corm: update without where clause is not allowed, use AllowEmptyWhere() to override")
	}

	if b.limit != nil {
		if *b.limit < 0 {
			return "", nil, errors.New("corm: invalid limit")
		}
		if b.d.Name() != "mysql" {
			return "", nil, errors.New("corm: update limit is only supported by mysql dialect")
		}
		buf.WriteString(" LIMIT ")
		buf.WriteString(ab.add(*b.limit))
	}

	return buf.String(), ab.args, nil
}

// Limit adds a LIMIT clause.
// Note: LIMIT on UPDATE is not standard SQL and may not be supported by all dialects (e.g. Postgres).
func (b *UpdateBuilder) Limit(limit int) *UpdateBuilder {
	if b.batch != nil {
		b.err = errors.New("corm: cannot use Limit on batch update")
		return b
	}
	b.limit = &limit
	return b
}

func (b *UpdateBuilder) Exec(ctx context.Context) (sql.Result, error) {
	if b.batch != nil {
		return b.batch.Exec(ctx)
	}
	if b.exec == nil {
		return nil, errors.New("corm: missing Executor for update")
	}
	sqlStr, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return b.exec.ExecContext(ctx, sqlStr, args...)
}
