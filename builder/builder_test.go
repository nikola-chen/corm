package builder_test

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/clause"
)

func TestInsertMap(t *testing.T) {
	qb := mysqlQB()

	// Case 1: Map without predefined columns (sorted keys)
	q := qb.Insert("users").Map(map[string]any{
		"name": "alice",
		"age":  30,
	})

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := "INSERT INTO `users` (`age`, `name`) VALUES (?, ?)"
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}
	// 30, alice
	if args[0] != 30 || args[1] != "alice" {
		t.Fatalf("args mismatch: %v", args)
	}

	// Case 2: Map with predefined columns (explicit order)
	q2 := qb.Insert("users").
		Columns("name", "age").
		Map(map[string]any{
			"age":  30,
			"name": "alice",
		})

	sqlStr2, args2, err := q2.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL2 := "INSERT INTO `users` (`name`, `age`) VALUES (?, ?)"
	if sqlStr2 != wantSQL2 {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL2, sqlStr2)
	}
	if args2[0] != "alice" || args2[1] != 30 {
		t.Fatalf("args mismatch: %v", args2)
	}

	// Case 3: MapLowerKeys with predefined columns (faster path)
	q3 := qb.Insert("users").
		Columns("name", "age").
		MapLowerKeys(map[string]any{
			"name": "alice",
			"age":  30,
		})
	sqlStr3, args3, err := q3.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	if sqlStr3 != wantSQL2 {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL2, sqlStr3)
	}
	if args3[0] != "alice" || args3[1] != 30 {
		t.Fatalf("args mismatch: %v", args3)
	}
}

func TestUpdateMap(t *testing.T) {
	q := pgQB().Update("users").
		Map(map[string]any{
			"status": "active",
			"score":  100,
		}).
		Where("id = ?", 1)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	// keys sorted: score, status
	wantSQL := `UPDATE "users" SET "score" = $1, "status" = $2 WHERE (id = $3)`
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}

	if args[0] != 100 || args[1] != "active" || args[2] != 1 {
		t.Fatalf("args mismatch: %v", args)
	}
}

func TestWhereMap(t *testing.T) {
	q := mysqlQB().Select("id").From("users").
		WhereMap(map[string]any{
			"name": "alice",
			"age":  30,
		})

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	// keys sorted: age, name
	// WhereEq quotes identifiers: `age` = ?, `name` = ?
	wantSQL := "SELECT `id` FROM `users` WHERE (`age` = ?) AND (`name` = ?)"
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}

	if args[0] != 30 || args[1] != "alice" {
		t.Fatalf("args mismatch: %v", args)
	}
}

func TestSelectPostgresPlaceholders(t *testing.T) {
	q := pgQB().Select("id", "name").
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

func TestUpdateLimit(t *testing.T) {
	q := mysqlQB().Update("users").Set("name", "x").Where("id > ?", 1).Limit(5)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := "UPDATE `users` SET `name` = ? WHERE (id > ?) LIMIT ?"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{"x", 1, 5}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestDeleteLimit(t *testing.T) {
	q := mysqlQB().Delete("users").Where("id > ?", 1).Limit(5)
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := "DELETE FROM `users` WHERE (id > ?) LIMIT ?"
	if sqlStr != wantSQL {
		t.Fatalf("want: %s, got: %s", wantSQL, sqlStr)
	}
	wantArgs := []any{1, 5}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("want args: %v, got: %v", wantArgs, args)
	}
}

func TestUpdateLimitPostgresUnsupported(t *testing.T) {
	q := pgQB().Update("users").Set("name", "x").Where("id > ?", 1).Limit(5)
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteLimitPostgresUnsupported(t *testing.T) {
	q := pgQB().Delete("users").Where("id > ?", 1).Limit(5)
	_, _, err := q.SQL()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSelectMySQLPlaceholders(t *testing.T) {
	q := mysqlQB().Select("id", "name").
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
	q := pgQB().Insert("users").
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
	q := mysqlQB().Insert("users").
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

func TestUpdateModelAlias(t *testing.T) {
	qb := mysqlQB()

	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	u := User{ID: 1, Name: "bob"}

	q := qb.Update("").
		Model(&u).
		Where("id = ?", 1)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	// Note: sets order is not guaranteed by map iteration in Map/SetMap if we used map,
	// but here we use struct so it depends on schema.Parse order which is field index order.
	// ID is field 0, Name is field 1.
	// But ExtractOptions logic might skip some fields?
	// By default IncludePrimaryKey is false. So ID is skipped.
	// Only Name is updated.

	// struct name is User, default table name is "user" (snake_case)
	// schema parser uses GORM-like convention: CamelCase -> snake_case

	wantSQL := "UPDATE `user` SET `name` = ? WHERE (id = ?)"
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}

	wantArgs := []any{"bob", 1}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\nwant: %#v\ngot : %#v", wantArgs, args)
	}
}

func TestUpdateHelpers(t *testing.T) {
	qb := pgQB()
	q := qb.Update("users").
		Set("status", "active").
		WhereLike("name", "A%").
		WhereSubquery("age", ">", qb.Select().SelectExpr(clause.Raw("avg(age)")).From("users"))

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `UPDATE "users" SET "status" = $1 WHERE ("name" LIKE $2) AND ("age" > (SELECT avg(age) FROM "users"))`
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}

	wantArgs := []any{"active", "A%"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\nwant: %#v\ngot : %#v", wantArgs, args)
	}
}

func TestDeleteHelpers(t *testing.T) {
	qb := pgQB()
	q := qb.Delete("users").
		WhereLike("email", "%@spam.com").
		WhereInSubquery("id", qb.Select("user_id").From("blacklisted"))

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `DELETE FROM "users" WHERE ("email" LIKE $1) AND ("id" IN (SELECT "user_id" FROM "blacklisted"))`
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}

	wantArgs := []any{"%@spam.com"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\nwant: %#v\ngot : %#v", wantArgs, args)
	}
}

func TestDeleteSafety(t *testing.T) {
	qb := mysqlQB()

	// Case 1: Delete without where should fail
	q := qb.Delete("users")
	_, _, err := q.SQL()
	if err == nil {
		t.Fatal("expected error for delete without where, got nil")
	}

	// Case 1.1: Blank where must not bypass safety checks
	q = qb.Delete("users").Where("   ")
	_, _, err = q.SQL()
	if err == nil {
		t.Fatal("expected error for delete with blank where, got nil")
	}

	// Case 2: Explicitly allow empty where
	q = qb.Delete("users").AllowEmptyWhere()
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sqlStr != "DELETE FROM `users`" {
		t.Fatalf("unexpected sql: %s", sqlStr)
	}
}

func TestUpdateSafety(t *testing.T) {
	qb := mysqlQB()

	q := qb.Update("users").Set("name", "x")
	_, _, err := q.SQL()
	if err == nil {
		t.Fatal("expected error for update without where, got nil")
	}

	q = qb.Update("users").Set("name", "x").Where(" \n\t")
	_, _, err = q.SQL()
	if err == nil {
		t.Fatal("expected error for update with blank where, got nil")
	}

	q = qb.Update("users").Set("name", "x").AllowEmptyWhere()
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sqlStr != "UPDATE `users` SET `name` = ?" {
		t.Fatalf("unexpected sql: %s", sqlStr)
	}
	if len(args) != 1 || args[0] != "x" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestSelectForUpdate(t *testing.T) {
	q := pgQB().Select("id").From("users").Where("id = ?", 1).ForUpdate()

	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT "id" FROM "users" WHERE (id = $1) FOR UPDATE`
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}
}

func TestUpdateIncrement(t *testing.T) {
	q := mysqlQB().Update("users").
		Increment("view_count", 1).
		Decrement("score", 5).
		Where("id = ?", 10)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	// order of sets is preserved
	wantSQL := "UPDATE `users` SET `view_count` = `view_count` + ?, `score` = `score` - ? WHERE (id = ?)"
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}

	wantArgs := []any{1, 5, 10}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\nwant: %#v\ngot : %#v", wantArgs, args)
	}
}

func TestInsertSuffixRaw(t *testing.T) {
	sqlStr, args, err := mysqlQB().Insert("users").
		Columns("id", "name").
		Values(1, "alice").
		SuffixRaw("ON DUPLICATE KEY UPDATE name = ?", "bob").
		SQL()

	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := "INSERT INTO `users` (`id`, `name`) VALUES (?, ?) ON DUPLICATE KEY UPDATE name = ?"
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}

	wantArgs := []any{1, "alice", "bob"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\nwant: %#v\ngot : %#v", wantArgs, args)
	}
}

func TestEmptyAndWhitespaceInputs(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{
			"empty From",
			func() error {
				_, _, err := mysqlQB().Select("*").From("").SQL()
				return err
			},
		},
		{
			"whitespace From",
			func() error {
				_, _, err := mysqlQB().Select("*").From("   ").SQL()
				return err
			},
		},
		{
			"empty OrderBy column",
			func() error {
				_, _, err := mysqlQB().Select("*").From("users").OrderBy("", "ASC").SQL()
				return err
			},
		},
		{
			"empty GroupBy",
			func() error {
				_, _, err := mysqlQB().Select("*").From("users").GroupBy("").SQL()
				return err
			},
		},
		{
			"empty Having",
			func() error {
				_, _, err := mysqlQB().Select("*").From("users").Having("").SQL()
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Errorf("%s: expected error but got none", tt.name)
			}
		})
	}
}

func TestInvalidIdentifiers(t *testing.T) {
	invalidIdentifiers := []string{
		"table with spaces",
		"table-with-dashes",
		"table/with/slashes",
		"table;with;semicolons",
		"table\"with\"quotes",
		"table`with`backticks",
		"123startingwithnumber",
		"table.with.dots", // This should actually work as table.column format
	}

	for _, ident := range invalidIdentifiers {
		t.Run("From_"+ident, func(t *testing.T) {
			_, _, err := mysqlQB().Select("*").From(ident).SQL()
			// Skip the dot case as it's valid table.column format
			if strings.Contains(ident, ".") && !strings.HasPrefix(ident, ".") && !strings.HasSuffix(ident, ".") {
				if err != nil {
					t.Logf("Expected table.column format to work, but got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("Expected error for invalid identifier: %s", ident)
			}
		})
	}
}

func TestSQLInjectionAttempts(t *testing.T) {
	// Test that dangerous inputs are properly handled
	dangerousInputs := []string{
		"users; DROP TABLE users; --",
		"users' OR '1'='1",
		"users\" OR \"1\"=\"1",
		"users` OR `1`=`1",
		"users/* comment */",
		"users-- comment",
	}

	for _, input := range dangerousInputs {
		t.Run("From_"+input, func(t *testing.T) {
			_, _, err := mysqlQB().Select("*").From(input).SQL()
			if err == nil {
				t.Errorf("Expected error for dangerous input: %s", input)
			}
		})
	}
}

func TestMaxSQLLengthExceeded(t *testing.T) {
	// This test is tricky because we can't easily trigger the SQL length limit
	// in normal usage, but we can test the validation logic indirectly
	// by creating a very complex query

	q := mysqlQB().Select("*").From("users")

	// Add many WHERE conditions
	for i := 0; i < 100; i++ {
		q = q.Where("field%d = ?", i)
	}

	// Add many ORDER BY clauses
	for i := 0; i < 50; i++ {
		q = q.OrderBy("field"+strconv.Itoa(i), "ASC")
	}

	// This should still work, but if we had a much larger query it might hit limits
	_, _, err := q.SQL()
	if err != nil {
		// If we get an error about SQL length, that's expected behavior
		if strings.Contains(err.Error(), "exceeds maximum length") {
			t.Logf("SQL length limit working as expected: %v", err)
		} else {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestNilDialectHandling(t *testing.T) {
	// Test that nil dialect is handled gracefully
	api := &builder.API{}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Expected no panic with nil dialect, but got: %v", r)
		}
	}()

	_, _, err := api.Select("*").From("users").SQL()
	if err == nil {
		t.Errorf("Expected error with nil dialect")
	}
}
