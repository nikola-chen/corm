package schema_test

import (
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

