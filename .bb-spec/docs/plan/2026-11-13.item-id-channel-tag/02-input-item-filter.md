---
name: 02-input-item-filter
description: 发往上游前，非思考条目按编号标记处理：同渠道还原编号，其余去掉编号和顶层加密内容、保留条目
---
# 请求方向处理非思考条目的编号

## 目标
每次发往上游前（含重试），Responses `input` 里思考记录和 `item_reference` 以外的每条条目：标记是本渠道 → 还原编号；其他情况 → 去掉 `id` 和顶层 `encrypted_content`，条目保留。

## 业务规则（来源：PRD 02-input-item-filter）
- 检查对象：`input` 数组里 `type` 不是 `reasoning`、不是 `item_reference` 的每一条，包括没有 `type` 字段的简写消息 `{"role":..,"content":..}`。
- `id` 用 `service.DecodeReasoningTag` 读得出且渠道 == 目标渠道（`info.ChannelId`）→ `id` 还原成原值，`encrypted_content` 等其余字段原样。
- 其他情况（别的渠道的标记、标记读不出、不是标记、没有 `id`）→ 删除 `id` 和条目顶层 `encrypted_content`（存在才删），其余字段（含 `call_id`、`role`、`content`、`type`）原样，条目不删除。
- 这项处理不计入清理段数：返回的清理段数只统计思考记录（与现状一致），智能路由结果的 `StrippedReasoningCount` 与日志字段 `cleaned_reasoning` 不因它变化，服务器日志里"stripped N reasoning segments"那一行也只按思考记录段数记。
- 有任何字段被改动时标记"有改动"；透传渠道只有"有改动"时才改写原始 body，否则原样发送（逐字节不变）。WebSocket 经同一个 `PrepareResponsesRequest`，同样生效。
- 思考记录、`item_reference` 现有规则不变。
- 条目上的 `approval_request_id`：标记读得出且渠道 == 目标渠道 → 还原原值；否则 `mcp_approval_response` 条目整条去掉，其他条目只删该字段。这项也不计入清理段数。
- 重试换渠道：每次尝试都从客户端原始请求重新处理（现有机制已保证），不会带上一次尝试的结果。

## 涉及文件
- relay/reasoning_filter.go（修改）
- relay/responses_request.go（修改：调用处改名）
- relay/smart_route_scorer_test.go（修改：补用例；该文件已有 `prepareResponsesSent` 等搭建工具）

## 函数清单
### relay/reasoning_filter.go
| 函数名 | 职责 |
|---|---|
| `filterResponsesInput` | 由 `filterResponsesReasoningInput` 改名：逐条处理 `input`——思考记录与 `item_reference` 沿用现有分支；其余条目按上述规则还原或去掉 `id`/`encrypted_content`；返回新 input、思考记录清理段数、是否有改动 |

### relay/responses_request.go
| 函数名 | 职责 |
|---|---|
| `PrepareResponsesRequest` | 两处调用改用 `filterResponsesInput`，其余不变 |

## 协作关系
- 依赖已有 `service.DecodeReasoningTag`；JSON 读写沿用文件现有方式（`common.Unmarshal`/`common.Marshal` 与 `json.RawMessage`），不直接调用 `encoding/json` 编解码。
- 依赖 01 产出的标记格式（与思考记录相同），本 plan 测试用 `service.EncodeReasoningTag` 直接构造标记，不依赖 01 的代码。
- 旧函数名不保留。

## 验证方式
- 测试入口：`relay.PrepareResponsesRequest`（复用 relay/smart_route_scorer_test.go 已有的 `prepareResponsesSent` 等搭建，转换与透传两种模式）。
- [ ] 目标渠道 8，input = [custom_tool_call（id 为渠道 5 标记，call_id `call_1`）, custom_tool_call_output（id `ctco_<uuid>`, call_id `call_1`）, message（id `msg_<uuid>`, role user）] → 上游三条均无 `id`，`call_id` 都是 `call_1`，其余字段与原条目一致，条目数 3。
- [ ] 目标渠道 8，input = [custom_tool_call（id 为渠道 8 标记、原值 `ctc_x`）, message（id 为渠道 8 标记、原值 `msg_y`、带 encrypted_content `E`）] → 上游 id 分别为 `ctc_x`、`msg_y`，消息 encrypted_content 为 `E`。
- [ ] 同上消息发往渠道 9 → 无 `id`、无 `encrypted_content`，`role`、`content` 不变。
- [ ] 编号为被篡改的标记、没有 `type` 的简写消息 `{"role":"user","content":"hi","id":"msg_z"}` → `id` 被去掉，其余不变。
- [ ] 只有非思考条目被去掉编号 → 清理段数 0，智能路由结果 `StrippedReasoningCount` 为 0、日志无 `cleaned_reasoning`、服务器日志无 "stripped" 行。
- [ ] 透传：input 的非思考条目都不带 `id` 和 `encrypted_content`（且无思考记录）→ 上游 body 与客户端原 body 逐字节相同；带别渠道 id → 被去掉。
- [ ] 第一次发往渠道 3、重试发往渠道 8（标记为渠道 8 的条目）→ 第一次去掉编号，重试那次还原（沿用现有重试测试方式）。
- [ ] mcp_approval_response（approval_request_id 为渠道 8 标记、原值 `mcpr_1`）→ 发往 8：为 `mcpr_1`；发往 9：整条不在。mcp_call 带渠道 5 标记的 approval_request_id、发往 9 → 删该字段，条目保留。
- [ ] 思考记录、`item_reference` 现有用例结果不变。
