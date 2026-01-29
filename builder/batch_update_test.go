package builder_test

import (
	"reflect"
	"strings"
	"testing"
)

type BatchUpdateUser struct {
	ID   int    `db:"id,pk"`
	Name string `db:"name"`
	Age  int    `db:"age,omitempty"`
}

func (BatchUpdateUser) TableName() string { return "users" }

func TestBatchUpdateModels_Postgres_OmitEmptyKeepsValue(t *testing.T) {
	users := []BatchUpdateUser{
		{ID: 1, Name: "a", Age: 0},
		{ID: 2, Name: "b", Age: 10},
	}
	q := pgQB().Update("").Models(users)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `UPDATE "users" SET "name" = CASE "id" WHEN $1 THEN $2 WHEN $3 THEN $4 ELSE "name" END, "age" = CASE "id" WHEN $5 THEN "age" WHEN $6 THEN $7 ELSE "age" END WHERE "id" IN ($8, $9)`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{1, "a", 2, "b", 1, 2, 10, 1, 2}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestBatchUpdateMaps_MySQL_MissingKeepsValue(t *testing.T) {
	rows := []map[string]any{
		{"id": 1, "name": "a", "age": 1},
		{"id": 2, "name": "b"},
	}
	q := mysqlQB().Update("users").Maps(rows)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := "UPDATE `users` SET `age` = CASE `id` WHEN ? THEN ? WHEN ? THEN `age` ELSE `age` END, `name` = CASE `id` WHEN ? THEN ? WHEN ? THEN ? ELSE `name` END WHERE `id` IN (?, ?)"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{1, 1, 2, 1, "a", 2, "b", 1, 2}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestBatchUpdateMaps_MySQL_WithExtraWhere(t *testing.T) {
	rows := []map[string]any{
		{"id": 1, "name": "a"},
		{"id": 2, "name": "b"},
	}
	q := mysqlQB().Update("users").Maps(rows).Where("status = ?", 1)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := "UPDATE `users` SET `name` = CASE `id` WHEN ? THEN ? WHEN ? THEN ? ELSE `name` END WHERE `id` IN (?, ?) AND (status = ?)"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{1, "a", 2, "b", 1, 2, 1}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

type BadUpdateTableNameUser struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func (BadUpdateTableNameUser) TableName() string { return "bad name" }

func TestBatchUpdateModels_ExplicitTableWinsOverModelTableName(t *testing.T) {
	users := []BadUpdateTableNameUser{
		{ID: 1, Name: "a"},
		{ID: 2, Name: "b"},
	}
	q := mysqlQB().Update("users").Models(users)
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	if !strings.HasPrefix(sqlStr, "UPDATE `users`") {
		t.Fatalf("got: %s", sqlStr)
	}
}

func TestBatchUpdateModels_EmptyTableAndInvalidModelTableName(t *testing.T) {
	users := []BadUpdateTableNameUser{
		{ID: 1, Name: "a"},
	}
	q := mysqlQB().Update("").Models(users)
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestBatchUpdateModels_EmptyTableAnonymousStruct(t *testing.T) {
	users := []struct {
		ID int `db:"id"`
	}{{ID: 1}}
	q := mysqlQB().Update("").Models(users)
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}
