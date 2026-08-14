---
name: claude-dialog-list-style
description: 前端：Claude 账户和用量弹窗（状态卡/动态窗口卡/原始 JSON）+ 列表用量条前数后与阈值配色 + 按类型点击分流 + i18n。
---

# Claude 弹窗与列表用量样式

## 目标

Claude 订阅渠道点击列表用量弹出账户和用量弹窗；列表用量单元格进度条在前、百分比在后，文字与进度条按阈值配色。

## 业务规则（来源：spec claude-usage-dialog / admin-list-display）

- 点击触发 `GET /api/channel/:id/claude/usage`，成功开弹窗；`success:false` 首次点击 toast 报错不开弹窗，弹窗内刷新失败顶部横幅展示 message。
- 弹窗三区块：状态卡（端点响应 `subscription_type` 套餐 badge，缺失不渲染；`HTTP <upstream_status>` badge；渠道名 + `(#id)`，敏感隐藏时掩码；刷新按钮，请求中禁用）；窗口卡（两形态动态渲染——顶层对象取 `five_hour` 与 `seven_day` 前缀键的 `utilization`，`limits[]` 按 `kind` 取 `percent`；`extra_usage` 等非窗口不渲染；每卡窗口名 + 大号百分比 + 进度条，≥95 红 / ≥80 橙 / 其余默认，百分比与进度条同色；重置时间字段存在才显示）；原始 JSON 折叠区（完整响应格式化 + 复制按钮）。关闭弹窗清空本地状态。
- 列表用量单元格：进度条在前、百分比数字在后；文字与进度条按同一阈值着色（≥95 红 / ≥80 橙 / 其余默认）。
- 点击分流：订阅渠道用量单元格均可点击，type 57 弹 Codex 弹窗、type 61 弹 Claude 弹窗；tooltip 点击提示按类型区分文案。
- 非订阅渠道行渲染不变；文案 i18n 7 语言键集对称。

## 涉及文件（均在 `web/src/features/channels/`，另注除外）

- `api.ts` — 修改（`getClaudeUsage` + 响应类型）
- `lib/channel-utils.ts` — 修改（阈值分级纯函数）
- `components/dialogs/claude-usage-dialog.tsx` — 新建
- `components/subscription-usage-cell.tsx` — 修改（点击分流、布局、配色）
- `lib/__tests__/subscription-usage.test.ts` — 修改（追加分级函数用例）
- `components/__tests__/claude-usage-dialog.test.tsx` — 新建
- `components/__tests__/subscription-usage-cell.test.tsx` — 修改（布局顺序与配色断言）
- `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json` — 修改

## 函数清单

### api.ts

| 符号 | 职责 |
|---|---|
| `ClaudeUsageResponse`（type） | `{success, message?, upstream_status?, subscription_type?, data?}`，data 为 unknown（两形态透传） |
| `getClaudeUsage` | `api.get('/api/channel/${channelId}/claude/usage', ...)`，写法先例同文件 `getCodexUsage`（disableDuplicate） |

### lib/channel-utils.ts

| 函数 | 职责 |
|---|---|
| `getUsagePercentLevel` | 百分比 → `'danger' | 'warning' | 'default'`（≥95 / ≥80 / 其余），列表 cell 与 Claude 弹窗共用；不改 Codex 弹窗既有私有实现 |

### components/dialogs/claude-usage-dialog.tsx

| 符号 | 职责 |
|---|---|
| `resolveClaudeUsageWindows`（导出纯函数） | 端点 data → 统一窗口数组 `{key, label, percent, resetsAt?}`：顶层对象形态取 `five_hour`/`seven_day` 前缀键（滤 `extra_usage` 等非窗口），`limits[]` 形态按 `kind`；utilization/percent 缺失按 0；宽容提取重置时间字段（缺失为 undefined） |
| `ClaudeUsageDialog` | 弹窗组件：props 含 open/onOpenChange/channelName/channelId/channelDisplayName/channelDisplayId/response/onRefresh/isRefreshing（对齐 CodexUsageDialog props 形状）；渲染状态卡 + 窗口卡（`getUsagePercentLevel` 配色）+ 原始 JSON（复制按钮）+ 错误横幅；关闭时清态由父组件负责 response 置空 |

### components/subscription-usage-cell.tsx

无新导出：`isClickable` 改为 `SUBSCRIPTION_CHANNEL_TYPES.has(channel.type)`；`handleClick` 按 type 分流调 `getCodexUsage` / `getClaudeUsage` 并各自 setState；按 type 条件挂载 `CodexUsageDialog` / `ClaudeUsageDialog`；tooltip 点击提示按 type 用 `Click to view Codex usage` / `Click to view Claude usage`；布局改为 `Progress` 在前、百分比 span 在后；百分比文字 class 与进度条 indicator class 按 `getUsagePercentLevel` 着色（进度条用 `[&_[data-slot=progress-indicator]]:bg-*` 范式，先例 `src/features/users/components/user-quota-cell.tsx`）。

### i18n

新增 key（英文源串）：`Claude Account & Usage`、`Claude Account Status`、`Click to view Claude usage`、窗口名（`5-Hour Window` 复用既有；`Weekly Window` 复用既有；`Weekly Opus Window`、`Weekly Sonnet Window` 等按需新增）；中性 key（`Base Limits`/`Raw JSON`/`Refresh` 等）复用既有。7 语言补齐，`bun run i18n:sync` 校验。

## 协作关系

- cell 点击 → `getClaudeUsage` → 后端端点（plan claude-usage-endpoint 产物）→ `ClaudeUsageDialog`。
- 复用：`@/components/status-badge`、`@/components/ui/progress`、`@/components/ui/tooltip`、`@/components/ui/dialog` 及 CodexUsageDialog 同款布局组件、`@/lib/format` 时间格式化。
- 阈值分级唯一实现 `getUsagePercentLevel`；Codex 弹窗内部实现不动（外科手术边界）。

## 验证方式

- 测试入口：`bun test`（web/ 下）；纯函数直接 import；组件按 happy-dom 引导范本以 props 驱动渲染断言 DOM。
- 测试输入与预期：
  - `getUsagePercentLevel`：94.9→default、95→danger、80→warning、79.9→default、96.2→danger、88→warning。
  - `resolveClaudeUsageWindows`：`{"five_hour":{"utilization":96.2},"seven_day":{"utilization":88},"seven_day_opus":{"utilization":41}}` → 3 窗口且百分比精确；`{"limits":[{"kind":"session","percent":40},{"kind":"weekly_all","percent":85}]}` → 2 窗口；含 `extra_usage` 的响应不产出对应窗口；非对象 data → 空数组。
  - `ClaudeUsageDialog`（props 驱动）：三窗口响应渲染三张卡且文本含各百分比；response 含 subscription_type:"max" 渲染该文本，缺失不渲染；success:false 渲染 message 横幅。
  - cell：DOM 顺序进度条节点在百分比文本之前；96.2 的百分比元素带 danger 色 class、88 带 warning 色 class；type 61 与 57 均可点击（cursor/点击处理存在）。
  - `bun run typecheck` 与 `bun run i18n:sync` 通过。
- [ ] 上述纯函数与组件分支各有断言精确期望值的测试。
- [ ] 非订阅渠道行渲染路径未被触碰。
