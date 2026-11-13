---
name: 03-claude-output-tag
description: Claude 格式响应里 thinking 块签名和 redacted_thinking 块 data 改写成渠道标记，流式和非流式
---
# Claude 响应打标记

## 目标
网关写给客户端的每个 Claude 格式响应里，思考块的签名（redacted 块的 data）换成渠道标记，思考正文不变。

## 业务规则（来源：PRD 01-channel-tag）
- `thinking` 块的 `signature`、`redacted_thinking` 块的 `data`，非空时换成 `EncodeReasoningTag(本次实际回答的渠道, 原值)`。渠道取 `common.GetContextKeyInt(c, constant.ContextKeyChannelId)`。
- 流式事件里要改写的位置：
  - `content_block_start` 的 `content_block`（类型为 thinking 时改 `signature`，为 redacted_thinking 时改 `data`）；
  - `content_block_delta` 中 `delta.type` 为 `signature_delta` 的 `delta.signature`。
- 非流式 body：顶层 `content[]` 中的 thinking / redacted_thinking 块。
- 思考正文（`thinking` 字段、`thinking_delta`）和其他块不改；没有可改内容的事件或 body 原样写出，一个字节都不改；JSON 解析失败原样写出。
- 覆盖 Claude 原生渠道（含 claude_oauth、aws）和 OpenAI / Gemini 上游被转成 Claude 格式的路径。

## 涉及文件
- service/reasoning_tag.go（修改：新增 Claude 改写函数）
- service/reasoning_tag_test.go（修改）
- relay/helper/common.go（修改：`ClaudeData`、`ClaudeChunkData`）
- relay/channel/claude/relay-claude.go（修改：`HandleClaudeResponseData` 中 Claude 格式分支写出前改写）
- relay/channel/openai/relay-openai.go（修改：非流式 `RelayFormatClaude` 分支写出前改写）
- relay/channel/gemini/relay-gemini.go（修改：非流式 `RelayFormatClaude` 分支写出前改写）
- relay/channel/openai/chat_via_responses.go（修改：若有 Claude 格式非流式写出点，同样改写；流式经 `ClaudeData` 已覆盖）
- relay/channel/claude/relay_claude_test.go（修改：补用例）

## 函数清单
### service/reasoning_tag.go
| 函数名 | 职责 |
|---|---|
| `TagClaudeThinkingEvent` | 按上述规则改写一个 Claude 流式事件的 JSON；没有可改内容时返回原字节 |
| `TagClaudeThinkingBody` | 改写一个非流式 Claude 响应 body 的 `content[]`；没有可改内容时返回原字节 |

### relay/helper/common.go
| 函数名 | 职责 |
|---|---|
| `ClaudeChunkData` | 写出前用 `TagClaudeThinkingEvent` 改写 data |
| `ClaudeData` | 序列化后、写出前用 `TagClaudeThinkingEvent` 改写 |

### 各 handler
| 函数名 | 职责 |
|---|---|
| `HandleClaudeResponseData`（Claude 分支）、OpenAI / Gemini 非流式的 Claude 分支 | 调 `IOCopyBytesGracefully` 前用 `TagClaudeThinkingBody` 改写 body |

## 协作关系
- 依赖 01 的 `EncodeReasoningTag`；与 02 都改 `relay/helper/common.go`，在 02 之后执行。
- JSON 改写方式同 02（ojson 或 `common.*`），不直接调用 `encoding/json` 编解码函数。

## 验证方式
- 测试入口：`service.TagClaudeThinkingEvent(data []byte, channelID int) []byte`、`service.TagClaudeThinkingBody(body []byte, channelID int) []byte`；HTTP 层用 `relay/channel/claude/relay_claude_test.go` 现有方式调 `ClaudeStreamHandler` / `ClaudeHandler`，gin 上下文设置 `channel_id`。
- [ ] 非流式 body `content` = [thinking(正文"T", signature `sig_z`), redacted_thinking(data `d+/=`), text("hi")]，渠道 7 → signature 解码得 (7, `sig_z`)，data 解码得 (7, `d+/=`)，正文"T"和 text 块不变。
- [ ] 流式 content_block_start(thinking, signature 空)、content_block_delta(thinking_delta "T")、content_block_delta(signature_delta `sig_z`)、content_block_start(redacted_thinking, data `d1`) → 只有 signature_delta 和 redacted data 被改写，解码得原值和渠道 7；thinking_delta 原样。
- [ ] 不含思考块的事件、非 JSON 文本 → 输出与输入逐字节相同。
- [ ] OpenAI 上游转 Claude 格式流式（`helper.ClaudeData` 路径）且产生带签名的 thinking 块时 → 签名是标记。
