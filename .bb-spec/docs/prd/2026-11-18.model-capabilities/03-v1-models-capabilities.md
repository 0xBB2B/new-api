---
name: 03-v1-models-capabilities
description: /v1/models 的 OpenAI 格式按 OpenRouter 命名返回配置过的模型能力，未配置的不返回
date: 2026-11-18
---

# /v1/models 返回能力

## 用户故事
### P0：作为接 API 的客户端开发者，我想从模型列表里读到每个模型的能力，以便只给用户展示模型支持的选项

**用例（主路径）**
- 场景：`gpt-6.2-sol` 配置了全部五项能力；`qwen4-max` 只配了思考等级 none、low、high、xhigh；`gpt-4o` 配置为不支持思考；`foo-model` 什么都没配；`auto` 是智能路由虚拟模型。
- 操作：用 API 令牌请求 `GET /v1/models`。
- 预期：
  - `gpt-6.2-sol` 的对象多出 `"context_length": 1050000`、`"top_provider": {"max_completion_tokens": 128000}`、`"architecture": {"input_modalities": ["text","image"], "output_modalities": ["text"]}`、`"reasoning": {"supported_efforts": ["none","low","medium","high","xhigh","max"]}`。
  - `qwen4-max` 只多出 `"reasoning": {"supported_efforts": ["none","low","high","xhigh"]}`。
  - `gpt-4o` 多出 `"reasoning": {"supported_efforts": []}`。
  - `foo-model`、`auto` 没有任何能力字段。
  - 所有对象原有的 `id`、`object`、`created`、`owned_by`、`supported_endpoint_types` 不变。

**用例（边界 / 异常）**
- 场景：前缀记录 `gpt-6` 配置了上下文大小 1050000，没有 `gpt-6.2-luna` 的完全匹配记录。操作：请求 `/v1/models`。预期：`gpt-6.2-luna` 返回 `"context_length": 1050000`。
- 场景：`claude-opus-5` 没有任何模型记录。操作：请求 `/v1/models`。预期：按 2026 年 Claude 的推算规则返回 `reasoning.supported_efforts`，其他能力字段不出现。
- 场景：请求带 `x-api-key` 走 Anthropic 格式，或走 Gemini 格式。预期：返回内容与改动前完全一样，没有能力字段。

## 约束
- C1：只有 `/v1/models` 的 OpenAI 格式返回能力字段；Anthropic 格式、Gemini 格式的返回内容不变。
- C2：字段位置与命名：上下文大小 → `context_length`；最大输出 token → `top_provider.max_completion_tokens`；输入模态 → `architecture.input_modalities`；输出模态 → `architecture.output_modalities`；思考等级 → `reasoning.supported_efforts`。
- C3：每个字段只在已配置时出现；未配置的字段不出现；`top_provider`、`architecture`、`reasoning` 里一个字段都没有时，这个对象本身也不出现。
- C4：确认不支持思考的模型返回 `"reasoning": {"supported_efforts": []}`。
- C5：思考档位按 none、minimal、low、medium、high、xhigh、max 的顺序返回。
- C6：一个模型名的能力取自命中它的模型记录，命中顺序沿用模型管理现有规则：完全匹配优先，其次前缀、后缀、包含，同一种规则里取第一条命中的；整条记录一起生效，不跨记录逐个字段拼接。
- C7：命中记录没有配置思考等级（或没有命中任何记录）时，思考等级按代码里的 2026 年 Claude、Gemini 3.x 推算规则（以及 relaykit 判定不支持思考的 Gemini 模型，不论年份，推算为不支持思考）取值；推算规则也认不出时为「能力未确认」，不返回 `reasoning`。其他四项能力没有推算规则，未配置就不返回。
- C8：智能路由虚拟模型的对象不返回任何能力字段。
- C9：模型记录改动（编辑抽屉保存、「能力配置 JSON」保存）后，下一次请求 `/v1/models` 就返回新值。
- C10：原有字段 `id`、`object`、`created`、`owned_by`、`supported_endpoint_types` 的值和顺序不变；模型列表的范围（哪些模型出现）不变。

## 验收
- [ ] C1：同一个令牌分别按 OpenAI、Anthropic、Gemini 格式请求 → 只有 OpenAI 格式出现能力字段，另两种与改动前逐字段一致。
- [ ] C2：主路径 `gpt-6.2-sol` 返回的字段路径与约束一致。
- [ ] C3：`qwen4-max` 只有 `reasoning`；`foo-model` 没有 `top_provider`、`architecture`、`reasoning`、`context_length`。
- [ ] C4：`gpt-4o` 返回 `supported_efforts: []`。
- [ ] C5：记录里存的档位乱序时，返回仍按七档顺序。
- [ ] C6：同时存在完全匹配记录 `gpt-6.2-sol`（未配置上下文大小）和前缀记录 `gpt-6`（上下文 1050000）→ `gpt-6.2-sol` 不返回 `context_length`；`gpt-6.2-luna` 返回 1050000。
- [ ] C7：无记录的 `claude-opus-5` 返回推算出的档位；无记录且推算不出的 `foo-model` 不返回 `reasoning`。
- [ ] C8：`auto` 没有能力字段。
- [ ] C9：在编辑抽屉改 `gpt-6.2-sol` 的上下文大小并保存 → 紧接着请求 `/v1/models` 返回新值。
- [ ] C10：改动前后同一令牌返回的模型 `id` 列表一致，原有五个字段的值一致。

## 验证路径
1. 按主路径准备五个模型的配置。
2. 用 API 令牌请求 `GET /v1/models` → 逐个核对五个模型的字段。
3. 换 Anthropic、Gemini 格式请求 → 没有能力字段。
4. 在编辑抽屉改一项能力并保存 → 再请求一次，返回新值。
