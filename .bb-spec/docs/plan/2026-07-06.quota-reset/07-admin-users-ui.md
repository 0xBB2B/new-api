---
name: admin-users-ui
description: 用户编辑抽屉新增专属规则与退出标记配置，用户列表页新增「立即按规则重置」按钮与确认对话框。
---

# 管理端用户规则配置与手动触发 UI

## 目标

管理员在用户编辑抽屉里为单个用户配置专属重置规则（周期 + 重置值）或退出标记；在用户列表页顶部操作区一键触发全员重置（带确认）。

## 业务规则（来源：spec rule-resolution / manual-trigger / reset-semantics）

- 专属规则 = 周期（daily/weekly/monthly）+ 重置值（≥0）；清除专属规则后用户回落全局规则。
- 退出标记使用户不参与任何自动/手动重置，优先于专属规则。
- 手动触发对全体有生效规则的用户按各自规则值重置；确认文案需说明会收回未用完余额。

## 涉及文件

- `web/default/src/features/users/api.ts` — 修改（两个新请求函数）
- `web/default/src/features/users/types.ts` — 修改（payload 类型）
- `web/default/src/features/users/lib/user-form.ts` — 修改（schema/defaults/两个 transform 四处同步）
- `web/default/src/features/users/components/users-mutate-drawer.tsx` — 修改（新表单字段）
- `web/default/src/features/users/components/users-primary-buttons.tsx` — 修改（全员重置按钮）
- `web/default/src/features/users/components/quota-reset-run-dialog.tsx` — 新建（确认对话框）
- `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json` — 修改

## 函数清单

### api.ts / types.ts（修改）

| 函数/类型 | 职责 |
|---|---|
| `updateUserQuotaResetRule` | `api.post('/api/user/quota_reset_rule', payload)`，payload 为 02 契约的 {user_id, rule 或 null, opt_out} |
| `runQuotaResetNow` | `api.post('/api/user/quota_reset/run')`，返回信封含重置人数 |
| `UpdateQuotaResetRulePayload` | 上述 payload 类型（period 联合类型 'daily'\|'weekly'\|'monthly'） |

### lib/user-form.ts（修改，四处同步）

| 位置 | 职责 |
|---|---|
| `userFormSchema` | 加 `quota_reset_opt_out: boolean`、`quota_reset_rule_enabled: boolean`、`quota_reset_period: enum`、`quota_reset_value: 非负数`（rule_enabled 为 UI 态：false 提交 rule=null） |
| `USER_FORM_DEFAULT_VALUES` | 新字段默认：opt_out=false、rule_enabled=false、period='monthly'、value=0 |
| `transformUserToFormDefaults` | 从 user.setting JSON 解析 `quota_reset_rule` / `quota_reset_opt_out` 回填表单 |
| `transformFormDataToPayload` | 不把新字段混入既有 updateUser payload（专属规则走独立端点，见抽屉保存流程） |

### users-mutate-drawer.tsx（修改）

| 位置 | 职责 |
|---|---|
| 更新模式的表单区（Group & Quota 附近） | 加"额度重置"字段组：退出 Switch、专属规则启用 Switch、周期 Select、重置值 Input（quota 单位换算展示复用 `formatQuota`/`getCurrencyDisplay` 既有用法）；保存时若重置相关字段 dirty 则在 `updateUser` 之后追加调 `updateUserQuotaResetRule` |

### users-primary-buttons.tsx + quota-reset-run-dialog.tsx

| 函数/组件 | 职责 |
|---|---|
| `UsersPrimaryButtons`（修改） | 加"立即按规则重置"按钮，点击开确认对话框 |
| `QuotaResetRunDialog`（新建，样板：user-quota-dialog.tsx 的 Dialog + useState 模式） | 确认文案（说明覆盖写会收回未用余额）→ 调 `runQuotaResetNow` → toast 显示重置人数 → 刷新用户列表查询 |

## 协作关系

- 后端依赖：02 的 `POST /api/user/quota_reset_rule` 与 04 的 `POST /api/user/quota_reset/run`。
- 抽屉回填数据源：`GET /api/user/:id` 返回的 setting JSON 字符串（既有下发链路，无后端改动）。
- 文案六份 locale 同步，`bun run i18n:sync` 校验。

## 验证方式

- [ ] `bun run build` 通过、`bun run i18n:sync` 无 missing key。
- [ ] 浏览器：为某用户配置专属规则保存 → 重开抽屉字段回填正确；关闭专属规则开关保存 → 规则被清除。
- [ ] 勾选退出标记的用户在点击全员重置后余额不变。
- [ ] 全员重置按钮 → 确认对话框 → 执行后 toast 显示人数，列表余额刷新。
