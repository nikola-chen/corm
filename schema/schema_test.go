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
	for i, s := range out {
		if s != first {
			t.Fatalf("schema pointer mismatch at %d", i)
		}
	}
}
