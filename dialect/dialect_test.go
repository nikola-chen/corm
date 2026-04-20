package dialect

import "testing"

func TestGetBuiltInDialects(t *testing.T) {
	if _, ok := Get("mysql"); !ok {
		t.Fatalf("expected mysql dialect to be registered")
	}
	if _, ok := Get("postgres"); !ok {
		t.Fatalf("expected postgres dialect to be registered")
	}
	if _, ok := Get("postgresql"); !ok {
		t.Fatalf("expected postgresql dialect to be registered")
	}
}

func TestRegisterAndGet(t *testing.T) {
	const name = "corm_test_dialect"
	Register(name, &mysqlDialect{})
	if d, ok := Get(name); !ok || d.Name() != "mysql" {
		t.Fatalf("unexpected dialect: ok=%v name=%v", ok, d.Name())
	}
}

func TestMustGet(t *testing.T) {
	d := MustGet("mysql")
	if d.Name() != "mysql" {
		t.Fatalf("expected mysql, got %s", d.Name())
	}
}

func TestMustGetPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustGet to panic for unknown dialect")
		}
	}()
	MustGet("nonexistent")
}

func TestMySQLQuoteIdent(t *testing.T) {
	d := &mysqlDialect{}

	tests := []struct {
		input string
		want  string
	}{
		{"id", "`id`"},
		{"", "``"},
		{"table`name", "`table``name`"},
		{"simple", "`simple`"},
	}

	for _, tt := range tests {
		got := d.QuoteIdent(tt.input)
		if got != tt.want {
			t.Errorf("QuoteIdent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPostgresQuoteIdent(t *testing.T) {
	d := &postgresDialect{}

	tests := []struct {
		input string
		want  string
	}{
		{"id", `"id"`},
		{"", `""`},
		{`table"name`, `"table""name"`},
		{"simple", `"simple"`},
	}

	for _, tt := range tests {
		got := d.QuoteIdent(tt.input)
		if got != tt.want {
			t.Errorf("QuoteIdent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMySQLPlaceholder(t *testing.T) {
	d := &mysqlDialect{}
	if d.Placeholder(1) != "?" {
		t.Errorf("expected ?, got %s", d.Placeholder(1))
	}
}

func TestPostgresPlaceholder(t *testing.T) {
	d := &postgresDialect{}

	tests := []struct {
		n    int
		want string
	}{
		{1, "$1"},
		{20, "$20"},
		{21, "$21"},
		{100, "$100"},
	}

	for _, tt := range tests {
		got := d.Placeholder(tt.n)
		if got != tt.want {
			t.Errorf("Placeholder(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestSupportsReturning(t *testing.T) {
	mysql := &mysqlDialect{}
	pg := &postgresDialect{}

	if mysql.SupportsReturning() {
		t.Error("mysql should not support RETURNING")
	}
	if !pg.SupportsReturning() {
		t.Error("postgres should support RETURNING")
	}
}

func TestQuoteIdentCache(t *testing.T) {
	d := &mysqlDialect{}

	result1 := d.QuoteIdent("cached_col")
	result2 := d.QuoteIdent("cached_col")
	if result1 != result2 {
		t.Errorf("cached results should be identical")
	}
	if result1 != "`cached_col`" {
		t.Errorf("expected `cached_col`, got %s", result1)
	}
}
