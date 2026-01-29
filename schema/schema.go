package schema

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
)

// TableNamer is an interface for structs to customize their table name.
type TableNamer interface {
	TableName() string
}

// Field represents a database column mapped to a struct field.
type Field struct {
	Name       string
	Column     string
	Index      []int
	Type       reflect.Type
	PrimaryKey bool
	Auto       bool
	Readonly   bool
	OmitEmpty  bool
}

// Schema represents the metadata of a struct model.
type Schema struct {
	Type        reflect.Type
	Table       string
	Fields      []*Field
	ByColumn    map[string]*Field
	PrimaryKeys []*Field
}

// ExtractOptions defines options for extracting values from a struct.
type ExtractOptions struct {
	IncludePrimaryKey bool
	IncludeAuto       bool
	IncludeReadonly   bool
	IncludeZero       bool
}

// ColumnsAndValues extracts column names and values from a struct instance based on the provided options.
func (s *Schema) ColumnsAndValues(dest any, opts ExtractOptions) ([]string, []any, error) {
	rv := reflect.ValueOf(dest)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, nil, ErrInvalidModel
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, nil, ErrInvalidModel
	}

	cols := make([]string, 0, len(s.Fields))
	vals := make([]any, 0, len(s.Fields))
	for _, f := range s.Fields {
		if f.Readonly && !opts.IncludeReadonly {
			continue
		}
		if f.Auto && !opts.IncludeAuto {
			continue
		}
		if f.PrimaryKey && !opts.IncludePrimaryKey {
			continue
		}
		fv := rv.FieldByIndex(f.Index)
		if f.OmitEmpty && !opts.IncludeZero && fv.IsZero() {
			continue
		}
		cols = append(cols, f.Column)
		vals = append(vals, fv.Interface())
	}
	return cols, vals, nil
}

var cache sync.Map
var cacheCount atomic.Uint64

// maxSchemaCacheEntries bounds the global schema cache to avoid unbounded memory growth
// in long-lived processes that may parse many different struct types.
const maxSchemaCacheEntries = 1024

// Parse parses a struct model and returns its Schema.
// It caches the result for future use.
// If the struct has fields mapping to the same column name (via tag or snake_case),
// the last defined field (in depth-first traversal) wins and overwrites previous mappings.
func Parse(model any) (*Schema, error) {
	if model == nil {
		return nil, ErrInvalidModel
	}
	t := reflect.TypeOf(model)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return ParseType(t)
}

type parseEntry struct {
	done chan struct{}
	s    *Schema
	err  error
}

// parseGroup avoids redundant parsing of the same type by concurrent callers.
var parseGroup sync.Map

func ParseType(t reflect.Type) (*Schema, error) {
	if t == nil {
		return nil, ErrInvalidModel
	}
	if t.Kind() != reflect.Struct {
		return nil, errors.New("corm: model must be struct, got " + t.Kind().String())
	}

	if v, ok := cache.Load(t); ok {
		return v.(*Schema), nil
	}

	e := &parseEntry{done: make(chan struct{})}
	actual, loaded := parseGroup.LoadOrStore(t, e)
	if loaded {
		e = actual.(*parseEntry)
		<-e.done
		if e.err != nil {
			return nil, e.err
		}
		return e.s, nil
	}

	var s *Schema
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("corm: schema parse panic: %v", r)
			}
		}()
		s, err = parseSlow(t)
	}()
	if err != nil {
		e.s = nil
		e.err = err
		close(e.done)
		parseGroup.Delete(t)
		return nil, err
	}

	actualS, loaded := cache.LoadOrStore(t, s)
	if loaded {
		s = actualS.(*Schema)
	} else {
		if cacheCount.Add(1) > maxSchemaCacheEntries {
			evicted := false
			cache.Range(func(k, _ any) bool {
				if k == t {
					return true
				}
				cache.Delete(k)
				cacheCount.Add(^uint64(0))
				evicted = true
				return true
			})
			if !evicted {
				cacheCount.Add(^uint64(0))
			}
		}
	}

	e.s = s
	e.err = nil
	close(e.done)
	parseGroup.Delete(t)
	return s, nil
}

var ErrInvalidModel = &schemaError{"corm: model must be struct or pointer to struct"}

type schemaError struct{ msg string }

func (e *schemaError) Error() string { return e.msg }

func parseSlow(t reflect.Type) (*Schema, error) {
	s := &Schema{
		Type:     t,
		Table:    defaultTableName(t),
		ByColumn: map[string]*Field{},
	}

	if tn, ok := reflect.New(t).Interface().(TableNamer); ok {
		if name := strings.TrimSpace(tn.TableName()); name != "" {
			s.Table = name
		}
	}

	parseStructFields(s, t, nil)

	if len(s.PrimaryKeys) == 0 {
		if f, ok := s.ByColumn["id"]; ok {
			f.PrimaryKey = true
			s.PrimaryKeys = append(s.PrimaryKeys, f)
		}
	}

	return s, nil
}

func parseStructFields(s *Schema, t reflect.Type, parentIndex []int) {
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.Anonymous && sf.Type.Kind() == reflect.Struct && sf.PkgPath == "" {
			parseStructFields(s, sf.Type, appendIndex(parentIndex, i))
			continue
		}
		if sf.PkgPath != "" {
			continue
		}

		tag := sf.Tag.Get("db")
		if tag == "-" {
			continue
		}

		col, opts := parseDBTag(tag)
		if col == "" {
			col = toSnake(sf.Name)
		}

		f := &Field{
			Name:      sf.Name,
			Column:    col,
			Index:     appendIndex(parentIndex, i),
			Type:      sf.Type,
			Auto:      opts["auto"] || opts["autoincr"] || opts["identity"],
			Readonly:  opts["readonly"],
			OmitEmpty: opts["omitempty"],
		}
		if opts["pk"] || sf.Tag.Get("pk") == "true" {
			f.PrimaryKey = true
			s.PrimaryKeys = append(s.PrimaryKeys, f)
		}

		s.Fields = append(s.Fields, f)
		s.ByColumn[strings.ToLower(col)] = f
	}
}

func appendIndex(parent []int, i int) []int {
	if len(parent) == 0 {
		return []int{i}
	}
	idx := make([]int, 0, len(parent)+1)
	idx = append(idx, parent...)
	idx = append(idx, i)
	return idx
}

func parseDBTag(tag string) (string, map[string]bool) {
	opts := map[string]bool{}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", opts
	}
	parts := strings.Split(tag, ",")
	col := strings.TrimSpace(parts[0])
	for _, p := range parts[1:] {
		p = strings.TrimSpace(p)
		if p != "" {
			opts[p] = true
		}
	}
	return col, opts
}

func defaultTableName(t reflect.Type) string {
	return toSnake(t.Name())
}

func toSnake(s string) string {
	if s == "" {
		return ""
	}

	// Fast path: check if all ASCII
	allASCII := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			allASCII = false
			break
		}
	}

	if allASCII {
		return toSnakeASCII(s)
	}
	return toSnakeUnicode(s)
}

// toSnakeASCII converts an ASCII string to snake_case.
func toSnakeASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)

	prevLower := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			nextLower := i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z'
			if i > 0 && (prevLower || nextLower) {
				b.WriteByte('_')
			}
			b.WriteByte(c + ('a' - 'A'))
			prevLower = false
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			prevLower = c >= 'a' && c <= 'z'
			b.WriteByte(c)
			continue
		}
		if c == '_' {
			b.WriteByte('_')
			prevLower = false
			continue
		}
	}
	return b.String()
}

// toSnakeUnicode converts a Unicode string to snake_case.
func toSnakeUnicode(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(runes) + 8)

	prevLower := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if unicode.IsUpper(r) {
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if i > 0 && (prevLower || nextLower) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			prevLower = false
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			prevLower = unicode.IsLower(r)
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		if r == '_' {
			b.WriteRune('_')
			prevLower = false
			continue
		}
	}
	return b.String()
}
