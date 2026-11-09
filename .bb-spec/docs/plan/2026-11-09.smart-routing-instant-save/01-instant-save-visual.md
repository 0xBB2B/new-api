---
name: 01-instant-save-visual
description: 去掉智能路由页右上角「保存更改」，添加、编辑、删除虚拟模型和覆盖、恢复思考能力确认后立即提交
---
# 可视化操作立即保存

## 目标
在智能路由页的可视化视图里，每个改配置的操作确认后立即提交到后台。页面右上角没有保存按钮。

## 业务规则（来源：PRD 01-visual-instant-save）
- 智能路由页（`/system-settings/request-policies/smart-routing`）不显示页面右上角的「Save Changes」按钮；「请求策略」其他分区（如重试）的保存按钮不变。
- 添加、编辑弹窗里点「Save」：先做现在的本地检查（重名等）和后台逐项检查（`POST /api/option/smart_routing/validate`），有错时和现在一样，弹窗不关、标红字、不提交配置。全部通过后立即提交，成功后弹窗关闭，表格显示新值，提示「Saved successfully」。
- 删除确认框说明改成 `Delete {{name}}? It takes effect immediately after you confirm.`（中文"确定删除 {{name}} 吗？确认后立即生效。"）。点「Delete」立即提交，成功后确认框关闭、这一行消失。删掉最后一个时显示现有空状态。
- 思考能力覆盖弹窗说明改成 `Tick the thinking efforts this model supports. Takes effect immediately after you save.`（中文"勾选这个模型支持的思考力度，保存后立即生效。"）。点「Save」时先做现在的「至少选择一个档位」检查，通过后立即提交，成功后弹窗关闭、来源变「Overridden」。
- 「Restore built-in」不弹确认框，点了立即提交，成功后来源变回「Built-in」。
- 一次操作只提交它改的那一项：改虚拟模型时请求体 `options` 里只有 `smart_routing_setting.routes`；改思考能力时只有 `smart_routing_setting.thinking_capabilities`。
- 提交过程中，弹窗或确认框里的「Cancel」和保存类按钮都不能点，保存类按钮显示「Saving...」。
- 提交失败（网络错误或后台返回 success=false）时：弹窗或确认框不关，填的内容保留，只弹出一次失败原因；表格保持提交前的值；可以直接再点保存重试。「Restore built-in」失败时只弹出失败原因，来源保持「Overridden」。
- 提交成功后，页面显示的就是后台刚保存的值（刷新后也一样）。
- 决策理由：两步保存容易让人误以为已生效、忘点就丢改动；后台本来就能只保存其中一项，不用新开接口；失败时保留弹窗是为了直接重试。

## 涉及文件
- 修改 `web/src/features/system-settings/request-policies/smart-routing-section.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/routes-card.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/route-editor-dialog.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/capabilities-card.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/capability-override-dialog.tsx`
- 修改 `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`（7 个文件）
- 修改 `web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx`

## 函数清单
### web/src/features/system-settings/request-policies/smart-routing-section.tsx
| 函数名 | 职责 |
|---|---|
| `SmartRoutingSection` | 去掉 `SettingsPageFormActions` 和整页 `save`；去掉思考能力草稿状态，页面直接显示后台已保存的值；把"提交虚拟模型列表""提交思考能力覆盖"两个立即保存动作传给两张卡片 |
| `saveRoutes`（组件内） | 序列化虚拟模型列表，只用 `smart_routing_setting.routes` 调 `useSavePolicy` 的 `mutateAsync`；失败时把错误抛给调用方，不再重复弹提示 |
| `saveCapabilities`（组件内） | 序列化思考能力覆盖，只用 `smart_routing_setting.thinking_capabilities` 调 `mutateAsync`；失败处理同上 |

### web/src/features/system-settings/request-policies/smart-routing/routes-card.tsx
| 函数名 | 职责 |
|---|---|
| `SmartRoutesCard` | 编辑弹窗 `onSave` 改成调用立即保存，并把成功或失败结果交回弹窗；删除确认框点「Delete」时调用立即保存，提交中给 `ConfirmDialog` 传 `isLoading`，成功后关闭，失败不关；换成新的删除说明文案 |

### web/src/features/system-settings/request-policies/smart-routing/route-editor-dialog.tsx
| 函数名 | 职责 |
|---|---|
| `RouteEditorDialog` | 提交中让「Cancel」「Save」都不能点，「Save」显示「Saving...」 |
| `submit` | 后台检查通过后等待 `props.onSave` 的提交结果：成功才关闭弹窗，失败保持打开并保留表单内容 |

### web/src/features/system-settings/request-policies/smart-routing/capabilities-card.tsx
| 函数名 | 职责 |
|---|---|
| `ThinkingCapabilitiesCard` | 覆盖弹窗 `onSave` 等待立即保存结果，成功才关闭；「Restore built-in」调用立即保存 |
| `restore` | 去掉该模型的覆盖后立即提交；失败时页面保持原值 |

### web/src/features/system-settings/request-policies/smart-routing/capability-override-dialog.tsx
| 函数名 | 职责 |
|---|---|
| `CapabilityOverrideDialog` | 换成新的说明文案；提交中「Cancel」「Save」都不能点，「Save」显示「Saving...」；`props.onSave` 失败时保持打开 |
| `save` | 通过本地检查后等待 `props.onSave` 的结果 |

### 翻译文件
- 新增两个键（7 种语言都要填）：`Delete {{name}}? It takes effect immediately after you confirm.`、`Tick the thinking efforts this model supports. Takes effect immediately after you save.`。
- 删除改完后代码里没人用的旧键：`Delete {{name}}? It takes effect after you save changes.`、`Tick the thinking efforts this model supports. Takes effect after you save changes.`。删除前用 grep 确认没有其他引用。`Save Changes`、`No changes to save` 还有其他分区在用，不删。
- 按 `i18n-translate` skill 操作，改完从 `web/` 跑 `bun run i18n:sync` 确认没有缺失。

## 协作关系
- `SmartRoutingSection` → `SmartRoutesCard`、`ThinkingCapabilitiesCard`（通过 props 传入立即保存动作）。
- `SmartRoutesCard` → `RouteEditorDialog.onSave`、`ConfirmDialog.handleConfirm` → `saveRoutes` → `useSavePolicy().mutateAsync` → `PATCH /api/option/request_policy`。
- `ThinkingCapabilitiesCard` → `CapabilityOverrideDialog.onSave` / `restore` → `saveCapabilities` → 同一个接口。
- `useSavePolicy`（`use-save-policy.ts`）成功时更新 `['request-policy']` 缓存并弹「Saved successfully」，失败时用 `handleServerError` 弹一次错误。调用方 catch 后不再调用 `handleServerError`，避免弹两次。
- 复用组件：`ConfirmDialog`（自带 `isLoading`）、现有 `Dialog`、`Button`。不新建组件。

## 验证方式
- 测试入口：`web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx`，沿用文件里现有的渲染方式和 `api.get`/`api.post`/`api.patch` mock；从 `web/` 跑 `bunx vitest run src/features/system-settings/request-policies`。
- 测试输入：后台已有一个虚拟模型 auto-gpt，有时还有一个 gpt-5.6-sol 的思考能力覆盖；`api.patch` 分别 mock 成功（返回新配置）、返回 `success:false`、抛出网络错误三种情况。
- 预期结果：看下面的验证项。断言用 `api.patch` 的调用参数和页面上看得到的文字或按钮状态。
- 已有用例里点「Save Changes」、断言「No changes to save」、断言旧删除文案的，按新行为改写，不要保留两步保存的断言。"draft rows" 一类用例改成每一步都立即保存，保留它们原本保护的重名、改回名称等规则。
- [ ] 页面上找不到名为「Save Changes」的按钮。
- [ ] 编辑 auto-gpt，后台检查通过后点「Save」→ `api.patch` 只被调用一次，`options` 里只有 `smart_routing_setting.routes`，内容是新配置；弹窗关闭，表格显示新值。
- [ ] 后台检查返回字段错误 → `api.patch` 没被调用，弹窗不关。
- [ ] `api.patch` 返回 `success:false` 或抛错 → 弹窗不关，表单里填的值还在，错误提示只出现一次，表格还是旧值；改成成功后再点「Save」→ 弹窗关闭。
- [ ] 提交 pending 时，弹窗里「Cancel」「Save」都是 disabled，「Save」文字是「Saving...」。
- [ ] 删除：确认框显示新说明；点「Delete」→ `api.patch` 只带 `smart_routing_setting.routes`，内容里没有这一行；失败时确认框不关；删掉唯一一行后显示空状态。
- [ ] 覆盖 gpt-5.6-sol 并保存 → `api.patch` 的 `options` 里只有 `smart_routing_setting.thinking_capabilities`；弹窗显示新说明；一档都不勾时显示「Select at least one level」，不调用接口。
- [ ] 「Restore built-in」→ 不出现确认框，`api.patch` 只带 `smart_routing_setting.thinking_capabilities`，内容里没有这个模型；失败时来源仍是「Overridden」。
- [ ] 7 个 locale 文件都有两个新键，旧的两个键已删除。
