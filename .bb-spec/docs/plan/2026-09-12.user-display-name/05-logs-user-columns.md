---
name: logs-user-columns
description: 使用日志与任务日志的用户列、移动端卡片、任务详情用户行改为名称主显 + tip 含用户名与 ID；敏感隐藏模式不弹 tip。
---

# 使用日志 / 任务日志用户列

## 目标

使用日志与任务日志页面里所有出现用户的地方（表格用户列、移动端卡片、任务详情对话框）显示用户名称，悬浮拿到用户名与 ID；头像与「点击弹用户信息」保持。

## 业务规则（来源：spec id-in-tooltip / name-resolution）

- 主文本：display_name → username → `User {{id}}`；不并排显示 ID。
- 悬浮 tip：`用户名: {username}`（空则无此行）+ `用户 ID: {user_id}`。
- 「隐藏敏感信息」开关打开时主文本 `••••`、无 tip，DOM 不含用户名与 ID。
- 使用日志用户列在无 username 时原本不渲染（`if (!log.username) return null`）：改为按名称解析规则渲染（有 user_id 即渲染 `User {{id}}`）；user_id 也为 0 的记录仍不渲染。

## 涉及文件

- `web/src/features/usage-logs/data/schema.ts` — 修改（`usageLogSchema` 加 `display_name: z.string().default('')`）
- `web/src/features/usage-logs/types.ts` — 修改（`TaskLog` 加 `display_name?: string`）
- `web/src/features/usage-logs/components/columns/common-logs-columns.tsx` — 修改（用户列）
- `web/src/features/usage-logs/components/common-log-mobile-card.tsx` — 修改（用户字段）
- `web/src/features/usage-logs/components/columns/task-logs-columns.tsx` — 修改（用户列）
- `web/src/features/usage-logs/components/dialogs/task-details-dialog.tsx` — 修改（用户行）
- `web/src/features/usage-logs/components/__tests__/mobile-card.test.tsx` — 修改（追加用户字段用例）
- `web/src/features/usage-logs/components/__tests__/user-columns.test.tsx` — 新建（表格用户列与任务详情用例）

## 函数清单

### common-logs-columns.tsx（修改）

| 函数 | 职责 |
|---|---|
| `UserCell`（既有 cell 组件，改渲染） | 头像 fallback 改用主文本首字符；名称部分替换为 `UserIdentityLabel`，`masked={!sensitiveVisible}`，`onClick` 仍打开用户信息对话框；删除原「超长才显示的 username tip」 |

### common-log-mobile-card.tsx（修改）

| 位置 | 职责 |
|---|---|
| `user` 字段定义 | `value` 改为 `resolveUserName(log, t)`；`visible` 条件改为 `cells.has('user') && (log.username || log.user_id)` |
| 用户字段展开区 | 主文本旁增加 `UserIdentityLabel`（`masked={!context.sensitiveVisible}`）；头像 fallback 用主文本 |

### task-logs-columns.tsx（修改）

| 函数 | 职责 |
|---|---|
| `UserCell`（既有，改渲染） | 删除 `displayName = username || user_id` 拼装；改用 `UserIdentityLabel` + 头像，`masked={!sensitiveVisible}`，点击仍弹用户信息 |

### task-details-dialog.tsx（修改）

| 位置 | 职责 |
|---|---|
| 「用户」`DetailRow` | `value` 从字符串改为 `<UserIdentityLabel user={log} />`（详情对话框无敏感开关，`masked` 恒 false） |

## 协作关系

- 依赖 02（使用日志响应 `display_name`）、03（任务响应 `display_name`）、04（`resolveUserName`、`UserIdentityLabel`）。
- `useUsageLogsContext` 的 `sensitiveVisible` 是 masked 的唯一来源；`setSelectedUserId` / `setUserInfoDialogOpen` 行为不变。
- `display_name` 键缺席时 zod `default('')` / 可选类型兜底为空串。

## 验证方式

- 测试入口：vitest 渲染列 cell / 移动端卡片 / 任务详情，包裹 `UsageLogsProvider`（或 mock `useUsageLogsContext` 返回 `sensitiveVisible`）。
- 测试输入：日志 `{user_id: 7, username: 'zhangsan', display_name: '张三'}`；任务 `{user_id: 7, username: 'zhangsan', display_name: '张三'}`；另一条 `{user_id: 8, username: '', display_name: ''}`。
- 预期结果：
  - 使用日志用户列：文本 `张三`，DOM 无 `zhangsan`/`7` 明文；悬浮后出现 `Username: zhangsan` 与 `User ID: 7`；user 8 渲染 `User 8`。
  - `sensitiveVisible=false`：文本 `••••`，悬浮无 tip。
  - 任务日志用户列与任务详情用户行：文本 `张三`，tip 同上。
  - 移动端卡片用户字段值 `张三`。
- [ ] 上述用例通过；既有 `mobile-card.test.tsx` 通过。
- [ ] `bun run typecheck && bun run lint && bun run test` 通过。
