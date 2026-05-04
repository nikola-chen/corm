package schema

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
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

// maxSchemaCacheEntries bounds the global schema cache to avoid unbounded memory growth
// in long-lived processes that may parse many different struct types.
const maxSchemaCacheEntries = 1024

type schemaCache struct {
	mu    sync.RWMutex
	items map[reflect.Type]*Schema
	count int
}

var scCache = &schemaCache{items: make(map[reflect.Type]*Schema, 256)}

type parseEntry struct {
	done chan struct{}
	s    *Schema
	err  error
}

type parseGroup struct {
	mu    sync.Mutex
	items map[reflect.Type]*parseEntry
}

var pg = &parseGroup{items: make(map[reflect.Type]*parseEntry)}

var tableNamerType = reflect.TypeFor[TableNamer]()

type tableNameCache struct {
	mu    sync.RWMutex
	items map[reflect.Type]string
	count int
}

var tnCache = &tableNameCache{items: make(map[reflect.Type]string, 256)}

const maxTableNameCacheEntries = 1024

func cachedTableName(t reflect.Type) string {
	tnCache.mu.RLock()
	if v, ok := tnCache.items[t]; ok {
		tnCache.mu.RUnlock()
		return v
	}
	tnCache.mu.RUnlock()

	name := defaultTableName(t)
	if reflect.PointerTo(t).Implements(tableNamerType) {
		v := reflect.New(t)
		if tn, ok := v.Interface().(TableNamer); ok {
			if n := strings.TrimSpace(tn.TableName()); n != "" {
				name = n
			}
		}
	}

	tnCache.mu.Lock()
	if tnCache.count >= maxTableNameCacheEntries {
		clear(tnCache.items)
		tnCache.count = 0
	}
	if _, ok := tnCache.items[t]; !ok {
		tnCache.items[t] = name
		tnCache.count++
	}
	tnCache.mu.Unlock()

	return name
}

func LookupTableName(t reflect.Type) string {
	if t == nil || t.Kind() != reflect.Struct {
		return ""
	}
	tnCache.mu.RLock()
	if v, ok := tnCache.items[t]; ok {
		tnCache.mu.RUnlock()
		return v
	}
	tnCache.mu.RUnlock()

	scCache.mu.RLock()
	if v, ok := scCache.items[t]; ok {
		scCache.mu.RUnlock()
		return v.Table
	}
	scCache.mu.RUnlock()

	return cachedTableName(t)
}

func TableNameOf(model any) string {
	if model == nil {
		return ""
	}
	t := reflect.TypeOf(model)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	return LookupTableName(t)
}

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

func ParseType(t reflect.Type) (*Schema, error) {
	if t == nil {
		return nil, ErrInvalidModel
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("corm: model must be struct, got %s", t.Kind().String())
	}

	scCache.mu.RLock()
	if v, ok := scCache.items[t]; ok {
		scCache.mu.RUnlock()
		return v, nil
	}
	scCache.mu.RUnlock()

	e := &parseEntry{done: make(chan struct{})}
	pg.mu.Lock()
	scCache.mu.RLock()
	if v, ok := scCache.items[t]; ok {
		scCache.mu.RUnlock()
		pg.mu.Unlock()
		return v, nil
	}
	scCache.mu.RUnlock()
	if existing, ok := pg.items[t]; ok {
		pg.mu.Unlock()
		<-existing.done
		if existing.err != nil {
			return nil, existing.err
		}
		return existing.s, nil
	}
	pg.items[t] = e
	pg.mu.Unlock()

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
		pg.mu.Lock()
		delete(pg.items, t)
		pg.mu.Unlock()
		return nil, err
	}

	scCache.mu.Lock()
	if scCache.count >= maxSchemaCacheEntries {
		evictSchemaRandom(scCache.items, scCache.count/4)
		scCache.count = len(scCache.items)
	}
	if _, ok := scCache.items[t]; !ok {
		scCache.items[t] = s
		scCache.count++
	}
	scCache.mu.Unlock()

	e.s = s
	e.err = nil
	close(e.done)
	pg.mu.Lock()
	delete(pg.items, t)
	pg.mu.Unlock()
	return s, nil
}

var ErrInvalidModel = &schemaError{"corm: model must be struct or pointer to struct"}

type schemaError struct{ msg string }

func (e *schemaError) Error() string { return e.msg }

func parseSlow(t reflect.Type) (*Schema, error) {
	s := &Schema{
		Type:     t,
		Table:    cachedTableName(t),
		ByColumn: map[string]*Field{},
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
	for i := range t.NumField() {
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
			col = ToSnake(sf.Name)
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
	idx := make([]int, len(parent)+1)
	copy(idx, parent)
	idx[len(parent)] = i
	return idx
}

func evictSchemaRandom(m map[reflect.Type]*Schema, n int) {
	for k := range m {
		delete(m, k)
		n--
		if n <= 0 {
			break
		}
	}
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
	return ToSnake(t.Name())
}

type snakeCache struct {
	mu    sync.RWMutex
	items map[string]string
	count int
}

var snCache = &snakeCache{items: make(map[string]string, 256)}

const maxSnakeCacheSize = 1024

// ToSnake converts a string to snake_case.
func ToSnake(s string) string {
	if s == "" {
		return ""
	}

	// Check cache first for common identifiers
	if len(s) <= 32 {
		snCache.mu.RLock()
		if cached, ok := snCache.items[s]; ok {
			snCache.mu.RUnlock()
			return cached
		}
		snCache.mu.RUnlock()
	}

	// Fast path: check if all ASCII and already snake_case
	allASCII := true
	hasUpper := false
	for i := range s {
		c := s[i]
		if c >= 0x80 {
			allASCII = false
			break
		}
		if c >= 'A' && c <= 'Z' {
			hasUpper = true
		}
	}

	// If already snake_case (no uppercase), return as-is
	if allASCII && !hasUpper {
		// Cache common identifiers
		if len(s) <= 32 {
			snCache.mu.Lock()
			if snCache.count < maxSnakeCacheSize {
				if _, ok := snCache.items[s]; !ok {
					snCache.items[s] = s
					snCache.count++
				}
			}
			snCache.mu.Unlock()
		}
		return s
	}

	var result string
	if allASCII {
		result = toSnakeASCII(s)
	} else {
		result = toSnakeUnicode(s)
	}

	// Cache the result for common identifiers
	if len(s) <= 32 && len(result) <= 64 {
		snCache.mu.Lock()
		if snCache.count < maxSnakeCacheSize {
			if _, ok := snCache.items[s]; !ok {
				snCache.items[s] = result
				snCache.count++
			}
		}
		snCache.mu.Unlock()
	}
	return result
}

// toSnakeASCII converts an ASCII string to snake_case.
func toSnakeASCII(s string) string {
	buf := make([]byte, 0, len(s)+4)
	prevLower := false
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			nextLower := i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z'
			if i > 0 && (prevLower || nextLower) {
				buf = append(buf, '_')
			}
			buf = append(buf, c+('a'-'A'))
			prevLower = false
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			prevLower = c >= 'a' && c <= 'z'
			buf = append(buf, c)
			continue
		}
		if c == '_' {
			buf = append(buf, '_')
			prevLower = false
			continue
		}
	}

	return string(buf)
}

// toSnakeUnicode converts a Unicode string to snake_case.
func toSnakeUnicode(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(runes) + 8)

	prevLower := false
	for i := range runes {
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
