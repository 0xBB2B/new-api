---
name: relay-error-wrap-newapierror
description: relay 及其上下游返回的错误统一包装成 *types.NewAPIError，携带 ErrorCode、StatusCode 与重试/日志选项。
---

# relay 错误统一封装

## 目的

让 relay 层的错误有分类、有可控的重试语义与日志脱敏能力，并能一次性转换成 OpenAI/Claude 兼容错误响应。

## 逻辑

`types.NewError(err, code, ops...)` / `NewErrorWithStatusCode` / `NewOpenAIError` / `InitOpenAIError` 是唯一构造入口，构造后返回 `*NewAPIError`（自带 `Unwrap` 兼容 `errors.Is` / `errors.As`）。

需要跳过 relay 重试的错误必须显式加 `types.ErrOptionWithSkipRetry()`。不希望进错误日志的加 `types.ErrOptionWithNoRecordErrorLog()`。

深层已经是 `*NewAPIError` 的错误，外层 `NewError` 会保留内层并叠加 options，不再重新包裹（`errors.As` 分支）。

最终响应通过 `ToOpenAIError` / `ToClaudeError` / `MaskSensitiveErrorWithStatusCode` 转出，向客户端屏蔽敏感堆栈。

## 约束

- relay handler / middleware / service 内的错误路径必须返回 `*types.NewAPIError` 而非裸 `error`。
- 跳过重试的场景显式携带 `ErrOptionWithSkipRetry`（例如请求解析、非法 apiType、模型映射失败）。
- 构造时必须提供 `ErrorCode`，禁用空串或临时字符串。
- 外层再包装应经由 `types.NewError`（保留深层结构），禁再 `fmt.Errorf` 丢弃 `*NewAPIError`。

## 例子

- 输入：rerank handler 校验请求体失败。
- 过程：handler 内 `return types.NewError(fmt.Errorf("failed to copy request: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())`。
- 预期结果：上层不重试；错误码 = `invalid_request`；响应经 `ToOpenAIError` 转为 OpenAI 兼容错误 body；敏感信息被 `MaskSensitiveInfo` 脱敏。

## 验收

- [ ] relay 层错误路径的返回值类型均为 `*types.NewAPIError`。
- [ ] grep `return .*NewError\|NewErrorWithStatusCode` 的调用位置覆盖 relay/service/middleware 全部错误分支。
- [ ] 客户端类错误（请求解析、映射失败等）100% 带 `ErrOptionWithSkipRetry()`。
- [ ] 外层再包装不出现 `fmt.Errorf("...: %w", apiErr)` 丢弃结构的情况。
