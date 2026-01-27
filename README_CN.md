# corm - 轻量级 Go ORM

`corm` 是一个轻量级、线程安全且易于使用的 Go 语言 ORM 库。它支持 MySQL 和 PostgreSQL，提供流畅的查询构建器、结构体映射和事务管理功能。

## 特性

- **流畅的查询构建器**：提供直观的 API 用于构建 SELECT、INSERT、UPDATE 和 DELETE 查询。
- **结构体映射**：自动将数据库行映射到结构体（以及结构体切片）。
- **事务支持**：提供基于闭包的 `Transaction` 辅助方法，简化事务管理。
- **跨数据库支持**：支持 MySQL 和 PostgreSQL（通过方言抽象）。
- **安全与防护**：内置 SQL 注入防护（参数绑定）和严格模式选项。
- **高性能**：针对结果集扫描进行了反射优化和内存分配缩减。

## 面向 AI/Agent

如果你正在使用外部 AI 自动编程工具或 AI Agent 来生成/修改使用 `corm` 的代码，建议先阅读 [AI_AGENT_GUIDE.md](file:///Users/macrochen/Codespace/AI/corm/AI_AGENT_GUIDE.md)。该文档提供安全约束、模块地图与可复制的代码模板，能显著降低生成代码的歧义与风险。

## 安装

```bash
go get github.com/nikola-chen/corm
```

## 快速开始

### 连接数据库

```go
package main

import (
	"context"
	"time"

	"github.com/nikola-chen/corm/engine"
	_ "github.com/go-sql-driver/mysql"
	// _ "github.com/lib/pq" // for postgres
)

func main() {
	// 打开连接
	e, err := engine.Open("mysql", "user:pass@tcp(localhost:3306)/dbname?parseTime=true",
		engine.WithConfig(engine.Config{
			MaxOpenConns: 10,
			MaxIdleConns: 5,
			LogSQL:       true, // 开启 SQL 日志
		}),
	)
	if err != nil {
		panic(err)
	}
	defer e.Close()

	// 验证连接
	if err := e.Ping(context.Background()); err != nil {
		panic(err)
	}
}
```

### 定义结构体

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

### CRUD 操作

#### 插入 (Insert)

```go
ctx := context.Background()
user := User{Name: "Alice", Age: 30}

// 插入一条记录
_, err := e.InsertInto("users").
	Model(&user).
	Exec(ctx)

// 插入指定列
_, err := e.InsertInto("users").
	Columns("name", "age").
	Values("Bob", 25).
	Exec(ctx)
```

#### 查询 (Select)

```go
// 查询单条记录
var u User
err := e.Select("id", "name", "age").
	From("users").
	Where("id = ?", 1).
	One(ctx, &u)

// 查询多条记录
var users []User
err := e.Select().
	From("users").
	Where("age > ?", 18).
	OrderByDesc("age").
	Limit(10).
	Offset(0).
	All(ctx, &users)

// 使用 IN 查询
err := e.Select().
	From("users").
	WhereIn("id", []int{1, 2, 3}).
	All(ctx, &users)
```

#### 更新 (Update)

```go
// 使用结构体更新（标记了 `omitempty` 的字段在未启用 IncludeZero 时会被跳过）
u.Age = 31
_, err := e.Update("users").
	SetStruct(&u).
	Where("id = ?", u.ID).
	Exec(ctx)

// 更新指定列
_, err := e.Update("users").
	Set("age", 32).
	Where("name = ?", "Alice").
	Exec(ctx)
```

#### 删除 (Delete)

```go
_, err := e.DeleteFrom("users").
	Where("id = ?", 1).
	Exec(ctx)
```

### 事务管理

`corm` 提供了一个便捷的 `Transaction` 方法，它会在函数成功执行后自动提交，若发生错误或 panic 则自动回滚。

```go
err := e.Transaction(ctx, func(tx *engine.Tx) error {
	// 在事务内部的操作请使用 'tx' 而不是 'e'
	if _, err := tx.InsertInto("users").Values("Dave", 40).Exec(ctx); err != nil {
		return err
	}
	
	if _, err := tx.Update("accounts").Set("balance", 100).Where("user_id = ?", 1).Exec(ctx); err != nil {
		return err
	}

	return nil // 提交事务
})
```

## 综合使用示例

以下示例展示了库中绝大多数核心功能，包括配置初始化、复杂查询构建、事务处理及各类 CRUD 操作。

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nikola-chen/corm/engine"
	_ "github.com/go-sql-driver/mysql"
)

// 定义用户结构体
type User struct {
	ID        int       `db:"id,pk"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Age       int       `db:"age"`
	Status    int       `db:"status"` // 0: 未激活, 1: 激活
	CreatedAt time.Time `db:"created_at,readonly"` // 插入时忽略，读取时正常
	UpdatedAt time.Time `db:"updated_at,omitempty"` // 更新时若为零值则跳过
}

// 表名定义
func (User) TableName() string { return "users" }

func main() {
	// 1. 初始化引擎与配置
	e, err := engine.Open("mysql", "user:pass@tcp(localhost:3306)/testdb?parseTime=true",
		engine.WithConfig(engine.Config{
			MaxOpenConns: 20,
			MaxIdleConns: 10,
			LogSQL:       true, // 在控制台打印生成的 SQL
			SlowQuery:    100 * time.Millisecond,
		}),
	)
	if err != nil {
		panic(err)
	}
	defer e.Close()

	ctx := context.Background()

	// 2. 插入 (Model) 与 返回值 (Returning - Postgres)
	newUser := User{Name: "John Doe", Email: "john@example.com", Age: 30, Status: 1}
	var newID int
	
	// 注意：MySQL 旧版本不支持 RETURNING。
	// 针对 MySQL，通常使用 Result.LastInsertId() 获取 ID。
	if e.DriverName() == "postgres" {
		// Postgres: 直接通过 Returning 获取新 ID
		err = e.InsertInto("").Model(&newUser).Returning("id").One(ctx, &newID)
	} else {
		// MySQL: 使用 LastInsertId
		res, _ := e.InsertInto("").Model(&newUser).Exec(ctx)
		id, _ := res.LastInsertId()
		newID = int(id)
	}

	// 3. 批量插入 (Values)
	e.InsertInto("users").
		Columns("name", "email", "age", "status").
		Values("Alice", "alice@test.com", 25, 1).
		Values("Bob", "bob@test.com", 28, 0).
		Exec(ctx)

	// 4. 复杂查询构建
	// 目标 SQL:
	// SELECT u.id, u.name, count(o.id) as order_count 
	// FROM users u 
	// LEFT JOIN orders o ON o.user_id = u.id 
	// WHERE u.status = 1 AND u.age > 18 AND u.id IN (1,2,3,4,5)
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
		WhereIn("u.id", []int{1, 2, 3, 4, 5}). // 自动展开切片为 IN (?,?,...)
		GroupBy("u.id", "u.name").
		Having("order_count >= ?", 0).
		OrderByDesc("u.age").
		Limit(10).
		Offset(0).
		All(ctx, &stats)
		
	if err != nil {
		fmt.Printf("Query failed: %v\n", err)
	}

	// 5. 更新操作 (SetMap / SetStruct)
	
	// 方式 A: 通过结构体更新 (自动推导表名)
	// 仅更新非零值字段 (因为定义了 omitempty)，且 WHERE 条件使用主键
	updateUser := User{ID: newID, Name: "John Updated"}
	e.Update("").
		SetStruct(&updateUser). 
		Where("id = ?", newID).
		Exec(ctx)

	// 方式 B: 通过 Map 或 Set 方法手动指定
	e.Update("users").
		SetMap(map[string]any{"status": 0}).
		Set("updated_at", time.Now()).
		Where("age < ?", 20).
		Exec(ctx)

	// 6. 事务处理
	err = e.Transaction(ctx, func(tx *engine.Tx) error {
		// 注意：事务内部必须使用 tx 对象，而不是 e
		
		// 6.1 删除操作
		if _, err := tx.DeleteFrom("users").Where("status = ?", 0).Exec(ctx); err != nil {
			return err // 返回 error 将触发 Rollback
		}
		
		// 6.2 插入日志
		if _, err := tx.InsertInto("logs").Columns("msg").Values("Cleanup done").Exec(ctx); err != nil {
			return err // 返回 error 将触发 Rollback
		}

		return nil // 返回 nil 将触发 Commit
	})
	
	if err != nil {
		fmt.Printf("Transaction failed: %v\n", err)
	}
}
```

## 进阶用法

### SQL 日志

通过 `WithConfig` 开启日志：

```go
engine.Open("mysql", dsn, engine.WithConfig(engine.Config{
    LogSQL:    true,
    SlowQuery: 200 * time.Millisecond,
}))
```

### 原生 SQL

对于复杂的查询，可以使用 `Raw` 子句，但请注意手动拼接字符串时可能存在的 SQL 注入风险。

```go
e.Select().
    WhereRaw("age > ? AND name LIKE ?", 18, "A%").
    All(ctx, &users)
```

## 许可证

MIT
