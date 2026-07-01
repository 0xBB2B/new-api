---
name: test-helpers-mark-t-helper
description: 测试脚手架函数（签名含 *testing.T）首行必须 t.Helper()；命名带业务语义。
---

# 测试脚手架首行 t.Helper()

## 目的

让失败堆栈定位到调用者的行号，避免所有失败都指向 helper 内部同一行导致「一处失败、行号无用」。

## 逻辑

Go 的 `testing.T` 提供 `Helper()` 显式声明「本函数是测试脚手架，失败时报告我的调用者的行号」。约束要求：任何以 `*testing.T` 为首参、内部会调用 `require` / `DB.Create` / 其他 helper 的函数，起手一行必须是 `t.Helper()`。

命名上，helper 用领域语义（例如 `insertUserForPaymentGuardTest`、`seedFlowQuotaData`、`clearPreferredOwnerTables`），一眼看出「这是为哪类测试准备状态」，禁通用工具味的 `setup` / `helper1`。

纯计算类 helper（无 `*testing.T` 参数）不受此规则约束。

## 规则口径

主要针对涉及 DB seed / context 构造 / 全局状态清理的测试脚手架。纯 controller 或 service 层的一次性 setup 可直接内联，不强制抽 helper。

## 约束

- 签名形如 `func f(t *testing.T, ...)` 且内部会触发断言或 DB 操作的函数首行必须 `t.Helper()`。
- helper 名字需带业务语义（`forXxxTest` / `seedXxx` / `clearXxx`）；禁 `setup` / `helper` 这种无信息名。
- 纯计算类 helper（无 `t` 参数）不受此规则约束。

## 例子

- 输入：多个用例都要先在 `users` 表里插入一条 `{ id, group, quota }` 记录再跑。
- 过程：抽出 `insertUserForQuotaTest(t *testing.T, id int, group string, quota int)` 函数；首行 `t.Helper()`；内部 `require.NoError(t, DB.Create(...).Error)`。
- 预期结果：任何用例调用该 helper 时插入失败，报错行号指向调用点而非 helper 内部的 `require` 行。

## 验收

- [ ] 全仓 grep 签名含 `t *testing.T` 且内部调 `require` 的 helper，首行均为 `t.Helper()`。
- [ ] helper 命名均含业务语义（含 `for/seed/clear/insert` 之类领域动词）；无 `setup` / `helper1` 之类通用名。
- [ ] 故意让 helper 内的断言失败时，`go test` 输出的行号指向调用点。
