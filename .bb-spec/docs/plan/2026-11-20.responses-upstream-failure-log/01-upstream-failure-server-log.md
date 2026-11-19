---
name: 01-upstream-failure-server-log
description: 流式 Responses 三条路径在上游失败事件到达时各写一行带原文的 ERR 服务器日志
---
# 上游失败原文写进服务器日志

## 目标
上游在流式 Responses 回复里发失败事件时，服务器日志里多一行 ERR，写明渠道号、事件类型、错误码、错误类型和截断、脱敏后的错误消息。

## 业务规则（来源：PRD 01-upstream-failure-server-log）
- 失败事件：`type` 为 `error`、`response.failed`、`response.error`，或事件里 `response.status` 为 `failed`。判断条件和 `service/responses_usage.go` 里 `ObserveResponsesOutcome` 的失败分支一致。
- 覆盖三条流式路径：HTTP 流式直接转发（`OaiResponsesStreamHandler`）、Responses 转 Chat 流式（`OaiResponsesToChatStreamHandler`）、WebSocket（`relay/responses_websocket.go`，包括带 error 对象的终止 `error` 帧和 `response.failed` 事件）。
- 两条非流缓冲路径（`OaiResponsesBufferedStreamHandler`、`OaiResponsesToChatBufferedStreamHandler`）不在范围内：它们把失败作为请求错误返回，已经会记 `channel error` 日志。
- 每个失败事件记一行，同一个请求里有几个就记几行。
- 错误码、错误类型、错误消息全都拿不到时，照样记一行，写明上游没给错误信息。
- 错误消息先用 `common.MaskSensitiveInfo(msg, common.UpstreamAddresses(c)...)` 脱敏，再用 `common.LocalLogPreview` 截断。截断上限 2048 字节，超出时注明原始长度，开了 DEBUG 时不截断。这套规则和 `service/relay_error.go:71` 的 `channel error` 日志一致。
- 级别用 ERR（`logger.LogError`），文本是英文，并经过 `common.LogText`。请求编号由 logger 自动带上。
- 不改客户端收到的内容，不改消费日志字段，不改扣费。新函数只写日志，不调用 StreamStatus，也不碰 accumulator。
- 决策理由：AI 和管理员都通过服务器日志（MCP 读的就是它）来排查，所以只写服务器日志；三条路径属于同一个问题，所以一起覆盖。

## 涉及文件
- 修改 `service/responses_usage.go`
- 修改 `relay/channel/openai/relay_responses.go`
- 修改 `relay/channel/openai/chat_via_responses.go`
- 修改 `relay/responses_websocket.go`
- 修改 `service/responses_usage_test.go`（补测试）

## 日志文本（测试按这个断言）
有错误信息时：
`upstream responses failure (channel #<渠道号>, event: <事件类型>): code=<错误码> type=<错误类型> message=<脱敏截断后的消息>`
三项全空时：
`upstream responses failure (channel #<渠道号>, event: <事件类型>): upstream gave no error detail`
code、type、message 中只缺某一项时，那一项留空，例如 `code= type=server_error message=...`。

## 函数清单
### service/responses_usage.go
| 函数名 | 职责 |
|---|---|
| `LogResponsesStreamFailure` | 新增：判断一个 Responses 流事件是不是失败事件；是的话，从 `event.Response.GetOpenAIError()` 取错误码、错误类型、错误消息，取不到就用事件顶层的 `Code`、`Message`，再交给 `LogResponsesUpstreamError` 去写日志；不是失败事件就什么都不做 |
| `LogResponsesUpstreamError` | 新增：按上面的日志文本，写一行 ERR 日志，内容是渠道号、事件类型，以及脱敏、截断后的错误信息；错误对象为空，或三项全空时，写"上游没给错误信息"那一版 |

### relay/channel/openai/relay_responses.go
| 函数名 | 职责 |
|---|---|
| `OaiResponsesStreamHandler` | 修改：每个事件解析成功后，先调 `service.LogResponsesStreamFailure`，再调 `accumulator.Observe` |

### relay/channel/openai/chat_via_responses.go
| 函数名 | 职责 |
|---|---|
| `OaiResponsesToChatStreamHandler` | 修改：每个事件解析成功后，进入失败分支之前，调 `service.LogResponsesStreamFailure` |

### relay/responses_websocket.go
| 函数名 | 职责 |
|---|---|
| 处理上游事件的循环（`error` 帧分支 + 通用 `accumulator.Observe` 处） | 修改：终止的 `error` 帧（`terminal` 为真，不管上游有没有接受请求）调 `service.LogResponsesUpstreamError`，传入 `rejection.Error` 和事件类型 `error`；其余事件在 `accumulator.Observe` 前调 `service.LogResponsesStreamFailure` |

## 协作关系
- `OaiResponsesStreamHandler`、`OaiResponsesToChatStreamHandler`、WebSocket 循环都调 `LogResponsesStreamFailure`，它再调 `LogResponsesUpstreamError`；WebSocket 的 `error` 帧直接调 `LogResponsesUpstreamError`。
- 渠道号取 `info.ChannelId`（`RelayInfo` 里的字段）。
- 外部依赖：`logger.LogError`、`common.LogText`、`common.MaskSensitiveInfo`、`common.UpstreamAddresses`、`common.LocalLogPreview`。

## 验证方式
- 测试入口：
  - `service.LogResponsesStreamFailure`、`service.LogResponsesUpstreamError`（包级导出函数）。
  - 用 httptest 构造 mock 上游，跑 `openai.OaiResponsesStreamHandler` 和 `openai.OaiResponsesToChatStreamHandler`；按 `controller/responses_websocket_test.go` 现有的 mock 方式跑 WebSocket。
  - 测试里把 `gin.DefaultErrorWriter` 换成一个 buffer 来抓日志，测完还原。
- 测试输入：上游 SSE 或 WebSocket 事件，带渠道号 7。
- 预期结果：
- [ ] C1：HTTP 流式直接转发收到 `response.failed`，`response.error` 为 `{"code":"server_error","type":"server_error","message":"boom"}` → buffer 里恰好一行 `[ERR]`，包含 `upstream responses failure (channel #7, event: response.failed): code=server_error type=server_error message=boom`
- [ ] C2：同一个事件走 `OaiResponsesToChatStreamHandler` → 恰好一行同样的文本
- [ ] C3：WebSocket 收到已接受请求后的终止 `error` 帧（error 带 code、type、message），以及 `response.failed` 事件 → 两种情况各一行，事件类型分别是 `error` 和 `response.failed`
- [ ] C4：`response.failed` 的 `response.error` 为 null，事件顶层也没有 code 和 message → 一行 `... (channel #7, event: response.failed): upstream gave no error detail`
- [ ] C5：同一个流里先来 `{"type":"error","code":"rate_limit","message":"slow down"}`，再来 `response.failed` → 恰好两行，第一行 `event: error` 带 `code=rate_limit` 和 `message=slow down`
- [ ] C6：消息有 5000 字节 → 日志里的消息只有前 2048 字节，并带 `original_length=5000`；`common.DebugEnabled=true` 时是全文
- [ ] C7：消息里有 `sk-` 开头的密钥 → 日志里没有密钥原文
- [ ] C8：不是失败的事件（`response.completed`、`response.output_text.delta`）→ 没有新增日志行；C1、C2 的用例里，客户端收到的字节、返回的 usage 和改动前一样（现有 `relay/channel/openai/relay_responses_billing_test.go`、`controller/responses_websocket_test.go` 全部通过）
