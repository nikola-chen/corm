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
	Register(name, mysqlDialect{})
	if d, ok := Get(name); !ok || d.Name() != "mysql" {
		t.Fatalf("unexpected dialect: ok=%v name=%v", ok, d.Name())
	}
}
