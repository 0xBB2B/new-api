---
name: 04-stream-opening-hold
description: 流式回复扣住开场事件到正文开始，正文前的失败和无终止断开改为返回可重试错误
---
# 扣住开场事件（relay/helper 与流式 handler）

> 本 plan 改上游响应里用量相关的流程（Responses 累加器、结算前的返回值）。实现与测试前必须用读文件工具完整读一遍 `.agents/rules/billing.md`，并遵守其中全部规则。

## 目标
Responses、Claude、OpenAI Chat 三种格式的流式回复，在正文开始前不向客户端发任何事件；正文开始前上游失败或无终止断开时，handler 返回可重试的错误，让重试循环换渠道。

## 业务规则（来源：PRD 01-sync-request-failover C2、C10、C15、C16）
- 开场事件名单（只认这些）：
  - Responses：`response.created`、`response.in_progress`、`response.queued`
  - Claude：`message_start`、`ping`
  - OpenAI Chat：没有文字、思考内容、工具调用、结束原因（finish_reason）和用量的数据块（通常只带 `role`）
- 名单外的事件一律算正文开始：先按原顺序发出所有扣住的事件，再发这个事件，并置 `ContextKeyStreamOutputSent`。扣住期间不置这个标记。
- 失败事件：Responses 的 `error`、`response.failed`、`response.error`，或事件里 `response.status` 为 `failed`（判定口径与 `service/responses_usage.go` 的失败判定一致）；Claude 的 `error` 事件；Chat 流里带 `error` 对象（type 或 message 非空）的数据块。正文开始前遇到失败事件：不发给客户端，记成"正文前失败"（可重试的上游错误，状态码取事件里的，没有时用 500），handler 随即返回这个错误。正文开始后遇到失败事件：行为与改动前一致。
- 终止事件：Responses 的 `response.completed`、`response.done`、`response.incomplete`；Claude 的 `message_stop`；Chat 的 `[DONE]`。正文开始前遇到终止事件：先补发扣住的事件，再发终止事件，按正常结束处理。
- 正文开始前流结束且没有终止事件（包括一个事件都没有）：不发任何东西，handler 返回可重试的上游错误（状态码 502）。
- 心跳（`PingData`）照旧，可以在扣住期间提交响应头；之后换渠道成功的回复照常写在同一个流里，全部失败时由现有 `writeRelayStreamError` 在流里返回错误。
- 格式转换路径也要遵守：Responses 客户端←Chat 上游（`relay/channel/openai/responses_via_chat.go`）、Responses 客户端←Claude 上游（`relay/channel/claude/relay_responses.go`）、Responses 客户端←Gemini 上游（`relay/channel/gemini/relay_responses.go`）现在会先发 `response.failed` 并带"跳过重试"标记：改为正文开始前失败时不发、返回可重试错误；正文开始后照旧。Chat 客户端←Responses 上游（`chat_via_responses.go`）已在没输出时返回错误，确认开场块被扣住后仍成立。
- Gemini 格式的输出（直接 `c.Render` 的路径）不在本 plan 范围内。
- 用量：正文前失败的尝试，handler 返回错误、不结算；Responses 累加器在这次尝试里观察到的内容不得进入后续结算（与 plan 03 的残留清空配合）。
- 正常请求（有正文）客户端收到的事件顺序、内容与改动前逐字节一致。

## 涉及文件
- 修改 `relay/helper/common.go`
- 修改 `relay/channel/openai/relay-openai.go`、`relay/channel/openai/relay_responses.go`、`relay/channel/openai/responses_via_chat.go`、`relay/channel/openai/chat_via_responses.go`
- 修改 `relay/channel/claude/relay-claude.go`、`relay/channel/claude/relay_responses.go`
- 修改 `relay/channel/gemini/relay_responses.go`
- 修改 `relay/channel/openai/relay_responses_billing_test.go`（其中 "done with failed response" 期望 `apiErr==nil` 的用例改为按新行为断言）、`relay/channel/openai/chat_via_responses_test.go`、`relay/channel/claude/relay_claude_test.go`、`relay/helper/stream_scanner_test.go`（测试，Red 阶段）

## 函数清单
### relay/helper/common.go
| 函数名 | 职责 |
|---|---|
| `StartStreamHold` | 在 context 上开启扣留状态（待发队列、正文是否开始、正文前失败） |
| `ResetStreamHold` | 清空扣留状态，供 plan 03 每次重试前调用 |
| `PreContentFailure` | 返回本次尝试记下的正文前失败（没有则 nil） |
| `FinishStreamHold` | 流结束时调用：见过终止事件则补发；正文没开始且没见过终止事件则记下可重试错误并返回 |
| `ClaudeData` / `writeClaudeEvent` / `ResponseChunkData` / `StringData` | 修改：写出前按上面的名单分类，开场事件入队、失败事件记录、其他事件先清队列再写并置输出标记 |

### 各 handler
| 函数名 | 职责 |
|---|---|
| `OaiStreamHandler`（relay-openai.go） | 开启扣留；每次写出后检查 `PreContentFailure`，有则返回；结束时调 `FinishStreamHold` |
| `OaiResponsesStreamHandler`（relay_responses.go） | 同上；正文前失败的事件不喂给累加器 |
| `ClaudeStreamHandler` / `HandleStreamResponseData`（relay-claude.go） | 同上；保留现有 `error` 事件返回错误的行为 |
| `responses_via_chat.go`、`claude/relay_responses.go`、`gemini/relay_responses.go` 中写 `response.failed` 的流处理函数 | 正文开始前失败时改为返回可重试错误，不带跳过重试标记 |

## 协作关系
- `ResetStreamHold` 由 plan 03 的 `Relay` 循环在每次重试前调用。
- handler 返回的错误交给 `controller/relay.go` 的 `service.DecideRelayRetry` 判定是否换渠道（plan 03）。

## 验证方式
- 测试入口：`go test ./relay/channel/openai/ ./relay/channel/claude/ ./relay/helper/`，用 `gin.CreateTestContext` + httptest 记录器（沿用 `relay_responses_billing_test.go` 的夹具，`DisablePing: true`）喂上游 SSE。
- 测试输入与预期（每条断言客户端收到的字节、handler 返回的错误、`ContextKeyStreamOutputSent`）：
  - Responses：`response.created` → `response.failed`(code=server_error) → 客户端 0 字节，返回可重试错误，输出标记为假。
  - Responses：`response.created` → `response.output_text.delta` → `response.completed` → 客户端收到三个事件，顺序和内容与改动前一致，返回 nil。
  - Responses：`response.created` → `response.completed` → 客户端收到这两个事件，返回 nil。
  - Responses：只有 `response.created` 然后 EOF → 客户端 0 字节，返回状态码 502 的可重试错误；一个事件都没有就 EOF → 同样。
  - Responses：`response.output_text.delta` 之后 `response.failed` → 与改动前一致（客户端收到失败事件，返回 nil）。
  - Claude：`message_start` → `error` → 客户端 0 字节，返回错误；`message_start` → `content_block_delta` → `message_stop` → 正常；`message_start` → `message_stop` → 补发两个事件，正常。
  - Chat：只有 role 的块 → 带 `error` 的块 → 客户端 0 字节，返回错误；只有 role 的块 → 文字块 → `[DONE]` → 与改动前一致；只有 role 的块 → `[DONE]` → 补发，正常；只有 role 的块后 EOF → 返回 502 可重试错误。
  - Responses 客户端←Chat 上游：首块即错误 → 客户端 0 字节，返回的错误不带跳过重试标记。
  - 心跳开启时扣住期间发 ping → 客户端只收到 ping，输出标记为假。
- [ ] 上述用例通过；`go test ./relay/...` 全绿；`gofmt` 无改动
