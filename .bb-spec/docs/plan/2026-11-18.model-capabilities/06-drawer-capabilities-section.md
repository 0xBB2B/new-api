---
name: 06-drawer-capabilities-section
description: 模型编辑抽屉新增「模型能力」分区，表单与 JSON 两种模式互转，前端校验并随资料一起保存
---
# 模型编辑抽屉「模型能力」分区

## 目标
管理员在模型编辑抽屉里用表单或 JSON 配置五项能力，随「保存资料」一起提交。

## 业务规则（来源：PRD 01-capability-on-model-record，原型 2026-11-18 第 5 版）
- 位置：「模型资料」标签页里，「匹配规则」之后、「端点」之前；任何名称规则的记录都显示。
- 头部：标题「模型能力」；右侧描边小按钮，表单模式写「JSON 模式」，JSON 模式写「可视模式」；说明「这里填的内容会通过 /v1/models 返回给调用方。没填的项不返回。」
- 表单：输入模态复选框 文字/图片/文件/音频（text/image/file/audio），说明「一个都不勾表示未配置。」；输出模态 文字/图片/音频，说明同上；上下文大小、最大输出 token 两个输入框并排，占位「例如 1050000」「例如 128000」，说明「单位是 token。不填表示未配置。」；思考等级：开关「不支持思考」+ 七个复选框 none…max，说明「勾选这个模型支持的档位。开关没开、一档也没勾，表示未配置。」
- 全部未配置可正常保存。开「不支持思考」时七档变灰不可点；关上后恢复原勾选，其他字段不变。
- 数字只接受 1 到 9007199254740991 的整数；否则输入框变红、提示「请输入大于 0 的整数」、不能保存。
- JSON 模式：标签「能力配置」，带行号 JSON 编辑器（行列号、「JSON」/「JSON 无效」、复制、格式化）；说明「形状和 /v1/models 返回的一致。没写的字段表示未配置；不支持思考写 "supported_efforts": []。」未配置显示 `{}`。
- JSON 格式错误：红色「JSON 无效」，下方「JSON 格式错误，改好后才能保存，也才能切回可视模式。」，「可视模式」按钮变灰，不能保存。
- JSON 内容不合法（不认识的值、多余字段、数字越界）：下方逐条「<字段路径> 里有不认识的值 <值>，可用的值：……」（数字、多余字段写明原因），不能保存；服务器返回的同类错误显示在底栏上方警示框。
- 两种模式互转内容一致；保存时提交规范 JSON（档位按 none→max 排序、去重）。
- 保存中、成功（抽屉关闭 + 「模型资料已保存」）、失败（警示框，抽屉不关，内容不丢）沿用现有抽屉行为。
- 所有文字走 i18n，7 个 locale 文件都补齐。

## 涉及文件
- 修改 `web/src/features/models/types.ts`（Model 增加 `capabilities?: string`）
- 修改 `web/src/features/models/lib/model-form.ts`（`modelFormSchema`、`MODEL_FORM_DEFAULT_VALUES`、`transformModelToFormDefaults`、`transformFormDataToModelPayload` 加 capabilities）
- 新建 `web/src/features/models/lib/model-capabilities.ts`（能力的解析、校验、规范化、表单值与 JSON 互转；07 复用）
- 新建 `web/src/features/models/components/drawers/model-capabilities-section.tsx`
- 修改 `web/src/features/models/components/drawers/model-mutate-drawer.tsx`（在匹配规则与端点之间放入分区）
- 修改 `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`（本分区文案 + 01/05 后端错误文案 key）
- 新建 `web/src/features/models/lib/__tests__/model-capabilities.test.ts`；新建或扩展 `web/src/features/models/components/drawers/__tests__/` 下抽屉测试

## 函数清单
### web/src/features/models/lib/model-capabilities.ts
| 函数名 | 职责 |
|---|---|
| `MODEL_INPUT_MODALITIES` / `MODEL_OUTPUT_MODALITIES` / `MODEL_REASONING_EFFORTS` | 可选值与排序 |
| `parseModelCapabilityJson` | 解析单个模型能力 JSON 文本，返回规范对象或逐条错误（含字段路径） |
| `normalizeModelCapability` | 去重、排序、空数组与 null 变未配置 |
| `serializeModelCapability` | 输出规范 JSON 文本；全部未配置为 `{}`（提交时空能力传空串） |
| `capabilityToFormValues` / `formValuesToCapability` | 表单值与能力对象互转 |
| `isPositiveSafeInteger` | 1 到 2^53−1 的整数判断 |

### web/src/features/models/components/drawers/model-capabilities-section.tsx
| 函数名 | 职责 |
|---|---|
| `ModelCapabilitiesSection` | 分区组件：模式切换、表单字段、JSON 编辑器、错误显示，经 react-hook-form 的 `capabilities` 字段读写 |

## 协作关系
- 复用：`SideDrawerSection` / `SideDrawerSectionHeader`（`@/components/drawer-layout`）、`JsonCodeEditor`（`@/components/json-code-editor`）、`Checkbox`、`Switch`、`Input`、`Button`、`FormField` 系列；不新做编辑器或开关。现有 `JsonEditor` 是键值对编辑器，不适合五项固定字段，所以这里直接用 `JsonCodeEditor`，在变更说明里写明。
- 保存仍走 `createModel` / `updateModel`（`features/models/api.ts:76,89`），请求体多一个 `capabilities` 字符串。
- 后端校验错误（400）的 message 是英文 key，经现有 `handleServerError` / 错误翻译显示。

## 验证方式
- 测试入口：`model-capabilities.ts` 导出函数（vitest）；抽屉组件渲染测试（Testing Library，mock 模型接口）。
- 测试输入：主路径能力；`["high","low","high"]`；`"video"`、`"ultra"`、`"foo":1`；上下文 0、-5、1.5、abc、9007199254740992、9007199254740991；缺右括号的 JSON。
- 预期结果：
- [ ] 解析/规范化/序列化结果与规范 JSON 一致，错误含字段路径
- [ ] 分区位置、标题、按钮文字、说明、选项与约束一致
- [ ] 全未配置可保存，提交体 `capabilities` 为空串
- [ ] 开关打开时七档禁用，关上后恢复原勾选
- [ ] 数字越界提示「请输入大于 0 的整数」且保存按钮不提交
- [ ] JSON 格式错误时「可视模式」与保存不可用
- [ ] 表单 ↔ JSON 切换内容一致
- [ ] 7 个 locale 文件都有新 key；`bun run typecheck`、`bun run lint`、`bun run test` 通过
