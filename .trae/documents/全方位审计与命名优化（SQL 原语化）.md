以下为本次审计结论（含最关键问题）与准备落地的修复/优化计划。将以“兼容优先、逐步弃用旧名”的方式推进，避免破坏现有调用方。

## 关键发现（必须修复）
### 1) `Limit(offset, limit)` 参数顺序与 SQL 原语冲突
- 当前 `SelectBuilder.Limit(offset, limit)` 的签名与 SQL `LIMIT <limit> OFFSET <offset>` 语义相反，极易引发线上分页逻辑错误；并且 README 示例写法也更像 `Limit(limit, offset)`，存在误导。
- 位置：`Limit(offset, limit)` 与 `LimitN(limit)` 在 [select.go](file:///Users/macrochen/Codespace/AI/corm/builder/select.go)；README 示例在 [README.md](file:///Users/macrochen/Codespace/AI/corm/README.md)。

## 命名审计结论（易歧义/不贴近 SQL 原语）
### 2) `LimitN` 命名不一致
- 单参 `LimitN` 与单参 `Offset` 不对称，且不符合 SQL 原语命名习惯。

### 3) `Where/Join/Having` 与 `WhereRaw/JoinRaw/HavingRaw` 的双轨容易困惑
- 目前 `Where` 本质就是 raw 条件字符串，`WhereRaw` 实际只是别名；`Join/JoinRaw` 同理。
- 这会让用户不清楚“哪个才是推荐写法”。

### 4) ORM 概念动词与 SQL 原语动词混用
- `InsertBuilder.Record` / `UpdateBuilder.SetModel` 更像 ORM 领域语义，容易与 `Values`/`Set` 混淆。

## 落地计划（将执行代码修改 + 测试 + 文档同步）
### 1) 彻底修复分页 API（最高优先级）
- 新增与 SQL 原语一致的方法：
  - `Limit(limit int)`（单参，生成 `LIMIT ?/$n`）
  - `Offset(offset int)`（保留）
  - `LimitOffset(limit, offset int)`（可选便捷）
- 将现有 `Limit(offset, limit)` 重命名为 `OffsetLimit(offset, limit)` 或 `Page(offset, limit)`（二选一），并在旧方法上加 `// Deprecated:` 注释。
- 更新所有 README/README_CN 示例与单元测试，确保示例与实际行为一致。

### 2) 统一链式命名，贴近 SQL 原语且避免歧义
- 将 `LimitN` 弃用，推荐使用新 `Limit(limit)`。
- 对 `WhereRaw/JoinRaw/HavingRaw` 做一致化：
  - 方案 A：保留 `Where/Join/Having` 作为主入口，`WhereRaw/JoinRaw/HavingRaw` 标记 Deprecated（减少选择成本）。
  - 并在 GoDoc 明确：`Where` 传入的就是“条件 SQL 片段 + 参数”。

### 3) 引入更贴近语义的结构体写入命名（兼容旧名）
- 为 INSERT：新增 `Model(dest any)`（或 `FromStruct`）作为 `Record` 的更直观别名，并标记 `Record` 弃用（或相反，取其一作为主入口，统一文档）。
- 为 UPDATE：新增 `SetStruct(dest any)` / `SetFromStruct(dest any)` 作为 `SetModel` 的更直观别名，并标记 `SetModel` 弃用。

### 4) 安全与可读性补强（不改变默认行为）
- 为 `OrderBy` 的 `dir` 已限制为 ASC/DESC；将补充 GoDoc 强调“列名与方向不要拼接用户输入”。
- 为 `Join`/`Where` 等 raw API 补充更醒目的 GoDoc 警示，并在文档中给出安全示例（参数绑定）与反例（拼接）。

### 5) 验证与交付
- 更新/新增单元测试覆盖：分页（limit/offset）、兼容旧 API、文档示例。
- 跑全量 `go test ./...`，并在最终输出中给出变更点与迁移对照表（旧名→新名）。