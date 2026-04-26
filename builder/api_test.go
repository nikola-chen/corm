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

func TestAPI_New(t *testing.T) {
	bd := builder.New(dialect.MustGet("mysql"))
	if bd == nil {
		t.Fatal("New returned nil")
	}
	q := bd.Select("id").From("users")
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	if sqlStr != "SELECT `id` FROM `users`" {
		t.Errorf("got %q", sqlStr)
	}
}

func TestAPI_For(t *testing.T) {
	exec := &mockExecutor{}
	bd := builder.For(dialect.MustGet("mysql"), exec)
	if bd == nil {
		t.Fatal("For returned nil")
	}
	q := bd.Select("id").From("users")
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	if sqlStr != "SELECT `id` FROM `users`" {
		t.Errorf("got %q", sqlStr)
	}
}

func TestAPI_MustFor(t *testing.T) {
	exec := &mockExecutor{}
	bd := builder.MustFor(dialect.MustGet("mysql"), exec)
	if bd == nil {
		t.Fatal("MustFor returned nil")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil dialect in MustFor")
		}
	}()
	builder.MustFor(nil, exec)
}

func TestAPI_MustDialect(t *testing.T) {
	bd := builder.MustDialect("mysql")
	if bd == nil {
		t.Fatal("MustDialect returned nil")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unknown dialect in MustDialect")
		}
	}()
	builder.MustDialect("nonexistent")
}

func TestAPI_Dialect(t *testing.T) {
	d := dialect.MustGet("mysql")
	bd := builder.NewAPI(d, nil)

	got := bd.Dialect()
	if got != d {
		t.Errorf("Dialect() = %v, want %v", got, d)
	}

	var nilAPI *builder.API
	if nilAPI.Dialect() != nil {
		t.Error("nil API Dialect() should return nil")
	}
}

func TestAPI_Err(t *testing.T) {
	bd := builder.Dialect("nonexistent")
	if bd.Err() == nil {
		t.Error("expected error for nonexistent dialect")
	}

	bdOK := builder.MySQL()
	if bdOK.Err() != nil {
		t.Errorf("expected nil error, got %v", bdOK.Err())
	}

	var nilAPI *builder.API
	if nilAPI.Err() == nil {
		t.Error("nil API Err() should return error")
	}
}

func TestAPI_NilDialect(t *testing.T) {
	bd := builder.NewAPI(nil, nil)
	if bd.Err() == nil {
		t.Error("expected error for nil dialect")
	}
	q := bd.Select("id").From("users")
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error from builder with nil dialect")
	}
}

func TestAPI_DialectEmpty(t *testing.T) {
	bd := builder.Dialect("")
	if bd.Err() == nil {
		t.Error("expected error for empty dialect")
	}
}

func TestAPI_NilAPI(t *testing.T) {
	var nilAPI *builder.API

	q := nilAPI.Select("id")
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error from nil API Select")
	}

	qi := nilAPI.Insert("users")
	_, _, err = qi.SQL()
	if err == nil {
		t.Error("expected error from nil API Insert")
	}

	qu := nilAPI.Update("users")
	_, _, err = qu.SQL()
	if err == nil {
		t.Error("expected error from nil API Update")
	}

	qd := nilAPI.Delete("users")
	_, _, err = qd.SQL()
	if err == nil {
		t.Error("expected error from nil API Delete")
	}
}
