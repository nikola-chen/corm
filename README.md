# corm - Lightweight Go ORM

`corm` is a lightweight and easy-to-use ORM library for Go. It supports MySQL and PostgreSQL, providing a fluent Query Builder, struct mapping, and transaction management.

Concurrency note:
- `Engine` is safe to share across goroutines.
- Query builders (e.g. `e.Select(...).Where(...)`) are mutable and must not be shared across goroutines.

## Features

- **Fluent Query Builder**: Intuitive API for building SELECT, INSERT, UPDATE, and DELETE queries.
- **Struct Mapping**: Automatically map database rows to structs (and slices of structs).
- **Transaction Support**: Easy-to-use transaction management with closure-based `Transaction` helper.
- **Cross-Database**: Supports MySQL and PostgreSQL (with dialect abstraction).
- **Safety & Security**: Built-in SQL injection protection (parameter binding) and safe identifier quoting.
- **Performance**: Optimized reflection and allocation reduction for result scanning.

## For AI/Agents

If you're using an AI coding tool or an AI agent to generate code with `corm`, read [AI_AGENT_GUIDE.md](file:///Users/macrochen/Codespace/AI/corm/AI_AGENT_GUIDE.md) first. It includes safe SQL rules, module map, and copy-paste templates.

## Installation

```bash
go get github.com/nikola-chen/corm
```

## Quick Start

### Connection

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/nikola-chen/corm/engine"
	_ "github.com/go-sql-driver/mysql"
	// _ "github.com/lib/pq" // for postgres
)

func main() {
	// Open connection
	e, err := engine.Open("mysql", "user:pass@tcp(localhost:3306)/dbname?parseTime=true",
		engine.WithConfig(engine.Config{
			MaxOpenConns: 10,
			MaxIdleConns: 5,
			LogSQL:       true, // Enable SQL logging
		}),
	)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer e.Close()

	// Verify connection
	ctx := context.Background()
	if err := e.Ping(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	// Optional: if you prefer builder-style in your own wrappers, bind dialect + executor once:
	qb := e.Builder()
	var rows []map[string]any
	if err := qb.Select("id").From("users").Limit(1).All(ctx, &rows); err != nil {
		log.Fatalf("select: %v", err)
	}
}
```

### Struct Definition

```go
type User struct {
	ID        int       `db:"id,pk"`
	Name      string    `db:"name"`
	Age       int       `db:"age"`
	CreatedAt time.Time `db:"created_at,readonly"`
}

func (u User) TableName() string {
	return "users"
}
```

### CRUD Operations

#### Insert

```go
ctx := context.Background()
user := User{Name: "Alice", Age: 30}

// Insert a record
_, err := e.Insert("users").
	Model(&user).
	Exec(ctx)

// Insert with specific columns
_, err := e.Insert("users").
	Columns("name", "age").
	Values("Bob", 25).
	Exec(ctx)

// Insert with map (map[string]any)
_, err := e.Insert("users").
	Map(map[string]any{"name": "Carol", "age": 20}).
	Exec(ctx)

// High-throughput inserts with predefined columns:
// If your map keys are already normalized to lower-case, prefer MapsLowerKeys to reduce per-row overhead.
rows := []map[string]any{
	{"name": "Alice", "age": 25},
	{"name": "Bob", "age": 28},
}
_, err = e.Insert("users").
	Columns("name", "age").
	MapsLowerKeys(rows).
	Exec(ctx)
```

#### Select

```go
// Select one record
var u User
err := e.Select("id", "name", "age").
	From("users").
	Where("id = ?", 1).
	One(ctx, &u)

// Select multiple records
var users []User
err := e.Select().
	From("users").
	Where("age > ?", 18).
	OrderByDesc("age").
	Limit(10).
	Offset(0).
	All(ctx, &users)

// Select with IN clause
err := e.Select().
	From("users").
	WhereIn("id", []int{1, 2, 3}).
	All(ctx, &users)
```

#### Update

```go
// Update with struct model (fields tagged with `omitempty` are skipped unless IncludeZero is enabled)
u.Age = 31
_, err := e.Update("users").
	Model(&u).
	Where("id = ?", u.ID).
	Exec(ctx)

// Update with explicit columns
_, err := e.Update("users").
	Set("age", 32).
	Where("name = ?", "Alice").
	WhereLike("email", "%@example.com").
	Exec(ctx)

// Update with map (keys must be valid column identifiers)
_, err = e.Update("users").
	Map(map[string]any{"age": 33}).
	Where("id = ?", 1).
	Exec(ctx)

// Batch update (single SQL via CASE WHEN)
batch := []User{
    {ID: 1, Name: "Alice", Age: 25},
    {ID: 2, Name: "Bob", Age: 28},
}
_, err = e.Update("").Models(batch).Exec(ctx)
```

Safety note:
- `Update(table)` requires a non-empty WHERE by default (to prevent updating the whole table).
- If you really want to update all rows, use `AllowEmptyWhere()` explicitly.

#### Delete

```go
_, err := e.Delete("users").
	Where("id = ?", 1).
	Exec(ctx)
```

Safety note:
- `Delete(table)` requires a non-empty WHERE by default (to prevent deleting the whole table).
- If you really want to delete all rows, use `AllowEmptyWhere()` explicitly:

```go
_, err := e.Delete("users").AllowEmptyWhere().Exec(ctx)
```

### Transactions

`corm` provides a handy `Transaction` method that automatically commits on success and rolls back on error or panic.

```go
err := e.Transaction(ctx, func(tx *engine.Tx) error {
	// Operations inside transaction use 'tx' instead of 'e'
	if _, err := tx.Insert("users").Values("Dave", 40).Exec(ctx); err != nil {
		return err
	}
	
	if _, err := tx.Update("accounts").Set("balance", 100).Where("user_id = ?", 1).Exec(ctx); err != nil {
		return err
	}

	return nil // Commit
})
```

## Comprehensive Example

Here is a complete example showcasing most of the features including configuration, complex queries, transactions, and advanced CRUD operations.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nikola-chen/corm/engine"
	"github.com/nikola-chen/corm/clause"
	_ "github.com/go-sql-driver/mysql"
)

// User schema definition
type User struct {
	ID        int       `db:"id,pk"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Age       int       `db:"age"`
	Status    int       `db:"status"` // 0: inactive, 1: active
	CreatedAt time.Time `db:"created_at,readonly"`
	UpdatedAt time.Time `db:"updated_at,omitempty"`
}

func (User) TableName() string { return "users" }

func main() {
	// 1. Initialize Engine with Configuration
	e, err := engine.Open("mysql", "user:pass@tcp(localhost:3306)/testdb?parseTime=true",
		engine.WithConfig(engine.Config{
			MaxOpenConns: 20,
			MaxIdleConns: 10,
			LogSQL:       true, // Print generated SQL to stdout
			SlowQuery:    100 * time.Millisecond,
		}),
	)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer e.Close()

	ctx := context.Background()

	// 2. Insert with Model & Returning (PostgreSQL support)
	newUser := User{Name: "John Doe", Email: "john@example.com", Age: 30, Status: 1}
	newID, err := e.Insert("").Model(&newUser).ExecAndReturnID(ctx, "id")
	if err != nil {
		log.Fatalf("insert user: %v", err)
	}

	// 3. Batch Insert using Values
	e.Insert("users").
		Columns("name", "email", "age", "status").
		Values("Alice", "alice@test.com", 25, 1).
		Values("Bob", "bob@test.com", 28, 0).
		Exec(ctx)

	// 3.1 Batch Insert using struct slice
	users := []User{
		{Name: "Alice", Email: "alice@test.com", Age: 25, Status: 1},
		{Name: "Bob", Email: "bob@test.com", Age: 28, Status: 0},
	}
	e.Insert("").Models(users).Exec(ctx)

	// 3.2 Batch Insert using map slice
	rows := []map[string]any{
		{"name": "Alice", "email": "alice@test.com", "age": 25, "status": 1},
		{"name": "Bob", "email": "bob@test.com", "age": 28, "status": 0},
	}
	e.Insert("users").Columns("name", "email", "age", "status").Maps(rows).Exec(ctx)

	// 4. Complex Select Query
	// SELECT u.id, u.name, count(o.id) as order_count 
	// FROM users AS u 
	// LEFT JOIN orders o ON o.user_id = u.id 
	// WHERE u.status = 1 AND u.age > 18 
	// GROUP BY u.id 
	// HAVING order_count >= 0 
	// ORDER BY u.age DESC 
	// LIMIT 10 OFFSET 0
	
	type UserStat struct {
		ID         int    `db:"id"`
		Name       string `db:"name"`
		OrderCount int    `db:"order_count"`
	}
	
	var stats []UserStat
	err = e.Select("u.id", "u.name").
		SelectExpr(clause.Raw("count(o.id) as order_count")).
		FromAs("users", "u").
		LeftJoinAs("orders", "o", clause.Raw("o.user_id = u.id")).
		Where("u.status = ?", 1).
		Where("u.age > ?", 18).
		WhereIn("u.id", []int{1, 2, 3, 4, 5}). // Helper for IN clause
		GroupBy("u.id", "u.name").
		Having("order_count >= ?", 0).
		OrderByDesc("u.age").
		Limit(10).
		Offset(0).
		All(ctx, &stats)
		
	if err != nil {
		fmt.Printf("Query failed: %v\n", err)
	}

	// 5. Update using Map and Model
	// Update via Struct (auto-infers table from struct)
	updateUser := User{ID: newID, Name: "John Updated"}
	e.Update("").
		Model(&updateUser).
		Where("id = ?", newID).
		Exec(ctx)

	// Update via Map or Set method
	e.Update("users").
		Map(map[string]any{"status": 0}).
		Set("updated_at", time.Now()).
		Where("age < ?", 20).
		Exec(ctx)

	// Update Batch using Maps
	updateRows := []map[string]any{
		{"id": 1, "status": 1, "age": 26},
		{"id": 2, "status": 0, "age": 29},
	}
	// CASE-WHEN bulk update based on 'id'
	e.Update("users").Key("id").Maps(updateRows).Exec(ctx)

	// Update Batch using Maps with Extra Where
	// This generates: UPDATE ... WHERE id IN (...) AND status = 1
	e.Update("users").
		Key("id").
		Maps(updateRows).
		Where("status = ?", 1).
		Exec(ctx)

	// 5. Update with Limit (MySQL only)
	_, err = e.Update("users").
		Set("status", 0).
		Where("age < ?", 18).
		Limit(100). // Limit affected rows
		Exec(ctx)

	// 6. Transaction
	err = e.Transaction(ctx, func(tx *engine.Tx) error {
		// Use 'tx' for all operations inside the transaction
		
		// 6.1 Lock row (if needed)
		// _ = tx.Select("*").From("users").Where("id = ?", newID).ForUpdate().One(ctx, &User{})
		
		// 6.2 Perform updates
		if _, err := tx.Delete("users").Where("status = ?", 0).Exec(ctx); err != nil {
			return err // Rollback
		}
		
		// 6.3 Insert log
		if _, err := tx.Insert("logs").Columns("msg").Values("Cleanup done").Exec(ctx); err != nil {
			return err // Rollback
		}

		return nil // Commit
	})
	
	if err != nil {
		fmt.Printf("Transaction failed: %v\n", err)
	}
}
```

## Advanced Usage

### SQL Logging
 
 Enable logging via `WithConfig`:
 
 ```go
 engine.Open("mysql", dsn, engine.WithConfig(engine.Config{
     LogSQL:    true,
     LogArgs:   true, // Enable argument logging (redacted by default for security)
     SlowQuery: 200 * time.Millisecond,
 }))
 ```

### Raw SQL

For complex queries, you can use `Raw` clauses, but be careful with SQL injection if you manually concatenate strings.

```go
e.Select().
    Where("age > ? AND name LIKE ?", 18, "A%").
    All(ctx, &users)
```

Safety note:
- Treat these as dangerous entry points unless the SQL is a trusted constant/whitelist: `Where`, `JoinRaw`, `Having`, `OrderByRaw`, `SuffixRaw`, `clause.Raw`.
- Prefer structured APIs like `WhereEq`, `WhereIn`, `OrderByAsc/Desc`, and `Join/JoinAs` whenever possible.

Note (PostgreSQL):
- When using string-based SQL fragments with args (e.g. `Where("x = ?", v)`), use `?` as the placeholder in the fragment.
- Avoid mixing JSONB operators `?/?|/?&` with `?` placeholders in the same parameterized fragment. Prefer `jsonb_exists/jsonb_exists_any/jsonb_exists_all` functions.

### SQL Builder (Without Execution)

If you only need to build SQL strings without executing them (e.g., for use with other libraries or testing), you can use the `builder` package directly with the new `API` helper.

```go
import "github.com/nikola-chen/corm/builder"

// Initialize a builder for MySQL (or Postgres)
// Note: Ensure the DB driver is imported (e.g. _ "github.com/go-sql-driver/mysql").
qb := builder.MySQL()
// Or: qb := builder.Postgres()
// Or: qb := builder.Dialect(driverName)       // carries error until SQL()/Exec()/Query()
// Or: qb := builder.MustDialect(driverName)   // panics early if unsupported (avoid in request path)
// Or: qb := builder.For(driverName, db)       // binds executor + dialect in one line
// Or: qb := builder.MustFor(driverName, db)   // panics early if unsupported (avoid in request path)

// Build UPDATE string
sqlStr, args, err := qb.Update("users").
    Set("name", "New Name").
    Where("id = ?", 1).
    SQL()

// Build SELECT string
sqlStr, args, err = qb.Select("id", "name").
    From("users").
    Where("age > ?", 18).
    SQL()
```

## Advanced Features

`corm` now supports a wide range of advanced SQL features.

Security note:
- `clause.Raw(...)`, `JoinRaw(...)`, `OrderByRaw(...)`, `SuffixRaw(...)` accept raw SQL. Never pass untrusted user input into these APIs.

### Logical Operators

```go
import "github.com/nikola-chen/corm/clause"

e.Select().From("users").
    WhereExpr(clause.Not(clause.Raw("age < ?", 18))).
    WhereExpr(clause.IsNull("deleted_at")).
    WhereExpr(clause.IsNotNull("email")).
    All(ctx, &users)
```

### JOINs

Support for structured joins (`Join`/`LeftJoin`/`RightJoin`/`InnerJoin`/`FullJoin`/`CrossJoin`) and raw joins (`JoinRaw`).

Recommended usage with arguments (using `FromAs` + `*JoinAs`):

```go
import "github.com/nikola-chen/corm/clause"

e.Select("u.name").
    FromAs("users", "u").
    LeftJoinAs("orders", "o", clause.And(
        clause.Raw("u.id = o.user_id"),
        clause.Eq("o.status", "active"), // Bind: "active"
    )).
    All(ctx, &results)
```

### Nested Transactions (Savepoints)

`corm` supports nested transactions via `SAVEPOINT`. You can call `tx.Transaction` inside a transaction block.

```go
import (
    "errors"
    "fmt"

    "github.com/nikola-chen/corm/engine"
)

err := e.Transaction(ctx, func(tx *engine.Tx) error {
    if _, err := tx.Insert("logs").Values("Start").Exec(ctx); err != nil {
        return fmt.Errorf("failed to insert log: %w", err)
    }

    // Nested transaction
    if err := tx.Transaction(ctx, func(subTx *engine.Tx) error {
        if _, err := subTx.Insert("users").Values("New User").Exec(ctx); err != nil {
            return err
        }
        return errors.New("oops") // Triggers rollback of sub-transaction
    }); err != nil {
        // Handle sub-transaction error (optional)
        return err
    }

    return nil
})
```

### Subqueries

**Nested SELECT in FROM:**

```go
sub := e.Select("id", "name").From("users").Where("age > ?", 18)

e.Select("u.name").
    FromSelect(sub, "u"). // SELECT ... FROM (SELECT ...) AS u
    All(ctx, &results)
```

**Subquery in WHERE:**

```go
sub := e.Select("id").From("banned_users")

e.Select().From("users").
    WhereInSubquery("id", sub). // WHERE id IN (SELECT id FROM banned_users)
    All(ctx, &users)
```

**INSERT INTO ... SELECT:**

```go
sub := e.Select("id", "name").From("old_users")

e.Insert("new_users").
    Columns("id", "name").
    FromSelect(sub).
    Exec(ctx)
```

### Aggregates

Helpers for `Count`, `Sum`, `Avg`, `Max`, `Min`.

```go
type Agg struct {
    Cnt    int     `db:"cnt"`
    AvgAge float64 `db:"avg_age"`
}
var a Agg
err := e.Select().
    SelectExpr(
        clause.Alias(clause.Count("id"), "cnt"),
        clause.Alias(clause.Avg("age"), "avg_age"),
    ).
    From("users").
    One(ctx, &a)
```

### UNION / UNION ALL

```go
q1 := e.Select("id").From("users_2023")
q2 := e.Select("id").From("users_2024")

// SELECT id FROM users_2023 UNION ALL SELECT id FROM users_2024
q1.UnionAll(q2).All(ctx, &ids)
```

### DISTINCT & LIMIT

```go
e.Select("name").From("users").Distinct().Limit(5).All(ctx, &names)
```

### Map Operations

```go
// INSERT with Map (keys are sorted for determinism)
_, err := e.Insert("users").
	Map(map[string]any{"name": "Alice", "age": 30}).
	Exec(ctx)

// UPDATE with Map
_, err := e.Update("users").
	Map(map[string]any{
		"age": 31,
		"status": "active",
	}).
	Where("name = ?", "Alice").
	Exec(ctx)

// SELECT with WhereMap (automatic AND)
err := e.Select("id", "name").
	From("users").
	WhereMap(map[string]any{
		"age": 30,
		"active": true,
	}).
	All(ctx, &users)
```

## Changelog

### v1.1.3

**Security Fixes:**
- Fixed SAVEPOINT name validation to prevent potential SQL injection
- Enhanced HAVING clause validation to return explicit errors instead of silently skipping empty expressions

**Performance Optimizations:**
- Extracted `NormalizeColumn` to `internal` package to eliminate code duplication
- Optimized memory allocation using `sync.Pool` (ToSnake, colsKey)
- Pre-allocated argBuilder args slice to reduce expansion overhead

**API Improvements:**
- Enhanced error messages with clearer debugging guidance
- Optimized chain API to be closer to SQL primitives

### v1.1.2

- Refactored placeholder rewrite functions to eliminate duplication
- Unified column normalization functions
- Added comprehensive documentation and AI Agent Guide

## License

MIT
