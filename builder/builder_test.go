package builder_test

import (
	"reflect"
	"testing"

	"corm/builder"
	"corm/dialect"
)

func TestSelectPostgresPlaceholders(t *testing.T) {
	d, ok := dialect.Get("postgres")
	if !ok {
		t.Fatalf("postgres dialect not registered")
	}

	q := builder.Select(nil, d, "id", "name").
		From("users").
		Where("age > ?", 18).
		OrderBy("id", "DESC").
		LimitOffset(10, 5)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT "id", "name" FROM "users" WHERE (age > $1) ORDER BY "id" DESC LIMIT $2 OFFSET $3`
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}

	wantArgs := []any{18, 10, 5}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\nwant: %#v\ngot : %#v", wantArgs, args)
	}
}

func TestSelectMySQLPlaceholders(t *testing.T) {
	d, ok := dialect.Get("mysql")
	if !ok {
		t.Fatalf("mysql dialect not registered")
	}

	q := builder.Select(nil, d, "id", "name").
		From("users").
		Where("age > ?", 18).
		OrderBy("id", "DESC").
		LimitOffset(10, 5)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := "SELECT `id`, `name` FROM `users` WHERE (age > ?) ORDER BY `id` DESC LIMIT ? OFFSET ?"
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}

	wantArgs := []any{18, 10, 5}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\nwant: %#v\ngot : %#v", wantArgs, args)
	}
}

func TestInsertReturningPostgres(t *testing.T) {
	d, _ := dialect.Get("postgres")

	q := builder.InsertInto(nil, d, "users").
		Columns("name").
		Values("alice").
		Returning("id")

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `INSERT INTO "users" ("name") VALUES ($1) RETURNING "id"`
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}
	wantArgs := []any{"alice"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\nwant: %#v\ngot : %#v", wantArgs, args)
	}
}

func TestInsertReturningMySQLIgnored(t *testing.T) {
	d, _ := dialect.Get("mysql")

	q := builder.InsertInto(nil, d, "users").
		Columns("name").
		Values("alice").
		Returning("id")

	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := "INSERT INTO `users` (`name`) VALUES (?)"
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}
}
