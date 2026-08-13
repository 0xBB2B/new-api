---
name: balanced-selection
description: 选择侧泛化：5 处 ChannelTypeCodex 判定改订阅渠道谓词，触顶过滤与动态权重函数更名订阅语义。
---

# 选择侧订阅渠道泛化

## 目标

渠道选择的触顶移出与动态权重对 Codex 与 Claude 订阅两类渠道一致生效，对其他渠道类型零影响。

## 业务规则（来源：spec weighted-selection / saturation-eviction）

- 动态缩放仅作用于订阅渠道（type 57、61）：有效权重 = 静态等效权重（整数平滑后，含全零补偿、小权重放大）× (100 − 瓶颈使用率)/100，向下取整后最小为 1。
- 缓存缺失或过期（距最近成功刷新超过 10 分钟）的订阅渠道回退静态等效权重。
- 触顶（瓶颈 ≥ 95）的订阅渠道移出候选集，不参与层内加权随机；不改渠道 Status、不写数据库。
- 亲和缓存命中的目标渠道若已触顶，本次跳过粘滞改走正常选择，亲和缓存不清除。
- 优先级分层、失败重试降级、`auto` 跨分组语义不变；同层非订阅渠道权重在机制前后完全一致。
- 两类订阅渠道在相同缓存状态与静态权重下行为完全一致。

## 涉及文件

- 修改 `model/ability.go`
- 修改 `model/channel_cache.go`
- 修改 `middleware/distributor.go`
- 修改 `model/ability_test.go`、`model/channel_cache_test.go`、`middleware/distributor_test.go`

## 函数清单

### model/ability.go

| 函数名 | 职责 |
|---|---|
| `filterSaturatedSubscriptionAbilities` | 原 `filterSaturatedCodexAbilities` 更名：按订阅渠道谓词 + 触顶判定过滤 ability 候选 |

`GetChannel` 内两处调用点同步改名（过滤调用与 `subscriptionEffectiveWeight` 调用），类型判定由 `constant.IsSubscriptionChannel` 承担。

### model/channel_cache.go

| 函数名 | 职责 |
|---|---|
| `filterSaturatedSubscriptionChannels` | 原 `filterSaturatedCodexChannels` 更名：内存缓存路径的触顶过滤 |
| `subscriptionEffectiveWeight` | 原 `codexEffectiveWeight` 更名：非订阅渠道原权重直返；订阅渠道按剩余比例缩放，下取整后 ≥1，缓存不可用回退原权重 |

### middleware/distributor.go

| 位置 | 改动 |
|---|---|
| 亲和命中检查（现 `preferred.Type == constant.ChannelTypeCodex && model.CacheIsCodexChannelSaturated(...)`） | 改为 `constant.IsSubscriptionChannel(preferred.Type) && model.CacheIsSubscriptionChannelSaturated(...)` |

## 协作关系

- 依赖 plan 01 的 `constant.IsSubscriptionChannel`、`CacheIsSubscriptionChannelSaturated`、`subscriptionChannelRemainingRatio`。
- 与 plan 03 无代码依赖（选择只读本地缓存，不触轮询）。
- 外部依赖：无新增；沿用既有 ability 查询、渠道内存缓存与亲和缓存路径。

## 验证方式

- 测试入口：`go test ./model -run 'Ability|ChannelCache'` 与 `go test ./middleware -run Distributor`（既有测试文件内改造 + 新增用例）。
- 测试输入：显式 fixture——同层混布 type 57 / type 61 / 普通类型渠道，缓存状态覆盖（新鲜低用量、新鲜高用量、触顶 95.0、边界 94.9、过期、缺失）。
- 预期结果：
  - 静态权重相同、缓存瓶颈 20 与 80 的两订阅渠道有效权重比 80:20；type 57 与 type 61 在相同状态下有效权重相等。
  - 触顶（95.0）渠道不出现在 DB 路径与内存缓存路径的候选中；94.9 正常参与。
  - 缓存缺失/过期渠道有效权重等于静态等效权重；瓶颈 94 且小权重时下取整后仍 ≥1。
  - 同层普通渠道的有效权重与机制引入前数值完全一致。
  - 亲和命中的触顶订阅渠道（含 type 61）被跳过且请求落到其他渠道，亲和缓存未被清除。
- [ ] 上述用例全绿（testify，表驱动 + t.Run）
- [ ] `go build ./...` 通过，仓库无 `filterSaturatedCodex*`/`codexEffectiveWeight`/`CacheIsCodexChannelSaturated` 旧符号残留
