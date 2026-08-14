---
name: usage-endpoint
description: 管理端只读端点 GET /api/channel/subscription_usage：model 导出用量快照（含服务端 saturated 判定）+ controller + 路由注册。
---

# 管理端订阅用量只读端点

## 目标

管理员经 `GET /api/channel/subscription_usage` 一次拿到全部订阅渠道的瓶颈使用率、最后刷新时间与触顶判定，全程只读本节点进程内缓存。

## 业务规则（来源：spec admin-usage-endpoint）

- 端点 `GET /api/channel/subscription_usage`，权限与渠道列表一致（AdminAuth + 渠道读权限），无请求参数。
- 响应走管理端 `{success, message, data}` 信封，成功 HTTP 200；未认证/非管理员被拒绝，行为与渠道列表接口一致。
- `data` 为对象 map：键为渠道 ID 十进制字符串，值含三字段——`bottleneck_percent`（float，[0,100]）、`refreshed_at`（最近一次成功刷新的 Unix 毫秒时间戳）、`saturated`（bool）。
- `saturated` 是服务端唯一权威判定，语义与渠道选路完全同源：距最近成功刷新 ≤10 分钟（新鲜）且瓶颈使用率 ≥95 为 true。因 `refreshedAt` 只在成功刷新时更新，该判定天然覆盖「曾触顶渠道在最近触顶后 10 分钟内即使刷新失败仍视为触顶；超 10 分钟自动失效」；从未触顶的过期条目为 false。
- `data` 覆盖缓存全部条目（含禁用渠道残留与过期条目）；缓存为空时 `data` 为 `{}`（非 `null`）。
- 端点不读数据库、不读 Redis、零写副作用；响应中不出现渠道凭据等敏感字段。

## 涉及文件

- `model/subscription_usage_cache.go` — 修改（新增导出视图类型与快照函数）
- `model/subscription_usage_cache_test.go` — 修改（新增快照函数测试）
- `controller/subscription_usage.go` — 新建
- `controller/subscription_usage_test.go` — 新建
- `router/channel-router.go` — 修改（`channelPermissionRoutes` 切片加一行）

## 函数清单

### model/subscription_usage_cache.go

| 函数/类型 | 职责 |
|---|---|
| `SubscriptionChannelUsageView`（导出 struct） | 端点响应条目：json tag `bottleneck_percent` / `refreshed_at` / `saturated`；与 Redis 快照类型 `subscriptionChannelUsageSnapshotEntry`（线格式 `refreshed_at_unix_ms`）互相独立，禁止合并或改动后者 |
| `SubscriptionChannelUsageOverview` | 持锁（RLock）遍历缓存，逐条就地计算 saturated（复用 `subscriptionChannelUsageEntryFresh` 与 `subscriptionChannelUsageSaturationThreshold`，不逐条二次取锁），键为 `strconv.Itoa(channelID)`；缓存为 nil/空时返回已初始化的空 map |

### controller/subscription_usage.go

| 函数 | 职责 |
|---|---|
| `GetSubscriptionChannelUsage` | 调 `model.SubscriptionChannelUsageOverview`，`common.ApiSuccess(c, ...)` 返回；无参数解析、无错误分支 |

### router/channel-router.go

无新函数：`channelPermissionRoutes` 切片新增条目 `{GET, "/subscription_usage", authz.ChannelRead, controller.GetSubscriptionChannelUsage}`，放在 `/ops` 之后、`/:id` 之前，与既有静态段（`/search`、`/models`、`/ops`）排布一致。

## 协作关系

- `GetSubscriptionChannelUsage` → `model.SubscriptionChannelUsageOverview` → 进程内 `subscriptionChannelUsageCache`（RWMutex 读锁）。
- 缓存写入方不变：master 轮询任务与非 master Redis 快照同步（既有代码，本 plan 不触碰）。
- 无 DB、无 Redis、无上游 HTTP 依赖。

## 验证方式

- 测试入口：HTTP `GET /api/channel/subscription_usage`（管理员鉴权后），以及包级可调用的 `model.CacheSetSubscriptionChannelUsage`（写入测试数据）与 `model.SubscriptionChannelUsageOverview`。
- 测试输入：经 `CacheSetSubscriptionChannelUsage` 预置若干渠道条目；模拟条目陈旧化（既有测试 helper 可将条目刷新时间前移）。
- 预期结果：
  - 预置渠道 7（62.4，刚刷新）与渠道 12（96.2，刚刷新）后请求端点：HTTP 200，`success=true`，`data["7"]` = `{bottleneck_percent:62.4, refreshed_at:<毫秒时间戳>, saturated:false}`，`data["12"].saturated=true`。
  - 渠道 12 条目陈旧 6 分钟（无新刷新）：`saturated=true`；陈旧 11 分钟：`saturated=false`。
  - 瓶颈 41、陈旧 11 分钟（从未触顶的过期条目）：`saturated=false`，条目仍出现在 `data` 中。
  - 缓存为空：`data` 序列化为 `{}` 而非 `null`。
  - 请求端点前后缓存内容不变（只读）。
- [ ] 上述五种场景各有断言精确期望值的测试（testify，table-driven 优先）。
- [ ] 响应 JSON 字段名恰为 `bottleneck_percent` / `refreshed_at` / `saturated`。
