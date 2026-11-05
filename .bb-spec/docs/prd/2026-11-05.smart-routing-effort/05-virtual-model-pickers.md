---
name: 05-virtual-model-pickers
description: 后台用户模型列表补上虚拟模型，CC Switch、密钥模型限制、演练场都能选到虚拟模型
date: 2026-11-05
---

# 各处可选虚拟模型

## 用户故事
### P1：作为员工，我想在后台选模型的地方直接选到虚拟模型，以便不用手动敲名字

**用例（主路径）**
- 场景：管理员配好了 auto-gpt（接口 Responses、Anthropic）和 auto-chat（接口 Chat）。
- 操作：员工在密钥页点「填入 CC Switch」，应用选 Codex，展开主模型下拉。
- 预期：列表先是真实模型，最后是 auto-gpt、auto-chat，写法和真实模型一样。
- 操作：新建密钥，在「高级设置 → 模型限制」里勾选 gpt-5.5 和 auto-gpt，保存。
- 预期：这个密钥只能请求 gpt-5.5 和 auto-gpt；请求 auto-gpt 时不检查它背后的真实模型在不在白名单里。

**用例（边界 / 异常）**
- 场景：员工打开演练场的模型下拉。
- 预期：能看到 auto-chat，看不到 auto-gpt（auto-gpt 没勾 Chat）。
- 场景：在「填入 CC Switch」里应用选 Gemini。
- 预期：主模型下拉里没有虚拟模型。

## 约束
- C1：后台的用户模型列表接口（`/api/user/models`，带或不带分组参数）返回的列表末尾加上全部已配置的虚拟模型名，排在真实模型后面；不按用户分组过滤虚拟模型，和 `/v1/models` 的规则一致。
- C2：「填入 CC Switch」弹窗：应用选 Claude 或 Codex 时，各个模型下拉（Claude 的主模型、Haiku、Sonnet、Opus 模型，Codex 的主模型）都列出全部虚拟模型，排在真实模型后面，写法和真实模型一样；应用选 Gemini 时不列虚拟模型。
- C3：新建 / 编辑密钥抽屉的「模型限制」下拉里列出全部虚拟模型，排在真实模型后面，可以和真实模型一样勾选、显示成已选标签、点 × 移除。白名单的生效规则沿用一期：白名单里有虚拟模型名就放行这个虚拟模型。
- C4：演练场的模型下拉只列接口里勾了 Chat Completions 的虚拟模型，排在真实模型后面。
- C5：价格页、「模型定价」设置、智能路由档位的模型下拉、渠道的模型配置里不出现虚拟模型。
- C6：首页「快速开始」的示例代码仍用列表里第一个真实模型。

## 验收
- [ ] C1：配置 auto-gpt、auto-chat 后请求 `/api/user/models` 和 `/api/user/models?group=subscription` → 两次结果末尾都是 auto-gpt、auto-chat。
- [ ] C2：CC Switch 选 Codex → 主模型下拉末尾有两个虚拟模型；选 Claude → 四个下拉末尾都有；选 Gemini → 没有虚拟模型。
- [ ] C3：在模型限制里勾 auto-gpt 并保存 → 编辑这个密钥时 auto-gpt 显示为已选；用它请求 auto-gpt 成功，请求白名单外的真实模型被拒。
- [ ] C4：演练场模型下拉里有 auto-chat，没有 auto-gpt；把 auto-gpt 也勾上 Chat 并保存更改 → 刷新后演练场出现 auto-gpt。
- [ ] C5：价格页、模型定价设置、档位模型下拉、渠道模型配置里搜 auto-gpt → 都搜不到。
- [ ] C6：首页快速开始的示例代码里的模型名是第一个真实模型，不是虚拟模型。

## 验证路径
1. 配好 auto-gpt、auto-chat → 打开密钥页「填入 CC Switch」，分别切 Codex、Claude、Gemini 看下拉。
2. 新建密钥只勾 auto-gpt → 用它在 Codex 里请求 auto-gpt 成功。
3. 打开演练场 → 只看到 auto-chat；选它发一条消息 → 正常回答，日志里显示「auto-chat · <档位>」。
4. 拿实际页面和确认过的预览对比：CC Switch 三种应用、密钥模型限制、演练场下拉。
