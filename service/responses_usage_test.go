package service

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/tokenkit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesUsageAccumulatorTerminalAccounting(t *testing.T) {
	operation_setting.SetToolPriceForTest("responses_priced_fn", 5)
	t.Cleanup(func() { operation_setting.DeleteToolPriceForTest("responses_priced_fn") })
	for _, tc := range []struct {
		eventType  string
		wantImages int
	}{
		{eventType: "response.completed", wantImages: 1},
		{eventType: "response.done", wantImages: 1},
		{eventType: "response.incomplete", wantImages: 1},
		{eventType: "response.failed", wantImages: 1},
		{eventType: "response.cancelled", wantImages: 1},
		{eventType: "response.canceled", wantImages: 1},
	} {
		t.Run(tc.eventType, func(t *testing.T) {
			info := &relaycommon.RelayInfo{OriginModelName: "gpt-5.1", StreamStatus: relaycommon.NewStreamStatus()}
			accumulator := NewResponsesUsageAccumulator(info)
			for _, item := range []dto.ResponsesOutput{
				{Type: dto.BuildInCallWebSearchCall},
				{Type: dto.BuildInCallFileSearchCall},
				{Type: dto.BuildInCallFunctionCall, Name: "responses_priced_fn"},
				{Type: dto.BuildInCallFunctionCall, Name: "responses_unpriced_fn"},
			} {
				accumulator.Observe(&dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemDone, Item: &item}, nil)
			}
			image := dto.ResponsesOutput{ID: "image-1", Type: dto.ResponsesOutputTypeImageGenerationCall, Status: "completed", Result: "image-data"}
			accumulator.Observe(&dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemDone, Item: &image}, nil)
			upstream := &dto.Usage{InputTokens: 20, OutputTokens: 5, TotalTokens: 25, InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 4}}
			upstream.BillingUsage = dto.NewOpenAIResponsesBillingUsage(upstream)
			terminal := &dto.ResponsesStreamResponse{
				Type: tc.eventType,
				Response: &dto.OpenAIResponsesResponse{
					Usage: upstream, Output: []dto.ResponsesOutput{image},
				},
			}
			accumulator.Observe(terminal, nil)
			accumulator.Observe(terminal, nil)
			usage := accumulator.Finish()
			assert.Equal(t, tc.eventType == "response.failed", info.StreamStatus.ResponseFailed())
			assert.NotEmpty(t, info.StreamStatus.ResponseOutcome())

			assert.Equal(t, 20, usage.PromptTokens)
			assert.Equal(t, 5, usage.CompletionTokens)
			assert.Equal(t, 25, usage.TotalTokens)
			assert.Equal(t, 4, usage.PromptTokensDetails.CachedTokens)
			require.NotNil(t, usage.BillingUsage)
			assert.Equal(t, upstream.BillingUsage, usage.BillingUsage)
			assert.NotSame(t, upstream.BillingUsage, usage.BillingUsage)
			tools := info.ResponsesUsageInfo.BuiltInTools
			for _, name := range []string{dto.BuildInToolWebSearchPreview, dto.BuildInToolFileSearch, "responses_priced_fn"} {
				require.Contains(t, tools, name)
				assert.Equal(t, 1, tools[name].CallCount)
			}
			assert.NotContains(t, tools, "responses_unpriced_fn")
			require.Contains(t, tools, dto.BuildInToolImageGeneration)
			assert.Equal(t, tc.wantImages, tools[dto.BuildInToolImageGeneration].CallCount)
		})
	}
}

func TestResponsesUsageAccumulatorInterruptedTextFallback(t *testing.T) {
	for _, withUsage := range []bool{false, true} {
		name := "disconnect without terminal usage"
		if withUsage {
			name = "failed response preserves native billing usage"
		}
		t.Run(name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"}}
			info.SetEstimatePromptTokens(100)
			accumulator := NewResponsesUsageAccumulator(info)
			accumulator.Observe(&dto.ResponsesStreamResponse{Type: "response.output_text.delta", Delta: "hello"}, nil)
			if withUsage {
				upstream := &dto.Usage{InputTokens: 20, InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 4}}
				upstream.BillingUsage = dto.NewOpenAIResponsesBillingUsage(upstream)
				accumulator.Observe(&dto.ResponsesStreamResponse{Type: "response.failed", Response: &dto.OpenAIResponsesResponse{Usage: upstream}}, nil)
			}
			usage := accumulator.Finish()
			assert.Equal(t, 1, usage.CompletionTokens)
			if withUsage {
				assert.Equal(t, 20, usage.PromptTokens)
				assert.Equal(t, 21, usage.TotalTokens)
				require.NotNil(t, usage.BillingUsage)
				assert.Equal(t, dto.BillingUsageSourceOAIResponses, usage.BillingUsage.Source)
				assert.True(t, usage.BillingUsage.Estimated)
				canonical, ok := usage.BillingUsage.CanonicalUsage()
				require.True(t, ok)
				assert.Equal(t, 4, canonical.PromptTokensDetails.CachedTokens)
				assert.Equal(t, 1, canonical.CompletionTokens)
			} else {
				assert.Equal(t, 100, usage.PromptTokens)
				assert.Equal(t, 101, usage.TotalTokens)
			}
			accumulator.Observe(&dto.ResponsesStreamResponse{Type: "response.output_text.delta", Delta: " late output"}, nil)
			assert.Equal(t, usage, accumulator.Finish())
			assert.Equal(t, 1, usage.CompletionTokens)
		})
	}
}

func TestObserveResponsesOutcomeRecordsProtocolFacts(t *testing.T) {
	for _, tc := range []struct {
		name        string
		event       string
		wantOutcome relaycommon.ResponseOutcome
		wantCode    string
		wantType    string
		wantIncompl string
	}{
		{"flat sse error", `{"type":"error","code":"context_length_exceeded","message":"too long"}`, relaycommon.ResponseOutcomeFailed, "context_length_exceeded", "", ""},
		{"done with failed status", `{"type":"response.done","response":{"status":"failed","error":{"code":"invalid_api_key","type":"invalid_request_error","message":"bad key"}}}`, relaycommon.ResponseOutcomeFailed, "invalid_api_key", "invalid_request_error", ""},
		{"incomplete keeps reason", `{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`, relaycommon.ResponseOutcomeIncomplete, "", "", "max_output_tokens"},
		{"in progress is not terminal", `{"type":"response.created","response":{"status":"in_progress"}}`, relaycommon.ResponseOutcomeUnknown, "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var event dto.ResponsesStreamResponse
			require.NoError(t, common.UnmarshalJsonStr(tc.event, &event))
			info := &relaycommon.RelayInfo{StreamStatus: relaycommon.NewStreamStatus()}
			ObserveResponsesOutcome(info, &event)
			outcome := info.StreamStatus.OutcomeSnapshot()
			assert.Equal(t, tc.wantOutcome, outcome.Response)
			assert.Equal(t, tc.wantCode, outcome.ErrorCode)
			assert.Equal(t, tc.wantType, outcome.ErrorType)
			assert.Equal(t, tc.wantIncompl, outcome.IncompleteReason)
		})
	}
}

func TestResponsesUsageAccumulatorDisconnectBillsCompletedImage(t *testing.T) {
	info := &relaycommon.RelayInfo{OriginModelName: "gpt-5.1", StreamStatus: relaycommon.NewStreamStatus()}
	accumulator := NewResponsesUsageAccumulator(info)
	accumulator.Observe(&dto.ResponsesStreamResponse{
		Type: dto.ResponsesOutputTypeItemDone,
		Item: &dto.ResponsesOutput{ID: "complete-image", Type: dto.ResponsesOutputTypeImageGenerationCall, Status: "completed", Result: "final-image-data"},
	}, nil)
	accumulator.Observe(&dto.ResponsesStreamResponse{
		Type: dto.ResponsesOutputTypeItemDone,
		Item: &dto.ResponsesOutput{ID: "partial-image", Type: dto.ResponsesOutputTypeImageGenerationCall, Status: "partial", Result: "partial-image-data"},
	}, nil)
	usage := accumulator.Finish()
	assert.Zero(t, usage.TotalTokens)
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, dto.BuildInToolImageGeneration)
	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration].CallCount)
	accumulator.Finish()
	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration].CallCount)
}

func TestApplyResponsesUsageCopiesTokenDetails(t *testing.T) {
	dst := &dto.Usage{}
	src := &dto.Usage{
		InputTokens:  11,
		OutputTokens: 7,
		TotalTokens:  18,
		InputTokensDetails: &dto.InputTokenDetails{
			CachedTokens:         3,
			CachedCreationTokens: 2,
			TextTokens:           6,
			AudioTokens:          4,
			ImageTokens:          5,
		},
		OutputTokensDetails: &dto.OutputTokenDetails{
			TextTokens:      1,
			AudioTokens:     2,
			ImageTokens:     3,
			ReasoningTokens: 4,
		},
		PromptCacheHitTokens: 3,
		UsageSemantic:        "openai",
		UsageSource:          "upstream",
	}

	ApplyResponsesUsage(dst, src)

	assert.Equal(t, 11, dst.PromptTokens)
	assert.Equal(t, 7, dst.CompletionTokens)
	assert.Equal(t, 18, dst.TotalTokens)
	require.NotNil(t, dst.InputTokensDetails)
	assert.Equal(t, *src.InputTokensDetails, dst.PromptTokensDetails)
	assert.Equal(t, src.InputTokensDetails, dst.InputTokensDetails)
	assert.NotSame(t, src.InputTokensDetails, dst.InputTokensDetails)
	assert.Equal(t, *src.OutputTokensDetails, dst.CompletionTokenDetails)
	require.NotNil(t, dst.OutputTokensDetails)
	assert.Equal(t, *src.OutputTokensDetails, *dst.OutputTokensDetails)
	assert.NotSame(t, src.OutputTokensDetails, dst.OutputTokensDetails)
	assert.Equal(t, 3, dst.PromptCacheHitTokens)
	assert.Equal(t, "openai", dst.UsageSemantic)
	assert.Equal(t, "upstream", dst.UsageSource)
}

func TestApplyResponsesUsageFallsBackToCompletionTokenDetails(t *testing.T) {
	dst := &dto.Usage{}
	src := &dto.Usage{
		CompletionTokenDetails: dto.OutputTokenDetails{
			ReasoningTokens: 9,
		},
	}

	ApplyResponsesUsage(dst, src)

	assert.Equal(t, 9, dst.CompletionTokenDetails.ReasoningTokens)
	require.NotNil(t, dst.OutputTokensDetails)
	assert.Equal(t, 9, dst.OutputTokensDetails.ReasoningTokens)
}

func TestApplyResponsesUsagePreservesBillingSnapshotAcrossPartialUpdates(t *testing.T) {
	dst := &dto.Usage{}
	src := &dto.Usage{
		InputTokens:  20,
		OutputTokens: 10,
		TotalTokens:  30,
		InputTokensDetails: &dto.InputTokenDetails{
			CachedTokens: 3,
			AudioTokens:  2,
		},
		CompletionTokenDetails: dto.OutputTokenDetails{AudioTokens: 4},
		UsageSemantic:          "openai",
		UsageSource:            "upstream",
	}
	src.BillingUsage = dto.NewOpenAIResponsesBillingUsage(src)
	ApplyResponsesUsage(dst, src)
	ApplyResponsesUsage(dst, &dto.Usage{
		OutputTokens:        12,
		InputTokensDetails:  &dto.InputTokenDetails{},
		OutputTokensDetails: &dto.OutputTokenDetails{ReasoningTokens: 5},
	})

	assert.Equal(t, 20, dst.PromptTokens)
	assert.Equal(t, 12, dst.CompletionTokens)
	assert.Equal(t, 32, dst.TotalTokens)
	assert.Equal(t, 3, dst.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 2, dst.PromptTokensDetails.AudioTokens)
	assert.Equal(t, 4, dst.CompletionTokenDetails.AudioTokens)
	assert.Equal(t, 5, dst.CompletionTokenDetails.ReasoningTokens)
	require.NotNil(t, dst.OutputTokensDetails)
	assert.Equal(t, dst.CompletionTokenDetails, *dst.OutputTokensDetails)
	assert.Equal(t, "openai", dst.UsageSemantic)
	assert.Equal(t, "upstream", dst.UsageSource)
	require.NotNil(t, dst.BillingUsage)
	assert.Equal(t, src.BillingUsage, dst.BillingUsage)
	assert.NotSame(t, src.BillingUsage, dst.BillingUsage)
}

func TestResponsesUsageAccumulatorMissingUsageEstimation(t *testing.T) {
	const model = "gpt-4o"
	const summary = "Inspect the repository before editing."
	const arguments = `{"command":["bash","-lc","ls"]}`
	inProgress := &dto.OpenAIResponsesResponse{Status: []byte(`"in_progress"`)}
	for _, tc := range []struct {
		name           string
		events         []dto.ResponsesStreamResponse
		wantPrompt     int
		wantCompletion int
	}{
		{
			name: "tool call stream cut before terminal usage",
			events: []dto.ResponsesStreamResponse{
				{Type: "response.created", Response: inProgress},
				{Type: "response.reasoning_summary_text.delta", Delta: summary},
				{Type: "response.function_call_arguments.delta", Delta: arguments},
				{Type: dto.ResponsesOutputTypeItemDone, Item: &dto.ResponsesOutput{Type: dto.BuildInCallFunctionCall, Name: "shell"}},
			},
			wantPrompt:     100,
			wantCompletion: tokenkit.Count(model, summary+arguments),
		},
		{
			name: "created only then disconnect bills the prompt",
			events: []dto.ResponsesStreamResponse{
				{Type: "response.created", Response: inProgress},
			},
			wantPrompt: 100,
		},
		{
			name: "incomplete without usage bills the prompt",
			events: []dto.ResponsesStreamResponse{
				{Type: "response.created", Response: inProgress},
				{Type: "response.incomplete", Response: &dto.OpenAIResponsesResponse{Status: []byte(`"incomplete"`)}},
			},
			wantPrompt: 100,
		},
		{
			name: "completed without usage estimates from terminal output",
			events: []dto.ResponsesStreamResponse{
				{Type: "response.completed", Response: &dto.OpenAIResponsesResponse{
					Status: []byte(`"completed"`),
					Output: []dto.ResponsesOutput{{
						Type:    "message",
						Role:    "assistant",
						Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "final answer"}},
					}},
				}},
			},
			wantPrompt:     100,
			wantCompletion: tokenkit.Count(model, "final answer"),
		},
		{
			name: "explicit failure without usage bills nothing",
			events: []dto.ResponsesStreamResponse{
				{Type: "response.created", Response: inProgress},
				{Type: "response.failed", Response: &dto.OpenAIResponsesResponse{Status: []byte(`"failed"`)}},
			},
		},
		{
			name: "flat error event bills nothing",
			events: []dto.ResponsesStreamResponse{
				{Type: "response.created", Response: inProgress},
				{Type: "error", Code: "server_error"},
			},
		},
		{
			name: "no upstream events bills nothing",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				ChannelMeta:  &relaycommon.ChannelMeta{UpstreamModelName: model},
				StreamStatus: relaycommon.NewStreamStatus(),
			}
			info.SetEstimatePromptTokens(100)
			accumulator := NewResponsesUsageAccumulator(info)
			for i := range tc.events {
				accumulator.Observe(&tc.events[i], nil)
			}
			usage := accumulator.Finish()
			assert.Equal(t, tc.wantPrompt, usage.PromptTokens)
			assert.Equal(t, tc.wantCompletion, usage.CompletionTokens)
			assert.Equal(t, tc.wantPrompt+tc.wantCompletion, usage.TotalTokens)
		})
	}
}

func captureResponsesFailureLog(t *testing.T) (*gin.Context, *relaycommon.RelayInfo, *bytes.Buffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	buf := &bytes.Buffer{}
	oldWriter, oldDebug := gin.DefaultErrorWriter, common.DebugEnabled
	gin.DefaultErrorWriter = buf
	t.Cleanup(func() {
		gin.DefaultErrorWriter = oldWriter
		common.DebugEnabled = oldDebug
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 7}}
	return c, info, buf
}

func responsesFailureLines(buf *bytes.Buffer) []string {
	var lines []string
	for line := range strings.SplitSeq(buf.String(), "\n") {
		if strings.Contains(line, "upstream responses failure") {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestLogResponsesStreamFailure(t *testing.T) {
	longMessage := strings.Repeat("a", 5000)
	for _, tc := range []struct {
		name   string
		debug  bool
		events []string
		want   [][]string
	}{
		{
			name:   "response failed carries error object",
			events: []string{`{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`},
			want:   [][]string{{"upstream responses failure (channel #7, event: response.failed): code=server_error type=server_error message=boom"}},
		},
		{
			name:   "failed status on another event type",
			events: []string{`{"type":"response.done","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`},
			want:   [][]string{{"upstream responses failure (channel #7, event: response.done): code=server_error type=server_error message=boom"}},
		},
		{
			name:   "response.error event type",
			events: []string{`{"type":"response.error","code":"bad","message":"oops"}`},
			want:   [][]string{{"upstream responses failure (channel #7, event: response.error): code=bad type= message=oops"}},
		},
		{
			name:   "missing code leaves it empty",
			events: []string{`{"type":"response.failed","response":{"status":"failed","error":{"type":"server_error","message":"boom"}}}`},
			want:   [][]string{{"upstream responses failure (channel #7, event: response.failed): code= type=server_error message=boom"}},
		},
		{
			name:   "null response error and no top-level detail",
			events: []string{`{"type":"response.failed","response":{"status":"failed","error":null}}`},
			want:   [][]string{{"upstream responses failure (channel #7, event: response.failed): upstream gave no error detail"}},
		},
		{
			name:   "flat error then response failed gives two lines",
			events: []string{`{"type":"error","code":"rate_limit","message":"slow down"}`, `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`},
			want: [][]string{
				{"upstream responses failure (channel #7, event: error): code=rate_limit type= message=slow down"},
				{"upstream responses failure (channel #7, event: response.failed): code=server_error type=server_error message=boom"},
			},
		},
		{
			name:   "oversized message truncated at 2048 bytes",
			events: []string{`{"type":"error","code":"c","message":"` + longMessage + `"}`},
			want:   [][]string{{"message=" + strings.Repeat("a", 2048), "original_length=5000"}},
		},
		{
			name:   "debug keeps full message",
			debug:  true,
			events: []string{`{"type":"error","code":"c","message":"` + longMessage + `"}`},
			want:   [][]string{{"message=" + longMessage}},
		},
		{
			name:   "completed event logs nothing",
			events: []string{`{"type":"response.completed","response":{"status":"completed"}}`},
		},
		{
			name:   "delta event logs nothing",
			events: []string{`{"type":"response.output_text.delta","delta":"hi"}`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, info, buf := captureResponsesFailureLog(t)
			common.DebugEnabled = tc.debug
			for _, raw := range tc.events {
				var event dto.ResponsesStreamResponse
				require.NoError(t, common.UnmarshalJsonStr(raw, &event))
				LogResponsesStreamFailure(c, info, &event)
			}
			lines := responsesFailureLines(buf)
			require.Len(t, lines, len(tc.want))
			for i, wants := range tc.want {
				assert.True(t, strings.HasPrefix(lines[i], "[ERR]"))
				for _, want := range wants {
					assert.Contains(t, lines[i], want)
				}
			}
			if tc.name == "oversized message truncated at 2048 bytes" {
				assert.NotContains(t, lines[0], strings.Repeat("a", 2049))
			}
		})
	}
}

func TestLogResponsesStreamFailureRedactsSecretsAndUpstreamAddress(t *testing.T) {
	const secret = "abcdefghijklmnopqrstuvwxyz123456"
	const host = "api.upstream-secret.example.com"
	for _, debug := range []bool{false, true} {
		c, info, buf := captureResponsesFailureLog(t)
		common.DebugEnabled = debug
		common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://"+host)
		var event dto.ResponsesStreamResponse
		require.NoError(t, common.UnmarshalJsonStr(`{"type":"error","code":"invalid_api_key","message":"bad api_key:`+secret+` at https://`+host+`/v1/responses"}`, &event))
		LogResponsesStreamFailure(c, info, &event)
		lines := responsesFailureLines(buf)
		require.Len(t, lines, 1)
		assert.NotContains(t, lines[0], secret)
		assert.NotContains(t, lines[0], host)
		assert.Contains(t, lines[0], "code=invalid_api_key")
	}
}

func TestLogResponsesUpstreamError(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eventType string
		err       *types.OpenAIError
		want      string
	}{
		{"terminal error frame", "error", &types.OpenAIError{Code: "rate_limit", Type: "server_error", Message: "slow down"}, "upstream responses failure (channel #7, event: error): code=rate_limit type=server_error message=slow down"},
		{"missing type", "error", &types.OpenAIError{Code: "rate_limit", Message: "slow down"}, "upstream responses failure (channel #7, event: error): code=rate_limit type= message=slow down"},
		{"nil error object", "error", nil, "upstream responses failure (channel #7, event: error): upstream gave no error detail"},
		{"empty error object", "response.failed", &types.OpenAIError{}, "upstream responses failure (channel #7, event: response.failed): upstream gave no error detail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, info, buf := captureResponsesFailureLog(t)
			LogResponsesUpstreamError(c, info, tc.eventType, tc.err)
			lines := responsesFailureLines(buf)
			require.Len(t, lines, 1)
			assert.Contains(t, lines[0], tc.want)
		})
	}
}

func TestLogResponsesUpstreamErrorEscapesLineBreaks(t *testing.T) {
	c, info, buf := captureResponsesFailureLog(t)
	LogResponsesUpstreamError(c, info, "x\n[ERR] fake", &types.OpenAIError{Code: "c\nx", Type: "t\ry", Message: "line1\nline2\r\n[ERR] fake"})
	lines := responsesFailureLines(buf)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], `line1\nline2\r\n[ERR] fake`)
	assert.Contains(t, lines[0], `code=c\nx type=t\ry`)
	assert.Contains(t, lines[0], `event: x\n[ERR] fake)`)
	for line := range strings.SplitSeq(buf.String(), "\n") {
		assert.False(t, strings.HasPrefix(line, "[ERR] fake"), line)
	}
}
