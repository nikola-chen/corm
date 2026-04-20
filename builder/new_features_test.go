package builder_test

import (
	"reflect"
	"testing"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

func TestNewFeatures_Select(t *testing.T) {
	bd := builder.Postgres()

	// Test Intersect, Except, ForShare
	q := bd.Select("id").From("users").WhereEq("status", 1).
		Intersect(bd.Select("id").From("admins")).
		Except(bd.Select("id").From("banned")).
		ForShare()

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT "id" FROM "users" WHERE ("status" = $1) INTERSECT (SELECT "id" FROM "admins") EXCEPT (SELECT "id" FROM "banned") FOR SHARE`
	if sqlStr != wantSQL {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr, wantSQL)
	}
	if !reflect.DeepEqual(args, []any{1}) {
		t.Errorf("args mismatch: got %v", args)
	}
}

func TestNewFeatures_WhereExt(t *testing.T) {
	bd := builder.Postgres()

	// Test WhereBetween, WhereNotIn, WhereNotLike, WhereExists, WhereNotExists
	q := bd.Select("id").From("users").
		WhereBetween("age", 18, 30).
		WhereNotIn("status", 4, 5).
		WhereNotLike("name", "%admin%").
		WhereExists(bd.Select().SelectExpr(clause.Raw("1")).From("profiles").Where(`"profiles"."user_id" = "users"."id"`)).
		WhereNotExists(bd.Select().SelectExpr(clause.Raw("1")).From("banned").Where(`"banned"."user_id" = "users"."id"`))

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `SELECT "id" FROM "users" WHERE ("age" BETWEEN $1 AND $2) AND ("status" NOT IN ($3, $4)) AND ("name" NOT LIKE $5) AND (EXISTS (SELECT 1 FROM "profiles" WHERE ("profiles"."user_id" = "users"."id"))) AND (NOT EXISTS (SELECT 1 FROM "banned" WHERE ("banned"."user_id" = "users"."id")))`
	if sqlStr != wantSQL {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr, wantSQL)
	}
	wantArgs := []any{18, 30, 4, 5, "%admin%"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("args mismatch: got %v", args)
	}
}

func TestNewFeatures_UpdateExt(t *testing.T) {
	bd := builder.Postgres()

	q := bd.Update("users").
		FromAs("departments", "d").
		Set("department_name", clause.Raw(`"d"."name"`)).
		Where(`"users"."department_id" = "d"."id"`).
		WhereBetween("age", 18, 60).
		Returning("id", "department_name")

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `UPDATE "users" SET "department_name" = "d"."name" FROM "departments" AS d WHERE ("users"."department_id" = "d"."id") AND ("age" BETWEEN $1 AND $2) RETURNING "id", "department_name"`
	if sqlStr != wantSQL {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr, wantSQL)
	}
	if !reflect.DeepEqual(args, []any{18, 60}) {
		t.Errorf("args mismatch: got %v", args)
	}

	bdMy := builder.MySQL()
	qMy := bdMy.Update("users").
		JoinAs("departments", "d", clause.Raw("users.department_id = d.id")).
		Set("department_name", clause.Raw("d.name")).
		WhereNotIn("status", 4, 5)

	sqlStrMy, argsMy, err := qMy.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQLMy := "UPDATE `users` JOIN `departments` AS d ON users.department_id = d.id SET `department_name` = d.name WHERE (`status` NOT IN (?, ?))"
	if sqlStrMy != wantSQLMy {
		t.Errorf("\ngot : %s\nwant: %s", sqlStrMy, wantSQLMy)
	}
	if !reflect.DeepEqual(argsMy, []any{4, 5}) {
		t.Errorf("args mismatch: got %v", argsMy)
	}
}

func TestNewFeatures_DeleteExt(t *testing.T) {
	bd := builder.Postgres()

	q := bd.Delete("users").
		UsingAs("logins", "l").
		Where(`"users"."id" = "l"."user_id"`).
		WhereNotLike("users.name", "test%").
		Returning("id")

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := `DELETE FROM "users" USING "logins" AS l WHERE ("users"."id" = "l"."user_id") AND ("users"."name" NOT LIKE $1) RETURNING "id"`
	if sqlStr != wantSQL {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr, wantSQL)
	}
	if !reflect.DeepEqual(args, []any{"test%"}) {
		t.Errorf("args mismatch: got %v", args)
	}
}

func TestNewFeatures_Upsert(t *testing.T) {
	bd := builder.Postgres()

	// 1. OnConflict DoUpdate Postgres
	q1 := bd.Insert("users").Columns("id", "name", "email").
		Values(1, "Alice", "alice@example.com").
		OnConflict("id").
		DoUpdate(map[string]any{"name": clause.Raw("EXCLUDED.name"), "email": "alice@example.com"})

	sqlStr1, args1, err := q1.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL1 := `INSERT INTO "users" ("id", "name", "email") VALUES ($1, $2, $3) ON CONFLICT ("id") DO UPDATE SET "email" = $4, "name" = EXCLUDED.name`
	if sqlStr1 != wantSQL1 {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr1, wantSQL1)
	}
	if !reflect.DeepEqual(args1, []any{1, "Alice", "alice@example.com", "alice@example.com"}) {
		t.Errorf("args mismatch: got %v", args1)
	}

	// 2. OnConflict DoNothing Postgres
	q2 := bd.Insert("users").Columns("id").Values(1).
		OnConflict("id", "email").
		DoNothing()

	sqlStr2, args2, err := q2.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL2 := `INSERT INTO "users" ("id") VALUES ($1) ON CONFLICT ("id", "email") DO NOTHING`
	if sqlStr2 != wantSQL2 {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr2, wantSQL2)
	}
	if !reflect.DeepEqual(args2, []any{1}) {
		t.Errorf("args mismatch: got %v", args2)
	}

	// 3. MySQL ON DUPLICATE KEY UPDATE
	bdMy := builder.MySQL()
	q3 := bdMy.Insert("users").Columns("id", "score").
		Values(1, 10).
		OnConflict("id"). // MySQL ignores column
		DoUpdate(map[string]any{"score": clause.Raw("score + VALUES(score)")})

	sqlStr3, args3, err := q3.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL3 := "INSERT INTO `users` (`id`, `score`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `score` = score + VALUES(score)"
	if sqlStr3 != wantSQL3 {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr3, wantSQL3)
	}
	if !reflect.DeepEqual(args3, []any{1, 10}) {
		t.Errorf("args mismatch: got %v", args3)
	}
}

func TestNewFeatures_CountExists(t *testing.T) {
	exec := &mockExecutor{}
	bd := builder.NewAPI(dialect.MustGet("postgres"), exec)

	q := bd.Select().From("users").WhereEq("status", 1)
	sqlStr, _, _ := q.SQL()
	if sqlStr != `SELECT * FROM "users" WHERE ("status" = $1)` {
		t.Errorf("expected default select")
	}
}

func TestInsertIgnore(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Insert("users").Columns("id", "name").Values(1, "Alice").InsertIgnore()
	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := "INSERT IGNORE INTO `users` (`id`, `name`) VALUES (?, ?)"
	if sqlStr != wantSQL {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr, wantSQL)
	}
	if !reflect.DeepEqual(args, []any{1, "Alice"}) {
		t.Errorf("args mismatch: got %v", args)
	}

	bdPg := builder.Postgres()
	qPg := bdPg.Insert("users").Columns("id").Values(1).InsertIgnore()
	_, _, errPg := qPg.SQL()
	if errPg == nil {
		t.Error("expected error for InsertIgnore on Postgres")
	}
}

func TestSetExpr(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Update("users").
		SetExpr("updated_at", clause.Raw("NOW()")).
		Set("name", "Alice").
		WhereEq("id", 1)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	wantSQL := "UPDATE `users` SET `updated_at` = NOW(), `name` = ? WHERE (`id` = ?)"
	if sqlStr != wantSQL {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr, wantSQL)
	}
	if !reflect.DeepEqual(args, []any{"Alice", 1}) {
		t.Errorf("args mismatch: got %v", args)
	}
}

func TestCountExpr(t *testing.T) {
	bd := builder.Postgres()

	q := bd.Select("id").From("users").WhereEq("status", 1)
	sqlStr, args, err := q.CountExprSQL(clause.Raw("COUNT(DISTINCT \"email\")"))
	if err != nil {
		t.Fatalf("CountExprSQL() error: %v", err)
	}

	wantSQL := `SELECT COUNT(DISTINCT "email") FROM "users" WHERE ("status" = $1)`
	if sqlStr != wantSQL {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr, wantSQL)
	}
	if !reflect.DeepEqual(args, []any{1}) {
		t.Errorf("args mismatch: got %v", args)
	}
}

func TestCountExprWithGroupBy(t *testing.T) {
	bd := builder.Postgres()

	q := bd.Select("status").From("users").WhereExpr(clause.Gt("id", 0)).GroupBy("status")
	sqlStr, args, err := q.CountExprSQL(clause.Raw("COUNT(*)"))
	if err != nil {
		t.Fatalf("CountExprSQL() error: %v", err)
	}

	wantSQL := `SELECT COUNT(*) FROM (SELECT "status" FROM "users" WHERE (id > $1) GROUP BY "status") AS sub`
	if sqlStr != wantSQL {
		t.Errorf("\ngot : %s\nwant: %s", sqlStr, wantSQL)
	}
	if !reflect.DeepEqual(args, []any{0}) {
		t.Errorf("args mismatch: got %v", args)
	}
}
