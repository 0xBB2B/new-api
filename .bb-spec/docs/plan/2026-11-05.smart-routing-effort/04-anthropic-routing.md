---
name: 04-anthropic-routing
description: 虚拟模型接收 Anthropic 格式请求：识别用户新消息与续跑、取打分内容、换模型名，换渠道时清理思考块
---
# Anthropic 格式接入智能路由

## 目标
客户端用 `POST /v1/messages` 请求勾了 Anthropic 的虚拟模型时，打分、选档、会话、计费、日志都和 Responses、Chat 一样工作；换渠道时历史里的思考块被清掉。

## 业务规则（来源：PRD）
- 03-endpoint-formats C3：客户端格式不在所勾接口里 → 400「虚拟模型 <名称> 只支持这些接口：<所勾接口>」，不打分、不扣费。
- C4：路由后交给渠道适配器按现有规则转换；扣费、重试和直接用同一格式请求同一真实模型相同。
- C5：
  - 最后一条 `messages` 是 role=user 且含文字 → 用户新消息，打分；最后一条 role=user 的内容只有 `tool_result` 块 → 工具结果续跑，不打分。
  - 打分内容和一期相同：最新一条用户消息的文字、前几条用户消息的开头、本地特征；`system`、`tool_result`、`thinking` 块不发。用户消息里的 `image`、`document` 块算附件。
  - 会话识别：Anthropic 请求没有 prompt_cache_key，用「密钥 + 第一条用户消息的内容」算指纹。
  - 请求体 `model` 换成真实模型名；思考力度按 03 plan 的规则写入。
  - 每次发往上游前（含重试换渠道后），这次渠道和会话里记的上一次渠道不同 → 去掉历史 assistant 消息里的 `thinking`、`redacted_thinking` 块，日志记清理段数（字段 `cleaned_reasoning`，和 Responses 共用）；渠道相同原样发送。
- PRD 决策：Anthropic 换渠道清理思考块是推测，要实测：在 Claude 订阅渠道和 OpenAI 渠道之间来回换档，确认清理后上游不报错。
- Responses WebSocket、压缩接口的一期规则不变。
- 03 审查时发现的 Anthropic 改写问题，接入时必须一起修（middleware/smart_route.go 的 Anthropic 分支）：
  - 档位模型是已知 Claude 模型时，思考设置改用 relaykit 现成的 `reasoning.RenderClaude` 生成（thinking 与 output_config），不手写。原因：手动预算型模型（如 claude-sonnet-4-5）的 `thinking:{"type":"enabled"}` 必须带 budget_tokens，只删预算会被上游 400；Sonnet 5.5 关闭思考要用 `between_tools` 而不是 `disabled`。
  - 非 Claude 模型（会被渠道转换成别的格式）继续写 `output_config.effort`。
  - 重复键检查把 Anthropic 的顶层 `thinking`、`output_config` 以及 `output_config` 内部的 `effort` 也纳入：客户端写两遍同名键会让 ojson 改第一个、relay 读最后一个，绕过档位力度。

## 涉及文件
- 修改 `service/smartroute/inspect.go`
- 修改 `middleware/smart_route.go`
- 修改 `relay/claude_handler.go`、`relay/smart_route_reasoning.go`
- 测试：扩展 `service/smartroute/smartroute_test.go`、`middleware/distributor_test.go`；新建 `relay/smart_route_reasoning_test.go`（若已有同类测试文件则扩展它）

## 函数清单
### service/smartroute/inspect.go
| 函数名 | 职责 |
|---|---|
| `RequestEndpointFromPath` | `/v1/messages` → `anthropic` |
| `InspectRequest` | 增加 Anthropic 分支：读 `messages`，按上面的规则判断用户新消息与续跑、收集用户消息文字和附件 |
| `messageText` | 认 Anthropic 的 `text` 块；`image`、`document` 块算附件；`tool_result` 块不算文字 |
### middleware/smart_route.go
| 函数名 | 职责 |
|---|---|
| `applySmartRoute` | Anthropic 路径进入路由流程；接口检查用所勾接口 |
### relay/smart_route_reasoning.go、relay/claude_handler.go
| 函数名 | 职责 |
|---|---|
| `stripClaudeThinkingBlocks` | 去掉历史 assistant 消息里的 `thinking`、`redacted_thinking` 块，返回清理段数；去掉后内容为空的 assistant 消息保留空文字块，不删消息，保持对话轮次 |
| `ClaudeHelper` | 复制请求后、转换前，`shouldStripSmartRouteReasoning` 为真时调用 `stripClaudeThinkingBlocks`，再用 `recordStrippedReasoning` 记段数 |

## 协作关系
`InspectRequest` 的结果照旧给打分、`SessionKey`、`DecideRoute` 用，不改它们。`ClaudeHelper` 每次重试都会重新进入，所以按本次渠道判断即可。Anthropic 请求体改写（模型名、力度）在 03 的 `rewriteSmartRouteBody` 里。

## 验证方式
- 测试入口：`smartroute.InspectRequest`、`smartroute.RequestEndpointFromPath`；`middleware.Distribute` 处理 `/v1/messages`；`relay` 包内对 Claude 请求的思考块清理（包级函数）
- 测试输入：
  - Anthropic 请求体：最后一条是用户文字；最后一条只有 `tool_result`；用户消息含 `image`；带 `system`
  - 虚拟模型 `endpoints` 只含 Responses、或含 Anthropic
  - 历史里有 `thinking`、`redacted_thinking` 块的请求，会话上一次渠道与本次相同 / 不同
- 预期结果：
  - [ ] 最后一条用户文字 → IsUserTurn=true，打分输入只有用户文字，没有 system 和工具结果；只有 tool_result → IsUserTurn=false，不打分、没有打分记录
  - [ ] 含 image 的用户消息 → HasAttachment=true
  - [ ] 两次请求第一条用户消息相同 → SessionKey 相同
  - [ ] 虚拟模型没勾 Anthropic → 400，信息「Virtual model auto-gpt only supports these endpoints: Responses」，没有打分记录和消费记录
  - [ ] 勾了 Anthropic → 上游请求的 model 是选中档位的真实模型名，日志 smart_routing 字段和 Responses 请求的结构相同
  - [ ] 渠道变了 → 发出的请求里没有 thinking、redacted_thinking 块，日志 `cleaned_reasoning` 等于去掉的段数；渠道没变 → 原样
  - [ ] 手动预算型 Claude（claude-sonnet-4-5）客户端带 `thinking:{"type":"enabled","budget_tokens":N}`、档位固定 high → 上游请求的 thinking 由 RenderClaude 生成，带合法 budget_tokens；Sonnet 5.5 力度 none → `thinking.type=between_tools`
  - [ ] 请求里 `output_config` 或 `thinking` 重复出现、或 `output_config` 内 `effort` 重复 → 400
  - [ ] 思考力度改写（03 已实现、在这里端到端验证）：力度 high → 上游 `output_config.effort=high` 且没有 `thinking.budget_tokens`；力度 none → `thinking={"type":"disabled"}` 且没有 `output_config.effort`；客户端 `thinking.type=disabled` 而档位力度为 high → 去掉整个 `thinking`；不支持思考的模型 → 去掉 `thinking` 和 `output_config.effort`
- 实测（执行完记进完成简报）：配 Claude 订阅渠道和 OpenAI 渠道各一档，用 Claude Code 在同一会话里先简单后困难问题来回换档，确认上游不报错
