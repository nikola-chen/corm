package builder_test

import (
	"testing"

	"github.com/nikola-chen/corm/builder"
)

func TestNilDialectDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()

	_, _, _ = builder.Insert(nil, nil, "users").Columns("id").Values(1).SQL()
	_, _, _ = builder.Update(nil, nil, "users").Set("id", 1).Where("id = ?", 1).SQL()
	_, _, _ = builder.Delete(nil, nil, "users").WhereEq("id", 1).SQL()
	_, _, _ = builder.Select(nil, nil, "id").From("users").WhereEq("id", 1).SQL()
}
