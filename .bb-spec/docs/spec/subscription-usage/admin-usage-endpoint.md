---
name: admin-usage-endpoint
description: 管理端只读端点 GET /api/channel/subscription_usage：读本节点用量缓存，返回各订阅渠道瓶颈使用率、刷新时间与触顶判定。
---

# 管理端订阅用量只读端点

## 目的

让管理员在不翻服务端日志的前提下看到每个订阅渠道的瓶颈使用率、数据最后刷新时间与是否触顶。

## 逻辑

新增 `GET /api/channel/subscription_usage`，权限门槛与渠道列表一致（管理员 + 渠道读权限）。端点只读本节点进程内的渠道用量缓存，把全部缓存条目一次性返回；不发起任何上游请求，不产生任何写副作用。

响应 `data` 为对象 map，键是渠道 ID 的十进制字符串，值为：

- `bottleneck_percent`：瓶颈使用率，浮点数，取值 [0, 100]。
- `refreshed_at`：该条目最近一次成功刷新的 Unix 毫秒时间戳。
- `saturated`：是否触顶，布尔值，由服务端按与渠道选择完全同源的判定逻辑计算（阈值 95 + 10 分钟新鲜窗：最近一次成功刷新显示触顶起 10 分钟内持续视为触顶，超窗自动失效）。

## 约束

- 请求为 GET，无参数；响应遵循管理端 `{success, message, data}` 信封，成功时 HTTP 200。
- 未认证或非管理员请求被拒绝，行为与渠道列表接口一致。
- `data` 覆盖缓存中的全部条目（含已禁用渠道的残留条目与已过期条目）；缓存为空时 `data` 为空对象 `{}`。
- `saturated` 是服务端判定的唯一权威输出；消费方不得用 `bottleneck_percent >= 95` 自行重算。
- 端点不读数据库、不读 Redis，只读本节点内存缓存；非 master 节点的数据来自 Redis 快照同步，相对 master 最多滞后约 30 秒。Redis 未启用时非 master 节点缓存为空，端点返回空 `data`，与该节点用量均衡不生效的现状一致。
- 端点响应中不出现渠道凭据或其他渠道敏感字段。

## 例子

缓存中有渠道 12（瓶颈 96.2，3 分钟前刷新）与渠道 7（瓶颈 62.4，30 秒前刷新）。请求返回：

```json
{
  "success": true,
  "message": "",
  "data": {
    "7":  { "bottleneck_percent": 62.4, "refreshed_at": 1765000230000, "saturated": false },
    "12": { "bottleneck_percent": 96.2, "refreshed_at": 1765000080000, "saturated": true }
  }
}
```

下一轮轮询渠道 12 刷新为 41 后，再次请求该条目变为 `"bottleneck_percent": 41, "saturated": false`。

## 验收

- [ ] 管理员请求返回缓存全部条目，字段为 bottleneck_percent / refreshed_at / saturated。
- [ ] 未认证请求与非管理员请求被拒绝，行为与渠道列表接口一致。
- [ ] 缓存为空时返回 success=true 且 data 为 `{}`。
- [ ] 瓶颈使用率 96.2 且 3 分钟前刷新的渠道 saturated=true；刷新为 41 后 saturated=false。
- [ ] 最近一次成功刷新显示触顶、其后 8 分钟无新刷新的条目 saturated=true；从未触顶的过期条目 saturated=false。
- [ ] 请求端点前后，用量缓存内容与渠道表数据无任何变化。
