---
name: 03-utilization-section-ui
description: 数据看板新增仅管理员可见的「利用率」分区：四张卡片 + 四段比例条 + 开关提示 + 加载/出错/空状态
---
# 利用率分区界面

## 目标
管理员在数据看板点「利用率」，能看到开通人数、深度使用、经常使用、非活跃四张卡片，以及一条四段比例条；普通用户看不到这个分区。

## 业务规则（来源：PRD 02-utilization-section，界面已按用户确认的 HTML 预览定稿）
- 分区栏顺序：概览（不在 tab 中显示，保持现状）、模型调用分析、Flow、用户分析、利用率。「用户分析」和「利用率」只有管理员和 Root 能看到；普通用户直接访问 `/dashboard/utilization` 也看不到内容（参照 users 分区的处理方式）。
- 卡片从左到右：开通人数（说明「启用状态的普通用户」）；深度使用 · 占 X%（深色底、橙色数字，说明「近 7 天 Token 用量超过 1 亿」）；经常使用 · 占 X%（说明「近 7 天 Token 用量超过 100 万」）；非活跃 · 占 X%（虚线框、灰蓝数字，说明「近 30 天 Token 用量不超过 100 万」）。
- 卡片上方有一行范围说明：「只统计启用状态的普通用户，不含管理员和 Root。近 7 天、近 30 天均从现在往前推算。」
- 比例条从左到右四段：深度使用 = deep；经常使用（未达 1 亿）= frequent − deep；近 30 天超过 100 万 = moderate；非活跃 = inactive。段长和人数成正比，人数为 0 的段不画，段与段之间留 2px 空隙，两端是圆角。鼠标悬停时显示这一段的名称、「N 人 · 占 X%」和判定条件。
- 图例列出四段的色块、名称、「N 人 · X%」。
- 占比 = 人数 ÷ total，四舍五入到整数百分比；total 为 0 时，所有占比显示「—」，比例条显示「暂无普通用户」。
- `/api/status` 的 `enable_data_export` 为 false 时，在最顶部显示提示：标题「数据看板未开启，统计不完整」，说明「利用率按数据看板记录的用量计算。开关关闭期间的调用没有被记录，相关用户会被算成非活跃。可在 系统设置 → 数据看板 中开启。」为 true 时不显示。
- 加载中显示和卡片、比例条同样布局的骨架；请求失败显示 `ErrorState`，标题「利用率数据加载失败」，点「重试」会重新请求。
- 所有新增文字补齐 7 个语言文件。

## 涉及文件
- `web/src/features/dashboard/section-registry.tsx` 修改：新增 `utilization` 分区（`titleKey: 'Utilization'`、`adminOnly: true`），并加进 `ADMIN_ONLY_SECTIONS`
- `web/src/features/dashboard/index.tsx` 修改：`SECTION_META` 加上 utilization；`visibleSections` 的过滤条件从硬编码的 `'users'` 改为判断分区是否只有管理员可见（`isAdmin` 为 false 时，users 和 utilization 都隐藏）；新增懒加载的 utilization 渲染分支；非管理员访问 utilization 时的处理和 users 一致
- `web/src/features/dashboard/api.ts` 修改：新增请求函数
- `web/src/features/dashboard/types.ts` 修改：新增 `UserUtilization` 类型，字段与后端契约一致
- `web/src/features/dashboard/lib/utilization.ts` 新建：纯函数，把接口数据转成卡片和比例条要显示的数据
- `web/src/features/dashboard/lib/utilization.test.ts` 新建
- `web/src/features/dashboard/lib/index.ts` 修改：导出新函数（照现有 lib 的导出方式）
- `web/src/features/dashboard/components/utilization/utilization-overview.tsx` 新建
- `web/src/styles/theme.css` 修改：新增比例条与卡片的颜色变量
- `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json` 修改

## 成品定义

接口数据类型（`types.ts`）：
```ts
export interface UserUtilization {
  total_users: number
  deep_users: number
  frequent_users: number
  moderate_users: number
  inactive_users: number
}
```

请求：`GET /api/data/utilization`，返回 `{ success: boolean; message?: string; data: UserUtilization }`，用 `requireServerSuccess` 处理，写法照 `getUserQuotaDataByUsers`。react-query 的 key 是 `['dashboard', 'user-utilization']`，`staleTime: 60_000`。

颜色变量（`theme.css`，均已用 dataviz 校验脚本检查过，亮色和暗色下相邻两段的色盲区分都通过）：
```css
/* @theme inline 块内，紧跟 --color-overview-accent-3 */
--color-utilization-deep: var(--utilization-deep);
--color-utilization-frequent: var(--utilization-frequent);
--color-utilization-moderate: var(--utilization-moderate);
--color-utilization-inactive: var(--utilization-inactive);
--color-utilization-highlight: var(--utilization-highlight);
--color-utilization-highlight-number: var(--utilization-highlight-number);

/* :root 块内，紧跟 --overview-accent-3：比例条四段由深到浅表示用得多到少；非活跃是中性灰蓝 */
--utilization-deep: #a93e17;
--utilization-frequent: #e8875a;
--utilization-moderate: #f3cdb4;
--utilization-inactive: #475569;
--utilization-highlight: #111827;
--utilization-highlight-number: #e8692f;

/* .dark 块内：暗色底上越亮表示用得越多，所以顺序反过来 */
--utilization-deep: #f59466;
--utilization-frequent: #c2552a;
--utilization-moderate: #7a3b1f;
--utilization-inactive: #94a3b8;
--utilization-highlight: oklch(0.18 0.01 40);
--utilization-highlight-number: #f59466;
```

文案（英文 key → 简体中文；其余 5 种语言按 i18n-translate skill 翻译）：
| 英文 key | 简体中文 |
|---|---|
| `Utilization` | 利用率 |
| `Only enabled regular users are counted, excluding admins and Root. The last 7 and 30 days are counted back from now.` | 只统计启用状态的普通用户，不含管理员和 Root。近 7 天、近 30 天均从现在往前推算。 |
| `Enabled users` | 开通人数 |
| `Enabled regular users` | 启用状态的普通用户 |
| `Deep usage` | 深度使用 |
| `Frequent usage` | 经常使用 |
| `Inactive` | 非活跃（en.json 若已有同名 key 则复用） |
| `{{percent}} of users` | 占 {{percent}} |
| `Over 100M tokens in the last 7 days` | 近 7 天 Token 用量超过 1 亿 |
| `Over 1M tokens in the last 7 days` | 近 7 天 Token 用量超过 100 万 |
| `At most 1M tokens in the last 30 days` | 近 30 天 Token 用量不超过 100 万 |
| `Frequent usage (under 100M)` | 经常使用（未达 1 亿） |
| `Over 1M in the last 30 days` | 近 30 天超过 100 万 |
| `Over 1M but at most 100M tokens in the last 7 days` | 近 7 天超过 100 万、未达 1 亿 |
| `At most 1M in the last 7 days, over 1M in the last 30 days` | 近 7 天未达 100 万，近 30 天超过 100 万 |
| `At most 1M tokens in the last 30 days, including unused` | 近 30 天不超过 100 万（含未使用） |
| `{{count}} users · {{percent}}` | {{count}} 人 · {{percent}} |
| `No regular users yet` | 暂无普通用户 |
| `Data dashboard is off, statistics are incomplete` | 数据看板未开启，统计不完整 |
| `Utilization is calculated from usage recorded by the data dashboard. Calls made while it is off are not recorded, so those users count as inactive. Turn it on in System Settings → Data Dashboard.` | 利用率按数据看板记录的用量计算。开关关闭期间的调用没有被记录，相关用户会被算成非活跃。可在 系统设置 → 数据看板 中开启。 |
| `Failed to load utilization data` | 利用率数据加载失败 |

## 函数清单
### web/src/features/dashboard/lib/utilization.ts
| 函数名 | 职责 |
|---|---|
| `buildUtilizationView` | 输入 `UserUtilization`，输出卡片数据（开通人数和三类的人数、占比文本）和比例条四段（key、人数、占比文本）。占比四舍五入取整、带 `%`，total 为 0 时为 `'—'`；frequent−deep 为负时按 0 处理 |

### web/src/features/dashboard/api.ts
| 函数名 | 职责 |
|---|---|
| `getUserUtilization` | 请求 `/api/data/utilization` |

### web/src/features/dashboard/components/utilization/utilization-overview.tsx
| 函数名 | 职责 |
|---|---|
| `UtilizationOverview` | 用 `useQuery` 取数据，用 `useStatus` 读 `enable_data_export`；渲染开关提示、范围说明、四张卡片、比例条（段上用 `Tooltip` 显示悬停信息）和图例；加载中用 `Skeleton`，出错用 `ErrorState`（`onRetry` 调 `refetch`） |

## 协作关系
`Dashboard(activeSection === 'utilization') → Suspense + lazy(UtilizationOverview) → getUserUtilization + useStatus → buildUtilizationView`。
复用：`ErrorState`（`components/error-state.tsx`）、`Alert`（`components/ui/alert.tsx`，用 className 加警告色）、`Skeleton`、`Tooltip`、`FadeIn`、`formatNumber`（`@/lib/format`）。
不复用 `StatCard`（`dashboard/components/ui/stat-card.tsx`）的原因：它必须带图标，字号固定，也没有深色高亮卡和虚线框这两种样式，而用户确认的预览需要 56px 大数字和这两种卡片样式。把这些能力加进共享组件，会让概览页的卡片也多出用不到的分支。这条理由要写进最终的改动说明。

## 验证方式
- 测试入口：`buildUtilizationView`（vitest，`bun run test`）；页面 `/dashboard/utilization`。
- 测试输入与预期：
  - `{total 36, deep 13, frequent 21, moderate 13, inactive 2}` → 卡片占比依次为 deep `36%`、frequent `58%`、inactive `6%`；比例条四段人数为 `[13, 8, 13, 2]`，占比为 `['36%', '22%', '36%', '6%']`。
  - 全为 0 → 所有占比 `'—'`，四段人数都是 0。
  - `{total 3, deep 1, frequent 1, moderate 1, inactive 1}` → 第二段（frequent−deep）人数为 0。
- [ ] `bun run test` 通过（含新测试）；`bun run build`、`bun run lint` 通过
- [ ] 浏览器里管理员看到的分区栏、卡片、比例条、图例、悬停提示，和确认过的预览（`.bb-spec/.cache/ui-preview/utilization.html`）逐项一致，亮色和暗色都对
- [ ] 关掉数据看板开关后出现提示，打开后提示消失
- [ ] 普通用户看不到「利用率」，直接访问 `/dashboard/utilization` 也看不到数据
- [ ] 切换成 English 后，新增文字都有翻译
