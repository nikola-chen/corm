package builder

import (
	"errors"
	"reflect"
	"strings"

	"github.com/nikola-chen/corm/schema"
)

func (b *InsertBuilder) Models(models any) *InsertBuilder {
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
		b.err = schema.ErrInvalidModel
		return b
	}

	s, err := schema.ParseType(baseT)
	if err != nil {
		b.err = err
		return b
	}
	if b.table == "" {
		if strings.TrimSpace(s.Table) == "" {
			b.err = errors.New("corm: missing table for insert: model has no table name")
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

	fields, err := b.insertFieldsForSchema(s)
	if err != nil {
		b.err = err
		return b
	}

	if cap(b.rows) == 0 && rv.Len() > 0 {
		b.rows = make([][]any, 0, rv.Len())
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
		row := make([]any, len(fields))
		for j, f := range fields {
			row[j] = ev.FieldByIndex(f.Index).Interface()
		}
		b.rows = append(b.rows, row)
	}
	return b
}

func (b *InsertBuilder) insertFieldsForSchema(s *schema.Schema) ([]*schema.Field, error) {
	if len(b.columns) > 0 {
		fields := make([]*schema.Field, len(b.columns))
		for i, col := range b.columns {
			key := normalizeInsertColumnKey(col)
			f := s.ByColumn[key]
			if f == nil {
				return nil, errors.New("corm: unknown column in Model: " + col)
			}
			fields[i] = f
		}
		return fields, nil
	}

	cols := make([]string, 0, len(s.Fields))
	fields := make([]*schema.Field, 0, len(s.Fields))
	for _, f := range s.Fields {
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
	return fields, nil
}

func normalizeInsertColumnKey(c string) string {
	return normalizeColumn(c)
}
