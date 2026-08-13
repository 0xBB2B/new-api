---
name: subscription-usage-cache
description: model 用量缓存泛化为瓶颈使用率单值条目并更名订阅语义 API；constant 新增订阅渠道类型谓词。
---

# 订阅渠道用量缓存泛化

## 目标

model 包提供以「瓶颈使用率单值 + 刷新时间」为条目的订阅渠道用量缓存，constant 包提供订阅渠道类型谓词，供轮询与选择两侧共用。

## 业务规则（来源：spec polling / saturation-eviction / weighted-selection）

- 订阅渠道 = type 57（Codex）与 type 61（Claude Subscription）两种渠道类型的统称。
- 缓存条目结构为「瓶颈使用率单值 + 刷新时间戳」；写入前瓶颈使用率钳制到 [0, 100]。
- 条目过期语义：距最近一次成功刷新超过 10 分钟即过期；过期判定由读取方执行，缓存不删除过期条目。
- 触顶 = 瓶颈使用率 ≥ 95；缓存缺失的渠道不判定触顶；未曾触顶的过期条目不判定触顶；曾触顶渠道在最近触顶后 10 分钟内即使条目过期也继续判定触顶。
- 剩余比例 = (100 − 瓶颈使用率)/100，条目缺失或过期时返回不可用。
- 渠道从未触顶变为触顶、从触顶回归时，各记录一条系统日志（含渠道 ID 与当时瓶颈使用率），措辞用订阅渠道通用语义。
- 全量快照 JSON 支持导出与加载往返：加载后触顶判定、剩余比例、过期判定（按条目自带刷新时间戳）与导出前一致。
- 本分支未发布，旧 codex 命名与 5h/7d 两字段结构直接替换，不留任何兼容层。

## 涉及文件

- 修改（更名重写）`model/codex_usage_cache.go` → `model/subscription_usage_cache.go`
- 修改（更名重写）`model/codex_usage_cache_test.go` → `model/subscription_usage_cache_test.go`
- 修改 `constant/channel.go`

## 函数清单

### constant/channel.go

| 函数名 | 职责 |
|---|---|
| `IsSubscriptionChannel` | 判定渠道类型是否属于参与用量均衡的订阅渠道（57、61） |

### model/subscription_usage_cache.go

| 函数名 | 职责 |
|---|---|
| `CacheSetSubscriptionChannelUsage` | 写入某渠道瓶颈使用率（钳制 [0,100]）与刷新时间；检测触顶状态迁移并记系统日志 |
| `CacheIsSubscriptionChannelSaturated` | 判定某渠道当前是否触顶（含曾触顶后 10 分钟保护窗语义） |
| `subscriptionChannelRemainingRatio` | 返回某渠道剩余比例；条目缺失或过期返回不可用 |
| `subscriptionChannelUsageEntryFresh` | 判定条目距最近成功刷新是否在 10 分钟内 |
| `SubscriptionChannelUsageSnapshotJSON` | 导出全部条目为 JSON 全量快照（瓶颈使用率 + 刷新时间戳毫秒） |
| `CacheLoadSubscriptionChannelUsageSnapshotJSON` | 解析快照 JSON 并整体替换本地缓存 |

原 `codexChannelUsageBottleneck` 随两字段结构一并删除（单值条目无瓶颈计算）；快照条目字段更名为 `bottleneck_percent` + `refreshed_at_unix_ms`。JSON 编解码走 `common.Marshal` / `common.Unmarshal`。

## 协作关系

- `CacheSetSubscriptionChannelUsage` 由轮询任务调用；`CacheIsSubscriptionChannelSaturated` 与 `subscriptionChannelRemainingRatio` 由选择侧（ability / channel_cache / distributor）调用；快照导出/加载由 master 写 Redis、非 master 同步任务调用。
- `constant.IsSubscriptionChannel` 由 model 选择侧、middleware 亲和检查、service 轮询任务共用。
- 无 DB、无外部接口依赖；纯进程内存 + 锁。

## 验证方式

- 测试入口：`go test ./model -run 'SubscriptionChannelUsage|SubscriptionUsage'`（包内测试直接调用上述导出与非导出函数）；`go test ./constant` 验证类型谓词。
- 测试输入：显式构造缓存写入序列（含钳制越界值 120/−5、阈值边界 94.9/95.0、过期时间戳、触顶后刷新失败序列）、快照 JSON 字节。
- 预期结果：
  - 写入 120 后触顶判定为真且快照导出值为 100；写入 −5 导出为 0。
  - 瓶颈 95.0 判定触顶，94.9 不触顶。
  - 条目刷新时间在 10 分钟内、瓶颈 90 → 剩余比例 0.10 可用；超过 10 分钟 → 不可用。
  - 曾触顶条目过期后 10 分钟内仍触顶，最近触顶超过 10 分钟且无新刷新不再触顶。
  - 快照导出→加载往返后触顶判定、剩余比例、过期判定与导出前一致；非法渠道 ID key 的快照加载报错且不替换本地缓存。
  - `IsSubscriptionChannel`：57、61 为真，其余（含 0、14、负值）为假。
- [ ] 上述用例全绿（testify，表驱动 + t.Run）
- [ ] `go build ./...` 通过（本 plan 完成时 model/constant 无旧引用残留由 03/04 收口）
