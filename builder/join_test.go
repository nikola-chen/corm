package builder_test

import (
	"reflect"
	"testing"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

func TestJoinOn(t *testing.T) {
	d, _ := dialect.Get("postgres")
	q := builder.Select(nil, d, "u.name", "o.amount").
		From("users u").
		LeftJoinOn("orders o", clause.Raw("u.id = o.user_id")).
		InnerJoinOn("status s", clause.And(
			clause.Raw("o.status_id = s.id"),
			clause.Eq("s.active", true), // Args: true
		))

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT "u"."name", "o"."amount" FROM users u LEFT JOIN orders o ON u.id = o.user_id INNER JOIN status s ON (o.status_id = s.id) AND (s.active = $1)`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}

	wantArgs := []any{true}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestJoinRawArgs(t *testing.T) {
	d, _ := dialect.Get("mysql")
	q := builder.Select(nil, d, "*").
		From("t1").
		JoinExpr("LEFT JOIN", "t2", clause.Raw("t1.id = t2.id AND t2.val > ?", 100))

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := "SELECT * FROM `t1` LEFT JOIN `t2` ON t1.id = t2.id AND t2.val > ?"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{100}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestFromSelectAliasValidation(t *testing.T) {
	d, _ := dialect.Get("postgres")
	sub := builder.Select(nil, d, "id").From("users")
	q := builder.Select(nil, d, "id").FromSelect(sub, "u;drop")
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestJoinExprInvalidType(t *testing.T) {
	d, _ := dialect.Get("mysql")
	q := builder.Select(nil, d, "*").From("t1").JoinExpr("BAD", "t2", clause.Raw("1=1"))
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestJoinRawExists(t *testing.T) {
	d, _ := dialect.Get("mysql")
	q := builder.Select(nil, d, "*").From("t1").JoinRaw("LEFT JOIN t2 ON t1.id = t2.id")
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
	d, _ := dialect.Get("postgres")
	q := builder.Select(nil, d, "id").FromAs("users", "u")
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL := `SELECT "id" FROM "users" AS u`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
}

func TestWhereInIdentRejectsBadColumn(t *testing.T) {
	d, _ := dialect.Get("mysql")
	q := builder.Select(nil, d, "*").From("users").WhereInIdent("id;drop", []int{1, 2})
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestWhereInIdentGeneratesSQL(t *testing.T) {
	d, _ := dialect.Get("postgres")
	q := builder.Select(nil, d, "*").From("users").WhereInIdent("id", []int{1, 2, 3})
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
