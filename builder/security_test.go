package builder_test

import (
	"testing"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/clause"
)

func TestSQLInjection_TableName(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Select().From("users; DROP TABLE users--")
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious table name")
	}
}

func TestSQLInjection_ColumnName(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Select("id; DROP TABLE users--").From("users")
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious column name")
	}
}

func TestSQLInjection_WhereColumn(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Select().From("users").WhereEq("id = 1; DROP TABLE users--", 1)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious where column")
	}
}

func TestSQLInjection_InsertColumns(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Insert("users").Columns("id", "name; DROP TABLE users--").Values(1, "test")
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious insert column")
	}
}

func TestSQLInjection_UpdateSet(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Update("users").Set("name = 'hacked'; DROP TABLE users--", "test").WhereEq("id", 1)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious update set column")
	}
}

func TestSQLInjection_DeleteWhere(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Delete("users").WhereEq("1 = 1; DROP TABLE users--", 0)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious delete where column")
	}
}

func TestSQLInjection_InColumn(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Select().From("users").WhereIn("id); DROP TABLE users--", 1, 2, 3)
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious IN column")
	}
}

func TestSQLInjection_GroupBy(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Select("status").From("users").GroupBy("status; DROP TABLE users--")
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious GROUP BY column")
	}
}

func TestSQLInjection_OrderBy(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Select().From("users").OrderBy("id; DROP TABLE users--", "ASC")
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious ORDER BY column")
	}
}

func TestSQLInjection_JoinTable(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Select().From("users").Join("orders; DROP TABLE users--", clause.Raw("users.id = orders.user_id"))
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious JOIN table")
	}
}

func TestSQLInjection_ConflictColumn(t *testing.T) {
	bd := builder.Postgres()

	q := bd.Insert("users").Columns("id", "name").Values(1, "test").
		OnConflict("id); DROP TABLE users--").
		DoNothing()
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious conflict column")
	}
}

func TestSQLInjection_ReturningColumn(t *testing.T) {
	bd := builder.Postgres()

	q := bd.Insert("users").Columns("id").Values(1).Returning("id; DROP TABLE users--")
	_, _, err := q.SQL()
	if err == nil {
		t.Error("expected error for malicious RETURNING column")
	}
}

func TestSQLInjection_LimitOffset(t *testing.T) {
	bd := builder.MySQL()

	q := bd.Select().From("users").Limit(10).Offset(5)
	sqlStr, _, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(sqlStr, "LIMIT") || !contains(sqlStr, "OFFSET") {
		t.Error("expected LIMIT and OFFSET in SQL")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
