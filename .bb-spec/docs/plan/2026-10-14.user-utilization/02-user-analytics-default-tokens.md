---
name: 02-user-analytics-default-tokens
description: 用户分析的指标切换默认选中 Tokens，仍可切回金额，不记忆上次选择
---
# 用户分析默认按 Token 显示

## 目标
打开数据看板的「用户分析」时，指标切换默认选中「Tokens」，两张图按 Token 用量画。

## 业务规则（来源：PRD 01-user-analytics-default-tokens）
- 没有做过选择时 → 指标切换选中「Tokens」，「用户消耗排行」和「用户消耗趋势」都按 Token 用量画。
- 点「金额」→ 两张图按金额画；再点「Tokens」→ 恢复按 Token 用量画。
- 刷新页面后重新打开 → 回到「Tokens」，不记住上次的选择（现状就是不记忆，保持不变）。

## 涉及文件
- `web/src/features/dashboard/index.tsx` 修改：`userChartsFilters` 的初始值里 `metric: 'quota'` 改为 `metric: 'tokens'`（现在位于第 216 行）

## 函数清单
### web/src/features/dashboard/index.tsx
| 函数名 | 职责 |
|---|---|
| `Dashboard` | `userChartsFilters` 的初始 `metric` 改为 `'tokens'`，其他不动 |

## 协作关系
`Dashboard` 把 `userChartsFilters` 传给 `UserCharts`（`components/users/user-charts.tsx`），由 `processUserChartData` 按 `metric` 画图。`UserCharts` 和 `processUserChartData` 都不需要改。

## 验证方式
- 测试入口：页面操作（这一处只改了初始值，不新增单元测试，避免锁死实现细节）。
- 测试输入：管理员登录，打开 `/dashboard/users`。
- 预期结果：指标切换选中「Tokens」，图上的数值是 Token 数；点「金额」后变成金额；刷新后又是「Tokens」。
- [ ] `bun run build` 通过
- [ ] 浏览器里按上面三步操作，结果都符合预期
