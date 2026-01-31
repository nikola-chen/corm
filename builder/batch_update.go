package builder

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"strings"

	"github.com/nikola-chen/corm/dialect"
	"github.com/nikola-chen/corm/internal"
	"github.com/nikola-chen/corm/schema"
)

type batchUpdateBuilder struct {
	exec Executor
	d    dialect.Dialect

	table     string
	modelType reflect.Type

	keyColumn string
	keyField  *schema.Field

	columns []string
	fields  []*schema.Field

	rowsKeys   []any
	rowsValues [][]any

	where whereBuilder

	err error

	includePrimaryKey bool
	includeAuto       bool
	includeReadonly   bool
	includeZero       bool
}

var keepCurrentSentinel = new(struct{})

func newBatchUpdate(exec Executor, d dialect.Dialect, table string) *batchUpdateBuilder {
	b := &batchUpdateBuilder{
		exec:      exec,
		d:         d,
		table:     table,
		keyColumn: "id",
		where:     whereBuilder{d: d},
	}

	table = strings.TrimSpace(table)
	if table != "" && d != nil {
		if _, err := validateTable(d, table); err != nil {
			b.err = err
		}
	}

	return b
}

// Key sets the column name to be used as the join key for batch updates.
// This column must exist in the input data (Map keys or Model fields).
// If not set, it defaults to "id".
func (b *batchUpdateBuilder) Key(column string) *batchUpdateBuilder {
	b.keyColumn = strings.TrimSpace(column)
	return b
}

func (b *batchUpdateBuilder) Columns(cols ...string) *batchUpdateBuilder {
	b.columns = append(b.columns, cols...)
	return b
}

func (b *batchUpdateBuilder) IncludePrimaryKey() *batchUpdateBuilder {
	b.includePrimaryKey = true
	return b
}

func (b *batchUpdateBuilder) IncludeAuto() *batchUpdateBuilder {
	b.includeAuto = true
	return b
}

func (b *batchUpdateBuilder) IncludeReadonly() *batchUpdateBuilder {
	b.includeReadonly = true
	return b
}

func (b *batchUpdateBuilder) IncludeZero() *batchUpdateBuilder {
	b.includeZero = true
	return b
}

func (b *batchUpdateBuilder) Models(models any) *batchUpdateBuilder {
	if b.err != nil {
		return b
	}
	rv := reflect.ValueOf(models)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			b.err = schema.ErrInvalidModel
			return b
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		b.err = schema.ErrInvalidModel
		return b
	}
	if rv.Len() == 0 {
		return b
	}

	elemT := rv.Type().Elem()
	ptrElem := elemT.Kind() == reflect.Pointer
	baseT := elemT
	if ptrElem {
		baseT = elemT.Elem()
	}
	if baseT.Kind() != reflect.Struct {
		b.err = errors.New("corm: batch update models must be a slice of structs, got " + baseT.Kind().String())
		return b
	}

	if b.modelType == nil {
		b.modelType = baseT
	} else if b.modelType != baseT {
		b.err = errors.New("corm: batch update models must be of the same type")
		return b
	}

	s, err := schema.ParseType(baseT)
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
	} else if b.table != s.Table && b.rowsValues != nil {
		// Warn or error on table mismatch?
		// Given we enforce type match, this is less critical unless table manually changed.
	}

	if b.keyColumn == "" {
		b.keyColumn = "id"
	}

	// If columns are already set, validate that new schema matches them
	if len(b.columns) > 0 {
		// batchUpdateFieldsForSchema will return error if columns don't match or fields missing
		// so we just proceed to call it.
	}

	if b.keyField == nil || b.keyField != nil && internal.NormalizeColumn(b.keyField.Column) != internal.NormalizeColumn(b.keyColumn) {
		b.keyField = s.ByColumn[internal.NormalizeColumn(b.keyColumn)]
	}
	if b.keyField == nil {
		b.err = errors.New("corm: invalid batch update key column")
		return b
	}

	fields, err := b.batchUpdateFieldsForSchema(s)
	if err != nil {
		b.err = err
		return b
	}

	if b.rowsValues == nil {
		b.rowsKeys = make([]any, 0, rv.Len())
		b.rowsValues = make([][]any, 0, rv.Len())
	}
	for i := 0; i < rv.Len(); i++ {
		ev := rv.Index(i)
		if ptrElem {
			if ev.IsNil() {
				b.err = schema.ErrInvalidModel
				return b
			}
			ev = ev.Elem()
		}
		if ev.Kind() != reflect.Struct {
			b.err = schema.ErrInvalidModel
			return b
		}

		keyV := ev.FieldByIndex(b.keyField.Index)
		b.rowsKeys = append(b.rowsKeys, keyV.Interface())

		row := make([]any, len(fields))
		for j, f := range fields {
			fv := ev.FieldByIndex(f.Index)
			if f.OmitEmpty && !b.includeZero && fv.IsZero() {
				row[j] = keepCurrentSentinel
				continue
			}
			row[j] = fv.Interface()
		}
		b.rowsValues = append(b.rowsValues, row)
	}
	return b
}

func (b *batchUpdateBuilder) Maps(rows []map[string]any) *batchUpdateBuilder {
	return b.mapsInternal(rows, false)
}

func (b *batchUpdateBuilder) MapsLowerKeys(rows []map[string]any) *batchUpdateBuilder {
	return b.mapsInternal(rows, true)
}

func (b *batchUpdateBuilder) mapsInternal(rows []map[string]any, lowerKeys bool) *batchUpdateBuilder {
	if b.err != nil {
		return b
	}
	if len(rows) == 0 {
		return b
	}
	if strings.TrimSpace(b.table) == "" {
		b.err = errors.New("corm: missing table for batch update")
		return b
	}
	if b.keyColumn == "" {
		b.keyColumn = "id"
	}
	if _, ok := quoteColumnStrict(b.d, b.keyColumn); !ok {
		b.err = errors.New("corm: invalid batch update key column")
		return b
	}
	if len(b.columns) == 0 {
		cols := make([]string, 0, len(rows[0]))
		for k := range rows[0] {
			if strings.EqualFold(k, b.keyColumn) {
				continue
			}
			if _, ok := quoteColumnStrict(b.d, k); !ok {
				b.err = errors.New("corm: invalid column identifier")
				return b
			}
			cols = append(cols, k)
		}
		sort.Strings(cols)
		b.columns = cols
	} else {
		for _, c := range b.columns {
			if strings.EqualFold(c, b.keyColumn) {
				b.err = errors.New("corm: batch update cannot include key column")
				return b
			}
			if _, ok := quoteColumnStrict(b.d, c); !ok {
				b.err = errors.New("corm: invalid column identifier")
				return b
			}
		}
	}

	if b.rowsValues == nil {
		b.rowsKeys = make([]any, 0, len(rows))
		b.rowsValues = make([][]any, 0, len(rows))
	}
	for i := range rows {
		m := rows[i]
		if m == nil {
			b.err = errors.New("corm: nil map in batch update")
			return b
		}

		var keyV any
		var ok bool
		if lowerKeys {
			keyV, ok = m[strings.ToLower(b.keyColumn)]
		} else {
			keyV, ok = m[b.keyColumn]
		}
		if !ok {
			b.err = errors.New("corm: missing key column in map: " + b.keyColumn)
			return b
		}
		b.rowsKeys = append(b.rowsKeys, keyV)

		row := make([]any, len(b.columns))
		for j, col := range b.columns {
			lookup := col
			if lowerKeys {
				lookup = strings.ToLower(col)
			}
			v, ok := m[lookup]
			if !ok {
				row[j] = keepCurrentSentinel
				continue
			}
			row[j] = v
		}
		b.rowsValues = append(b.rowsValues, row)
	}
	return b
}

func (b *batchUpdateBuilder) SQL() (string, []any, error) {
	if b.err != nil {
		return "", nil, b.err
	}
	if b.d == nil {
		return "", nil, errors.New("corm: nil dialect")
	}
	buf := getBuffer()
	defer putBuffer(buf)
	ab := newArgBuilder(b.d, buf)
	if err := b.appendSQL(buf, ab); err != nil {
		return "", nil, err
	}
	return buf.String(), ab.args, nil
}

func (b *batchUpdateBuilder) appendSQL(buf *strings.Builder, ab *argBuilder) error {
	if strings.TrimSpace(b.table) == "" {
		return errors.New("corm: missing table for batch update")
	}
	if len(b.rowsKeys) == 0 || len(b.rowsValues) == 0 {
		return errors.New("corm: missing batch rows for update")
	}
	if len(b.rowsKeys) != len(b.rowsValues) {
		return errors.New("corm: batch update rows mismatch")
	}
	if len(b.columns) == 0 && len(b.fields) == 0 {
		return errors.New("corm: missing columns for batch update")
	}

	keyQuoted, ok := quoteColumnStrict(b.d, b.keyColumn)
	if !ok {
		return errors.New("corm: invalid column identifier")
	}

	cols := b.columns
	var fields []*schema.Field
	if len(b.fields) > 0 {
		fields = b.fields
		cols = make([]string, len(fields))
		for i := range fields {
			cols[i] = fields[i].Column
		}
	}

	qCols := make([]string, len(cols))
	for i, c := range cols {
		q, ok := quoteColumnStrict(b.d, c)
		if !ok {
			return errors.New("corm: invalid column identifier")
		}
		qCols[i] = q
	}

	buf.WriteString("UPDATE ")
	qTable, ok := quoteIdentStrict(b.d, b.table)
	if !ok {
		return errors.New("corm: invalid table identifier")
	}
	buf.WriteString(qTable)
	buf.WriteString(" SET ")

	for ci := range qCols {
		if ci > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(qCols[ci])
		buf.WriteString(" = CASE ")
		buf.WriteString(keyQuoted)
		for ri := range b.rowsKeys {
			buf.WriteString(" WHEN ")
			buf.WriteString(ab.add(b.rowsKeys[ri]))
			buf.WriteString(" THEN ")
			v := b.rowsValues[ri][ci]
			if v == keepCurrentSentinel {
				buf.WriteString(qCols[ci])
			} else {
				buf.WriteString(ab.add(v))
			}
		}
		buf.WriteString(" ELSE ")
		buf.WriteString(qCols[ci])
		buf.WriteString(" END")
	}

	buf.WriteString(" WHERE ")
	buf.WriteString(keyQuoted)
	buf.WriteString(" IN (")
	for i := range b.rowsKeys {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(ab.add(b.rowsKeys[i]))
	}
	buf.WriteString(")")
	if err := b.where.appendAndWhere(buf, ab); err != nil {
		return err
	}

	return nil

}

func (b *batchUpdateBuilder) Exec(ctx context.Context) (sql.Result, error) {
	if b.exec == nil {
		return nil, errors.New("corm: missing Executor for batch update")
	}
	sqlStr, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return b.exec.ExecContext(ctx, sqlStr, args...)
}

func (b *batchUpdateBuilder) batchUpdateFieldsForSchema(s *schema.Schema) ([]*schema.Field, error) {
	if len(b.columns) > 0 {
		fields := make([]*schema.Field, 0, len(b.columns))
		cols := make([]string, 0, len(b.columns))
		keyKey := internal.NormalizeColumn(b.keyColumn)
		for _, col := range b.columns {
			if internal.NormalizeColumn(col) == keyKey {
				return nil, errors.New("corm: batch update cannot include key column")
			}
			f := s.ByColumn[internal.NormalizeColumn(col)]
			if f == nil {
				return nil, errors.New("corm: unknown column in batch update: " + col)
			}
			if f.Readonly && !b.includeReadonly {
				return nil, errors.New("corm: readonly column in batch update: " + col)
			}
			if f.Auto && !b.includeAuto {
				return nil, errors.New("corm: auto column in batch update: " + col)
			}
			if f.PrimaryKey && !b.includePrimaryKey {
				return nil, errors.New("corm: primary key column in batch update: " + col)
			}
			fields = append(fields, f)
			cols = append(cols, f.Column)
		}
		b.fields = fields
		b.columns = cols
		return fields, nil
	}

	keyKey := internal.NormalizeColumn(b.keyColumn)
	cols := make([]string, 0, len(s.Fields))
	fields := make([]*schema.Field, 0, len(s.Fields))
	for _, f := range s.Fields {
		if internal.NormalizeColumn(f.Column) == keyKey {
			continue
		}
		if f.Readonly && !b.includeReadonly {
			continue
		}
		if f.Auto && !b.includeAuto {
			continue
		}
		if f.PrimaryKey && !b.includePrimaryKey {
			continue
		}
		cols = append(cols, f.Column)
		fields = append(fields, f)
	}
	b.columns = cols
	b.fields = fields
	return fields, nil
}
