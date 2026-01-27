package exec

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"

	"corm/schema"
)

type structPlanKey struct {
	t    reflect.Type
	cols string
}

var structPlanCache sync.Map

var anySlicePool sync.Pool

func getAnySlice(n int) []any {
	if v := anySlicePool.Get(); v != nil {
		s := v.([]any)
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]any, n)
}

func putAnySlice(s []any) {
	for i := range s {
		s[i] = nil
	}
	anySlicePool.Put(s)
}

func colsKey(cols []string) string {
	var b strings.Builder
	for i, c := range cols {
		if i > 0 {
			b.WriteByte(0x1f)
		}
		b.WriteString(strings.ToLower(normalizeColumn(c)))
	}
	return b.String()
}

func structPlan(s *schema.Schema, cols []string) [][]int {
	key := structPlanKey{t: s.Type, cols: colsKey(cols)}
	if v, ok := structPlanCache.Load(key); ok {
		return v.([][]int)
	}

	plan := make([][]int, len(cols))
	for i, c := range cols {
		f := s.ByColumn[strings.ToLower(normalizeColumn(c))]
		if f == nil {
			continue
		}
		idx := make([]int, len(f.Index))
		copy(idx, f.Index)
		plan[i] = idx
	}
	actual, _ := structPlanCache.LoadOrStore(key, plan)
	return actual.([][]int)
}

// ScanAll scans all rows into a slice of structs or maps.
func ScanAll(rows *sql.Rows, dest any) error {
	defer rows.Close()

	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("corm: dest must be non-nil pointer")
	}
	sliceV := rv.Elem()
	if sliceV.Kind() != reflect.Slice {
		return errors.New("corm: dest must be pointer to slice")
	}

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	elemT := sliceV.Type().Elem()
	elemIsPtr := elemT.Kind() == reflect.Pointer
	baseElemT := elemT
	if elemIsPtr {
		baseElemT = elemT.Elem()
	}

	switch baseElemT.Kind() {
	case reflect.Map:
		if elemT.Kind() != reflect.Map || elemT.Key().Kind() != reflect.String {
			return errors.New("corm: map element must be map[string]T")
		}
		valT := elemT.Elem()
		n := len(cols)
		holders := getAnySlice(n)
		defer putAnySlice(holders)
		for i := range holders {
			var v any
			holders[i] = &v
		}
		for rows.Next() {
			if err := rows.Scan(holders...); err != nil {
				return err
			}
			m := reflect.MakeMapWithSize(elemT, n)
			for i, c := range cols {
				key := reflect.ValueOf(c)
				raw := *(holders[i].(*any))
				v := reflect.New(valT).Elem()
				if raw != nil {
					rv := reflect.ValueOf(raw)
					if rv.Type().AssignableTo(valT) {
						v.Set(rv)
					} else if rv.Type().ConvertibleTo(valT) {
						v.Set(rv.Convert(valT))
					} else if valT.Kind() == reflect.Interface {
						v.Set(rv)
					}
				}
				m.SetMapIndex(key, v)
				*(holders[i].(*any)) = nil
			}
			sliceV.Set(reflect.Append(sliceV, m))
		}
		return rows.Err()
	case reflect.Struct:
		s, err := schema.Parse(reflect.New(baseElemT).Interface())
		if err != nil {
			return err
		}
		plan := structPlan(s, cols)
		n := len(cols)
		holders := getAnySlice(n)
		defer putAnySlice(holders)
		dummy := new(any)
		for rows.Next() {
			elem := reflect.New(s.Type).Elem()
			for i := 0; i < n; i++ {
				if plan[i] == nil {
					holders[i] = dummy
					continue
				}
				holders[i] = elem.FieldByIndex(plan[i]).Addr().Interface()
			}
			if err := rows.Scan(holders...); err != nil {
				return err
			}
			if elemIsPtr {
				sliceV.Set(reflect.Append(sliceV, elem.Addr()))
			} else {
				sliceV.Set(reflect.Append(sliceV, elem))
			}
		}
		return rows.Err()
	default:
		return errors.New("corm: slice element must be struct, *struct, or map")
	}
}

// ScanOne scans a single row into a struct or map.
func ScanOne(rows *sql.Rows, dest any) error {
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}

	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("corm: dest must be non-nil pointer")
	}

	base := rv.Elem()
	if base.Kind() == reflect.Pointer {
		if base.IsNil() {
			base.Set(reflect.New(base.Type().Elem()))
		}
		base = base.Elem()
	}

	switch base.Kind() {
	case reflect.Map:
		if base.Type().Key().Kind() != reflect.String {
			return errors.New("corm: dest map key must be string")
		}
		out := reflect.MakeMapWithSize(base.Type(), len(cols))
		valT := base.Type().Elem()
		n := len(cols)
		holders := getAnySlice(n)
		defer putAnySlice(holders)
		for i := range holders {
			var v any
			holders[i] = &v
		}
		if err := rows.Scan(holders...); err != nil {
			return err
		}
		for i, c := range cols {
			key := reflect.ValueOf(c)
			raw := *(holders[i].(*any))
			v := reflect.New(valT).Elem()
			if raw != nil {
				rv := reflect.ValueOf(raw)
				if rv.Type().AssignableTo(valT) {
					v.Set(rv)
				} else if rv.Type().ConvertibleTo(valT) {
					v.Set(rv.Convert(valT))
				} else if valT.Kind() == reflect.Interface {
					v.Set(rv)
				}
			}
			out.SetMapIndex(key, v)
			*(holders[i].(*any)) = nil
		}
		base.Set(out)
		return nil
	case reflect.Struct:
		s, err := schema.Parse(base.Addr().Interface())
		if err != nil {
			return err
		}
		plan := structPlan(s, cols)
		n := len(cols)
		holders := getAnySlice(n)
		defer putAnySlice(holders)
		dummy := new(any)
		for i := 0; i < n; i++ {
			if plan[i] == nil {
				holders[i] = dummy
				continue
			}
			holders[i] = base.FieldByIndex(plan[i]).Addr().Interface()
		}
		return rows.Scan(holders...)
	default:
		return errors.New("corm: dest must be struct/*struct or map/*map")
	}
}

func normalizeColumn(c string) string {
	c = strings.TrimSpace(c)
	c = strings.Trim(c, "`\"")
	if i := strings.LastIndexByte(c, '.'); i >= 0 {
		c = c[i+1:]
	}
	return c
}
