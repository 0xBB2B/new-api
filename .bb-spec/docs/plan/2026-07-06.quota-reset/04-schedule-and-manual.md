---
name: schedule-and-manual
description: 定时 ticker 检测本地 0 点边界跨越触发重置批次（错过不补），加管理员手动全员重置端点与审计。
---

# 定时调度与手动触发

## 目标

服务运行期间在每日/每周一/每月 1 日本地 00:00 自动执行重置批次；管理员可随时手动触发一轮全员重置作为停机补救或提前发放。

## 业务规则（来源：spec schedule-trigger / manual-trigger）

- 到达触发时点执行一次；同一触发时点对同一用户至多一次重置与一条日志。
- 服务在触发时点未运行（宕机、重启跨过时点）→ 该次不执行、恢复后不自动补，包括跨多个周期的停机。
- 手动触发仅管理员；执行范围与定时一致（遍历全体用户，按各自生效规则重置，无生效规则跳过）；日志触发方式记手动；不影响下一个定时时点；非管理员调用拒绝且零余额变更。

## 涉及文件

- `service/quota_reset_task.go` — 新建
- `main.go` — 修改（wiring 一行）
- `controller/user.go` — 修改（手动触发 handler）
- `controller/audit.go` — 修改（审计模板）
- `router/api-router.go` — 修改（adminRoute 加路由）

## 函数清单

### service/quota_reset_task.go（新建，样板：service/subscription_reset_task.go 的 ticker 骨架）

| 函数 | 职责 |
|---|---|
| `StartQuotaResetTask` | `sync.Once` + 仅 `common.IsMasterNode` 启动；gopool goroutine 每分钟 tick；启动时刻初始化内存 `lastTick`（重启后从启动时刻起算，天然实现"触发时点未运行则不执行"，不做任何持久化追赶） |
| `quotaResetTick` | 计算 `(lastTick, now]` 内跨越的边界集合（`crossedQuotaResetPeriods`），非空则调 `RunQuotaResetPass(periods, scheduled)`；执行完把 lastTick 推进到 now；`atomic.Bool` 防重入 |
| `crossedQuotaResetPeriods` | 纯函数：给定 (from, to] 两个本地时间，返回跨越的周期集合——跨过任一本地 00:00 → 含 daily；该 00:00 落在周一 → 含 weekly；落在 1 日 → 含 monthly；单 tick 跨多天也只返回集合（重置是覆盖写，无需逐边界重放） |

### controller/user.go（修改）

| 函数 | 职责 |
|---|---|
| `RunQuotaResetNow`（新增） | AdminAuth 路由后置 handler：调 `service.RunQuotaResetPass(nil, manual)`，同步等待；返回信封含重置人数；`recordManageAuditFor(c, 操作者, "user.quota_reset_run", {count})` 审计 |

### controller/audit.go（修改）

| 位置 | 职责 |
|---|---|
| `auditContentTemplates` | 加 `"user.quota_reset_run"` 英文兜底模板（含重置人数占位符） |

### router/api-router.go（修改）

| 位置 | 职责 |
|---|---|
| `adminRoute`（/api/user 管理区段） | 加 `POST /quota_reset/run` → `controller.RunQuotaResetNow`（AdminAuth 由分组中间件承担，非管理员被拒且无副作用） |

### main.go（修改）

| 位置 | 职责 |
|---|---|
| 启动序列（紧邻既有 `StartSubscriptionQuotaResetTask` 调用处） | 加 `service.StartQuotaResetTask()` |

## 协作关系

- `quotaResetTick` → `service.RunQuotaResetPass`（03 产出）传 periods 集合；手动 handler 传空集合表示"全部生效规则用户"。
- 幂等边界：单实例 master 门禁 + tick 内 `atomic.Bool` + lastTick 单调推进，保证同一边界只触发一轮；批次内单用户只被处理一次由游标扫描保证。
- 手动与定时互不影响：两者无共享状态（手动不读不写 lastTick）。

## 验证方式

- [ ] `crossedQuotaResetPeriods` 表测：不跨 0 点空集合；跨普通日 0 点仅 daily；跨周一 0 点 daily+weekly；跨 1 日 0 点 daily+monthly（+周一则三者）；(23:59→00:01) 与跨多天各正确。
- [ ] 手工冒烟：把系统时间边界用短周期模拟（或直接调 `quotaResetTick` 注入 from/to），确认触发重置且日志触发方式为定时。
- [ ] `POST /api/user/quota_reset/run` 管理员调用返回人数、日志触发方式为手动；普通用户 token 调用被 AdminAuth 拒绝且无余额变更。
- [ ] 重启服务后（模拟停机跨 0 点）无任何补执行日志。
