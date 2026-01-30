package corm_test

import (
	"testing"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/schema"
)

type BenchUser struct {
	ID        int    `db:"id,pk"`
	Name      string `db:"name"`
	Email     string `db:"email"`
	Age       int    `db:"age"`
	Status    int    `db:"status"`
	CreatedAt string `db:"created_at"`
}

func (BenchUser) TableName() string { return "users" }

// BenchmarkSchemaParse
func BenchmarkSchemaParse(b *testing.B) {
	user := BenchUser{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := schema.Parse(&user)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSelectBuild
func BenchmarkSelectBuild(b *testing.B) {
	qb := builder.MySQL()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := qb.Select("id", "name", "email").
			From("users").
			Where("age > ?", 18).
			Where("status = ?", 1).
			OrderByDesc("created_at").
			Limit(10).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkInsertBuild
func BenchmarkInsertBuild(b *testing.B) {
	qb := builder.MySQL()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := qb.Insert("users").
			Columns("name", "email", "age").
			Values("Alice", "alice@test.com", 25).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUpdateBuild
func BenchmarkUpdateBuild(b *testing.B) {
	qb := builder.MySQL()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := qb.Update("users").
			Set("name", "Bob").
			Set("age", 30).
			Where("id = ?", 1).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDeleteBuild
func BenchmarkDeleteBuild(b *testing.B) {
	qb := builder.MySQL()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := qb.Delete("users").
			Where("id = ?", 1).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSelectBuildPostgres
func BenchmarkSelectBuildPostgres(b *testing.B) {
	qb := builder.Postgres()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := qb.Select("id", "name", "email").
			From("users").
			Where("age > ?", 18).
			Where("status = ?", 1).
			OrderByDesc("created_at").
			Limit(10).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSelectComplex
func BenchmarkSelectComplex(b *testing.B) {
	qb := builder.MySQL()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := qb.Select("u.id", "u.name", "p.title").
			FromAs("users", "u").
			LeftJoinAs("posts", "p", clause.Raw("u.id = p.user_id")).
			Where("u.age > ?", 18).
			WhereIn("u.status", []int{1, 2, 3}).
			GroupBy("u.id").
			Having("count(*) > ?", 5).
			OrderByDesc("u.created_at").
			Limit(20).
			Offset(10).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkToSnake
func BenchmarkToSnake(b *testing.B) {
	testCases := []string{
		"UserID",
		"CreatedAt",
		"HTTPResponseCode",
		"simple",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range testCases {
			_ = schema.ToSnake(s)
		}
	}
}
