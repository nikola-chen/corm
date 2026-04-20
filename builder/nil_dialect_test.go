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

	_, _, _ = builder.NewAPI(nil, nil).Insert("users").Columns("id").Values(1).SQL()
	_, _, _ = builder.NewAPI(nil, nil).Update("users").Set("id", 1).Where("id = ?", 1).SQL()
	_, _, _ = builder.NewAPI(nil, nil).Delete("users").WhereEq("id", 1).SQL()
	_, _, _ = builder.NewAPI(nil, nil).Select("id").From("users").WhereEq("id", 1).SQL()
}
