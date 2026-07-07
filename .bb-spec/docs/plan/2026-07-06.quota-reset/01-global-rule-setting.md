---
name: global-rule-setting
description: 新建 quota_reset_setting 分层配置模块承载全局重置规则，并在 option 更新入口加 period 枚举与 reset_value ≥0 校验。
---

# 全局重置规则配置模块

## 目标

站点级"额度重置全局规则"（启用开关 + 周期 + 重置值）可通过 `PUT /api/option/` 配置、持久化到 option 表、多实例同步，业务代码可读内存值。

## 业务规则（来源：spec schedule-trigger / reset-semantics / rule-resolution）

- 周期字段只接受 `daily` / `weekly` / `monthly` 三个值，其他值拒绝保存。
- 重置值为 quota 内部单位的整数，≥ 0 合法（0 = 当期无预算），负数拒绝保存。
- 全局规则默认关闭；关闭且用户无专属规则时系统对该用户零行为。

## 涉及文件

- `setting/operation_setting/quota_reset_setting.go` — 新建
- `controller/option.go` — 修改（UpdateOption 的 key 特定校验区）

## 函数清单

### setting/operation_setting/quota_reset_setting.go（新建，样板：同目录 checkin_setting.go）

| 函数/结构 | 职责 |
|---|---|
| `QuotaResetSetting` | struct：`Enabled bool`（json `enabled`）、`Period string`（json `period`）、`ResetValue int`（json `reset_value`） |
| `quotaResetSetting`（包级变量） | 默认值：Enabled=false、Period="monthly"、ResetValue=0 |
| `init` | `config.GlobalConfig.Register("quota_reset_setting", &quotaResetSetting)`，注册后 option key 自动为 `quota_reset_setting.enabled` 等三个 |
| `GetQuotaResetSetting` | 返回 `*QuotaResetSetting` 供业务读内存值 |
| `QuotaResetPeriodDaily` / `QuotaResetPeriodWeekly` / `QuotaResetPeriodMonthly`（常量） | 周期枚举值 `"daily"` / `"weekly"` / `"monthly"`，规则解析与前端约定共用 |
| `IsValidQuotaResetPeriod` | 判断字符串是否为三枚举之一，option 校验与每用户规则校验共用 |

### controller/option.go（修改）

| 函数 | 职责 |
|---|---|
| `UpdateOption`（既有，加分支） | 在 key 特定校验区新增：key 为 `quota_reset_setting.period` 时经 `IsValidQuotaResetPeriod` 校验；key 为 `quota_reset_setting.reset_value` 时校验为 ≥0 整数；不合法返回既有信封格式错误（HTTP 200 + success:false） |

## 协作关系

- 持久化与内存同步走既有链路，无需新代码：`model.UpdateOption` → option 表落库 → `updateOptionMap` → `handleConfigUpdate` 前缀匹配 `quota_reset_setting` 反射写回 struct；`model.InitOptionMap` 的 `ExportAllConfigs()` 自动导出默认值；`SyncOptions` 多实例同步。
- option 表为 KV 表，新增 key 零 DB 迁移。
- 无需在 `handleConfigUpdate` 加副作用钩子：调度任务每次 tick 直接读 `GetQuotaResetSetting()` 内存值，配置变更下一个 tick 自然生效。
- 下游消费方：重置引擎的规则解析读 `GetQuotaResetSetting()`（03-reset-engine）；管理端设置 UI 提交这三个 key（06-admin-settings-ui）。

## 验证方式

- [ ] `PUT /api/option/` 提交 `quota_reset_setting.period` = `"monthly"` 成功；= `"yearly"` 返回 success:false。
- [ ] 提交 `quota_reset_setting.reset_value` = `0` 成功；= `-1` 返回 success:false。
- [ ] 重启服务后 `GET /api/option/` 能读回已保存的三个 key，默认未配置时 enabled 为 false。
