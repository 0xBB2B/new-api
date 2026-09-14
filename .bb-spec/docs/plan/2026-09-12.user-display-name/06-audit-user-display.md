---
name: audit-user-display
description: 审计日志用户名列与详情「操作者」行改为名称主显、tip 含用户名与 ID；buildAuditDetails 的 actor 拆为结构化字段。
---

# 审计日志用户展示

## 目标

审计日志列表的用户列与详情对话框的「操作者」行显示用户名称，悬浮拿到用户名与 ID，不再内联 `名称 (ID: x)`。

## 业务规则（来源：spec id-in-tooltip / name-resolution）

- 主文本：display_name → username → `User {{id}}`。
- tip 两行：`用户名: {username}`（空则无）+ `用户 ID: {user_id}`。
- 详情「操作者」：主文本为名称，ID 与用户名只在该行悬浮 tip 中出现，不再 `名称 (ID: x)` 内联。
- 操作者优先取 `other.admin_info.admin_username` / `admin_id`（既有逻辑），否则取记录的 `username` / `user_id`；display_name 只对记录本身的用户有效，admin_info 路径无显示名时按 username 回退。
- 审计页无敏感隐藏开关，`masked` 恒为 false。
- 「目标」（target）字段保持既有 `名称 (ID: x)` 写法不改（不在本次范围）。

## 涉及文件

- `web/src/features/usage-logs/audit/api.ts` — 修改（`AuditLog` 加 `display_name?: string`）
- `web/src/features/usage-logs/audit/components/audit-log-columns.tsx` — 修改（用户名列）
- `web/src/features/usage-logs/audit/lib/audit-details.ts` — 修改（actor 结构）
- `web/src/features/usage-logs/audit/components/audit-log-details-dialog.tsx` — 修改（操作者行）
- `web/src/features/usage-logs/audit/__tests__/details.test.tsx` — 修改（追加操作者用例）
- `web/src/features/usage-logs/audit/__tests__/viewer.test.tsx` — 修改（追加用户列用例）

## 函数清单

### audit-log-columns.tsx（修改）

| 位置 | 职责 |
|---|---|
| `username` 列定义 | `accessorKey` 保持 `'username'`（排序/筛选键不变），header 改为 `t('User')`，新增 `cell` 渲染 `<UserIdentityLabel user={row.original} />` |

### audit-details.ts（修改）

| 函数 / 类型 | 职责 |
|---|---|
| `AuditDetails.actor`（类型改） | 由 `string` 改为 `UserIdentity | null`（`{ user_id, username, display_name }`） |
| `buildAuditDetails`（既有，改逻辑） | 组装 `actor` 对象而非拼字符串：`admin_info` 同时含 number 型 `admin_id` 与 `admin_username` 时整体取管理员身份（无显示名）；否则取记录本身的 `user_id`、`username`、`display_name`；记录 `user_id` 为 0 且 `username` 为空时 `actor` 为 null |

### audit-log-details-dialog.tsx（修改）

| 位置 | 职责 |
|---|---|
| 「操作者」`DetailRow` | `value` 改为 `<UserIdentityLabel user={detail.actor} />`；`hasOperation` 判定改用 `detail.actor !== null` |

## 协作关系

- 依赖 02（审计响应 `display_name`）、04（`UserIdentityLabel`）。
- `buildAuditDetails` 在列 accessor（事件列 summary）中也被调用，那里不使用 `actor`，改类型不影响。
- `AuditDetails.target` 及其他字段不动。

## 验证方式

- 测试入口：vitest；`buildAuditDetails` 直接调用；列与详情用 testing-library 渲染。
- 测试输入：
  - 记录 A：`{user_id: 7, username: 'zhangsan', display_name: '张三', other: null}`。
  - 记录 B：`{user_id: 7, username: 'zhangsan', display_name: '张三', other: {admin_info: {admin_id: 1, admin_username: 'root'}}}`。
  - 记录 C：`{user_id: 0, username: '', other: null}`。
- 预期结果：
  - `buildAuditDetails(A).actor` 等于 `{user_id: 7, username: 'zhangsan', display_name: '张三'}`；B 的 actor 为 `{user_id: 1, username: 'root', display_name: undefined}`；C 的 actor 为 null。
  - 详情对话框渲染 A：操作者行文本 `张三`，DOM 不含 `(ID: 7)`；悬浮出现 `Username: zhangsan`、`User ID: 7`。
  - 列表用户列渲染 A：文本 `张三`；渲染 B：文本 `张三`（列显示记录用户，而非 admin_info）。
- [ ] 上述用例通过；既有 `details.test.tsx`、`viewer.test.tsx` 其余用例通过。
- [ ] `bun run typecheck && bun run lint && bun run test` 通过。
