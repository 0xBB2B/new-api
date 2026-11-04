---
name: 08-reasoning-cleanup
description: Responses 请求经智能路由、且本次渠道与会话上一次渠道不同时，发上游前去掉历史里的加密思考内容
---
# 换渠道时清理加密思考内容

## 目标
同一段对话里换了渠道（换模型或换账号），上游不会因为看不懂别的渠道留下的加密思考内容而报 `invalid_encrypted_content`。

## 业务规则（来源：PRD 03-session-tier-switch C8）
- 只对经过智能路由的 Responses 请求生效；Chat Completions 不清理。
- 每次发往上游之前（包括重试换渠道之后）比较两个渠道：一个是本次尝试使用的渠道（`relayInfo.ChannelId`），一个是 07 放进 context 的会话记录里的 `LastChannelID`。两者不同才清理；会话没有记录时不清理。
- 清理规则（只动 `input` 数组里 `type` 为 `reasoning` 的条目）：
  - 有可读摘要（`summary` 是非空数组）→ 去掉 `encrypted_content` 和 `id` 两个字段，保留其余字段；
  - 没有摘要 → 整个条目去掉。
- 清理掉的条目数（两种情况都算）累加进路由结果，供 09 写进日志；没清理就不写。
- 依据：LiteLLM 文档说明加密思考内容绑定生成它的组织或密钥，换渠道后原样转发会报 invalid_encrypted_content。
- 每次尝试只改本次尝试的那份请求拷贝，不影响下一次尝试（下一次可能换回原渠道）。

## 涉及文件
- relay/responses_request.go（修改：在 `:37` `common.DeepCopy(req)` 之后改 DTO 的 Input；在 `:52` 透传分支改本次要发送的请求体字节）
- relay/smart_route_reasoning.go（新建）

## 函数清单
### relay/smart_route_reasoning.go
| 函数名 | 职责 |
|---|---|
| `shouldStripSmartRouteReasoning` | 根据 context 里的路由结果和 `relayInfo.ChannelId` 判断本次要不要清理 |
| `stripEncryptedReasoningInput` | 对 Responses `input` 的原始 JSON 按清理规则处理，返回新 JSON 和清理条数；input 不是数组时原样返回 |

### relay/responses_request.go
| 函数名 | 职责 |
|---|---|
| （Responses 处理主函数，`:25` 起） | 修改：DeepCopy 之后，如需清理，就替换拷贝里的 `Input`；透传分支在发送前对请求体字节里的 `input` 做同样处理；清理条数写回路由结果 |

JSON 处理用 `ojson`（项目现有），禁止直接调用 `encoding/json`。

## 协作关系
- 依赖 07 定义的 `smartroute.GetRouteResult`（service/smartroute/result.go）。
- 09 读取清理条数写进日志。

## 验证方式
- 测试入口：`stripEncryptedReasoningInput`（表格驱动，写在 relay 包现有的 responses 相关测试文件里；没有就合并进 06 新建的 `relay/smart_route_scorer_test.go`）；`shouldStripSmartRouteReasoning`。
- 测试输入：input 数组含 2 条带摘要的 reasoning、1 条无摘要的 reasoning、普通 message 和 function_call 条目。
- 预期结果：
- [ ] 带摘要的两条：`encrypted_content` 和 `id` 被去掉，`summary`、`type` 保留；无摘要的那条整条去掉；其他条目一字不变；返回清理条数 3
- [ ] input 为字符串 → 原样返回，清理条数 0
- [ ] 渠道与会话上一次相同 → 不清理；会话没有记录 → 不清理；不是智能路由请求 → 不清理
- [ ] 透传模式下，发往上游的字节已被清理，context 里缓存的原始请求体不变（重试时还能用原始内容重新判断）
