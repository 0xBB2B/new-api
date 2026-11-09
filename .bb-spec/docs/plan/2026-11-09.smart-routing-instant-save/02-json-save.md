---
name: 02-json-save
description: JSON 视图在编辑框下方加「保存」按钮，没改动时变灰；有未保存改动时切回可视化先确认是否放弃
---
# JSON 视图的保存按钮

## 目标
JSON 视图里的改动只在点编辑框下方的「保存」时提交；切回可视化时，没保存的改动要经管理员确认才丢掉。

## 业务规则（来源：PRD 02-json-save）
- JSON 视图里，`JsonCodeEditor` 下方靠右有一个「Save」按钮（`Button size='sm'`，主色，28px 高）。可视化视图里没有这个按钮。
- "和已保存的一样"按内容判断：把 JSON 文字用现有的 `parseSmartRoutes` 解析、再用 `serializeSmartRoutes` 序列化，和后台已保存配置的序列化结果相同，就算没有改动。只改空格、换行或点「Format JSON」都算没有改动。JSON 写错（解析失败）算有改动。
- 没有改动时「Save」disabled；有改动时可以点。
- 点「Save」时检查规则和现在 JSON 视图保存时一样：本地解析失败（不是数组、JSON 语法错等）用 `smartRoutesErrorMessage` 弹出原因，不调接口；本地通过后提交，后台检查（名称、模型渠道等）不通过时弹出后台返回的原因。两种情况框里的内容都不变。
- 检查通过后只提交 `smart_routing_setting.routes`。提交中「Save」disabled 并显示「Saving...」。成功后提示「Saved successfully」，停在 JSON 视图，「Save」变回 disabled（框里的内容就是已保存的配置）。
- 提交失败（网络错误或后台拒绝）时只弹出一次失败原因，框里的内容不变，「Save」恢复可点。
- 有改动时点「Visual」标签：用现有的 `ConfirmDialog` 弹出确认框，标题用现有键 `Discard unsaved changes?`（中文"放弃未保存的修改？"），说明用新键 `The changes in JSON have not been saved. Switching to visual will discard them.`（中文"JSON 里的改动还没保存，切回可视化会丢掉这些改动。"），确认按钮用现有键 `Discard changes`（中文"放弃修改"），`destructive`。点「Cancel」留在 JSON、内容不变；点「Discard changes」丢掉 JSON 草稿、切回可视化，表格显示已保存的配置。
- 没有改动时点「Visual」直接切换，不弹框。切回可视化时不再做"incomplete"那类检查，因为显示的是已保存的配置。
- JSON 视图里「Add virtual model」按钮保持 disabled（现状）。
- 决策理由：JSON 改到一半的内容常常不完整，不能边打字边保存；切回可视化前问是否放弃，是为了不让下一次立即保存把没确认的 JSON 改动带上去；按钮放框下方是用户在原型里选的方案。

## 涉及文件
- 修改 `web/src/features/system-settings/request-policies/smart-routing/routes-card.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing-section.tsx`（只在需要调整传给卡片的 props 时改）
- 修改 `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`（7 个文件）
- 修改 `web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx`

## 函数清单
### web/src/features/system-settings/request-policies/smart-routing/routes-card.tsx
| 函数名 | 职责 |
|---|---|
| `SmartRoutesCard` | 持有 JSON 草稿（只在 JSON 视图存在）和"放弃确认框是否打开"的状态；JSON 视图在编辑框下方渲染「Save」；渲染放弃确认框 |
| `changeMode` | 切到 JSON：用已保存的配置格式化（缩进 2）生成草稿。切回可视化：草稿和已保存的一样就直接切换并清空草稿，不一样就打开放弃确认框 |
| `saveJson`（组件内） | 本地解析草稿，失败就弹出 `smartRoutesErrorMessage` 后返回；成功则调用 01 里传入的"提交虚拟模型列表"动作，等待结果；成功后把草稿重置为刚保存配置的格式化文字 |

`routes-card.tsx` 已经引入 `ConfirmDialog`、`Button`、`parseSmartRoutes`、`serializeSmartRoutes`、`smartRoutesErrorMessage`，直接复用。不新建组件，不改 `JsonCodeEditor`。

### 翻译文件
- 新增键 `The changes in JSON have not been saved. Switching to visual will discard them.`，7 种语言都填。
- 复用已有键 `Discard unsaved changes?`、`Discard changes`、`Save`、`Saving...`、`Cancel`。
- 01 去掉切回可视化时的 incomplete 检查后，如果 `hasMissingEndpoints` 或相关提示键在全仓库没有别的引用，就一起删掉（删前 grep 确认）。按 `i18n-translate` skill 操作，从 `web/` 跑 `bun run i18n:sync`。

## 协作关系
- `SmartRoutesCard.saveJson` → props 里的"提交虚拟模型列表"动作（plan 01 在 `SmartRoutingSection` 里实现的 `saveRoutes`）→ `useSavePolicy().mutateAsync` → `PATCH /api/option/request_policy`。
- 已保存的配置来自 `SmartRoutingSection` 读的 `['request-policy']` 查询；保存成功后 `useSavePolicy` 会更新这份缓存，卡片据此判断有没有改动。
- 依赖 plan 01：页面右上角按钮去掉之后，JSON 视图只能靠这里的「Save」保存。

## 验证方式
- 测试入口：`web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx`，沿用现有渲染和 `api` mock；从 `web/` 跑 `bunx vitest run src/features/system-settings/request-policies`。
- 测试输入：后台已有 auto-gpt（困难档 gpt-5.6-sol）；在 JSON 编辑框里输入文字的方式沿用现有 JSON 视图用例（如 `a non-array JSON draft reports an error and stays in the JSON view`）。
- 预期结果：看下面的验证项。断言 `api.patch` 的调用次数和参数、按钮的 disabled 状态和文字、确认框的标题。
- 已有 JSON 用例里点「Save Changes」的改成点 JSON 框下方的「Save」。用例 `a route pasted in the JSON view without endpoints keeps the JSON view and shows an incomplete notice` 断言的是切换时的 incomplete 检查，按 PRD 改成：有改动时点「Visual」弹出放弃确认框。
- [ ] 切到 JSON →「Save」存在且 disabled；切回可视化 → 页面上没有这个按钮。
- [ ] 把草稿改成同一份配置换一种缩进 →「Save」仍是 disabled。
- [ ] 把困难档改成 gpt-5.5 →「Save」可点；点击后 `api.patch` 只调用一次，`options` 只有 `smart_routing_setting.routes`，内容里困难档是 gpt-5.5；成功后停在 JSON 视图，「Save」变回 disabled。
- [ ] 草稿改成非数组 JSON 后点「Save」→ 出现现有的格式错误提示，`api.patch` 没被调用，草稿不变。
- [ ] `api.patch` 返回 `success:false` → 出现后台返回的消息（只一次），草稿不变，「Save」可点。
- [ ] 提交 pending 时「Save」disabled，文字是「Saving...」。
- [ ] 有改动时点「Visual」→ 出现「Discard unsaved changes?」确认框；点「Cancel」→ 还在 JSON，草稿不变；再点「Visual」→「Discard changes」→ 回到可视化，表格显示已保存的 gpt-5.6-sol，`api.patch` 没被调用。
- [ ] 没改动时点「Visual」→ 不出现确认框，直接回到可视化。
- [ ] JSON 视图里「Add virtual model」是 disabled。
- [ ] 7 个 locale 文件都有新的说明键。
