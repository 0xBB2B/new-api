---
name: 01-output-item-tag
description: Responses 响应改写从"只改思考记录编号"扩大到"所有带编号的输出条目"，函数随之改名
---
# Responses 所有输出条目编号打标记

## 目标
网关写给客户端的 Responses 响应里，每条带非空 `id` 的输出条目（不论类型），编号都换成渠道标记；同一条目在所有事件里标记一致。

## 业务规则（来源：PRD 01-output-item-tag）
- 每条带非空 `id` 的输出条目，`id` 换成 `service.EncodeReasoningTag(本次实际回答的渠道, 原 id)`，不论 `type`：消息、函数调用、自定义工具调用、联网搜索调用、思考记录，以及以后新增的类型。渠道号 <= 0 时不改。
- 流式事件中要改写的位置：
  - `response.output_item.added` / `response.output_item.done` 的 `item.id`（任何类型）；
  - 任何带顶层 `item_id` 字段的事件，例如 `response.output_text.delta`、`response.function_call_arguments.delta`、`response.custom_tool_call_input.delta`、`response.content_part.added`、`response.reasoning_*`；
  - `response.created` / `in_progress` / `completed` / `incomplete` / `failed` / `done` / `cancelled` / `canceled` 中 `response.output[]` 各条目的 `id`。
- 非流式 body：顶层 `output[]` 中各条目的 `id`。
- 不改：工具调用的 `call_id`、响应本身的 `id`（`response.id` 与非流式顶层 `id`）、条目其他字段。原 `id` 为空不改；JSON 无效原样返回；没有可改内容时返回原字节（逐字节不变）。
- 编码确定性已由 `EncodeReasoningTag` 保证：同一条目在不同事件里得到同一标记；还原值与原值逐字节相同。
- 覆盖面不变：现有调用点已覆盖 OpenAI 原生流式/非流式、codex 缓冲流式、Chat/Claude/Gemini 上游转 Responses、透传、WebSocket。

## 涉及文件
- service/reasoning_tag.go（修改）
- service/reasoning_tag_test.go（修改）
- relay/helper/common.go（修改：调用处改名）
- relay/responses_websocket.go（修改：两处调用改名）
- relay/channel/openai/relay_responses.go、relay/channel/openai/responses_via_chat.go、relay/channel/claude/relay-claude.go、relay/channel/gemini/relay_responses.go（修改：调用处改名）
- relay/channel/openai/relay_responses_billing_test.go（修改：补一个非思考条目的 HTTP 用例）

## 函数清单
### service/reasoning_tag.go
| 函数名 | 职责 |
|---|---|
| `TagResponsesItemEvent` | 由 `TagResponsesReasoningEvent` 改名并扩大范围：按上述规则改写一个流式事件 / WebSocket 帧中所有条目编号与 `item_id`；便宜的字节预判由 `"reasoning"` 改为是否含 `"id"` |
| `TagResponsesItemBody` | 由 `TagResponsesReasoningBody` 改名并扩大范围：改写非流式 body `output[]` 中所有条目的 `id` |
| `tagReasoningIDAt`、`tagReasoningOutput` | 保留职责，`tagReasoningOutput` 去掉"只改 reasoning 类型"的过滤；名称改为 `tagItemIDAt`、`tagItemOutput` |

### 调用处
| 位置 | 职责 |
|---|---|
| `ResponseChunkData`、`writeClient`、`runRequest` 终止帧处、各 handler 写出前 | 改为调用新函数名，行为随之扩大 |

## 协作关系
- 依赖已有 `EncodeReasoningTag`；改写 JSON 继续用 `github.com/MoeForge/moego/ojson`（项目已在用），不直接调用 `encoding/json` 编解码。
- 旧函数名不保留别名。

## 验证方式
- 测试入口：`service.TagResponsesItemEvent(data []byte, channelID int) []byte`、`service.TagResponsesItemBody(body []byte, channelID int) []byte`（原 `TagResponsesReasoning*` 用例改用新名）；HTTP 层用 relay/channel/openai/relay_responses_billing_test.go 现有的 `OaiResponsesHandler` / `OaiResponsesStreamHandler` 搭建方式，gin 上下文设 `channel_id`。
- [ ] 非流式 body：`output` = [reasoning `rs_r`, custom_tool_call `msg_aaa`（call_id `call_1`）, message `msg_bbb`, web_search_call `ws_ccc`]，渠道 5 → 四条 `id` 分别解码得 (5, 原值)；`call_id` 仍 `call_1`；顶层 `id` `resp_1` 不变。
- [ ] 流式事件逐个改写：output_item.added(custom_tool_call `msg_aaa`)、`response.custom_tool_call_input.delta`(item_id `msg_aaa`)、`response.output_text.delta`(item_id `msg_bbb`)、`response.content_part.added`(item_id `msg_bbb`)、output_item.done(message `msg_bbb`)、`response.completed`(response.id `resp_1`, output 含两条) → 每处 `msg_aaa` 都是同一标记、每处 `msg_bbb` 都是同一标记，`response.id` 仍为 `resp_1`。
- [ ] 无 `id`/`item_id` 的事件、`id` 为空的条目、非 JSON、渠道 0 → 逐字节不变。
- [ ] 思考记录的现有用例结果不变。
- [ ] HTTP：`OaiResponsesStreamHandler` 上游流含 custom_tool_call `msg_aaa` → 客户端收到的编号是标记，解码出本次渠道。
