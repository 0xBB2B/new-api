package openai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/tokenkit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The next upstream read stays blocked until the scanner closes its body.
// The channels let the downstream writer force cancellation during the final
// callback without depending on sleeps or goroutine scheduling.
type responsesLifecycleBody struct {
	data      *strings.Reader
	waiting   chan struct{}
	closed    chan struct{}
	readOnce  sync.Once
	closeOnce sync.Once
	readErr   error
	closeErr  error
}

func (b *responsesLifecycleBody) Read(p []byte) (int, error) {
	if b.data.Len() > 0 {
		return b.data.Read(p)
	}
	b.readOnce.Do(func() { close(b.waiting) })
	if b.readErr != nil {
		return 0, b.readErr
	}
	<-b.closed
	return 0, b.closeErr
}

func (b *responsesLifecycleBody) Close() error {
	b.closeOnce.Do(func() { close(b.closed) })
	return nil
}

type responsesLifecycleWriter struct {
	*httptest.ResponseRecorder
	body              *responsesLifecycleBody
	cancel            context.CancelFunc
	cancelBeforeFlush bool
	waitForCleanup    bool
	failWrite         int
	writeErr          error
	flushErr          error
	writes            int
	needle            string
}

func (w *responsesLifecycleWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failWrite {
		return 0, w.writeErr
	}
	n, err := w.ResponseRecorder.Write(p)
	if w.cancelBeforeFlush && strings.Contains(w.Body.String(), w.needle) {
		<-w.body.waiting
		w.cancel()
		<-w.body.closed
	}
	return n, err
}

func (w *responsesLifecycleWriter) Flush() {
	w.ResponseRecorder.Flush()
	if !strings.Contains(w.Body.String(), w.needle) || w.body == nil {
		return
	}
	<-w.body.waiting
	if w.cancel != nil {
		w.cancel()
	}
	if w.waitForCleanup {
		<-w.body.closed
	}
}

func (w *responsesLifecycleWriter) FlushError() error {
	return w.flushErr
}

func TestOaiResponsesStreamHandlerCompletionLifecycle(t *testing.T) {
	oldMode, oldTimeout := gin.Mode(), constant.StreamingTimeout
	gin.SetMode(gin.TestMode)
	constant.StreamingTimeout = 1
	t.Cleanup(func() {
		gin.SetMode(oldMode)
		constant.StreamingTimeout = oldTimeout
	})

	for _, tc := range []struct {
		name              string
		event             string
		responseStatus    string
		cancel            bool
		cancelBeforeFlush bool
		closeEOF          bool
		eof               bool
		scannerError      bool
		priorError        bool
		failWrite         int
		failFlush         bool
		wantEnd           relaycommon.StreamEndReason
		wantError         bool
	}{
		{name: "completed without upstream EOF", event: "response.completed", responseStatus: "completed", wantEnd: relaycommon.StreamEndReasonDone},
		{name: "done alias without upstream EOF", event: "response.done", responseStatus: "completed", wantEnd: relaycommon.StreamEndReasonDone},
		{name: "disconnect during terminal flush", event: "response.completed", responseStatus: "completed", cancel: true, wantEnd: relaycommon.StreamEndReasonDone},
		{name: "completion then cancellation and cleanup EOF", event: "response.completed", responseStatus: "completed", cancel: true, closeEOF: true, wantEnd: relaycommon.StreamEndReasonDone},
		{name: "midstream disconnect", event: "response.output_text.delta", cancel: true, wantEnd: relaycommon.StreamEndReasonClientGone, wantError: true},
		{name: "midstream disconnect followed by cleanup EOF", event: "response.output_text.delta", cancel: true, closeEOF: true, wantEnd: relaycommon.StreamEndReasonClientGone, wantError: true},
		{name: "cancellation before flush", event: "response.completed", responseStatus: "completed", cancel: true, cancelBeforeFlush: true, wantEnd: relaycommon.StreamEndReasonClientGone, wantError: true},
		{name: "event write failure", event: "response.completed", responseStatus: "completed", failWrite: 1, wantEnd: relaycommon.StreamEndReasonHandlerStop, wantError: true},
		{name: "data write failure", event: "response.completed", responseStatus: "completed", failWrite: 2, wantEnd: relaycommon.StreamEndReasonHandlerStop, wantError: true},
		{name: "delimiter write failure", event: "response.completed", responseStatus: "completed", failWrite: 3, wantEnd: relaycommon.StreamEndReasonHandlerStop, wantError: true},
		{name: "flush failure", event: "response.completed", responseStatus: "completed", failFlush: true, wantEnd: relaycommon.StreamEndReasonHandlerStop, wantError: true},
		{name: "protocol error before completion", event: "response.completed", responseStatus: "completed", priorError: true, cancel: true, wantEnd: relaycommon.StreamEndReasonClientGone, wantError: true},
		{name: "upstream read failure before handler returns", event: "response.completed", responseStatus: "completed", scannerError: true, wantEnd: relaycommon.StreamEndReasonScannerErr, wantError: true},
		{name: "done with failed response", event: "response.done", responseStatus: "failed", eof: true, wantEnd: relaycommon.StreamEndReasonEOF, wantError: true},
		{name: "done with incomplete response", event: "response.done", responseStatus: "incomplete", eof: true, wantEnd: relaycommon.StreamEndReasonEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"type":%q,"response":{"status":%q,"usage":{"input_tokens":7,"output_tokens":5,"total_tokens":12},"output":[{"id":"img_1","type":"image_generation_call","status":"completed","result":"image-data"}]}}`, tc.event, tc.responseStatus)
			if tc.responseStatus == "" {
				payload = `{"type":"response.output_text.delta","delta":"hello"}`
			}
			sse := "data: " + payload + "\n\n"
			if tc.priorError {
				sse = "data: invalid JSON\n\n" + sse
			}
			body := &responsesLifecycleBody{data: strings.NewReader(sse), waiting: make(chan struct{}), closed: make(chan struct{}), closeErr: io.ErrClosedPipe}
			if tc.closeEOF {
				body.closeErr = io.EOF
			}
			if tc.scannerError {
				body.readErr = io.ErrUnexpectedEOF
			}
			t.Cleanup(func() { require.NoError(t, body.Close()) })
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			writer := &responsesLifecycleWriter{
				ResponseRecorder: httptest.NewRecorder(), body: body, needle: payload + "\n\n",
				cancelBeforeFlush: tc.cancelBeforeFlush, waitForCleanup: tc.cancel || tc.scannerError,
				failWrite: tc.failWrite, writeErr: errors.New("downstream write failed"),
			}
			if tc.cancel {
				writer.cancel = cancel
			}
			if tc.failFlush {
				writer.flushErr = writer.writeErr
			}
			var upstream io.ReadCloser = body
			if tc.eof {
				upstream = io.NopCloser(strings.NewReader(sse))
				writer.body = nil
			}
			c, _ := gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
			info := &relaycommon.RelayInfo{
				OriginModelName: "gpt-4o", IsStream: true, DisablePing: true, StartTime: time.Now(),
				RelayFormat: types.RelayFormatOpenAIResponses,
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"},
			}
			info.SetEstimatePromptTokens(7)
			resp := &http.Response{StatusCode: http.StatusOK, Body: upstream, Header: http.Header{"Content-Type": {"text/event-stream"}}}
			usage, apiErr := OaiResponsesStreamHandler(c, info, resp)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			require.NotNil(t, info.StreamStatus)
			assert.Equal(t, tc.wantEnd, info.StreamStatus.EndReason)
			assert.Equal(t, tc.responseStatus, info.StreamStatus.ResponseOutcome())
			log := service.GenerateTextOtherInfo(c, info, 1, 1, 1, 0, 1, 0, 1).Snapshot()["stream_status"].(map[string]any)
			assert.Equal(t, tc.wantError, log["status"] == "error")
			if tc.responseStatus != "" {
				assert.Equal(t, 7, usage.PromptTokens)
				assert.Equal(t, 5, usage.CompletionTokens)
				assert.Equal(t, 12, usage.TotalTokens)
				require.NotNil(t, info.ResponsesUsageInfo)
				require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, dto.BuildInToolImageGeneration)
				assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration].CallCount)
			}
			if tc.failWrite == 0 {
				assert.Contains(t, writer.Body.String(), payload+"\n\n")
			} else {
				assert.ErrorIs(t, info.StreamStatus.EndError, writer.writeErr)
			}
			if tc.failFlush {
				assert.ErrorIs(t, info.StreamStatus.EndError, writer.flushErr)
			}
			if !tc.eof {
				select {
				case <-body.closed:
				default:
					t.Fatal("upstream body was not closed")
				}
			}
		})
	}
}

func TestOaiResponsesHandlerCountsOutputCallsNotDeclarations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	operation_setting.SetToolPriceForTest("priced_fn", 5.0)
	t.Cleanup(func() {
		operation_setting.DeleteToolPriceForTest("priced_fn")
	})

	body, err := common.Marshal(dto.OpenAIResponsesResponse{
		Tools: []map[string]any{
			{"type": "web_search_preview"},
			{"type": "file_search"},
		},
		Output: []dto.ResponsesOutput{
			{Type: dto.BuildInCallWebSearchCall},
			{Type: dto.BuildInCallWebSearchCall},
			{Type: dto.BuildInCallFunctionCall, Name: "priced_fn"},
			{Type: dto.BuildInCallFunctionCall, Name: "unpriced_fn"},
		},
		Usage: &dto.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.1",
		ResponsesUsageInfo: &relaycommon.ResponsesUsageInfo{
			BuiltInTools: map[string]*relaycommon.BuildInToolInfo{
				dto.BuildInToolWebSearchPreview: {ToolName: dto.BuildInToolWebSearchPreview, CallCount: 0},
				dto.BuildInToolFileSearch:       {ToolName: dto.BuildInToolFileSearch, CallCount: 0},
			},
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	usage, apiErr := OaiResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 2, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview].CallCount)
	assert.Equal(t, 0, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolFileSearch].CallCount)
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, "priced_fn")
	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools["priced_fn"].CallCount)
	assert.NotContains(t, info.ResponsesUsageInfo.BuiltInTools, "unpriced_fn")
}

func TestOaiResponsesHandlerDeclaredToolsWithoutOutputCountZero(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body, err := common.Marshal(dto.OpenAIResponsesResponse{
		Tools: []map[string]any{
			{"type": "web_search_preview"},
			{"type": "file_search"},
		},
		Output: []dto.ResponsesOutput{
			{Type: "message", Role: "assistant"},
		},
		Usage: &dto.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.1",
		ResponsesUsageInfo: &relaycommon.ResponsesUsageInfo{
			BuiltInTools: map[string]*relaycommon.BuildInToolInfo{
				dto.BuildInToolWebSearchPreview: {ToolName: dto.BuildInToolWebSearchPreview, CallCount: 0},
				dto.BuildInToolFileSearch:       {ToolName: dto.BuildInToolFileSearch, CallCount: 0},
			},
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	_, apiErr := OaiResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)
	assert.Equal(t, 0, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview].CallCount)
	assert.Equal(t, 0, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolFileSearch].CallCount)
}

func TestOaiResponsesHandlerCountsCompletedImageGenerationOutputs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body, err := common.Marshal(dto.OpenAIResponsesResponse{
		Status: []byte(`"completed"`),
		Output: []dto.ResponsesOutput{
			{
				Type:   dto.ResponsesOutputTypeImageGenerationCall,
				ID:     "img_1",
				Status: "completed",
				Result: "base64-a",
			},
			{
				Type:   dto.ResponsesOutputTypeImageGenerationCall,
				ID:     "img_2",
				Status: "completed",
				Result: "base64-b",
			},
			{
				Type:   dto.ResponsesOutputTypeImageGenerationCall,
				ID:     "img_empty",
				Status: "completed",
				Result: "",
			},
		},
		Usage: &dto.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{OriginModelName: "gpt-5.1"}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	_, apiErr := OaiResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, dto.BuildInToolImageGeneration)
	assert.Equal(t, 2, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration].CallCount)
	assert.False(t, c.GetBool("image_generation_call"))
}

func TestOaiResponsesHandlerCountsImageGenerationOnIncompleteStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body, err := common.Marshal(dto.OpenAIResponsesResponse{
		Status: []byte(`"incomplete"`),
		Output: []dto.ResponsesOutput{
			{
				Type:   dto.ResponsesOutputTypeImageGenerationCall,
				ID:     "img_1",
				Status: "completed",
				Result: "base64-a",
			},
		},
		Usage: &dto.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.1",
		ResponsesUsageInfo: &relaycommon.ResponsesUsageInfo{
			BuiltInTools: map[string]*relaycommon.BuildInToolInfo{
				dto.BuildInToolImageGeneration: {ToolName: dto.BuildInToolImageGeneration, CallCount: 0},
			},
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	_, apiErr := OaiResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)
	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration].CallCount)
}

func runResponsesImageBillingStream(t *testing.T, events ...string) *relaycommon.RelayInfo {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
	})

	var body strings.Builder
	for _, event := range events {
		body.WriteString("data: ")
		body.WriteString(event)
		body.WriteString("\n\n")
	}
	body.WriteString("data: [DONE]\n\n")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "responses-image-billing-test")
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.1",
		DisablePing:     true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.1",
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body.String())),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	_, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, info.ResponsesUsageInfo)
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, dto.BuildInToolImageGeneration)
	return info
}

func TestOaiResponsesStreamHandlerDeduplicatesCompletedImageOutput(t *testing.T) {
	item := `{"type":"image_generation_call","id":"img_1","call_id":"call_1","status":"completed","result":"base64-a"}`
	info := runResponsesImageBillingStream(
		t,
		`{"type":"response.output_item.done","output_index":0,"item":`+item+`}`,
		`{"type":"response.completed","response":{"status":"completed","output":[`+item+`],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
	)

	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration].CallCount)
}

func TestOaiResponsesStreamHandlerCountsImageOutputOnIncomplete(t *testing.T) {
	info := runResponsesImageBillingStream(
		t,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"image_generation_call","id":"img_1","status":"completed","result":"base64-a"}}`,
		`{"type":"response.incomplete","response":{"status":"incomplete"}}`,
	)

	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration].CallCount)
}

func TestOaiResponsesStreamHandlerDoesNotCountPartialImageEvent(t *testing.T) {
	info := runResponsesImageBillingStream(
		t,
		`{"type":"response.image_generation_call.partial_image","output_index":0,"partial_image_b64":"partial-bytes"}`,
		`{"type":"response.completed","response":{"status":"completed","output":[]}}`,
	)

	assert.Equal(t, 0, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration].CallCount)
}

// The 12h cache-write split reaches billing from JSON, stream and compaction
// responses. The usage is the OpenAI prompt caching guide example.
func TestOaiResponsesHandlersKeepCacheWriteTTLSplit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
	})

	const usage = `"usage":{"input_tokens":12000,"output_tokens":0,"total_tokens":12000,"input_tokens_details":{"cached_tokens":4000,"cache_write_tokens":7000,"cache_write_tokens_details":{"ttl_30m_tokens":3000,"ttl_12h_tokens":4000}}}`
	const response = `{"id":"resp_1","object":"response","model":"gpt-6-astra","status":"completed","output":[],` + usage + `}`
	for _, tt := range []struct {
		name, contentType, body string
	}{
		{"json", "application/json", response},
		{"stream", "text/event-stream", `data: {"type":"response.completed","response":` + response + "}\n\ndata: [DONE]\n\n"},
		{"compaction", "application/json", `{"id":"cmp_1","object":"response.compaction","output":[],` + usage + `}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			info := &relaycommon.RelayInfo{
				RelayFormat:     types.RelayFormatOpenAIResponses,
				OriginModelName: "gpt-6-astra",
				IsStream:        tt.name == "stream",
				DisablePing:     true,
				ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI, UpstreamModelName: "gpt-6-astra"},
			}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
				Header:     http.Header{"Content-Type": []string{tt.contentType}},
			}
			var usage *dto.Usage
			var apiErr *types.NewAPIError
			switch tt.name {
			case "json":
				usage, apiErr = OaiResponsesHandler(c, info, resp)
			case "stream":
				usage, apiErr = OaiResponsesStreamHandler(c, info, resp)
			default:
				usage, apiErr = OaiResponsesCompactionHandler(c, resp)
			}
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, 12000, usage.PromptTokens)
			assert.Equal(t, 4000, usage.PromptTokensDetails.CachedTokens)
			assert.Equal(t, 7000, usage.PromptTokensDetails.CacheCreationTokensTotal())
			assert.Equal(t, 4000, usage.PromptTokensDetails.CacheWrite12hTokens())
		})
	}
}

func TestOaiResponsesHandlerRewritesSGLangCreatedAtToInt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeSGLang},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_1","object":"response","created_at":1786588600.0,"status":"completed","output":[]}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	_, apiErr := OaiResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)
	assert.Contains(t, w.Body.String(), `"created_at":1786588600`)
	assert.NotContains(t, w.Body.String(), `1786588600.0`)
}

func TestOaiResponsesStreamHandlerRewritesSGLangCreatedAtToInt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "sglang-created-at-test")

	info := &relaycommon.RelayInfo{
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeSGLang,
			UpstreamModelName: "served-model",
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"created_at\":1.7865886E9,\"status\":\"completed\",\"output\":[]}}\n\n" +
				"data: [DONE]\n\n",
		)),
		Header: http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	_, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	assert.Contains(t, w.Body.String(), `"created_at":1786588600`)
	assert.NotContains(t, w.Body.String(), `1.7865886E9`)
	assert.NotContains(t, w.Body.String(), `1786588600.0`)
}

func TestOaiResponsesStreamHandlerKeepsNonSGLangCreatedAt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "openai-created-at-test")

	info := &relaycommon.RelayInfo{
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI, UpstreamModelName: "gpt-test"},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"created_at\":1786588600.0,\"status\":\"completed\",\"output\":[]}}\n\n" +
				"data: [DONE]\n\n",
		)),
		Header: http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	_, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	assert.Contains(t, w.Body.String(), `1786588600.0`)
}

// A converted hosted web search reaches Azure and Xinference Chat Completions
// as web_search_options, like OpenAI; the upstream decides whether it applies.
// Azure Responses bills web search by tool_usage.web_search.num_requests (Bing
// transactions) under bing_web_search, and the explicit zeros silence the
// web_search_call items, which leaves one surcharge row.
func TestAzureHostedWebSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
	})

	t.Run("Claude to Chat", func(t *testing.T) {
		const body = `{"model":"gpt-5.1","max_tokens":64,"messages":[{"role":"user","content":"Weather?"}],"tools":[{"type":"web_search_20250305","name":"web_search"}]}`
		for _, channelType := range []int{constant.ChannelTypeAzure, constant.ChannelTypeXinference, constant.ChannelTypeOpenAI} {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{
				RelayFormat: types.RelayFormatClaude,
				ChannelMeta: &relaycommon.ChannelMeta{ChannelType: channelType, UpstreamModelName: "gpt-5.1"},
			}
			adaptor := &Adaptor{}
			adaptor.Init(info)
			var request dto.ClaudeRequest
			require.NoError(t, common.UnmarshalJsonStr(body, &request))
			converted, err := adaptor.ConvertClaudeRequest(c, info, &request)
			require.NoError(t, err)
			wire, err := common.Marshal(converted)
			require.NoError(t, err)
			assert.True(t, strings.Contains(string(wire), `"web_search_options"`), channelType)
			var codes []string
			for _, diagnostic := range info.ConversionDiagnostics() {
				codes = append(codes, diagnostic.Code)
			}
			assert.False(t, slices.Contains(codes, "hosted_web_search_omitted"), channelType)
		}
	})

	const searchCall = `{"type":"web_search_call","status":"completed","action":{"type":"search","query":"go"}}`
	const usage = `"usage":{"input_tokens":10,"output_tokens":3,"total_tokens":13},"tool_usage":{"web_search":{"num_requests":5}}`
	output := strings.Repeat(searchCall+",", 3) + `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Go 1.25"}]}`
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{
			name:        "json",
			contentType: "application/json",
			body:        `{"id":"resp_1","object":"response","model":"gpt-5.1","status":"completed","output":[` + output + `],` + usage + `}`,
		},
		{
			name:        "stream",
			contentType: "text/event-stream",
			body: strings.Repeat(`data: {"type":"response.output_item.done","output_index":0,"item":`+searchCall+"}\n\n", 3) +
				`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.1","status":"completed","output":[` + output + `],` + usage + "}}\n\ndata: [DONE]\n\n",
		},
	}
	for _, tt := range tests {
		t.Run("Responses "+tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			info := &relaycommon.RelayInfo{
				RelayMode:       relayconstant.RelayModeResponses,
				RelayFormat:     types.RelayFormatOpenAIResponses,
				OriginModelName: "gpt-5.1",
				IsStream:        tt.name == "stream",
				DisablePing:     true,
				ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeAzure, UpstreamModelName: "gpt-5.1"},
			}
			adaptor := &Adaptor{}
			adaptor.Init(info)
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
				Header:     http.Header{"Content-Type": []string{tt.contentType}},
			}
			var apiErr *types.NewAPIError
			if info.IsStream {
				_, apiErr = OaiResponsesStreamHandler(c, info, resp)
			} else {
				_, apiErr = OaiResponsesHandler(c, info, resp)
			}
			require.Nil(t, apiErr)
			billed := map[string]int{}
			for name, tool := range info.ResponsesUsageInfo.BuiltInTools {
				if tool.CallCount > 0 {
					billed[name] = tool.CallCount
				}
			}
			assert.Equal(t, map[string]int{"bing_web_search": 5}, billed)
		})
	}
}

func newItemTagContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldSecret, oldTimeout := common.CryptoSecret, constant.StreamingTimeout
	common.CryptoSecret = "reasoning-tag-test-secret"
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		common.CryptoSecret = oldSecret
		constant.StreamingTimeout = oldTimeout
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "reasoning-tag-test")
	common.SetContextKey(c, constant.ContextKeyChannelId, 5)
	return c, w
}

func requireTaggedItemID(t *testing.T, tag, wantOriginal string) {
	t.Helper()
	channelID, original, ok := service.DecodeReasoningTag(tag)
	require.True(t, ok, "tag=%q", tag)
	assert.Equal(t, 5, channelID)
	if wantOriginal != "" {
		assert.Equal(t, wantOriginal, original)
	}
}

func reasoningIDsInSSE(t *testing.T, body string) []string {
	t.Helper()
	var ids []string
	collect := func(item any) {
		m, _ := item.(map[string]any)
		if m != nil && m["type"] == "reasoning" {
			ids = append(ids, m["id"].(string))
		}
	}
	for line := range strings.SplitSeq(body, "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "[DONE]" {
			continue
		}
		var event map[string]any
		require.NoError(t, common.Unmarshal([]byte(data), &event))
		collect(event["item"])
		if response, ok := event["response"].(map[string]any); ok {
			output, _ := response["output"].([]any)
			for _, item := range output {
				collect(item)
			}
		}
	}
	return ids
}

func TestOaiResponsesHandlerTagsItemIDs(t *testing.T) {
	c, w := newItemTagContext(t)
	upstream := `{"id":"resp_1","output":[{"type":"reasoning","id":"rs_y","summary":[]},{"type":"message","id":"msg_1","content":[]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(upstream)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	_, apiErr := OaiResponsesHandler(c, &relaycommon.RelayInfo{OriginModelName: "gpt-5.1"}, resp)
	require.Nil(t, apiErr)

	var got struct {
		Output []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"output"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Output, 2)
	requireTaggedItemID(t, got.Output[0].ID, "rs_y")
	requireTaggedItemID(t, got.Output[1].ID, "msg_1")
}

func TestOaiResponsesStreamHandlerTagsItemIDs(t *testing.T) {
	c, w := newItemTagContext(t)
	body := "data: " + `{"type":"response.output_item.added","item":{"type":"reasoning","id":"rs_y"}}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"id":"resp_1","output":[{"type":"reasoning","id":"rs_y"},{"type":"message","id":"msg_1"}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}` + "\n\n" +
		"data: [DONE]\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.1",
		DisablePing:     true,
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.1"},
	}

	_, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)

	ids := reasoningIDsInSSE(t, w.Body.String())
	require.Len(t, ids, 2)
	assert.Equal(t, ids[0], ids[1])
	requireTaggedItemID(t, ids[0], "rs_y")
	assert.NotContains(t, w.Body.String(), `"id":"msg_1"`)
	var msgTags []string
	for line := range strings.SplitSeq(w.Body.String(), "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "[DONE]" {
			continue
		}
		var event struct {
			Response struct {
				Output []struct {
					Type string `json:"type"`
					ID   string `json:"id"`
				} `json:"output"`
			} `json:"response"`
		}
		require.NoError(t, common.Unmarshal([]byte(data), &event))
		for _, item := range event.Response.Output {
			if item.Type == "message" {
				msgTags = append(msgTags, item.ID)
			}
		}
	}
	require.Len(t, msgTags, 1)
	requireTaggedItemID(t, msgTags[0], "msg_1")
}

func TestOaiResponsesStreamHandlerTagsNonReasoningItemID(t *testing.T) {
	c, w := newItemTagContext(t)
	body := "data: " + `{"type":"response.output_item.added","item":{"type":"custom_tool_call","id":"msg_aaa","call_id":"call_1"}}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"id":"resp_1","output":[{"type":"custom_tool_call","id":"msg_aaa","call_id":"call_1"}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}` + "\n\n" +
		"data: [DONE]\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.1",
		DisablePing:     true,
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.1"},
	}

	_, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)

	var ids []string
	for line := range strings.SplitSeq(w.Body.String(), "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "[DONE]" {
			continue
		}
		var event struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
			Response struct {
				Output []struct {
					ID string `json:"id"`
				} `json:"output"`
			} `json:"response"`
		}
		require.NoError(t, common.Unmarshal([]byte(data), &event))
		if event.Item.ID != "" {
			ids = append(ids, event.Item.ID)
		}
		for _, item := range event.Response.Output {
			ids = append(ids, item.ID)
		}
	}
	require.Len(t, ids, 2)
	assert.Equal(t, ids[0], ids[1])
	requireTaggedItemID(t, ids[0], "msg_aaa")
	assert.Contains(t, w.Body.String(), `"call_id":"call_1"`)
}

func TestOaiChatToResponsesStreamHandlerTagsReasoningID(t *testing.T) {
	c, recorder, resp, info := newResponsesChatTestContext(t, strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1710000000,"model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"thinking"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1710000000,"model":"gpt-test","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":"stop"}]}`,
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1710000000,"model":"gpt-test","choices":[],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`,
		`data: [DONE]`,
		``,
	}, "\n"), true)
	gin.SetMode(gin.TestMode)
	oldSecret, oldTimeout := common.CryptoSecret, constant.StreamingTimeout
	common.CryptoSecret = "reasoning-tag-test-secret"
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		common.CryptoSecret = oldSecret
		constant.StreamingTimeout = oldTimeout
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	common.SetContextKey(c, constant.ContextKeyChannelId, 5)

	_, apiErr := OaiChatToResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)

	ids := reasoningIDsInSSE(t, recorder.Body.String())
	require.NotEmpty(t, ids)
	for _, id := range ids {
		requireTaggedItemID(t, id, "")
	}
}

func TestOaiResponsesBufferedStreamHandlerTagsReasoningID(t *testing.T) {
	c, w := newItemTagContext(t)
	body := "data: " + `{"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","id":"rs_b","summary":[]}}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"id":"resp_1","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}` + "\n\n" +
		"data: [DONE]\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	_, apiErr := OaiResponsesBufferedStreamHandler(c, &relaycommon.RelayInfo{OriginModelName: "gpt-5.1"}, resp)
	require.Nil(t, apiErr)

	var got struct {
		Output []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"output"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Output, 1)
	requireTaggedItemID(t, got.Output[0].ID, "rs_b")
}

func TestOaiChatToResponsesHandlerTagsReasoningID(t *testing.T) {
	c, w := newItemTagContext(t)
	upstream := `{"id":"chatcmpl_1","object":"chat.completion","created":1710000000,"model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"hello","reasoning_content":"thinking"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(upstream)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
		RelayFormat: types.RelayFormatOpenAIResponses,
	}

	_, apiErr := OaiChatToResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)

	var got struct {
		Output []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"output"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &got))
	var reasoningIDs []string
	for _, item := range got.Output {
		if item.Type == "reasoning" {
			reasoningIDs = append(reasoningIDs, item.ID)
		}
	}
	require.NotEmpty(t, reasoningIDs, "body=%s", w.Body.String())
	for _, id := range reasoningIDs {
		requireTaggedItemID(t, id, "")
	}
}

func TestResponsesHandlersTagReasoningWithRetriedChannel(t *testing.T) {
	retriedContext := func(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
		c, w := newItemTagContext(t)
		common.SetContextKey(c, constant.ContextKeyChannelId, 3)
		common.SetContextKey(c, constant.ContextKeyChannelId, 5)
		return c, w
	}

	t.Run("non-stream", func(t *testing.T) {
		c, w := retriedContext(t)
		upstream := `{"id":"resp_1","output":[{"type":"reasoning","id":"rs_r","summary":[]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(upstream)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}

		_, apiErr := OaiResponsesHandler(c, &relaycommon.RelayInfo{OriginModelName: "gpt-5.1"}, resp)
		require.Nil(t, apiErr)

		var got struct {
			Output []struct {
				ID string `json:"id"`
			} `json:"output"`
		}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &got))
		require.Len(t, got.Output, 1)
		requireTaggedItemID(t, got.Output[0].ID, "rs_r")
	})

	t.Run("stream", func(t *testing.T) {
		c, w := retriedContext(t)
		body := "data: " + `{"type":"response.completed","response":{"id":"resp_1","output":[{"type":"reasoning","id":"rs_r"}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}` + "\n\n" +
			"data: [DONE]\n\n"
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		}
		info := &relaycommon.RelayInfo{
			OriginModelName: "gpt-5.1",
			DisablePing:     true,
			ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.1"},
		}

		_, apiErr := OaiResponsesStreamHandler(c, info, resp)
		require.Nil(t, apiErr)

		ids := reasoningIDsInSSE(t, w.Body.String())
		require.NotEmpty(t, ids)
		for _, id := range ids {
			requireTaggedItemID(t, id, "rs_r")
		}
	})
}

func nonReasoningItemsInSSE(t *testing.T, body string) []map[string]any {
	t.Helper()
	var items []map[string]any
	collect := func(item any) {
		m, _ := item.(map[string]any)
		if m != nil && (m["type"] == "message" || m["type"] == "function_call") {
			items = append(items, m)
		}
	}
	for line := range strings.SplitSeq(body, "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "[DONE]" {
			continue
		}
		var event map[string]any
		require.NoError(t, common.Unmarshal([]byte(data), &event))
		collect(event["item"])
		if response, ok := event["response"].(map[string]any); ok {
			output, _ := response["output"].([]any)
			for _, item := range output {
				collect(item)
			}
		}
	}
	return items
}

func requireChatToolItemsTagged(t *testing.T, items []map[string]any) {
	t.Helper()
	seen := map[any]bool{}
	for _, item := range items {
		seen[item["type"]] = true
		id, _ := item["id"].(string)
		requireTaggedItemID(t, id, "")
		if item["type"] == "function_call" {
			callID, _ := item["call_id"].(string)
			_, _, isTag := service.DecodeReasoningTag(callID)
			assert.False(t, isTag, "call_id=%q", callID)
			assert.Equal(t, "call_1", callID)
		}
	}
	assert.True(t, seen["message"], "items=%v", items)
	assert.True(t, seen["function_call"], "items=%v", items)
}

func TestOaiChatToResponsesHandlerTagsMessageAndFunctionCallID(t *testing.T) {
	c, w := newItemTagContext(t)
	upstream := `{"id":"chatcmpl_1","object":"chat.completion","created":1710000000,"model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"hello","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(upstream)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
		RelayFormat: types.RelayFormatOpenAIResponses,
	}

	_, apiErr := OaiChatToResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)

	var got struct {
		Output []map[string]any `json:"output"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &got))
	requireChatToolItemsTagged(t, got.Output)
}

func TestOaiChatToResponsesStreamHandlerTagsMessageAndFunctionCallID(t *testing.T) {
	c, recorder, resp, info := newResponsesChatTestContext(t, strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1710000000,"model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1710000000,"model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1710000000,"model":"gpt-test","choices":[],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`,
		`data: [DONE]`,
		``,
	}, "\n"), true)
	gin.SetMode(gin.TestMode)
	oldSecret, oldTimeout := common.CryptoSecret, constant.StreamingTimeout
	common.CryptoSecret = "reasoning-tag-test-secret"
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		common.CryptoSecret = oldSecret
		constant.StreamingTimeout = oldTimeout
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	common.SetContextKey(c, constant.ContextKeyChannelId, 5)

	_, apiErr := OaiChatToResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)

	requireChatToolItemsTagged(t, nonReasoningItemsInSSE(t, recorder.Body.String()))
}

// The stream scanner logs from its own goroutine in debug mode, so the
// captured log writer must be safe for concurrent writes.
type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func runResponsesFailureStream(t *testing.T, handler func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError), relayFormat types.RelayFormat, events ...string) (logLines []string, clientBody string, usage *dto.Usage) {
	t.Helper()
	logLines, clientBody, usage, _, _ = runResponsesFailureStreamFull(t, handler, relayFormat, events...)
	return logLines, clientBody, usage
}

func runResponsesFailureStreamFull(t *testing.T, handler func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError), relayFormat types.RelayFormat, events ...string) (logLines []string, clientBody string, usage *dto.Usage, apiErr *types.NewAPIError, info *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldTimeout, oldWriter := constant.StreamingTimeout, gin.DefaultErrorWriter
	logBuf := &lockedLogBuffer{}
	constant.StreamingTimeout = 30
	gin.DefaultErrorWriter = logBuf
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
		gin.DefaultErrorWriter = oldWriter
	})

	var body strings.Builder
	for _, event := range events {
		body.WriteString("data: " + event + "\n\n")
	}
	body.WriteString("data: [DONE]\n\n")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "responses-failure-log-test")
	info = &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.1",
		IsStream:        true,
		DisablePing:     true,
		StartTime:       time.Now(),
		RelayFormat:     relayFormat,
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 7, UpstreamModelName: "gpt-5.1"},
	}
	info.SetEstimatePromptTokens(100)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body.String())),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	usage, apiErr = handler(c, info, resp)
	for line := range strings.SplitSeq(logBuf.String(), "\n") {
		if strings.Contains(line, "upstream responses failure") {
			logLines = append(logLines, line)
		}
	}
	return logLines, w.Body.String(), usage, apiErr, info
}

func TestResponsesStreamHandlersLogUpstreamFailure(t *testing.T) {
	const failed = `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`
	const failedNoDetail = `{"type":"response.failed","response":{"status":"failed","error":null}}`
	const flatError = `{"type":"error","code":"rate_limit","message":"slow down"}`
	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	const delta = `{"type":"response.output_text.delta","delta":"hi"}`
	const completed = `{"type":"response.completed","response":{"id":"resp_1","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`
	for _, handler := range []struct {
		name   string
		run    func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError)
		format types.RelayFormat
	}{
		{"direct forward", OaiResponsesStreamHandler, types.RelayFormatOpenAIResponses},
		{"responses to chat", OaiResponsesToChatStreamHandler, types.RelayFormatOpenAI},
	} {
		for _, tc := range []struct {
			name   string
			events []string
			want   [][]string
		}{
			{name: "response failed", events: []string{created, failed}, want: [][]string{{"upstream responses failure (channel #7, event: response.failed): code=server_error type=server_error message=boom", "responses-failure-log-test"}}},
			{name: "no error detail", events: []string{created, failedNoDetail}, want: [][]string{{"upstream responses failure (channel #7, event: response.failed): upstream gave no error detail"}}},
			{name: "flat error then response failed", events: []string{created, flatError, failed}, want: [][]string{
				{"event: error): code=rate_limit", "message=slow down"},
				{"event: response.failed): code=server_error"},
			}},
			{name: "non-failure events log nothing", events: []string{created, delta, completed}},
		} {
			t.Run(handler.name+"/"+tc.name, func(t *testing.T) {
				lines, _, _ := runResponsesFailureStream(t, handler.run, handler.format, tc.events...)
				require.Len(t, lines, len(tc.want))
				for i, wants := range tc.want {
					for _, want := range wants {
						assert.Contains(t, lines[i], want)
					}
				}
			})
		}
	}
}

func TestOaiResponsesStreamHandlerFailureLogKeepsClientBody(t *testing.T) {
	const failed = `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`
	lines, body, _ := runResponsesFailureStream(t, OaiResponsesStreamHandler, types.RelayFormatOpenAIResponses, failed)
	require.Len(t, lines, 1)
	assert.Contains(t, body, failed+"\n\n")
}

func TestOaiResponsesToChatStreamHandlerFailureLogKeepsClientBody(t *testing.T) {
	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	const delta = `{"type":"response.output_text.delta","delta":"hi"}`
	const failed = `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`
	lines, body, _ := runResponsesFailureStream(t, OaiResponsesToChatStreamHandler, types.RelayFormatOpenAI, created, delta, failed)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "upstream responses failure (channel #7, event: response.failed): code=server_error type=server_error message=boom")

	chunks := strings.Split(body, "\n\n")
	require.Len(t, chunks, 3)
	assert.True(t, strings.HasPrefix(chunks[0], `data: {"id":"chatcmpl-responses-failure-log-test","object":"chat.completion.chunk"`))
	assert.Contains(t, chunks[0], `"delta":{"content":"","role":"assistant"}`)
	assert.Contains(t, chunks[1], `"delta":{"content":"hi"}`)
	assert.Equal(t, `{"error":{"message":"boom","type":"server_error","param":"","code":"server_error"}}`, chunks[2])
	assert.NotContains(t, body, "[DONE]")
	assert.NotContains(t, body, "response.failed")
}

func TestResponsesStreamHandlersFailureSettlesUpstreamUsage(t *testing.T) {
	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	const delta = `{"type":"response.output_text.delta","delta":"hi"}`
	const failedWithUsage = `{"type":"response.failed","response":{"status":"failed","usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10},"error":{"code":"server_error","type":"server_error","message":"boom"}}}`
	const failedNoUsage = `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`
	for _, handler := range []struct {
		name            string
		run             func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError)
		format          types.RelayFormat
		wantNoOutputErr bool
	}{
		{"direct forward", OaiResponsesStreamHandler, types.RelayFormatOpenAIResponses, false},
		{"responses to chat", OaiResponsesToChatStreamHandler, types.RelayFormatOpenAI, true},
	} {
		t.Run(handler.name+"/failure event carries usage", func(t *testing.T) {
			lines, _, usage, apiErr, info := runResponsesFailureStreamFull(t, handler.run, handler.format, created, delta, failedWithUsage)
			require.Len(t, lines, 1)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, 7, usage.PromptTokens)
			assert.Equal(t, 3, usage.CompletionTokens)
			assert.Equal(t, 10, usage.TotalTokens)
			require.NotNil(t, info.StreamStatus)
			assert.Equal(t, "failed", info.StreamStatus.ResponseOutcome())
		})
		t.Run(handler.name+"/failure without any output", func(t *testing.T) {
			lines, _, usage, apiErr, info := runResponsesFailureStreamFull(t, handler.run, handler.format, created, failedNoUsage)
			require.Len(t, lines, 1)
			if handler.wantNoOutputErr {
				require.NotNil(t, apiErr)
				assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
				relayErr, ok := apiErr.RelayError.(types.OpenAIError)
				require.True(t, ok)
				assert.Equal(t, "boom", relayErr.Message)
				assert.Equal(t, "server_error", relayErr.Type)
				assert.Equal(t, "server_error", relayErr.Code)
				assert.Nil(t, usage)
			} else {
				require.Nil(t, apiErr)
				require.NotNil(t, usage)
				assert.Equal(t, 0, usage.PromptTokens)
				assert.Equal(t, 0, usage.CompletionTokens)
				assert.Equal(t, 0, usage.TotalTokens)
			}
			require.NotNil(t, info.StreamStatus)
			assert.Equal(t, "failed", info.StreamStatus.ResponseOutcome())
		})
		t.Run(handler.name+"/failure event without usage", func(t *testing.T) {
			lines, _, usage, apiErr, info := runResponsesFailureStreamFull(t, handler.run, handler.format, created, delta, failedNoUsage)
			require.Len(t, lines, 1)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			wantCompletion := tokenkit.Count("gpt-5.1", "hi")
			assert.Equal(t, 100, usage.PromptTokens)
			assert.Equal(t, wantCompletion, usage.CompletionTokens)
			assert.Equal(t, 100+wantCompletion, usage.TotalTokens)
			require.NotNil(t, info.StreamStatus)
			assert.Equal(t, "failed", info.StreamStatus.ResponseOutcome())
		})
	}
}

type streamRun struct {
	usage  *dto.Usage
	body   string
	apiErr *types.NewAPIError
	sent   bool
	info   *relaycommon.RelayInfo
	ctx    *gin.Context
	w      *httptest.ResponseRecorder
}

type streamHandler = func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError)

func failoverProbe(result bool) func(*types.NewAPIError) bool {
	return func(*types.NewAPIError) bool { return result }
}

// probe 为 nil 表示不设置探针。
type probeMode struct {
	name  string
	probe func(*types.NewAPIError) bool
}

var probeModes = []probeMode{
	{"probe true", failoverProbe(true)},
	{"probe false", failoverProbe(false)},
	{"no probe", nil},
}

func runRawStream(t *testing.T, handler streamHandler, relayFormat types.RelayFormat, path, upstream string, probe func(*types.NewAPIError) bool) streamRun {
	t.Helper()
	return runRawStreamTuned(t, handler, relayFormat, path, upstream, probe, nil)
}

func runRawStreamTuned(t *testing.T, handler streamHandler, relayFormat types.RelayFormat, path, upstream string, probe func(*types.NewAPIError) bool, tune func(*relaycommon.RelayInfo)) streamRun {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	c.Set(common.RequestIdKey, "pre-body-hold-test")
	if probe != nil {
		helper.SetFailoverProbe(c, probe)
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.1",
		IsStream:        true,
		DisablePing:     true,
		StartTime:       time.Now(),
		RelayFormat:     relayFormat,
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 7, UpstreamModelName: "gpt-5.1"},
	}
	if relayFormat == types.RelayFormatClaude {
		info.ClaudeConvertInfo = &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone}
	}
	info.SetEstimatePromptTokens(100)
	if tune != nil {
		tune(info)
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(upstream)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	usage, apiErr := handler(c, info, resp)
	return streamRun{usage: usage, body: w.Body.String(), apiErr: apiErr, sent: common.GetContextKeyBool(c, constant.ContextKeyStreamOutputSent), info: info, ctx: c, w: w}
}

func responsesSSE(events ...string) string {
	var b strings.Builder
	for _, event := range events {
		b.WriteString("data: " + event + "\n\n")
	}
	return b.String()
}

func responsesClientBytes(events ...string) string {
	var b strings.Builder
	for _, event := range events {
		b.WriteString("event: " + gjsonType(event) + "\ndata: " + event + "\n\n")
	}
	return b.String()
}

func gjsonType(event string) string {
	var e struct {
		Type string `json:"type"`
	}
	_ = common.UnmarshalJsonStr(event, &e)
	return e.Type
}

func TestOaiResponsesStreamHandlerHoldsOpeningEventsUntilBody(t *testing.T) {
	const (
		created    = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
		inProgress = `{"type":"response.in_progress","response":{"id":"resp_1","status":"in_progress"}}`
		queued     = `{"type":"response.queued","response":{"id":"resp_1","status":"queued"}}`
		delta      = `{"type":"response.output_text.delta","delta":"hi"}`
		completed  = `{"type":"response.completed","response":{"id":"resp_1","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`
		incomplete = `{"type":"response.incomplete","response":{"id":"resp_1","status":"incomplete"}}`
		failed     = `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`
		doneFailed = `{"type":"response.done","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`
		respError  = `{"type":"response.error","code":"server_error","message":"boom"}`
		flatError  = `{"type":"error","code":"rate_limit","message":"slow down"}`
	)
	for _, tc := range []struct {
		name       string
		events     []string
		wantClient []string // 正文已开始或没有失败时，所有探针模式下一致
		failover   bool     // 正文前失败：探针为真时换渠道，否则与改动前一致
		wantErr    int      // 探针为真时返回的状态码
		oldClient  []string // 探针为假或未设置时客户端收到的事件
	}{
		{name: "failed after created", events: []string{created, failed}, failover: true, wantErr: http.StatusInternalServerError, oldClient: []string{created, failed}},
		{name: "failed after all opening events", events: []string{created, inProgress, queued, failed}, failover: true, wantErr: http.StatusInternalServerError, oldClient: []string{created, inProgress, queued, failed}},
		{name: "error event after created", events: []string{created, flatError}, failover: true, wantErr: http.StatusInternalServerError, oldClient: []string{created, flatError}},
		{name: "response.error after created", events: []string{created, respError}, failover: true, wantErr: http.StatusInternalServerError, oldClient: []string{created, respError}},
		{name: "done with failed status first", events: []string{doneFailed}, failover: true, wantErr: http.StatusInternalServerError, oldClient: []string{doneFailed}},
		{name: "created then EOF", events: []string{created}, failover: true, wantErr: http.StatusBadGateway, oldClient: []string{created}},
		{name: "no event then EOF", failover: true, wantErr: http.StatusBadGateway},
		{name: "body then completed", events: []string{created, delta, completed}, wantClient: []string{created, delta, completed}},
		{name: "opening events flushed in order before body", events: []string{created, inProgress, queued, delta}, wantClient: []string{created, inProgress, queued, delta}},
		{name: "completed right after created", events: []string{created, completed}, wantClient: []string{created, completed}},
		{name: "incomplete right after created", events: []string{created, incomplete}, wantClient: []string{created, incomplete}},
		{name: "failure after body", events: []string{created, delta, failed}, wantClient: []string{created, delta, failed}},
	} {
		for _, mode := range probeModes {
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				run := runRawStream(t, OaiResponsesStreamHandler, types.RelayFormatOpenAIResponses, "/v1/responses", responsesSSE(tc.events...), mode.probe)
				if !tc.failover {
					assert.Equal(t, responsesClientBytes(tc.wantClient...), run.body)
					assert.True(t, run.sent)
					assert.Nil(t, run.apiErr)
					return
				}
				if mode.name == "probe true" {
					assert.Empty(t, run.body)
					assert.False(t, run.sent)
					require.NotNil(t, run.apiErr)
					assert.Equal(t, tc.wantErr, run.apiErr.StatusCode)
					assert.False(t, types.IsSkipRetryError(run.apiErr))
					return
				}
				assert.Equal(t, responsesClientBytes(tc.oldClient...), run.body)
				assert.Equal(t, len(tc.oldClient) > 0, run.sent)
				assert.Nil(t, run.apiErr)
			})
		}
	}
}

func TestOaiResponsesStreamHandlerPreBodyFailureLeavesNoToolCounts(t *testing.T) {
	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	const failed = `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"},"output":[{"id":"img_1","type":"image_generation_call","status":"completed","result":"image-data"}]}}`
	run := runRawStream(t, OaiResponsesStreamHandler, types.RelayFormatOpenAIResponses, "/v1/responses", responsesSSE(created, failed), failoverProbe(true))
	require.NotNil(t, run.apiErr)
	if run.info.ResponsesUsageInfo != nil {
		assert.Zero(t, run.info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolImageGeneration].CallCount)
	}
}

type pingObservingWriter struct {
	*httptest.ResponseRecorder
	mu       sync.Mutex
	pinged   chan struct{}
	pingOnce sync.Once
}

func (w *pingObservingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.ResponseRecorder.Write(p)
	if strings.Contains(w.Body.String(), ": PING") {
		w.pingOnce.Do(func() { close(w.pinged) })
	}
	return n, err
}

func (w *pingObservingWriter) snapshot() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Body.String()
}

func TestOaiResponsesStreamHandlerKeepAliveDuringHoldSendsOnlyPing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	general := operation_setting.GetGeneralSetting()
	oldEnabled, oldSeconds := general.PingIntervalEnabled, general.PingIntervalSeconds
	general.PingIntervalEnabled, general.PingIntervalSeconds = true, 1
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
		general.PingIntervalEnabled, general.PingIntervalSeconds = oldEnabled, oldSeconds
	})

	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	pr, pw := io.Pipe()
	writer := &pingObservingWriter{ResponseRecorder: httptest.NewRecorder(), pinged: make(chan struct{})}
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	helper.SetFailoverProbe(c, failoverProbe(true))
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.1", IsStream: true, StartTime: time.Now(),
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.1"},
	}
	var duringHold string
	var sentDuringHold bool
	go func() {
		defer pw.Close()
		_, _ = io.WriteString(pw, responsesSSE(created))
		select {
		case <-writer.pinged:
		case <-time.After(10 * time.Second):
		}
		duringHold = writer.snapshot()
		sentDuringHold = common.GetContextKeyBool(c, constant.ContextKeyStreamOutputSent)
	}()
	resp := &http.Response{StatusCode: http.StatusOK, Body: pr, Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
	_, apiErr := OaiResponsesStreamHandler(c, info, resp)

	assert.NotEmpty(t, duringHold)
	assert.Empty(t, strings.ReplaceAll(duringHold, ": PING\n\n", ""))
	assert.False(t, sentDuringHold)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
}

const (
	chatRole     = `{"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-5.1","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`
	chatText     = `{"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-5.1","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`
	chatUsage    = `{"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-5.1","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`
	chatFinish   = `{"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-5.1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`
	chatTool     = `{"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-5.1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":""}}]},"finish_reason":null}]}`
	chatErrChunk = `{"error":{"message":"boom","type":"server_error"}}`
	chatOnePiece = `{"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-5.1","choices":[{"index":0,"delta":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`
)

func TestOaiStreamHandlerHoldsRoleChunkUntilBody(t *testing.T) {
	for _, tc := range []struct {
		name      string
		upstream  string
		failover  bool
		wantErr   int
		oldOrder  []string // 探针为假或未设置时，客户端按序收到的内容
		wantOrder []string
	}{
		{name: "error chunk after role", upstream: responsesSSE(chatRole, chatErrChunk), failover: true, wantErr: http.StatusInternalServerError, oldOrder: []string{`"role":"assistant"`, `"message":"boom"`}},
		{name: "role then EOF", upstream: responsesSSE(chatRole), failover: true, wantErr: http.StatusBadGateway, oldOrder: []string{`"role":"assistant"`}},
		{name: "no chunk then EOF", failover: true, wantErr: http.StatusBadGateway, oldOrder: []string{"[DONE]"}},
		{name: "text then DONE", upstream: responsesSSE(chatRole, chatText, "[DONE]"), wantOrder: []string{`"role":"assistant"`, `"content":"hi"`, "[DONE]"}},
		{name: "DONE right after role", upstream: responsesSSE(chatRole, "[DONE]"), wantOrder: []string{`"role":"assistant"`, "[DONE]"}},
		{name: "error chunk after text", upstream: responsesSSE(chatRole, chatText, chatErrChunk), wantOrder: []string{`"role":"assistant"`, `"content":"hi"`, `"message":"boom"`}},
	} {
		for _, mode := range probeModes {
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				run := runRawStream(t, OaiStreamHandler, types.RelayFormatOpenAI, "/v1/chat/completions", tc.upstream, mode.probe)
				if tc.failover && mode.name == "probe true" {
					assert.Empty(t, run.body)
					assert.False(t, run.sent)
					require.NotNil(t, run.apiErr)
					assert.Equal(t, tc.wantErr, run.apiErr.StatusCode)
					assert.False(t, types.IsSkipRetryError(run.apiErr))
					return
				}
				assert.Nil(t, run.apiErr)
				want := tc.wantOrder
				if tc.failover {
					want = tc.oldOrder
				}
				assert.Equal(t, len(want) > 0, run.sent)
				if len(want) == 0 {
					assert.Empty(t, run.body)
					return
				}
				requireOrderedSubstrings(t, run.body, want...)
			})
		}
	}
}

func TestOaiStreamHandlerBodyStartKindsEndHold(t *testing.T) {
	for _, tc := range []struct {
		name  string
		chunk string
		want  string
	}{
		{name: "usage only", chunk: chatUsage, want: `"usage"`},
		{name: "finish_reason only", chunk: chatFinish, want: `"finish_reason":"stop"`},
		{name: "tool_calls only", chunk: chatTool, want: `"name":"lookup"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := runRawStream(t, OaiStreamHandler, types.RelayFormatOpenAI, "/v1/chat/completions", responsesSSE(chatRole, tc.chunk, chatErrChunk), failoverProbe(true))
			assert.True(t, run.sent)
			assert.Nil(t, run.apiErr)
			requireOrderedSubstrings(t, run.body, `"role":"assistant"`, tc.want, `"message":"boom"`)
		})
	}
}

func TestOaiStreamHandlerClaudeAndGeminiClientsEndingWithoutDone(t *testing.T) {
	for _, tc := range []struct {
		name     string
		format   types.RelayFormat
		path     string
		upstream string
		want     []string
	}{
		{name: "claude one chunk", format: types.RelayFormatClaude, path: "/v1/messages", upstream: responsesSSE(chatOnePiece), want: []string{`"text":"hi"`, `"type":"message_stop"`}},
		{name: "claude role then text", format: types.RelayFormatClaude, path: "/v1/messages", upstream: responsesSSE(chatRole, chatText), want: []string{`"type":"message_start"`, `"text":"hi"`}},
		{name: "gemini one chunk", format: types.RelayFormatGemini, path: "/v1beta/models/gpt-5.1:streamGenerateContent", upstream: responsesSSE(chatOnePiece), want: []string{`"candidates"`, `"text":"hi"`}},
		{name: "gemini role then text", format: types.RelayFormatGemini, path: "/v1beta/models/gpt-5.1:streamGenerateContent", upstream: responsesSSE(chatRole, chatText), want: []string{`"candidates"`, `"text":"hi"`}},
	} {
		for _, mode := range probeModes {
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				run := runRawStream(t, OaiStreamHandler, tc.format, tc.path, tc.upstream, mode.probe)
				assert.Nil(t, run.apiErr)
				assert.True(t, run.sent)
				requireOrderedSubstrings(t, run.body, tc.want...)
			})
		}
	}
}

func TestStreamWritesAfterHeldFailureReachClient(t *testing.T) {
	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	const failed = `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`
	for _, tc := range []struct {
		name     string
		handler  streamHandler
		format   types.RelayFormat
		upstream string
	}{
		{"responses", OaiResponsesStreamHandler, types.RelayFormatOpenAIResponses, responsesSSE(created, failed)},
		{"chat", OaiStreamHandler, types.RelayFormatOpenAI, responsesSSE(chatRole, chatErrChunk)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := runRawStream(t, tc.handler, tc.format, "/v1/chat/completions", tc.upstream, failoverProbe(true))
			require.NotNil(t, run.apiErr)
			require.Empty(t, run.body)

			require.NoError(t, helper.ObjectData(run.ctx, map[string]any{"error": map[string]any{"message": "final", "type": "server_error"}}))
			helper.Done(run.ctx)

			assert.Equal(t, `data: {"error":{"message":"final","type":"server_error"}}`+"\n\n"+"data: [DONE]\n\n", run.w.Body.String())
			assert.True(t, common.GetContextKeyBool(run.ctx, constant.ContextKeyStreamOutputSent))
		})
	}
}

func TestOaiResponsesToChatStreamHandlerPreBodyFailure(t *testing.T) {
	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	const failed = `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`
	for _, mode := range probeModes {
		t.Run(mode.name, func(t *testing.T) {
			run := runRawStream(t, OaiResponsesToChatStreamHandler, types.RelayFormatOpenAI, "/v1/chat/completions", responsesSSE(created, failed), mode.probe)
			if mode.name == "probe true" {
				assert.Empty(t, run.body)
				assert.False(t, run.sent)
				require.NotNil(t, run.apiErr)
				assert.False(t, types.IsSkipRetryError(run.apiErr))
				return
			}
			assert.True(t, run.sent)
			assert.Contains(t, run.body, `"role":"assistant"`)
			require.NotNil(t, run.apiErr)
		})
	}
}

func TestFailoverProbeReceivesReturnedError(t *testing.T) {
	for _, tc := range []struct {
		name     string
		upstream string
	}{
		{"chat upstream invalid JSON first chunk", "data: {not json\n\n"},
		{"chat upstream error first chunk", responsesSSE(chatErrChunk)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var seen *types.NewAPIError
			run := runRawStream(t, OaiChatToResponsesStreamHandler, types.RelayFormatOpenAIResponses, "/v1/responses", tc.upstream, func(err *types.NewAPIError) bool {
				seen = err
				return true
			})
			assert.Empty(t, run.body)
			require.NotNil(t, run.apiErr)
			require.NotNil(t, seen)
			assert.Equal(t, run.apiErr.GetErrorCode(), seen.GetErrorCode())
			assert.Equal(t, run.apiErr.StatusCode, seen.StatusCode)
		})
	}
}

func TestFailoverProbeRealRuleKeepsInvalidJSONBehavior(t *testing.T) {
	var seen *types.NewAPIError
	run := runRawStream(t, OaiChatToResponsesStreamHandler, types.RelayFormatOpenAIResponses, "/v1/responses", "data: {not json\n\n", func(err *types.NewAPIError) bool {
		seen = err
		return !operation_setting.IsAlwaysSkipRetryCode(err.GetErrorCode())
	})

	require.NotNil(t, seen)
	require.NotNil(t, run.apiErr)
	assert.Equal(t, run.apiErr.GetErrorCode(), seen.GetErrorCode())
	assert.Equal(t, run.apiErr.StatusCode, seen.StatusCode)
	assert.True(t, run.sent)
	assert.Contains(t, run.body, "event: response.failed")
	assert.True(t, types.IsSkipRetryError(run.apiErr))
}

func TestConvertedStreamsFailOverOnOpeningOnlyEOF(t *testing.T) {
	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	for _, tc := range []struct {
		name             string
		handler          streamHandler
		format           types.RelayFormat
		path             string
		upstream         string
		noFailoverMarker string // 探针为假或未设置时，客户端收到的内容
	}{
		{name: "responses upstream to chat, created only", handler: OaiResponsesToChatStreamHandler, format: types.RelayFormatOpenAI, path: "/v1/chat/completions", upstream: responsesSSE(created), noFailoverMarker: `"role":"assistant"`},
		{name: "responses upstream to chat, no event", handler: OaiResponsesToChatStreamHandler, format: types.RelayFormatOpenAI, path: "/v1/chat/completions", noFailoverMarker: `"role":"assistant"`},
		{name: "responses upstream to claude, created only", handler: OaiResponsesToChatStreamHandler, format: types.RelayFormatClaude, path: "/v1/messages", upstream: responsesSSE(created), noFailoverMarker: `"type":"message_start"`},
		{name: "responses upstream to gemini, created only", handler: OaiResponsesToChatStreamHandler, format: types.RelayFormatGemini, path: "/v1beta/models/gpt-5.1:streamGenerateContent", upstream: responsesSSE(created), noFailoverMarker: `"candidates"`},
		{name: "chat upstream to responses, role only", handler: OaiChatToResponsesStreamHandler, format: types.RelayFormatOpenAIResponses, path: "/v1/responses", upstream: responsesSSE(chatRole), noFailoverMarker: "event: response.created"},
		{name: "chat upstream to responses, no chunk", handler: OaiChatToResponsesStreamHandler, format: types.RelayFormatOpenAIResponses, path: "/v1/responses", noFailoverMarker: "event: response.completed"},
	} {
		for _, mode := range probeModes {
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				run := runRawStream(t, tc.handler, tc.format, tc.path, tc.upstream, mode.probe)
				if mode.name == "probe true" {
					assert.Empty(t, run.body)
					assert.False(t, run.sent)
					require.NotNil(t, run.apiErr)
					assert.False(t, types.IsSkipRetryError(run.apiErr))
					return
				}
				assert.Nil(t, run.apiErr)
				assert.True(t, run.sent)
				assert.Contains(t, run.body, tc.noFailoverMarker)
			})
		}
	}
}

func TestChatToResponsesStreamUsageBeforeFailureIsNotSettled(t *testing.T) {
	run := runRawStream(t, OaiChatToResponsesStreamHandler, types.RelayFormatOpenAIResponses, "/v1/responses", responsesSSE(chatUsage, chatErrChunk), failoverProbe(true))

	assert.Empty(t, run.body)
	assert.False(t, run.sent)
	require.NotNil(t, run.apiErr)
	assert.Nil(t, run.usage, "a failed attempt that will be replaced must not return usage to settle")
}

func TestStreamFailureStatusFromStringCodeKeepsNonRetryableOutcome(t *testing.T) {
	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	failedWith := func(errorJSON string) string {
		return `{"type":"response.failed","response":{"status":"failed","error":` + errorJSON + `}}`
	}
	for _, tc := range []struct {
		name       string
		handler    streamHandler
		format     types.RelayFormat
		path       string
		events     []string
		wantStatus int
	}{
		{name: "responses failed with context_length_exceeded code", handler: OaiResponsesStreamHandler, format: types.RelayFormatOpenAIResponses, path: "/v1/responses",
			events: []string{created, failedWith(`{"code":"context_length_exceeded","message":"too long"}`)}, wantStatus: http.StatusBadRequest},
		{name: "responses failed with invalid_request_error type", handler: OaiResponsesStreamHandler, format: types.RelayFormatOpenAIResponses, path: "/v1/responses",
			events: []string{created, failedWith(`{"code":"bad_param","type":"invalid_request_error","message":"bad"}`)}, wantStatus: http.StatusBadRequest},
		{name: "responses flat error with context_length_exceeded code", handler: OaiResponsesStreamHandler, format: types.RelayFormatOpenAIResponses, path: "/v1/responses",
			events: []string{created, `{"type":"error","code":"context_length_exceeded","message":"too long"}`}, wantStatus: http.StatusBadRequest},
		{name: "responses failed with server_error code stays retryable", handler: OaiResponsesStreamHandler, format: types.RelayFormatOpenAIResponses, path: "/v1/responses",
			events: []string{created, failedWith(`{"code":"server_error","type":"server_error","message":"boom"}`)}, wantStatus: http.StatusInternalServerError},
		{name: "chat error chunk with context_length_exceeded code", handler: OaiStreamHandler, format: types.RelayFormatOpenAI, path: "/v1/chat/completions",
			events: []string{chatRole, `{"error":{"message":"too long","type":"invalid_request_error","code":"context_length_exceeded"}}`}, wantStatus: http.StatusBadRequest},
		{name: "chat error chunk with server_error type stays retryable", handler: OaiStreamHandler, format: types.RelayFormatOpenAI, path: "/v1/chat/completions",
			events: []string{chatRole, chatErrChunk}, wantStatus: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decider, _ := gin.CreateTestContext(httptest.NewRecorder())
			var seen *types.NewAPIError
			probe := func(err *types.NewAPIError) bool {
				seen = err
				return service.DecideRelayRetry(decider, err, 1).Action == "retry"
			}

			run := runRawStream(t, tc.handler, tc.format, tc.path, responsesSSE(tc.events...), probe)

			require.NotNil(t, seen)
			assert.Equal(t, tc.wantStatus, seen.StatusCode)
			if tc.wantStatus == http.StatusInternalServerError {
				assert.Empty(t, run.body)
				require.NotNil(t, run.apiErr)
				return
			}
			assert.True(t, run.sent)
			assert.Nil(t, run.apiErr)
			for _, event := range tc.events {
				assert.Contains(t, run.body, event[:len(event)-1], "the client receives the original events")
			}
		})
	}
}

func TestChatToResponsesStreamFailureMessageIgnoresMetadata(t *testing.T) {
	const withMetadata = `{"error":{"message":"boom","type":"server_error","metadata":{"raw":"secret-detail"}}}`
	for _, mode := range []probeMode{{"probe false", failoverProbe(false)}, {"no probe", nil}} {
		t.Run(mode.name, func(t *testing.T) {
			run := runRawStream(t, OaiChatToResponsesStreamHandler, types.RelayFormatOpenAIResponses, "/v1/responses", responsesSSE(withMetadata), mode.probe)

			require.NotNil(t, run.apiErr)
			assert.Equal(t, "boom", run.apiErr.Error())
			assert.NotContains(t, run.body, "secret-detail")
			failed := ""
			for _, block := range strings.Split(run.body, "\n\n") {
				if strings.HasPrefix(block, "event: response.failed\n") {
					failed = strings.TrimPrefix(strings.SplitN(block, "\n", 2)[1], "data: ")
				}
			}
			require.NotEmpty(t, failed, run.body)
			var event struct {
				Response struct {
					Error map[string]any `json:"error"`
				} `json:"response"`
			}
			require.NoError(t, common.UnmarshalJsonStr(failed, &event))
			assert.Equal(t, map[string]any{"code": "server_error", "message": "boom"}, event.Response.Error)
		})
	}
}

func TestOaiStreamHandlerFailsOverWithFormatRewriteEnabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		tune func(*relaycommon.RelayInfo)
	}{
		{name: "neither", tune: nil},
		{name: "force_format", tune: func(info *relaycommon.RelayInfo) { info.ChannelSetting.ForceFormat = true }},
		{name: "thinking_to_content", tune: func(info *relaycommon.RelayInfo) { info.ChannelSetting.ThinkingToContent = true }},
		{name: "both", tune: func(info *relaycommon.RelayInfo) {
			info.ChannelSetting.ForceFormat = true
			info.ChannelSetting.ThinkingToContent = true
		}},
	} {
		for _, upstream := range []struct{ name, body string }{
			{"error chunk first", responsesSSE(chatErrChunk)},
			{"error chunk after role", responsesSSE(chatRole, chatErrChunk)},
			{"error chunk then DONE", responsesSSE(chatErrChunk, "[DONE]")},
			{"role then EOF", responsesSSE(chatRole)},
		} {
			t.Run(tc.name+"/"+upstream.name, func(t *testing.T) {
				run := runRawStreamTuned(t, OaiStreamHandler, types.RelayFormatOpenAI, "/v1/chat/completions", upstream.body, failoverProbe(true), tc.tune)

				assert.Empty(t, run.body)
				assert.False(t, run.sent)
				require.NotNil(t, run.apiErr)
				assert.False(t, types.IsSkipRetryError(run.apiErr))
			})
		}
	}
}

func TestResponsesStreamFailureReturnsWithoutWaitingForUpstreamClose(t *testing.T) {
	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	const failed = `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"boom"}}}`
	for _, tc := range []struct {
		name    string
		handler streamHandler
		format  types.RelayFormat
	}{
		{"responses client", OaiResponsesStreamHandler, types.RelayFormatOpenAIResponses},
		{"chat client", OaiResponsesToChatStreamHandler, types.RelayFormatOpenAI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			oldTimeout := constant.StreamingTimeout
			constant.StreamingTimeout = 30
			t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
			pr, pw := io.Pipe()
			t.Cleanup(func() { _ = pw.Close() })
			go func() { _, _ = io.WriteString(pw, responsesSSE(created, failed)) }()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			helper.SetFailoverProbe(c, failoverProbe(true))
			info := &relaycommon.RelayInfo{
				OriginModelName: "gpt-5.1", IsStream: true, DisablePing: true, StartTime: time.Now(), RelayFormat: tc.format,
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.1"},
			}
			info.SetEstimatePromptTokens(100)
			resp := &http.Response{StatusCode: http.StatusOK, Body: pr, Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
			returned := make(chan *types.NewAPIError, 1)
			go func() {
				_, apiErr := tc.handler(c, info, resp)
				returned <- apiErr
			}()

			select {
			case apiErr := <-returned:
				require.NotNil(t, apiErr)
				assert.Empty(t, w.Body.String())
			case <-time.After(2 * time.Second):
				require.FailNow(t, "the handler kept waiting for the upstream to close the connection")
			}
		})
	}
}

func TestConvertedStreamFailureStatusFollowsErrorType(t *testing.T) {
	const created = `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`
	failedWith := func(errType string) string {
		return `{"type":"response.failed","response":{"status":"failed","error":{"type":"` + errType + `","message":"upstream said no"}}}`
	}
	chatError := func(errType string) string {
		return `{"error":{"message":"upstream said no","type":"` + errType + `"}}`
	}
	for _, tc := range []struct {
		name        string
		handler     streamHandler
		format      types.RelayFormat
		path        string
		events      []string
		wantStatus  int
		wantMarker  string // 不换渠道时客户端收到的内容
		errReturned bool   // 不换渠道时错误由 handler 返回，不写进响应体
	}{
		{"responses to chat invalid_request_error", OaiResponsesToChatStreamHandler, types.RelayFormatOpenAI, "/v1/chat/completions", []string{created, failedWith("invalid_request_error")}, http.StatusBadRequest, `"role":"assistant"`, true},
		{"responses to chat server_error", OaiResponsesToChatStreamHandler, types.RelayFormatOpenAI, "/v1/chat/completions", []string{created, failedWith("server_error")}, http.StatusInternalServerError, "", false},
		{"chat to responses invalid_request_error", OaiChatToResponsesStreamHandler, types.RelayFormatOpenAIResponses, "/v1/responses", []string{chatError("invalid_request_error")}, http.StatusBadRequest, "event: response.failed", false},
		{"chat to responses server_error", OaiChatToResponsesStreamHandler, types.RelayFormatOpenAIResponses, "/v1/responses", []string{chatError("server_error")}, http.StatusInternalServerError, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decider, _ := gin.CreateTestContext(httptest.NewRecorder())
			var seen *types.NewAPIError
			probe := func(err *types.NewAPIError) bool {
				seen = err
				return service.DecideRelayRetry(decider, err, 1).Action == "retry"
			}

			run := runRawStream(t, tc.handler, tc.format, tc.path, responsesSSE(tc.events...), probe)

			require.NotNil(t, seen)
			assert.Equal(t, tc.wantStatus, seen.StatusCode)
			if tc.wantMarker == "" {
				assert.Empty(t, run.body)
				assert.False(t, run.sent)
				require.NotNil(t, run.apiErr)
				return
			}
			assert.True(t, run.sent)
			assert.Contains(t, run.body, tc.wantMarker)
			if tc.errReturned {
				require.NotNil(t, run.apiErr)
				assert.Equal(t, tc.wantStatus, run.apiErr.StatusCode)
				assert.Contains(t, run.apiErr.Error(), "upstream said no")
				return
			}
			assert.Contains(t, run.body, "upstream said no")
		})
	}
}

func TestAggregatedResponsesFailureStatusFollowsErrorType(t *testing.T) {
	failedWith := func(errorJSON string) string {
		return responsesSSE(`{"type":"response.failed","response":{"status":"failed","error":` + errorJSON + `}}`)
	}
	for _, handler := range []struct {
		name string
		run  streamHandler
		path string
	}{
		{"responses buffered", OaiResponsesBufferedStreamHandler, "/v1/responses"},
		{"chat client buffered", OaiResponsesToChatBufferedStreamHandler, "/v1/chat/completions"},
	} {
		for _, tc := range []struct {
			name       string
			errorJSON  string
			wantStatus int
			wantRetry  bool
		}{
			{"invalid_request_error type", `{"code":"bad_param","type":"invalid_request_error","message":"upstream said no"}`, http.StatusBadRequest, false},
			{"context_length_exceeded code", `{"code":"context_length_exceeded","message":"upstream said no"}`, http.StatusBadRequest, false},
			{"server_error type", `{"code":"server_error","type":"server_error","message":"upstream said no"}`, http.StatusInternalServerError, true},
		} {
			t.Run(handler.name+"/"+tc.name, func(t *testing.T) {
				decider, _ := gin.CreateTestContext(httptest.NewRecorder())

				run := runRawStream(t, handler.run, types.RelayFormatOpenAI, handler.path, failedWith(tc.errorJSON), nil)

				require.NotNil(t, run.apiErr)
				assert.Contains(t, run.apiErr.Error(), "upstream said no")
				assert.Equal(t, tc.wantStatus, run.apiErr.StatusCode)
				assert.Equal(t, tc.wantRetry, service.DecideRelayRetry(decider, run.apiErr, 1).Action == "retry")
			})
		}
	}
}
