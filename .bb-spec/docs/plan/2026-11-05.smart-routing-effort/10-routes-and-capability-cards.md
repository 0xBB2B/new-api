---
name: 10-routes-and-capability-cards
description: 智能路由页虚拟模型表格改第二行文字和超出能力标黄，新增「模型思考能力」卡片与覆盖弹窗，和虚拟模型一起保存
---
# 智能路由页：表格与模型思考能力卡片

## 目标
智能路由页的虚拟模型表格显示接口和力度摘要、超出能力时标黄；新增「模型思考能力」卡片，管理员能覆盖、修改、恢复内置，改动随「保存更改」一起保存。

## 业务规则（来源：PRD）
- 04-editor-layout C13：名称列第二行「<接口简称用 / 连接> 接口」；三档列第二行「<力度摘要> · <分组用 → 连接>」；格子窄时省略、悬停显示全文。
- 02-thinking-capability C9：档位存的力度超出有效能力 → 那一格力度文字标黄（warning 色），下面多一行黄色小字「超出模型能力，实际按 <X> 发送」；存的配置不自动改。
- C4：「虚拟模型」卡片下面新增「模型思考能力」卡片：标题后显示模型数量；说明「智能路由按这里的信息决定能发哪些思考力度。内置值来自各厂商官方文档，可以按模型覆盖。」；下面表格。
- C5：只列所有虚拟模型三档用到的模型，去重；不列打分、顾问模型。列：模型、来源、支持的档位、操作。来源 Badge：内置 secondary、已覆盖 default（主色）、未确认 warning；档位每档一个 Badge secondary，未确认的行用 outline；不支持思考写「不支持思考」。操作：没覆盖的行「覆盖」；已覆盖的行「修改」「恢复内置」。
- C6：没有虚拟模型时用页面现有的空状态组件，写「还没有虚拟模型用到的模型」。
- C7：覆盖弹窗标题「覆盖 <模型> 的思考能力」，说明「勾选这个模型支持的思考力度，保存更改后生效。」；一个 Switch「不支持思考」+ 七个 Checkbox（none～max），打开时填当前有效值；开关打开时勾选框禁用；「取消」「保存」；没开关又一档没勾就保存 → 红字「至少选择一个档位」，弹窗不关。
- C8：覆盖 / 修改 / 恢复内置只改页面草稿，点右上角「保存更改」时，虚拟模型和覆盖表里改过的选项在同一次请求里提交；没有任何改动时提示「没有需要保存的更改」（一期行为）。
- C11：新增文字七种语言都有翻译。

## 涉及文件
- 修改 `web/src/features/system-settings/request-policies/smart-routing-section.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/routes-card.tsx`
- 新建 `web/src/features/system-settings/request-policies/smart-routing/capabilities-card.tsx`
- 新建 `web/src/features/system-settings/request-policies/smart-routing/capability-override-dialog.tsx`
- 修改 7 个 `web/src/i18n/locales/*.json`
- 测试：扩展 `web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx`

## 函数清单
### smart-routing-section.tsx
| 函数名 | 职责 |
|---|---|
| `SmartRoutingSection` | 同时持有虚拟模型和覆盖表两份草稿与基线；查内置能力（按 07 的 `tierModelsOfRoutes`）；保存时一起提交两个选项键；把草稿覆盖传给表格、弹窗、能力卡片 |
### routes-card.tsx
| 函数名 | 职责 |
|---|---|
| `SmartRoutesTable` | 名称、档位第二行改用 07 的摘要函数；超出能力时标黄并加一行提示 |
### capabilities-card.tsx
| 函数名 | 职责 |
|---|---|
| `ThinkingCapabilitiesCard` | 卡片头、空状态、表格、操作按钮 |
### capability-override-dialog.tsx
| 函数名 | 职责 |
|---|---|
| `CapabilityOverrideDialog` | 开关 + 七个勾选框、校验、保存回草稿 |

## 协作关系
用 07 的 `effectiveCapability`、`effortOverflow`、`tierModelsOfRoutes`、`getBuiltinThinkingCapabilities`、选项读写函数；把能力数据和草稿覆盖传给 09 的弹窗。组件优先用项目现有的 Card、Table、Badge、Switch、Checkbox、Dialog、EmptyState，写法和「虚拟模型」卡片保持一致。

## 验证方式
- 测试入口：渲染智能路由页（mock `GET /api/option/request_policy` 返回两个选项、mock 内置能力接口、mock 保存接口）
- 测试输入：auto-gpt（三档 gpt-5.4-mini / gpt-5.5 / gpt-5.6-sol，困难档跟随 medium～xhigh）、auto-chat（gpt-4.1 / glm-5.1 / gpt-5.6-sol）；覆盖表含 gpt-5.5；内置能力：gpt-5.6-sol low～xhigh，gpt-4.1 不支持思考，glm-5.1 未知
- 预期结果：
  - [ ] 表格：auto-gpt 名称下「Responses / Anthropic 接口」；困难档第二行「跟随打分 medium～xhigh · subscription → api」；gpt-4.1 那格「不支持思考 · api」
  - [ ] 能力卡片标题数量 5；gpt-5.6-sol 只出现一次；gpt-5.5「已覆盖」并有「修改」「恢复内置」；glm-5.1「未确认」、档位为描边样式；gpt-5.6-luna 不出现
  - [ ] 覆盖 gpt-5.6-sol 去掉 xhigh 并保存弹窗 → auto-gpt 困难档那格标黄并显示「超出模型能力，实际按 high 发送」；点「保存更改」→ 提交的请求里有覆盖表键且含 gpt-5.6-sol；虚拟模型没改，就不带虚拟模型键（只提交改过的选项，避免没动过的路由因渠道或价格变化被校验挡住）
  - [ ] 覆盖弹窗全不勾就保存 → 「至少选择一个档位」，弹窗不关；打开「不支持思考」→ 勾选框禁用
  - [ ] 点「恢复内置」→ 该行回到「内置」，标黄消失
  - [ ] 删光虚拟模型 → 能力卡片显示「还没有虚拟模型用到的模型」
  - [ ] 不改任何东西点「保存更改」→ 提示「没有需要保存的更改」
