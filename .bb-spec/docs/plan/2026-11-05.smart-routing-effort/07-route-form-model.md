---
name: 07-route-form-model
description: 前端虚拟模型类型、表单校验、摘要文字和思考能力合并逻辑，跟上新的配置结构
---
# 前端配置模型与摘要

## 目标
前端能读写新的虚拟模型配置（接口多选、三种力度方式）和思考能力覆盖，能算出表格、Tab 摘要要显示的文字，能把内置能力和草稿覆盖合并成有效能力。

## 业务规则（来源：PRD）
- 03-endpoint-formats C1：接口三选多，至少一项，新建默认只选 Responses。显示简称：Responses / Chat / Anthropic。
- 01-tier-effort C1：力度方式不覆盖 / 固定 / 跟随打分，新建默认不覆盖；可选值七档；跟随打分最低不能高于最高。
- 02-thinking-capability C2/C3：有效能力 = 草稿覆盖 ?? 内置；内置未知时为未确认，按 low / medium / high。
- 02 C9：档位存的力度（固定值或跟随打分的最低、最高）超出有效能力 → 需要提示「超出模型能力，实际按 <X> 发送」，X 是最高那个值按「最接近、一样近取更高」收敛后的结果；不支持思考时不提示（显示「不支持思考」）。
- 04-editor-layout C2：左栏摘要：打分、顾问写模型名，没选写「未设置」；档位写「<已设置档数>/3 已设置」（档位模型选了就算已设置）。
- 04 C5/C13：档位力度摘要是「不覆盖」「固定 medium」「跟随打分 medium～xhigh」「不支持思考」；表格名称第二行「<接口简称用 / 连接> 接口」；档位第二行「<力度摘要> · <分组用 → 连接>」。
- 后端字段见 01 plan 的成品定义：`endpoints`、`effort_mode`（"" / fixed / follow）、`effort`、`effort_min`、`effort_max`；覆盖选项键 `smart_routing_setting.thinking_capabilities`，值 `{模型: {no_thinking, efforts}}`。

## 涉及文件
- 修改 `web/src/features/system-settings/request-policies/smart-routing/types.ts`、`route-form.ts`、`lib.ts`、`api.ts`
- 新建 `web/src/features/system-settings/request-policies/smart-routing/capability.ts`
- 修改 7 个 `web/src/i18n/locales/*.json`（本 plan 用到的新文案）
- 测试：新建 `web/src/features/system-settings/request-policies/smart-routing/__tests__/route-model.test.ts`

## 函数清单
### types.ts
| 函数名 | 职责 |
|---|---|
| `SmartRouteTier`、`SmartRoute`、`SmartEndpoint`、`SmartEffortMode`、`SmartEffort`、`ThinkingCapabilityOverride` | 类型跟上新结构 |
### route-form.ts
| 函数名 | 职责 |
|---|---|
| `createRouteFormSchema` | 增加接口至少一项、按方式校验力度、最低不高于最高 |
| `emptyRouteFormValues` | 新建默认：接口只选 Responses，三档不覆盖 |
### lib.ts
| 函数名 | 职责 |
|---|---|
| `SMART_THINKING_CAPABILITIES_OPTION_KEY` | 新选项键常量 |
| `parseThinkingCapabilities` / `serializeThinkingCapabilities` | 覆盖选项的读写 |
| `endpointShortNames` | 接口列表 → 「Responses / Anthropic」 |
| `tierEffortSummary` | 档位 + 有效能力 → 力度摘要文字 |
| `slotSummaries` | 表单值 → 左栏三个 Tab 的摘要 |
| `routeEndpoints`（删除） | 一期按模型推接口的逻辑去掉 |
### capability.ts
| 函数名 | 职责 |
|---|---|
| `effectiveCapability` | 合并内置与草稿覆盖，给出来源 builtin / override / unconfirmed |
| `clampEffort` | 收敛到最近一档，一样近取更高 |
| `effortOverflow` | 档位存的力度是否超出能力，超出时给出实际发送的档位 |
| `tierModelsOfRoutes` | 所有虚拟模型三档用到的模型，去重、保持首次出现顺序 |
### api.ts
| 函数名 | 职责 |
|---|---|
| `getBuiltinThinkingCapabilities` | 调 `GET /api/option/smart_routing/thinking_capabilities?models=...` |
| `validateSmartRoute` | 返回值去掉 `endpoints` |

## 协作关系
被 09（弹窗）、10（表格和能力卡片）使用。`effectiveCapability` / `clampEffort` 的规则和后端 02 plan 相同。

## 验证方式
- 测试入口：上面列出的导出函数（vitest，`bun run test`）
- 测试输入：各种路由对象、覆盖表、内置能力响应
- 预期结果：
  - [ ] `emptyRouteFormValues()` → `endpoints=['openai-response']`，三档 `effort_mode=''`
  - [ ] 表单校验：接口为空 → 「Select at least one endpoint」；跟随打分 high～low → 报错；固定且没选值 → 报错
  - [ ] `endpointShortNames(['openai-response','anthropic'])` → 「Responses / Anthropic」
  - [ ] `tierEffortSummary`：不覆盖 → 「不覆盖」；固定 medium → 「固定 medium」；跟随 medium～xhigh → 「跟随打分 medium～xhigh」；能力不支持思考 → 「不支持思考」
  - [ ] `slotSummaries`：新建 → 「未设置」「0/3 已设置」「未设置」；三档都选了模型 → 「3/3 已设置」
  - [ ] `effectiveCapability`：草稿覆盖优先；内置未知 → unconfirmed、low / medium / high
  - [ ] `clampEffort`：能力 low、high，输入 medium → high；`effortOverflow`：跟随 medium～xhigh、能力到 high → 超出、实际 high
  - [ ] `tierModelsOfRoutes`：两个虚拟模型共用 gpt-5.6-sol → 只出现一次；不含打分和顾问模型
