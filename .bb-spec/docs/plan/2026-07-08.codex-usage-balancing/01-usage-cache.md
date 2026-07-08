---
name: usage-cache
description: model 包进程级 Codex 渠道用量缓存：写入钳制与触顶迁移日志、读取剩余比例与触顶判定、5 分钟过期。
---

# Codex 渠道用量缓存

## 目标

model 包内有一份进程级缓存，保存每个 Codex 渠道（type 57）最近一次成功抓取的 5h/7d 使用率，供选择路径（同包）读取、service 轮询任务（跨包）写入。

## 业务规则（来源：spec usage-polling / saturation-eviction）

- 缓存条目 = 5 小时窗口 used_percent + 7 天窗口 used_percent + 最近成功刷新时间戳；used_percent 写入前钳制到 [0, 100]
- 距最近一次成功刷新超过 5 分钟即过期；过期判定由读取方执行，写入方不删除过期条目
- 触顶判定：瓶颈使用率 max(5h, 7d) ≥ 95 即触顶；缓存缺失或过期的渠道**不判定**触顶
- 渠道从未触顶变为触顶、从触顶回归时，各记一条服务端系统日志，内容含渠道 ID 与当时使用率
- 不引入新的持久化字段，纯内存

## 涉及文件

- `model/codex_usage_cache.go` — 新建
- `model/codex_usage_cache_test.go` — 新建

## 函数清单

### model/codex_usage_cache.go

包级私有状态：`map[int]` 条目 + 独立 `sync.RWMutex`（沿用 `channelsIDM` + `channelSyncLock` 先例，不复用其锁）；阈值 95、过期 5 分钟、tick 无关常量均定义在此。

| 函数名 | 职责 |
|---|---|
| `CacheSetCodexChannelUsage` | 导出写入：钳制两窗口值到 [0,100]，记录当前时间为刷新时间戳；对比写入前后触顶状态，发生迁移时用 `common.SysLog` 记「触顶/回归 + 渠道 ID + 瓶颈使用率」 |
| `CacheIsCodexChannelSaturated` | 导出触顶判定：条目存在且未过期且 max(5h,7d) ≥ 95 返回 true；缺失/过期一律 false（供 middleware 亲和路径与选择路径共用） |
| `codexChannelRemainingRatio` | 包内读取：条目新鲜时返回 (100 − max(5h,7d))/100 与 ok=true；缺失/过期返回 ok=false（选择路径据此回退静态权重） |

## 协作关系

- 写入方：service 轮询任务（02-usage-poll-task）调用 `CacheSetCodexChannelUsage`
- 读取方：`GetRandomSatisfiedChannel` / `GetChannel`（03-balanced-selection，同包直调私有函数）、`middleware/distributor.go` 亲和校验（调导出的 `CacheIsCodexChannelSaturated`）
- 无 DB / Redis / 第三方依赖；日志走 `common.SysLog`（model 包既有风格）

## 验证方式

- [ ] 写入 (5h=120, 7d=-3) 后读出为 (100, 0)
- [ ] 写入 (5h=95.0, 7d=10) 判定触顶；(5h=94.9, 7d=10) 不触顶
- [ ] 未写入过的渠道 ID：不触顶、remaining 返回 ok=false
- [ ] 刷新时间戳超过 5 分钟的条目：不触顶、remaining 返回 ok=false（测试通过注入时钟或可设时间戳的内部写入函数实现，禁 sleep）
- [ ] 从 (5h=50) 写入 (5h=96)：产生一条触顶日志；再写入 (5h=10)：产生一条回归日志；连续同态写入不重复记日志
- [ ] 测试文件用 testify（require/assert），确定性精确断言
