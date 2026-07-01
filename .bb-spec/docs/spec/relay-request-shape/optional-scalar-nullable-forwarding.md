---
name: optional-scalar-nullable-forwarding
description: 顶层向上游发送的主请求 DTO 中，可选数值/布尔字段用指针 + omitempty，保留显式零值；嵌套结构不受此约束。
---

# 顶层主 DTO 可选标量用可空指针 + omitempty

## 目的

保证客户端在顶层主请求上设置的显式零值（`0` / `0.0` / `false`）不被网关在再序列化时静默丢弃，避免上游拿到与客户端语义相反的默认值。

## 逻辑

API 网关在客户端请求与上游请求之间做「反序列化 → 转换 → 再序列化」。若可选数值/布尔字段使用值类型 + `omitempty`，语言的 JSON 序列化无法区分「字段缺失」与「字段被显式设为 0 / false」，会把显式零值当作缺省剔除。

**规则口径**：本约束限定于**顶层向上游发送的主请求 DTO**——即 `GeneralOpenAIRequest`、`ClaudeRequest`、`EmbeddingRequest`、`OpenAIResponsesRequest` 之类的顶层入口 struct。这一层的可选数值/布尔标量一律采用可空指针（`*int` / `*float64` / `*bool` 等）配 `omitempty`，缺失 → `nil` → 不出现；显式零值 → 非 `nil` 的零 → 出现且值为 `0` / `false`。

**嵌套结构不受此约束**：诸如 `EmbeddingOptions`、`StreamOptions`、`ClaudeWebSearchTool`、`ClaudeToolChoice`、`GeminiThinkingConfig`、`GeminiImageParameters`、`GeminiEmbeddingRequest` 等嵌套 struct 里的可选标量目前使用值类型 + `omitempty` 是项目实况，不视为漂移。若某个嵌套字段被证明需要「显式零值透传」语义时，可单独按此规则改造，不作一刀切要求。

字符串等本身空值有明确「缺省」语义的字段可保持值类型 + `omitempty`，不受此约束。

## 约束

- 顶层向上游发送的主请求 DTO 中，可选的数值/布尔字段必须是指针类型。
- 同字段的 JSON tag 必须同时带 `omitempty`。
- 顶层主 DTO 的往返测试：客户端显式设置为 `0` / `false` 的字段，必须仍出现在上游请求 body 中。
- 顶层主 DTO 禁用非指针值类型配 `omitempty` 承接可选标量。
- 嵌套 struct 的字段类型不作强制要求；若嵌套字段有透传显式零值的需求，可按需改为指针。

## 例子

- 输入：客户端发送 `{ "temperature": 0, "stream": false, "max_tokens": 0 }`，三字段均为顶层主 DTO 字段，且在上游语义里对结果有实际影响。
- 过程：网关将请求解析为内部 DTO，然后按渠道适配器把 DTO 重新序列化发送给上游。
- 预期结果：上游收到的 JSON 里 `temperature` 仍为 `0`、`stream` 仍为 `false`、`max_tokens` 仍为 `0`；不得因为「零值等于缺省」而被丢弃。

## 验收

- [ ] 顶层主请求 DTO 中，可选的 `int` / `uint` / `float` / `bool` 字段类型均为指针。
- [ ] 顶层主请求 DTO 中所有 pointer 字段的 JSON tag 均带 `omitempty`。
- [ ] 顶层主 DTO 的零值往返测试覆盖 `0` / `false` / 缺失三种输入的再序列化结果。
- [ ] 全仓 grep 顶层主 DTO 中「非指针数值/布尔标量 + `omitempty`」命中数为 0。
