# corm AI/Agent Guide（面向自动编程工具）

本文件专门为外部 AI/AI Agent/自动编程工具提供：如何在 Go 项目中**正确、安全、稳定**地使用 `corm` 进行数据库访问与 SQL 构建。

如果你是人类开发者，也可以把它当作“最短正确路径”的参考。

---

## 1. TL;DR（最短正确路径）

- 初始化：`engine.Open(driver, dsn, engine.WithConfig(...))`
- 查询：`e.Select(...).From(...).Where("x = ?", v).Limit(...).Offset(...).All(ctx, &dest)`
- IN：`WhereIn("id", []int{1,2,3})` 或 `WhereExpr(clause.In("id", ids))`
- 插入结构体：`e.InsertInto("").Model(&obj).Exec(ctx)`（会从 `TableName()` 推导表名）
- 更新结构体：`e.Update("").SetStruct(&obj).Where("id = ?", obj.ID).Exec(ctx)`
- 事务：`e.Transaction(ctx, func(tx *engine.Tx) error { ... })`

---

## 2. 目录与职责（面向 AI 的模块地图）

- `engine/`：对外入口（连接、事务、配置、SQL 日志）
- `builder/`：链式 Query Builder（SELECT/INSERT/UPDATE/DELETE）与 SQL 生成
- `clause/`：可复用表达式（`And/Or/In/Raw`）
- `schema/`：结构体字段解析（`db` tag、`TableName()`、字段策略）
- `exec/`：结果集扫描（ScanAll/ScanOne）
- `dialect/`：方言（MySQL/PostgreSQL 占位符与标识符引用）

---

## 3. 强约束（AI 生成代码必须遵守）

### 3.1 永远不要把不可信输入拼进 SQL 字符串

✅ 正确：

```go
q := e.Select().From("users").Where("id = ?", userID)
```

❌ 错误（SQL 注入风险）：

```go
q := e.Select().From("users").Where("id = " + userInput)
```

`Where/Join/Having/OrderBy` 的字符串参数都应被视为“需要人工确认的危险入口”。除非值来自受信任的常量/白名单，否则必须使用占位符参数绑定。

### 3.2 不要把用户输入当作标识符（表名/列名）

表名/列名来自：
- 代码常量（推荐）
- 结构体 `TableName()`（推荐）
- 结构体 `db:"col"` tag（推荐）

### 3.3 PostgreSQL 的占位符规则

`corm` 在 PostgreSQL 下会生成 `$1,$2,...`；MySQL 下使用 `?`。
对于 `Where("x = ?", v)` 这种 Raw 片段，内部会做占位符重写。

---

## 4. 常用 API（按 SQL 原语对齐）

### 4.1 SELECT

```go
var users []User
err := e.Select("id", "name").
    From("users").
    Where("age > ?", 18).
    OrderByDesc("age").
    Limit(10).
    Offset(0).
    All(ctx, &users)
```

常用：
- `From(table)`
- `Where(sql, args...)`
- `WhereExpr(clause.Expr)`
- `WhereIn(column, values...)`
- `Join(joinSQL)`（需要自行写 `LEFT JOIN ... ON ...` 片段；不要拼接用户输入）
- `GroupBy(cols...)`
- `Having(sql, args...)`
- `OrderBy(column, "ASC|DESC")` / `OrderByAsc` / `OrderByDesc`
- `Limit(limit)` / `Offset(offset)` / `LimitOffset(limit, offset)`

### 4.2 INSERT

#### 结构体插入（推荐）

```go
type User struct {
    ID   int    `db:"id,pk"`
    Name string `db:"name"`
}
func (User) TableName() string { return "users" }

u := User{Name: "alice"}
_, err := e.InsertInto("").Model(&u).Exec(ctx)
```

说明：
- `InsertInto("")` 允许空表名，`Model` 会从 schema 推导 `TableName()`。
- 结构体字段 tag：
  - `db:"col"` 映射列名
  - `db:"-,..."` 忽略字段
  - `pk` 主键
  - `readonly` 只读字段（默认不写入）
  - `omitempty` 零值跳过（除非开启 IncludeZero）

字段策略开关（需要时才用）：
- `IncludePrimaryKey()`
- `IncludeAuto()`
- `IncludeReadonly()`
- `IncludeZero()`

#### Columns/Values（手工）

```go
_, err := e.InsertInto("users").
    Columns("name", "age").
    Values("bob", 20).
    Exec(ctx)
```

### 4.3 UPDATE

#### 结构体更新（推荐）

```go
u := User{ID: 1, Name: "alice"}
_, err := e.Update("").SetStruct(&u).
    Where("id = ?", u.ID).
    Exec(ctx)
```

说明：
- `Update("")` 允许空表名，`SetStruct` 会从 schema 推导 `TableName()`。
- 结构体字段策略同 INSERT（readonly/omitempty 等）。

#### 手工 SET

```go
_, err := e.Update("users").
    Set("name", "alice").
    Where("id = ?", 1).
    Exec(ctx)
```

### 4.4 DELETE

```go
_, err := e.DeleteFrom("users").
    Where("id = ?", 1).
    Exec(ctx)
```

---

## 5. 事务（AI 推荐用法）

```go
err := e.Transaction(ctx, func(tx *engine.Tx) error {
    if _, err := tx.InsertInto("users").Columns("name").Values("a").Exec(ctx); err != nil {
        return err
    }
    if _, err := tx.Update("users").Set("name", "b").Where("id = ?", 1).Exec(ctx); err != nil {
        return err
    }
    return nil
})
```

原则：
- 事务内使用 `tx`，不要混用 `e`。
- 返回 error 会触发 rollback；panic 也会 rollback 后继续 panic。

---

## 6. 扫描（ScanAll/ScanOne）能力边界

`All/One` 支持把结果扫描到：
- `[]Struct` / `[]*Struct`
- `[]map[string]T`（key 必须是 string）
- `Struct` / `*Struct`
- `map[string]T` / `*map[string]T`

列名匹配规则：按列名（忽略引用符与表前缀）匹配到 `db:"col"`（或默认 snake_case）。

---

## 7. AI 代码生成模板（可复制）

### 7.1 查询模板

```go
type Row struct {
    ID   int    `db:"id"`
    Name string `db:"name"`
}

func QueryRows(ctx context.Context, e *engine.Engine, minID int) ([]Row, error) {
    var out []Row
    err := e.Select("id", "name").
        From("users").
        Where("id >= ?", minID).
        OrderByAsc("id").
        Limit(100).
        All(ctx, &out)
    return out, err
}
```

### 7.2 写入模板

```go
type User struct {
    ID   int    `db:"id,pk"`
    Name string `db:"name,omitempty"`
}
func (User) TableName() string { return "users" }

func CreateUser(ctx context.Context, e *engine.Engine, name string) error {
    u := User{Name: name}
    _, err := e.InsertInto("").Model(&u).Exec(ctx)
    return err
}
```

---

## 8. 版本与兼容性提示

- Go 版本：见 [go.mod](file:///Users/macrochen/Codespace/AI/corm/go.mod)
- SQL 占位符与引用规则由方言决定：见 `dialect/`

