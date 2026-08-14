---
name: list-display
description: 渠道列表订阅渠道打标：常量 + API 函数 + 用量 cell（使用率/进度条/tooltip）+ 状态列触顶徽标 + i18n + 测试。
---

# 渠道列表订阅用量打标

## 目标

管理端渠道列表中，订阅渠道（type 57 Codex / type 61 Claude Subscription）行显示瓶颈使用率与触顶标，悬停可见最后刷新与预计下次刷新；非订阅渠道行与现状完全一致。

## 业务规则（来源：spec admin-list-display）

- 列表加载后额外请求一次 `GET /api/channel/subscription_usage`（响应 `{success, data: {"<channelId>": {bottleneck_percent, refreshed_at, saturated}}}`，`refreshed_at` 为 Unix 毫秒），按渠道 ID 合并到订阅渠道行。
- 用量：订阅渠道的余额列位置显示瓶颈使用率百分比 + 迷你进度条。
- 触顶标：`saturated=true` 时状态列在「已启用」徽标旁追加红色「触顶」徽标；false 时无此徽标。触顶判定只读端点 `saturated` 字段，前端禁止用 `bottleneck_percent >= 95` 重算。
- tooltip（悬停触顶标或使用率）：瓶颈使用率（附触顶阈值 95 说明）、最后刷新时间（绝对 + 相对）、预计下次刷新 = `refreshed_at` + 类型周期（57→60 秒、61→180 秒），文案注明预计值、轮询失败或渠道禁用时不兑现。
- 降级：端点无该渠道条目 → 用量位置显示「暂无数据」，无触顶标与进度条；`refreshed_at` 距今 >10 分钟 → 使用率保留最后已知值 + 「已过期」徽标；端点请求失败 → 列表正常渲染，订阅渠道按「暂无数据」处理。
- 所有新增文案走 i18n（英文源串作 key），7 语言（en/zh/zh-TW/fr/ja/ru/vi）键集对称。
- 不改渠道列表接口请求/响应结构，不新增持久化字段。

## 涉及文件（均在 `web/src/features/channels/` 下，另注除外）

- `constants.ts` — 修改（订阅类型集合 + 轮询周期映射）
- `api.ts` — 修改（请求函数 + 响应类型）
- `components/subscription-usage-cell.tsx` — 新建（hook + 用量 cell + 触顶徽标组件）
- `components/channels-columns.tsx` — 修改（balance 列分流、status 列追加触顶徽标）
- `lib/channel-utils.ts` — 修改（毫秒时间戳的过期判定与预计下次刷新纯函数）
- `lib/__tests__/subscription-usage.test.ts` — 新建（纯函数测试）
- `components/__tests__/subscription-usage-cell.test.tsx` — 新建（组件测试）
- `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json` — 修改（新增文案键）

## 函数清单

### constants.ts

| 常量 | 职责 |
|---|---|
| `SUBSCRIPTION_CHANNEL_TYPES` | `Set([57, 61])`，与后端 `constant.IsSubscriptionChannel` 对齐的订阅渠道类型集合（先例 `MODEL_FETCHABLE_TYPES`） |
| `SUBSCRIPTION_USAGE_POLL_INTERVAL_SECONDS` | 类型→轮询周期秒数映射（57→60、61→180），供预计下次刷新推算 |

### api.ts

| 函数/类型 | 职责 |
|---|---|
| `SubscriptionUsageResponse`（type） | 端点响应类型：`data` 为 `Record<string, {bottleneck_percent, refreshed_at, saturated}>` |
| `getSubscriptionUsage` | `api.get('/api/channel/subscription_usage', channelActionConfig())`；`channelActionConfig` 静默业务错误，失败由调用方降级 |

### lib/channel-utils.ts

| 函数 | 职责 |
|---|---|
| `isSubscriptionUsageExpired` | 判定毫秒级 `refreshed_at` 距今是否超过 10 分钟 |
| `estimateNextSubscriptionRefresh` | 由 `refreshed_at` 与渠道类型周期算出预计下次刷新毫秒时间戳 |

### components/subscription-usage-cell.tsx

| 函数/组件 | 职责 |
|---|---|
| `useSubscriptionUsage` | react-query hook：queryFn 调 `getSubscriptionUsage`，失败返回空 map；同 queryKey 在多个 cell 间自动去重（先例 `useGroupRatios`），不动 `useChannelsColumns` 的 useMemo deps |
| `SubscriptionUsageCell` | 余额列位置的用量展示：百分比 + `Progress` 迷你条（范本 `user-quota-cell.tsx`）+ Tooltip（使用率/阈值说明/最后刷新绝对+相对/预计下次刷新+预计值说明）；无条目渲染「暂无数据」StatusBadge；过期时追加「已过期」徽标；type 57 保留点击打开 `CodexUsageDialog` 的既有入口，type 61 不可点击 |
| `SubscriptionSaturationBadge` | `saturated=true` 时渲染 `StatusBadge variant='danger'` 「触顶」，false/无条目渲染 null；内部经 `useSubscriptionUsage` 取数 |

### components/channels-columns.tsx

无新函数：balance 列 cell 按 `SUBSCRIPTION_CHANNEL_TYPES.has(type)` 分流到 `SubscriptionUsageCell`（非订阅渠道走既有 `BalanceCell` 不动，`BalanceCell` 内被取代的 type 57 徽标特判随之清理）；status 列在既有 `StatusBadge` 返回处（含 status===3 分支）外包 flex 容器追加 `SubscriptionSaturationBadge`。

## 协作关系

- `SubscriptionUsageCell` / `SubscriptionSaturationBadge` → `useSubscriptionUsage` → `getSubscriptionUsage` → 后端端点（plan usage-endpoint 产物）。
- 相对时间复用 `formatRelativeTime`（入参秒级，毫秒需除 1000）与 `formatTimestampToDate`；进度条复用 `@/components/ui/progress`；徽标复用 `@/components/status-badge`；Tooltip 复用 `@/components/ui/tooltip`（TooltipTrigger render prop 形式）。
- 卡片视图经 `channel-card.tsx` 的 `renderCell` 自动复用列 cell，无需改动。
- i18n 新键随代码提交补齐 7 语言，`bun run i18n:sync` 作对称校验。

## 验证方式

- 测试入口：`bun test`（web/ 下，node:test + happy-dom）；纯函数直接 import；组件测试用 happy-dom 引导范本（先例 `api-key-group-cell.test.tsx`）渲染组件断言 DOM。
- 测试输入：构造用量条目（含/不含渠道、saturated 真/假、refreshed_at 新鲜/陈旧 11 分钟）；组件以 props 或 mock query 数据驱动。
- 预期结果：
  - `isSubscriptionUsageExpired`：距今 9 分钟 false、11 分钟 true。
  - `estimateNextSubscriptionRefresh`：type 57 为 refreshed_at+60000ms，type 61 为 refreshed_at+180000ms。
  - 用量 cell：有条目（62.4，新鲜）渲染 `62.4%` 文本与进度条；无条目渲染「暂无数据」文案；陈旧条目额外出现「已过期」文案。
  - 触顶徽标：saturated=true 渲染「触顶」文案；false 渲染无该文案。
  - `bun run i18n:sync` 通过，7 语言键集对称；`bun run typecheck` 通过。
- [ ] 上述纯函数与组件分支各有断言精确期望值的测试。
- [ ] 非订阅渠道行渲染路径未被触碰（BalanceCell 对非订阅类型行为不变）。
