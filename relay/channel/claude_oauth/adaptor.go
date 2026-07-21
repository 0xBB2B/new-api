package claude_oauth

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const claudeCodeSystemPrompt = "You are Claude Code, Anthropic's official CLI for Claude."

type Adaptor struct {
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, "/v1/messages", info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	oauthKey, err := ParseOAuthKey(strings.TrimSpace(info.ApiKey))
	if err != nil {
		return err
	}

	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	if accessToken == "" {
		return errors.New("claude subscription channel: access_token is required")
	}
	req.Set("Authorization", "Bearer "+accessToken)

	anthropicBeta := c.Request.Header.Get("anthropic-beta")
	if anthropicBeta == "" {
		req.Set("anthropic-beta", "oauth-2025-04-20")
	} else if !strings.Contains(anthropicBeta, "oauth-2025-04-20") {
		req.Set("anthropic-beta", anthropicBeta+","+"oauth-2025-04-20")
	} else {
		req.Set("anthropic-beta", anthropicBeta)
	}

	anthropicVersion := c.Request.Header.Get("anthropic-version")
	if anthropicVersion == "" {
		anthropicVersion = "2023-06-01"
	}
	req.Set("anthropic-version", anthropicVersion)

	return nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	prependClaudeCodeSystem(request)
	return request, nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatClaude, request)
	if err != nil {
		return nil, err
	}
	claudeReq, ok := result.Value.(*dto.ClaudeRequest)
	if !ok {
		return nil, fmt.Errorf("expected Claude request after conversion, got %T", result.Value)
	}
	prependClaudeCodeSystem(claudeReq)
	return claudeReq, nil
}

func prependClaudeCodeSystem(request *dto.ClaudeRequest) {
	identityBlock := dto.ClaudeMediaMessage{Type: "text", Text: common.GetPointer(claudeCodeSystemPrompt)}
	existing := normalizeClaudeSystem(request.System)
	request.System = append([]dto.ClaudeMediaMessage{identityBlock}, filterIdentityBlock(existing)...)
}

func filterIdentityBlock(blocks []dto.ClaudeMediaMessage) []dto.ClaudeMediaMessage {
	filtered := make([]dto.ClaudeMediaMessage, 0, len(blocks))
	for _, block := range blocks {
		if block.GetText() == claudeCodeSystemPrompt {
			continue
		}
		filtered = append(filtered, block)
	}
	return filtered
}

func normalizeClaudeSystem(system any) []dto.ClaudeMediaMessage {
	switch v := system.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []dto.ClaudeMediaMessage{{Type: "text", Text: common.GetPointer(v)}}
	case []dto.ClaudeMediaMessage:
		return v
	default:
		blocks, _ := common.Any2Type[[]dto.ClaudeMediaMessage](system)
		return blocks
	}
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("claude subscription channel: endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("claude subscription channel: endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("claude subscription channel: endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("claude subscription channel: endpoint not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("claude subscription channel: endpoint not supported")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("claude subscription channel: endpoint not supported")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	info.FinalRequestRelayFormat = types.RelayFormatClaude
	if info.IsStream {
		return claude.ClaudeStreamHandler(c, resp, info)
	}
	return claude.ClaudeHandler(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
