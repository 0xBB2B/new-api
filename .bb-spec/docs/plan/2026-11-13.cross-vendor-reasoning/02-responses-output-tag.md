---
name: 02-responses-output-tag
description: Responses 格式响应（流式、非流式、WebSocket、各上游转换）里的思考记录编号改写成渠道标记
---
# Responses 响应打标记

## 目标
网关写给客户端的每个 Responses 格式响应里，`type` 为 `reasoning` 的条目编号都换成渠道标记，同一条记录在所有事件里标记一致。

## 业务规则（来源：PRD 01-channel-tag）
- 每条 `type` 为 `reasoning` 的输出条目，返回给客户端的 `id` 换成 `EncodeReasoningTag(本次实际回答的渠道, 原 id)`。渠道取 `common.GetContextKeyInt(c, constant.ContextKeyChannelId)`，重试后就是最后成功的渠道；WebSocket 用会话锁定的渠道 `s.lockedChannelID`。
- 流式事件里要改写的位置：
  - `response.output_item.added` / `response.output_item.done` 的 `item.id`，前提是 `item.type` 为 `reasoning`；
  - 所有 `type` 以 `response.reasoning_` 开头的事件（摘要、正文的增量和完成事件）的 `item_id`；
  - `response.created` / `response.in_progress` / `response.completed` / `response.incomplete` / `response.failed` 里 `response.output[]` 中的 reasoning 条目 `id`。
- 非流式 body：顶层 `output[]` 中的 reasoning 条目 `id`。
- 其他条目（消息、工具调用）的编号和内容一律不改；没有 reasoning 条目的事件或 body 原样写出，一个字节都不改。
- 原 id 为空的 reasoning 条目不改。JSON 解析失败时原样写出，不报错。
- 覆盖开透传的渠道（响应同样经下列出口）、WebSocket、以及中台把 Chat / Claude / Gemini 上游回复转成 Responses 的路径。

## 涉及文件
- service/reasoning_tag.go（修改：新增 Responses 改写函数）
- service/reasoning_tag_test.go（修改）
- relay/helper/common.go（修改：`ResponseChunkData`）
- relay/channel/openai/relay_responses.go（修改：`OaiResponsesHandler`、`OaiResponsesBufferedStreamHandler` 写出前改写）
- relay/channel/openai/responses_via_chat.go（修改：`OaiChatToResponsesHandler` 写出前改写）
- relay/channel/claude/relay-claude.go（修改：`HandleClaudeResponseData` 中 Responses 格式分支写出前改写）
- relay/channel/gemini/relay_responses.go（修改：`GeminiResponsesHandler` 写出前改写）
- relay/responses_websocket.go（修改：`writeClient` 与终止帧 `WriteMessage` 处）
- relay/responses_websocket_test.go、relay/channel/openai/relay_responses_billing_test.go（修改：补用例）

## 函数清单
### service/reasoning_tag.go
| 函数名 | 职责 |
|---|---|
| `TagResponsesReasoningEvent` | 按上述规则改写一个 Responses 流式事件或 WebSocket 消息的 JSON；没有可改的内容时返回原字节 |
| `TagResponsesReasoningBody` | 改写一个非流式 Responses 响应 body 的 `output[]`；没有可改的内容时返回原字节 |

### relay/helper/common.go
| 函数名 | 职责 |
|---|---|
| `ResponseChunkData` | 写出前用 `TagResponsesReasoningEvent` 改写 data |

### 各 handler / WebSocket
| 函数名 | 职责 |
|---|---|
| `OaiResponsesHandler`、`OaiResponsesBufferedStreamHandler`、`OaiChatToResponsesHandler`、`HandleClaudeResponseData`（Responses 分支）、`GeminiResponsesHandler` | 调 `IOCopyBytesGracefully` 前用 `TagResponsesReasoningBody` 改写 body |
| `writeClient` 及 `runRequest` 里直接 `WriteMessage` 终止帧处 | 文本帧写出前用 `TagResponsesReasoningEvent` 改写 |

## 协作关系
- 依赖 01 的 `EncodeReasoningTag`。改写 JSON 用项目已在用的 `github.com/MoeForge/moego/ojson`（`relay/smart_route_reasoning.go` 已在用）或 `common.Unmarshal`/`common.Marshal`；不直接调用 `encoding/json` 的编解码函数。
- `/v1/responses/compact` 的 compaction 输出不改写（PRD 非目标）。

## 验证方式
- 测试入口：`service.TagResponsesReasoningEvent(data []byte, channelID int) []byte`、`service.TagResponsesReasoningBody(body []byte, channelID int) []byte`；HTTP 层用 `httptest` 起假上游，调 `openai.OaiResponsesHandler` / `OaiResponsesStreamHandler`（gin 测试上下文设置 `channel_id`）；WebSocket 用 `relay/responses_websocket_test.go` 现有的假上游搭建方式。
- [ ] 非流式 body `output` = [reasoning `rs_y`, message `msg_1`]，渠道 5 → reasoning 的 id 解码得 (5, `rs_y`)，message 的 id 仍是 `msg_1`，其余字段不变。
- [ ] 流式依次发 output_item.added(reasoning `rs_y`)、reasoning_summary_text.delta(item_id `rs_y`)、output_text.delta(item_id `msg_1`)、output_item.done(reasoning `rs_y`)、response.completed(output 含 `rs_y` 和 `msg_1`) → 客户端收到的 `rs_y` 的四处出现都是同一个标记、解码得 (5, `rs_y`)；`msg_1` 各处不变。
- [ ] 不含 reasoning 的事件、非 JSON 文本 → 输出与输入逐字节相同。
- [ ] WebSocket：上游发 response.completed（output 含 reasoning `rs_w`），锁定渠道 9 → 客户端收到的帧里该 id 解码得 (9, `rs_w`)。
- [ ] Chat 上游转 Responses（`OaiChatToResponsesStreamHandler`，上游 chunk 带 `reasoning_content`）→ 客户端收到的 reasoning 条目 id 是标记，解码出本次渠道。
