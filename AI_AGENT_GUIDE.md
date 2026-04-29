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
- MySQL INSERT IGNORE：`e.Insert("users").Columns("id","name").Values(1,"a").InsertIgnore().Exec(ctx)`
- 更新结构体：`e.Update("").Model(&obj).Where("id = ?", obj.ID).Exec(ctx)`
- 更新 map（单行）：`e.Update("users").Map(map[string]any{...}).Where("id = ?", 1).Exec(ctx)`
- 更新表达式列：`e.Update("users").SetExpr("updated_at", clause.Raw("NOW()")).Where("id = ?", 1).Exec(ctx)`
- 批量更新（结构体切片）：`e.Update("").Models([]User{...}).Exec(ctx)`（单条 SQL，CASE WHEN）
- 删除：`e.Delete("users").Where("id = ?", 1).Exec(ctx)`（默认禁止无 WHERE 全表删除）
- 计数：`count, err := e.Select().From("users").WhereEq("status", 1).Count(ctx)`
- 自定义计数：`count, err := e.Select().From("users").CountExpr(ctx, clause.Raw("COUNT(DISTINCT `email`)"))`
- 存在性检查：`exists, err := e.Select().From("users").WhereEq("id", 1).Exists(ctx)`
- 安全行迭代：`e.Select(...).From(...).QueryFunc(ctx, func(rows *sql.Rows) error { ... })`
- 现代流式迭代 (Go 1.23+)：`for u, err := range engine.Iter[User](ctx, e.Select().From("users")) { ... }`
- PostgreSQL RETURNING：`e.Update("users").Set("status", 1).Where("id = 1").Returning("id").QueryFunc(...)`
- Upsert (MySQL/PostgreSQL)：`e.Insert("users").Model(&u).OnConflict("id").DoUpdate(map[string]any{"name": u.Name}).Exec(ctx)`
- Raw Exec：`e.RawExec(ctx, "UPDATE users SET age = ?", 18)`
- 业务侧统一封装（推荐）：`qb := e.Builder()` 或 `qb := tx.Builder()`（预绑定 dialect + executor，便于在你的 DAO/Repository 层复用）
- 仅构建 SQL（driver 运行时确定）：`qb := builder.Dialect(driverName)` 或 `qb := builder.MustDialect(driverName)`
- 绑定 executor + dialect：`qb := builder.For(dialect.MustGet(driverName), exec)` 或 `qb := builder.MustFor(dialect.MustGet(driverName), exec)`
- 查询 API 状态：`qb.Dialect()` 获取绑定的方言，`qb.Err()` 获取存储的错误
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
- `SuffixRaw(sql, args...)` for database-specific suffix (e.g. upsert)

### 4.3 UPDATE

- Use `Update(table)`.
- `Set(col, val)` or `Map(map[string]any)`.
- `Model(interface{})` with `IncludeZero()`, `IncludePrimaryKey()` options.
- **Batch Update**: `Key("id").Models(slice)` or `Key("id").Maps(sliceOfMaps)`.
- **Batch Update + Where**: `Key("id").Maps(slice).Where("status = ?", 1)`.
- **Note**: Batch Update (using `Key`) is mutually exclusive with `Set/Map/Model` (single update). Do not mix them.
- `Limit(int)`: Adds a LIMIT clause. **Warning**: Only supported by MySQL dialect. Postgres does not support LIMIT on UPDATE/DELETE; using it will return an error.
- `Returning(cols...)`: Adds a RETURNING clause. **Warning**: Only supported by PostgreSQL dialect. MySQL does not support RETURNING; using it will return an error.
- `Increment(column, amount)`: Increments a numeric column by the given amount (e.g. `Increment("views", 1)`).
- `Decrement(column, amount)`: Decrements a numeric column by the given amount (e.g. `Decrement("stock", 1)`).
- Default requires WHERE; use `AllowEmptyWhere()` only when you really want to update all rows.

### 4.4 DELETE

- Use `Delete(table)`.
- `Limit(int)`: Adds a LIMIT clause. **Warning**: Only supported by MySQL dialect. Postgres does not support LIMIT on DELETE.
- `Returning(cols...)`: Adds a RETURNING clause. **Warning**: Only supported by PostgreSQL dialect. MySQL does not support RETURNING; using it will return an error.
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
- 当前版本：`v2.1.11` (Go 1.26.2)

### v2.1.11 变更摘要（第十二轮深度审计 — 交叉审计清理）

**死代码清理：**
- 移除 `builder/errors.go` 中未使用的 `errUnsupportedDialect` 变量（staticcheck U1000 确认）。

**代码风格与一致性：**
- 将 `errSQLTooLong` 哨兵错误从 `builder/arg_builder.go` 移至集中的 `builder/errors.go`，与 v2.1.10 错误统一化策略保持一致。
- 移除 `errConflictDoNothing` 错误消息末尾的句号，符合 Go 错误字符串惯例。

**审计摘要：**
- `go vet` 零告警，全部测试通过，`staticcheck` 零警告。
- 未发现死代码或未使用的导入。
- 覆盖率：总体 70.7%。

### v2.1.10 变更摘要（第九轮、第十轮 & 第十一轮深度审计）

**Bug 修复：**
- 修复 `assignInt64` 安全 bug：`uint`/`uint64` 类型未检查负值即进行转换，可能静默产生错误结果。已为无符号整数类型添加溢出检查。

**代码风格与一致性：**
- 统一 `UpdateBuilder.Limit()` 和 `DeleteBuilder.Limit()` 行为与 `SelectBuilder.Limit()` 一致：值 ≤ 0 表示"无限制"（省略 LIMIT 子句），而非静默接受负值。
- 简化 `batchUpdateBuilder.Models()` 中冗余的 nil 检查。
- 将 `builder` 和 `engine` 包中所有内联 `errors.New("corm: ...")` 替换为哨兵错误（sentinel errors），实现一致的、可比较的错误处理。
- 新增 `engine/errors.go`，集中定义哨兵错误（`errEngineNotInit`、`errContextCanceled`）。
- 将 `errors.New("corm: unsupported dialect: " + driverName)` 替换为 `fmt.Errorf`，使用正确的格式化方式。

**死代码清理：**
- 移除 `batchUpdateBuilder` 中未被调用的方法：`Columns()`、`IncludePrimaryKey()`、`IncludeAuto()`、`IncludeReadonly()`、`IncludeZero()`。

**架构重构：**
- 移除冗余包装函数，直接调用 `internal.NormalizeColumn`。
- 从 `ConflictBuilder.DoUpdate()` 提取共享的 `buildSetClause()` 和 `buildConflictPrefix()` 辅助函数，消除 PostgreSQL 和 MySQL 分支间约 40 行重复代码。

**安全与健壮性：**
- 事务管理中增加 savepoint 名称校验，防止 SQL 注入。
- 增强 `defaultArgFormatter`，在 SQL 日志中正确脱敏敏感类型。

**测试增强：**
- 新增 `In()`、`Like`、`Alias` 函数及 `defaultArgFormatter` 的全面测试。
- 新增 `SelectBuilder.All/One/Scalar/Count/Exists`、`InsertBuilder.One` 及 `Iter` 在 nil executor 下的错误路径测试。
- 更新 LIMIT 测试以匹配新语义。
- 覆盖率：总体 70.7%。

**审计摘要：**
- `go vet` 零告警，全部测试通过，`go test -race` 零竞态。
- 未发现死代码或未使用的导入。
- `modernize` 静态分析工具零告警。

### v2.1.9 变更摘要（第八轮深度审计）

**Bug 修复：**
- 修复 `batchUpdateBuilder.mapsInternal()` 列验证遗漏：当从 map key 推导列名时，位于 key column 之后的 key 未被 `quoteColumnStrict` 校验，存在未校验标识符绕过检查的潜在风险。现在先校验所有 key，再移除 key column，确保所有列都经过安全验证。

**架构重构：**
- 提取通用 SQL tokenizer（`tokenizeSQL`），消除 `countQuestionPlaceholders` 和 `rewritePlaceholders` 之间约 200 行重复的状态机解析逻辑，统一为声明式 token 遍历模式。
- 移除冗余包装函数 `normalizeInsertColumnKey`，直接调用 `internal.NormalizeColumn`。
- 优化 `quoteIdentWithStar` 中的特殊字符检测，使用 `[256]bool` 查找表替代内联多条件分支，提升标识符校验性能。
- 简化 `quoteColumnStrict`，移除与 `isSimpleIdent` 重复的特殊字符检测逻辑。

**审计摘要：**
- `go vet` 零告警，全部测试通过。
- 未发现死代码或未使用的导入。
- 未发现废弃 API 使用。
- 所有 `sync.Pool`、`sync.RWMutex` 模式验证正确。
- `modernize` 静态分析工具零告警。
- 测试覆盖率：总体 65.7%（internal 100%、dialect 97.2%、schema 89.0%、engine 83.2%、scan 76.4%、builder 58.5%、clause 77.6%）。

### v2.1.8 变更摘要（第六轮深度审计）

**LIMIT 语法审计与修复：**
- `SelectBuilder.Limit(0)` 和 `Offset(0)` 现在正确省略 LIMIT/OFFSET 子句（语义：无限制/无偏移），而非生成 `LIMIT 0`（返回 0 行）。
- 修复 `colsKey` 中 `cap` 变量与内置函数 `cap()` 的命名冲突，改为 `totalCap`。

**代码风格：**
- 无内置函数名冲突。

### v2.1.7 变更摘要（第五轮深度审计）

**Go 1.26 现代化续：**
- `clause.In()` 引入泛型 `flattenSlice[S ~[]E, E any]` 消除 8 个冗余类型分支。
- 消除 `In()` 函数过时性能注释。
- 现代化 `for i := 0; i < rv.Len(); i++` → `for i := range rv.Len()`（insert_batch.go、batch_update.go、schema.go、clause/expr.go）。
- `buildIn` 增加空切片防御。

**测试增强：**
- scan 包新增 `TestIterEmpty`、`TestIterEarlyExit`、`TestIterWrongMapKeyType`、`TestIterNonStructNonMap` 测试（覆盖率 75.1% → 76.4%）。
- schema 包覆盖率 89.0% → 90.4%。

**代码清理：**
- 移除 `In()` 函数中 70 行重复类型分支代码（8 → 11 行）。
- clause 包覆盖率 77.2% → 77.6%。

### v2.1.6 变更摘要（第四轮深度审计）

**Go 1.26 现代化：**
- 所有 benchmark 函数应用 `b.Loop()`。
- builder 包采用 `maps.Keys()` + `slices.Collect()` 提取 map 键。
- executor 格式化中采用 `for i, arg := range args` 遍历。
- 全部代码通过 `modernize` 静态分析。

**性能优化：**
- `wrapExecutor` 统一 executor 包装逻辑。
- `trimSpaceASCII` 提取复用。
- `colsKey` 缓冲区动态计算。

**测试增强：**
- scan 包新增未映射列、ScanOne 变体测试（覆盖率 60.2% → 75.1%）。
- builder 包新增 Union、子查询、JOIN、DISTINCT 等测试（覆盖率 54.8% → 58.6%）。
- engine 包覆盖率 31.1% → 83.2%，schema 包 71.8% → 89.0%。

### v2.1.5 变更摘要（Go 1.26 现代语法审计）

**Go 1.26 现代化：**
- 采用 `new(expr)` 表达式参数语法，替换 `intPtr` 辅助函数和 `&variable` 模式。
- 用 `strings.Join` 替代手动 `strings.Builder` 循环生成占位符（Go 1.26 中性能持平）。
- 应用现代 `for i := range n` 整数范围语法。
- 用 `reflect.TypeFor[string]()` 替代 `reflect.TypeOf("")`。
- 采用 `strings.Cut` 进行更简洁的字符串分割。
- 将 `for i := 0; i < len(x); i++` 循环现代化为 `for i := range x`。

**性能优化：**
- 用 `strings.IndexByte` 替代 `strings.Contains` 进行单字符搜索。
- 合并多个 `strings.Contains` 为单个 `strings.ContainsAny`。
- 优化 `TrimSpace/ToUpper` 执行顺序。
- 利用 Green Tea GC 加速小对象分配。

**代码质量：**
- 更新 `interface{}` → `any` 注释。
- 移除冗余 `intPtr` 辅助函数。
- 全部 8 个包通过 `go vet` 和 `go test`。

### v2.1.4 变更摘要（第三轮深度审计）

**代码健壮性增强：**
- `Engine` 方法（`Close/Stats/Ping/DB/Dialect`）增加 nil 保护，防止空指针解引用。
- 统一 `Returning()` 验证逻辑：Insert/Update/Delete Builder 均使用 `quoteColumnStrict` 进行列标识符校验，错误信息统一为 `"corm: invalid column identifier"`。
- `Returning` 子句 SQL 生成时增加校验，无效列标识符会返回错误而非静默输出空字符串。

**代码风格统一与重构：**
- 提取 `dialect.quoteCache` 共享结构体，消除 MySQL/PostgreSQL 方言中重复的缓存逻辑（`map[string]string` + `sync.RWMutex` + 计数器）。
- `quoteCache.Get/Set` 封装读写锁操作，减少代码重复约 40 行。
- 移除 `mysqlDialect`/`postgresDialect` 中的 `mu/cache/cacheLen` 字段和 `maxQuoteCacheSize`/`maxPgQuoteCacheSize` 常量，统一使用 `dialect.quoteCache`。

**性能基准测试：**
- 新增 `BenchmarkSelectBuild`、`BenchmarkInsertBuild`、`BenchmarkUpdateBuild`、`BenchmarkDeleteBuild` 覆盖核心 SQL 构建路径。
- 新增 `BenchmarkScanAllStruct`、`BenchmarkScanAllMap`、`BenchmarkIterStruct`、`BenchmarkIterMap` 覆盖扫描与迭代路径。
- dialect 包测试覆盖率提升至 97.2%。

**代码清理：**
- 修复 `scan.go`/`iter.go` 中多余的 `sql.RawBytes` type switch case。`sql.RawBytes` 是 `[]byte` 的类型别名，在 type switch 中 `case []byte:` 已覆盖所有情况，`case sql.RawBytes:` 为死代码，已移除。
- scan 包测试覆盖率提升至 75.1%。

### v2.1.3 变更摘要（第二轮深度审计）

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

### v2.1.2 变更摘要

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
- 新增 `UpdateBuilder.Increment()`/`Decrement()` 文档。

### v2.1.1 变更摘要

**性能优化：**
- `clause.In()` 处理 `[]any` 类型时消除不必要的切片拷贝。
- `colsKey` 缓冲区大小动态计算，减少内存重分配。
- 扫描逻辑使用每行独立的 dummy 变量，避免潜在竞态条件。
- `trimSpaceASCII` 提取为可复用函数，减少代码重复。
- `wrapExecutor` 统一 Engine 和 Tx 的 executor 包装逻辑。

**测试增强：**
- scan 包新增结构体未映射列、ScanOne 变体等测试（覆盖率 60.2% → 69.3%）。
- builder 包新增 Union、子查询、JOIN、DISTINCT、FOR UPDATE 等测试（覆盖率 54.8% → 55.1%）。
- schema 包测试覆盖率 71.8% → 73.2%。

### v2.1.0 (Latest Audit)

**Go 1.26.2 现代化:**
- 全面拥抱 Go 1.23+ 迭代器，支持 `engine.Iter[T]` 与 `builder.Iter[T]`。
- 彻底移除 `sync.Pool` 对象复用，转而信任现代 Go 运行时的逃逸分析与栈分配优化，提升稳定性并杜绝连接泄露风险。
- 优化核心 Scan 逻辑，减少反射调用深度。

**深度审计修复:**
- 修复 `schema` 解析中的潜在竞态条件。
- 统一所有内部缓存（Schema, SnakeCase, Dialect Quote, StructPlan）的 RWMutex 读写分离逻辑与驱逐策略。
- 增强 SQL 语句与表名长度限制的集中式校验。

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

### v2.0.1 更新内容

**新功能：**
- `InsertIgnore()`：MySQL 专用，生成 `INSERT IGNORE INTO ...`，跳过重复键错误行。
- `SetExpr(column, expr)`：更新时将列设置为原始 SQL 表达式（如 `NOW()`），而非绑定参数。
- `CountExpr(ctx, expr)`：自定义计数表达式（如 `COUNT(DISTINCT email)`）。
- `CountExprSQL(expr)`：构建计数 SQL 但不执行（便于调试/测试）。
- `QueryFunc(ctx, fn)`：安全行迭代，保证 `rows.Close()` 被调用。
- `clause.In()` / `clause.NotIn()` 新增 `[]uint32` 类型支持。

**Bug 修复：**
- `Count()` 在 `GROUP BY` 存在时自动包装子查询，返回正确的总组数。
- `Count()` 不再在简单计数查询中包含 `GROUP BY`/`HAVING`。

**性能优化：**
- `NormalizeColumn` 为纯 ASCII 列名添加快速路径，减少内存分配。

**测试覆盖：**
- 新增 SQL 注入防护测试（覆盖所有 builder 类型）。
- dialect 覆盖率：17.2% → 96.9%。
- clause 覆盖率：38.9% → 77.1%。
- builder 覆盖率：53.4% → 54.9%。

### v2.0.0 更新内容

**核心升级：**
- 内部完全迁移到 Go 1.26 标准（依赖 `slices`, `maps` 等），带来更出色的内联和内存管理。
- 移除了 builder 层的 `sync.Pool` 缓存对象复用机制。由于最新 Go 版本的底层栈分配优化，这减少了内存不可抗力下的泄露风险，并提升了常规执行流控制。

**功能补充 (DQL/DML)：**
- 添加 `Count()`、`Exists()` 的原生一键执行 API。
- 添加 `OnConflict().DoUpdate().DoNothing()` 支持更为明确的 Upsert 语句（自动处理 PostgreSQL / MySQL 方言）。
- 添加 `WhereBetween`, `WhereNotIn`, `WhereNotLike`, `WhereExists`, `WhereNotExists` 等常用扩展语法。
- Update 添加 `JoinAs`, `FromAs`。
- Select 增加 `ForShare`, `Intersect`, `Except` 支持。
- Engine / Tx 层面暴露直接底层 `RawExec`, `RawQuery`, `RawQueryFunc`。

**安全强化：**
- 对 Dialect 的 `quoteCache` 增加了强缓存上限机制（最大 10000 key）并配合原子操作定时驱逐，防御潜在内存耗尽攻击。
- 配置增加 `ConnMaxIdleTime` 限定连接安全存活时间。
- 对所有的 Select Exec 等加了空指针防护机制。

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
