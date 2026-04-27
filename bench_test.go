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
	for b.Loop() {
		_, err := schema.Parse(&user)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkToSnakeASCII(b *testing.B) {
	b.Run("CamelCase", func(b *testing.B) {
		for b.Loop() {
			_ = schema.ToSnake("HTTPResponseCode")
		}
	})
	b.Run("AlreadySnake", func(b *testing.B) {
		for b.Loop() {
			_ = schema.ToSnake("already_snake_case")
		}
	})
	b.Run("SingleWord", func(b *testing.B) {
		for b.Loop() {
			_ = schema.ToSnake("simple")
		}
	})
	b.Run("Empty", func(b *testing.B) {
		for b.Loop() {
			_ = schema.ToSnake("")
		}
	})
}

func BenchmarkToSnakeUnicode(b *testing.B) {
	b.Run("GermanUmlaut", func(b *testing.B) {
		for b.Loop() {
			_ = schema.ToSnake("GrößeHandel")
		}
	})
	b.Run("Cyrillic", func(b *testing.B) {
		for b.Loop() {
			_ = schema.ToSnake("ПриветМир")
		}
	})
	b.Run("MixedASCIIUnicode", func(b *testing.B) {
		for b.Loop() {
			_ = schema.ToSnake("HelloWörld")
		}
	})
}

func BenchmarkToSnakeCacheHit(b *testing.B) {
	schema.ToSnake("CachedColumnName")
	b.ResetTimer()
	for b.Loop() {
		_ = schema.ToSnake("CachedColumnName")
	}
}

func BenchmarkSchemaParseCacheHit(b *testing.B) {
	schema.Parse(BenchUser{})
	b.ResetTimer()
	for b.Loop() {
		_, err := schema.Parse(BenchUser{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSchemaColumnsAndValues(b *testing.B) {
	user := BenchUser{ID: 1, Name: "Alice", Email: "alice@test.com", Age: 25, Status: 1, CreatedAt: "2024-01-01"}
	s, err := schema.Parse(&user)
	if err != nil {
		b.Fatal(err)
	}
	opts := schema.ExtractOptions{IncludePrimaryKey: true}
	b.ResetTimer()
	for b.Loop() {
		_, _, err := s.ColumnsAndValues(user, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSchemaColumnsAndValuesOmitEmpty(b *testing.B) {
	type OmitModel struct {
		ID   int    `db:"id,pk"`
		Name string `db:"name,omitempty"`
		Age  int    `db:"age,omitempty"`
	}
	model := OmitModel{ID: 1, Name: "Alice", Age: 0}
	s, err := schema.Parse(&model)
	if err != nil {
		b.Fatal(err)
	}
	opts := schema.ExtractOptions{}
	b.ResetTimer()
	for b.Loop() {
		_, _, err := s.ColumnsAndValues(model, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelectBuildParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			qb := builder.MySQL()
			_, _, err := qb.Select("id", "name").
				From("users").
				Where("age > ?", 18).
				Limit(10).
				SQL()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkInsertBuildParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			qb := builder.MySQL()
			_, _, err := qb.Insert("users").
				Columns("name", "age").
				Values("Alice", 25).
				SQL()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkSchemaParseParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := schema.Parse(BenchUser{})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkToSnakeParallel(b *testing.B) {
	inputs := []string{"UserID", "CreatedAt", "HTTPResponseCode", "already_snake"}
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_ = schema.ToSnake(inputs[i%len(inputs)])
			i++
		}
	})
}

func BenchmarkLargeWhereInBuild(b *testing.B) {
	qb := builder.MySQL()
	ids := make([]int, 100)
	for i := range ids {
		ids[i] = i + 1
	}
	b.ResetTimer()
	for b.Loop() {
		_, _, err := qb.Select("*").
			From("users").
			WhereIn("id", ids).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkInsertBatchLargeBuild(b *testing.B) {
	qb := builder.MySQL()
	users := make([]BenchUser, 50)
	for i := range users {
		users[i] = BenchUser{Name: "User" + string(rune('A'+i%26)), Age: 20 + i%30}
	}
	b.ResetTimer()
	for b.Loop() {
		_, _, err := qb.Insert("users").
			Models(users).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSelectBuild
func BenchmarkSelectBuild(b *testing.B) {
	qb := builder.MySQL()
	b.ResetTimer()
	for b.Loop() {
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
	for b.Loop() {
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
	for b.Loop() {
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
	for b.Loop() {
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
	for b.Loop() {
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
	for b.Loop() {
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
	for b.Loop() {
		for _, s := range testCases {
			_ = schema.ToSnake(s)
		}
	}
}

// BenchmarkBatchUpdateBuild
func BenchmarkBatchUpdateBuild(b *testing.B) {
	qb := builder.MySQL()
	users := []BenchUser{
		{ID: 1, Name: "Alice", Age: 25},
		{ID: 2, Name: "Bob", Age: 30},
		{ID: 3, Name: "Charlie", Age: 35},
	}
	b.ResetTimer()
	for b.Loop() {
		_, _, err := qb.Update("users").
			Models(users).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkInsertBatchBuild
func BenchmarkInsertBatchBuild(b *testing.B) {
	qb := builder.MySQL()
	users := []BenchUser{
		{Name: "Alice", Age: 25},
		{Name: "Bob", Age: 30},
		{Name: "Charlie", Age: 35},
	}
	b.ResetTimer()
	for b.Loop() {
		_, _, err := qb.Insert("users").
			Models(users).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWhereInBuild
func BenchmarkWhereInBuild(b *testing.B) {
	qb := builder.MySQL()
	ids := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b.ResetTimer()
	for b.Loop() {
		_, _, err := qb.Select("*").
			From("users").
			WhereIn("id", ids).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJoinComplexBuild
func BenchmarkJoinComplexBuild(b *testing.B) {
	qb := builder.MySQL()
	b.ResetTimer()
	for b.Loop() {
		_, _, err := qb.Select("u.name", "p.title", "c.content").
			FromAs("users", "u").
			LeftJoinAs("posts", "p", clause.Raw("u.id = p.user_id")).
			InnerJoinAs("comments", "c", clause.Raw("p.id = c.post_id")).
			Where("u.status = ?", 1).
			OrderBy("u.created_at", "ASC").
			Limit(50).
			SQL()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSchemaParseComplex
func BenchmarkSchemaParseComplex(b *testing.B) {
	type ComplexStruct struct {
		ID          int    `db:"id,pk"`
		Name        string `db:"name"`
		Email       string `db:"email"`
		Age         int    `db:"age"`
		Status      int    `db:"status"`
		CreatedAt   string `db:"created_at"`
		UpdatedAt   string `db:"updated_at"`
		Profile     string `db:"profile"`
		Preferences string `db:"preferences"`
		Metadata    string `db:"metadata"`
	}
	user := ComplexStruct{}
	b.ResetTimer()
	for b.Loop() {
		_, err := schema.Parse(&user)
		if err != nil {
			b.Fatal(err)
		}
	}
}
