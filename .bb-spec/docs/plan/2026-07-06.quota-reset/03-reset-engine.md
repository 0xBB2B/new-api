---
name: reset-engine
description: service/quota_reset.go 重置引擎：规则解析、下次触发时点计算、批量重置执行（事务 + 缓存同步 + 系统日志），含表测。
---

# 重置引擎（规则解析 + 时点计算 + 批量执行）

## 目标

提供三个可独立测试的纯业务能力：对任意用户解析生效规则、计算下次触发时点、对全量用户执行一轮重置批次。定时与手动两个入口共用同一批次函数。

## 业务规则（来源：spec rule-resolution / reset-semantics / schedule-trigger）

- 生效规则解析顺序（命中即停）：①退出自动重置标记 → 无规则；②用户状态非启用 → 无规则；③有专属规则 → 用专属（周期与值不与全局混合）；④全局规则启用 → 用全局；⑤否则无规则。角色不影响结果。
- 重置动作：quota **覆盖写**为重置值，与重置前余额无关；used_quota、request_count 及其他字段不动；每执行一次恰好写一条系统日志（重置前余额、重置后余额、触发方式定时/手动）；被跳过的用户不产生日志。重置幂等。
- 触发时点（服务器本地时区）：daily = 每天 00:00，weekly = 周一 00:00，monthly = 每月 1 日 00:00。

## 涉及文件

- `service/quota_reset.go` — 新建
- `service/quota_reset_test.go` — 新建
- `model/user.go` — 修改（新增 override 写函数）

## 函数清单

### model/user.go（修改）

| 函数 | 职责 |
|---|---|
| `ResetUserQuota`（新增） | 事务内锁定用户行（样板：model/subscription.go 批量重置的 FOR UPDATE 用法，SQLite 无行锁自然退化为串行事务）→ 读旧 quota → `Update("quota", value)` → 事务提交后 `updateUserQuotaCache` 同步 Redis 绝对值 → 返回旧值供日志用 |

### service/quota_reset.go（新建）

| 函数/类型 | 职责 |
|---|---|
| `QuotaResetTrigger`（类型 + 常量） | 触发方式枚举：`scheduled` / `manual`，日志与审计共用 |
| `ResolveQuotaResetRule` | 按上述五步顺序对单个用户（输入其 status 与 `dto.UserSetting`）解析生效规则；返回 (period, value, ok)；全局规则读 `operation_setting.GetQuotaResetSetting()` |
| `NextQuotaResetTime` | 给定 period 与当前时间（`time.Time`，本地时区），返回下一个触发时点；纯函数，供 GetSelf 展示与测试 |
| `RunQuotaResetPass` | 一轮批次：id 游标分页扫全表用户（每批固定条数），逐用户 `ResolveQuotaResetRule`；periods 参数非空时仅重置生效规则周期在集合内的用户（定时入口），空则全部生效规则用户（手动入口）；命中者调 `model.ResetUserQuota` + `model.RecordLog(uid, LogTypeSystem, 内容含旧余额/新余额/触发方式)`；单用户失败记 warn 日志继续下一个；返回重置人数 |

### service/quota_reset_test.go（新建，testify，表测 + t.Run 命名子测试）

| 测试 | 覆盖 |
|---|---|
| `TestResolveQuotaResetRule` | 表测五步顺序：退出标记压过专属规则 / 禁用用户无规则 / 专属整体覆盖全局 / 仅全局 / 全关零规则；管理员角色不影响 |
| `TestNextQuotaResetTime` | 表测三种周期：普通日、周一当天 00:00 之后、月末跨月、跨年（12 月→1 月 1 日） |
| `TestRunQuotaResetPass` | SQLite 内存库 fixture：混合用户（全局适用/专属/退出/禁用/quota 高于与低于重置值）执行一轮，断言各自终态 quota 精确值、used_quota 不变、日志条数与内容、返回人数；重复执行结果幂等 |

## 协作关系

- `RunQuotaResetPass` → `ResolveQuotaResetRule` → `operation_setting.GetQuotaResetSetting`（01 产出）与 `dto.UserSetting` 新字段（02 产出）。
- `RunQuotaResetPass` → `model.ResetUserQuota` → DB + `updateUserQuotaCache`；→ `model.RecordLog`（既有，LogTypeSystem=4）。
- 调用方：定时 ticker 传当次跨越的 periods 集合、手动端点传空集合（04-schedule-and-manual）；`NextQuotaResetTime` 被 GetSelf 消费（05-self-api-visibility）。
- 日志内容格式复用 `logger.LogQuota` 做额度显示格式化，与管理员调额日志风格一致。

## 验证方式

- [ ] `go test ./service/ -run 'QuotaReset'` 全绿。
- [ ] 三个测试覆盖 spec reset-semantics 的全部验收项（覆盖写、统计不动、0 值合法、每次恰一条日志、幂等）与 rule-resolution 的解析表。
- [ ] 手工冒烟：两个用户（一个全局适用、一个退出）执行 `RunQuotaResetPass`，DB 中 quota 与 logs 表符合预期。
