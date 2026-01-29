package builder_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/dialect"
)

func TestAPI_MySQL(t *testing.T) {
	bd := builder.MySQL()

	// Verify dialect is set correctly (indirectly via SQL generation)
	q := bd.Update("users").Set("name", "bob").Where("id = ?", 1)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	// MySQL uses backticks and ?
	wantSQL := "UPDATE `users` SET `name` = ? WHERE (id = ?)"
	if sqlStr != wantSQL {
		t.Errorf("got %q, want %q", sqlStr, wantSQL)
	}
	wantArgs := []any{"bob", 1}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("got %v, want %v", args, wantArgs)
	}
}

func TestAPI_Postgres(t *testing.T) {
	bd := builder.Postgres()

	q := bd.Select("name").From("users").Where("id = ?", 1)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	// Postgres uses double quotes and $1
	wantSQL := `SELECT "name" FROM "users" WHERE (id = $1)`
	if sqlStr != wantSQL {
		t.Errorf("got %q, want %q", sqlStr, wantSQL)
	}
	wantArgs := []any{1}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("got %v, want %v", args, wantArgs)
	}
}

func TestAPI_NewAPI(t *testing.T) {
	// Test NewAPI with nil executor (safe for building SQL)
	bd := builder.NewAPI(dialect.MustGet("mysql"), nil)

	q := bd.Delete("users").Where("id = ?", 1)
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	if sqlStr != "DELETE FROM `users` WHERE (id = ?)" {
		t.Errorf("got %q", sqlStr)
	}
}

func ExampleAPI_buildSQL() {
	bd := builder.Postgres()
	sqlStr, args, err := bd.Select("name").From("users").Where("id = ?", 1).SQL()
	if err != nil {
		panic(err)
	}
	fmt.Println(sqlStr)
	fmt.Println(args)
	// Output:
	// SELECT "name" FROM "users" WHERE (id = $1)
	// [1]
}

func ExampleAPI_exec() {
	exec := &mockExecutor{}
	bd := builder.NewAPI(dialect.MustGet("mysql"), exec)
	_, _ = bd.Delete("users").Where("id = ?", 1).Exec(context.Background())
	fmt.Println(exec.sql)
	fmt.Println(exec.args)
	// Output:
	// DELETE FROM `users` WHERE (id = ?)
	// [1]
}
