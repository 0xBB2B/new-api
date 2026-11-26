package controller

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayKeepAliveDuringImagePreparation(t *testing.T) {
	settings := operation_setting.GetGeneralSetting()
	previousEnabled, previousInterval := settings.PingIntervalEnabled, settings.PingIntervalSeconds
	previousCount, previousMedia := constant.CountToken, constant.GetMediaToken
	fetchSettings := system_setting.GetFetchSetting()
	previousFetchSettings := *fetchSettings
	fetchSettings.AllowPrivateIp = true
	fetchSettings.AllowedPorts = []string{"1-65535"}
	settings.PingIntervalEnabled, settings.PingIntervalSeconds = true, 1
	constant.CountToken, constant.GetMediaToken = true, true
	t.Cleanup(func() {
		settings.PingIntervalEnabled, settings.PingIntervalSeconds = previousEnabled, previousInterval
		constant.CountToken, constant.GetMediaToken = previousCount, previousMedia
		*fetchSettings = previousFetchSettings
	})
	service.InitHttpClient()

	imageRequested := make(chan struct{})
	releaseImage := make(chan struct{})
	var releaseOnce sync.Once
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(imageRequested)
		<-releaseImage
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer imageServer.Close()
	engine := gin.New()
	engine.POST("/v1/messages", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-sonnet-4-20250514")
		Relay(c, types.RelayFormatClaude)
	})
	gateway := httptest.NewServer(engine)
	defer gateway.Close()
	defer releaseOnce.Do(func() { close(releaseImage) })

	body := `{"model":"claude-sonnet-4-20250514","max_tokens":16,"stream":true,"messages":[{"role":"user","content":[{"type":"image","source":{"type":"url","url":"` + imageServer.URL + `"}}]}]}`
	client := &http.Client{Timeout: 4 * time.Second}
	response, err := client.Post(gateway.URL+"/v1/messages", "application/json", strings.NewReader(body))
	require.NoError(t, err, "the gateway must send keepalive while its image download is blocked")
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "text/event-stream", response.Header.Get("Content-Type"))
	reader := bufio.NewReader(response.Body)
	for range 2 {
		line, readErr := reader.ReadString('\n')
		require.NoError(t, readErr)
		require.Equal(t, ": PING\n", line)
		line, readErr = reader.ReadString('\n')
		require.NoError(t, readErr)
		require.Equal(t, "\n", line)
	}
	select {
	case <-imageRequested:
	default:
		require.FailNow(t, "the request never reached image preparation")
	}
	releaseOnce.Do(func() { close(releaseImage) })
	remaining, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Contains(t, string(remaining), "event: error\n")
	require.Contains(t, string(remaining), `"type":"error"`)
	require.Contains(t, string(remaining), "data: ")
}

func TestRelayKeepAliveAcrossUpstreamRetries(t *testing.T) {
	settings := operation_setting.GetGeneralSetting()
	previousEnabled, previousInterval := settings.PingIntervalEnabled, settings.PingIntervalSeconds
	settings.PingIntervalEnabled, settings.PingIntervalSeconds = true, 1
	t.Cleanup(func() {
		settings.PingIntervalEnabled, settings.PingIntervalSeconds = previousEnabled, previousInterval
	})
	service.InitHttpClient()
	var attempts atomic.Int32
	retryStarted, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		close(retryStarted)
		select {
		case <-release:
			_, _ = io.WriteString(w, "data: ok\n\n")
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()
	engine := gin.New()
	engine.POST("/v1/messages", func(c *gin.Context) {
		stop := helper.StartStreamKeepAlive(c, 10*time.Millisecond)
		defer stop()
		if !assert.NoError(t, http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(5*time.Second))) {
			return
		}
		info := &relaycommon.RelayInfo{IsStream: true, ChannelMeta: &relaycommon.ChannelMeta{}}
		for range 2 {
			request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, upstream.URL, strings.NewReader("{}"))
			if !assert.NoError(t, err) {
				return
			}
			response, err := channel.DoRequest(c, request, info)
			if !assert.NoError(t, err) {
				return
			}
			if response.StatusCode == http.StatusTooManyRequests {
				response.Body.Close()
				continue
			}
			_, _ = io.Copy(c.Writer, response.Body)
			response.Body.Close()
		}
	})
	gateway := httptest.NewServer(engine)
	defer gateway.Close()
	defer releaseOnce.Do(func() { close(release) })
	client := &http.Client{Timeout: 4 * time.Second}
	response, err := client.Post(gateway.URL+"/v1/messages", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	defer response.Body.Close()
	select {
	case <-retryStarted:
	case <-time.After(time.Second):
		require.FailNow(t, "the upstream retry did not start")
	}
	reader := bufio.NewReader(response.Body)
	for range 2 {
		line, readErr := reader.ReadString('\n')
		require.NoError(t, readErr)
		require.Equal(t, ": PING\n", line)
		line, readErr = reader.ReadString('\n')
		require.NoError(t, readErr)
		require.Equal(t, "\n", line)
	}
	releaseOnce.Do(func() { close(release) })
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Contains(t, string(body), "data: ok\n\n")
	require.Equal(t, int32(2), attempts.Load())
}

func TestRelayKeepAlivePreservesValidationErrors(t *testing.T) {
	settings := operation_setting.GetGeneralSetting()
	previousEnabled := settings.PingIntervalEnabled
	settings.PingIntervalEnabled = true
	t.Cleanup(func() { settings.PingIntervalEnabled = previousEnabled })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-20250514","stream":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	Relay(c, types.RelayFormatClaude)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
	require.NotContains(t, recorder.Body.String(), ": PING")
}

func TestStreamKeepAlivePreservesFastHTTPFailures(t *testing.T) {
	settings := operation_setting.GetGeneralSetting()
	previousEnabled, previousInterval := settings.PingIntervalEnabled, settings.PingIntervalSeconds
	settings.PingIntervalEnabled, settings.PingIntervalSeconds = true, 60
	t.Cleanup(func() {
		settings.PingIntervalEnabled, settings.PingIntervalSeconds = previousEnabled, previousInterval
	})
	service.InitHttpClient()
	for _, status := range []int{http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"error":"request rejected"}`)
			}))
			defer upstream.Close()
			engine := gin.New()
			engine.POST("/v1/messages", func(c *gin.Context) {
				stop := helper.StartStreamKeepAlive(c, time.Minute)
				defer stop()
				request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, upstream.URL, strings.NewReader("{}"))
				if !assert.NoError(t, err) {
					return
				}
				info := &relaycommon.RelayInfo{IsStream: true, ChannelMeta: &relaycommon.ChannelMeta{}}
				response, err := channel.DoRequest(c, request, info)
				if !assert.NoError(t, err) {
					return
				}
				defer response.Body.Close()
				body, err := io.ReadAll(response.Body)
				if assert.NoError(t, err) {
					c.Header("Content-Type", "application/json")
					c.Data(response.StatusCode, "application/json", body)
				}
			})
			gateway := httptest.NewServer(engine)
			defer gateway.Close()
			client := &http.Client{Timeout: 4 * time.Second}
			response, err := client.Post(gateway.URL+"/v1/messages", "application/json", strings.NewReader("{}"))
			require.NoError(t, err)
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, status, response.StatusCode)
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))
			require.JSONEq(t, `{"error":"request rejected"}`, string(body))
		})
	}
}

func TestRelayKeepAliveHonorsStreamSettings(t *testing.T) {
	settings := operation_setting.GetGeneralSetting()
	previousEnabled := settings.PingIntervalEnabled
	previousCount, previousMedia, previousNonStreamMedia := constant.CountToken, constant.GetMediaToken, constant.GetMediaTokenNotStream
	constant.CountToken, constant.GetMediaToken, constant.GetMediaTokenNotStream = true, true, true
	t.Cleanup(func() {
		settings.PingIntervalEnabled = previousEnabled
		constant.CountToken, constant.GetMediaToken, constant.GetMediaTokenNotStream = previousCount, previousMedia, previousNonStreamMedia
	})
	for _, test := range []struct {
		name    string
		enabled bool
		stream  bool
	}{
		{name: "ping disabled", enabled: false, stream: true},
		{name: "non-streaming", enabled: true, stream: false},
		{name: "fast streaming failure", enabled: true, stream: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings.PingIntervalEnabled = test.enabled
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			body := `{"model":"claude-sonnet-4-20250514","max_tokens":16,"stream":` + strconv.FormatBool(test.stream) + `,"messages":[{"role":"user","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"!"}}]}]}`
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-sonnet-4-20250514")
			Relay(c, types.RelayFormatClaude)
			require.Equal(t, http.StatusInternalServerError, recorder.Code)
			require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
			require.NotContains(t, recorder.Body.String(), ": PING")
			var response map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, "error", response["type"])
		})
	}
}

func TestRelayStreamErrorAfterKeepAlive(t *testing.T) {
	for _, format := range []types.RelayFormat{types.RelayFormatClaude, types.RelayFormatOpenAI} {
		t.Run(string(format), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/stream", nil)
			helper.SetEventStreamHeaders(c)
			require.NoError(t, helper.PingData(c))
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://provider.example")
			common.SetContextKey(c, constant.ContextKeyUpstreamRequestOrigin, "https://provider.example")
			apiError := types.NewErrorWithStatusCode(errors.New("upstream rejected input at https://provider.example/v1/messages"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
			writeRelayStreamError(c, format, apiError)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
			require.Contains(t, recorder.Body.String(), "upstream rejected input")
			require.NotContains(t, recorder.Body.String(), "provider.example")
			status, ok := common.GetContextKeyType[*relaycommon.StreamStatus](c, constant.ContextKeyResponseStreamStatus)
			require.True(t, ok)
			require.True(t, status.ResponseFailed(), "stream errors must not consume the successful-request rate limit")
			require.Equal(t, http.StatusBadRequest, status.OutcomeSnapshot().ErrorStatus)
			if format == types.RelayFormatClaude {
				require.Contains(t, recorder.Body.String(), "event: error\n")
			} else {
				require.Contains(t, recorder.Body.String(), "data: [DONE]")
			}
		})
	}
}

func TestStreamKeepAliveCancellation(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/stream", nil).WithContext(requestContext)
	stop := helper.StartStreamKeepAlive(c, time.Hour)
	cancel()
	stopped := make(chan struct{})
	go func() {
		stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		require.FailNow(t, "keepalive did not stop after client cancellation")
	}
	_, err := c.Writer.WriteString("data: done\n\n")
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(recorder.Body.String(), "data: done\n\n"))
}

func TestRelayStreamErrorAfterOutputDoesNotRetry(t *testing.T) {
	claudeFirst := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-x\",\"content\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"A\"}}\n\n" +
		"event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"x\"}}\n\n"
	claudeSecond := strings.Replace(claudeFirst, "\"text\":\"A\"", "\"text\":\"B\"", 1)
	claudeSecond = claudeSecond[:strings.Index(claudeSecond, "event: error")] +
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	responsesFirst := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"status\":\"in_progress\"}}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"A\"}\n\n" +
		"data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_1\",\"status\":\"failed\",\"error\":{\"code\":\"server_error\",\"message\":\"x\"}}}\n\n"
	responsesSecond := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_2\",\"status\":\"in_progress\"}}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"B\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_2\",\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"

	for _, tc := range []struct {
		name, path, first, second, body, startMarker string
	}{
		{"claude overloaded error frame", "/v1/messages", claudeFirst, claudeSecond, `{"model":"ws-billing","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"hi"}]}`, `"type":"message_start"`},
		{"responses response.failed", "/v1/responses", responsesFirst, responsesSecond, `{"model":"ws-billing","input":"hi","stream":true}`, `"type":"response.created"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previousRetry := common.RetryTimes
			common.RetryTimes = 1
			t.Cleanup(func() { common.RetryTimes = previousRetry })
			fixture := newResponsesWSBillingTest(t, `tier("request", fixed(0.002))`, func(*websocket.Conn, *http.Request) {})
			var attempts atomic.Int32
			fixture.httpUpstream = func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				payload := tc.second
				if attempts.Add(1) == 1 {
					payload = tc.first
				}
				_, _ = io.WriteString(w, payload)
				w.(http.Flusher).Flush()
			}
			path := tc.path
			if tc.path == "/v1/messages" {
				require.NoError(t, model.DB.Model(fixture.channel).Updates(map[string]any{"type": constant.ChannelTypeAnthropic, "model_mapping": ""}).Error)
				engine := gin.New()
				engine.POST(path, middleware.TokenAuth(), middleware.ModelRequestRateLimit(), middleware.Distribute(), func(c *gin.Context) {
					c.Set(common.RequestIdKey, "stream-retry-claude")
					Relay(c, types.RelayFormatClaude)
				})
				gateway := httptest.NewServer(engine)
				t.Cleanup(gateway.Close)
				fixture.gatewayURL = gateway.URL
			}
			request, err := http.NewRequest(http.MethodPost, fixture.gatewayURL+path, strings.NewReader(tc.body))
			require.NoError(t, err)
			request.Header.Set("Authorization", "Bearer sk-"+fixture.token.Key)
			request.Header.Set("x-api-key", "sk-"+fixture.token.Key)
			request.Header.Set("Content-Type", "application/json")
			response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
			require.NoError(t, err)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			t.Logf("attempts=%d status=%d body=%q", attempts.Load(), response.StatusCode, string(body))
			assert.Equal(t, int32(1), attempts.Load(), "no second upstream attempt once output reached the client")
			assert.Equal(t, 1, strings.Count(string(body), tc.startMarker), "client must not receive two concatenated streams")
		})
	}
}

type failoverRequest struct{ channel, credential string }

type failoverUpstreams struct {
	mu       sync.Mutex
	requests []failoverRequest
}

func (u *failoverUpstreams) record(channel, credential string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.requests = append(u.requests, failoverRequest{channel, credential})
}

func (u *failoverUpstreams) channels() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	names := make([]string, len(u.requests))
	for i, request := range u.requests {
		names[i] = request.channel
	}
	return names
}

func (u *failoverUpstreams) credentials() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	credentials := make([]string, len(u.requests))
	for i, request := range u.requests {
		credentials[i] = request.credential
	}
	return credentials
}

type failoverChannel struct {
	name        string
	group       string // 空表示 default
	channelType int
	priority    int64
	keys        []string
	websocket   bool
	statusMap   string
	respond     func(name string, w http.ResponseWriter, r *http.Request)
}

func failoverFail(status int) func(string, http.ResponseWriter, *http.Request) {
	return func(name string, w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"error":{"message":"upstream %s down","type":"server_error"}}`, name)
	}
}

// failoverOK answers in the format of the request path it was sent to.
func failoverOK(name string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(r.URL.Path, "/embeddings"):
		_, _ = io.WriteString(w, `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"model":"ws-billing","usage":{"prompt_tokens":3,"total_tokens":3}}`)
	case strings.HasSuffix(r.URL.Path, "/images/generations"):
		_, _ = io.WriteString(w, `{"created":1,"data":[{"url":"https://example.com/a.png"}]}`)
	case strings.HasSuffix(r.URL.Path, "/audio/speech"):
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = io.WriteString(w, "audio-from-"+name)
	case strings.HasSuffix(r.URL.Path, "/rerank"):
		_, _ = fmt.Fprintf(w, `{"id":"rerank_%s","results":[{"index":0,"relevance_score":0.9}],"usage":{"prompt_tokens":3,"total_tokens":3}}`, name)
	case strings.HasSuffix(r.URL.Path, "/messages"):
		_, _ = fmt.Fprintf(w, `{"id":"msg_%s","type":"message","role":"assistant","model":"ws-billing","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":1}}`, name)
	case strings.HasSuffix(r.URL.Path, "/responses"):
		_, _ = fmt.Fprintf(w, `{"id":"resp_%s","object":"response","status":"completed","model":"ws-billing","output":[],"usage":{"input_tokens":10,"output_tokens":1,"total_tokens":11}}`, name)
	default:
		_, _ = fmt.Fprintf(w, `{"id":"chat_%s","object":"chat.completion","model":"ws-billing","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":1,"total_tokens":11}}`, name)
	}
}

func failoverResponsesSSE(events func(name string) []string) func(string, http.ResponseWriter, *http.Request) {
	return func(name string, w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range events(name) {
			_, _ = io.WriteString(w, "data: "+event+"\n\n")
			w.(http.Flusher).Flush()
		}
	}
}

func defaultFailoverKey(channelType int, name string) string {
	switch channelType {
	case constant.ChannelTypeCodex:
		return `{"access_token":"token-` + name + `","account_id":"account"}`
	case constant.ChannelTypeClaudeSubscription:
		return `{"claudeAiOauth":{"accessToken":"token-` + name + `"}}`
	}
	return "sk-" + name
}

// newFailoverFixture leaves the fixture's own channel without abilities so
// that only the channels added by the test can serve the model.
func newFailoverFixture(t *testing.T) *responsesWSBillingTest {
	t.Helper()
	fixture := newResponsesWSBillingTest(t, `tier("request", fixed(0.002))`, func(*websocket.Conn, *http.Request) {})
	require.NoError(t, model.DB.Where("channel_id = ?", fixture.channel.Id).Delete(&model.Ability{}).Error)
	return fixture
}

func addFailoverChannel(t *testing.T, upstreams *failoverUpstreams, spec failoverChannel) *model.Channel {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreams.record(spec.name, r.Header.Get("Authorization"))
		spec.respond(spec.name, w, r)
	}))
	t.Cleanup(server.Close)
	keys := spec.keys
	if len(keys) == 0 {
		keys = []string{defaultFailoverKey(spec.channelType, spec.name)}
	}
	group := spec.group
	if group == "" {
		group = "default"
	}
	channel := &model.Channel{
		Name: spec.name, Key: strings.Join(keys, "\n"), Status: common.ChannelStatusEnabled, Type: spec.channelType,
		Group: group, Models: "ws-billing", BaseURL: &server.URL,
		Priority: common.GetPointer(spec.priority), Weight: common.GetPointer(uint(1)),
	}
	if len(keys) > 1 {
		channel.ChannelInfo = model.ChannelInfo{IsMultiKey: true, MultiKeySize: len(keys), MultiKeyMode: constant.MultiKeyModePolling}
	}
	if spec.statusMap != "" {
		channel.StatusCodeMapping = &spec.statusMap
	}
	if spec.websocket {
		channel.SetSetting(dto.ChannelSettings{ResponsesWebSocketEnabled: true})
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(model.DB))
	return channel
}

type failoverGateway struct {
	url  string
	done chan struct{}
}

func newFailoverGateway(t *testing.T) *failoverGateway {
	t.Helper()
	gateway := &failoverGateway{done: make(chan struct{}, 8)}
	engine := gin.New()
	for path, format := range map[string]types.RelayFormat{
		"/v1/chat/completions":   types.RelayFormatOpenAI,
		"/v1/messages":           types.RelayFormatClaude,
		"/v1/responses":          types.RelayFormatOpenAIResponses,
		"/v1/embeddings":         types.RelayFormatEmbedding,
		"/v1/images/generations": types.RelayFormatOpenAIImage,
		"/v1/audio/speech":       types.RelayFormatOpenAIAudio,
		"/v1/rerank":             types.RelayFormatRerank,
	} {
		engine.POST(path, middleware.TokenAuth(), middleware.ModelRequestRateLimit(), middleware.Distribute(), func(c *gin.Context) {
			defer func() { gateway.done <- struct{}{} }()
			c.Set(common.RequestIdKey, "failover-request")
			Relay(c, format)
		})
	}
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	gateway.url = server.URL
	return gateway
}

// post returns once the relay handler has finished, settlement included.
func (g *failoverGateway) post(t *testing.T, path, tokenKey, body string) (int, string) {
	t.Helper()
	status, _, payload := g.postWithHeader(t, path, tokenKey, body)
	return status, payload
}

func (g *failoverGateway) postWithHeader(t *testing.T, path, tokenKey, body string) (int, http.Header, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, g.url+path, strings.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer sk-"+tokenKey)
	request.Header.Set("x-api-key", "sk-"+tokenKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	require.NoError(t, err)
	payload, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	select {
	case <-g.done:
	case <-time.After(3 * time.Second):
		require.FailNow(t, "relay handler did not finish")
	}
	return response.StatusCode, response.Header, string(payload)
}

const failoverChatBody = `{"model":"ws-billing","messages":[{"role":"user","content":"hello"}]}`
const failoverResponsesBody = `{"model":"ws-billing","input":"hello"}`

// requireAttemptGroups checks that got is the concatenation of groups, where the
// members of one group may come in any order.
func requireAttemptGroups(t *testing.T, got []string, groups ...[]string) {
	t.Helper()
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	require.Len(t, got, total, "attempts: %v", got)
	offset := 0
	for _, group := range groups {
		assert.ElementsMatch(t, group, got[offset:offset+len(group)], "attempts: %v", got)
		offset += len(group)
	}
}

func TestRelaySyncFailoverAcrossChannels(t *testing.T) {
	// The ChatGPT subscription channel only serves Responses and the Claude
	// subscription channel does not, so the two types are exercised on
	// different endpoints.
	mainPath := func(dRespond func(string, http.ResponseWriter, *http.Request)) []failoverChannel {
		return []failoverChannel{
			{name: "A", channelType: constant.ChannelTypeClaudeSubscription, priority: 0, respond: failoverFail(500)},
			{name: "B", channelType: constant.ChannelTypeClaudeSubscription, priority: 0, respond: failoverFail(500)},
			{name: "C", channelType: constant.ChannelTypeOpenAI, priority: 10, respond: failoverFail(500)},
			{name: "D", channelType: constant.ChannelTypeOpenAI, priority: 5, respond: dRespond},
		}
	}
	for _, tc := range []struct {
		name       string
		channels   []failoverChannel
		path, body string
		retryTimes int
		retryCodes string
		pin        string
		wantStatus int
		wantGroups [][]string
		wantBody   string
		wantQuotas []int
	}{
		{name: "claude subscription channels first then by priority", channels: mainPath(failoverOK), path: "/v1/chat/completions", body: failoverChatBody,
			wantStatus: 200, wantBody: "chat_D", wantGroups: [][]string{{"A", "B"}, {"C"}, {"D"}}, wantQuotas: []int{1000}},
		{name: "chatgpt subscription channel before higher priority openai", path: "/v1/responses", body: failoverResponsesBody,
			channels: []failoverChannel{
				{name: "A", channelType: constant.ChannelTypeCodex, priority: 0, respond: failoverFail(500)},
				{name: "C", channelType: constant.ChannelTypeOpenAI, priority: 10, respond: failoverFail(500)},
				{name: "D", channelType: constant.ChannelTypeOpenAI, priority: 5, respond: failoverOK},
			},
			wantStatus: 200, wantBody: "resp_D", wantGroups: [][]string{{"A"}, {"C"}, {"D"}}, wantQuotas: []int{1000}},
		{name: "all channels fail returns the last error", channels: mainPath(failoverFail(500)), path: "/v1/chat/completions", body: failoverChatBody,
			wantStatus: 500, wantBody: "upstream D down", wantGroups: [][]string{{"A", "B"}, {"C"}, {"D"}}},
		{name: "single channel is tried once whatever the retry times", path: "/v1/chat/completions", body: failoverChatBody, retryTimes: 3,
			channels:   []failoverChannel{{name: "E", channelType: constant.ChannelTypeOpenAI, priority: 0, respond: failoverFail(500)}},
			wantStatus: 500, wantBody: "upstream E down", wantGroups: [][]string{{"E"}}},
		{name: "status outside the retry list stops", path: "/v1/chat/completions", body: failoverChatBody,
			channels: []failoverChannel{
				{name: "A", channelType: constant.ChannelTypeOpenAI, priority: 10, respond: failoverFail(400)},
				{name: "B", channelType: constant.ChannelTypeOpenAI, priority: 5, respond: failoverOK},
			},
			wantStatus: 400, wantBody: "upstream A down", wantGroups: [][]string{{"A"}}},
		{name: "status added to the retry list moves on", path: "/v1/chat/completions", body: failoverChatBody, retryCodes: "400,500-599",
			channels: []failoverChannel{
				{name: "A", channelType: constant.ChannelTypeOpenAI, priority: 10, respond: failoverFail(400)},
				{name: "B", channelType: constant.ChannelTypeOpenAI, priority: 5, respond: failoverOK},
			},
			wantStatus: 200, wantBody: "chat_B", wantGroups: [][]string{{"A"}, {"B"}}, wantQuotas: []int{1000}},
		{name: "token pinned channel is not replaced", path: "/v1/chat/completions", body: failoverChatBody, pin: "A",
			channels: []failoverChannel{
				{name: "A", channelType: constant.ChannelTypeOpenAI, priority: 5, respond: failoverFail(500)},
				{name: "B", channelType: constant.ChannelTypeOpenAI, priority: 10, respond: failoverOK},
			},
			wantStatus: 500, wantBody: "upstream A down", wantGroups: [][]string{{"A"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previousRetry := common.RetryTimes
			previousCodes := operation_setting.AutomaticRetryStatusCodesToString()
			common.RetryTimes = tc.retryTimes
			t.Cleanup(func() {
				common.RetryTimes = previousRetry
				require.NoError(t, operation_setting.AutomaticRetryStatusCodesFromString(previousCodes))
			})
			if tc.retryCodes != "" {
				require.NoError(t, operation_setting.AutomaticRetryStatusCodesFromString(tc.retryCodes))
			}
			fixture := newFailoverFixture(t)
			gateway := newFailoverGateway(t)
			upstreams := &failoverUpstreams{}
			channels := map[string]*model.Channel{}
			for _, spec := range tc.channels {
				channels[spec.name] = addFailoverChannel(t, upstreams, spec)
			}
			tokenKey := fixture.token.Key
			if tc.pin != "" {
				require.NoError(t, model.DB.Model(fixture.user).Update("role", common.RoleAdminUser).Error)
				tokenKey = fmt.Sprintf("%s-%d", tokenKey, channels[tc.pin].Id)
			}

			status, body := gateway.post(t, tc.path, tokenKey, tc.body)

			assert.Equal(t, tc.wantStatus, status, body)
			assert.Contains(t, body, tc.wantBody)
			assert.NotContains(t, strings.ToLower(body), "no available channel")
			requireAttemptGroups(t, upstreams.channels(), tc.wantGroups...)
			assertResponsesWSAccounting(t, fixture, tc.wantQuotas)
		})
	}
}

func TestRelaySyncFailoverUsesEachKeyOnce(t *testing.T) {
	fixture := newFailoverFixture(t)
	gateway := newFailoverGateway(t)
	upstreams := &failoverUpstreams{}
	keys := make([]string, 3)
	for i := range keys {
		keys[i] = fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"k%d"}}`, i+1)
	}
	addFailoverChannel(t, upstreams, failoverChannel{name: "F", channelType: constant.ChannelTypeClaudeSubscription, keys: keys,
		respond: func(name string, w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer k3" {
				failoverFail(http.StatusTooManyRequests)(name, w, r)
				return
			}
			failoverOK(name, w, r)
		}})

	status, body := gateway.post(t, "/v1/chat/completions", fixture.token.Key, failoverChatBody)

	assert.Equal(t, http.StatusOK, status, body)
	assert.Equal(t, []string{"Bearer k1", "Bearer k2", "Bearer k3"}, upstreams.credentials())
	assertResponsesWSAccounting(t, fixture, []int{1000})
}

func TestRelaySyncFailoverDropsFailedAttemptStream(t *testing.T) {
	failedThenOK := func(name string) []string {
		if name == "A" {
			return []string{
				`{"type":"response.created","response":{"id":"resp_A","status":"in_progress"}}`,
				`{"type":"response.failed","response":{"id":"resp_A","status":"failed","error":{"code":"server_error","message":"upstream A failed"}}}`,
			}
		}
		return []string{
			`{"type":"response.created","response":{"id":"resp_B","status":"in_progress"}}`,
			`{"type":"response.output_text.delta","delta":"hello"}`,
			`{"type":"response.completed","response":{"id":"resp_B","status":"completed","usage":{"input_tokens":10,"output_tokens":1,"total_tokens":11}}}`,
		}
	}
	openingThenFailed := func(openings int) func(string) []string {
		return func(name string) []string {
			if name != "A" {
				return failedThenOK(name)
			}
			events := []string{`{"type":"response.created","response":{"id":"resp_A","status":"in_progress"}}`}
			for range openings - 1 {
				events = append(events, `{"type":"response.in_progress","response":{"id":"resp_A","status":"in_progress"}}`)
			}
			return append(events, `{"type":"response.failed","response":{"id":"resp_A","status":"failed","error":{"code":"server_error","message":"upstream A failed"}}}`)
		}
	}
	for _, tc := range []struct {
		name        string
		channels    []string
		events      func(name string) []string
		wantGroups  [][]string
		wantBody    []string
		notBody     []string
		wantQuotas  []int
		wantCreated int
	}{
		{name: "64 held opening events then failure moves on", channels: []string{"A", "B"}, events: openingThenFailed(64),
			wantGroups: [][]string{{"A"}, {"B"}}, wantBody: []string{`"id":"resp_B"`, "response.completed"}, notBody: []string{"resp_A", "upstream A failed"}, wantQuotas: []int{1000}},
		{name: "65 opening events are released so the later failure is not retried", channels: []string{"A", "B"}, events: openingThenFailed(65),
			wantGroups: [][]string{{"A"}}, wantBody: []string{"upstream A failed"}, notBody: []string{"resp_B"}, wantQuotas: []int{1000}, wantCreated: 65},
		{name: "next channel succeeds", channels: []string{"A", "B"}, wantGroups: [][]string{{"A"}, {"B"}},
			wantBody: []string{`"id":"resp_B"`, "response.completed"}, notBody: []string{"resp_A", "upstream A failed"}, wantQuotas: []int{1000}},
		{name: "last channel keeps the failure events", channels: []string{"A"}, wantGroups: [][]string{{"A"}},
			wantBody: []string{`"id":"resp_A"`, "response.created", "response.failed", "upstream A failed"}, notBody: []string{"resp_B"}, wantQuotas: []int{1000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := failedThenOK
			if tc.events != nil {
				events = tc.events
			}
			fixture := newFailoverFixture(t)
			upstreams := &failoverUpstreams{}
			for i, name := range tc.channels {
				addFailoverChannel(t, upstreams, failoverChannel{name: name, channelType: constant.ChannelTypeOpenAI, priority: int64(10 - i), respond: failoverResponsesSSE(events)})
			}

			gateway := newFailoverGateway(t)

			status, body := gateway.post(t, "/v1/responses", fixture.token.Key, `{"model":"ws-billing","input":"hello","stream":true}`)

			assert.Equal(t, http.StatusOK, status)
			for _, want := range tc.wantBody {
				assert.Contains(t, body, want)
			}
			for _, unwanted := range tc.notBody {
				assert.NotContains(t, body, unwanted)
			}
			if tc.wantCreated > 0 {
				assert.Equal(t, tc.wantCreated, strings.Count(body, `"id":"resp_A"`)-1, "every opening event plus the failure event reaches the client")
				assert.Contains(t, body[:min(len(body), 80)], `"type":"response.created"`, "response.created is the first event")
				assert.Less(t, strings.LastIndex(body, `"type":"response.in_progress"`), strings.Index(body, `"type":"response.failed"`), "opening events precede the failure event")
			}
			requireAttemptGroups(t, upstreams.channels(), tc.wantGroups...)
			assertResponsesWSAccounting(t, fixture, tc.wantQuotas)
		})
	}
}

func TestRelaySyncFailoverClaudeStreamStartsFresh(t *testing.T) {
	fixture := newFailoverFixture(t)
	upstreams := &failoverUpstreams{}
	chat := func(name string, w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		frames := []string{`{"error":{"message":"upstream A overloaded","type":"server_error"}}`, "[DONE]"}
		if name == "B" {
			frames = []string{
				`{"id":"chat_B","object":"chat.completion.chunk","model":"ws-billing","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
				`{"id":"chat_B","object":"chat.completion.chunk","model":"ws-billing","choices":[{"index":0,"delta":{"content":"from B"}}]}`,
				`{"id":"chat_B","object":"chat.completion.chunk","model":"ws-billing","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
				"[DONE]",
			}
		}
		for _, frame := range frames {
			_, _ = io.WriteString(w, "data: "+frame+"\n\n")
			w.(http.Flusher).Flush()
		}
	}
	addFailoverChannel(t, upstreams, failoverChannel{name: "A", channelType: constant.ChannelTypeOpenAI, priority: 10, respond: chat})
	addFailoverChannel(t, upstreams, failoverChannel{name: "B", channelType: constant.ChannelTypeOpenAI, priority: 5, respond: chat})
	gateway := newFailoverGateway(t)

	status, body := gateway.post(t, "/v1/messages", fixture.token.Key, `{"model":"ws-billing","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"hi"}]}`)

	assert.Equal(t, http.StatusOK, status)
	requireAttemptGroups(t, upstreams.channels(), []string{"A"}, []string{"B"})
	assert.Equal(t, 1, strings.Count(body, `"type":"message_start"`), body)
	assert.Contains(t, body, "from B")
	assert.Contains(t, body, `"type":"message_stop"`)
	assert.NotContains(t, body, "upstream A overloaded")
	assertResponsesWSAccounting(t, fixture, []int{1000})
}

func TestRelaySyncFailoverClaudeStreamHoldLimit(t *testing.T) {
	claudeSSE := func(pings int) func(string, http.ResponseWriter, *http.Request) {
		return func(name string, w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			write := func(event, data string) {
				_, _ = io.WriteString(w, "event: "+event+"\ndata: "+data+"\n\n")
				w.(http.Flusher).Flush()
			}
			write("message_start", `{"type":"message_start","message":{"id":"msg_`+name+`","type":"message","role":"assistant","model":"ws-billing","content":[],"usage":{"input_tokens":10,"output_tokens":0}}}`)
			if name == "A" {
				for range pings {
					write("ping", `{"type":"ping"}`)
				}
				write("error", `{"type":"error","error":{"type":"overloaded_error","message":"upstream A overloaded"}}`)
				return
			}
			write("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
			write("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"from B"}}`)
			write("content_block_stop", `{"type":"content_block_stop","index":0}`)
			write("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`)
			write("message_stop", `{"type":"message_stop"}`)
		}
	}
	for _, tc := range []struct {
		name       string
		pings      int
		wantGroups [][]string
		wantFromB  bool
	}{
		{name: "64 held opening events then error moves on", pings: 63, wantGroups: [][]string{{"A"}, {"B"}}, wantFromB: true},
		{name: "65 opening events are released so the later error is not retried", pings: 64, wantGroups: [][]string{{"A"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newFailoverFixture(t)
			upstreams := &failoverUpstreams{}
			addFailoverChannel(t, upstreams, failoverChannel{name: "A", channelType: constant.ChannelTypeAnthropic, priority: 10, respond: claudeSSE(tc.pings)})
			addFailoverChannel(t, upstreams, failoverChannel{name: "B", channelType: constant.ChannelTypeAnthropic, priority: 5, respond: claudeSSE(tc.pings)})
			gateway := newFailoverGateway(t)

			status, body := gateway.post(t, "/v1/messages", fixture.token.Key, `{"model":"ws-billing","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"hi"}]}`)

			assert.Equal(t, http.StatusOK, status)
			requireAttemptGroups(t, upstreams.channels(), tc.wantGroups...)
			if tc.wantFromB {
				assert.Equal(t, 1, strings.Count(body, `"type":"message_start"`), body)
				assert.Contains(t, body, "from B")
				assert.NotContains(t, body, "msg_A")
				assert.NotContains(t, body, "upstream A overloaded")
				return
			}
			assert.Equal(t, 1+tc.pings, strings.Count(body, `"type":"message_start"`)+strings.Count(body, `"type":"ping"`), "every held opening event reaches the client")
			assert.Less(t, strings.LastIndex(body, `"type":"ping"`), strings.Index(body, "upstream A overloaded"), "opening events precede the error")
			assert.NotContains(t, body, "from B")
		})
	}
}

func TestRelaySyncFailoverNonResponsesRequests(t *testing.T) {
	chatStream := func(name string, w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, frame := range []string{
			`{"id":"chat_` + name + `","object":"chat.completion.chunk","model":"ws-billing","choices":[{"index":0,"delta":{"role":"assistant","content":"from ` + name + `"}}]}`,
			`{"id":"chat_` + name + `","object":"chat.completion.chunk","model":"ws-billing","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
			"[DONE]",
		} {
			_, _ = io.WriteString(w, "data: "+frame+"\n\n")
			w.(http.Flusher).Flush()
		}
	}
	for _, tc := range []struct {
		name, path, body string
		ok               func(string, http.ResponseWriter, *http.Request)
		wantBody         string
	}{
		{name: "embedding", path: "/v1/embeddings", body: `{"model":"ws-billing","input":"hello"}`, ok: failoverOK, wantBody: "embedding"},
		{name: "image", path: "/v1/images/generations", body: `{"model":"ws-billing","prompt":"a cat"}`, ok: failoverOK, wantBody: "a.png"},
		{name: "speech", path: "/v1/audio/speech", body: `{"model":"ws-billing","input":"hello","voice":"alloy"}`, ok: failoverOK, wantBody: "audio-from-P3"},
		{name: "rerank", path: "/v1/rerank", body: `{"model":"ws-billing","query":"q","documents":["a","b"]}`, ok: failoverOK, wantBody: "relevance_score"},
		{name: "streaming chat", path: "/v1/chat/completions", body: `{"model":"ws-billing","stream":true,"messages":[{"role":"user","content":"hello"}]}`, ok: chatStream, wantBody: "from P3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previousRetry := common.RetryTimes
			common.RetryTimes = 0
			t.Cleanup(func() { common.RetryTimes = previousRetry })
			fixture := newFailoverFixture(t)
			gateway := newFailoverGateway(t)
			upstreams := &failoverUpstreams{}
			for i, name := range []string{"P1", "P2", "P3"} {
				respond := failoverFail(http.StatusInternalServerError)
				if name == "P3" {
					respond = tc.ok
				}
				addFailoverChannel(t, upstreams, failoverChannel{name: name, channelType: constant.ChannelTypeOpenAI, priority: int64(30 - 10*i), respond: respond})
			}

			status, body := gateway.post(t, tc.path, fixture.token.Key, tc.body)

			assert.Equal(t, http.StatusOK, status, body)
			assert.Contains(t, body, tc.wantBody)
			assert.NotContains(t, body, "P1")
			assert.NotContains(t, body, "P2")
			assert.Equal(t, []string{"P1", "P2", "P3"}, upstreams.channels())
			assertResponsesWSAccounting(t, fixture, []int{1000})
		})
	}
}

func TestRelaySyncFailoverRealtimeDialFailure(t *testing.T) {
	previousRetry := common.RetryTimes
	common.RetryTimes = 0
	t.Cleanup(func() { common.RetryTimes = previousRetry })
	fixture := newFailoverFixture(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"billing_setting.billing_mode": `{}`}))
	previousRatios := ratio_setting.ModelRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"ws-billing":1}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousRatios)) })
	upstreams := &failoverUpstreams{}
	addFailoverChannel(t, upstreams, failoverChannel{name: "A", channelType: constant.ChannelTypeOpenAI, priority: 10, respond: failoverFail(http.StatusInternalServerError)})
	addFailoverChannel(t, upstreams, failoverChannel{name: "B", channelType: constant.ChannelTypeOpenAI, priority: 5,
		respond: func(_ string, w http.ResponseWriter, r *http.Request) {
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"session.created","session":{"id":"sess_B"}}`))
			_, _, _ = conn.ReadMessage()
		}})
	engine := gin.New()
	engine.GET("/v1/realtime", middleware.TokenAuth(), middleware.Distribute(), func(c *gin.Context) {
		c.Set(common.RequestIdKey, "failover-request")
		Relay(c, types.RelayFormatOpenAIRealtime)
	})
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	header := http.Header{"Sec-WebSocket-Protocol": []string{"realtime, openai-insecure-api-key.sk-" + fixture.token.Key}}
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/v1/realtime?model=ws-billing", header)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, message, err := client.ReadMessage()

	require.NoError(t, err)
	assert.Contains(t, string(message), "sess_B")
	assert.Equal(t, []string{"A", "B"}, upstreams.channels())
}

func TestRelaySyncFailoverStatusCodeMapping(t *testing.T) {
	sse := func(first string) func(string, http.ResponseWriter, *http.Request) {
		return failoverResponsesSSE(func(name string) []string {
			if name == "A" {
				return []string{`{"type":"response.created","response":{"id":"resp_A","status":"in_progress"}}`, first}
			}
			return []string{
				`{"type":"response.created","response":{"id":"resp_B","status":"in_progress"}}`,
				`{"type":"response.output_text.delta","delta":"hello"}`,
				`{"type":"response.completed","response":{"id":"resp_B","status":"completed","usage":{"input_tokens":10,"output_tokens":1,"total_tokens":11}}}`,
			}
		})
	}
	for _, tc := range []struct {
		name       string
		statusMap  string
		failure    string
		wantGroups [][]string
		wantBody   []string
		notBody    []string
	}{
		{name: "mapped to a status outside the retry list keeps the failure", statusMap: `{"500":"400"}`,
			failure:    `{"type":"response.failed","response":{"id":"resp_A","status":"failed","error":{"code":"500","message":"upstream A failed"}}}`,
			wantGroups: [][]string{{"A"}}, wantBody: []string{"response.created", "response.failed", "upstream A failed"}, notBody: []string{"resp_B"}},
		{name: "mapped to a retryable status moves on", statusMap: `{"400":"500"}`,
			failure:    `{"type":"error","code":"400","message":"upstream A rejected"}`,
			wantGroups: [][]string{{"A"}, {"B"}}, wantBody: []string{`"id":"resp_B"`, "response.completed"}, notBody: []string{"resp_A", "upstream A rejected"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newFailoverFixture(t)
			gateway := newFailoverGateway(t)
			upstreams := &failoverUpstreams{}
			addFailoverChannel(t, upstreams, failoverChannel{name: "A", channelType: constant.ChannelTypeOpenAI, priority: 10, statusMap: tc.statusMap, respond: sse(tc.failure)})
			addFailoverChannel(t, upstreams, failoverChannel{name: "B", channelType: constant.ChannelTypeOpenAI, priority: 5, respond: sse(tc.failure)})

			status, body := gateway.post(t, "/v1/responses", fixture.token.Key, `{"model":"ws-billing","input":"hello","stream":true}`)

			assert.Equal(t, http.StatusOK, status)
			for _, want := range tc.wantBody {
				assert.Contains(t, body, want)
			}
			for _, unwanted := range tc.notBody {
				assert.NotContains(t, body, unwanted)
			}
			requireAttemptGroups(t, upstreams.channels(), tc.wantGroups...)
			assertResponsesWSAccounting(t, fixture, []int{1000})
		})
	}
}

func TestRelaySyncFailoverStopsAfterClientDisconnect(t *testing.T) {
	fixture := newFailoverFixture(t)
	gateway := newFailoverGateway(t)
	upstreams := &failoverUpstreams{}
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	slowFail := func(name string, w http.ResponseWriter, r *http.Request) {
		if name == "A" {
			close(started)
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}
		failoverFail(http.StatusInternalServerError)(name, w, r)
	}
	for i, name := range []string{"A", "B", "C"} {
		addFailoverChannel(t, upstreams, failoverChannel{name: name, channelType: constant.ChannelTypeOpenAI, priority: int64(30 - 10*i), respond: slowFail})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, gateway.url+"/v1/chat/completions", strings.NewReader(failoverChatBody))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer sk-"+fixture.token.Key)
	request.Header.Set("Content-Type", "application/json")
	clientDone := make(chan error, 1)
	go func() {
		response, err := http.DefaultClient.Do(request)
		if err == nil {
			response.Body.Close()
		}
		clientDone <- err
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		require.FailNow(t, "the first channel was never requested")
	}
	cancel()
	require.Error(t, <-clientDone)
	releaseOnce.Do(func() { close(release) })
	select {
	case <-gateway.done:
	case <-time.After(3 * time.Second):
		require.FailNow(t, "relay handler did not finish")
	}

	assert.Equal(t, []string{"A"}, upstreams.channels(), "a disconnected client must not start further attempts")
}

func failoverHeaderSSE(headers map[string]string, events func(name string) []string) func(string, http.ResponseWriter, *http.Request) {
	return func(name string, w http.ResponseWriter, r *http.Request) {
		for key, value := range headers {
			w.Header().Set(key, value)
		}
		failoverResponsesSSE(events)(name, w, r)
	}
}

func TestRelaySyncFailoverDropsFailedAttemptHeaders(t *testing.T) {
	failedAttempt := func(string) []string {
		return []string{
			`{"type":"response.created","response":{"id":"resp_A","status":"in_progress"}}`,
			`{"type":"response.failed","response":{"id":"resp_A","status":"failed","error":{"code":"server_error","message":"upstream A failed"}}}`,
		}
	}
	succeeded := func(string) []string {
		return []string{
			`{"type":"response.created","response":{"id":"resp_B","status":"in_progress"}}`,
			`{"type":"response.output_text.delta","delta":"hello"}`,
			`{"type":"response.completed","response":{"id":"resp_B","status":"completed","usage":{"input_tokens":10,"output_tokens":1,"total_tokens":11}}}`,
		}
	}
	for _, tc := range []struct {
		name        string
		secondEmits map[string]string
		wantTurn    []string
	}{
		{name: "second channel sends no codex headers", wantTurn: nil},
		{name: "second channel sends its own turn state", secondEmits: map[string]string{"X-Codex-Turn-State": "B"}, wantTurn: []string{"B"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newFailoverFixture(t)
			upstreams := &failoverUpstreams{}
			addFailoverChannel(t, upstreams, failoverChannel{name: "A", channelType: constant.ChannelTypeCodex, priority: 0,
				respond: failoverHeaderSSE(map[string]string{"X-Codex-Turn-State": "A", "X-Reasoning-Included": "true"}, failedAttempt)})
			addFailoverChannel(t, upstreams, failoverChannel{name: "B", channelType: constant.ChannelTypeOpenAI, priority: 0,
				respond: failoverHeaderSSE(tc.secondEmits, succeeded)})
			gateway := newFailoverGateway(t)

			status, header, body := gateway.postWithHeader(t, "/v1/responses", fixture.token.Key, `{"model":"ws-billing","input":"hello","stream":true}`)

			assert.Equal(t, http.StatusOK, status)
			assert.Contains(t, body, `"id":"resp_B"`)
			requireAttemptGroups(t, upstreams.channels(), []string{"A"}, []string{"B"})
			assert.Equal(t, tc.wantTurn, header.Values("X-Codex-Turn-State"))
			assert.Empty(t, header.Values("X-Reasoning-Included"))
		})
	}
}

func TestRelaySyncFailoverCountsSuccessForRateLimit(t *testing.T) {
	for _, tc := range []struct {
		name        string
		firstFails  bool
		wantChannel [][]string
	}{
		{name: "success after a failed channel", firstFails: true, wantChannel: [][]string{{"A"}, {"B"}}},
		{name: "success on the first channel", firstFails: false, wantChannel: [][]string{{"B"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, i18n.Init())
			fixture := newFailoverFixture(t)
			setting.ModelRequestRateLimitEnabled = true
			setting.ModelRequestRateLimitDurationMinutes = 1
			setting.ModelRequestRateLimitCount = 0
			setting.ModelRequestRateLimitSuccessCount = 1
			upstreams := &failoverUpstreams{}
			if tc.firstFails {
				addFailoverChannel(t, upstreams, failoverChannel{name: "A", channelType: constant.ChannelTypeOpenAI, priority: 10,
					respond: failoverResponsesSSE(func(string) []string {
						return []string{
							`{"type":"response.created","response":{"id":"resp_A","status":"in_progress"}}`,
							`{"type":"response.failed","response":{"id":"resp_A","status":"failed","error":{"code":"server_error","message":"upstream A failed"}}}`,
						}
					})})
			}
			addFailoverChannel(t, upstreams, failoverChannel{name: "B", channelType: constant.ChannelTypeAnthropic, priority: 5,
				respond: func(_ string, w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					for _, event := range []string{
						`{"type":"message_start","message":{"id":"msg_B","type":"message","role":"assistant","model":"ws-billing","content":[],"usage":{"input_tokens":10,"output_tokens":1}}}`,
						`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
						`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
						`{"type":"content_block_stop","index":0}`,
						`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
						`{"type":"message_stop"}`,
					} {
						_, _ = io.WriteString(w, "data: "+event+"\n\n")
						w.(http.Flusher).Flush()
					}
				}})
			gateway := newFailoverGateway(t)
			body := `{"model":"ws-billing","input":"hello","stream":true}`

			status, _ := gateway.post(t, "/v1/responses", fixture.token.Key, body)
			require.Equal(t, http.StatusOK, status)
			requireAttemptGroups(t, upstreams.channels(), tc.wantChannel...)

			request, err := http.NewRequest(http.MethodPost, gateway.url+"/v1/responses", strings.NewReader(body))
			require.NoError(t, err)
			request.Header.Set("Authorization", "Bearer sk-"+fixture.token.Key)
			request.Header.Set("Content-Type", "application/json")
			response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())

			assert.Equal(t, http.StatusTooManyRequests, response.StatusCode, "the first request succeeded and counts as one success")
		})
	}
}

func TestDistributeAbortsWhenSelectedChannelHasNoEnabledKey(t *testing.T) {
	require.NoError(t, i18n.Init())
	fixture := newFailoverFixture(t)
	upstreams := &failoverUpstreams{}
	channel := addFailoverChannel(t, upstreams, failoverChannel{name: "F", channelType: constant.ChannelTypeOpenAI, keys: []string{"sk-k1", "sk-k2"}, respond: failoverOK})
	channel.ChannelInfo.MultiKeyStatusList = map[int]int{0: common.ChannelStatusAutoDisabled, 1: common.ChannelStatusAutoDisabled}
	require.NoError(t, model.DB.Model(channel).Update("channel_info", channel.ChannelInfo).Error)
	gateway := newFailoverGateway(t)

	request, err := http.NewRequest(http.MethodPost, gateway.url+"/v1/chat/completions", strings.NewReader(failoverChatBody))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer sk-"+fixture.token.Key)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	require.NoError(t, err, "the gateway must answer instead of panicking")
	payload, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	assert.GreaterOrEqual(t, response.StatusCode, http.StatusBadRequest, string(payload))
	var failure struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(payload, &failure), string(payload))
	assert.Equal(t, string(types.ErrorCodeChannelNoAvailableKey), failure.Error.Code, "the key failure must be reported, not an attempt to call an unconfigured upstream")
	assert.Empty(t, upstreams.channels(), "no upstream request may be sent without a key")
}
