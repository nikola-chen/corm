package builder_test

import (
	"reflect"
	"testing"

	"github.com/nikola-chen/corm/clause"
)

func TestJoinOn(t *testing.T) {
	q := pgQB().Select("u.name", "o.amount").
		FromAs("users", "u").
		LeftJoinAs("orders", "o", clause.Raw("u.id = o.user_id")).
		InnerJoinAs("status", "s", clause.And(
			clause.Raw("o.status_id = s.id"),
			clause.Eq("s.active", true), // Args: true
		))

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT "u"."name", "o"."amount" FROM "users" AS u LEFT JOIN "orders" AS o ON u.id = o.user_id INNER JOIN "status" AS s ON (o.status_id = s.id) AND (s.active = $1)`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}

	wantArgs := []any{true}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestJoinRawArgs(t *testing.T) {
	q := mysqlQB().Select("*").
		From("t1").
		JoinRaw("LEFT JOIN t2 ON t1.id = t2.id AND t2.val > ?", 100)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := "SELECT * FROM `t1` LEFT JOIN t2 ON t1.id = t2.id AND t2.val > ?"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{100}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestFromSelectAliasValidation(t *testing.T) {
	qb := pgQB()
	sub := qb.Select("id").From("users")
	q := qb.Select("id").FromSelect(sub, "u;drop")
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestJoinEmptyCondition(t *testing.T) {
	q := mysqlQB().Select("*").From("t1").LeftJoin("t2", clause.Raw(""))
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestJoinExists(t *testing.T) {
	q := mysqlQB().Select("*").From("t1").JoinRaw("LEFT JOIN t2 ON t1.id = t2.id")
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := "SELECT * FROM `t1` LEFT JOIN t2 ON t1.id = t2.id"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
}

func TestFromAs(t *testing.T) {
	q := pgQB().Select("id").FromAs("users", "u")
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := `SELECT "id" FROM "users" AS u`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
}

func TestWhereInRejectsBadColumn(t *testing.T) {
	q := mysqlQB().Select("*").From("users").WhereIn("id;drop", []int{1, 2})
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestWhereInGeneratesSQL(t *testing.T) {
	q := pgQB().Select("*").From("users").WhereIn("id", []int{1, 2, 3})
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := `SELECT * FROM "users" WHERE ("id" IN ($1, $2, $3))`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{1, 2, 3}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}
