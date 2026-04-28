package builder_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nikola-chen/corm/builder"
)

func TestSelectAll_NoExecutor(t *testing.T) {
	b := builder.Dialect("mysql").Select("*").From("users")
	var dest []map[string]any
	err := b.All(context.Background(), &dest)
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestSelectOne_NoExecutor(t *testing.T) {
	b := builder.Dialect("mysql").Select("*").From("users")
	var dest map[string]any
	err := b.One(context.Background(), &dest)
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestSelectScalar_NoExecutor(t *testing.T) {
	b := builder.Dialect("mysql").Select("id").From("users")
	var dest int64
	err := b.Scalar(context.Background(), &dest)
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestSelectCount_NoExecutor(t *testing.T) {
	b := builder.Dialect("mysql").Select("*").From("users")
	_, err := b.Count(context.Background())
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestSelectExists_NoExecutor(t *testing.T) {
	b := builder.Dialect("mysql").Select("*").From("users")
	_, err := b.Exists(context.Background())
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestInsertOne_NoExecutor(t *testing.T) {
	var dest int64
	b := builder.Dialect("mysql").Insert("users")
	err := b.One(context.Background(), &dest)
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestInsertOne_NoReturning(t *testing.T) {
	var dest int64
	b := builder.Dialect("mysql").Insert("users").Columns("name").Values("test")
	err := b.One(context.Background(), &dest)
	if err == nil {
		t.Fatal("expected error for missing RETURNING")
	}
}

func TestInsertOne_UnsupportedDialect(t *testing.T) {
	var dest int64
	b := builder.Dialect("mysql").Insert("users").Columns("name").Values("test").Returning("id")
	err := b.One(context.Background(), &dest)
	if err == nil {
		t.Fatal("expected error for unsupported RETURNING on MySQL")
	}
}

func TestIter_NoExecutor(t *testing.T) {
	seq := builder.Iter[map[string]any](context.Background(), builder.Dialect("mysql").Select("*").From("users"))
	_, err := func() (map[string]any, error) {
		for v, e := range seq {
			return v, e
		}
		return nil, sql.ErrNoRows
	}()
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}
