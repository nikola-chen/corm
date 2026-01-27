# corm - Lightweight Go ORM

`corm` is a lightweight, thread-safe, and easy-to-use ORM library for Go. It supports MySQL and PostgreSQL, providing a fluent Query Builder, struct mapping, and transaction management.

## Features

- **Fluent Query Builder**: Intuitive API for building SELECT, INSERT, UPDATE, and DELETE queries.
- **Struct Mapping**: Automatically map database rows to structs (and slices of structs).
- **Transaction Support**: Easy-to-use transaction management with closure-based `Transaction` helper.
- **Cross-Database**: Supports MySQL and PostgreSQL (with dialect abstraction).
- **Safety & Security**: Built-in SQL injection protection (parameter binding) and strict mode options.
- **Performance**: Optimized reflection and allocation reduction for result scanning.

## For AI/Agents

If you're using an AI coding tool or an AI agent to generate code with `corm`, read [AI_AGENT_GUIDE.md](file:///Users/macrochen/Codespace/AI/corm/AI_AGENT_GUIDE.md) first. It includes safe SQL rules, module map, and copy-paste templates.

## Installation

```bash
go get github.com/nikola-chen/corm
```

*(Note: Replace `github.com/nikola-chen/corm` with the actual repository path)*

## Quick Start

### Connection

```go
package main

import (
	"context"
	"time"

	"corm/engine"
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
		panic(err)
	}
	defer e.Close()

	// Verify connection
	if err := e.Ping(context.Background()); err != nil {
		panic(err)
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
_, err := e.InsertInto("users").
	Model(&user).
	Exec(ctx)

// Insert with specific columns
_, err := e.InsertInto("users").
	Columns("name", "age").
	Values("Bob", 25).
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
// Update using struct (fields tagged with `omitempty` are skipped unless IncludeZero is enabled)
u.Age = 31
_, err := e.Update("users").
	SetStruct(&u).
	Where("id = ?", u.ID).
	Exec(ctx)

// Update specific columns
_, err := e.Update("users").
	Set("age", 32).
	Where("name = ?", "Alice").
	Exec(ctx)
```

#### Delete

```go
_, err := e.DeleteFrom("users").
	Where("id = ?", 1).
	Exec(ctx)
```

### Transactions

`corm` provides a handy `Transaction` method that automatically commits on success and rolls back on error or panic.

```go
err := e.Transaction(ctx, func(tx *engine.Tx) error {
	// Operations inside transaction use 'tx' instead of 'e'
	if _, err := tx.InsertInto("users").Values("Dave", 40).Exec(ctx); err != nil {
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
	"time"

	"corm/engine"
	"corm/clause"
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
		panic(err)
	}
	defer e.Close()

	ctx := context.Background()

	// 2. Insert with Model & Returning (PostgreSQL support)
	newUser := User{Name: "John Doe", Email: "john@example.com", Age: 30, Status: 1}
	var newID int
	
	// Note: MySQL doesn't support RETURNING in older versions, this is for demo.
	// For MySQL, use LastInsertId() from Result.
	if e.DriverName() == "postgres" {
		err = e.InsertInto("").Model(&newUser).Returning("id").One(ctx, &newID)
	} else {
		res, _ := e.InsertInto("").Model(&newUser).Exec(ctx)
		id, _ := res.LastInsertId()
		newID = int(id)
	}

	// 3. Batch Insert using Values
	e.InsertInto("users").
		Columns("name", "email", "age", "status").
		Values("Alice", "alice@test.com", 25, 1).
		Values("Bob", "bob@test.com", 28, 0).
		Exec(ctx)

	// 4. Complex Select Query
	// SELECT u.id, u.name, count(o.id) as order_count 
	// FROM users u 
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
	err = e.Select("u.id", "u.name", "count(o.id) as order_count").
		From("users u").
		Join("LEFT JOIN orders o ON o.user_id = u.id").
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

	// 5. Update using SetMap and SetStruct
	// Update via Struct (auto-infers table from struct)
	updateUser := User{ID: newID, Name: "John Updated"}
	e.Update("").
		SetStruct(&updateUser). // Only updates non-zero fields (unless configured otherwise)
		Where("id = ?", newID).
		Exec(ctx)

	// Update via Map or Set method
	e.Update("users").
		SetMap(map[string]any{"status": 0}).
		Set("updated_at", time.Now()).
		Where("age < ?", 20).
		Exec(ctx)

	// 6. Transaction
	err = e.Transaction(ctx, func(tx *engine.Tx) error {
		// Use 'tx' for all operations inside the transaction
		
		// 6.1 Lock row (if needed, via Raw SQL)
		// tx.ExecContext(ctx, "SELECT * FROM users WHERE id = ? FOR UPDATE", newID)
		
		// 6.2 Perform updates
		if _, err := tx.DeleteFrom("users").Where("status = ?", 0).Exec(ctx); err != nil {
			return err // Rollback
		}
		
		// 6.3 Insert log
		if _, err := tx.InsertInto("logs").Columns("msg").Values("Cleanup done").Exec(ctx); err != nil {
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
    SlowQuery: 200 * time.Millisecond,
}))
```

### Raw SQL

For complex queries, you can use `Raw` clauses, but be careful with SQL injection if you manually concatenate strings.

```go
e.Select().
    WhereRaw("age > ? AND name LIKE ?", 18, "A%").
    All(ctx, &users)
```

## License

MIT
