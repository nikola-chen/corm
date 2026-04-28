package scan

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"

	"github.com/nikola-chen/corm/internal"
	"github.com/nikola-chen/corm/schema"
)

type structPlanKey struct {
	t    reflect.Type
	cols string
}

type structPlanCache struct {
	mu    sync.RWMutex
	items map[structPlanKey][][]int
	count int
}

var spCache = &structPlanCache{items: make(map[structPlanKey][][]int, 256)}

const maxStructPlanCacheEntries = 1024

func colsKey(cols []string) string {
	if len(cols) == 0 {
		return ""
	}

	// Estimate total capacity to avoid reallocations
	totalCap := 0
	for _, c := range cols {
		totalCap += len(c) + 1
	}

	buf := make([]byte, 0, totalCap)

	for i, c := range cols {
		if i > 0 {
			buf = append(buf, 0x1f)
		}
		c = strings.TrimSpace(c)
		c = strings.Trim(c, "`\"")
		if idx := strings.LastIndexByte(c, '.'); idx >= 0 {
			c = c[idx+1:]
		}
		buf = appendLowerASCII(buf, c)
	}

	return string(buf)
}

// appendLowerASCII appends the lowercase version of s to buf and returns the new slice.
func appendLowerASCII(buf []byte, s string) []byte {
	for i := range s {
		ch := s[i]
		if ch >= 0x80 {
			return append(buf, strings.ToLower(s[i:])...)
		}
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		buf = append(buf, ch)
	}
	return buf
}

func structPlan(s *schema.Schema, cols []string) [][]int {
	key := structPlanKey{t: s.Type, cols: colsKey(cols)}
	spCache.mu.RLock()
	if v, ok := spCache.items[key]; ok {
		spCache.mu.RUnlock()
		return v
	}
	spCache.mu.RUnlock()

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

	spCache.mu.Lock()
	if spCache.count >= maxStructPlanCacheEntries {
		spCache.items = make(map[structPlanKey][][]int, 256)
		spCache.count = 0
	}
	if _, ok := spCache.items[key]; !ok {
		spCache.items[key] = plan
		spCache.count++
	}
	spCache.mu.Unlock()
	return plan
}

func ScanAll(rows *sql.Rows, dest any) error {
	return scanAll(rows, dest, false, 0)
}

func ScanAllStrict(rows *sql.Rows, dest any) error {
	return scanAll(rows, dest, true, 0)
}

func ScanAllCap(rows *sql.Rows, dest any, capHint int) error {
	return scanAll(rows, dest, false, capHint)
}

func scanAll(rows *sql.Rows, dest any, strictStructColumns bool, capHint int) error {
	defer rows.Close()

	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("corm: dest must be non-nil pointer")
	}
	sliceV := rv.Elem()
	if sliceV.Kind() != reflect.Slice {
		return errors.New("corm: dest must be pointer to slice")
	}

	// Optimization: pre-allocate slice if capHint provided and slice is nil/empty
	if capHint > 0 && (sliceV.IsNil() || sliceV.Len() == 0) {
		sliceV.Set(reflect.MakeSlice(sliceV.Type(), 0, capHint))
	}

	if !rows.Next() {
		return rows.Err()
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
			return errors.New("corm: map element must have string keys")
		}
		valT := elemT.Elem()
		n := len(cols)
		holders := make([]any, n)
		for i := range holders {
			holders[i] = new(any)
		}

		// Pre-allocate reflect.Value keys to avoid allocation in loop
		keys := make([]reflect.Value, n)
		for i, c := range cols {
			keys[i] = reflect.ValueOf(c)
		}

		for {
			if err := rows.Scan(holders...); err != nil {
				return err
			}
			m := reflect.MakeMapWithSize(elemT, n)
			for i := range cols {
				raw := *(holders[i].(*any))
				if raw == nil {
					// For map[string]any, we can set nil directly if value type allows interface
					// But reflect.New(valT).Elem() is zero value, which is nil for interface/ptr
					v := reflect.Zero(valT)
					m.SetMapIndex(keys[i], v)
					*(holders[i].(*any)) = nil
					continue
				}

				// Safe copy for byte slices (sql.RawBytes is volatile)
				if b, ok := raw.([]byte); ok {
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
					// If not assignable/convertible, try to fit into any if target is interface
					if valT.Kind() == reflect.Interface {
						v = rv
					} else {
						// Fallback: create zero value
						v = reflect.Zero(valT)
					}
				}
				m.SetMapIndex(keys[i], v)
				*(holders[i].(*any)) = nil
			}
			sliceV.Set(reflect.Append(sliceV, m))

			if !rows.Next() {
				break
			}
		}
		return rows.Err()
	case reflect.Struct:
		if strictStructColumns {
			if err := validateStructColumns(cols); err != nil {
				return err
			}
		}
		s, err := schema.ParseType(baseElemT)
		if err != nil {
			return err
		}
		plan := structPlan(s, cols)
		n := len(cols)
		holders := make([]any, n)

		for {
			elem := reflect.New(s.Type).Elem()
			for i := range n {
				if plan[i] == nil {
					// Each row gets its own dummy to avoid any potential issues
					var dummy any
					holders[i] = &dummy
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

			if !rows.Next() {
				break
			}
		}
		return rows.Err()
	default:
		return errors.New("corm: slice element must be struct, *struct, or map")
	}
}

func ScanOne(rows *sql.Rows, dest any) error {
	return scanOne(rows, dest, false)
}

func ScanOneStrict(rows *sql.Rows, dest any) error {
	return scanOne(rows, dest, true)
}

func scanOne(rows *sql.Rows, dest any, strictStructColumns bool) error {
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
		valT := base.Type().Elem()
		n := len(cols)
		holders := make([]any, n)
		for i := range holders {
			holders[i] = new(any)
		}
		if err := rows.Scan(holders...); err != nil {
			return err
		}

		out := reflect.MakeMapWithSize(base.Type(), n)
		// Pre-allocate reflect.Value keys
		keys := make([]reflect.Value, n)
		for i, c := range cols {
			keys[i] = reflect.ValueOf(c)
		}

		for i := range cols {
			raw := *(holders[i].(*any))
			if raw == nil {
				v := reflect.Zero(valT)
				out.SetMapIndex(keys[i], v)
				*(holders[i].(*any)) = nil
				continue
			}

			if b, ok := raw.([]byte); ok {
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
			out.SetMapIndex(keys[i], v)
			*(holders[i].(*any)) = nil
		}
		base.Set(out)
		return nil
	case reflect.Struct:
		if strictStructColumns {
			if err := validateStructColumns(cols); err != nil {
				return err
			}
		}
		s, err := schema.Parse(base.Addr().Interface())
		if err != nil {
			return err
		}
		plan := structPlan(s, cols)
		n := len(cols)
		holders := make([]any, n)

		for i := range n {
			if plan[i] == nil {
				var dummy any
				holders[i] = &dummy
				continue
			}
			holders[i] = base.FieldByIndex(plan[i]).Addr().Interface()
		}
		return rows.Scan(holders...)
	default:
		return errors.New("corm: dest must be struct/*struct or map/*map")
	}
}

func validateStructColumns(cols []string) error {
	seen := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		n := strings.ToLower(normalizeColumn(c))
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			return errors.New("corm: duplicate column name after normalization: " + n + ", use AS to alias")
		}
		seen[n] = struct{}{}
	}
	return nil
}

// normalizeColumn normalizes a column name for case-insensitive comparison.
// It delegates to internal.NormalizeColumn for consistency.
func normalizeColumn(c string) string {
	return internal.NormalizeColumn(c)
}
