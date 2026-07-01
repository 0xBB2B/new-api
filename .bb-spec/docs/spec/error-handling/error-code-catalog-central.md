---
name: error-code-catalog-central
description: ErrorCode/ErrorType 枚举集中在 types 包；业务包只引用常量，禁自造字面串。
---

# 错误码枚举集中登记

## 目的

让错误码分类唯一、可枚举、可跨层比对，避免各 handler 自造字符串导致日志/监控/前端映射割裂。

## 逻辑

`types/error.go` 声明 `type ErrorCode string` 与 `type ErrorType string` 两个枚举，所有取值以 `const` 块列出，按 domain 分组（channel: / 请求 / 响应 / sql / quota 等）。

上游包（relay / controller / service / model / middleware）只以 `types.ErrorCodeXxx` 常量传递，禁在业务包直接写字符串字面量作为错误码。

新增错误分类要求先在 `types/error.go` 增加常量，再在调用点引用。跨层比对（含日志脱敏、重试判定）以常量为准。

## 约束

- `ErrorCode` 枚举值只能在 `types/error.go` 定义（仓库内唯一定义文件）。
- 调用方以 `types.ErrorCodeXxx` 常量传参，禁内联字符串字面量作为错误码。
- 同类别错误共用前缀（例如 `channel:*`、`response/*`），跨包保持命名一致。
- 新增常量必须选择恰当的 domain 分组，禁将不同业务域的常量混编在一起。

## 例子

- 输入：新增一个渠道 key 无效的错误分类。
- 过程：先在 `types/error.go` 的 `const` 块加 `ErrorCodeChannelInvalidKey`；然后 relay adapter 用 `types.NewError(err, types.ErrorCodeChannelInvalidKey, types.ErrOptionWithSkipRetry())` 抛出。
- 预期结果：中间件 / controller 通过 `GetErrorCode()` 分类判定 `skipRetry`；日志/监控看到统一 tag。

## 验收

- [ ] 全仓 grep `ErrorCode\w+ *ErrorCode *= *"` 的定义仅出现在 `types/error.go`。
- [ ] 业务包调用点均引用 `types.ErrorCodeXxx` 常量，无内联字符串字面量。
- [ ] 常量分组注释清晰反映业务域。
- [ ] 前端错误映射表与 `types/error.go` 常量表 1:1 对齐。
