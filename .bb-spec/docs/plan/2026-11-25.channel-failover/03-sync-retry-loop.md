---
name: 03-sync-retry-loop
description: 同步重试循环改为试完全部渠道为止、全失败返回最后一次错误，每次尝试前清空上一次残留状态
---
# 同步重试循环（controller / relay 层）

> 本 plan 碰到计费相关状态（预扣、结算、工具调用计数）。实现与测试前必须用读文件工具完整读一遍 `.agents/rules/billing.md`，并遵守其中全部规则。

## 目标
同步请求失败后换渠道，直到成功或所有渠道和密钥都试过；全部失败时返回最后一次尝试的错误；第二次尝试不会带着第一次尝试留下的状态。

## 业务规则（来源：PRD 01-sync-request-failover、02-async-retry-setting）
- 同步请求（`controller/relay.go` 的 `Relay` 循环，覆盖 OpenAI、Claude、Gemini、Responses、向量、图片、语音、重排、Realtime）不受 `common.RetryTimes` 限制；能不能换渠道仍由 `service.DecideRelayRetry` 判定（已输出、严格会话、钉死渠道、跳过重试标记、状态码列表规则都不变）。同步调用方传给它的剩余次数恒为 1，表示同步请求没有次数预算；"预算用尽"只会出现在异步任务。
- 循环开始时在 context 上设置"会换渠道"探针（`helper.SetFailoverProbe`）：给定一次尝试的错误，`service.DecideRelayRetry` 判为重试、并且 `service.HasFailoverChannel` 为真时返回真（PRD C17：只有确定会换渠道时，流式 handler 才扣下正文前的失败；否则按改动前的方式把失败发给客户端并照常计费）。
- 循环开始时调用 `service.StartFailoverLedger` 补记第一次的渠道和密钥；之后每次重试用 `service.PickFailoverChannel` 选渠道，再走现有的分组倍率刷新、`BeginAttempt`、`SetupContextForSelectedChannel`。
- `PickFailoverChannel` 返回"试完了"：退出循环，保留上一次尝试的错误返回给客户端，不改成"没有可用渠道"，也不再记新的决策事件。第一次就选不到渠道的报错（分发中间件里）照旧。
- 每次重试前清空上一次尝试留下、会被下一次读到的状态：`ContextKeyStreamOutputSent`（只有上一次没输出才会走到重试，清空是安全的）、plan 04 的开场事件队列与正文前失败记录、`relayInfo.ClaudeConvertInfo`、`relayInfo.ResponsesUsageInfo.BuiltInTools`、`relayInfo.ReceivedResponseCount`、context 键 `claude_signature_buffer`。`FirstResponseTime` 不清（首字时间仍按请求计）。
- 计费：预扣在循环前一次（`controller/relay.go:190`），失败的尝试由 handler 返回错误、不结算；全部失败后由现有 defer 退预扣。本 plan 不改预扣、结算、退款逻辑，只保证上面这些计数不跨尝试累加（PRD C16）。
- Responses WebSocket（`relay/responses_websocket.go` 建连循环）：同样不受 `common.RetryTimes` 限制，建连失败后用尝试记录与 `PickFailoverChannel` 选下一个渠道，试完后返回最后一次建连错误；连上之后的行为不变。
- 异步任务循环（`controller/relay.go` 的 `RelayTask`，约 :672）不改，仍按 `common.RetryTimes`。
- 重试日志 `retry: 1->2->3`（`use_channel`）照旧输出。

## 涉及文件
- 修改 `controller/relay.go`
- 修改 `relay/responses_websocket.go`
- 修改 `relay/common/relay_info.go`
- 修改 `controller/relay_stream_test.go`、`controller/channel_pin_retry_test.go`、`relay/responses_websocket_test.go`、`service/relay_error_test.go`（测试，Red 阶段；其中锁定"同步请求按 RetryTimes 停止"的旧断言改成新行为）

## 函数清单
### controller/relay.go
| 函数名 | 职责 |
|---|---|
| `Relay` | 修改：循环改为直到成功、不可重试或试完；启动尝试记录；每次重试前清空残留状态；试完时保留最后错误 |
| `getChannel` | 修改：同步请求重试时改调 `service.PickFailoverChannel`，把"试完了"作为单独结果交回 `Relay`；异步任务调用保持原逻辑 |

### relay/common/relay_info.go
| 函数名 | 职责 |
|---|---|
| `(*RelayInfo) ResetForFailoverAttempt` | 清空上面列出的、属于 relayInfo 的跨尝试残留字段 |

### relay/responses_websocket.go
| 函数名 | 职责 |
|---|---|
| 建连重试循环所在函数（约 :342） | 修改：建连失败后按尝试记录换渠道，试完返回最后一次建连错误 |

## 协作关系
- 调 plan 02 的 `StartFailoverLedger`、`PickFailoverChannel`；调 plan 04 提供的清空开场事件状态的函数（`helper.ResetStreamHold`）。
- 仍调 `service.DecideRelayRetry`、`service.RecordPolicyFailure`、`processChannelError`、`service.AppendUsedChannel`。

## 验证方式
- 测试入口：`go test ./controller/ -run 'Relay|Affinity|RoutingDatabaseMatrix'`、`go test ./relay/ -run ResponsesWebsocket`、`go test ./service/ -run DecideRelayRetry`；用例加进上面列出的已有测试文件，用 httptest mock 上游渠道。
- 测试输入与预期：
  - `common.RetryTimes=0`，模型 m 有订阅 A、B（优先级 0）、OpenAI C（10）、D（5），A、B、C 返回 500，D 返回 200 → 客户端拿到 D 的结果；mock 记录每个渠道 1 次，C 在 D 前。
  - 四个都返回 500，D 的错误消息 `upstream D down` → 客户端状态码 500，消息含 `upstream D down`，不含"no available channel"。
  - 只有一个单密钥渠道 E 返回 500，`RetryTimes=3` → E 被请求 1 次。
  - A 返回 400（默认状态码列表）→ 只请求 A；把 400 加进自动重试状态码 → 换到 B。
  - 令牌钉死 A、A 返回 500 → 只请求 A。
  - 第一次尝试把 `BuiltInTools` 记了 1 次、`ClaudeConvertInfo.Done` 置真后失败，第二次成功并记 2 次 → 结算按 2 次，第二次的 Claude 流包含 `message_start`。
  - 只有一个渠道、按次固定价，上游 `response.created` 后接 `response.failed` → 客户端收到 HTTP 200 流和这两个事件，按次收费一次（`controller/responses_websocket_test.go` 的 `TestResponsesStreamOutcomesPreserveAccounting`、`controller/channel_pin_retry_test.go` 的 `TestRelayChannelAffinityAfterFinalError` 保持改动前的断言并通过）；两个渠道 A 这样失败、B 成功 → 客户端只收到 B 的事件，A 不收费。
  - 异步任务：`RetryTimes=1`，3 个渠道前 2 个失败 → 共请求 2 次后报错（与改动前一致）。
  - Responses WebSocket：前两个渠道建连失败、第三个成功 → 连接成功；`RetryTimes=0` 时也一样。
- [ ] 上述用例通过；`TestChannelAffinityRetryPriorities`、`TestRequestPolicyRoutingDatabaseMatrix` 按新行为更新后在 SQLite、MySQL、PostgreSQL 上通过，缓存开关两遍
- [ ] `go build ./...`、`go test ./controller/ ./relay/ ./service/` 全绿
