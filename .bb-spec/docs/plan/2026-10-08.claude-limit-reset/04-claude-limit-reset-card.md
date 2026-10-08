---
name: 04-claude-limit-reset-card
description: Claude 用量弹窗新增「限额重置」卡片：纯函数把上游 cedar_ember / juniper_tide 解析成显示内容；点「免费重置」先二次确认，再调重置接口，成功后刷新用量。
---
# Claude 用量弹窗的限额重置卡片

## 目标
Claude 用量弹窗里能看到全部重置与 5 小时重置的状态，并能在确认后执行重置；Codex 弹窗不变。

## 业务规则（来源：spec claude-limit-reset-card、claude-limit-reset-redeem）
- 数据：用量接口响应的 `data.cedar_ember`、`data.juniper_tide`、`limit_reset_unavailable`，不另外请求。
- 卡片位置：窗口网格（`RateLimitWindowGrid`）之下、原始 JSON 折叠区之上。
- 卡片状态按优先级取第一个命中的：
  1. 用量请求中 → 两行骨架；
  2. 用量接口失败（`success=false`）→ 错误说明；
  3. `limit_reset_unavailable == "client_version"` → 「取不到 Claude Code 最新版本，暂时无法查询重置」；
  4. 两块都缺失或为 null → 「上游未返回重置信息」；
  5. 否则显示两行。
- 全部重置行：取 `grants` 里 `id == next_grant_id` 的那条。
  - 找到时显示：`label` · 剩余 `resets_left`/`resets_total` 次 · `ends_at` 过期；「会清空：」后面是 `clears` 映射的窗口名（`five_hour` → 5 小时窗口，`seven_day` → 每周窗口，其他原样显示）。
  - 按钮可点要同时满足六个条件：`eligible == true`；`usable_now == true`；`paused != true`；`resets_left > 0`；`cooldown_until` 为空或已过；`use_requires_limit != true` 或 `at_limit == true`。
  - `cooldown_until` 未过时，说明里显示「冷却中，<时间> 后可用」。
  - 找不到这条记录时显示「目前没有。拿到后会显示在这里」，不显示按钮。
- 5 小时重置行：
  - `eligible == true && available == true` 时显示「本周剩余可用 · 只清空 5 小时窗口，不影响每周上限」和按钮；
  - 否则按 `ineligible_reason` 显示说明：`not_at_wall` → 「5 小时额度用完时才会出现」；`surface` / `cli_version` → 「客户端身份校验未通过」；其他非空值原样显示；空值且 `next_available_at` 有值 → 「<时间> 后可用」；都没有 → 「目前没有。拿到后会显示在这里」。
- 时间字段是 ISO 8601，用浏览器本地时区显示到分钟（复用 `@/lib/format` 的日期格式化；需要 Intl 时先过 `toIntlLocale`）；解析失败显示 `-`。
- 按钮文字「免费重置」。点击后用 `ConfirmDialog` 二次确认：全部重置说明会清空哪些窗口、次数用完就没有了；5 小时重置说明只清空 5 小时窗口。确认后调 `POST /api/channel/:id/claude/usage/reset`：
  - 全部重置的请求体为 `{program:"cedar_ember", grant_id, resets_left}`；
  - 5 小时重置为 `{program:"juniper_tide"}`。
- 执行中按钮禁用。成功后显示成功提示并调用弹窗已有的 `onRefresh` 重新拉用量；失败显示接口返回的 `message`。
- 卡片全部文字走 i18n。

## 涉及文件
- 修改 `web/src/features/channels/lib/subscription-usage.ts`
- 新建 `web/src/features/channels/components/dialogs/claude-limit-reset-card.tsx`
- 修改 `web/src/features/channels/components/dialogs/claude-usage-dialog.tsx`
- 修改 `web/src/features/channels/api.ts`
- 修改 `web/src/features/channels/lib/__tests__/subscription-usage.test.ts`
- 修改 `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`

## 函数清单
### web/src/features/channels/lib/subscription-usage.ts
| 函数名 | 职责 |
|---|---|
| `resolveClaudeLimitResets` | 纯函数：输入接口响应与当前时间，输出卡片状态（状态 2～5）以及两行要显示的数据（说明、过期 / 冷却时间、要清空的窗口、按钮能否点、执行时的请求体） |
| `parseIsoTimestamp` | ISO 8601 字符串转 Unix 秒，无效返回 0；供上面的函数和卡片显示使用 |

### web/src/features/channels/api.ts
| 函数名 | 职责 |
|---|---|
| `ClaudeUsageResponse`（类型） | 增加可选字段 `limit_reset_unavailable` |
| `ClaudeLimitResetRequest` / `ClaudeLimitResetResponse`（类型） | 重置接口的请求和响应 |
| `resetClaudeLimit` | POST `/api/channel/${channelId}/claude/usage/reset` |

### web/src/features/channels/components/dialogs/claude-limit-reset-card.tsx（新建）
| 函数名 | 职责 |
|---|---|
| `ClaudeLimitResetCard` | 渲染卡片的五种状态和两行；管理确认框、执行中、成功 / 失败提示；成功后调用父组件传入的刷新回调 |

### web/src/features/channels/components/dialogs/claude-usage-dialog.tsx
| 函数名 | 职责 |
|---|---|
| `ClaudeUsageDialog` | 在窗口网格和原始 JSON 之间放 `ClaudeLimitResetCard`，传入 `channelId`、`response`、`isRefreshing`、`onRefresh`；关闭弹窗时清掉卡片的提示状态 |

## 协作关系
`ClaudeUsageDialog` → `ClaudeLimitResetCard` → `resolveClaudeLimitResets`（纯函数）；确认后 → `resetClaudeLimit` → 成功时调 `onRefresh`（`channels-columns.tsx` 里已有的刷新逻辑会重新调用 `getClaudeUsage`）。确认框用 `@/components/confirm-dialog`，提示用现有的 `Alert`、`Skeleton`，按钮用 `Button`；实施前先读 `web/AGENTS.md` 和 shadcn-ui skill，按里面的复用优先级选组件。可复用的现有 i18n 键（如 `Reset completed`、`Reset failed`、`Resetting...`）直接用，新键按 i18n-translate skill 补齐 7 种语言。

## 验证方式
- 测试入口：`cd web && bun run test src/features/channels/lib/__tests__/subscription-usage.test.ts`
- 测试输入与预期（`resolveClaudeLimitResets`，固定当前时间为 `2026-10-08T05:00:00Z`）：
  - 实测样本：`cedar_ember` 为 `eligible=true`、`next_grant_id="opus55-launch-promax-20260921"`，授予记录 `resets_left=1`、`resets_total=1`、`usable_now=true`、`paused=false`、`use_requires_limit=false`、`ends_at="2026-10-22T16:00:00+00:00"`、`clears=["five_hour","seven_day","seven_day_overage_included"]`；`juniper_tide` 为 `eligible=false`、`ineligible_reason="not_at_wall"`。预期：状态为显示两行；全部重置按钮可点，请求体为 `{program:"cedar_ember", grant_id:"opus55-launch-promax-20260921", resets_left:1}`，要清空的窗口依次是 5 小时窗口、每周窗口、`seven_day_overage_included`；5 小时重置没有按钮，原因为「5 小时额度用完时才会出现」。
  - 同一样本，`cooldown_until="2026-10-08T05:10:00Z"` → 全部重置按钮不可点，带冷却结束时间。
  - 六个条件逐个改成不满足（`eligible=false`、`usable_now=false`、`paused=true`、`resets_left=0`、`use_requires_limit=true` 且 `at_limit=false`、冷却未过）→ 每种情况按钮都不可点。
  - `cedar_ember.ineligible_reason="surface"`、`grants=[]` → 全部重置为「目前没有」。
  - 两块都为 null → 状态 4；带 `limit_reset_unavailable:"client_version"` → 状态 3；`success=false` → 状态 2。
  - `juniper_tide` 为 `eligible=true`、`available=true` → 5 小时重置按钮可点，请求体为 `{program:"juniper_tide"}`。
  - `parseIsoTimestamp("bad")` → 0。
- [ ] 上述测试通过
- [ ] `bun run typecheck`、`bun run lint`、`bun run build` 通过；`bun run i18n:sync` 后 7 种语言都有新键
- [ ] 真机：打开 Claude 用量弹窗，卡片位置、两行文案、确认框和预览页第二部分一致；Codex 弹窗没有这张卡片；执行重置由用户亲手点一次，确认成功提示和用量刷新
