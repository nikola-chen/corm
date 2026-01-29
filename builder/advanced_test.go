package builder_test

import (
	"reflect"
	"testing"

	"github.com/nikola-chen/corm/clause"
)

func TestDistinct(t *testing.T) {
	q := pgQB().Select("name").
		From("users").
		Distinct()

	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT DISTINCT "name" FROM "users"`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
}

func TestLimit(t *testing.T) {
	q := mysqlQB().Select("name").
		From("users").
		Limit(5)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := "SELECT `name` FROM `users` LIMIT ?"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{5}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestLogicalOps(t *testing.T) {
	q := pgQB().Select("*").
		From("users").
		WhereExpr(clause.Not(clause.Raw("age < ?", 18))).
		WhereExpr(clause.IsNull("deleted_at")).
		WhereExpr(clause.IsNotNull("email"))

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT * FROM "users" WHERE (NOT (age < $1)) AND (deleted_at IS NULL) AND (email IS NOT NULL)`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{18}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestAlias(t *testing.T) {
	q := pgQB().Select().
		SelectExpr(clause.Raw(clause.Alias("count(*)", "cnt"))).
		From("users")

	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT count(*) AS cnt FROM "users"`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
}

func TestSelectColumnsStrictRejectsExpressions(t *testing.T) {
	q := pgQB().Select("count(*)").From("users")
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestJoins(t *testing.T) {
	q := pgQB().Select("u.name", "p.title").
		FromAs("users", "u").
		LeftJoinAs("posts", "p", clause.Raw("u.id = p.user_id")).
		InnerJoinAs("comments", "c", clause.Raw("p.id = c.post_id"))

	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT "u"."name", "p"."title" FROM "users" AS u LEFT JOIN "posts" AS p ON u.id = p.user_id INNER JOIN "comments" AS c ON p.id = c.post_id`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
}

func TestUnion(t *testing.T) {
	qb := mysqlQB()
	u1 := qb.Select("id", "name").From("users").Where("id < ?", 10)
	u2 := qb.Select("id", "name").From("users").Where("id > ?", 20)

	q := u1.Union(u2)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := "SELECT `id`, `name` FROM `users` WHERE (id < ?) UNION (SELECT `id`, `name` FROM `users` WHERE (id > ?))"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}

	wantArgs := []any{10, 20}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestAggregates(t *testing.T) {
	q := pgQB().Select().
		SelectExpr(
			clause.Count("id"),
			clause.Max("age"),
		).
		From("users")

	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT COUNT(id), MAX(age) FROM "users"`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
}

func TestInsertSelect(t *testing.T) {
	qb := pgQB()
	sub := qb.Select("id", "name").From("old_users").Where("age > ?", 30)
	q := qb.Insert("users").
		Columns("id", "name").
		FromSelect(sub)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `INSERT INTO "users" ("id", "name") SELECT "id", "name" FROM "old_users" WHERE (age > $1)`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{30}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestSubqueryFrom(t *testing.T) {
	qb := pgQB()
	sub := qb.Select("id", "name").From("users").Where("age > ?", 18)
	q := qb.Select("id").
		FromSelect(sub, "u").
		Where("u.name = ?", "alice")

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT "id" FROM (SELECT "id", "name" FROM "users" WHERE (age > $1)) AS u WHERE (u.name = $2)`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{18, "alice"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestSubqueryWhere(t *testing.T) {
	qb := pgQB()
	sub := qb.Select("id").From("banned_users")
	q := qb.Select("*").
		From("users").
		WhereInSubquery("id", sub)

	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT * FROM "users" WHERE ("id" IN (SELECT "id" FROM "banned_users"))`
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
}
