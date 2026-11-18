---
name: 02-v1-models-capabilities
description: /v1/models 的 OpenAI 格式按 OpenRouter 命名返回已配置的模型能力
---
# /v1/models 返回能力

## 目标
调用方用 API 令牌请求 `/v1/models`（OpenAI 格式）时，配置过能力的模型对象带上能力字段。

## 业务规则（来源：PRD 03-v1-models-capabilities）
- 只改 OpenAI 格式；Anthropic、Gemini 格式返回内容不变。
- 字段：`context_length`、`top_provider.max_completion_tokens`、`architecture.input_modalities`、`architecture.output_modalities`、`reasoning.supported_efforts`。
- 只返回已配置的字段；容器对象里一个字段都没有时整个对象不出现。
- 不支持思考返回 `"reasoning": {"supported_efforts": []}`（空数组必须输出，不能省略）。
- 档位按 none→max 顺序。
- 能力取自命中该名字的模型记录（完全匹配 → 前缀 → 后缀 → 包含，整条记录生效）。
- 记录没配思考等级或没命中记录时，思考等级用智能路由的 2026 年 Claude、Gemini 3.x 推算规则；仍认不出（能力未确认）不返回 `reasoning`。其他四项没有推算规则。
- 智能路由虚拟模型不返回能力字段。
- 原有 `id`、`object`、`created`、`owned_by`、`supported_endpoint_types` 的值和顺序、以及列表范围不变。
- 记录改动后下一次请求就返回新值。

## 涉及文件
- 修改 `relaykit/dto/pricing.go`（OpenAIModels 增加四个可选字段及子结构）
- 修改 `controller/model.go`（`buildOpenAIModel` 填能力；`buildSmartRouteModels` 保持不填）
- 修改 `controller/model_test.go` 或现有覆盖 ListModels 的测试文件（无则在 `controller/model_management_test.go` 追加）

## 成品定义
`relaykit/dto/pricing.go` 新增字段（追加在 `SupportedEndpointTypes` 之后，保证原有字段顺序不变）：
```go
type OpenAIModels struct {
	Id                     string               `json:"id"`
	Object                 string               `json:"object"`
	Created                int                  `json:"created"`
	OwnedBy                string               `json:"owned_by"`
	SupportedEndpointTypes []types.EndpointType `json:"supported_endpoint_types"`
	ContextLength          *int64                    `json:"context_length,omitempty"`
	TopProvider            *OpenAIModelTopProvider   `json:"top_provider,omitempty"`
	Architecture           *OpenAIModelArchitecture  `json:"architecture,omitempty"`
	Reasoning              *OpenAIModelReasoning     `json:"reasoning,omitempty"`
}

type OpenAIModelTopProvider struct {
	MaxCompletionTokens *int64 `json:"max_completion_tokens,omitempty"`
}

type OpenAIModelArchitecture struct {
	InputModalities  []string `json:"input_modalities,omitempty"`
	OutputModalities []string `json:"output_modalities,omitempty"`
}

// SupportedEfforts 不带 omitempty：空数组表示确认不支持思考，必须输出。
type OpenAIModelReasoning struct {
	SupportedEfforts []string `json:"supported_efforts"`
}
```

## 函数清单
### controller/model.go
| 函数名 | 职责 |
|---|---|
| `buildOpenAIModel`（现有） | 调 `model.GetModelCapabilityByName` 填 ContextLength、TopProvider、Architecture；思考等级调 `smartroute.EffectiveThinkingCapability`（由 03 改为先查记录再推算），能力未确认时不填 Reasoning，不支持思考填空数组 |

## 协作关系
- 依赖 01 的 `GetModelCapabilityByName`、03 的 `EffectiveThinkingCapability`（返回来源：记录 / 推算 / 未确认）。
- `ListModels` 的 Anthropic、Gemini 分支只取 Id 等原字段，不受影响。
- relaykit 改动后必须在 `relaykit/` 下 `GOWORK=off go build ./...` 通过。

## 验证方式
- 测试入口：`GET /v1/models`（OpenAI 格式、`x-api-key` 的 Anthropic 格式、Gemini 格式），沿用现有 ListModels 测试的令牌与渠道夹具。
- 测试输入：记录 `gpt-6.2-sol` 五项全配；`qwen4-max` 只配思考 none/low/high/xhigh；`gpt-4o` 配不支持思考；`foo-model` 无记录；无记录的 `claude-opus-5`；前缀记录 `gpt-6` 配上下文 1050000 且有渠道模型 `gpt-6.2-luna`；虚拟模型 `auto`。
- 预期结果：
- [ ] `gpt-6.2-sol` 返回四组能力字段，值与配置一致
- [ ] `qwen4-max` 只有 `reasoning`
- [ ] `gpt-4o` 返回 `"supported_efforts": []`
- [ ] `foo-model`、`auto` 没有任何能力字段
- [ ] `claude-opus-5` 返回推算出的档位，没有其他能力字段
- [ ] `gpt-6.2-luna` 返回 `context_length: 1050000`
- [ ] Anthropic、Gemini 格式响应与改动前逐字段相同
- [ ] 原有五个字段的值、模型 id 列表与改动前相同
- [ ] 编辑记录后紧接着请求返回新值
- [ ] `cd relaykit && GOWORK=off go build ./...` 通过
