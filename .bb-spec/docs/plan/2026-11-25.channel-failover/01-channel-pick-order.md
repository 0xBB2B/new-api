---
name: 01-channel-pick-order
description: 渠道候选排序改为先订阅渠道后其他，并新增按"已试完"排除渠道、按已试下标排除密钥的选取函数
---
# 渠道排序与排除式选取（model 层）

## 目标
model 层能按"订阅渠道在前、其次优先级高在前、同档按权重随机"挑一个渠道，并能跳过调用方认定已经试完的渠道；多密钥渠道能跳过调用方给出的已试密钥下标。

## 业务规则（来源：PRD 01-sync-request-failover）
- 订阅渠道 = 渠道类型 `constant.ChannelTypeCodex`（57，界面名 ChatGPT Subscription (Codex)）或 `constant.ChannelTypeClaudeSubscription`（64）。Sub2API（59）和其他类型都不算。
- 同一分组、同一模型的候选，先排订阅渠道，再排其他渠道；两类内部各按优先级从高到低分档，同档按权重随机（沿用现有的权重平滑规则）。
- 现有按"第 N 次重试取第 N 档"的选取函数 `GetRandomSatisfiedChannel` 保留给异步任务和智能路由使用，它的"档"改成"（是否订阅，优先级）"组合档：订阅渠道的各档整体排在其他渠道之前。订阅渠道不提供异步任务的模型，异步任务的结果实际不变。
- 新的排除式选取：调用方传入一个"这个渠道是否已试完"的判断，函数从第一个还有未试完渠道的档里按权重随机挑一个；全部试完时返回 nil、不报错。
- 过滤条件（`filterCandidateIDs` / `filterAbilitiesByConstraints`）、找不到精确模型名时回退归一模型名的逻辑，两个函数都照旧。
- 内存缓存关闭的路径（`GetChannel`，查 abilities 表）**不改 SQL 查询本身**，排序在 Go 里做，避免改动数据库行为。
- 多密钥：新增可跳过已试下标的取密钥方法；随机模式在"启用且未试"的下标里随机，轮询模式从保存的轮询位置往后找第一个"启用且未试"的下标，并照旧推进轮询位置；没有"启用且未试"的下标时返回一个专门的"本次已试完"结果，**不得**返回 `ErrorCodeChannelNoAvailableKey`（这个错误码以 `channel:` 开头，会被当成渠道错误触发自动禁用，见 `service/channel.go:72-74`）。原 `GetNextEnabledKey` 行为不变，供其他 7 处调用方使用。

## 涉及文件
- 修改 `model/channel_cache.go`
- 修改 `model/ability.go`
- 修改 `model/channel.go`
- 修改 `model/channel_priority_test.go`（测试，Red 阶段）

## 函数清单
### model/channel.go
| 函数名 | 职责 |
|---|---|
| `IsSubscriptionChannelType` | 判断渠道类型是否为订阅渠道（57、64） |
| `(*Channel) GetNextUntriedEnabledKey` | 按多密钥模式取一个"启用且不在已试下标集合里"的密钥及下标；单密钥渠道在下标 0 已试时返回"已试完" |
| `(*Channel) EnabledKeyIndexes` | 返回当前启用的密钥下标（单密钥渠道为 [0]），供上层判断渠道是否试完 |

### model/channel_cache.go
| 函数名 | 职责 |
|---|---|
| `GetRandomSatisfiedChannel` | 修改：档的划分改为（是否订阅，优先级），订阅在前 |
| `GetUntriedSatisfiedChannel` | 新增：同样的候选与过滤，按档顺序跳过已试完渠道，在第一个有剩余的档里按权重随机；缓存关闭时转到 ability.go 的对应函数 |
| `pickWeightedChannel` | 新增：从同一档的渠道里按权重（含现有平滑规则）随机挑一个，供上面两个函数和 ability.go 共用 |

### model/ability.go
| 函数名 | 职责 |
|---|---|
| `GetChannel` | 修改：查询结果在 Go 里按（是否订阅，优先级）分档后再按 retry 取档 |
| `GetUntriedChannel` | 新增：缓存关闭时的排除式选取，复用 `GetChannel` 现有的查询与过滤 |

## 协作关系
- `GetUntriedSatisfiedChannel` 由 plan 02 的 `service.PickFailoverChannel` 调用；"是否已试完"的判断由 plan 02 的尝试记录提供。
- `GetNextUntriedEnabledKey`、`EnabledKeyIndexes` 由 plan 02 的 `middleware.SetupContextForSelectedChannel` 与尝试记录调用。
- 依赖内存渠道缓存 `group2model2channels`、`channelsIDM` 与 abilities 表现有查询。

## 验证方式
- 测试入口：`go test ./model/ -run 'Priority|Untried|Subscription'`，扩展 `model/channel_priority_test.go`（沿用其现有夹具建渠道与缓存；缓存开、关两种都跑）。
- 测试输入与预期：
  - 分组 g、模型 m：Claude 订阅渠道 A（优先级 0）、ChatGPT 订阅渠道 B（优先级 0）、OpenAI 渠道 C（优先级 10）、OpenAI 渠道 D（优先级 5）、Sub2API 渠道 S（优先级 20）。
  - `GetRandomSatisfiedChannel(g, m, 0, nil)` 只会返回 A 或 B；retry=1 返回 S；retry=2 返回 C；retry=3 返回 D；retry=9 仍返回 D。
  - `GetUntriedSatisfiedChannel`：判断函数标 A、B 已试完 → 返回 S；再标 S、C → 返回 D；全部标完 → 返回 nil 且 err 为 nil。
  - 只有非订阅渠道时，`GetRandomSatisfiedChannel` 各 retry 的返回与改动前一致（异步任务不受影响）。
  - 多密钥渠道 3 个密钥、下标 1 已禁用：已试 {0} → 只会返回下标 2；已试 {0,2} → 返回"已试完"，错误码不是 `ErrorCodeChannelNoAvailableKey`；轮询模式下返回后轮询位置前进。
  - `EnabledKeyIndexes` 返回 [0,2]；单密钥渠道返回 [0]。
- [ ] 缓存开、关两种模式结果一致
- [ ] `IsSubscriptionChannelType(57)`、`(64)` 为真，`(59)`、`(1)` 为假
- [ ] `go build ./...` 通过；`controller/channel_pin_retry_test.go` 的 `TestRequestPolicyRoutingDatabaseMatrix`、`TestChannelAffinityRetryPriorities` 在 SQLite、MySQL、PostgreSQL 上跑通（本 plan 不改 SQL，但改了 GetChannel 的分档，按 AGENTS.md 用现成三库矩阵确认）
