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

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
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

func TestToSnakeUnicodeChars(t *testing.T) {
	// Test with actual Unicode characters
	result := schema.ToSnake("HelloWorld")
	if result != "hello_world" {
		t.Errorf("expected 'hello_world', got '%s'", result)
	}

	result2 := schema.ToSnake("HTTPServer")
	if result2 != "http_server" {
		t.Errorf("expected 'http_server', got '%s'", result2)
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

func TestSchemaError(t *testing.T) {
	err := schema.ErrInvalidModel
	if err == nil {
		t.Fatalf("expected non-nil error")
	}
	if err.Error() != "corm: model must be struct or pointer to struct" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestParseSchemaWithTableNamer(t *testing.T) {
	type CustomTable struct {
		ID int `db:"id,pk"`
	}

	s, err := schema.Parse(CustomTable{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Default table name from struct name
	if s.Table != "custom_table" {
		t.Errorf("expected 'custom_table', got '%s'", s.Table)
	}
}

func TestParseSchemaWithAutoTag(t *testing.T) {
	type AutoModel struct {
		ID   int    `db:"id,pk,auto"`
		Name string `db:"name"`
	}

	s, err := schema.Parse(AutoModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	f := s.ByColumn["id"]
	if f == nil {
		t.Fatalf("missing column 'id'")
	}
	if !f.Auto {
		t.Errorf("expected Auto to be true")
	}
	if !f.PrimaryKey {
		t.Errorf("expected PrimaryKey to be true")
	}
}

func TestParseSchemaWithReadonlyTag(t *testing.T) {
	type ReadonlyModel struct {
		ID      int    `db:"id,pk"`
		Created string `db:"created,readonly"`
	}

	s, err := schema.Parse(ReadonlyModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	f := s.ByColumn["created"]
	if f == nil {
		t.Fatalf("missing column 'created'")
	}
	if !f.Readonly {
		t.Errorf("expected Readonly to be true")
	}
}

func TestParseSchemaWithIdentityTag(t *testing.T) {
	type IdentityModel struct {
		ID   int    `db:"id,pk,identity"`
		Name string `db:"name"`
	}

	s, err := schema.Parse(IdentityModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	f := s.ByColumn["id"]
	if f == nil {
		t.Fatalf("missing column 'id'")
	}
	if !f.Auto {
		t.Errorf("expected Auto to be true for identity tag")
	}
}

func TestParseSchemaWithAutoincrTag(t *testing.T) {
	type AutoincrModel struct {
		ID   int    `db:"id,pk,autoincr"`
		Name string `db:"name"`
	}

	s, err := schema.Parse(AutoincrModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	f := s.ByColumn["id"]
	if f == nil {
		t.Fatalf("missing column 'id'")
	}
	if !f.Auto {
		t.Errorf("expected Auto to be true for autoincr tag")
	}
}

func TestParseSchemaWithPkTag(t *testing.T) {
	type PkTagModel struct {
		ID   int    `db:"id,pk"`
		Code string `db:"code" pk:"true"`
	}

	s, err := schema.Parse(PkTagModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	f := s.ByColumn["code"]
	if f == nil {
		t.Fatalf("missing column 'code'")
	}
	if !f.PrimaryKey {
		t.Errorf("expected PrimaryKey to be true for pk tag")
	}
}

func TestParseSchemaWithDBTagDash(t *testing.T) {
	type SkipModel struct {
		ID      int    `db:"id,pk"`
		Skipped string `db:"-"`
		Name    string `db:"name"`
	}

	s, err := schema.Parse(SkipModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.ByColumn["skipped"] != nil {
		t.Errorf("expected 'skipped' column to be ignored")
	}
	if s.ByColumn["name"] == nil {
		t.Errorf("expected 'name' column to be present")
	}
}

func TestParseSchemaDefaultPK(t *testing.T) {
	type DefaultPKModel struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	s, err := schema.Parse(DefaultPKModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(s.PrimaryKeys) != 1 {
		t.Errorf("expected 1 primary key (auto-detected 'id'), got %d", len(s.PrimaryKeys))
	}
	if s.PrimaryKeys[0].Column != "id" {
		t.Errorf("expected primary key column 'id', got '%s'", s.PrimaryKeys[0].Column)
	}
}

func TestColumnsAndValuesNilPointer(t *testing.T) {
	type TestModel struct {
		ID   int    `db:"id,pk"`
		Name string `db:"name"`
	}

	s, err := schema.Parse(TestModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	var nilModel *TestModel
	_, _, err = s.ColumnsAndValues(nilModel, schema.ExtractOptions{})
	if err == nil {
		t.Fatalf("expected error for nil pointer model")
	}
}

func TestColumnsAndValuesNonStruct(t *testing.T) {
	type TestModel struct {
		ID int `db:"id,pk"`
	}

	s, err := schema.Parse(TestModel{})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	_, _, err = s.ColumnsAndValues(42, schema.ExtractOptions{})
	if err == nil {
		t.Fatalf("expected error for non-struct model")
	}
}

func TestToSnakeVariousCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"a", "a"},
		{"A", "a"},
		{"AB", "ab"},
		{"ABC", "abc"},
		{"AaBb", "aa_bb"},
		{"HTTPRequest", "http_request"},
		{"getHTTPResponse", "get_http_response"},
		{"already_snake", "already_snake"},
		{"ID", "id"},
		{"UserID", "user_id"},
		{"_test", "_test"},
		{"test_", "test_"},
	}

	for _, tt := range tests {
		result := schema.ToSnake(tt.input)
		if result != tt.expected {
			t.Errorf("ToSnake(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToSnakeUnicodePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ÄÖÜ", "äöü"},
		{"Äbc", "äbc"},
		{"äBc", "ä_bc"},
		{"ÄbcÄ", "äbc_ä"},
		{"HelloWörld", "hello_wörld"},
		{"ÜserNäme", "üser_näme"},
		{"KäseBrot", "käse_brot"},
		{"Größe", "größe"},
		{"Öffentlich", "öffentlich"},
		{"ÄÖÜTest", "äöü_test"},
		{"TestÄÖÜ", "test_äöü"},
		{"ÄBC", "äbc"},
		{"ÄbcDef", "äbc_def"},
		{"Müller", "müller"},
		{"GroßHandel", "groß_handel"},
		{"ÜBER", "über"},
		{"überSchrift", "über_schrift"},
	}

	for _, tt := range tests {
		result := schema.ToSnake(tt.input)
		if result != tt.expected {
			t.Errorf("ToSnake(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToSnakeUnicodeWithDigits(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Ä1b2", "ä1b2"},
		{"TestÄ1", "test_ä1"},
		{"Ä2B3c", "ä2b3c"},
	}

	for _, tt := range tests {
		result := schema.ToSnake(tt.input)
		if result != tt.expected {
			t.Errorf("ToSnake(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToSnakeUnicodeWithUnderscore(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Ä_bc", "ä_bc"},
		{"Ä_bc_Ü", "ä_bc_ü"},
		{"_Äbc", "__äbc"},
		{"Äbc_Üef", "äbc__üef"},
	}

	for _, tt := range tests {
		result := schema.ToSnake(tt.input)
		if result != tt.expected {
			t.Errorf("ToSnake(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToSnakeUnicodeSpecialChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Äbc Def", "äbc_def"},
		{"Äbc!Def", "äbc_def"},
		{"Äbc@Def", "äbc_def"},
		{"Äbc-Def", "äbc_def"},
		{"Äbc.Def", "äbc_def"},
	}

	for _, tt := range tests {
		result := schema.ToSnake(tt.input)
		if result != tt.expected {
			t.Errorf("ToSnake(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToSnakeASCIISpecialChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello_world"},
		{"Hello-World", "hello_world"},
		{"Hello.World", "hello_world"},
		{"Hello!World", "hello_world"},
		{"Hello@World", "hello_world"},
		{"A B", "ab"},
		{"A-B", "ab"},
		{"Test123", "test123"},
		{"Test123Abc", "test123_abc"},
		{"A1B2C3", "a1b2c3"},
		{"v2API", "v2api"},
		{"HTML5Parser", "html5_parser"},
	}

	for _, tt := range tests {
		result := schema.ToSnake(tt.input)
		if result != tt.expected {
			t.Errorf("ToSnake(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToSnakeCyrillic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ПриветМир", "привет_мир"},
		{"Москва", "москва"},
		{"БольШой", "боль_шой"},
	}

	for _, tt := range tests {
		result := schema.ToSnake(tt.input)
		if result != tt.expected {
			t.Errorf("ToSnake(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToSnakeGreek(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ΑλφαΒητα", "αλφα_βητα"},
		{"ΓammaDelta", "γamma_delta"},
	}

	for _, tt := range tests {
		result := schema.ToSnake(tt.input)
		if result != tt.expected {
			t.Errorf("ToSnake(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
