package builder_test

import (
	"reflect"
	"testing"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/dialect"
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

func TestUpdateModelAlias(t *testing.T) {
	d, _ := dialect.Get("mysql")
	
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	u := User{ID: 1, Name: "bob"}

	q := builder.Update(nil, d, "").
		Model(&u).
		Where("id = ?", 1)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	// Note: sets order is not guaranteed by map iteration in SetStruct if we used map,
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
	d, _ := dialect.Get("postgres")
	
	q := builder.Update(nil, d, "users").
		Set("status", "active").
		WhereLike("name", "A%").
		WhereSubquery("age", ">", builder.Select(nil, d, "avg(age)").From("users"))

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `UPDATE "users" SET "status" = $1 WHERE ("name" LIKE $2) AND (age > (SELECT avg(age) FROM "users"))`
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}
	
	wantArgs := []any{"active", "A%"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\nwant: %#v\ngot : %#v", wantArgs, args)
	}
}

func TestDeleteHelpers(t *testing.T) {
	d, _ := dialect.Get("postgres")
	
	q := builder.DeleteFrom(nil, d, "users").
		WhereLike("email", "%@spam.com").
		WhereInSubquery("id", builder.Select(nil, d, "user_id").From("blacklisted"))

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `DELETE FROM "users" WHERE ("email" LIKE $1) AND (id IN (SELECT "user_id" FROM "blacklisted"))`
	if sqlStr != wantSQL {
		t.Fatalf("sql mismatch:\nwant: %s\ngot : %s", wantSQL, sqlStr)
	}
	
	wantArgs := []any{"%@spam.com"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\nwant: %#v\ngot : %#v", wantArgs, args)
	}
}
