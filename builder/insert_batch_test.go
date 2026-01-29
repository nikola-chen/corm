package builder_test

import (
	"reflect"
	"testing"
)

type BatchUser struct {
	ID   int    `db:"id,pk"`
	Name string `db:"name"`
	Age  int    `db:"age,omitempty"`
}

func (BatchUser) TableName() string { return "users" }

func TestInsertModelsBatch_Postgres(t *testing.T) {
	users := []BatchUser{
		{Name: "a", Age: 0},
		{Name: "b", Age: 10},
	}
	q := pgQB().Insert("").Models(users)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := `INSERT INTO "users" ("name", "age") VALUES ($1, $2), ($3, $4)`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{"a", 0, "b", 10}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestInsertModelsBatch_WithColumns(t *testing.T) {
	users := []*BatchUser{
		{Name: "a", Age: 0},
		{Name: "b", Age: 10},
	}
	q := mysqlQB().Insert("users").Columns("name").Models(users)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := "INSERT INTO `users` (`name`) VALUES (?), (?)"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{"a", "b"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestInsertMapsBatch_InferColumns(t *testing.T) {
	rows := []map[string]any{
		{"name": "a", "age": 1},
		{"name": "b", "age": 2},
	}
	q := pgQB().Insert("users").Maps(rows)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := `INSERT INTO "users" ("age", "name") VALUES ($1, $2), ($3, $4)`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{1, "a", 2, "b"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestInsertMapsBatch_MissingColumn(t *testing.T) {
	rows := []map[string]any{
		{"name": "a", "age": 1},
		{"name": "b"},
	}
	q := mysqlQB().Insert("users").Maps(rows)
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

type BadTableNameUser struct {
	Name string `db:"name"`
}

func (BadTableNameUser) TableName() string { return "bad name" }

func TestInsertModelsBatch_ExplicitTableWinsOverModelTableName(t *testing.T) {
	users := []BadTableNameUser{{Name: "a"}}
	q := mysqlQB().Insert("users").Models(users)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	if sqlStr != "INSERT INTO `users` (`name`) VALUES (?)" {
		t.Fatalf("got: %s", sqlStr)
	}
	if !reflect.DeepEqual(args, []any{"a"}) {
		t.Fatalf("got args: %v", args)
	}
}

func TestInsertModelsBatch_EmptyTableAndInvalidModelTableName(t *testing.T) {
	users := []BadTableNameUser{{Name: "a"}}
	q := mysqlQB().Insert("").Models(users)
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestInsertModelsBatch_EmptyTableAnonymousStruct(t *testing.T) {
	users := []struct {
		Name string `db:"name"`
	}{{Name: "a"}}
	q := mysqlQB().Insert("").Models(users)
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}
