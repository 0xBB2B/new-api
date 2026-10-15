package codex

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLAlphaSearch(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodex,
			ChannelBaseUrl: "https://chatgpt.com",
		},
		RelayMode: relayconstant.RelayModeAlphaSearch,
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://chatgpt.com/backend-api/codex/alpha/search", url)
}

// The Codex backend rejects these fields, so the adaptor clears them rather
// than forwarding what the client sent.
func TestConvertOpenAIResponsesRequestDropsPenalties(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeCodex},
		RelayMode:   relayconstant.RelayModeResponses,
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:            "gpt-5-codex",
		Input:            json.RawMessage(`"hello"`),
		MaxOutputTokens:  lo.ToPtr(uint(128)),
		Temperature:      lo.ToPtr(1.0),
		FrequencyPenalty: json.RawMessage(`1.5`),
		PresencePenalty:  json.RawMessage(`1.5`),
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Nil(t, request.MaxOutputTokens)
	assert.Nil(t, request.Temperature)
	assert.Nil(t, request.FrequencyPenalty)
	assert.Nil(t, request.PresencePenalty)
}

func newCodexResponsesInfo(mode int, isStream bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeCodex,
			UpstreamModelName: "gpt-5.1",
			ApiKey:            `{"access_token":"x","account_id":"y"}`,
		},
		OriginModelName: "gpt-5.1",
		RelayMode:       mode,
		IsStream:        isStream,
		DisablePing:     true,
		ResponsesUsageInfo: &relaycommon.ResponsesUsageInfo{
			BuiltInTools: map[string]*relaycommon.BuildInToolInfo{
				dto.BuildInToolWebSearchPreview: {ToolName: dto.BuildInToolWebSearchPreview},
			},
		},
	}
}

func TestConvertOpenAIResponsesRequestStreamField(t *testing.T) {
	tests := []struct {
		name       string
		mode       int
		stream     *bool
		wantStream any
	}{
		{"responses stream omitted", relayconstant.RelayModeResponses, nil, true},
		{"responses stream false", relayconstant.RelayModeResponses, lo.ToPtr(false), true},
		{"responses stream true", relayconstant.RelayModeResponses, lo.ToPtr(true), true},
		{"compact omitted stays absent", relayconstant.RelayModeResponsesCompact, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := newCodexResponsesInfo(tt.mode, false)
			converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
				Model:  "gpt-5-codex",
				Input:  json.RawMessage(`"hello"`),
				Stream: tt.stream,
			})
			require.NoError(t, err)

			raw, err := common.Marshal(converted)
			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, common.Unmarshal(raw, &fields))
			assert.Equal(t, tt.wantStream, fields["stream"])
		})
	}
}

func TestSetupRequestHeaderAccept(t *testing.T) {
	tests := []struct {
		name       string
		mode       int
		isStream   bool
		wantAccept string
	}{
		{"responses non-stream client still asks for SSE", relayconstant.RelayModeResponses, false, "text/event-stream"},
		{"responses stream client", relayconstant.RelayModeResponses, true, "text/event-stream"},
		{"compact non-stream stays JSON", relayconstant.RelayModeResponsesCompact, false, "application/json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			info := newCodexResponsesInfo(tt.mode, tt.isStream)

			headers := http.Header{}
			require.NoError(t, (&Adaptor{}).SetupRequestHeader(c, &headers, info))
			assert.Equal(t, tt.wantAccept, headers.Get("Accept"))
		})
	}
}

func sseBody(events ...string) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString("data: ")
		b.WriteString(e)
		b.WriteString("\n\n")
	}
	return b.String()
}

func runCodexDoResponse(t *testing.T, isStream bool, body string) (*httptest.ResponseRecorder, *relaycommon.RelayInfo, *dto.Usage, *types.NewAPIError) {
	t.Helper()
	info := newCodexResponsesInfo(relayconstant.RelayModeResponses, isStream)
	recorder, usage, apiErr := runCodexDoResponseWithInfo(t, info, body)
	return recorder, info, usage, apiErr
}

func runCodexDoResponseWithInfo(t *testing.T, info *relaycommon.RelayInfo, body string) (*httptest.ResponseRecorder, *dto.Usage, *types.NewAPIError) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "codex-test")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	var typed *dto.Usage
	if usage != nil {
		var ok bool
		typed, ok = usage.(*dto.Usage)
		require.True(t, ok)
	}
	return recorder, typed, apiErr
}

func decodeJSONObject(t *testing.T, raw string) map[string]any {
	t.Helper()
	var obj map[string]any
	require.NoError(t, common.UnmarshalJsonStr(raw, &obj))
	return obj
}

const codexCompletedResponse = `{"id":"resp_1","object":"response","status":"completed","model":"gpt-5.1","output":[{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"hi"}]}],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15},"x_extra":"keep"}`

func TestDoResponseNonStreamClientGetsCompleteJSON(t *testing.T) {
	body := sseBody(
		`{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"hi"}]}}`,
		`{"type":"response.completed","response":`+codexCompletedResponse+`}`,
	)

	recorder, info, usage, apiErr := runCodexDoResponse(t, false, body)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 5, usage.CompletionTokens)
	assert.Equal(t, 15, usage.TotalTokens)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
	assert.NotContains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	assert.Equal(t, decodeJSONObject(t, codexCompletedResponse), decodeJSONObject(t, recorder.Body.String()))
	require.NotNil(t, info.StreamStatus)
	assert.True(t, info.StreamStatus.IsNormalEnd())
}

func TestDoResponseNonStreamSupplementsEmptyOutputInIndexOrder(t *testing.T) {
	reasoning := `{"type":"reasoning","id":"rs_1","summary":[]}`
	call := `{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"{}"}`
	body := sseBody(
		`{"type":"response.output_item.done","output_index":1,"item":`+call+`}`,
		`{"type":"response.output_item.done","output_index":0,"item":`+reasoning+`}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`,
	)

	recorder, _, _, apiErr := runCodexDoResponse(t, false, body)
	require.Nil(t, apiErr)

	got := decodeJSONObject(t, recorder.Body.String())
	assert.Equal(t, []any{decodeJSONObject(t, reasoning), decodeJSONObject(t, call)}, got["output"])
}

func TestDoResponseNonStreamBillingMatchesStreamPath(t *testing.T) {
	searchItem := `{"type":"web_search_call","id":"ws_1","status":"completed"}`
	withUsage := sseBody(
		`{"type":"response.output_item.done","output_index":0,"item":`+searchItem+`}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`,
		`[DONE]`,
	)

	_, streamInfo, _, streamErr := runCodexDoResponse(t, true, withUsage)
	require.Nil(t, streamErr)
	_, bufferedInfo, _, bufferedErr := runCodexDoResponse(t, false, withUsage)
	require.Nil(t, bufferedErr)

	streamCount := streamInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview].CallCount
	require.Equal(t, 1, streamCount)
	assert.Equal(t, streamCount, bufferedInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview].CallCount)
}

func TestDoResponseNonStreamEstimatesUsageWhenMissing(t *testing.T) {
	body := sseBody(
		`{"type":"response.output_text.delta","output_index":0,"delta":"hello there, this is some generated text"}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[]}}`,
		`[DONE]`,
	)

	_, _, streamUsage, streamErr := runCodexDoResponse(t, true, body)
	require.Nil(t, streamErr)
	require.NotNil(t, streamUsage)
	require.Greater(t, streamUsage.CompletionTokens, 0)

	_, _, bufferedUsage, bufferedErr := runCodexDoResponse(t, false, body)
	require.Nil(t, bufferedErr)
	require.NotNil(t, bufferedUsage)
	assert.Equal(t, streamUsage.CompletionTokens, bufferedUsage.CompletionTokens)
}

func TestDoResponseNonStreamIncompleteReturnsResponse(t *testing.T) {
	incomplete := `{"id":"resp_1","status":"incomplete","output":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}`
	body := sseBody(`{"type":"response.incomplete","response":` + incomplete + `}`)

	recorder, _, usage, apiErr := runCodexDoResponse(t, false, body)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 15, usage.TotalTokens)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, decodeJSONObject(t, incomplete), decodeJSONObject(t, recorder.Body.String()))
}

func TestDoResponseNonStreamFailures(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{
			"response.failed",
			sseBody(`{"type":"response.failed","response":{"id":"resp_1","status":"failed","error":{"code":"server_is_overloaded","message":"upstream boom"}}}`),
			"upstream boom",
		},
		{
			"top-level error event",
			sseBody(`{"type":"error","code":"rate_limit_exceeded","message":"slow down","param":null}`),
			"slow down",
		},
		{
			"stream ends without terminal event",
			sseBody(`{"type":"response.output_text.delta","delta":"partial"}`),
			"",
		},
		{
			"malformed data line",
			"data: {not json\n\n",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder, _, _, apiErr := runCodexDoResponse(t, false, tt.body)
			require.NotNil(t, apiErr)
			assert.Empty(t, recorder.Body.String())
			if tt.wantMsg != "" {
				assert.Contains(t, apiErr.Error(), tt.wantMsg)
			}
		})
	}
}

func TestDoResponseNonStreamRetryDoesNotDoubleCountToolCalls(t *testing.T) {
	searchDone := `{"type":"response.output_item.done","output_index":0,"item":{"type":"web_search_call","id":"ws_1","status":"completed"}}`
	failed := sseBody(searchDone, `{"type":"response.failed","response":{"id":"resp_1","status":"failed","error":{"code":"server_is_overloaded","message":"upstream boom"}}}`)
	succeeded := sseBody(searchDone, `{"type":"response.completed","response":{"id":"resp_2","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`)

	info := newCodexResponsesInfo(relayconstant.RelayModeResponses, false)
	_, _, firstErr := runCodexDoResponseWithInfo(t, info, failed)
	require.NotNil(t, firstErr)

	_, _, secondErr := runCodexDoResponseWithInfo(t, info, succeeded)
	require.Nil(t, secondErr)
	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview].CallCount)
}

func TestDoResponseStreamClientStillGetsSSE(t *testing.T) {
	body := sseBody(
		`{"type":"response.completed","response":`+codexCompletedResponse+`}`,
		`[DONE]`,
	)

	recorder, _, usage, apiErr := runCodexDoResponse(t, true, body)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 15, usage.TotalTokens)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	assert.Contains(t, recorder.Body.String(), "response.completed")
}
