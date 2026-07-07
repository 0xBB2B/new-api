---
name: admin-settings-ui
description: 系统设置 billing 页新增「额度重置」section：启用开关 + 周期下拉 + 重置值输入，提交 quota_reset_setting.* 三个 option key。
---

# 管理端全局规则设置 UI

## 目标

Root 管理员在系统设置的计费页配置全局重置规则：启用开关、周期（每天/每周/每月）、重置值；保存后逐 key 提交 `PUT /api/option/`。

## 业务规则（来源：spec rule-resolution / schedule-trigger / reset-semantics）

- 全局规则含三项：启用开关（默认关）、周期（daily/weekly/monthly 三选一）、重置值（≥0 整数，quota 内部单位）。
- 周期与重置值仅在启用后有意义，UI 在开关关闭时收起这两项（样板行为同签到设置区）。

## 涉及文件

- `web/default/src/features/system-settings/general/quota-reset-section.tsx` — 新建
- `web/default/src/features/system-settings/billing/section-registry.tsx` — 修改（注册 section）
- `web/default/src/features/system-settings/billing/index.tsx` — 修改（defaultBillingSettings 加默认值）
- `web/default/src/features/system-settings/types.ts` — 修改（BillingSettings 类型加三 key）
- `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json` — 修改（新文案 key）

## 函数清单

### quota-reset-section.tsx（新建；结构样板：general/checkin-settings-section.tsx，周期下拉样板：general/pricing-section.tsx 的 Select 用法）

| 函数/结构 | 职责 |
|---|---|
| `quotaResetSchema`（zod） | `quota_reset_setting: { enabled: boolean, period: enum(daily/weekly/monthly), reset_value: 非负数 }` |
| `QuotaResetSection` | `useSettingsForm` + `useUpdateOption`，onSubmit 遍历 changedFields 逐 key `mutateAsync({key, value})`；`form.watch` enabled 控制周期/重置值显隐；周期用 `Select` items 三项、重置值用 number `Input` |

### section-registry.tsx / billing/index.tsx / types.ts（修改）

| 位置 | 职责 |
|---|---|
| `BILLING_SECTIONS` | 加 `{id: 'quota-reset', titleKey, build}` 条目，把 settings 中三 key 映射为 section defaultValues |
| `defaultBillingSettings` | 加 `'quota_reset_setting.enabled': false`、`'quota_reset_setting.period': 'monthly'`、`'quota_reset_setting.reset_value': 0` 回退默认 |
| `BillingSettings` 类型 | 同步加三 key 类型 |

## 协作关系

- 提交链路复用既有：`useUpdateOption` → `PUT /api/option/`（RootAuth）→ 后端 key 校验（01 产出）→ option 表 + 内存 struct。
- defaultValues 来源复用既有：`getSystemOptions()` → `SettingsPage` 注入 build(settings)。
- 文案全部走 `useTranslation` 的 `t('English key')`，六份 locale 同步补 key（英文源串即 key），落盘后跑 `bun run i18n:sync` 校验对称。

## 验证方式

- [ ] `cd web/default && bun run build` 通过，`bun run i18n:sync` 报告无 missing key。
- [ ] 浏览器：计费设置页出现"额度重置"区；开关关闭时周期/重置值隐藏；保存后刷新页面值保持。
- [ ] 周期下拉只有三项；重置值输入负数被 zod 校验拦截。
