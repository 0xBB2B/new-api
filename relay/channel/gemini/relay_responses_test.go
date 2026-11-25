package gemini

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiResponsesHandlerReturnsOpenAIResponsesJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "gemini-responses-test")

	info := newGeminiResponsesRelayInfo(false)
	payload := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "hello"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     2,
			CandidatesTokenCount: 3,
			TotalTokenCount:      5,
		},
	}
	body, err := common.Marshal(payload)
	require.NoError(t, err)

	usage, newAPIError := GeminiResponsesHandler(c, info, &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	})
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 2, usage.PromptTokens)
	assert.Equal(t, 3, usage.CompletionTokens)

	got := recorder.Body.String()
	assert.Contains(t, got, `"object":"response"`)
	assert.Contains(t, got, `"status":"completed"`)
	assert.Contains(t, got, `"type":"output_text"`)
	assert.Contains(t, got, `"text":"hello"`)
	assert.Contains(t, got, `"input_tokens":2`)
	assert.Contains(t, got, `"output_tokens":3`)
	assert.NotContains(t, got, `"choices"`)
	assert.NotContains(t, got, `"candidates"`)
}

func TestGeminiResponsesHandlerTagsReasoningID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "gemini-reasoning-tag-test")
	common.SetContextKey(c, constant.ContextKeyChannelId, 7)

	payload := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Content: dto.GeminiChatContent{
				Role: "model",
				Parts: []dto.GeminiPart{
					{Text: "pondering", Thought: true},
					{Text: "hello"},
				},
			},
		}},
		UsageMetadata: dto.GeminiUsageMetadata{PromptTokenCount: 2, CandidatesTokenCount: 3, TotalTokenCount: 5},
	}
	body, err := common.Marshal(payload)
	require.NoError(t, err)

	_, newAPIError := GeminiResponsesHandler(c, newGeminiResponsesRelayInfo(false), &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	})
	require.Nil(t, newAPIError)

	var got struct {
		Output []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"output"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &got))
	var reasoningIDs []string
	for _, item := range got.Output {
		if item.Type == "reasoning" {
			reasoningIDs = append(reasoningIDs, item.ID)
		}
	}
	require.NotEmpty(t, reasoningIDs, "body=%s", recorder.Body.String())
	for _, id := range reasoningIDs {
		channelID, _, ok := service.DecodeReasoningTag(id)
		require.True(t, ok, "id=%q", id)
		assert.Equal(t, 7, channelID)
	}
}

func TestGeminiResponsesHandlerTagsMessageID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "gemini-message-tag-test")
	common.SetContextKey(c, constant.ContextKeyChannelId, 7)

	payload := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Content: dto.GeminiChatContent{
				Role:  "model",
				Parts: []dto.GeminiPart{{Text: "hello"}},
			},
		}},
		UsageMetadata: dto.GeminiUsageMetadata{PromptTokenCount: 2, CandidatesTokenCount: 3, TotalTokenCount: 5},
	}
	body, err := common.Marshal(payload)
	require.NoError(t, err)

	_, newAPIError := GeminiResponsesHandler(c, newGeminiResponsesRelayInfo(false), &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	})
	require.Nil(t, newAPIError)

	var got struct {
		Output []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"output"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &got))
	var messageIDs []string
	for _, item := range got.Output {
		if item.Type == "message" {
			messageIDs = append(messageIDs, item.ID)
		}
	}
	require.NotEmpty(t, messageIDs, "body=%s", recorder.Body.String())
	for _, id := range messageIDs {
		channelID, _, ok := service.DecodeReasoningTag(id)
		require.True(t, ok, "id=%q", id)
		assert.Equal(t, 7, channelID)
	}
}

func TestGeminiResponsesHandlerClosesBodyOnReadError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "gemini-responses-read-error-test")

	body := &failingReadCloser{}
	usage, newAPIError := GeminiResponsesHandler(c, newGeminiResponsesRelayInfo(false), &http.Response{Body: body})

	require.Nil(t, usage)
	require.NotNil(t, newAPIError)
	assert.True(t, body.closed)
}

func TestGeminiResponsesStreamHandlerReturnsOpenAIResponsesSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "gemini-responses-stream-test")

	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	info := newGeminiResponsesRelayInfo(true)
	first := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "hello"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     2,
			CandidatesTokenCount: 3,
			TotalTokenCount:      5,
		},
	}
	stop := "STOP"
	final := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				FinishReason: &stop,
				Content: dto.GeminiChatContent{
					Role:  "model",
					Parts: []dto.GeminiPart{{Text: ""}},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     2,
			CandidatesTokenCount: 3,
			TotalTokenCount:      5,
		},
	}
	firstData, err := common.Marshal(first)
	require.NoError(t, err)
	finalData, err := common.Marshal(final)
	require.NoError(t, err)
	streamBody := strings.Join([]string{
		"data: " + string(firstData),
		"",
		"data: " + string(finalData),
		"",
		"data: [DONE]",
		"",
	}, "\n")

	usage, newAPIError := GeminiResponsesStreamHandler(c, info, &http.Response{
		Body: io.NopCloser(strings.NewReader(streamBody)),
	})
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 5, usage.TotalTokens)

	got := recorder.Body.String()
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	assert.Contains(t, got, `event: response.created`)
	assert.Contains(t, got, `event: response.output_text.delta`)
	assert.Contains(t, got, `"delta":"hello"`)
	assert.Contains(t, got, `event: response.completed`)
	assert.Contains(t, got, `"input_tokens":2`)
	assert.Contains(t, got, `"output_tokens":3`)
	assert.NotContains(t, got, `"choices"`)
	assert.NotContains(t, got, `"candidates"`)
	requireOrderedGeminiResponsesSubstrings(t, got,
		`event: response.created`,
		`event: response.output_item.added`,
		`event: response.output_text.delta`,
		`event: response.output_text.done`,
		`event: response.completed`,
	)
}

func newGeminiResponsesRelayInfo(isStream bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		IsStream:        isStream,
		RelayMode:       relayconstant.RelayModeResponses,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RequestURLPath:  "/v1/responses",
		DisablePing:     true,
		OriginModelName: "gemini-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-test",
		},
	}
}

type failingReadCloser struct {
	closed bool
}

func (r *failingReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func (r *failingReadCloser) Close() error {
	r.closed = true
	return nil
}

func requireOrderedGeminiResponsesSubstrings(t *testing.T, s string, parts ...string) {
	t.Helper()
	offset := 0
	for _, part := range parts {
		idx := strings.Index(s[offset:], part)
		require.NotEqualf(t, -1, idx, "missing %q after byte offset %d", part, offset)
		offset += idx + len(part)
	}
}

func TestGeminiResponsesStreamHandlerPreBodyFailure(t *testing.T) {
	for _, tc := range []struct {
		name      string
		upstream  string
		wantErr   string
		oldClient []string
	}{
		{name: "unparsable first frame", upstream: "data: {busy\n\n", wantErr: "unmarshal Gemini stream response", oldClient: []string{"event: response.failed", "unmarshal Gemini stream response"}},
	} {
		for _, mode := range []struct {
			name  string
			probe func(*types.NewAPIError) bool
		}{
			{"probe true", func(*types.NewAPIError) bool { return true }},
			{"probe false", func(*types.NewAPIError) bool { return false }},
			{"no probe", nil},
		} {
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				gin.SetMode(gin.TestMode)
				oldStreamingTimeout := constant.StreamingTimeout
				constant.StreamingTimeout = 300
				t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				if mode.probe != nil {
					helper.SetFailoverProbe(c, mode.probe)
				}

				_, apiErr := GeminiResponsesStreamHandler(c, newGeminiResponsesRelayInfo(true), &http.Response{Body: io.NopCloser(strings.NewReader(tc.upstream))})

				if mode.name == "probe true" {
					assert.Empty(t, recorder.Body.String())
					assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyStreamOutputSent))
					require.NotNil(t, apiErr)
					assert.Contains(t, apiErr.Error(), tc.wantErr)
					assert.False(t, types.IsSkipRetryError(apiErr))
					return
				}
				assert.Nil(t, apiErr)
				assert.True(t, common.GetContextKeyBool(c, constant.ContextKeyStreamOutputSent))
				for _, want := range tc.oldClient {
					assert.Contains(t, recorder.Body.String(), want)
				}
			})
		}
	}
}

func TestGeminiResponsesStreamHandlerProbeReceivesReturnedError(t *testing.T) {
	for _, mode := range []struct {
		name   string
		answer bool
	}{{"probe true", true}, {"probe false", false}} {
		t.Run(mode.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			oldStreamingTimeout := constant.StreamingTimeout
			constant.StreamingTimeout = 300
			t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			var seen *types.NewAPIError
			helper.SetFailoverProbe(c, func(err *types.NewAPIError) bool {
				seen = err
				return mode.answer
			})

			usage, apiErr := GeminiResponsesStreamHandler(c, newGeminiResponsesRelayInfo(true), &http.Response{Body: io.NopCloser(strings.NewReader("data: {busy\n\n"))})

			if mode.answer {
				assert.Empty(t, recorder.Body.String())
				require.NotNil(t, apiErr)
				require.NotNil(t, seen)
				assert.Equal(t, apiErr.GetErrorCode(), seen.GetErrorCode())
				assert.Equal(t, apiErr.StatusCode, seen.StatusCode)
				return
			}
			assert.Nil(t, apiErr)
			assert.NotNil(t, usage)
			assert.Contains(t, recorder.Body.String(), "event: response.failed")
		})
	}
}

func TestGeminiResponsesStreamHandlerRealRuleProbeKeepsBadFrameBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	var seen *types.NewAPIError
	helper.SetFailoverProbe(c, func(err *types.NewAPIError) bool {
		seen = err
		return !operation_setting.IsAlwaysSkipRetryCode(err.GetErrorCode())
	})

	usage, apiErr := GeminiResponsesStreamHandler(c, newGeminiResponsesRelayInfo(true), &http.Response{Body: io.NopCloser(strings.NewReader("data: {busy\n\n"))})

	require.NotNil(t, seen)
	assert.True(t, operation_setting.IsAlwaysSkipRetryCode(seen.GetErrorCode()))
	assert.Nil(t, apiErr)
	assert.NotNil(t, usage)
	assert.True(t, common.GetContextKeyBool(c, constant.ContextKeyStreamOutputSent))
	assert.Contains(t, recorder.Body.String(), "event: response.failed")
}

func TestGeminiResponsesStreamHandlerFailsOverWithoutFinishOrBody(t *testing.T) {
	const usageFrame = `{"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":0,"totalTokenCount":3}}`
	const finishFrame = `{"candidates":[{"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":3,"totalTokenCount":3}}`
	const blockedFrame = `{"promptFeedback":{"blockReason":"SAFETY"},"usageMetadata":{"promptTokenCount":3,"totalTokenCount":3}}`
	for _, tc := range []struct {
		name     string
		upstream string
		failover bool
	}{
		{name: "usage frame then EOF", upstream: "data: " + usageFrame + "\n\n", failover: true},
		{name: "no frame then EOF", failover: true},
		{name: "finishReason without body is a normal end", upstream: "data: " + finishFrame + "\n\n"},
		{name: "blocked prompt is a normal end", upstream: "data: " + blockedFrame + "\n\n"},
	} {
		for _, mode := range []struct {
			name  string
			probe func(*types.NewAPIError) bool
		}{
			{"probe true", func(*types.NewAPIError) bool { return true }},
			{"probe false", func(*types.NewAPIError) bool { return false }},
			{"no probe", nil},
		} {
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				gin.SetMode(gin.TestMode)
				oldStreamingTimeout := constant.StreamingTimeout
				constant.StreamingTimeout = 300
				t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				if mode.probe != nil {
					helper.SetFailoverProbe(c, mode.probe)
				}

				_, apiErr := GeminiResponsesStreamHandler(c, newGeminiResponsesRelayInfo(true), &http.Response{Body: io.NopCloser(strings.NewReader(tc.upstream))})

				if tc.failover && mode.name == "probe true" {
					assert.Empty(t, recorder.Body.String())
					assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyStreamOutputSent))
					require.NotNil(t, apiErr)
					assert.False(t, types.IsSkipRetryError(apiErr))
					return
				}
				assert.Nil(t, apiErr)
				assert.True(t, common.GetContextKeyBool(c, constant.ContextKeyStreamOutputSent))
				assert.Contains(t, recorder.Body.String(), "event: response.created")
				assert.Contains(t, recorder.Body.String(), "event: response.completed")
			})
		}
	}
}
