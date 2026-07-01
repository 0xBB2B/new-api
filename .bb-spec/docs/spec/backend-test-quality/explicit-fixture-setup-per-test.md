---
name: explicit-fixture-setup-per-test
description: 依赖 DB/context/settings/cache 的测试必须在用例内部显式初始化，禁靠前一个用例残留。
---

# 测试 fixture 显式初始化

## 目的

让每个测试自成一体、可乱序运行、可单独 `-run`，避免因执行顺序或全局状态污染出现偶发失败。

## 逻辑

共享外部状态的测试遵循「清 → 播 → 跑 → 断言」四步：

1. 起手清空相关状态：truncate 相关表、reset 缓存、reset 全局设置。
2. 播种本用例所需的 seed 数据。
3. 构造上下文：新建 `gin.CreateTestContext`，显式设置 `user_group` / `token_group` 等 context key。
4. 调用被测函数并断言。

清理与播种通常抽成带 `t.Helper()` 的辅助函数，命名带业务语义（例如 `seedFlowQuotaData`、`clearPreferredOwnerTables`），失败行号仍指向调用点。

禁靠上一个 `Test*` 留下来的行做假设。

## 约束

- 涉及 DB 的测试起手必须清空相关表（例如 `truncateTables(t, ...)` 或 `DELETE FROM ...`）。
- 涉及 `gin.Context` 的测试必须新建 `gin.CreateTestContext` 并显式设置所需 context key。
- seed / insert / setup 辅助函数（签名含 `*testing.T`）必须首行调用 `t.Helper()`。
- 测试内禁读全局单例状态而不先重置。

## 例子

- 输入：被测函数从 DB 查询「某用户在某分组下的可用配额」，依赖两张表和 context 里的 `user_group` 键。
- 过程：起手 `truncateTables(t, "users", "quotas")`；`seedUser(t, id=101, quota=1000)` + `seedQuota(t, ...)`；`ctx := gin.CreateTestContext(...)` 并 `SetContextKey(ctx, UserGroup, "vip")`；再调用被测函数。
- 预期结果：测试单独 `-run` 或与任意其他测试同批跑均得到同一结果；无隐式顺序依赖。

## 验收

- [ ] DB 测试起手包含 truncate/清空调用。
- [ ] context 测试均新建 `gin.CreateTestContext`，不复用全局。
- [ ] seed/setup helper 首行有 `t.Helper()`。
- [ ] 打乱 `go test` 顺序（`-shuffle=on`）连续跑 3 次全部通过。
