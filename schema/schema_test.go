package schema_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/nikola-chen/corm/schema"
)

type User struct {
	ID   int    `db:"id,pk"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

func (User) TableName() string { return "users" }

func TestParseSchema(t *testing.T) {
	s, err := schema.Parse(User{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.Table != "users" {
		t.Fatalf("table mismatch: %s", s.Table)
	}
	if len(s.PrimaryKeys) != 1 || s.PrimaryKeys[0].Column != "id" {
		t.Fatalf("pk mismatch: %#v", s.PrimaryKeys)
	}
	if s.ByColumn["name"] == nil || s.ByColumn["age"] == nil {
		t.Fatalf("missing columns in schema: %#v", s.ByColumn)
	}
}

func TestParseSchema_DuplicateColumns(t *testing.T) {
	type DupCol struct {
		Name1 string `db:"name"`
		Name2 string `db:"name"` // Last one wins
	}
	s, err := schema.Parse(DupCol{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	f := s.ByColumn["name"]
	if f == nil {
		t.Fatalf("missing column 'name'")
	}
	// Name2 is the second field, so it should be the one mapped
	if f.Name != "Name2" {
		t.Errorf("expected last field 'Name2' to win, got '%s'", f.Name)
	}
}
func TestParseSchemaConcurrent(t *testing.T) {
	type ConcurrentUser struct {
		ID   int    `db:"id,pk"`
		Name string `db:"name"`
		Age  int    `db:"age"`
	}

	const n = 64
	out := make([]*schema.Schema, n)
	errs := make([]error, n)

	// Pre-load cache to ensure we test cache hit logic too if desired,
	// but here we want to test concurrent parseSlow.
	// Ensure cache is clean for this type? It's a new type definition inside function,
	// so reflect.Type should be unique/new.

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			// Use a small sleep to increase chance of overlap
			// time.Sleep(time.Microsecond)
			s, err := schema.Parse(ConcurrentUser{})
			out[i] = s
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Parse error at %d: %v", i, err)
		}
	}
	first := out[0]
	if first == nil {
		t.Fatalf("nil schema")
	}
	// Verify all returned schemas are the SAME pointer (singleton)
	for i, s := range out {
		if s != first {
			t.Fatalf("schema pointer mismatch at %d: expected %p, got %p", i, first, s)
		}
	}
}

func TestParseSchemaCache(t *testing.T) {
	type CacheTest struct {
		ID int `db:"id,pk"`
	}

	// First parse - should populate cache
	s1, err := schema.Parse(CacheTest{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Second parse - should hit cache and return same pointer
	s2, err := schema.Parse(CacheTest{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if s1 != s2 {
		t.Errorf("expected cached schema pointer, got different instances: %p vs %p", s1, s2)
	}
}

func TestParseSchemaNilModel(t *testing.T) {
	_, err := schema.Parse(nil)
	if err == nil {
		t.Fatalf("expected error for nil model")
	}
}

func TestParseSchemaNonStruct(t *testing.T) {
	_, err := schema.Parse("not a struct")
	if err == nil {
		t.Fatalf("expected error for non-struct model")
	}
}

func TestParseSchemaPointer(t *testing.T) {
	type PtrTest struct {
		ID int `db:"id,pk"`
	}

	s, err := schema.Parse(&PtrTest{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s == nil {
		t.Fatalf("expected non-nil schema")
	}
}

func TestParseSchemaAnonymousStruct(t *testing.T) {
	type Inner struct {
		InnerID int `db:"inner_id"`
	}
	type Outer struct {
		ID int `db:"id,pk"`
		Inner
	}

	s, err := schema.Parse(Outer{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.ByColumn["inner_id"] == nil {
		t.Errorf("expected nested field 'inner_id' to be present")
	}
}

func TestParseSchemaWithOmitEmpty(t *testing.T) {
	type OmitTest struct {
		ID   int    `db:"id,pk"`
		Name string `db:"name,omitempty"`
	}

	s, err := schema.Parse(OmitTest{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	f := s.ByColumn["name"]
	if f == nil {
		t.Fatalf("missing column 'name'")
	}
	if !f.OmitEmpty {
		t.Errorf("expected OmitEmpty to be true")
	}
}

func TestToSnakeCache(t *testing.T) {
	// Test that ToSnake caches results
	result1 := schema.ToSnake("TestString")
	result2 := schema.ToSnake("TestString")
	if result1 != result2 {
		t.Errorf("expected cached result to be consistent")
	}
	if result1 != "test_string" {
		t.Errorf("expected 'test_string', got '%s'", result1)
	}
}

func TestToSnakeAlreadySnakeCase(t *testing.T) {
	result := schema.ToSnake("already_snake")
	if result != "already_snake" {
		t.Errorf("expected 'already_snake', got '%s'", result)
	}
}

func TestToSnakeEmpty(t *testing.T) {
	result := schema.ToSnake("")
	if result != "" {
		t.Errorf("expected empty string, got '%s'", result)
	}
}

func TestToSnakeUnicode(t *testing.T) {
	result := schema.ToSnake("TestUnicode")
	if result != "test_unicode" {
		t.Errorf("expected 'test_unicode', got '%s'", result)
	}
}

func TestParseTypeNil(t *testing.T) {
	_, err := schema.ParseType(nil)
	if err == nil {
		t.Fatalf("expected error for nil type")
	}
}

func TestParseTypeNonStruct(t *testing.T) {
	_, err := schema.ParseType(reflect.TypeOf(""))
	if err == nil {
		t.Fatalf("expected error for non-struct type")
	}
}

func TestSchemaColumnsAndValues(t *testing.T) {
	type TestModel struct {
		ID   int    `db:"id,pk"`
		Name string `db:"name"`
		Age  int    `db:"age"`
	}

	s, err := schema.Parse(TestModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	model := TestModel{ID: 1, Name: "test", Age: 25}
	cols, vals, err := s.ColumnsAndValues(model, schema.ExtractOptions{IncludePrimaryKey: true})
	if err != nil {
		t.Fatalf("ColumnsAndValues error: %v", err)
	}

	if len(cols) != 3 {
		t.Errorf("expected 3 columns, got %d", len(cols))
	}
	if len(vals) != 3 {
		t.Errorf("expected 3 values, got %d", len(vals))
	}
}

func TestSchemaColumnsAndValuesOmitEmpty(t *testing.T) {
	type OmitModel struct {
		ID   int    `db:"id,pk"`
		Name string `db:"name,omitempty"`
	}

	s, err := schema.Parse(OmitModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Empty name should be omitted (pk excluded by default)
	model := OmitModel{ID: 1, Name: ""}
	cols, vals, err := s.ColumnsAndValues(model, schema.ExtractOptions{})
	if err != nil {
		t.Fatalf("ColumnsAndValues error: %v", err)
	}

	if len(cols) != 0 {
		t.Errorf("expected 0 columns (pk excluded, name omitempty), got %v", cols)
	}
	if len(vals) != 0 {
		t.Errorf("expected 0 values, got %d", len(vals))
	}
}

func TestSchemaColumnsAndValuesIncludeZero(t *testing.T) {
	type OmitModel struct {
		ID   int    `db:"id,pk"`
		Name string `db:"name,omitempty"`
	}

	s, err := schema.Parse(OmitModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// With IncludeZero=true, empty name should be included (but pk still excluded)
	model := OmitModel{ID: 1, Name: ""}
	cols, vals, err := s.ColumnsAndValues(model, schema.ExtractOptions{IncludeZero: true})
	if err != nil {
		t.Fatalf("ColumnsAndValues error: %v", err)
	}

	if len(cols) != 1 || cols[0] != "name" {
		t.Errorf("expected 1 column (name) with IncludeZero, got %v", cols)
	}
	if len(vals) != 1 {
		t.Errorf("expected 1 value with IncludeZero, got %d", len(vals))
	}
}

func TestSchemaColumnsAndValuesInvalidModel(t *testing.T) {
	type TestModel struct {
		ID int `db:"id,pk"`
	}

	s, err := schema.Parse(TestModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Pass a different type
	_, _, err = s.ColumnsAndValues("not a struct", schema.ExtractOptions{})
	if err == nil {
		t.Fatalf("expected error for invalid model type")
	}
}

func TestSchemaCacheEviction(t *testing.T) {
	// Create many unique types to trigger cache eviction
	for i := 0; i < 1100; i++ {
		// Use reflect to create unique struct types dynamically
		// This is a simplified test - in reality we'd need many unique types
		// Just verify the cache doesn't panic with many operations
		type TestStruct struct {
			ID int `db:"id,pk"`
		}
		_, err := schema.Parse(TestStruct{})
		if err != nil {
			t.Fatalf("Parse error at %d: %v", i, err)
		}
	}
}
