---
name: 04-scorer-effort-field
description: 编辑弹窗「打分」页签新增思考力度下拉，页签小字带上思考力度，补七种语言翻译
---
# 打分思考力度下拉与页签小字

## 目标
管理员在编辑虚拟模型弹窗的「打分」页签选打分思考力度，左栏页签小字显示「模型 · 思考力度」。

## 业务规则（来源：PRD，界面以确认过的原型为准）
- 来源 `01-scorer-effort`：
  - C1：「打分模型」下面新增一行「思考力度」（标签用已有键 `Reasoning effort`），单选下拉；第一项「不设置（模型默认）」（值为空串），其余只列打分模型在能力表里支持的档位，按 none→max 的顺序。下拉宽度和样式与档位力度下拉一致。
  - C2：打分模型「能力未确认」→ 只有「不设置」和 low/medium/high，行末黄色小标签「Capability not confirmed」，悬停提示用已有键「This model is not in the built-in capability table. Add it under Model thinking capabilities」。
  - C3：打分模型不支持思考 → 整行换成灰字（已有键「This model does not support reasoning effort. No reasoning parameter is sent with the request」），没有下拉；之前选的档位被清空。
  - C4：能力加载失败 → 下拉禁用，行末红字（已有键「Thinking capability failed to load」）；还没选打分模型 → 下拉禁用，显示「不设置（模型默认）」。
  - C5：「思考力度」旁 ⓘ 的说明：留空不发思考参数、由模型自己决定；选了档位而模型不支持时发出前换成最接近的一档；打分输出不限长度，耗时由「超时」控制；下拉只列这个模型支持的档位。
  - C6：左栏「打分」页签小字：没选模型 → 「Not set」；不支持思考 → 「<模型> · No thinking」；留空 → 「<模型> · Model default」；选了档位 → 「<模型> · <档位>」。
  - C10：旧配置没有 `effort` → 表单按空串处理，显示「不设置（模型默认）」。
  - C11：新增文字补齐 en、zh、zh-TW、fr、ru、ja、vi 七种语言。
- 原型确认的中文文案：「不设置（模型默认）」「模型默认」；帮助说明中文：「留空：请求里不发思考参数，由模型自己决定。选了档位：发给打分模型；模型不支持这一档时，发出前换成能力表里最接近的一档。打分输出不限长度，耗时由「超时」控制。下拉只列这个模型支持的档位」。

## 涉及文件
- 修改 `web/src/features/system-settings/request-policies/smart-routing/types.ts`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/route-form.ts`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/route-editor-panels.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/route-editor-dialog.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/lib.ts`
- 修改 `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`
- 修改 `web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/__tests__/route-model.test.ts`

## 函数清单
### types.ts / route-form.ts
| 函数名 | 职责 |
|---|---|
| `SmartRoute['scorer']`（类型修改） | 增加 `effort: SmartEffort \| ''` |
| `createRouteFormSchema`（修改） | `scorer.effort` 接受空串或七档之一 |
| `emptyRouteFormValues`（修改） | `scorer.effort` 默认空串；`routeToFormValues` 合并默认值后旧配置自然补成空串 |

### route-editor-panels.tsx
| 函数名 | 职责 |
|---|---|
| `ScorerPanel`（修改） | 在 `ModelRow` 与超时之间渲染 `ScorerEffortField`；新增 props：打分模型的能力、是否未确认、是否不可用、是否加载失败 |
| `ScorerEffortField`（新增，同文件） | 按 C1～C5 渲染下拉 / 灰字 / 禁用 / 红字与帮助气泡；复用 `tier-effort-field.tsx` 导出的 `FIELD_ROW_CLASS`、`FieldInfo`、`Select`、`Badge`、`Tooltip` 写法 |

### route-editor-dialog.tsx
| 函数名 | 职责 |
|---|---|
| `RouteEditorDialog`（修改） | `askedModels` 加入打分模型；算出打分模型的能力、未确认、不可用状态传给 `ScorerPanel`；打分模型确定为不支持思考时清空 `scorer.effort`（与档位清空的 effect 同样的判定：模型在内置结果或覆盖里） |

### lib.ts
| 函数名 | 职责 |
|---|---|
| `slotSummaries`（修改） | 打分小字按 C6 拼「模型 · 力度」；需要打分模型的能力，调用方传入 |

## 协作关系
- 能力计算复用 `capability.ts` 的 `effectiveCapability`；未确认、不可用的判定与档位相同（`isUnconfirmed` / `isUnavailable`）。
- 不复用 `TierEffortField` 本身：它的三种方式切换（不覆盖 / 固定 / 跟随打分）对打分不适用，表单字段路径也绑在 `tiers.*` 上；新控件只复用它的样式常量和子组件。
- 翻译按项目 `i18n-translate` skill 补齐七个语言文件，键为英文原文。

## 验证方式
- 测试入口：系统设置页「智能路由」区块渲染测试（`smart-routing.test.tsx` 现有夹具：`show()`、`pickModel()`、编辑弹窗打开方式）；`lib.ts` 导出的 `slotSummaries`。
- 测试输入与预期结果：
  - [ ] 打分模型选内置支持 none～max 的模型 → 「Reasoning effort」下拉选项依次为「Not set (model default)」、none、low、medium、high、xhigh、max。
  - [ ] 打分模型选不在内置表里的模型 → 选项只有「Not set (model default)」、low、medium、high，并显示「Capability not confirmed」。
  - [ ] 先选 low，再把打分模型换成内置不支持思考的模型 → 出现灰字说明、没有下拉；保存后提交的路由里 `scorer.effort` 为空串。
  - [ ] 能力接口报错 → 下拉禁用并显示「Thinking capability failed to load」；新建虚拟模型未选打分模型 → 下拉禁用。
  - [ ] 选 none 并保存 → 提交的路由 `scorer.effort` 为 `none`；左栏「Scoring」页签小字为「<模型> · none」。
  - [ ] `slotSummaries`：没选模型 → 「Not set」；不支持思考 → 「gpt-4.1 · No thinking」；留空 → 「gpt-5.6-luna · Model default」；low → 「gpt-5.6-luna · low」。
  - [ ] 旧配置（没有 `effort`）打开编辑弹窗 → 显示「Not set (model default)」，原样保存后 `scorer.effort` 为空串。
  - [ ] 后端返回 `scorer.effort` 字段错误 → 弹窗停在「Scoring」页签，下拉下方显示「Select a valid reasoning effort」。
- [ ] 在 `web/` 下 `bun run i18n:sync` 无缺失键；`bun run test`、`bun run lint`、`bun run typecheck` 通过。
