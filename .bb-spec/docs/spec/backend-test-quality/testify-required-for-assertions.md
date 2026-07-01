---
name: testify-required-for-assertions
description: 后端测试断言统一走 testify；require 用于致命前置，assert 用于值检查；禁手写 t.Fatalf。
---

# 测试断言统一走 testify

## 目的

让失败信息统一含期望/实际值与行号，避免每个测试文件重复造断言 helper，也避免手写字符串拼接的失败信息让读者靠格式化模板脑补。

## 逻辑

Go 内建的 `t.Errorf` / `t.Fatalf` 只能拼字符串，读者要从格式化模板脑补期望值与结构。断言分成两类：

- **致命前置条件**——一失败后续断言无意义，用会立即中止测试的 `require` 系列（例如 `require.NoError`、`require.NotNil`）。
- **值检查**——可继续跑给出更多失败信号，用 `assert` 系列（例如 `assert.Equal`、`assert.Contains`、`assert.True`）。

同一个测试可以混用，但同一处断言只能选其一。手写 `t.Fatalf` 仅在测试库本身不便封装（例如与外部协议交互的假 server 桩），或 pre-testify 遗留文件里容忍。

## 约束

- 新增或重写的 `*_test.go` 必须 `import "github.com/stretchr/testify/require"` 或 `"github.com/stretchr/testify/assert"`；同文件同时出现两者是常态。
- 阻断后续断言的前置条件（DB 初始化、Unmarshal、NoError）必须用 `require`。
- 值等价、包含、真/假类断言必须用 `assert`。
- 禁在新代码里写 `if got != want { t.Fatalf(...) }` 手写断言。

## 例子

- 输入：一个纯函数 `Discount(price, tier)` 返回折扣价；测试要覆盖 vip/regular 两档，且解析折扣表可能失败。
- 过程：起手 `require.NoError(loadDiscountTable())`（失败没必要往下跑）；每档用 `assert.Equal(expected, Discount(price, tier))` 断言。
- 预期结果：断言失败时框架自动打印期望值、实际值与调用行号；致命前置失败立即停在 `require` 处。

## 验收

- [ ] 新增/重写测试文件 grep `stretchr/testify` 命中数为文件数本身（100% 覆盖）。
- [ ] 全仓 grep `t\.Fatalf\(` 在新代码中命中数为 0；遗留 pre-testify 文件可在触碰时迁移。
- [ ] 致命前置断言均使用 `require.*`；非致命值断言均使用 `assert.*`。
