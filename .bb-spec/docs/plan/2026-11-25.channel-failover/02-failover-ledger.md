---
name: 02-failover-ledger
description: 同步请求的尝试记录（已试渠道与密钥）、排除已试密钥的选密钥、组内试完再按跨组设置换组的换渠道选取
---
# 尝试记录与换渠道选取（service / middleware 层）

## 目标
同步请求有一份"本次请求已经用过哪些（渠道，密钥下标）"的记录；换渠道时按记录跳过试完的渠道和密钥，auto 分组在组内试完后按令牌的跨组设置换组；没有可试的了就明确告诉调用方"试完了"。

## 业务规则（来源：PRD 01-sync-request-failover）
- 一次同步请求里，每个渠道的每个启用密钥最多用一次；单密钥渠道用过一次就算试完；多密钥渠道里每个当前启用的密钥都用过才算试完。请求中途被自动禁用的密钥不算启用，渠道被禁用则不会出现在候选里。
- 尝试记录只由同步请求创建。异步任务没有记录，选密钥、选渠道的行为与改动前完全一致。
- 第一次尝试由分发中间件选定（含非严格渠道亲和优先、订阅优先）；同步请求的重试循环开始时，把这次的（渠道，密钥下标）补记进记录。
- 换渠道顺序：在当前分组里用 plan 01 的排除式选取（订阅在前，再按优先级、同档按权重随机）。渠道还有未试密钥时可以被再次选中，取密钥时跳过已试下标。
- auto 分组：从当前分组开始找；当前分组没有可试的了，令牌开了跨组重试（`ContextKeyTokenCrossGroupRetry`）才往后换分组，跳过对这个模型没有渠道的分组，并更新 `ContextKeyAutoGroupIndex` / `ContextKeyAutoGroup`；没开就返回"试完了"。同步请求不再使用 `common.RetryTimes` 决定何时换组。
- 选到的渠道仍要满足请求的过滤条件（`GetChannelConstraints(c).Filters`）。
- "试完了"与"出错"分开返回，调用方据此保留上一次的上游错误（PRD C5）。

## 涉及文件
- 新建 `service/channel_failover.go`
- 修改 `middleware/distributor.go`
- 修改 `service/channel_select_auto_groups_test.go`（测试，Red 阶段）
- 修改 `middleware/distributor_test.go`（测试，Red 阶段）

## 函数清单
### service/channel_failover.go
| 函数名 | 职责 |
|---|---|
| `FailoverLedger`（类型） | 本次请求已用过的（渠道 id → 已试密钥下标集合） |
| `StartFailoverLedger` | 在 gin context 上创建记录，并补记分发中间件选定的渠道和密钥下标（读 `ContextKeyChannelId`、`ContextKeyChannelMultiKeyIndex`） |
| `GetFailoverLedger` | 读 context 上的记录；没有时返回 nil（异步任务） |
| `(*FailoverLedger) Record` | 记一次（渠道，密钥下标） |
| `(*FailoverLedger) TriedKeys` | 返回某渠道已试的密钥下标集合 |
| `(*FailoverLedger) ChannelExhausted` | 结合 `Channel.EnabledKeyIndexes` 判断渠道是否试完 |
| `PickFailoverChannel` | 按上面的分组与 auto 分组规则选下一个渠道，返回渠道、所选分组、是否试完、错误 |

### middleware/distributor.go
| 函数名 | 职责 |
|---|---|
| `SetupContextForSelectedChannel` | 修改：context 上有尝试记录时，改用 `GetNextUntriedEnabledKey` 跳过已试下标，并把所选下标记进记录；没有记录时照旧调用 `GetNextEnabledKey` |

## 协作关系
- `PickFailoverChannel` 调 plan 01 的 `model.GetUntriedSatisfiedChannel`，判断函数用 `ledger.ChannelExhausted`；auto 分组列表用现有 `GetRequestAutoGroups`。
- `StartFailoverLedger`、`PickFailoverChannel` 由 plan 03 的重试循环调用。
- `SetupContextForSelectedChannel` 已被分发中间件与 `controller/relay.go` 的 `getChannel` 调用。

## 验证方式
- 测试入口：`go test ./service/ -run 'AutoGroup|Failover'`（扩展 `service/channel_select_auto_groups_test.go`，沿用其 SQLite 内存库 + `InitChannelCache` 夹具）；`go test ./middleware/ -run 'Distributor|SetupContext'`（扩展 `middleware/distributor_test.go`）。
- 测试输入与预期：
  - 分组 default，模型 m：订阅 A、订阅 B（优先级 0）、OpenAI C（10）、D（5）。`StartFailoverLedger` 补记 A 后，连续调用 `PickFailoverChannel` 并模拟每次用过 → 依次得到 B、C、D，然后"试完了"为真、错误为 nil。
  - 多密钥订阅渠道 F 有 3 个启用密钥：记录里已有 F 的下标 0 → `PickFailoverChannel` 仍可返回 F，经 `SetupContextForSelectedChannel` 取到的下标不是 0；F 的 3 个下标都记过 → 不再返回 F。
  - auto 分组 g1（C、订阅 A）、g2（订阅 B），已用 A：开跨组 → 依次 C、B；关跨组 → C 之后"试完了"。g1 与 g2 之间插入一个对 m 没有渠道的分组 gx → 开跨组时跳过 gx。
  - 过滤条件排除了 B → 永远不返回 B。
  - context 上没有记录时（模拟异步任务）调用 `SetupContextForSelectedChannel` → 取密钥行为与改动前一致（轮询位置照旧推进）。
- [ ] 上述用例全部通过；`go test ./service/ ./middleware/` 全绿
