目标：用最新 Go（建议 go1.22+，以本机安装版本为准）实现一个轻量、易用、链式调用的 ORM/Query Builder，支持 MySQL 与 PostgreSQL；除两种驱动外不引入任何第三方库；覆盖 CRUD 主要场景；性能优先（少反射、可缓存、低分配）。

## 设计原则
- 只依赖标准库 + 驱动（database/sql、context、sync、reflect、bytes/strings等）。
- 以“Builder + 执行器”形式：构建 SQL + args，再交给 database/sql 执行。
- 始终参数化（避免拼接值），Where/Set/Values 等只拼接结构，参数走 args。
- 方言隔离：占位符（? vs $n）、标识符引用（`"`）、LIMIT/OFFSET 差异等。
- 性能：结构体 schema 缓存、SQL 构建用 bytes.Buffer、尽量避免每次反射。

## 目录结构（建议）
- corm.go：对外入口（Open、New、WithDB、Close）
- engine/：持有 *sql.DB、方言、配置
- dialect/：Dialect 接口 + mysql/postgres 实现
- builder/：链式构建器（Select/Insert/Update/Delete）
- clause/：可组合 SQL 片段（WHERE/ORDER/LIMIT等）
- schema/：结构体解析（tag、字段、主键、列名映射）+ 缓存
- exec/：Scan/Rows 映射（到 struct slice、map、单值）
- tx/：事务包装（Begin/Commit/Rollback，支持链式在事务里执行）

## 公开 API（链式风格）
- Builder 入口（两种都支持，贴近你的示例）：
  - Db.Select([]string{"id","name"}).From("users").Where("age > ?", 18).OrderBy("id","DESC").Limit(0,10)
  - 也提供更 Go 友好的变参：Db.Select("id","name").From("users")...
- CRUD 覆盖：
  - Select：Columns/From/Join/Where/GroupBy/Having/OrderBy/Limit/Offset
  - Insert：Into/Columns/Values(…)/ValueStruct(&obj)/Returning(仅PG可用，MySQL按不支持处理)
  - Update：Table/Set("a",1)/SetMap(map)/SetStruct(&obj)/Where
  - Delete：From/Where
- 执行与取数：
  - SQL() (string, []any)：导出 SQL 与参数，方便调试/复用
  - Exec(ctx) (sql.Result, error)
  - One(ctx, dest) / All(ctx, destSlice)
  - Scalar(ctx, &v)
- 事务：Db.BeginTx(ctx, opts) -> Tx（Tx 上同样支持 Select/Insert/Update/Delete）

## 方言（MySQL/PG）
- Dialect 接口建议包含：
  - Name() string
  - Placeholder(n int) string（MySQL: ?；PG: $1..$n）
  - QuoteIdent(s string) string（MySQL:`col`；PG:"col"）
  - BuildLimitOffset(limit, offset) string（处理 LIMIT/OFFSET 组合）
  - TableExistsSQL(table string) (string, []any)
  - (可选) ReturningSupport() bool
- Where/Set 里允许用户写 ? 占位，PG 会在最终编译阶段统一转换为 $n。

## Schema 与映射
- 结构体 tag 约定（仅标准库反射）：
  - `db:"col"` 指定列名；`db:"-"` 忽略
  - `table:"users"` 或实现 TableName() string 覆盖表名
  - 主键：`pk:"true"` 或默认字段名 id/ID（可配置）
- schema 缓存：sync.Map[key=reflect.Type] -> *Schema，避免重复解析。
- 插入/更新：支持 struct 与 map 两种输入；struct 仅导出字段参与。

## SQL 构建（Clause/Builder）
- 内部用 Clause 组合：SELECT/FROM/WHERE/ORDER/LIMIT/INSERT/UPDATE/DELETE。
- Builder 保存 clause 列表 + args，最终 Compile(dialect) -> (sql, args)。
- OrderBy：OrderBy("col","DESC")；也支持 OrderBy("col", corm.Desc)。
- Limit(m,n)：按你的示例实现为 offset=m, limit=n（并提供 Limit(n) / Offset(m) 便于使用）。

## 日志与配置（不引第三方）
- 提供可选 Logger 接口（默认无输出），避免运行期开销。
- 支持配置：MaxOpenConns、MaxIdleConns、ConnMaxLifetime、Context 超时等。

## 测试与验证策略（无外部依赖）
- 单元测试：仅测试 SQL 编译结果（MySQL/PG 占位符、引号、LIMIT/OFFSET、CRUD 语句），不需要真实数据库。
- 可选集成测试：通过环境变量 DSN 打开（用户自己提供 MySQL/PG），默认跳过。

## 交付物
- 可直接 `go get` 引用的模块结构与 go.mod。
- 完整的链式 CRUD API + Dialect(MySQL/PG) + SQL 编译与单元测试。
- README：快速上手示例（Select/Insert/Update/Delete/Tx）。

如果你确认该计划，我将开始在仓库中创建上述结构并实现核心功能与测试。