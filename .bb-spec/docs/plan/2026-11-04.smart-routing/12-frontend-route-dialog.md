---
name: 12-frontend-route-dialog
description: 添加/编辑虚拟模型弹窗：三档、打分模型、顾问、高级设置；保存时调后台检查，字段下显示红字
---
# 添加 / 编辑虚拟模型弹窗

## 目标
管理员在弹窗里填好一个虚拟模型；点「保存」时由后台逐项检查，出错的字段下面显示原因，全部通过才回到表格。

## 业务规则（来源：PRD 05-config-page C5~C7，以及用户确认过的预览页）
- 弹窗写法和「编辑规则」弹窗（`channel-affinity/rule-editor-dialog.tsx`）相同：`@/components/dialog` 的 `Dialog`，`contentClassName='sm:max-w-5xl'`，底部「取消」「保存」，「高级设置」用 `Collapsible`。标题为「添加虚拟模型」或「编辑虚拟模型」。
- 内容从上到下：
  1. 两列：名称（必填，占位"例如 auto-gpt"）和接口。接口只读，显示由定价数据推导的接口；没选模型时占位"选好模型后自动判断"。说明文字"按所选模型自动判断，同一个虚拟模型里的模型必须走同一类接口"。
  2. 分隔线，「档位」小标题，说明"用户每发一条新消息，打分后落到其中一档。打分失败、超时或把握不足时用默认档。"
  3. 简单档、普通档、困难档各一块：左边是「X档模型」下拉，右边是「思考力度」下拉（不覆盖 / low / medium / high，默认不覆盖；第一块下面加说明"选"不覆盖"时保留客户端传来的值"）；下面是分组顺序。
  4. 「默认档」下拉（简单档 / 普通档 / 困难档，默认普通档）。
  5. 分隔线，「打分模型」小标题，说明"用户每发一条新消息调用一次。费用按打分模型的价格，单独记一条消费记录。"；两列：模型下拉、超时（毫秒，默认 2000，说明"超时就用默认档"）；下面是分组顺序。
  6. 分隔线，「顾问」小标题，说明"同一段对话里，用户连续多条消息表示不满意时，这一轮交给顾问模型回答。"；两列：模型下拉、连续不满意几次后交给顾问（默认 2）；下面是分组顺序。
  7. 分隔线，「高级设置」折叠区，三列：
     - 降档时历史 token 上限（默认 20000，说明"对话历史超过这个长度就不降档，避免按原价重读整段历史"）；
     - 可信度门槛（默认 0.6，说明"打分模型的把握低于它时用默认档"）；
     - 简单题消息长度上限（字，默认 2000，说明"最新消息超过这个长度就不算简单题"）。
- 模型下拉只列出至少在一个分组里有可用渠道的模型：数据来自 `features/channels/api.ts` 的 `getEnabledModels()`（`/api/channel/models_enabled`），下拉组件用 `components/ui/combobox-input.tsx` 的 `ComboboxInput`（不允许自定义输入）。
- 分组顺序复用 `features/keys/components/auto-group-order-editor.tsx` 的 `AutoGroupOrderEditor`，固定 `mode='custom'`，分组选项来自 `features/users/api.ts` 的 `getGroups()`。这里要做一处兼容扩展：新增两个可选 prop，`hideInheritControls`（隐藏"已选择 X / Y 个分组"和「恢复全局 Auto」那一行）和 `addLabel`（把「添加 Auto 分组」换成「添加分组」）。不传时行为和现在完全一样。原因：现有组件只为 API 密钥的 auto 分组设计，没有关掉"继承全局"控件的开关，复制一份会造成重复实现。
- 点「保存」：
  1. 先做本地检查（zod：必填、数字范围），不通过直接在字段下显示；
  2. 再调用 `POST /api/option/smart_routing/validate`，请求体 `{ route, original_name }`（添加时 original_name 为空）；
  3. 返回的 `errors` 逐条 `form.setError(field, { message: t(message.key, message.params) })`，弹窗不关；错误落在高级设置的字段时，自动展开折叠区（参考 `features/channels/lib/channel-form-errors.ts`）；
  4. `errors` 为空才把虚拟模型交回表格并关闭弹窗。
- 后端 `field` 到表单字段的对应：两边路径写法相同（如 `tiers.hard.model`），直接用作 RHF 的字段名。
- 新增文字要有七种翻译，做法同 11。

## 涉及文件
- web/src/features/system-settings/request-policies/smart-routing/route-editor-dialog.tsx（新建）
- web/src/features/system-settings/request-policies/smart-routing/route-form.ts（新建：zod schema、默认值、表单值与 SmartRoute 互转）
- web/src/features/system-settings/request-policies/smart-routing/api.ts（新建：`validateSmartRoute`）
- web/src/features/keys/components/auto-group-order-editor.tsx（修改：新增两个可选 prop）
- web/src/i18n/locales/*.json（修改）

## 成品定义
新增翻译键（英文原文 → 简体中文）。后端 02 成品里列出的全部错误消息键（含保存失败时的 `Virtual model {{name}}: {{detail}}`）也在这一步一起补齐七种语言：

| key | zh |
|---|---|
| Edit virtual model | 编辑虚拟模型 |
| Name | 名称 |
| e.g. auto-gpt | 例如 auto-gpt |
| Endpoint | 接口 |
| Detected after you pick models | 选好模型后自动判断 |
| Detected from the selected models. All models of a virtual model must use the same endpoint | 按所选模型自动判断，同一个虚拟模型里的模型必须走同一类接口 |
| Tiers | 档位 |
| Each new user message is scored and goes to one tier. The default tier is used when scoring fails, times out or is unsure. | 用户每发一条新消息，打分后落到其中一档。打分失败、超时或把握不足时用默认档。 |
| {{tier}} model | {{tier}}模型 |
| Reasoning effort | 思考力度 |
| Keep client value keeps the effort sent by the client | 选"不覆盖"时保留客户端传来的值 |
| Default tier | 默认档 |
| Group order | 分组顺序 |
| Groups are tried in order. The next group is used when every channel in the current one fails. | 按顺序尝试这些分组，前一个分组的渠道都失败了再试下一个。 |
| Add group | 添加分组 |
| Called once for each new user message. It is billed at the scorer model price as a separate usage record. | 用户每发一条新消息调用一次。费用按打分模型的价格，单独记一条消费记录。 |
| Timeout (ms) | 超时（毫秒） |
| The default tier is used on timeout | 超时就用默认档 |
| When a user is dissatisfied several messages in a row in one conversation, the advisor answers this turn. | 同一段对话里，用户连续多条消息表示不满意时，这一轮交给顾问模型回答。 |
| Dissatisfied messages before the advisor answers | 连续不满意几次后交给顾问 |
| Downgrade history token limit | 降档时历史 token 上限 |
| No downgrade once the history exceeds this length, to avoid re-reading the whole history at full price | 对话历史超过这个长度就不降档，避免按原价重读整段历史 |
| Confidence threshold | 可信度门槛 |
| The default tier is used when the scorer is less sure than this | 打分模型的把握低于它时用默认档 |
| Simple message length limit (characters) | 简单题消息长度上限（字） |
| A latest message longer than this is not treated as simple | 最新消息超过这个长度就不算简单题 |

## 函数清单
### smart-routing/route-editor-dialog.tsx
| 函数名 | 职责 |
|---|---|
| `RouteEditorDialog` | 弹窗本体：表单布局、提交流程（本地检查 → 后端检查 → 回填错误或交回结果） |
| `TierFields` | 一个档位块：模型下拉、思考力度下拉、分组顺序 |
| `GroupOrderField` | 用扩展后的 `AutoGroupOrderEditor` 编辑分组顺序 |

### smart-routing/route-form.ts
| 函数名 | 职责 |
|---|---|
| `createRouteFormSchema` | zod schema，数字范围和后端一致 |
| `emptyRouteFormValues` / `routeToFormValues` / `formValuesToRoute` | 默认值与互转 |

### smart-routing/api.ts
| 函数名 | 职责 |
|---|---|
| `validateSmartRoute` | 调用后端检查接口，返回 errors 和 endpoints |

### features/keys/components/auto-group-order-editor.tsx
| 函数名 | 职责 |
|---|---|
| `AutoGroupOrderEditor` | 修改：支持 `hideInheritControls`、`addLabel` 两个可选 prop |

## 协作关系
- 由 11 的 `SmartRoutesCard` 打开；保存通过的结果交回 11 的本地列表。
- 后端：`POST /api/option/smart_routing/validate`（02）。

## 验证方式
- 测试入口：在 11 的测试文件里渲染 `RouteEditorDialog`；`vi.spyOn(api, 'post')` 模拟检查接口，`vi.spyOn(api, 'get')` 返回模型和分组。另在 `features/keys/components/__tests__/auto-group-order-editor.test.tsx` 补两个新 prop 的用例。
- 预期结果：
- [ ] 添加弹窗的默认值：思考力度"不覆盖"、默认档"普通档"、超时 2000、连续不满意次数 2、降档上限 20000、门槛 0.6、长度上限 2000
- [ ] 名称为空、超时 50 → 本地检查报错，没有调用检查接口
- [ ] 检查接口返回 `tiers.hard.model` 和 `scorer.model` 两条错误 → 两个字段下出现翻译后的红字，弹窗不关
- [ ] 返回 `confidence_threshold` 错误 → 高级设置自动展开
- [ ] 返回 errors 为空 → 弹窗关闭，表格多出（或更新）这一行
- [ ] `AutoGroupOrderEditor` 不传新 prop 时，现有测试全部通过；传 `hideInheritControls` 时不显示恢复全局按钮
