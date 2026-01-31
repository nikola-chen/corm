# corm AI/Agent Guide（面向自动编程工具）

本文件专门为外部 AI/AI Agent/自动编程工具提供：如何在 Go 项目中**正确、安全、稳定**地使用 `corm` 进行数据库访问与 SQL 构建。

如果你是人类开发者，也可以把它当作“最短正确路径”的参考。

---

## 1. TL;DR（最短正确路径）

- 初始化：`engine.Open(driver, dsn, engine.WithConfig(...))`
- 查询：`e.Select(...).From(...).Where("x = ?", v).Limit(...).Offset(...).All(ctx, &dest)`
- IN：`WhereIn("id", []int{1,2,3})` 或 `WhereExpr(clause.In("id", ids))`
- 插入结构体：`e.Insert("").Model(&obj).Exec(ctx)`（会从 `TableName()` 推导表名）
- 插入 map（单行）：`e.Insert("users").Map(map[string]any{...}).Exec(ctx)`
- 批量插入（结构体切片）：`e.Insert("").Models([]User{...}).Exec(ctx)`
- 批量插入（map 切片）：`e.Insert("users").Columns("a","b").Maps([]map[string]any{...}).Exec(ctx)`
- 插入并返回 ID：`id, err := e.Insert("").Model(&obj).ExecAndReturnID(ctx, "id")`
- 更新结构体：`e.Update("").Model(&obj).Where("id = ?", obj.ID).Exec(ctx)`
- 更新 map（单行）：`e.Update("users").Map(map[string]any{...}).Where("id = ?", 1).Exec(ctx)`
- 批量更新（结构体切片）：`e.Update("").Models([]User{...}).Exec(ctx)`（单条 SQL，CASE WHEN）
- 删除：`e.Delete("users").Where("id = ?", 1).Exec(ctx)`（默认禁止无 WHERE 全表删除）
- 业务侧统一封装（推荐）：`qb := e.Builder()` 或 `qb := tx.Builder()`（预绑定 dialect + executor，便于在你的 DAO/Repository 层复用）
- 仅构建 SQL（不执行）：`qb := builder.MySQL(); sql, args, err := qb.Update("users").Set("x", 1).Where("id = ?", 1).SQL()`（`Exec/Query` 需要 Executor）
- 仅构建 SQL（driver 运行时确定）：`qb := builder.Dialect(driverName)` 或 `qb := builder.MustDialect(driverName)`
- 绑定 executor + dialect（driver 运行时确定）：`qb := builder.For(driverName, exec)` 或 `qb := builder.MustFor(driverName, exec)`
- 事务：`e.Transaction(ctx, func(tx *engine.Tx) error { ... })`
- 嵌套事务（Savepoint）：`tx.Transaction(ctx, func(subTx *engine.Tx) error { ... })`

---

## 2. 目录与职责（面向 AI 的模块地图）

- `engine/`：对外入口（连接、事务、配置、SQL 日志、连接池监控）
- `builder/`：链式 Query Builder（SELECT/INSERT/UPDATE/DELETE）与 SQL 生成
- `clause/`：可复用表达式（`And/Or/In/Raw/Not/IsNull/IsNotNull`，以及聚合函数辅助）
- `schema/`：结构体字段解析（`db` tag、`TableName()`、字段策略）
- `scan/`：结果集扫描（ScanAll/ScanOne）
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

`Where/JoinRaw/Having/OrderByRaw/SuffixRaw` 以及 `clause.Raw(...)` 都应被视为“需要人工确认的危险入口”。除非值来自受信任的常量/白名单，否则必须使用占位符参数绑定。

### 3.2 不要把用户输入当作标识符（表名/列名）

表名/列名来自：

- 代码常量（推荐）
- 结构体 `TableName()`（推荐）
- 结构体 `db:"col"` tag（推荐）

推荐优先使用更“安全默认”的接口：`From/FromAs/OrderBy/WhereEq/WhereIn`（会校验并引用标识符），而不是把用户输入直接拼进 `Where/JoinRaw/OrderByRaw`。

### 3.3 PostgreSQL 的占位符规则

`corm` 在 PostgreSQL 下会输出 `$1,$2,...`；MySQL 下使用 `?`。
库内部在构建 SQL 时统一维护参数序号，因此 **子查询/UNION 等组合场景也能保持编号连续**。

注意：在 PostgreSQL 下，如果你使用 `Where/Join/Having/OrderByRaw` 等“字符串 SQL + args”接口，SQL 字符串里应使用 `?` 作为占位符；但请避免在同一段参数化 SQL 中使用 JSONB 的 `?/?|/?&` 操作符（会与占位符冲突）。如需该能力，优先使用 `jsonb_exists/jsonb_exists_any/jsonb_exists_all` 等函数写法。

### 3.4 日志与敏感信息

`Config.LogArgs` 会把参数值写入日志，可能泄露密码/Token/PII。生产环境建议关闭，必要时仅在短时间排障窗口开启，并确保日志系统具备脱敏与访问控制。
默认参数格式化对 `string` 做全量脱敏，并默认不展开 `error/fmt.Stringer` 内容；如自定义 `Config.ArgFormatter`，也必须保持脱敏策略（避免明文输出敏感字段）。
可通过 `Config.MaxLogSQLLen/MaxLogArgsItems/MaxLogArgsLen` 控制日志体积，避免超长 SQL/参数导致日志放大。

### 3.5 SQL 长度限制

`corm` 对生成的 SQL 语句长度有限制，默认最大长度为 1MB。如果生成的 SQL 超过此限制，会返回错误：

```
corm: SQL statement exceeds maximum length of 1MB
```

此限制是为了防止恶意或意外生成的超长 SQL 导致数据库拒绝或内存耗尽。对于正常业务场景，1MB 的限制已经足够。如果确实需要更长的 SQL，请考虑重构查询逻辑或分批执行。

### 3.6 表名长度限制

`corm` 对表名长度有限制，最大长度为 128 字符（与 SAVEPOINT 名称限制保持一致）。如果表名超过此限制，会返回错误：

```
corm: table name exceeds maximum length of 128 characters
```

此限制确保了标识符的合理性和数据库兼容性。

### 3.7 错误处理最佳实践

`corm` 内部返回的 error 可能被 `fmt.Errorf` 包装。
推荐使用 `errors.Is(err, sql.ErrNoRows)` 来判断是否未找到记录，而不是字符串匹配。
对于事务中的错误，务必直接返回 error 以触发 rollback，不要吞掉错误。

补充：

- `MustDialect/MustFor/MustGet` 会在 dialect 不支持时直接 panic；仅建议用于启动期/脚本场景，不建议在长期运行服务的请求路径中使用。

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

说明：

- `Select("col", "t.col", "*")` 的字符串列名仅允许"标识符/通配符"形式（会安全引用）；如需 `COUNT(*) AS cnt` 等表达式列，请使用 `SelectExpr(clause.Alias(clause.Count("id"), "cnt"))` 等显式声明。

聚合表达式示例：

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

常用：

- `From(table)`
- `FromAs(table, alias)`（安全别名）
- `SelectExpr(exprs...)`（选择表达式列；例如聚合/别名）
- `Where(sql, args...)`
- `WhereEq(column, value)`（仅允许标识符）
- `WhereExpr(clause.Expr)`
- `WhereIn(column, values...)`（仅允许标识符；会校验并引用）
- `JoinRaw(joinSQL, args...)`（原生 JOIN 片段；不要拼接用户输入）
- `Join/LeftJoin/RightJoin/InnerJoin/FullJoin(table, onExpr)`（结构化 JOIN + 参数绑定）
- `JoinAs/LeftJoinAs/RightJoinAs/InnerJoinAs/FullJoinAs(table, alias, onExpr)`（安全别名 + 参数绑定）
- `JoinSelectAs/LeftJoinSelectAs/... (sub, alias, onExpr)`（JOIN 子查询 + 参数绑定）
- `GroupBy(cols...)`
- `Having(sql, args...)`
- `OrderBy(column, "ASC|DESC")` / `OrderByAsc` / `OrderByDesc`
- `OrderByExpr(clause.Raw(sql, args...))`（复杂排序；支持参数绑定）
- `OrderByRaw(raw)`（危险入口；不要拼接用户输入）
- `Limit(limit)` / `Offset(offset)` / `LimitOffset(limit, offset)`

#### JOIN 示例

```go
// Correct: Structured API with parameter binding (Recommended)
e.Select("u.name").
    FromAs("users", "u").
    LeftJoinAs("orders", "o", clause.Raw("u.id = o.user_id")). // raw condition
    All(ctx, &results)

// Correct: Using raw JOIN string (Caller must ensure safety)
e.Select("u.name").
    FromAs("users", "u").
    JoinRaw("LEFT JOIN orders o ON u.id = o.user_id").
    All(ctx, &results)
```

### 4.2 INSERT

- Use `Insert(table)`.
- `Columns(...)` + `Values(...)` for standard inserts.
- `Map(map[string]any)` for map-based inserts.
- For high-throughput map inserts with predefined Columns(...), prefer `MapLowerKeys/MapsLowerKeys` when keys are already normalized to lower-case.
- `Model(interface{})` for struct-based inserts.
- `ExecAndReturnID(ctx, pkName)` for Postgres returning ID.
- `SuffixRaw(sql, args...)` for database-specific suffix (e.g. upsert).

### 4.3 UPDATE

- Use `Update(table)`.
- `Set(col, val)` or `Map(map[string]any)`.
- `Model(interface{})` with `IncludeZero()`, `IncludePrimaryKey()` options.
- **Batch Update**: `Key("id").Models(slice)` or `Key("id").Maps(sliceOfMaps)`.
- **Batch Update + Where**: `Key("id").Maps(slice).Where("status = ?", 1)`.
- **Note**: Batch Update (using `Key`) is mutually exclusive with `Set/Map/Model` (single update). Do not mix them.
- `Limit(int)`: Adds a LIMIT clause. **Warning**: Only supported by MySQL dialect. Postgres does not support LIMIT on UPDATE/DELETE; using it will return an error.
- Default requires WHERE; use `AllowEmptyWhere()` only when you really want to update all rows.

### 4.4 DELETE

- Use `Delete(table)`.
- `Limit(int)`: Adds a LIMIT clause. **Warning**: Only supported by MySQL dialect. Postgres does not support LIMIT on DELETE.
- Default requires WHERE; use `AllowEmptyWhere()` only when you really want to delete all rows.

---

## 5. 事务（AI 推荐用法）

```go
err := e.Transaction(ctx, func(tx *engine.Tx) error {
    if _, err := tx.Insert("users").Columns("name").Values("a").Exec(ctx); err != nil {
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

- `[]Struct` / `[]*Struct`（推荐，性能最佳，支持预计算缓存）
- `[]map[string]any`（便利，但内存分配略高）
- `Struct` / `*Struct`
- `map[string]any` / `*map[string]any`

列名匹配规则：按列名（忽略引用符与表前缀）匹配到 `db:"col"`（或默认 snake_case）。

**Strict Mode (严格模式)**:

- `scan.ScanOneStrict(rows, dest)` / `scan.ScanAllStrict(rows, dest)`
- 当查询结果中存在重复列（归一化后同名，如 `u.id` 和 `o.id`）时，严格模式会直接报错，防止静默覆盖导致的数据错误。

**预分配优化**:

- `scan.ScanAllCap(rows, dest, capacity)`: 如果已知大概行数，可传入 `capacity` 预分配切片容量，减少 `append` 时的扩容分配。

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
    _, err := e.Insert("").Model(&u).Exec(ctx)
    return err
}
```

### 7.3 使用 QueryFunc 处理大量数据（推荐）

当需要手动处理 `*sql.Rows` 时，使用 `QueryFunc` 可以确保资源被正确释放，避免连接泄漏。

```go
import "database/sql"

func ProcessLargeDataset(ctx context.Context, e *engine.Engine) error {
    return e.Select("id", "name", "email").
        From("users").
        Where("status = ?", "active").
        OrderByAsc("id").
        QueryFunc(ctx, func(rows *sql.Rows) error {
            for rows.Next() {
                var id int
                var name, email string
                if err := rows.Scan(&id, &name, &email); err != nil {
                    return err
                }
                // 处理每一行数据
                if err := processUser(id, name, email); err != nil {
                    return err
                }
            }
            return rows.Err()
        })
}
```

对比：不推荐的 `Query` 方式（容易忘记关闭 rows）

```go
// ❌ 不推荐：容易忘记 defer rows.Close()
rows, err := e.Select("*").From("users").Query(ctx)
if err != nil {
    return err
}
defer rows.Close() // 容易遗漏！
// ... 处理 rows
```

---

## 8. 最佳实践：连接池监控

`corm` 提供了连接池监控功能，可以通过 `Engine.Stats()` 方法获取连接池的统计信息，用于监控和诊断连接池状态。

```go
import "database/sql"

// 获取连接池统计信息
stats := e.Stats()

// stats 包含以下字段：
// - OpenConnections: 当前打开的连接数
// - InUse: 正在使用的连接数
// - Idle: 空闲连接数
// - WaitCount: 等待连接的总次数
// - WaitDuration: 等待连接的总时长
// - MaxIdleClosed: 因超过最大空闲连接数而关闭的连接数
// - MaxLifetimeClosed: 因超过最大生命周期而关闭的连接数

// 示例：监控连接池健康状态
func MonitorPoolHealth(db *engine.Engine) {
	stats := db.Stats()

	// 检查连接池是否接近饱和
	if stats.InUse >= stats.MaxOpenConns*9/10 {
		log.Printf("WARNING: Connection pool nearly full: %d/%d", stats.InUse, stats.MaxOpenConns)
	}

	// 检查是否有大量等待
	if stats.WaitCount > 1000 {
		log.Printf("WARNING: High connection wait count: %d", stats.WaitCount)
	}

	// 检查平均等待时间
	if stats.WaitCount > 0 {
		avgWait := stats.WaitDuration / time.Duration(stats.WaitCount)
		if avgWait > 100*time.Millisecond {
			log.Printf("WARNING: High average wait time: %v", avgWait)
		}
	}
}
```

建议在生产环境中定期调用 `Stats()` 方法，将连接池指标上报到监控系统（如 Prometheus、Datadog 等），以便及时发现连接池问题。

## 9. 最佳实践：Context 超时控制

强烈建议为所有数据库操作设置 Context 超时，防止长时间阻塞。

```go
func GetUserWithTimeout(db *engine.Engine, userID int) (*User, error) {
    // 建议：默认超时 3-5 秒，根据业务调整
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    var u User
    // 所有 All/One/Exec 方法都接受 context
    err := db.Select("*").From("users").Where("id = ?", userID).One(ctx, &u)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return nil, fmt.Errorf("query timeout: %w", err)
        }
        return nil, err
    }
    return &u, nil
}
```

## 10. 最佳实践：大批量数据分批处理

对于超过 1000 行的批量插入或更新，建议分批执行以避免 SQL 语句过长或数据库包大小限制。

```go
func BatchInsertUsers(ctx context.Context, db *engine.Engine, users []User) error {
    const batchSize = 1000
    for i := 0; i < len(users); i += batchSize {
        end := i + batchSize
        if end > len(users) {
            end = len(users)
        }
        chunk := users[i:end]
        if _, err := db.Insert("").Models(chunk).Exec(ctx); err != nil {
            return err
        }
    }
    return nil
}
```

## 11. 常用设计模式

### 11.1 Repository 模式

```go
type UserRepository struct {
    db *engine.Engine
}

func NewUserRepository(db *engine.Engine) *UserRepository {
    return &UserRepository{db: db}
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*User, error) {
    var u User
    err := r.db.Select().From("users").Where("id = ?", id).One(ctx, &u)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, nil
        }
        return nil, err
    }
    return &u, nil
}

func (r *UserRepository) ListByStatus(ctx context.Context, status int, limit int) ([]User, error) {
    var users []User
    err := r.db.Select().From("users").
        Where("status = ?", status).
        OrderByDesc("created_at").
        Limit(limit).
        All(ctx, &users)
    return users, err
}
```

### 11.2 事务中的多表操作

```go
func Transfer(ctx context.Context, db *engine.Engine, fromID, toID int64, amount float64) error {
    return db.Transaction(ctx, func(tx *engine.Tx) error {
        // 扣减转出账户
        _, err := tx.Update("accounts").
            Increment("balance", -amount).
            Where("id = ? AND balance >= ?", fromID, amount).
            Exec(ctx)
        if err != nil {
            return fmt.Errorf("deduct from account: %w", err)
        }

        // 增加转入账户
        _, err = tx.Update("accounts").
            Increment("balance", amount).
            Where("id = ?", toID).
            Exec(ctx)
        if err != nil {
            return fmt.Errorf("add to account: %w", err)
        }

        // 记录交易日志
        _, err = tx.Insert("transfers").
            Map(map[string]any{
                "from_id": fromID,
                "to_id":   toID,
                "amount":  amount,
                "created_at": time.Now(),
            }).
            Exec(ctx)
        if err != nil {
            return fmt.Errorf("record transfer: %w", err)
        }

        return nil
    })
}
```

### 11.3 乐观锁模式

```go
type Product struct {
    ID      int64 `db:"id,pk"`
    Name    string `db:"name"`
    Stock   int    `db:"stock"`
    Version int    `db:"version"`
}

func (r *ProductRepository) DecrementStock(ctx context.Context, productID int64, quantity int) error {
    result, err := r.db.Update("products").
        Set("stock", clause.Raw("stock - ?", quantity)).
        Increment("version", 1).
        Where("id = ? AND stock >= ?", productID, quantity).
        Exec(ctx)
    if err != nil {
        return err
    }

    rows, _ := result.RowsAffected()
    if rows == 0 {
        return errors.New("insufficient stock or product not found")
    }
    return nil
}
```

## 12. 版本与兼容性提示

- Go 版本：见 [go.mod](file:///Users/macrochen/Codespace/AI/corm/go.mod)
- SQL 占位符与引用规则由方言决定：见 `dialect/`
- 当前版本：`v1.2.2`

### v1.2.2 更新内容

**代码风格与文档：**

- 对所有 Go 源文件应用 `gofmt` 格式化，统一代码风格
- 移除 README 中重复的「查询缓存注意事项」章节
- 修复缓存章节中的不完整文档内容
- 所有测试通过竞态检测
- `go vet` 检查无警告

### v1.2.1 更新内容

**代码质量提升：**

- 全面代码审计，确保无代码错误、遗漏和安全隐患
- 优化代码扩展性和易用性
- 提高代码健壮性和复用性
- 统一代码风格和命名规范
- 优化链式调用 API，更贴近 SQL 原语
- 所有测试通过（包括竞态检测测试）
- 性能基准测试验证，内存分配优化效果显著

### v1.2.0 更新内容

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

### v1.1.3 更新内容

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
