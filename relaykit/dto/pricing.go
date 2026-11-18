package dto

import "github.com/QuantumNous/new-api/relaykit/types"

// 这里不好动就不动了，本来想独立出来的（
type OpenAIModels struct {
	Id                     string                   `json:"id"`
	Object                 string                   `json:"object"`
	Created                int                      `json:"created"`
	OwnedBy                string                   `json:"owned_by"`
	SupportedEndpointTypes []types.EndpointType     `json:"supported_endpoint_types"`
	ContextLength          *int64                   `json:"context_length,omitempty"`
	TopProvider            *OpenAIModelTopProvider  `json:"top_provider,omitempty"`
	Architecture           *OpenAIModelArchitecture `json:"architecture,omitempty"`
	Reasoning              *OpenAIModelReasoning    `json:"reasoning,omitempty"`
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

type AnthropicModel struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

type GeminiModel struct {
	Name                       any   `json:"name"`
	BaseModelId                any   `json:"baseModelId"`
	Version                    any   `json:"version"`
	DisplayName                any   `json:"displayName"`
	Description                any   `json:"description"`
	InputTokenLimit            any   `json:"inputTokenLimit"`
	OutputTokenLimit           any   `json:"outputTokenLimit"`
	SupportedGenerationMethods []any `json:"supportedGenerationMethods"`
	Thinking                   any   `json:"thinking"`
	Temperature                any   `json:"temperature"`
	MaxTemperature             any   `json:"maxTemperature"`
	TopP                       any   `json:"topP"`
	TopK                       any   `json:"topK"`
}
