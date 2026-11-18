---
name: 07-capabilities-json-dialog
description: 模型管理页「能力配置 JSON」按钮与两步窗口：查看复制下载、预览并整体替换保存
---
# 「能力配置 JSON」窗口

## 目标
管理员在模型管理页一次查看、复制、下载全部模型能力，改完预览后整体替换保存。

## 业务规则（来源：PRD 02-capabilities-json-window，原型 2026-11-18 第 5 版）
- 入口：标题右侧按钮顺序 同步资料、同步价格、能力配置 JSON（描边小按钮）、＋ 添加模型、⋯ 菜单。
- 头部：标题「能力配置 JSON」；说明「这里是所有模型现在的能力配置，按模型名组织，写法和 /v1/models 返回的一致。可以复制、下载，也可以直接改或者粘贴新内容后保存。」；步骤条「1. 查看和修改」「2. 预览并保存」，当前步骤高亮。
- 第 1 步：编辑器显示 GET 接口返回的 `capabilities`（格式化）。有没列进来的记录时，上方提示框「有 N 条记录没有列在下面」，分两类：非完全匹配（名字和规则）「它们的名字不是具体的模型名，请在编辑抽屉里单独修改，这里保存不会动它们。」；未配置「要给它配置，可以在下面加上它的名字。」
- 编辑器下方说明：「保存时整体替换：保存后，每个模型的能力就是这里写的样子。没写的字段、写成 null 的字段都会被清空。从 JSON 里删掉一个模型，它的能力配置会被清空，记录本身保留。模型管理里还没有记录的模型，会自动新建一条完全匹配的记录。」
- 底栏：左边「N 个模型有能力配置」或「有未保存的改动」；右边「关闭」「下载 JSON 文件」「预览改动」。复制用编辑器工具栏按钮，成功提示「已复制到剪贴板」。下载当前编辑器内容，文件名 `model-capabilities-<YYYY-MM-DD>.json`。
- 第 1 步状态：没改动时「预览改动」变灰；空数据时编辑器为空带示例占位、底栏「还没有模型配置过能力，可以直接粘贴 JSON」、下载与预览变灰；JSON 格式错误时「JSON 无效」+「JSON 格式错误，改好后才能预览。」、预览变灰；加载中「加载中...」；加载出错「糟糕！出错了」「读取模型能力失败」+「重试」。
- 第 2 步（调 preview 接口）：徽标 新建 N、更新 N、清空 N（红）、不变 N，有错加红色「校验没通过 N」；有清空时提示框「有 N 个模型的能力配置会被清空」+ 每个「<模型名> 不在 JSON 里。如果不是有意删掉的，请返回修改，把它加回去。」；每个模型一块（名字 + 徽标 + 表「字段 / 当前值 / 更改为」只列变化字段；新建当前值「—」+「模型管理里还没有这个模型，会新建一条完全匹配的记录。」；清空的字段显示「清空（未配置）」；清空块下「JSON 里没有这个模型，它的能力配置会被清空，记录本身保留。」；不变只写「和现在的配置一样，不会改动。」）；底栏汇总「将新建 N 条、更新 N 条、清空 N 条记录」+「返回修改」「替换保存 N 条改动」。
- 校验失败：红色警示框「有 N 个条目校验没通过，现在不能保存」「请返回修改 JSON，全部通过后再保存。」+ 表「校验没通过的条目（N）」；保存变灰。
- 保存中：按钮变灰、「正在保存...」。保存失败（非 409）：「保存失败，没有任何改动生效」+ 原因，可再点保存。409：「模型资料已变化，没有保存」「预览之后，有模型的资料被改过，下面的对比已经不是最新的。请重新预览，确认最新的改动后再保存。」，保存按钮换成主色「重新预览」（重新调 preview），「返回修改」可用。
- 成功：窗口关闭，页面顶部「模型能力已保存」+「新建 N 条、更新 N 条、清空 N 条记录」，刷新模型列表与定价查询。
- 「返回修改」回第 1 步保留 JSON；关闭丢弃未保存改动。所有文字走 i18n，7 个 locale 补齐。

## 涉及文件
- 修改 `web/src/features/models/api.ts`（三个接口函数）
- 修改 `web/src/features/models/types.ts`（接口类型）
- 修改 `web/src/features/models/components/models-provider.tsx`（`DialogType` 加 `'capabilities-json'`）
- 修改 `web/src/features/models/components/models-primary-buttons.tsx`（按钮）
- 修改 `web/src/features/models/components/models-dialogs.tsx`（注册窗口）
- 新建 `web/src/features/models/components/dialogs/capabilities-json-dialog.tsx`
- 修改 `web/src/i18n/locales/*.json`（7 个）
- 新建 `web/src/features/models/components/dialogs/__tests__/capabilities-json-dialog.test.tsx`

## 函数清单
### web/src/features/models/api.ts
| 函数名 | 职责 |
|---|---|
| `getModelCapabilities` | GET `/api/models/capabilities` |
| `previewModelCapabilities` | POST `/api/models/capabilities/preview` |
| `replaceModelCapabilities` | PUT `/api/models/capabilities`，带 preview 返回的 version |

### web/src/features/models/components/dialogs/capabilities-json-dialog.tsx
| 函数名 | 职责 |
|---|---|
| `CapabilitiesJsonDialog` | 两步窗口：读取、编辑、复制、下载、预览、保存、各状态 |
| `CapabilityPreviewItem` | 单个模型的徽标与字段对比表 |

## 协作关系
- 接口契约见 05：GET `{capabilities, non_exact, unconfigured}`；preview `{version, items[{model_name, action, before, after}], errors[{model_name, message}], summary}`；PUT 成功 `{create, update, clear}`，校验错误 400，版本冲突 409。
- 复用：`Dialog`（`@/components/dialog`）、`JsonCodeEditor`、`StaticDataTable`、`Badge`、`Alert`、`EmptyState/ErrorState/LoadingState`；步骤条照 `sync-wizard-dialog.tsx:70-75,383-398` 的写法（两处使用，若抽公共组件需说明）；下载按项目现有 Blob + `a.download` 写法（`view-logs-dialog.tsx:164-172`）；复制用 `JsonCodeEditor` 自带按钮。
- JSON 本地格式检查用 06 的 `parseModelCapabilityJson` 逐个模型检查格式；最终合法性以 preview 接口的 errors 为准。
- 成功后 invalidate `modelsQueryKeys.all` 与 `['pricing']`，toast 用 `sonner`。

## 验证方式
- 测试入口：渲染 `CapabilitiesJsonDialog`（mock 三个接口），点击、输入、断言文案与按钮状态。
- 测试输入：GET 返回两模型 + `non_exact` 一条 + `unconfigured` 一条；preview 返回 create/update/clear/unchanged 各一；preview 返回 errors；PUT 返回 409 / 500 / 成功。
- 预期结果：
- [ ] 按钮顺序正确，点击打开窗口
- [ ] 提示框两类内容与文案正确；编辑器内容为格式化 JSON
- [ ] 未改动时「预览改动」禁用；改动后可用；格式错误时禁用并提示
- [ ] 下载文件名为当天日期，内容为编辑器当前文本
- [ ] 预览徽标、清空提示框、每块表格、底栏汇总正确
- [ ] errors 非空时警示框与错误表出现，保存禁用
- [ ] 409 时出现「模型资料已变化，没有保存」与「重新预览」，点后重新调 preview
- [ ] 500 时出现「保存失败，没有任何改动生效」，可重试
- [ ] 成功后窗口关闭、toast 文案与数字正确
- [ ] 7 个 locale 有全部新 key；`bun run typecheck`、`bun run lint`、`bun run test` 通过
