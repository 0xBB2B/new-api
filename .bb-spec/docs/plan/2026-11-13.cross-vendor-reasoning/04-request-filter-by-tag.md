---
name: 04-request-filter-by-tag
description: 发往上游前按每条思考记录的标记还原或清理，取代智能路由会话判断，覆盖所有 Responses 与 Claude 请求
---
# 请求方向按标记还原或清理

## 目标
每次发往上游前（含重试），逐条看思考记录的标记：标记里的渠道就是这次的渠道就还原原值；否则清理。判断只依据标记。

## 业务规则（来源：PRD 02-strip-by-tag）
- 检查对象：Responses 请求 `input` 里 `type` 为 `reasoning` 的条目（看 `id`）；Claude 请求 assistant 消息里的 `thinking` 块（看 `signature`）和 `redacted_thinking` 块（看 `data`）。
- 用 `DecodeReasoningTag` 读标记；读得出且渠道等于这次的 `info.ChannelId` → 把 `id` / `signature` / `data` 换回原值，其余原样。
- 渠道不同、没有标记、或读不出 → 清理：
  - Responses：去掉 `id` 和 `encrypted_content`，保留 `summary`；`summary` 为空的整条去掉。
  - Claude：去掉这些思考块；去掉后变空的 assistant 消息整条删除；请求以工具结果结尾、而这一轮第一条 assistant 消息有思考块被清掉、且客户端 `thinking.type` 为 `enabled` 时，把思考设置改为 `{"type":"disabled"}` 并去掉 `output_config.effort`（沿用现有 `claudeTurnFirstAssistant`、`disableClaudeThinking` 的做法）。
- 是否清理只看标记，不看智能路由会话里记的上一次渠道、不看是否走 auto。
- 透传渠道同样处理原始 body；请求里没有任何思考记录时，透传 body 原样发送，一个字节都不改。
- 清理段数 > 0 时用 `logger.LogInfo` 记一行（英文，经 `common.LogText`），写明清理了几段；走 auto 的请求照旧调用 `recordStrippedReasoning` 让使用日志显示"上一轮用的渠道不同，已清理 X 段"，不走 auto 的请求它本来就不写（`GetRouteResult` 为空直接返回）。
- 思考记录以外的条目不增减、不改内容。
- `shouldStripSmartRouteReasoning` 改完后没有调用者，删除；`stripEncryptedReasoningInput`、`stripClaudeThinkingBlocks`、`stripClaudeThinkingBlocksInBody` 改为按标记逐条决定，并按新职责改名。

## 涉及文件
- relay/smart_route_reasoning.go（修改：本文件职责变为按标记处理思考记录，文件名改为 relay/reasoning_filter.go）
- relay/responses_request.go（修改：`PrepareResponsesRequest` 调用处）
- relay/claude_handler.go（修改：非透传与透传两处调用）
- relay/smart_route_reasoning_test.go → relay/reasoning_filter_test.go（改名并改写用例）
- relay/smart_route_scorer_test.go（修改：632、670、692、725 附近按会话判断清理的用例改成按标记）

## 函数清单
### relay/reasoning_filter.go
| 函数名 | 职责 |
|---|---|
| `filterResponsesReasoningInput` | 对 Responses 的 `input` JSON 逐条按标记还原或清理，返回新 input、清理段数、是否有改动 |
| `filterClaudeThinkingBlocks` | 对已解析的 Claude 请求逐块按标记还原或清理，返回清理段数和"这一轮第一条 assistant 是否丢了思考" |
| `filterClaudeThinkingBlocksInBody` | 对透传的 Claude 原始 body 做同样处理，返回新 body、清理段数 |
| `recordStrippedReasoning` | 保留现有行为（只写入智能路由结果），并在段数 > 0 时记服务器日志 |
| `claudeTurnFirstAssistant`、`claudeTurnLostThinking`、`disableClaudeThinking`、`claudeBlockTypes`、`isClaudeThinkingBlock`、`isToolResultOnlyBlocks` | 保留现有职责 |

### relay/responses_request.go、relay/claude_handler.go
| 函数名 | 职责 |
|---|---|
| `PrepareResponsesRequest` | 非透传：对 `request.Input` 调 `filterResponsesReasoningInput`；透传：只有 input 里有思考记录时才改写原始 body，否则原样 |
| Claude 处理入口（`claude_handler.go` 两处） | 非透传调 `filterClaudeThinkingBlocks` + 视结果关思考；透传调 `filterClaudeThinkingBlocksInBody` |

## 协作关系
- 依赖 01 的 `DecodeReasoningTag`；`info.ChannelId` 由 `InitChannelMeta` 每次尝试重取，重试换渠道时重新计算。
- 清理后的 Responses 条目没有 `id`，05 的聊天格式转换据此把它整条丢掉。

## 验证方式
- 测试入口：`relay.PrepareResponsesRequest`（gin 测试上下文 + `RelayInfo`，`channel_id` 设为目标渠道，可开关 `ChannelSetting.PassThroughBodyEnabled`）；Claude 用 `relay/smart_route_scorer_test.go` 现有的 Claude 请求测试搭建方式；标记用 `service.EncodeReasoningTag` 生成。
- [ ] input 含 4 条标记为渠道 5 的 reasoning（id 原值 `rs_a`…），目标渠道 5 → 发出的 4 条与原条目逐字段相同、id 是原值。
- [ ] 标记为渠道 8 的：2 条带 summary + encrypted_content、1 条只有 encrypted_content、3 条只有 content 明文，目标渠道 9 → 剩 2 条只含 summary（无 id、无 encrypted_content），其余 4 条不在；清理段数 6，服务器日志有一行。
- [ ] 无标记的 `msg_f836…`、标记被改一个字符的条目 → 被清理，请求正常构建。
- [ ] 智能路由会话的上一次渠道 = 9、标记渠道 8、目标 9 → 清理；会话 = 8、标记 9、目标 9 → 还原；无会话 → 同样按标记。
- [ ] 透传开启：input 无 reasoning → 发出的 body 与客户端原 body 逐字节相同；有 → 按上面规则处理。
- [ ] Claude：一条 assistant 含渠道 7 标记的 thinking + text、另一条只有 redacted_thinking，目标 6 → 第一条只剩 text，第二条整条删除；请求以 tool_result 结尾、本轮首条 assistant 丢了思考、thinking enabled → 上游收到 `{"type":"disabled"}`；目标 7 → 签名与 data 还原为原值。透传 Claude body 同样三种情况。
- [ ] message、function_call、function_call_output、tool_use、tool_result 条目数量和内容不变。
