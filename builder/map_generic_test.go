package builder_test

import (
	"reflect"
	"testing"
)

func TestInsertMap_Postgres_SortsColumns(t *testing.T) {
	m := map[string]any{"name": "a", "age": 1}
	q := pgQB().Insert("users").Map(m)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := `INSERT INTO "users" ("age", "name") VALUES ($1, $2)`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{1, "a"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestInsertMap_RejectsBadColumn(t *testing.T) {
	m := map[string]any{"id;drop": 1}
	q := mysqlQB().Insert("users").Map(m)
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestUpdateSetMapIdent_MySQL(t *testing.T) {
	m := map[string]any{"age": 1}
	q := mysqlQB().Update("users").Map(m).Where("id = ?", 7)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := "UPDATE `users` SET `age` = ? WHERE (id = ?)"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{1, 7}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestUpdateSetMapIdent_RejectsBadColumn(t *testing.T) {
	m := map[string]any{"age;drop": 1}
	q := mysqlQB().Update("users").Map(m).Where("id = ?", 7)
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}
