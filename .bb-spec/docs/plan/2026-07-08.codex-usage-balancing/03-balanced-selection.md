---
name: balanced-selection
description: 渠道选择两条路径对 Codex 渠道注入触顶过滤 + 剩余额度动态权重；亲和路径触顶跳过且不清亲和缓存。
---

# 用量加权选择与触顶过滤

## 目标

渠道选择时 Codex 渠道（type 57）按剩余额度动态缩放权重、触顶渠道移出候选；其他类型渠道行为与现状完全一致。

## 业务规则（来源：spec usage-weighted-selection / saturation-eviction）

- Codex 渠道使用率 = max(5h, 7d used_percent)（读缓存）；有效权重 = 现行整数平滑后的等效权重 × (100 − 使用率)/100，向下取整、下限 1
- 缓存缺失或过期（>5 分钟）→ 回退现行静态等效权重，行为与无本机制时一致
- 触顶（使用率 ≥ 95）渠道移出候选，不参与层内加权随机；不改渠道 Status、不触发 auto-ban
- 层内全部 Codex 触顶：有其他渠道则在其余渠道中选；整层空按既有重试语义降级下一优先级
- 亲和命中的目标渠道已触顶：本次跳过粘滞走正常选择，**亲和缓存不清除**
- 优先级分层、失败重试、`auto` 跨分组语义不变

## 涉及文件

- `model/channel_cache.go` — 修改（内存路径）
- `model/ability.go` — 修改（DB 直查路径）
- `middleware/distributor.go` — 修改（亲和校验）
- `model/channel_cache_test.go`、`model/ability_test.go` — 增补用例（文件已存在则增补，不存在则新建）

## 函数清单

### model/channel_cache.go

| 函数名 | 职责 |
|---|---|
| `GetRandomSatisfiedChannel` | 修改：① 候选过滤阶段（`filterChannelsByRequestPath` 同层）剔除触顶 Codex 渠道——必须在优先级收集**之前**，防止「目标层全触顶 → targetChannels 空 → rand.Intn(0) panic」，且层内全触顶时自然落到下一优先级；② 加权随机段：对 Codex 渠道把 (GetWeight()×smoothingFactor + smoothingAdjustment) 乘以剩余比例、向下取整、下限 1，totalWeight 改为各渠道有效权重之和（不再是 sumWeight×smoothingFactor）；平滑参数（smoothingFactor/smoothingAdjustment）仍按过滤后候选的原始权重和计算 |
| `codexEffectiveWeight` | 新增（本文件私有）：输入平滑后等效权重与渠道 ID，返回缩放后的有效权重（缓存缺失/过期原样返回；下限 1）——两条选择路径共用，并承接精确值断言测试 |

### model/ability.go

| 函数名 | 职责 |
|---|---|
| `GetChannel` | 修改：`filterAbilitiesByRequestPath` 之后剔除触顶 Codex 渠道（渠道类型来自该过滤函数已查出的 channels 结果，顺带得到 id→type 映射，不加查询）；剔空则返回「无可用渠道」错误，沿用既有重试降级；加权随机段对 Codex 渠道把 (Weight+10) 经 `codexEffectiveWeight` 缩放，weightSum 同步改为有效权重之和 |

### middleware/distributor.go

| 函数名 | 职责 |
|---|---|
| `Distribute` | 修改亲和校验条件（现有「Status 启用 + 路径支持」处）：追加 `!model.CacheIsCodexChannelSaturated(preferred.Id)`；触顶导致的不可用**跳过** `ClearCurrentChannelAffinityCache` 分支（用局部标记区分「渠道失效」与「触顶暂避」两种不可用原因） |

## 协作关系

- 读取：`codexChannelRemainingRatio` / `CacheIsCodexChannelSaturated`（01-usage-cache 产物；model 内部用私有函数，middleware 用导出函数）
- 渠道类型判定：`channel.Type == constant.ChannelTypeCodex`
- 不新增查询、不新增持久化；`filterChannelsByRequestPath` 的「调用者持读锁、缓存 slice 不可变」约束保持——剔除触顶须产生新 slice，禁原地改缓存数组

## 验证方式

- [ ] `codexEffectiveWeight` 精确断言：权重 100 + usage 60 → 40；usage 94 + 权重 10 → 下限保护后 ≥1；缓存缺失 → 原值
- [ ] 内存路径确定性用例：同层两渠道，Codex 渠道触顶 → 恒选另一渠道；层内唯一渠道触顶 → 返回空/错误（不 panic）
- [ ] DB 路径同上两用例（sqlite 测试夹具，参考 `model/model_owner_test.go` 的渠道/ability 构造）
- [ ] 非 Codex 渠道在缓存有数据时权重不受影响（对照断言）
- [ ] 亲和用例：preferred 渠道触顶 → 选择落到其他渠道且亲和缓存键仍存在
- [ ] `go test ./model/... ./service/... ./middleware/...` 全绿
