package scan

import (
	"database/sql"
	"errors"
	"iter"
	"reflect"

	"github.com/nikola-chen/corm/schema"
)

// Iter returns an iterator that yields elements or an error for each row.
// The element type T can be a struct, *struct, or map[string]any.
func Iter[T any](rows *sql.Rows) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}

		var zeroT T
		targetT := reflect.TypeOf(zeroT)

		isPtr := targetT != nil && targetT.Kind() == reflect.Pointer
		baseElemT := targetT
		if isPtr {
			baseElemT = targetT.Elem()
		}

		if baseElemT == nil {
			yield(zeroT, errors.New("corm: dest type cannot be nil interface"))
			return
		}

		switch baseElemT.Kind() {
		case reflect.Struct:
			s, err := schema.ParseType(baseElemT)
			if err != nil {
				yield(zeroT, err)
				return
			}
			plan := structPlan(s, cols)
			n := len(cols)
			holders := make([]any, n)

			for rows.Next() {
				elem := reflect.New(s.Type).Elem()
				for i := 0; i < n; i++ {
					if plan[i] == nil {
						var dummy any
						holders[i] = &dummy
						continue
					}
					holders[i] = elem.FieldByIndex(plan[i]).Addr().Interface()
				}
				if err := rows.Scan(holders...); err != nil {
					yield(zeroT, err)
					return
				}
				
				var out T
				if isPtr {
					out = elem.Addr().Interface().(T)
				} else {
					out = elem.Interface().(T)
				}
				
				if !yield(out, nil) {
					return
				}
			}
			if err := rows.Err(); err != nil {
				yield(zeroT, err)
			}
		
		case reflect.Map:
			if targetT.Kind() != reflect.Map || targetT.Key().Kind() != reflect.String {
				yield(zeroT, errors.New("corm: map element must have string keys"))
				return
			}
			valT := targetT.Elem()
			n := len(cols)
			holders := make([]any, n)
			for i := range holders {
				holders[i] = new(any)
			}
			
			keys := make([]reflect.Value, n)
			for i, c := range cols {
				keys[i] = reflect.ValueOf(c)
			}

			for rows.Next() {
				if err := rows.Scan(holders...); err != nil {
					yield(zeroT, err)
					return
				}
				m := reflect.MakeMapWithSize(targetT, n)
				for i := range cols {
					raw := *(holders[i].(*any))
					if raw == nil {
						v := reflect.Zero(valT)
						m.SetMapIndex(keys[i], v)
						*(holders[i].(*any)) = nil
						continue
					}

					switch b := raw.(type) {
					case []byte:
						c := make([]byte, len(b))
						copy(c, b)
						raw = c
					case sql.RawBytes:
						c := make([]byte, len(b))
						copy(c, b)
						raw = c
					}

					rv := reflect.ValueOf(raw)
					var v reflect.Value
					if rv.Type().AssignableTo(valT) {
						v = rv
					} else if rv.Type().ConvertibleTo(valT) {
						v = rv.Convert(valT)
					} else {
						if valT.Kind() == reflect.Interface {
							v = rv
						} else {
							v = reflect.Zero(valT)
						}
					}
					m.SetMapIndex(keys[i], v)
					*(holders[i].(*any)) = nil
				}
				
				out := m.Interface().(T)
				if !yield(out, nil) {
					return
				}
			}
			if err := rows.Err(); err != nil {
				yield(zeroT, err)
			}
		
		default:
			yield(zeroT, errors.New("corm: dest must be struct, *struct, or map"))
		}
	}
}
