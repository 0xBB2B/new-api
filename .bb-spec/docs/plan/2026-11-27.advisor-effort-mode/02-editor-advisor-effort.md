---
name: 02-editor-advisor-effort
description: 路由编辑窗口的顾问面板加「思考力度」一行，顾问标签显示力度摘要并可悬停看全文
---
# 前端：编辑窗口里设置顾问的力度方式

## 目标
管理员在路由编辑窗口的「顾问」标签里设置顾问的力度方式。控件和三个档位面板的「思考力度」一行完全一样。左侧「顾问」标签第二行显示「模型名 · 力度摘要」。

## 业务规则（来源：PRD 02-advisor-effort-settings）
- 顾问面板从上到下四行：顾问模型、思考力度（新增）、连续不满意几次后交给顾问、分组顺序。其余三行不变。
- 「思考力度」一行和档位面板的完全一样：
  - 三个方式按钮：不覆盖、固定、跟随打分；
  - 固定时一个下拉，跟随打分时「最低 ～ 最高」两个下拉，下拉只列顾问模型支持的档位；
  - 说明气泡的文字相同；
  - 切换方式时清空其他方式的值；
  - 模型不支持思考时，显示灰色的「该模型不支持思考力度，请求时不发送思考参数」，并清空已选的力度设置；
  - 能力未确认时显示「能力未确认」徽标，下拉只列 low、medium、high；
  - 能力加载中，方式按钮不可点；
  - 能力加载失败，方式按钮不可点，并显示「思考能力加载失败」。
- 默认值：添加窗口里，以及编辑顾问没有力度字段的已有路由时，选中「不覆盖」。
- 校验和档位相同：
  - 固定时没选档位 → 1 个错误；
  - 跟随打分时两个都没选 → 2 个错误，两行「请选择有效的思考力度」；
  - 最低高于最高 → 「最高」报「最低力度不能高于最高力度」；
  - 有错误时，左侧「顾问」标签出红点，摘要变红，显示「N 个错误」。
- 左侧「顾问」标签第二行：
  - 有顾问模型时写「模型名 · 力度摘要」，力度摘要是「不覆盖」「固定 high」「跟随打分 medium～max」「不支持思考」之一；
  - 没选顾问模型时写「未设置」；
  - 放不下时省略，鼠标移上去出气泡显示全文。气泡只给顾问标签加，「打分」「档位」标签保持原样。
- 新增文案在 7 种语言文件里都要有翻译。沿用已有的英文键（`Not overridden`、`Fixed {{effort}}`、`Follow scoring {{min}}～{{max}}`、`No thinking`、`Not set`）时，不新增键。

## 涉及文件
- 修改 `web/src/features/system-settings/request-policies/smart-routing/types.ts`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/route-form.ts`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/capability.ts`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/lib.ts`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/tier-effort-field.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/route-editor-panels.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/route-editor-dialog.tsx`
- 修改 `web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx`（扩展）
- 只有出现新英文键时才修改 `web/src/i18n/locales/*.json`

## 成品定义
前端 `SmartRoute` 类型中 `advisor` 的结构，与后端 `smart_routing_setting.routes` 的 `advisor` 对象一致：

```ts
// 档位和顾问共用的思考力度设置
export type SmartEffortSetting = {
  effort_mode: SmartEffortMode
  effort: SmartEffort | ''
  effort_min: SmartEffort | ''
  effort_max: SmartEffort | ''
}
export type SmartRouteTier = SmartEffortSetting & {
  model: string
  groups: string[]
}
export type SmartRouteAdvisor = SmartEffortSetting & {
  model: string
  groups: string[]
  after_dissatisfied: number
}
// SmartRoute.advisor 的类型改为 SmartRouteAdvisor
```

## 函数清单
### types.ts
| 函数名 | 职责 |
|---|---|
| `SmartEffortSetting` / `SmartRouteTier` / `SmartRouteAdvisor`（类型） | 按成品定义调整 |

### route-form.ts
| 函数名 | 职责 |
|---|---|
| `createRouteFormSchema`（修改） | 把档位的力度字段和 `superRefine` 校验抽成一个力度对象 schema，档位和顾问共用；`advisor` 加四个力度字段 |
| `emptyRouteFormValues`（修改） | `advisor` 默认四个力度字段都为空串 |
| `routeToFormValues`（不改） | 已用默认值合并 `advisor`，旧路由自然得到空力度 |

### capability.ts
| 函数名 | 职责 |
|---|---|
| `capabilityModelsOfRoutes`（修改） | 要查思考能力的模型里加上顾问模型 |
| `effortOverflow`（修改类型） | 参数改收 `SmartEffortSetting`，供档位和顾问共用 |

### lib.ts
| 函数名 | 职责 |
|---|---|
| `tierEffortSummary`（修改类型） | 参数改收 `SmartEffortSetting` |
| `slotSummaries`（修改） | 多收顾问能力；`advisor` 摘要改为「模型 · 力度摘要」，没模型时写「未设置」 |

### tier-effort-field.tsx
| 函数名 | 职责 |
|---|---|
| `TierEffortField`（修改） | 字段前缀由 `tierId` 改为 `base`（`tiers.<档位>` 或 `advisor`），其余行为不变 |
| `EffortMessage`（修改类型） | `name` 允许顾问的力度字段 |

### route-editor-panels.tsx
| 函数名 | 职责 |
|---|---|
| `AdvisorPanel`（修改） | 在顾问模型行下面渲染 `TierEffortField base='advisor'`；新增能力、能力未确认、加载中、加载失败几个 props |
| `TiersPanel`（修改调用） | 按新的 `base` 参数传值 |

### route-editor-dialog.tsx
| 函数名 | 职责 |
|---|---|
| 编辑窗口组件（修改） | 计算顾问能力并传给 `AdvisorPanel` 和 `slotSummaries`；顾问模型不支持思考时清空顾问的四个力度字段和错误，写法与档位相同；左侧标签只在顾问那一项用 `Tooltip` 包住，悬停显示摘要全文 |

## 协作关系
`AdvisorPanel` → `TierEffortField` → react-hook-form 的 `advisor.*` 字段。编辑窗口 → `getThinkingCapabilities`，模型列表来自 `capabilityModelsOfRoutes`，然后 → `effectiveCapability` → `slotSummaries`。保存时 → zod schema；后端会再用 01 的规则校验一遍。

## 验证方式
- 测试入口：`smart-routing.test.tsx` 里已有的编辑窗口渲染夹具（mock 思考能力接口与保存接口）；`slotSummaries`、`capabilityModelsOfRoutes`、`createRouteFormSchema` 这些纯函数。
- 测试输入：顾问模型 gpt-6.2-astra，能力是 low、medium、high、xhigh、max；另有一个不支持思考的模型、一个能力未确认的模型；能力接口失败；一条顾问没有力度字段的旧路由。
- 预期结果：
- [ ] 切到「顾问」→ 四行顺序正确，「不覆盖」选中；点「固定」展开下拉，列出 5 档。
- [ ] 跟随打分不选范围就保存 → 两行「Select a valid reasoning effort」，顾问标签「2 errors」；最低 max、最高 medium → 「The minimum effort cannot be higher than the maximum」。
- [ ] 选跟随打分 medium～max 并保存 → 保存接口收到的 `advisor` 含 `effort_mode: 'follow'`、`effort_min: 'medium'`、`effort_max: 'max'`。
- [ ] 顾问换成不支持思考的模型 → 显示不支持思考那一行，保存时四个力度字段为空；能力未确认 → 下拉只有 low、medium、high，并显示徽标；接口失败 → 按钮禁用，并显示失败文字。
- [ ] `slotSummaries`：跟随打分 medium～max → 「gpt-6.2-astra · Follow scoring medium～max」；没有模型 → 「Not set」；不支持思考 → 「<模型> · No thinking」。
- [ ] 鼠标移到顾问标签 → 气泡显示摘要全文。
- [ ] `capabilityModelsOfRoutes` 结果里有顾问模型。
- [ ] 在 `web/` 下 `bun run test`、`bun run typecheck`、`bun run lint`、`bun run i18n:sync` 都通过，没有缺失的翻译键。
