package claude_oauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const claudeCodeIdentityPrompt = "You are Claude Code, Anthropic's official CLI for Claude."

const validOAuthKeyJSON = `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-xxx","refreshToken":"r","expiresAt":1751808000000}}`

func newRelayInfo(apiKey string, channelSetting dto.ChannelSettings) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ApiKey:         apiKey,
		ChannelBaseUrl: "https://api.anthropic.com",
		ChannelType:    constant.ChannelTypeClaudeSubscription,
		ChannelSetting: channelSetting,
	}}
}

func newGinContext(clientHeaders map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	for k, v := range clientHeaders {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c
}

func TestSetupRequestHeader_ValidOAuthKey_SetsAuthAndBetaAndVersion(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{})
	c := newGinContext(nil)
	req := &http.Header{}

	err := (&Adaptor{}).SetupRequestHeader(c, req, info)

	require.NoError(t, err)
	assert.Equal(t, "Bearer sk-ant-oat01-xxx", req.Get("Authorization"))
	assert.Contains(t, req.Get("anthropic-beta"), "oauth-2025-04-20")
	assert.Equal(t, "2023-06-01", req.Get("anthropic-version"))
	assert.Empty(t, req.Get("x-api-key"))
}

func TestSetupRequestHeader_MergesClientAnthropicBetaHeader(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{})
	c := newGinContext(map[string]string{"anthropic-beta": "prompt-caching-2024-07-31"})
	req := &http.Header{}

	err := (&Adaptor{}).SetupRequestHeader(c, req, info)

	require.NoError(t, err)
	merged := req.Get("anthropic-beta")
	assert.Contains(t, merged, "prompt-caching-2024-07-31")
	assert.Contains(t, merged, "oauth-2025-04-20")
}

func TestSetupRequestHeader_InvalidCredential_ReturnsError(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
	}{
		{"key is not JSON", "sk-ant-not-json-string"},
		{"claudeAiOauth field missing", `{"foo":"bar"}`},
		{"accessToken is empty", `{"claudeAiOauth":{"accessToken":"","refreshToken":"r","expiresAt":1751808000000}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := newRelayInfo(tt.apiKey, dto.ChannelSettings{})
			c := newGinContext(nil)
			req := &http.Header{}

			err := (&Adaptor{}).SetupRequestHeader(c, req, info)

			require.Error(t, err)
		})
	}
}

func TestConvertClaudeRequest_NoClientSystem_IdentityIsOnlySystemBlock(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{})
	c := newGinContext(nil)
	request := &dto.ClaudeRequest{Model: "claude-3-5-sonnet-20241022"}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](convertedRequest.System)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	assert.Equal(t, claudeCodeIdentityPrompt, blocks[0].GetText())
}

func TestConvertClaudeRequest_ClientSystemPresent_IdentityPrependsBeforeIt(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{})
	c := newGinContext(nil)
	request := &dto.ClaudeRequest{
		Model:  "claude-3-5-sonnet-20241022",
		System: "你是翻译助手",
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](convertedRequest.System)
	require.NoError(t, err)
	require.Len(t, blocks, 2)
	assert.Equal(t, claudeCodeIdentityPrompt, blocks[0].GetText())
	assert.Equal(t, "你是翻译助手", blocks[1].GetText())
}

// 共享请求管道先于 adaptor 运行：客户端未带 system 时，管道已把 System 置为自定义串。
// adaptor 只应前置身份串，不得对已含自定义串的 System 再次注入。
func TestConvertClaudeRequest_SystemPromptOverride_OrderIsIdentityThenCustomPrompt(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{
		SystemPrompt:         "输出简体中文",
		SystemPromptOverride: true,
	})
	c := newGinContext(nil)
	request := &dto.ClaudeRequest{Model: "claude-3-5-sonnet-20241022", System: "输出简体中文"}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](convertedRequest.System)
	require.NoError(t, err)
	require.Len(t, blocks, 2)
	assert.Equal(t, claudeCodeIdentityPrompt, blocks[0].GetText())
	assert.Equal(t, "输出简体中文", blocks[1].GetText())
}

func joinSystemText(blocks []dto.ClaudeMediaMessage) string {
	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		texts = append(texts, block.GetText())
	}
	return strings.Join(texts, "\n")
}

func TestConvertClaudeRequest_SystemPromptOverride_ClientSystemStringAlreadyMerged_CustomPromptNotDuplicated(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{
		SystemPrompt:         "输出简体中文",
		SystemPromptOverride: true,
	})
	c := newGinContext(nil)
	request := &dto.ClaudeRequest{
		Model:  "claude-3-5-sonnet-20241022",
		System: "输出简体中文\n你是翻译助手",
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](convertedRequest.System)
	require.NoError(t, err)
	require.NotEmpty(t, blocks)
	assert.Equal(t, claudeCodeIdentityPrompt, blocks[0].GetText())
	joined := joinSystemText(blocks)
	assert.Equal(t, 1, strings.Count(joined, "输出简体中文"))
	assert.Contains(t, joined, "你是翻译助手")
}

func TestConvertClaudeRequest_SystemPromptOverride_ClientSystemArrayAlreadyMerged_CustomPromptNotDuplicated(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{
		SystemPrompt:         "输出简体中文",
		SystemPromptOverride: true,
	})
	c := newGinContext(nil)
	request := &dto.ClaudeRequest{
		Model: "claude-3-5-sonnet-20241022",
		System: []dto.ClaudeMediaMessage{
			{Type: "text", Text: common.GetPointer("输出简体中文")},
			{Type: "text", Text: common.GetPointer("你是翻译助手")},
		},
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](convertedRequest.System)
	require.NoError(t, err)
	require.NotEmpty(t, blocks)
	assert.Equal(t, claudeCodeIdentityPrompt, blocks[0].GetText())
	joined := joinSystemText(blocks)
	assert.Equal(t, 1, strings.Count(joined, "输出简体中文"))
	assert.Contains(t, joined, "你是翻译助手")
}

func TestConvertClaudeRequest_SystemPromptNotOverride_ClientSystemPresent_CustomPromptNotInjected(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{
		SystemPrompt:         "输出简体中文",
		SystemPromptOverride: false,
	})
	c := newGinContext(nil)
	request := &dto.ClaudeRequest{
		Model:  "claude-3-5-sonnet-20241022",
		System: "你是翻译助手",
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](convertedRequest.System)
	require.NoError(t, err)
	require.Len(t, blocks, 2)
	assert.Equal(t, claudeCodeIdentityPrompt, blocks[0].GetText())
	assert.Equal(t, "你是翻译助手", blocks[1].GetText())
	for _, block := range blocks {
		assert.NotEqual(t, "输出简体中文", block.GetText())
	}
}

func TestConvertClaudeRequest_ClientAlreadyHasIdentityString_NotDuplicated(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{})
	c := newGinContext(nil)
	request := &dto.ClaudeRequest{
		Model:  "claude-3-5-sonnet-20241022",
		System: claudeCodeIdentityPrompt,
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](convertedRequest.System)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	assert.Equal(t, claudeCodeIdentityPrompt, blocks[0].GetText())
}

func TestGetRequestURL_EndsWithV1Messages(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{})

	url, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(url, "/v1/messages"))
}

func TestConvertClaudeRequest_ArraySystemFromRealUnmarshal_ClientBlockPreservedWithIdentityPrepended(t *testing.T) {
	var request dto.ClaudeRequest
	err := common.Unmarshal([]byte(`{"model":"claude-3-5-sonnet","system":[{"type":"text","text":"你是翻译助手"}],"messages":[{"role":"user","content":"hi"}]}`), &request)
	require.NoError(t, err)

	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{})
	c := newGinContext(nil)

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, &request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](convertedRequest.System)
	require.NoError(t, err)
	require.Len(t, blocks, 2)
	assert.Equal(t, claudeCodeIdentityPrompt, blocks[0].GetText())
	assert.Equal(t, "你是翻译助手", blocks[1].GetText())
}

func TestConvertOpenAIRequest_InjectsIdentityBeforeClientSystemMessage(t *testing.T) {
	info := newRelayInfo(validOAuthKeyJSON, dto.ChannelSettings{})
	c := newGinContext(nil)
	request := &dto.GeneralOpenAIRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []dto.Message{
			{Role: "system", Content: "你是翻译助手"},
			{Role: "user", Content: "hi"},
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](convertedRequest.System)
	require.NoError(t, err)
	require.Len(t, blocks, 2)
	assert.Equal(t, claudeCodeIdentityPrompt, blocks[0].GetText())
	assert.Equal(t, "你是翻译助手", blocks[1].GetText())
}
