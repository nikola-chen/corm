package scan

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/nikola-chen/corm/schema"
)

type structPlanKey struct {
	t    reflect.Type
	cols string
}

var structPlanCache sync.Map
var structPlanCacheCount atomic.Uint64

const maxStructPlanCacheEntries = 1024

var anySlicePool sync.Pool

const maxPooledAnySliceCap = 4096

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
	if s == nil {
		return
	}
	for i := range s {
		s[i] = nil
	}
	if cap(s) > maxPooledAnySliceCap {
		return
	}
	anySlicePool.Put(s)
}

func colsKey(cols []string) string {
	// Build a stable cache key for a column list.
	// 0x1f is used as a separator byte to minimize collisions and allocations.
	n := len(cols) // separators
	for _, c := range cols {
		n += len(c)
	}

	var b strings.Builder
	b.Grow(n)

	for i, c := range cols {
		if i > 0 {
			b.WriteByte(0x1f)
		}
		// normalize inline to avoid allocation
		c = strings.TrimSpace(c)
		c = strings.Trim(c, "`\"")
		if idx := strings.LastIndexByte(c, '.'); idx >= 0 {
			c = c[idx+1:]
		}
		// Fast path: ASCII-only lowercasing avoids an extra allocation from strings.ToLower.
		nonASCII := false
		for i := 0; i < len(c); i++ {
			if c[i] >= 0x80 {
				nonASCII = true
				break
			}
		}
		if nonASCII {
			b.WriteString(strings.ToLower(c))
			continue
		}
		for i := 0; i < len(c); i++ {
			ch := c[i]
			if ch >= 'A' && ch <= 'Z' {
				ch += 'a' - 'A'
			}
			b.WriteByte(ch)
		}
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
	if structPlanCacheCount.Load() >= maxStructPlanCacheEntries {
		// Simple eviction strategy: if cache is full, just return the plan without storing it.
		// In a long-running process with diverse queries, this prevents memory leak.
		// For better performance, we could implement LRU or random eviction, but for now this is safe.
		// A potential improvement: if cache is full, clear it entirely (blunt but effective for changing workloads).
		// However, avoiding the lock contention of clearing is preferred.
		return plan
	}
	actual, loaded := structPlanCache.LoadOrStore(key, plan)
	if !loaded {
		structPlanCacheCount.Add(1)
	}
	return actual.([][]int)
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
		holders := getAnySlice(n)
		defer putAnySlice(holders)
		for i := range holders {
			var v any
			holders[i] = &v
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
					// If not assignable/convertible, try to fit into interface{} if target is interface
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
		holders := getAnySlice(n)
		defer putAnySlice(holders)

		var dummySink any

		for {
			elem := reflect.New(s.Type).Elem()
			for i := 0; i < n; i++ {
				if plan[i] == nil {
					holders[i] = &dummySink
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
		holders := getAnySlice(n)
		defer putAnySlice(holders)
		for i := range holders {
			var v any
			holders[i] = &v
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

			// Safe copy for []byte (sql.RawBytes is volatile)
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
		holders := getAnySlice(n)
		defer putAnySlice(holders)

		// Shared dummy for unmapped columns
		var sharedDummy any

		for i := 0; i < n; i++ {
			if plan[i] == nil {
				sharedDummy = nil
				holders[i] = &sharedDummy
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

func normalizeColumn(c string) string {
	c = strings.TrimSpace(c)
	c = strings.Trim(c, "`\"")
	if i := strings.LastIndexByte(c, '.'); i >= 0 {
		c = c[i+1:]
	}
	return c
}
