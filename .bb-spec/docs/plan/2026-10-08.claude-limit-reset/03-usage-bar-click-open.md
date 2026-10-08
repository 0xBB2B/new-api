---
name: 03-usage-bar-click-open
description: 订阅渠道余额列去掉「账户信息」徽章；进度条、无数据的 -、打码徽章都变成可点击、可键盘操作的入口，打开该渠道自己的用量弹窗。
---
# 点用量区域打开用量弹窗

## 目标
Codex 与 Claude 订阅渠道的余额列只剩一个入口：点它（或用键盘）就打开该渠道的用量弹窗，省出「账户信息」按钮占的宽度。

## 业务规则（来源：spec usage-bar-click-open、usage-bar-layout）
- 适用：Codex（type 57）与 Claude 订阅渠道的非标签行。普通渠道的已用 / 余额徽章、标签行、vLLM / SGLang 状态徽章不变。
- 余额列只渲染一个入口，不渲染「账户信息」徽章：
  - 敏感信息可见且有快照 → 进度条整块；
  - 可见但无快照 → `-`，悬浮提示「暂无用量数据，点击查询」；
  - 敏感信息隐藏 → 打码徽章 `••••`，没有悬浮提示。
- 点击：Codex 渠道请求 Codex 用量并打开 Codex 弹窗，Claude 订阅渠道请求 Claude 用量并打开 Claude 弹窗；失败时按现有方式提示，不打开弹窗。这正是现有 `handleClickUpdate` 的行为，直接复用。
- 请求中：入口透明度降低、鼠标为等待样式、`aria-busy=true`，再点击不发新请求（`handleClickUpdate` 已用 `isUpdating` 拦截）。
- 入口是 `role=button`、`tabIndex=0`，回车或空格等同点击。
- 每个窗口行原有的悬浮提示（「将于 <时间> 重置」/「暂未开始计时」）保留。
- 布局规则（80px×6px 进度条、5h/7d 标签、颜色阈值 80% / 95%）都不变。

## 涉及文件
- 修改 `web/src/features/channels/components/subscription-usage-bar.tsx`
- 修改 `web/src/features/channels/components/channels-columns.tsx`
- 修改 `web/src/features/channels/components/__tests__/subscription-usage-bar.test.tsx`
- 修改 `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`（新键「No usage data, click to query」；若「No usage data」与「Account Info」已无其他引用则删除）

## 函数清单
### web/src/features/channels/components/subscription-usage-bar.tsx
| 函数名 | 职责 |
|---|---|
| `SubscriptionUsageBar` | 新增「打开」回调与「加载中」两个输入；有快照时根节点、无快照时的 `-` 都是入口（按钮语义、键盘、加载样式），保留每行 Tooltip |
| `UsageRow` | 不再自带 `cursor-help`，由入口统一鼠标样式；其余不变 |

### web/src/features/channels/components/channels-columns.tsx
| 函数名 | 职责 |
|---|---|
| 余额列渲染组件（`isSubscriptionChannel` 分支） | 订阅渠道只渲染入口：可见时为 `SubscriptionUsageBar`（传入 `handleClickUpdate` 和 `isUpdating`），隐藏时为打码徽章入口；删除「账户信息」标签、`Click to view Claude usage` / `Click to view Codex usage` 提示等只服务于该徽章的分支和变量；非订阅渠道的代码路径不动 |

## 协作关系
入口点击 / 回车 / 空格 → `handleClickUpdate`（已有）→ `getClaudeUsage` / `getCodexUsage` → `setClaudeUsageOpen` / `setCodexUsageOpen`。实施前先读 `web/AGENTS.md` 和项目的 shadcn-ui skill，按钮语义优先复用现有组件；`StatusBadge` 已支持 `onClick`。新文案按 i18n-translate skill 补齐 7 种语言。

## 验证方式
- 测试入口：`cd web && bun run test src/features/channels/components/__tests__/subscription-usage-bar.test.tsx`
- 测试输入与预期（渲染 `SubscriptionUsageBar`，传入记录调用次数的打开回调）：
  - 有快照（5h 29%、7d 31%）→ `getByRole('button')` 存在；点击后回调调用 1 次；聚焦后按 Enter、按 Space 各触发 1 次。
  - 加载中 → 按钮 `aria-busy="true"`；点击后回调调用 0 次。
  - 无快照 → 显示 `-` 且角色为按钮；悬停显示「No usage data, click to query」；点击后回调调用 1 次。
  - 原有布局和每行悬浮提示的测试保持通过。
- [ ] 上述测试通过
- [ ] `bun run typecheck`、`bun run lint`、`bun run build` 通过
- [ ] `bun run i18n:sync` 后 7 种语言都有新键，没有遗留只服务于「账户信息」徽章的键
- [ ] 真机：渠道列表里 Codex 与 Claude 订阅行都看不到「账户信息」；点进度条 / `-` / `••••` 分别打开各自的弹窗（对照预览页 `.bb-spec/.cache/ui-preview/subscription-usage-reset.html` 第一部分）
