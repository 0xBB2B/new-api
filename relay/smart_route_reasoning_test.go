package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func claudeRequestFromJSON(t *testing.T, body string) *dto.ClaudeRequest {
	t.Helper()
	request := &dto.ClaudeRequest{}
	require.NoError(t, common.UnmarshalJsonStr(body, request))
	return request
}

func TestStripClaudeThinkingBlocks(t *testing.T) {
	tests := []struct {
		name      string
		messages  string
		want      string
		wantCount int
	}{
		{
			"thinking and redacted_thinking are removed",
			`[{"role":"user","content":"q"},{"role":"assistant","content":[{"type":"thinking","thinking":"a","signature":"s"},{"type":"redacted_thinking","data":"x"},{"type":"text","text":"answer"}]}]`,
			`[{"role":"user","content":"q"},{"role":"assistant","content":[{"type":"text","text":"answer"}]}]`,
			2,
		},
		{
			"assistant left empty is dropped",
			`[{"role":"user","content":"q"},{"role":"assistant","content":[{"type":"thinking","thinking":"a","signature":"s"}]},{"role":"user","content":"again"}]`,
			`[{"role":"user","content":"q"},{"role":"user","content":"again"}]`,
			1,
		},
		{
			"counts every block across messages",
			`[{"role":"assistant","content":[{"type":"thinking","thinking":"a","signature":"s"},{"type":"tool_use","id":"t1","name":"f","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]},{"role":"assistant","content":[{"type":"thinking","thinking":"b","signature":"s2"},{"type":"text","text":"done"}]}]`,
			`[{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"f","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]},{"role":"assistant","content":[{"type":"text","text":"done"}]}]`,
			2,
		},
		{
			"nothing to remove",
			`[{"role":"user","content":"q"},{"role":"assistant","content":"plain"},{"role":"assistant","content":[{"type":"text","text":"t"}]}]`,
			`[{"role":"user","content":"q"},{"role":"assistant","content":"plain"},{"role":"assistant","content":[{"type":"text","text":"t"}]}]`,
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := claudeRequestFromJSON(t, `{"model":"m","messages":`+tt.messages+`}`)

			count := stripClaudeThinkingBlocks(request)

			assert.Equal(t, tt.wantCount, count)
			sent, err := common.Marshal(request.Messages)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(sent))
		})
	}
}

const claudeHistoryWithThinking = `{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"q"},{"role":"assistant","content":[{"type":"thinking","thinking":"a","signature":"s"},{"type":"redacted_thinking","data":"x"},{"type":"text","text":"answer"}]},{"role":"user","content":"again"}]}`

type claudeAttempt struct {
	c        *gin.Context
	upstream *httptest.Server
	sent     chan []byte
}

func newClaudeAttempt(t *testing.T, raw string, setting dto.ChannelSettings, lastChannelID int) *claudeAttempt {
	t.Helper()
	service.InitHttpClient()
	a := &claudeAttempt{sent: make(chan []byte, 4)}
	a.upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		a.sent <- body
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"stop here"}}`))
	}))
	t.Cleanup(a.upstream.Close)

	a.c, _ = gin.CreateTestContext(httptest.NewRecorder())
	a.c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	storage, err := common.CreateBodyStorage([]byte(raw))
	require.NoError(t, err)
	t.Cleanup(func() { storage.Close() })
	a.c.Set(common.KeyBodyStorage, storage)
	common.SetContextKey(a.c, constant.ContextKeyOriginalModel, "claude-sonnet-4-5")
	common.SetContextKey(a.c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	common.SetContextKey(a.c, constant.ContextKeyChannelBaseUrl, a.upstream.URL)
	common.SetContextKey(a.c, constant.ContextKeyChannelKey, "sk-placeholder")
	common.SetContextKey(a.c, constant.ContextKeyChannelSetting, setting)
	smartroute.SetRouteResult(a.c, smartroute.RouteResult{Session: &smartroute.SessionRecord{LastChannelID: lastChannelID}})
	return a
}

func (a *claudeAttempt) run(t *testing.T, raw string, channelID int) []byte {
	t.Helper()
	common.SetContextKey(a.c, constant.ContextKeyChannelId, channelID)
	request := claudeRequestFromJSON(t, raw)
	info := relaycommon.GenRelayInfoClaude(a.c, request)
	_ = ClaudeHelper(a.c, info)
	select {
	case body := <-a.sent:
		return body
	default:
		require.Fail(t, "no request reached the upstream")
		return nil
	}
}

func cleanedReasoningLog(t *testing.T, c *gin.Context) any {
	t.Helper()
	fields, ok := common.GetContextKey(c, constant.ContextKeySmartRouteLog)
	require.True(t, ok)
	return fields.(map[string]any)["cleaned_reasoning"]
}

func TestClaudeHelperStripsThinkingBlocksOnTheWire(t *testing.T) {
	const cleaned = `{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"q"},{"role":"assistant","content":[{"type":"text","text":"answer"}]},{"role":"user","content":"again"}]}`
	t.Run("converted request changes channel", func(t *testing.T) {
		a := newClaudeAttempt(t, claudeHistoryWithThinking, dto.ChannelSettings{}, 5)

		sent := a.run(t, claudeHistoryWithThinking, 9)

		assert.JSONEq(t, cleaned, string(sent))
		assert.Equal(t, 2, cleanedReasoningLog(t, a.c))
	})

	t.Run("channel passthrough changes channel", func(t *testing.T) {
		a := newClaudeAttempt(t, claudeHistoryWithThinking, dto.ChannelSettings{PassThroughBodyEnabled: true}, 5)

		sent := a.run(t, claudeHistoryWithThinking, 9)

		assert.JSONEq(t, cleaned, string(sent))
		assert.Equal(t, 2, cleanedReasoningLog(t, a.c))
	})

	t.Run("global passthrough changes channel", func(t *testing.T) {
		settings := model_setting.GetGlobalSettings()
		original := settings.PassThroughRequestEnabled
		t.Cleanup(func() { settings.PassThroughRequestEnabled = original })
		settings.PassThroughRequestEnabled = true
		a := newClaudeAttempt(t, claudeHistoryWithThinking, dto.ChannelSettings{}, 5)

		sent := a.run(t, claudeHistoryWithThinking, 9)

		assert.JSONEq(t, cleaned, string(sent))
		assert.Equal(t, 2, cleanedReasoningLog(t, a.c))
	})

	t.Run("passthrough keeps big integers and unknown fields byte for byte", func(t *testing.T) {
		const raw = `{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"q"},{"role":"assistant","cache_hint":{"x":1},"content":[{"type":"thinking","thinking":"a","signature":"s"},{"type":"redacted_thinking","data":"x"},{"type":"tool_use","id":"t1","name":"f","input":{"id":12345678901234567890},"extra_flag":true},{"type":"text","text":"answer","citations_hint":[1]}]},{"role":"user","content":"again"}]}`
		const want = `{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"q"},{"role":"assistant","cache_hint":{"x":1},"content":[{"type":"tool_use","id":"t1","name":"f","input":{"id":12345678901234567890},"extra_flag":true},{"type":"text","text":"answer","citations_hint":[1]}]},{"role":"user","content":"again"}]}`
		a := newClaudeAttempt(t, raw, dto.ChannelSettings{PassThroughBodyEnabled: true}, 5)

		sent := a.run(t, raw, 9)

		assert.Equal(t, want, string(sent))
		assert.Equal(t, 2, cleanedReasoningLog(t, a.c))
	})

	t.Run("same channel sends the request as is", func(t *testing.T) {
		a := newClaudeAttempt(t, claudeHistoryWithThinking, dto.ChannelSettings{PassThroughBodyEnabled: true}, 5)

		sent := a.run(t, claudeHistoryWithThinking, 5)

		assert.JSONEq(t, claudeHistoryWithThinking, string(sent))
		assert.Nil(t, cleanedReasoningLog(t, a.c))
	})

	t.Run("retry back to the last channel drops the stale count", func(t *testing.T) {
		a := newClaudeAttempt(t, claudeHistoryWithThinking, dto.ChannelSettings{PassThroughBodyEnabled: true}, 5)

		a.run(t, claudeHistoryWithThinking, 9)
		require.Equal(t, 2, cleanedReasoningLog(t, a.c))
		sent := a.run(t, claudeHistoryWithThinking, 5)

		assert.JSONEq(t, claudeHistoryWithThinking, string(sent))
		assert.Nil(t, cleanedReasoningLog(t, a.c))
	})

	t.Run("passthrough without thinking blocks records nothing", func(t *testing.T) {
		const plain = `{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"q"}]}`
		a := newClaudeAttempt(t, plain, dto.ChannelSettings{PassThroughBodyEnabled: true}, 5)

		sent := a.run(t, plain, 9)

		assert.JSONEq(t, plain, string(sent))
		assert.Nil(t, cleanedReasoningLog(t, a.c))
	})
}
