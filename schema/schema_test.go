package schema_test

import (
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
