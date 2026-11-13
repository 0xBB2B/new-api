package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MoeForge/moego/ojson"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func claudeRequestFromJSON(t *testing.T, body string) *dto.ClaudeRequest {
	t.Helper()
	request := &dto.ClaudeRequest{}
	require.NoError(t, common.UnmarshalJsonStr(body, request))
	return request
}

type claudeAttempt struct {
	c        *gin.Context
	upstream *httptest.Server
	sent     chan []byte
}

func newClaudeAttempt(t *testing.T, raw string, setting dto.ChannelSettings) *claudeAttempt {
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
	smartroute.SetRouteResult(a.c, smartroute.RouteResult{})
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

const (
	claudeOriginalSignature = "sig_a"
	claudeOriginalRedacted  = "data_b"
)

func claudeHistory(signature, redacted string) string {
	return `{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"q"},` +
		`{"role":"assistant","content":[{"type":"thinking","thinking":"a","signature":"` + signature + `"},{"type":"text","text":"answer"}]},` +
		`{"role":"user","content":"q2"},` +
		`{"role":"assistant","content":[{"type":"redacted_thinking","data":"` + redacted + `"}]},` +
		`{"role":"user","content":"q3"}]}`
}

const claudeHistoryCleaned = `{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"q"},{"role":"assistant","content":[{"type":"text","text":"answer"}]},{"role":"user","content":"q2"},{"role":"user","content":"q3"}]}`

func TestClaudeHelperDecidesThinkingBlocksByTheirOwnChannelTag(t *testing.T) {
	taggedBy7 := claudeHistory(service.EncodeReasoningTag(7, claudeOriginalSignature), service.EncodeReasoningTag(7, claudeOriginalRedacted))
	restored := claudeHistory(claudeOriginalSignature, claudeOriginalRedacted)
	tampered := service.EncodeReasoningTag(7, claudeOriginalSignature)
	tampered = tampered[:len(tampered)-6] + "A" + tampered[len(tampered)-5:]
	if tampered == service.EncodeReasoningTag(7, claudeOriginalSignature) {
		tampered = tampered[:len(tampered)-6] + "B" + tampered[len(tampered)-5:]
	}

	cases := []struct {
		name       string
		history    string
		channelID  int
		want       string
		wantLogged any
	}{
		{"other channel cleans tagged blocks", taggedBy7, 6, claudeHistoryCleaned, 2},
		{"same channel restores signature and data", taggedBy7, 7, restored, nil},
		{"untagged blocks are cleaned", claudeHistory("plain_sig", "plain_data"), 7, claudeHistoryCleaned, 2},
		{"tampered tags are cleaned", claudeHistory(tampered, tampered), 7, claudeHistoryCleaned, 2},
	}
	modes := []struct {
		name   string
		global bool
		set    dto.ChannelSettings
	}{
		{"converted", false, dto.ChannelSettings{}},
		{"channel passthrough", false, dto.ChannelSettings{PassThroughBodyEnabled: true}},
		{"global passthrough", true, dto.ChannelSettings{}},
	}
	for _, mode := range modes {
		for _, tt := range cases {
			t.Run(mode.name+" "+tt.name, func(t *testing.T) {
				settings := model_setting.GetGlobalSettings()
				original := settings.PassThroughRequestEnabled
				t.Cleanup(func() { settings.PassThroughRequestEnabled = original })
				settings.PassThroughRequestEnabled = mode.global
				a := newClaudeAttempt(t, tt.history, mode.set)

				sent := a.run(t, tt.history, tt.channelID)

				assert.JSONEq(t, tt.want, string(sent))
				assert.Equal(t, tt.wantLogged, cleanedReasoningLog(t, a.c))
			})
		}
	}

	t.Run("route result does not change the decision", func(t *testing.T) {
		a := newClaudeAttempt(t, taggedBy7, dto.ChannelSettings{})
		smartroute.SetRouteResult(a.c, smartroute.RouteResult{})

		sent := a.run(t, taggedBy7, 6)

		assert.JSONEq(t, claudeHistoryCleaned, string(sent))
		a = newClaudeAttempt(t, taggedBy7, dto.ChannelSettings{})
		smartroute.SetRouteResult(a.c, smartroute.RouteResult{})
		sent = a.run(t, taggedBy7, 7)
		assert.JSONEq(t, restored, string(sent))
	})

	t.Run("retry onto the issuing channel drops the stale count", func(t *testing.T) {
		a := newClaudeAttempt(t, taggedBy7, dto.ChannelSettings{PassThroughBodyEnabled: true})

		a.run(t, taggedBy7, 6)
		require.Equal(t, 2, cleanedReasoningLog(t, a.c))
		sent := a.run(t, taggedBy7, 7)

		assert.JSONEq(t, restored, string(sent))
		assert.Nil(t, cleanedReasoningLog(t, a.c))
	})

	t.Run("passthrough keeps big integers and unknown fields byte for byte", func(t *testing.T) {
		sig := service.EncodeReasoningTag(7, claudeOriginalSignature)
		red := service.EncodeReasoningTag(7, claudeOriginalRedacted)
		tail := `{"type":"tool_use","id":"t1","name":"f","input":{"id":12345678901234567890},"extra_flag":true},{"type":"text","text":"answer","citations_hint":[1]}]},{"role":"user","content":"again"}]}`
		prefix := `{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"q"},{"role":"assistant","cache_hint":{"x":1},"content":[`
		raw := prefix + `{"type":"thinking","thinking":"a","signature":"` + sig + `"},{"type":"redacted_thinking","data":"` + red + `"},` + tail
		a := newClaudeAttempt(t, raw, dto.ChannelSettings{PassThroughBodyEnabled: true})

		sent := a.run(t, raw, 6)

		assert.Equal(t, prefix+tail, string(sent))
		assert.Equal(t, 2, cleanedReasoningLog(t, a.c))
	})

	t.Run("passthrough without thinking blocks is sent byte for byte", func(t *testing.T) {
		const plain = `{ "model":"claude-sonnet-4-5",  "max_tokens":64,"big":12345678901234567890,"messages":[{"role":"user","content":"q"}]}`
		a := newClaudeAttempt(t, plain, dto.ChannelSettings{PassThroughBodyEnabled: true})

		sent := a.run(t, plain, 9)

		assert.Equal(t, plain, string(sent))
		assert.Nil(t, cleanedReasoningLog(t, a.c))
	})
}

func TestClaudeHelperDisablesThinkingWhenLastTurnLosesItsThinkingBlock(t *testing.T) {
	const toolTurn = `{"type":"tool_use","id":"t1","name":"f","input":{}}`
	const toolResult = `{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}`
	signature := service.EncodeReasoningTag(7, claudeOriginalSignature)
	build := func(thinking, lastAssistant string) string {
		return `{"model":"claude-sonnet-4-5","max_tokens":4096,"thinking":` + thinking + `,"output_config":{"effort":"high"},` +
			`"messages":[{"role":"user","content":"q"},{"role":"assistant","content":[` + lastAssistant + `]},` + toolResult + `]}`
	}
	const enabled = `{"type":"enabled","budget_tokens":2048}`
	const adaptive = `{"type":"adaptive"}`
	withThinking := `{"type":"thinking","thinking":"a","signature":"` + signature + `"},` + toolTurn

	for _, mode := range []struct {
		name    string
		setting dto.ChannelSettings
	}{
		{"converted", dto.ChannelSettings{}},
		{"channel passthrough", dto.ChannelSettings{PassThroughBodyEnabled: true}},
	} {
		t.Run(mode.name+" enabled thinking is disabled", func(t *testing.T) {
			raw := build(enabled, withThinking)
			a := newClaudeAttempt(t, raw, mode.setting)

			sent := a.run(t, raw, 6)

			assert.JSONEq(t, `{"type":"disabled"}`, string(ojson.GetBytes(sent, "thinking").Raw))
			assert.False(t, ojson.GetBytes(sent, "output_config.effort").Exists(), string(sent))
		})

		t.Run(mode.name+" thinking blocks from the target channel keep enabled thinking", func(t *testing.T) {
			raw := build(enabled, withThinking)
			a := newClaudeAttempt(t, raw, mode.setting)

			sent := a.run(t, raw, 7)

			assert.Equal(t, "enabled", ojson.GetBytes(sent, "thinking.type").String())
			assert.Equal(t, "high", ojson.GetBytes(sent, "output_config.effort").String())
			assert.Equal(t, claudeOriginalSignature, ojson.GetBytes(sent, "messages.1.content.0.signature").String())
		})

		t.Run(mode.name+" adaptive thinking is untouched", func(t *testing.T) {
			raw := build(adaptive, withThinking)
			a := newClaudeAttempt(t, raw, mode.setting)

			sent := a.run(t, raw, 6)

			assert.Equal(t, "adaptive", ojson.GetBytes(sent, "thinking.type").String())
			assert.Equal(t, "high", ojson.GetBytes(sent, "output_config.effort").String())
		})

		t.Run(mode.name+" thinking is disabled when the first assistant message of the turn loses its thinking block", func(t *testing.T) {
			raw := `{"model":"claude-sonnet-4-5","max_tokens":4096,"thinking":` + enabled + `,"output_config":{"effort":"high"},` +
				`"messages":[{"role":"user","content":"q"},{"role":"assistant","content":[` + withThinking + `]},` + toolResult +
				`,{"role":"assistant","content":[{"type":"tool_use","id":"t2","name":"f","input":{}}]},` +
				`{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"ok"}]}]}`
			a := newClaudeAttempt(t, raw, mode.setting)

			sent := a.run(t, raw, 6)

			assert.JSONEq(t, `{"type":"disabled"}`, string(ojson.GetBytes(sent, "thinking").Raw))
			assert.False(t, ojson.GetBytes(sent, "output_config.effort").Exists(), string(sent))
		})

		t.Run(mode.name+" a new user question keeps enabled thinking after the history is cleaned", func(t *testing.T) {
			raw := `{"model":"claude-sonnet-4-5","max_tokens":4096,"thinking":` + enabled + `,"output_config":{"effort":"high"},` +
				`"messages":[{"role":"user","content":"q"},{"role":"assistant","content":[{"type":"thinking","thinking":"a","signature":"` + signature + `"},{"type":"text","text":"answer"}]},` +
				`{"role":"user","content":"q2"}]}`
			a := newClaudeAttempt(t, raw, mode.setting)

			sent := a.run(t, raw, 6)

			assert.Equal(t, "enabled", ojson.GetBytes(sent, "thinking.type").String())
			assert.Equal(t, int64(2048), ojson.GetBytes(sent, "thinking.budget_tokens").Int())
			assert.Equal(t, "high", ojson.GetBytes(sent, "output_config.effort").String())
			assert.NotContains(t, string(sent), `"thinking":"a"`)
		})

		t.Run(mode.name+" last turn without thinking blocks keeps enabled thinking", func(t *testing.T) {
			raw := `{"model":"claude-sonnet-4-5","max_tokens":4096,"thinking":` + enabled + `,"output_config":{"effort":"high"},` +
				`"messages":[{"role":"user","content":"q"},{"role":"assistant","content":[{"type":"thinking","thinking":"a","signature":"` + signature + `"},{"type":"text","text":"answer"}]},` +
				`{"role":"user","content":"again"},{"role":"assistant","content":[` + toolTurn + `]},` + toolResult + `]}`
			a := newClaudeAttempt(t, raw, mode.setting)

			sent := a.run(t, raw, 6)

			assert.Equal(t, "enabled", ojson.GetBytes(sent, "thinking.type").String())
			assert.Equal(t, int64(2048), ojson.GetBytes(sent, "thinking.budget_tokens").Int())
		})
	}
}
