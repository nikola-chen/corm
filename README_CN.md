# corm - 轻量级 Go ORM

`corm` 是一个轻量级且易于使用的 Go 语言 ORM 库。它支持 MySQL 和 PostgreSQL，提供流畅的查询构建器、结构体映射和事务管理功能。

并发说明：
- `Engine` 可以在多个 goroutine 间安全共享。
- 链式 Query Builder（例如 `e.Select(...).Where(...)`）是可变对象，禁止跨 goroutine 共享复用。

## 特性

- **流畅的查询构建器**：提供直观的 API 用于构建 SELECT、INSERT、UPDATE 和 DELETE 查询。
- **结构体映射**：自动将数据库行映射到结构体（以及结构体切片）。
- **事务支持**：提供基于闭包的 `Transaction` 辅助方法，简化事务管理。
- **跨数据库支持**：支持 MySQL 和 PostgreSQL（通过方言抽象）。
- **安全与防护**：内置 SQL 注入防护（参数绑定）与标识符安全引用。
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
	"log"
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
		log.Fatalf("open db: %v", err)
	}
	defer e.Close()

	// 验证连接
	ctx := context.Background()
	if err := e.Ping(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	// 可选：如果你想在自己的封装层里复用 builder 风格写法，可以把 dialect + executor 预绑定一次：
	qb := e.Builder()
	var rows []map[string]any
	if err := qb.Select("id").From("users").Limit(1).All(ctx, &rows); err != nil {
		log.Fatalf("select: %v", err)
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
_, err := e.Insert("users").
	Model(&user).
	Exec(ctx)

// 插入指定列
_, err := e.Insert("users").
	Columns("name", "age").
	Values("Bob", 25).
	Exec(ctx)

// 使用 map 插入（map[string]any）
// 键按字母顺序排序，确保 SQL 确定性
_, err := e.Insert("users").
	Map(map[string]any{"name": "Carol", "age": 20}).
	Exec(ctx)

// 高吞吐插入（已 Columns(...) 且 map key 已统一为小写时）：
// 推荐 MapsLowerKeys 以减少每行 key 归一化开销
rows := []map[string]any{
	{"name": "Alice", "age": 25},
	{"name": "Bob", "age": 28},
}
_, err = e.Insert("users").
	Columns("name", "age").
	MapsLowerKeys(rows).
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

// 使用 WhereMap 查询 (键按字母序排序，自动 AND 连接)
err = e.Select("id", "name").
	From("users").
	WhereMap(map[string]any{
		"age": 18,
		"status": "active",
	}).
	All(ctx, &users)

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
	Model(&u).
	Where("id = ?", u.ID).
	Exec(ctx)

// 使用 Set 设置单个字段
_, err := e.Update("users").
	Set("age", 30).
	Where("id = ?", 1).
	Exec(ctx)

// 使用 Map 设置多个字段 (键按字母序排序)
_, err := e.Update("users").
	Map(map[string]any{
		"age": 31,
		"status": "active",
	}).
	Where("id = ?", 1).
	Exec(ctx)

// 批量更新（单条 SQL，CASE WHEN）
batch := []User{
    {ID: 1, Name: "Alice", Age: 25},
    {ID: 2, Name: "Bob", Age: 28},
}
_, err = e.Update("").Models(batch).Exec(ctx)
```

安全提示：
- `Update(table)` 默认要求 WHERE 非空（防止误更新整表）。
- 如果你确实要更新全表数据，需要显式调用 `AllowEmptyWhere()`。

#### 删除 (Delete)

```go
_, err := e.Delete("users").
	Where("id = ?", 1).
	Exec(ctx)
```

安全提示：
- `Delete(table)` 默认要求 WHERE 非空（防止误删整表）。
- 如果你确实要删除全表数据，需要显式调用 `AllowEmptyWhere()`：

```go
_, err := e.Delete("users").AllowEmptyWhere().Exec(ctx)
```

### 事务管理

`corm` 提供了一个便捷的 `Transaction` 方法，它会在函数成功执行后自动提交，若发生错误或 panic 则自动回滚。

```go
err := e.Transaction(ctx, func(tx *engine.Tx) error {
	// 在事务内部的操作请使用 'tx' 而不是 'e'
	if _, err := tx.Insert("users").Values("Dave", 40).Exec(ctx); err != nil {
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
	"log"
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
		log.Fatalf("open db: %v", err)
	}
	defer e.Close()

	ctx := context.Background()

	// 2. 插入 (Model) 与 返回值 (Returning - Postgres)
	newUser := User{Name: "John Doe", Email: "john@example.com", Age: 30, Status: 1}
	newID, err := e.Insert("").Model(&newUser).ExecAndReturnID(ctx, "id")
	if err != nil {
		log.Fatalf("insert user: %v", err)
	}

	// 3. 批量插入 (Values)
	e.Insert("users").
		Columns("name", "email", "age", "status").
		Values("Alice", "alice@test.com", 25, 1).
		Values("Bob", "bob@test.com", 28, 0).
		Exec(ctx)

	// 3.1 批量插入（结构体切片）
	users := []User{
		{Name: "Alice", Email: "alice@test.com", Age: 25, Status: 1},
		{Name: "Bob", Email: "bob@test.com", Age: 28, Status: 0},
	}
	e.Insert("").Models(users).Exec(ctx)

	// 3.2 批量插入（map 切片）
	rows := []map[string]any{
		{"name": "Alice", "email": "alice@test.com", "age": 25, "status": 1},
		{"name": "Bob", "email": "bob@test.com", "age": 28, "status": 0},
	}
	e.Insert("users").Columns("name", "email", "age", "status").Maps(rows).Exec(ctx)

	// 4. 复杂查询构建
	// 目标 SQL:
	// SELECT u.id, u.name, count(o.id) as order_count 
	// FROM users AS u 
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
	err = e.Select("u.id", "u.name").
		SelectExpr(clause.Raw("count(o.id) as order_count")).
		FromAs("users", "u").
		LeftJoinAs("orders", "o", clause.Raw("o.user_id = u.id")).
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

	// 5. 更新操作 (Map / Model)
	
	// 方式 A: 通过结构体更新 (自动推导表名)
	// 仅更新非零值字段 (因为定义了 omitempty)，且 WHERE 条件使用主键
	updateUser := User{ID: newID, Name: "John Updated"}
	e.Update("").
		Model(&updateUser). 
		Where("id = ?", newID).
		Exec(ctx)

	// 方式 B: 通过 Map 或 Set 方法手动指定
	e.Update("users").
		Map(map[string]any{"status": 0}).
		Set("updated_at", time.Now()).
		Where("age < ?", 20).
		Exec(ctx)

	// 方式 C: 批量更新 Maps
	updateRows := []map[string]any{
		{"id": 1, "status": 1, "age": 26},
		{"id": 2, "status": 0, "age": 29},
	}
	// 基于 'id' 生成 CASE-WHEN 批量更新语句
	e.Update("users").Key("id").Maps(updateRows).Exec(ctx)

	// 方式 D: 批量更新 Maps 叠加 Where 条件
	// 结果 SQL: UPDATE ... WHERE id IN (...) AND status = 1
	e.Update("users").
		Key("id").
		Maps(updateRows).
		Where("status = ?", 1).
		Exec(ctx)

	// 5. 更新带 Limit (仅 MySQL)
	_, err = e.Update("users").
		Set("status", 0).
		Where("age < ?", 18).
		Limit(100). // 限制影响行数
		Exec(ctx)

	// 6. 事务处理
	err = e.Transaction(ctx, func(tx *engine.Tx) error {
		// 注意：事务内部必须使用 tx 对象，而不是 e
		
		// 6.1 删除操作
		if _, err := tx.Delete("users").Where("status = ?", 0).Exec(ctx); err != nil {
			return err // 返回 error 将触发 Rollback
		}
		
		// 6.2 插入日志
		if _, err := tx.Insert("logs").Columns("msg").Values("Cleanup done").Exec(ctx); err != nil {
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
     LogArgs:   true, // 开启参数日志（默认脱敏，防止敏感信息泄露）
     SlowQuery: 200 * time.Millisecond,
 }))
 ```

### 原生 SQL

对于复杂的查询，可以使用 `Raw` 子句，但请注意手动拼接字符串时可能存在的 SQL 注入风险。

```go
e.Select().
    Where("age > ? AND name LIKE ?", 18, "A%").
    All(ctx, &users)
```

安全提示：
- 这些接口都应视为“危险入口”（除非 SQL 来自受信任常量/白名单）：`Where`、`JoinRaw`、`Having`、`OrderByRaw`、`SuffixRaw`、`clause.Raw`
- 尽量优先使用结构化/安全默认的接口：`WhereEq`、`WhereIn`、`OrderByAsc/Desc`、`Join/JoinAs`

注意（PostgreSQL）：
- 当使用“字符串 SQL + args”的片段（例如 `Where("x = ?", v)`）时，片段内占位符统一使用 `?`。
- 避免在同一段参数化 SQL 片段中混用 JSONB 的 `?/?|/?&` 操作符与 `?` 占位符；优先使用 `jsonb_exists/jsonb_exists_any/jsonb_exists_all` 函数写法。

### 仅构建 SQL（不执行）

如果你只需要生成 SQL 字符串而不执行（例如用于测试、日志、或交给其它库执行），可以直接使用 `builder` 包的 `API` 辅助对象来预绑定方言，从而避免反复传入 `nil, d`。

```go
import "github.com/nikola-chen/corm/builder"

qb := builder.MySQL() // 或 builder.Postgres()
// 或：qb := builder.Dialect(driverName)       // 直到 SQL()/Exec()/Query() 才返回该错误
// 或：qb := builder.MustDialect(driverName)   // 不支持则直接 panic（仅建议启动期/脚本，避免在请求路径使用）
// 或：qb := builder.MustFor(driverName, db)   // 不支持则直接 panic（仅建议启动期/脚本，避免在请求路径使用）

sqlStr, args, err := qb.Update("users").
    Set("name", "New Name").
    Where("id = ?", 1).
    SQL()

sqlStr, args, err = qb.Select("id", "name").
    From("users").
    Where("age > ?", 18).
    SQL()
```

## 高级特性

`corm` 也支持一系列更高级的 SQL 能力（逻辑表达式、JOIN、子查询、聚合、UNION、DISTINCT 等）。

安全提示：
- `clause.Raw(...)`、`JoinRaw(...)`、`OrderByRaw(...)`、`SuffixRaw(...)` 接受原生 SQL，禁止拼接任何不可信用户输入。

### 逻辑运算符

```go
import "github.com/nikola-chen/corm/clause"

e.Select().From("users").
    WhereExpr(clause.Not(clause.Raw("age < ?", 18))).
    WhereExpr(clause.IsNull("deleted_at")).
    WhereExpr(clause.IsNotNull("email")).
    All(ctx, &users)
```

### JOIN
支持结构化 JOIN（`Join/LeftJoin/RightJoin/InnerJoin/FullJoin/CrossJoin`）以及原生 JOIN（`JoinRaw`）。
推荐用法（带参数绑定，使用 `FromAs` + `*JoinAs`）：

```go
import "github.com/nikola-chen/corm/clause"

e.Select("u.name").
    FromAs("users", "u").
    LeftJoinAs("orders", "o", clause.And(
        clause.Raw("u.id = o.user_id"),
        clause.Eq("o.status", "active"), // 自动绑定: "active"
    )).
    All(ctx, &results)
```

### 嵌套事务 (Nested Transactions)

`corm` 支持基于 `SAVEPOINT` 的嵌套事务。您可以在事务块内部调用 `tx.Transaction`。

```go
import (
    "errors"

    "github.com/nikola-chen/corm/engine"
)

err := e.Transaction(ctx, func(tx *engine.Tx) error {
    if _, err := tx.Insert("logs").Values("Start").Exec(ctx); err != nil {
        return err
    }

    _ = tx.Transaction(ctx, func(subTx *engine.Tx) error {
        if _, err := subTx.Insert("users").Values("New User").Exec(ctx); err != nil {
            return err
        }
        return errors.New("oops")
    })

    return nil
})
```

### 子查询

**FROM 子查询（Nested SELECT in FROM）：**

```go
sub := e.Select("id", "name").From("users").Where("age > ?", 18)

e.Select("u.name").
    FromSelect(sub, "u"). // SELECT ... FROM (SELECT ...) AS u
    All(ctx, &results)
```

**WHERE 子查询（Subquery in WHERE）：**

```go
sub := e.Select("id").From("banned_users")

e.Select().From("users").
    WhereInSubquery("id", sub). // WHERE id IN (SELECT id FROM banned_users)
    All(ctx, &users)
```

**INSERT INTO ... SELECT：**

```go
sub := e.Select("id", "name").From("old_users")

e.Insert("new_users").
    Columns("id", "name").
    FromSelect(sub).
    Exec(ctx)
```

### 聚合函数

提供 `Count`, `Sum`, `Avg`, `Max`, `Min` 等辅助函数（可配合别名映射到结构体字段）。

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

## 更新日志

### v1.1.3

**安全修复：**
- 修复 SAVEPOINT 名称验证，防止潜在的 SQL 注入风险
- 加强 HAVING 子句空表达式检查，返回明确错误而非静默跳过

**性能优化：**
- 抽取 `NormalizeColumn` 到 `internal` 包，消除代码重复
- 使用 `sync.Pool` 优化内存分配（ToSnake, colsKey）
- 预分配 argBuilder args 切片，减少扩容开销

**API 改进：**
- 增强错误信息，提供更明确的调试指引
- 优化链式调用 API，更贴近 SQL 原语

### v1.1.2

- 重构占位符重写函数，消除重复代码
- 统一列名归一化函数
- 添加完善的文档和 AI Agent 指南

## 许可证

MIT
