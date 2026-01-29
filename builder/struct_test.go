package builder_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/dialect"
)

type User struct {
	ID        int       `db:"id,pk"`
	Name      string    `db:"name"`
	Age       int       `db:"age"`
	CreatedAt time.Time `db:"created_at"`
}

func (u User) TableName() string {
	return "users"
}

type mockExecutor struct {
	sql  string
	args []any
}

func (m *mockExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	m.sql = query
	m.args = args
	return nil, nil
}

func (m *mockExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	m.sql = query
	m.args = args
	return nil, nil
}

func TestInsertRecord(t *testing.T) {
	exec := &mockExecutor{}
	qb := builder.NewAPI(dialect.MustGet("mysql"), exec)

	user := User{
		Name:      "Alice",
		Age:       30,
		CreatedAt: time.Now(),
	}

	b := qb.Insert("").Model(user)
	sqlStr, args, err := b.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sqlStr, "INSERT INTO `users`") {
		t.Errorf("expected INSERT INTO `users`, got %s", sqlStr)
	}
	if !strings.Contains(sqlStr, "`name`") || !strings.Contains(sqlStr, "`age`") {
		t.Errorf("missing columns in SQL: %s", sqlStr)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestUpdateModel(t *testing.T) {
	exec := &mockExecutor{}
	qb := builder.NewAPI(dialect.MustGet("postgres"), exec)

	user := User{
		Name: "Bob",
		Age:  25,
	}

	b := qb.Update("").Model(user).Where("id = ?", 1)
	sqlStr, args, err := b.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sqlStr, `UPDATE "users" SET`) {
		t.Errorf("expected UPDATE \"users\" SET, got %s", sqlStr)
	}
	if !strings.Contains(sqlStr, `"name" = $`) || !strings.Contains(sqlStr, `"age" = $`) {
		t.Errorf("missing columns in SQL: %s", sqlStr)
	}
	if len(args) != 4 {
		t.Errorf("expected 4 args, got %d", len(args))
	}
}
