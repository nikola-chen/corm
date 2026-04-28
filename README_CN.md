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

## 功能范围

`corm` 的功能范围仅限于提供 SQL 中的以下功能：
- **DQL** (数据查询语言)：如 `SELECT` 操作。
- **DML** (数据操纵语言)：如 `INSERT`、`UPDATE`、`DELETE` 操作。

它**不提供**以下功能：
- **DDL** (数据定义语言)：如创建/删除表、索引或修改表结构。
- **DCL** (数据控制语言)：如用户授权或撤销权限。

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
// 或：qb := builder.For(dialect.MustGet(driverName), db)   // 绑定 executor + dialect
// 或：qb := builder.MustFor(dialect.MustGet(driverName), db) // 不支持则直接 panic

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

### InsertIgnore（MySQL）

跳过会导致重复键错误的行：

```go
_, err := e.Insert("users").
    Columns("id", "name").
    Values(1, "Alice").
    InsertIgnore(). // 生成：INSERT IGNORE INTO ...
    Exec(ctx)
```

注意：`InsertIgnore` 仅支持 MySQL，在 PostgreSQL 上调用会返回错误。

### SetExpr（使用原始表达式更新）

将列设置为原始 SQL 表达式而非绑定参数：

```go
_, err := e.Update("users").
    SetExpr("updated_at", clause.Raw("NOW()")).
    Set("name", "Alice").
    WhereEq("id", 1).
    Exec(ctx)
// 生成：UPDATE `users` SET `updated_at` = NOW(), `name` = ? WHERE (`id` = ?)
```

### CountExpr（自定义计数表达式）

使用自定义表达式进行计数，如 `COUNT(DISTINCT column)`：

```go
count, err := e.Select().From("users").WhereEq("status", 1).
    CountExpr(ctx, clause.Raw("COUNT(DISTINCT `email`)"))
```

当查询包含 `GROUP BY` 时，`CountExpr` 会自动将查询包装在子查询中：

```go
// SELECT COUNT(*) FROM (SELECT ... FROM users GROUP BY status) AS sub
count, err := e.Select("status").From("users").GroupBy("status").
    Count(ctx)
```

// 即使 fn 发生 panic，rows.Close() 也会被自动调用
```

### Iter (Go 1.23+ 现代迭代器)

使用 Go 1.23+ 的 range-over-function 特性，以最优雅的方式处理查询结果。

```go
// SELECT id, name FROM users WHERE status = 1
query := e.Select("id", "name").From("users").WhereEq("status", 1)

// Iter 会在循环结束或提前中断时自动关闭 rows
for u, err := range engine.Iter[User](ctx, query) {
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(u.Name)
}
```

## 更新日志

### v2.1.7（第五轮深度审计）

**Go 1.26 现代化续：**
- `clause.In()` 引入泛型工具函数 `flattenSlice[S ~[]E, E any]`，消除 8 个冗余类型分支（约 70 行 → 11 行）。
- 消除 `In()` 函数中过时的性能优化注释。
- 现代化 `for i := 0; i < rv.Len(); i++` → `for i := range rv.Len()`（insert_batch.go、batch_update.go、schema.go、clause/expr.go）。
- `buildIn` 函数增加空切片防御检查。

**测试增强：**
- scan 包新增 `TestIterEmpty`、`TestIterEarlyExit`、`TestIterWrongMapKeyType`、`TestIterNonStructNonMap` 测试（覆盖率 75.1% → 76.4%）。
- schema 包覆盖率 89.0% → 90.4%，clause 包 77.2% → 77.6%。

**代码清理：**
- `In()` 函数中 8 个重复类型分支（`[]string`/`[]int`/`[]int64`/`[]uint64`/`[]int32`/`[]uint32`/`[]uint`）统一为泛型调用。

### v2.1.6（第四轮深度审计）

**Go 1.26 现代化：**
- 所有 benchmark 函数应用 `b.Loop()`，简化基准测试循环。
- builder 包采用 `maps.Keys()` + `slices.Collect()` 提取 map 键，代码更简洁。
- executor 格式化中将 `for i := 0; i < len(args); i++` 现代化为 `for i, arg := range args`。
- 全部代码通过 `modernize` 静态分析工具。

**性能优化：**
- 使用共享的 `wrapExecutor` 辅助函数统一 executor 包装逻辑。
- 提取 `trimSpaceASCII` 函数，在标识符引用中复用。
- 优化 `colsKey` 缓冲区大小计算，减少内存重分配。

**测试增强：**
- scan 包新增未映射列、ScanOne 变体、边界情况测试。
- builder 包新增 Union、子查询 FROM、JOIN 更新、USING 删除、DISTINCT、FOR UPDATE/FOR SHARE、HAVING 测试。
- 覆盖率提升：scan 60.2% → 75.1%，builder 54.8% → 58.6%，engine 31.1% → 83.2%，schema 71.8% → 89.0%。

### v2.1.5（Go 1.26 现代语法审计）

**Go 1.26 现代化：**
- 采用 `new(expr)` 表达式参数语法，替换 `intPtr` 辅助函数和 Limit/Offset Builder 中的 `&variable` 模式。
- 在 `clause.In()` 中用 `strings.Join` 替代手动 `strings.Builder` 循环生成占位符（Go 1.26 中性能持平）。
- 全代码库应用现代 `for i := range n` 整数范围语法。
- 用 `reflect.TypeFor[string]()` 替代 `reflect.TypeOf("")` 获取泛型类型。
- 采用 `strings.Cut` 进行更简洁的字符串分割。
- 将 `for i := 0; i < len(x); i++` 循环模式现代化为 `for i := range x`。

**性能优化：**
- 用 `strings.IndexByte` 替代 `strings.Contains(column, ".")` 进行单字符搜索。
- 将多个 `strings.Contains` 检查合并为单个 `strings.ContainsAny`。
- 优化 `TrimSpace/ToUpper` 执行顺序，减少处理字符数。
- 利用 Go 1.26 默认启用的 Green Tea GC 加速小对象分配。

**代码质量：**
- 将注释中的 `interface{}` 更新为 `any`。
- 移除冗余的 `intPtr` 辅助函数。
- 全部 8 个包通过 `go vet` 和 `go test`。

### v2.1.4（第三轮深度审计）

**健壮性增强：**
- `Engine` 方法（`Close/Stats/Ping/DB/Dialect`）增加 nil 保护，防止空指针解引用。
- 统一 `Returning()` 验证逻辑：Insert/Update/Delete Builder 均使用 `quoteColumnStrict` 进行列标识符校验，错误信息统一为 `"corm: invalid column identifier"`。
- `Returning` 子句 SQL 生成时增加校验，无效列标识符会返回错误而非静默输出空字符串。

**代码风格统一与重构：**
- 提取 `dialect.quoteCache` 共享结构体，消除 MySQL/PostgreSQL 方言中重复的缓存逻辑。
- `quoteCache.Get/Set` 封装读写锁操作，减少约 40 行代码重复。
- 移除 `mysqlDialect`/`postgresDialect` 中的 `mu/cache/cacheLen` 字段和 `maxQuoteCacheSize`/`maxPgQuoteCacheSize` 常量，统一使用 `dialect.quoteCache`。

**性能基准测试：**
- 新增 `BenchmarkSelectBuild`、`BenchmarkInsertBuild`、`BenchmarkUpdateBuild`、`BenchmarkDeleteBuild` 覆盖核心 SQL 构建路径。
- 新增 `BenchmarkScanAllStruct`、`BenchmarkScanAllMap`、`BenchmarkIterStruct`、`BenchmarkIterMap` 覆盖扫描与迭代路径。
- dialect 包测试覆盖率提升至 97.2%。

**代码清理：**
- 修复 `scan.go`/`iter.go` 中多余的 `sql.RawBytes` type switch case。`sql.RawBytes` 是 `[]byte` 的类型别名，在 type switch 中 `case []byte:` 已覆盖所有情况，`case sql.RawBytes:` 为死代码，已移除。
- scan 包测试覆盖率提升至 75.1%。

### v2.1.3（第二轮深度审计）

**测试覆盖增强：**
- engine 包测试覆盖率从 31.1% 大幅提升，新增全面的单元测试覆盖所有核心方法。
- scan 包新增 `ScanOneStrict`、指针切片分配、map 键类型验证等测试（覆盖率 69.3% → 77.5%）。
- schema 包新增错误处理、标签解析（auto/identity/autoincr/pk/readonly/omitEmpty）、默认主键检测、ColumnsAndValues 边界情况等测试（覆盖率 73.2% → 89.5%）。
- 创建独立的内存测试驱动，消除外部数据库依赖，提升测试可靠性。

**安全修复：**
- 修复 `Tx.Transaction` 中 SAVEPOINT 名称未经验证直接拼接的潜在 SQL 注入风险（虽然名称由内部序列生成，但增加防御性验证）。

**代码健壮性：**
- 所有包通过 `go vet` 静态检查无警告。
- 所有包通过 `go test -race` 竞态检测无数据竞争。
- 统一错误处理模式，生产代码无未处理错误。

### v2.1.2 (Bug 修复与性能优化)

**Bug 修复：**
- 修复 `scan.appendLowerASCII`/`writeLowerASCII` 在遇到非 ASCII 字符时未正确处理剩余子串的 bug。
- 修复 `Update`/`Delete` Builder 在 MySQL 方言下使用 `Returning()` 未返回错误的问题（MySQL 不支持 RETURNING）。
- 实现 `builder.For`/`MustFor`/`MustDialect` 函数（此前文档引用但代码缺失）。

**性能优化：**
- `NormalizeColumn` 使用 `[]byte` 替代 `strings.Builder`，减少内存分配。
- `builder/internal.go` 添加 `sync.Pool` 复用 `strings.Builder`，减少链式构建时的堆分配。
- 合并 `arg_builder.go` 中重复的 placeholder 重写逻辑为统一的 `rewritePlaceholders` 函数。

**API 增强：**
- 新增 `API.Dialect()` 访问器，返回绑定的方言。
- 新增 `API.Err()` 访问器，返回存储的错误。
- 新增 `UpdateBuilder.Returning()` 方言兼容检查（MySQL 下返回错误）。
- 新增 `DeleteBuilder.Returning()` 方言兼容检查（MySQL 下返回错误）。

### v2.1.1 (优化与测试增强)

**性能优化：**
- 优化 `clause.In()` 处理 `[]any` 类型时的性能，消除不必要的切片拷贝，减少内存分配。
- 改进 `colsKey` 缓冲区大小计算，减少结果扫描时的内存重分配。
- 增强扫描逻辑，将共享的 dummy 变量替换为每行独立分配，防止潜在的竞态条件。
- 提取可复用的 `trimSpaceASCII` 函数，减少标识符引用中的代码重复。
- 合并 `engine/executor.go` 中重复的 executor 包装逻辑，使用共享的 `wrapExecutor` 辅助函数。

**测试增强：**
- 添加 executor 包的全面测试（formatArgs、truncateSQL、loggingExecutor）。
- 添加 mock executor 测试以验证日志记录行为。
- 添加 scan 包测试，覆盖包含未映射列的结构体、ScanOne 变体和边缘情况。
- 添加 builder 包测试，覆盖 Union、UnionAll、子查询 FROM、INSERT FROM SELECT、JOIN 更新、USING 删除、DISTINCT、FOR UPDATE/FOR SHARE 和 HAVING 子句。
- 测试覆盖率提升：scan 60.2% → 69.3%，builder 54.8% → 55.1%，schema 71.8% → 73.2%。

### v2.1.0 (深度审计版本)

**Go 1.26.2 现代化适配:**
- 全面支持 Go 1.23+ 迭代器，提供 `engine.Iter[T]` 与 `builder.Iter[T]`。
- 彻底移除全库 `sync.Pool` 使用，转而利用现代 Go 栈分配优化及逃逸分析，提升极端压力下的稳定性。
- 优化核心 Scan 逻辑，通过减少反射中间层提升处理速度。

**架构与稳定性增强:**
- 修复 `schema` 解析在高并发下的竞态风险。
- 统一内部所有缓存（Schema, Dialect, SnakeCase, Plan）的读写锁模式与确定性驱逐策略。
- 强化 SQL 语句与标识符长度的集中校验逻辑。

### v2.0.2

**新功能：**
- 添加 `InsertIgnore()` 支持 MySQL 批量插入优化（生成 `INSERT IGNORE INTO ...`）。
- 添加 `SetExpr()` 支持使用原始 SQL 表达式更新列（如 `SET updated_at = NOW()`）。
- 添加 `CountExpr()` 支持自定义计数表达式，如 `COUNT(DISTINCT column)`。
- 添加 `QueryFunc()` 安全行迭代，保证 `rows.Close()` 清理。
- 添加 `[]uint32` 类型支持到 `clause.In()` 和 `clause.NotIn()`。

**Bug 修复：**
- 修复 `Count()` 方法在包含 `GROUP BY` 时结果不正确的问题，现在会自动包装为子查询。
- 修复 `Count()` 方法在简单计数查询中不再包含 `GROUP BY`/`HAVING`。

**性能优化：**
- 优化 `NormalizeColumn`，为纯 ASCII 列名添加快速路径，减少内存分配。

**测试：**
- 添加全面的 SQL 注入防护测试，覆盖所有 builder 类型。
- 添加 dialect 单元测试（覆盖率：17.2% → 96.9%）。
- 添加 clause 单元测试（覆盖率：38.9% → 77.1%）。
- 添加 builder 新功能单元测试（覆盖率：53.4% → 54.9%）。

### v2.0.2

**性能优化：**
- 将所有 `sync.Map` 缓存替换为 `sync.RWMutex` + 类型化 map，提升性能并增强类型安全：
  - `dialect/mysql.go` 和 `dialect/postgres.go`：QuoteIdent 缓存使用 RWMutex 与可控驱逐策略。
  - `schema/schema.go`：Schema 解析缓存和 ToSnake 缓存使用 RWMutex，提升并发读性能。
  - `scan/scan.go`：Struct plan 缓存使用 RWMutex，在高并发场景下查询更快。
- 缩小 API 暴露面，将 builder 工厂函数设为私有（`newSelectBuilder`、`newInsertBuilder` 等），强制使用 `builder.NewAPI()` 或 engine/transaction 方法。

**代码结构改进：**
- 增强 builder 包的封装性，减少公开 API。
- 统一所有包的缓存驱逐策略（超过阈值时全量清空）。
- 移除缓存重构后未使用的 `sync/atomic` 导入。

**测试：**
- 补充 schema 包测试（缓存、ColumnsAndValues、边界情况）。
- 补充 scan 包测试（nil dest、非 slice、缓存验证）。
- 所有包均具备针对错误路径和边界情况的健壮测试覆盖。

### v2.0.0

**核心升级：**
- 内部完全迁移到 Go 1.26 标准（依赖 `slices`, `maps` 等），带来更出色的内联和内存管理。
- 移除了 builder 层的 `sync.Pool` 缓存对象复用机制。由于最新 Go 版本的底层栈分配优化，这减少了内存不可抗力下的泄露风险，并提升了常规执行流控制。

**功能补充 (DQL/DML)：**
- 添加 `Count()`、`Exists()` 的原生一键执行 API。
- 添加 `OnConflict().DoUpdate().DoNothing()` 支持更为明确的 Upsert 语句（自动处理 PostgreSQL / MySQL 方言）。
- 添加 `WhereBetween`, `WhereNotIn`, `WhereNotLike`, `WhereExists`, `WhereNotExists` 等常用扩展语法。
- Update 添加 `JoinAs`, `FromAs`, `Returning`。
- Select 增加 `ForShare`, `Intersect`, `Except` 支持。
- Delete 增加 `Using`, `UsingAs`, `Returning`。
- Engine / Tx 层面暴露直接底层 `RawExec`, `RawQuery`, `RawQueryFunc`。

**安全强化：**
- 对 Dialect 的 `quoteCache`, `pgQuoteCache` 和 `snakeCache` 增加了强缓存上限机制（最大 10000 key）并配合原子操作定时驱逐，防御潜在内存耗尽攻击。
- 配置增加 `ConnMaxIdleTime` 限定连接安全存活时间。
- 对所有的 Select Exec 等加了空指针防护机制。

### v1.2.2

**代码风格与文档：**

- 对所有 Go 源文件应用 `gofmt` 格式化，统一代码风格
- 移除 README 中重复的「查询缓存注意事项」章节
- 修复缓存章节中的不完整文档内容
- 所有测试通过竞态检测（`go test -race ./...`）
- `go vet` 检查无警告

### v1.2.1

**代码质量提升：**

- 全面代码审计，确保无代码错误、遗漏和安全隐患
- 优化代码扩展性和易用性
- 提高代码健壮性和复用性
- 统一代码风格和命名规范
- 优化链式调用 API，更贴近 SQL 原语
- 所有测试通过（包括竞态检测测试）
- 性能基准测试验证，内存分配优化效果显著

### v1.2.0

**安全修复：**

- 修复 SAVEPOINT 名称验证，防止潜在的 SQL 注入风险
- 加强 HAVING 子句空表达式检查，返回明确错误而非静默跳过
- 添加 SQL 语句长度限制（1MB），防止超长 SQL 导致数据库拒绝或内存耗尽
- 添加表名长度限制（128 字符），与 SAVEPOINT 名称限制保持一致

**性能优化：**

- 抽取 `NormalizeColumn` 到 `internal` 包，消除代码重复
- 使用 `sync.Pool` 优化内存分配（ToSnake, colsKey, argBuilder, whereBuilder）
- 预分配 argBuilder args 切片，减少扩容开销
- 添加 QuoteIdent 缓存（MySQL/PostgreSQL），减少重复标识符引用的内存分配
- 添加 ToSnake 缓存，减少重复 snake_case 转换的内存分配

**API 改进：**

- 增强错误信息，提供更明确的调试指引
- 优化链式调用 API，更贴近 SQL 原语
- 添加 `Engine.Stats()` 方法，提供连接池监控功能
- 修复 SelectBuilder、UpdateBuilder、DeleteBuilder 中的 nil slice bug

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
