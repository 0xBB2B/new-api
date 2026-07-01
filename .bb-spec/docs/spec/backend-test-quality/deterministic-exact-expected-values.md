---
name: deterministic-exact-expected-values
description: 断言必须给出确定的具体期望值；禁「不为空/长度 > 0/包含任意错误」等模糊断言。
---

# 确定性精确期望值

## 目的

让测试在被测行为发生任何观察差异时都失败，避免绿灯下代码悄悄漂移。

## 逻辑

模糊断言（`NotNil`、`Len > 0`、`Err != nil`）只能证明「有东西」，不能证明「东西对」。约束要求：

- 值断言必须给期望值（例如 `require.Equal(t, 具体值, got)`）。
- 结构存在性用精确 key 存在或字段等价（例如 `gjson.GetBytes(...).Exists()`、struct 相等）。
- 错误断言用 `assert.ErrorIs` 到具体 sentinel，或 `assert.Contains` 到具体错误短语。

当输入是复合结构（JSON、DB 行、HTTP 响应）时，把关心的每个字段一条条断言到底，而不是断「不为空」。

**允许模糊断言的唯一场景**：验证「存在这个字段」本身就是契约（例如零值保留：字段必须出现在编码结果里），此时用精确 key 存在性断言，并在同一测试里配合值断言收尾。

## 约束

- 值断言必须写明期望值：`require.Equal(t, 具体值, got)` / `assert.Equal(t, 具体值, got)`。
- 错误断言必须绑定具体错误：`assert.Contains(err.Error(), "具体短语")` 或 `errors.Is` 到具体 sentinel。
- 禁把 `require.NotEmpty` / `require.Len(x, >0)` 当值断言用。
- 结构存在性断言（例如精确 key `Exists()`、字段 `nil` / 非 `nil`）本身即契约时允许，但同测试内需配合值断言收尾。

## 例子

- 输入：被测函数把一个 struct 编码成 JSON，字段 `max_tokens=0` 必须保留（不被 `omitempty` 吃掉）。
- 过程：`require.NoError(t, err)` 拿到 `encoded []byte`；`require.True(t, gjson.GetBytes(encoded, "max_tokens").Exists())` 断言 key 存在；`require.Equal(t, int64(0), gjson.GetBytes(encoded, "max_tokens").Int())` 断言值。
- 预期结果：任何把该字段丢掉、写错名字、或改写值的实现都必然让某一条断言失败。

## 验收

- [ ] 值断言均写明期望值；grep `NotEmpty` / `Len.*>.*0` 用作值断言的处为 0。
- [ ] 错误断言均绑定具体 sentinel 或短语；无 `assert.Error(err)` 单独结尾的场景。
- [ ] 复合结构断言逐字段展开，不断「整体不为空」。
- [ ] 存在性断言（例如 `Exists()`）后同测试内必有值断言收尾。
