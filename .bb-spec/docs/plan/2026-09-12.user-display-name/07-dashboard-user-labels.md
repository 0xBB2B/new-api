---
name: dashboard-user-labels
description: 看板用户排行/趋势图坐标轴与图例显示名称、tooltip 标签「名称 · 用户名 · ID:{id}」；流向图用户节点标签用名称解析。
---

# 看板用户标签

## 目标

数据看板的用户消耗排行图、用户趋势图与流向图用户节点以用户名称为可见标签；排行/趋势图 tooltip 携带用户名与 ID。

## 业务规则（来源：spec id-in-tooltip / name-resolution）

- 排行/趋势图仍以 username 作为分组键、系列键与颜色键（唯一）；坐标轴与图例显示名称。
- tooltip 标签：`名称 · 用户名 · ID:{id}`；名称与用户名相同（含显示名为空回退到用户名）时省略用户名段，即 `用户名 · ID:{id}`。
- 名称解析：display_name（去空白）→ username → `User {{id}}`。
- 流向图用户节点标签为名称；节点 id 仍按 `user:{user_id}`（无 id 时 `user:{username}`）不变，筛选与去重不受影响。

## 涉及文件

- `web/src/features/dashboard/types.ts` — 修改（`FlowQuotaDataItem` 加 `display_name?: string`）
- `web/src/features/dashboard/lib/charts.ts` — 修改（`processUserChartData`）
- `web/src/features/dashboard/lib/charts.test.ts` — 修改（重写用户图表用例）
- `web/src/features/dashboard/lib/flow.ts` — 修改（`userNode`）
- `web/src/features/dashboard/lib/flow.test.ts` — 修改（追加用户节点标签用例）

## 函数清单

### charts.ts（修改）

| 函数 | 职责 |
|---|---|
| `processUserChartData`（既有，改逻辑） | 遍历数据时为每个 username 记录 `user_id` 与解析后的名称，构建 `userLabels`（tooltip 标签，格式见业务规则）与 `userNames`（坐标轴/图例名称）两个 map；`spec_user_rank` 的 band 轴与 `spec_user_trend` 的 `legends` 通过 VChart `label.formatMethod`（axes）/ `legends.item.label.formatMethod` 把 username 映射为名称；tooltip `key` 仍取 `Label` |

`resolveUserName` 来自 04 的 `@/lib/user-identity`；`processUserChartData` 不在 React 上下文中，`t` 用 i18next 单例（与项目 `i18n` 非组件用法一致）。

### flow.ts（修改）

| 函数 | 职责 |
|---|---|
| `userNode`（既有，改逻辑） | `label` 改为 `resolveUserName({user_id, username, display_name}, t)`；`id` 逻辑不变 |

## 协作关系

- 依赖 02（流向响应 `display_name`；按用户分组接口已带该字段）、04（`resolveUserName`）。
- 图表 UI 组件（`components/models/*`、`components/flow/*`）不改，只消费 `charts.ts` / `flow.ts` 产出的 spec 与节点。
- 流向图按用户筛选（`flow-node-filter`）用节点 `id` 与 `label`，label 改为名称后筛选下拉显示名称、匹配仍按 id。

## 验证方式

- 测试入口：vitest 直接调用 `processUserChartData`、`buildFlowPaths`（flow.ts 既有测试入口）。
- 测试输入（沿用既有 `charts.test.ts` 数据）：`{1, oidc_1, 'Alice Liddell'}`、`{2, bob, 'bob'}`、`{9, ghost, 无 display_name}`。
- 预期结果：
  - 排行值的 `User` 仍为 `['oidc_1', 'bob', 'ghost']`。
  - tooltip `key` 依次为 `Alice Liddell · oidc_1 · ID:1`、`bob · ID:2`、`ghost · ID:9`。
  - 排行图 band 轴 `label.formatMethod('oidc_1')` 返回 `Alice Liddell`；趋势图图例 `formatMethod('oidc_1')` 返回 `Alice Liddell`。
  - 流向图：行 `{user_id: 1, username: 'oidc_1', display_name: 'Alice Liddell'}` 生成用户节点 `id='user:1'`、`label='Alice Liddell'`；无 display_name 的行 label 为 username。
- [ ] `charts.test.ts`、`flow.test.ts`、`flow-selection.test.ts` 全部通过。
- [ ] `bun run typecheck && bun run lint && bun run test` 通过。
