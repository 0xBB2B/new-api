---
name: user-identity-label
description: 前端共享的用户名称解析函数与 UserIdentityLabel 组件：主文本显示名称，悬浮 tip 两行（用户名、用户 ID），masked 时显示 •••• 且无 tip。
---

# 用户标识共享组件

## 目标

前端有一个纯函数把 `{display_name, username, user_id}` 解析为主文本，以及一个共享组件把「主文本 + 悬浮 tip」封装起来，供使用日志、任务日志、审计日志复用；看板只用纯函数。

## 业务规则（来源：spec name-resolution / id-in-tooltip）

- 主文本取第一个非空值：`display_name`（去首尾空白）→ `username` → `t('User {{id}}')`；永远非空；只显示一个名称，不并排 `a (b)` / `a (a)`。
- 悬浮 tip 固定两行：`用户名: {username}`（username 为空时该行不显示）、`用户 ID: {user_id}`；ID 行永远存在。
- masked（敏感隐藏）时主文本为 `••••`，不渲染 tip，DOM 中不出现用户名与 ID。
- 文案走 i18n，复用既有键 `Username`、`User ID`、`User {{id}}`；新增键 `Username: {{name}}` 与 `User ID: {{id}}` 若既有键不能直接带冒号，则按 i18n 约定添加到全部 7 个 locale。

## 涉及文件

- `web/src/lib/user-identity.ts` — 新建（纯函数）
- `web/src/lib/__tests__/user-identity.test.ts` — 新建
- `web/src/components/user-identity-label.tsx` — 新建（共享组件）
- `web/src/components/__tests__/user-identity-label.test.tsx` — 新建
- `web/src/i18n/locales/*.json` — 修改（仅当需要新增键；用 `bun run i18n:sync` 同步）

## 函数清单

### web/src/lib/user-identity.ts（新建）

| 函数 / 类型 | 职责 |
|---|---|
| `UserIdentity`（类型） | `{ user_id: number; username?: string; display_name?: string }` |
| `resolveUserName` | 输入 `UserIdentity` 与 `t`，按 display_name → username → `User {{id}}` 顺序返回主文本 |
| `describeUserTooltip` | 输入 `UserIdentity` 与 `t`，返回 tip 行数组：有 username 时 `[用户名行, ID 行]`，否则 `[ID 行]` |

### web/src/components/user-identity-label.tsx（新建）

| 组件 / props | 职责 |
|---|---|
| `UserIdentityLabel` | props：`user: UserIdentity`、`masked?: boolean`、`className?`、`onClick?`。渲染主文本 span（`truncate`），包在项目 `Tooltip`/`TooltipTrigger`/`TooltipContent`（`@/components/ui/tooltip`）中，tip 内容为 `describeUserTooltip` 的行；`masked` 为真时只渲染 `••••`、不渲染 Tooltip；`onClick` 存在时主文本可点击（沿用现有用户列 `hover:underline` 样式） |

## 协作关系

- 组件只负责名称与 tip；头像（`Avatar` + `getUserAvatarStyle`）、点击弹用户信息对话框仍由调用方（05）组合，组件不内置。
- 看板（07）只 import `resolveUserName`，不用组件。
- 与既有 `TruncatedCell`（`@/components/data-table/core/truncated-cell`）的差异：本组件 tip 内容是结构化的两行身份信息而非溢出全文，且需 masked 分支；不复用 `TruncatedCell`。

## 验证方式

- 测试入口：vitest（`cd web && bun run test`）；纯函数直接调用；组件用 testing-library 渲染并 `userEvent.hover`。
- 测试输入 / 预期（`t` 用 identity 或 i18next 测试实例）：

| display_name | username | user_id | 主文本 | tip 行 |
|---|---|---|---|---|
| `张三` | `zhangsan` | 7 | `张三` | `Username: zhangsan`、`User ID: 7` |
| `  ` | `zhangsan` | 7 | `zhangsan` | 同上 |
| `zhangsan` | `zhangsan` | 7 | `zhangsan` | 同上 |
| 空 | 空 | 7 | `User 7` | `User ID: 7` |

- 组件：`masked=false` 悬浮后 tip 出现上述两行；`masked=true` 时文本为 `••••`，DOM 不含 `zhangsan` 与 `7`，悬浮无 tip。
- [ ] 纯函数四组表驱动用例通过。
- [ ] 组件两种模式用例通过。
- [ ] `bun run typecheck && bun run lint && bun run i18n:sync`（无 diff）通过。
