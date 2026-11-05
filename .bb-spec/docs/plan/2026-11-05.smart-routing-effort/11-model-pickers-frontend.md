---
name: 11-model-pickers-frontend
description: CC Switch 按应用决定是否列虚拟模型，演练场只拿接收 Chat 的虚拟模型，密钥模型限制直接受益
---
# 前端各处的模型下拉

## 目标
CC Switch、密钥模型限制、演练场的模型下拉按规则出现虚拟模型。

## 业务规则（来源：PRD 05-virtual-model-pickers）
- 后端 `/api/user/models` 已在末尾追加虚拟模型；带 `endpoint=<格式>` 时只追加接收该格式的虚拟模型（`endpoint=gemini` 时一个都不追加）。
- C2：「填入 CC Switch」：应用 Claude、Codex → 各模型下拉列出全部虚拟模型（不带参数）；应用 Gemini → 不列虚拟模型（带 `endpoint=gemini`）。虚拟模型写法和真实模型一样，不加标记。
- C3：密钥抽屉「模型限制」下拉列出全部虚拟模型，可勾选、显示成已选标签、点 × 移除；不需要改代码以外的东西，只需确认行为。
- C4：演练场模型下拉只列勾了 Chat 的虚拟模型（带 `endpoint=openai`，同时保留现有的 `group` 参数）。
- C6：首页「快速开始」示例仍用列表第一个模型（虚拟模型排在后面，不受影响）。

## 涉及文件
- 修改 `web/src/lib/api.ts`（`getUserModels` 接受可选的格式参数）
- 修改 `web/src/features/keys/components/dialogs/cc-switch-dialog.tsx`
- 修改 `web/src/features/playground/api.ts`
- 测试：扩展 `web/src/features/keys/components/__tests__/api-keys-mutate-drawer.test.tsx`；新建 `web/src/features/keys/components/__tests__/cc-switch-dialog.test.tsx`（目录里没有同类文件时）；扩展演练场现有测试或新建 `web/src/features/playground/__tests__/playground-models.test.ts`

## 函数清单
### web/src/lib/api.ts
| 函数名 | 职责 |
|---|---|
| `getUserModels` | 可选传入格式，拼到 `endpoint` 查询参数 |
### cc-switch-dialog.tsx
| 函数名 | 职责 |
|---|---|
| `CCSwitchDialog`（现有组件） | 模型列表查询按应用区分：Gemini 带 `endpoint=gemini`，其他不带；查询键包含这个区分 |
### web/src/features/playground/api.ts
| 函数名 | 职责 |
|---|---|
| `getUserModels(group)` | 请求时同时带 `group` 和 `endpoint=openai` |

## 协作关系
依赖 06 的后端参数。密钥抽屉 `api-keys-mutate-drawer.tsx` 已经用 `getUserModels()`，不需要改代码。

## 验证方式
- 测试入口：渲染 CC Switch 弹窗、密钥抽屉；调用演练场 `getUserModels`（mock `api.get` 断言请求参数）
- 测试输入：mock `/api/user/models` 不带参数返回 [gpt-5.5, auto-gpt, auto-chat]，带 `endpoint=gemini` 返回 [gpt-5.5]
- 预期结果：
  - [ ] CC Switch 选 Codex → 主模型选项末尾有 auto-gpt、auto-chat；选 Claude → 四个下拉都有；选 Gemini → 选项里没有虚拟模型，并且请求带了 `endpoint=gemini`
  - [ ] 密钥抽屉模型限制能勾选 auto-gpt，提交的数据里含 auto-gpt
  - [ ] 演练场请求 `/api/user/models` 时参数含 `group` 和 `endpoint=openai`
