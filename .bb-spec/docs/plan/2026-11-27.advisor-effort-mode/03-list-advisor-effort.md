---
name: 03-list-advisor-effort
description: 智能路由列表的顾问列第二行显示力度摘要，超出模型能力时显示黄字
---
# 前端：列表里显示顾问的力度

## 目标
智能路由页列表的「顾问」列，不打开窗口也能看到顾问的力度设置，以及会不会被换档。

## 业务规则（来源：PRD 02-advisor-effort-settings）
- 「顾问」列第一行是顾问模型名；第二行是灰色小字「力度摘要 · 连续 N 次不满意」，比如「跟随打分 medium～max · 连续 2 次不满意」。
- 力度摘要有四种写法：不覆盖、固定 high、跟随打分 medium～max、不支持思考。写法和档位列相同。
- 放不下时省略，鼠标移上去显示全文，沿用列表现有的做法。
- 顾问设的力度超出模型能力时：力度摘要变黄色，下面多一行黄字「超出模型能力，实际按 <实际档位> 发送」，和档位列相同。
- 顾问模型的思考能力还没查到时不显示黄字。这和档位列用 `known` 判断的规则一样。
- 文案沿用已有的键：`After {{count}} dissatisfied messages`、`Exceeds the model capability, sent as {{effort}}`，以及档位摘要用的那几个键。

## 涉及文件
- 修改 `web/src/features/system-settings/request-policies/smart-routing/routes-card.tsx`
- 修改 `web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx`（扩展）

## 函数清单
### routes-card.tsx
| 函数名 | 职责 |
|---|---|
| 列定义中 `advisor` 列的 `cell`（修改） | 用 `effectiveCapability` 和 `tierEffortSummary` 算顾问的力度摘要，拼成「摘要 · 连续 N 次不满意」；用 `effortOverflow` 判断是否超出，超出时摘要加 `text-warning`，并多渲染一行黄字。结构照抄档位列的 `cell` |

列表页查思考能力用的是 `capabilityModelsOfRoutes`，02 已经把顾问模型加进去，这里不用再改。

## 协作关系
`advisor` 列 `cell` 依次调用 `effectiveCapability`、`tierEffortSummary`、`effortOverflow`（都在 02 改成收 `SmartEffortSetting`），再用 `TruncatedCell` 渲染。

## 验证方式
- 测试入口：`smart-routing.test.tsx` 里已有的列表渲染夹具（mock 设置接口与思考能力接口）。
- 测试输入：4 条路由，顾问分别是「不覆盖」、固定 high、跟随打分 medium～max、不支持思考的模型；另 1 条顾问固定 max，但模型最高只支持 high。
- 预期结果：
- [ ] 顾问列第二行依次是「Not overridden · After 2 dissatisfied messages」「Fixed high · After 3 dissatisfied messages」「Follow scoring medium～max · After 2 dissatisfied messages」「No thinking · After 2 dissatisfied messages」。
- [ ] 固定 max 那一行：摘要带警告色，下面有「Exceeds the model capability, sent as high」。
- [ ] 思考能力接口还没返回时，不显示黄字。
- [ ] 在 `web/` 下 `bun run test`、`bun run lint` 通过。
