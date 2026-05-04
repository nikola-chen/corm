package builder_test

import (
	"reflect"
	"strings"
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

func TestUpdateWhereExt(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Update("users").
		Set("status", 1).
		WhereLike("name", "A%").
		WhereNotIn("role", "admin", "super").
		WhereBetween("age", 18, 60)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	if !strings.Contains(sqlStr, "`name` LIKE ?") {
		t.Errorf("missing LIKE clause: %s", sqlStr)
	}
	if !strings.Contains(sqlStr, "`role` NOT IN (?, ?)") {
		t.Errorf("missing NOT IN clause: %s", sqlStr)
	}
	if !strings.Contains(sqlStr, "`age` BETWEEN ? AND ?") {
		t.Errorf("missing BETWEEN clause: %s", sqlStr)
	}
	if len(args) != 6 {
		t.Errorf("expected 6 args, got %d: %v", len(args), args)
	}
}

func TestUpdateWhereSubquery(t *testing.T) {
	bd := builder.MySQL()
	sub := bd.Select("id").From("banned_users")

	q := bd.Update("users").
		Set("status", 0).
		WhereSubquery("id", "IN", sub)

	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	if !strings.Contains(sqlStr, "WHERE (`id` IN (SELECT") {
		t.Errorf("missing subquery: %s", sqlStr)
	}
}

func TestUpdateIncrementDecrement(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Update("users").
		Increment("views", 1).
		Decrement("credits", 10).
		WhereEq("id", 1)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}

	if !strings.Contains(sqlStr, "`views` = `views` + ?") {
		t.Errorf("missing increment: %s", sqlStr)
	}
	if !strings.Contains(sqlStr, "`credits` = `credits` - ?") {
		t.Errorf("missing decrement: %s", sqlStr)
	}
	if !reflect.DeepEqual(args, []any{1, 10, 1}) {
		t.Errorf("args mismatch: got %v", args)
	}
}

func TestUpdateIncrementOnBatch(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Update("users").
		Increment("views", 1).
		Models([]User{{ID: 1}, {ID: 2}})
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for Increment on batch update")
	}
}

func TestUpdateDecrementOnBatch(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Update("users").
		Decrement("views", 1).
		Models([]User{{ID: 1}, {ID: 2}})
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for Decrement on batch update")
	}
}

func TestUpdateSetOnBatch(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Update("users").
		Set("name", "test").
		Models([]User{{ID: 1}, {ID: 2}})
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for Set on batch update")
	}
}

func TestUpdateSetExprOnBatch(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Update("users").
		SetExpr("name", clause.Raw("NOW()")).
		Models([]User{{ID: 1}, {ID: 2}})
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for SetExpr on batch update")
	}
}

func TestUpdateMapOnBatch(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Update("users").
		Map(map[string]any{"name": "test"}).
		Models([]User{{ID: 1}, {ID: 2}})
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for Map on batch update")
	}
}

func TestUpdateLimitMySQL(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Update("users").
		Set("status", 0).
		Where("id > ?", 0).
		Limit(100)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	if !strings.Contains(sqlStr, "LIMIT ?") {
		t.Errorf("missing LIMIT: %s", sqlStr)
	}
	if !reflect.DeepEqual(args, []any{0, 0, 100}) {
		t.Errorf("args mismatch: got %v", args)
	}
}

func TestUpdateLimitPostgres(t *testing.T) {
	bd := builder.Postgres()
	q := bd.Update("users").
		Set("status", 0).
		Where("id > ?", 0).
		Limit(100)

	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for LIMIT on Postgres update")
	}
}

func TestDeleteLimitMySQL(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Delete("users").
		Where("id > ?", 0).
		Limit(100)

	sqlStr, args, err := q.SQL()
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	if !strings.Contains(sqlStr, "LIMIT ?") {
		t.Errorf("missing LIMIT: %s", sqlStr)
	}
	if !reflect.DeepEqual(args, []any{0, 100}) {
		t.Errorf("args mismatch: got %v", args)
	}
}

func TestDeleteLimitPostgres(t *testing.T) {
	bd := builder.Postgres()
	q := bd.Delete("users").
		Where("id > ?", 0).
		Limit(100)

	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for LIMIT on Postgres delete")
	}
}

func TestDeleteInvalidLimit(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Delete("users").Where("id > ?", 0).Limit(-1)
	_, _, err := q.SQL()
	if err != nil {
		t.Errorf("expected no error for negative LIMIT (treated as no limit), got: %v", err)
	}
}

func TestWhereInvalidColumn(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Select("id").From("users").WhereEq("invalid column", 1)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for invalid column in WhereEq")
	}
}

func TestWhereSubqueryNil(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").WhereSubquery("id", "IN", nil)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for nil subquery")
	}
}

func TestWhereSubqueryInvalidOp(t *testing.T) {
	bd := builder.MySQL()
	sub := bd.Select("id").From("banned")
	q := bd.Select("id").From("users").WhereSubquery("id", "INVALID", sub)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for invalid operator")
	}
}

func TestWhereExistsNil(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").WhereExists(nil)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for nil subquery in WhereExists")
	}
}

func TestWhereNotExistsNil(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").WhereNotExists(nil)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for nil subquery in WhereNotExists")
	}
}

func TestWhereInInvalidColumn(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").WhereIn("invalid col", 1, 2)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for invalid column in WhereIn")
	}
}

func TestWhereLikeInvalidColumn(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").WhereLike("invalid col", "test%")
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for invalid column in WhereLike")
	}
}

func TestWhereNotInInvalidColumn(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").WhereNotIn("invalid col", 1, 2)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for invalid column in WhereNotIn")
	}
}

func TestWhereBetweenInvalidColumn(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").WhereBetween("invalid col", 1, 10)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for invalid column in WhereBetween")
	}
}

func TestWhereNotLikeInvalidColumn(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").WhereNotLike("invalid col", "test%")
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for invalid column in WhereNotLike")
	}
}

func TestWhereMapInvalidColumn(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").WhereMap(map[string]any{"invalid col": 1})
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for invalid column in WhereMap")
	}
}

func TestSelectWhereEmpty(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").Where("")
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(sqlStr, "WHERE") {
		t.Errorf("empty where should be omitted: %s", sqlStr)
	}
}

func TestSelectWhereExprEmpty(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Select("id").From("users").WhereExpr(clause.Raw(""))
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(sqlStr, "WHERE") {
		t.Errorf("empty where expr should be omitted: %s", sqlStr)
	}
}

func TestSelectForShare(t *testing.T) {
	bd := builder.Postgres()
	q := bd.Select("id").From("users").WhereEq("id", 1).ForShare()
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sqlStr, "FOR SHARE") {
		t.Errorf("missing FOR SHARE: %s", sqlStr)
	}
}

func TestSelectIntersect(t *testing.T) {
	bd := builder.Postgres()
	q1 := bd.Select("id").From("users_2023")
	q2 := bd.Select("id").From("users_2024")
	q := q1.Intersect(q2)
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sqlStr, "INTERSECT") {
		t.Errorf("missing INTERSECT: %s", sqlStr)
	}
}

func TestSelectExcept(t *testing.T) {
	bd := builder.Postgres()
	q1 := bd.Select("id").From("users")
	q2 := bd.Select("id").From("banned")
	q := q1.Except(q2)
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sqlStr, "EXCEPT") {
		t.Errorf("missing EXCEPT: %s", sqlStr)
	}
}

func TestUpdateInvalidLimit(t *testing.T) {
	bd := builder.MySQL()
	q := bd.Update("users").Set("status", 0).Where("id > ?", 0).Limit(-1)
	_, _, err := q.SQL()
	if err != nil {
		t.Errorf("expected no error for negative LIMIT (treated as no limit), got: %v", err)
	}
}

func TestUpdateWhereMethods(t *testing.T) {
	bd := builder.Postgres()

	t.Run("WhereEq", func(t *testing.T) {
		q := bd.Update("users").Set("status", 1).WhereEq("id", 42)
		sqlStr, args, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, `"id" = $2`) {
			t.Errorf("missing WhereEq clause: %s", sqlStr)
		}
		if !reflect.DeepEqual(args, []any{1, 42}) {
			t.Errorf("args mismatch: got %v", args)
		}
	})

	t.Run("WhereIn", func(t *testing.T) {
		q := bd.Update("users").Set("status", 0).WhereIn("id", 1, 2, 3)
		sqlStr, args, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, `"id" IN ($2, $3, $4)`) {
			t.Errorf("missing WhereIn clause: %s", sqlStr)
		}
		if !reflect.DeepEqual(args, []any{0, 1, 2, 3}) {
			t.Errorf("args mismatch: got %v", args)
		}
	})

	t.Run("WhereLike", func(t *testing.T) {
		q := bd.Update("users").Set("status", 1).WhereLike("name", "%test%")
		sqlStr, args, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, `"name" LIKE $2`) {
			t.Errorf("missing WhereLike clause: %s", sqlStr)
		}
		if !reflect.DeepEqual(args, []any{1, "%test%"}) {
			t.Errorf("args mismatch: got %v", args)
		}
	})

	t.Run("WhereMap", func(t *testing.T) {
		q := bd.Update("users").Set("status", 1).WhereMap(map[string]any{"id": 10, "age": 20})
		sqlStr, args, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, "WHERE") {
			t.Errorf("missing WHERE clause: %s", sqlStr)
		}
		if len(args) != 3 {
			t.Errorf("expected 3 args, got %d: %v", len(args), args)
		}
	})

	t.Run("WhereSubquery", func(t *testing.T) {
		sub := bd.Select("user_id").From("banned")
		q := bd.Update("users").Set("status", 0).WhereSubquery("id", "IN", sub)
		sqlStr, _, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, `"id" IN (SELECT`) {
			t.Errorf("missing WhereSubquery clause: %s", sqlStr)
		}
	})

	t.Run("WhereInSubquery", func(t *testing.T) {
		sub := bd.Select("user_id").From("banned")
		q := bd.Update("users").Set("status", 0).WhereInSubquery("id", sub)
		sqlStr, _, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, `"id" IN (SELECT`) {
			t.Errorf("missing WhereInSubquery clause: %s", sqlStr)
		}
	})

	t.Run("WhereExpr", func(t *testing.T) {
		q := bd.Update("users").Set("status", 1).WhereExpr(clause.Raw(`"age" > ?`, 18))
		sqlStr, args, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, `"age" > $2`) {
			t.Errorf("missing WhereExpr clause: %s", sqlStr)
		}
		if !reflect.DeepEqual(args, []any{1, 18}) {
			t.Errorf("args mismatch: got %v", args)
		}
	})

	t.Run("WhereNotIn", func(t *testing.T) {
		q := bd.Update("users").Set("status", 1).WhereNotIn("role", 4, 5)
		sqlStr, args, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, `"role" NOT IN ($2, $3)`) {
			t.Errorf("missing WhereNotIn clause: %s", sqlStr)
		}
		if !reflect.DeepEqual(args, []any{1, 4, 5}) {
			t.Errorf("args mismatch: got %v", args)
		}
	})

	t.Run("WhereBetween", func(t *testing.T) {
		q := bd.Update("users").Set("status", 1).WhereBetween("age", 18, 60)
		sqlStr, args, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, `"age" BETWEEN $2 AND $3`) {
			t.Errorf("missing WhereBetween clause: %s", sqlStr)
		}
		if !reflect.DeepEqual(args, []any{1, 18, 60}) {
			t.Errorf("args mismatch: got %v", args)
		}
	})

	t.Run("WhereNotLike", func(t *testing.T) {
		q := bd.Update("users").Set("status", 1).WhereNotLike("name", "%admin%")
		sqlStr, args, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, `"name" NOT LIKE $2`) {
			t.Errorf("missing WhereNotLike clause: %s", sqlStr)
		}
		if !reflect.DeepEqual(args, []any{1, "%admin%"}) {
			t.Errorf("args mismatch: got %v", args)
		}
	})

	t.Run("WhereExists", func(t *testing.T) {
		sub := bd.Select().SelectExpr(clause.Raw("1")).From("active").Where(`"active"."user_id" = "users"."id"`)
		q := bd.Update("users").Set("status", 1).WhereExists(sub)
		sqlStr, _, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, "EXISTS (SELECT") {
			t.Errorf("missing WhereExists clause: %s", sqlStr)
		}
	})

	t.Run("WhereNotExists", func(t *testing.T) {
		sub := bd.Select().SelectExpr(clause.Raw("1")).From("banned").Where(`"banned"."user_id" = "users"."id"`)
		q := bd.Update("users").Set("status", 1).WhereNotExists(sub)
		sqlStr, _, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, "NOT EXISTS (SELECT") {
			t.Errorf("missing WhereNotExists clause: %s", sqlStr)
		}
	})

	t.Run("MapsLowerKeys", func(t *testing.T) {
		rows := []map[string]any{
			{"id": 1, "name": "alice"},
			{"id": 2, "name": "bob"},
		}
		q := bd.Update("users").MapsLowerKeys(rows)
		sqlStr, _, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, "UPDATE") {
			t.Errorf("missing UPDATE: %s", sqlStr)
		}
	})
}

func TestUpdateWhereMethodsMySQL(t *testing.T) {
	bd := builder.MySQL()

	t.Run("WhereIn", func(t *testing.T) {
		q := bd.Update("users").Set("status", 0).WhereIn("id", 1, 2)
		sqlStr, args, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, "`id` IN (?, ?)") {
			t.Errorf("missing WhereIn clause: %s", sqlStr)
		}
		if !reflect.DeepEqual(args, []any{0, 1, 2}) {
			t.Errorf("args mismatch: got %v", args)
		}
	})

	t.Run("WhereMap", func(t *testing.T) {
		q := bd.Update("users").Set("status", 1).WhereMap(map[string]any{"id": 5})
		sqlStr, args, err := q.SQL()
		if err != nil {
			t.Fatalf("SQL() error: %v", err)
		}
		if !strings.Contains(sqlStr, "WHERE") {
			t.Errorf("missing WHERE: %s", sqlStr)
		}
		if len(args) != 2 {
			t.Errorf("expected 2 args, got %d: %v", len(args), args)
		}
	})
}
