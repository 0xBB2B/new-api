package smartroute_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	responsesEndpoint = "openai-response"
	chatEndpoint      = "openai"
)

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestInspectRequest_UserTurn(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		body     string
		wantUser bool
		wantMsgs []string
	}{
		{
			name:     "responses input string is a user turn",
			endpoint: responsesEndpoint,
			body:     `{"model":"m","input":"你好"}`,
			wantUser: true,
			wantMsgs: []string{"你好"},
		},
		{
			name:     "responses last item function_call_output is tool resume",
			endpoint: responsesEndpoint,
			body: `{"input":[
				{"type":"message","role":"user","content":[{"type":"input_text","text":"查天气"}]},
				{"type":"function_call","call_id":"c1","name":"w","arguments":"{}"},
				{"type":"function_call_output","call_id":"c1","output":"晴"}]}`,
			wantUser: false,
			wantMsgs: []string{"查天气"},
		},
		{
			name:     "responses custom_tool_call_output is tool resume",
			endpoint: responsesEndpoint,
			body: `{"input":[
				{"type":"message","role":"user","content":"a"},
				{"type":"custom_tool_call_output","call_id":"c1","output":"x"}]}`,
			wantUser: false,
			wantMsgs: []string{"a"},
		},
		{
			name:     "responses local_shell_call_output is tool resume",
			endpoint: responsesEndpoint,
			body: `{"input":[
				{"type":"message","role":"user","content":"a"},
				{"type":"local_shell_call_output","call_id":"c1","output":"x"}]}`,
			wantUser: false,
			wantMsgs: []string{"a"},
		},
		{
			name:     "responses mcp_call_output is tool resume",
			endpoint: responsesEndpoint,
			body: `{"input":[
				{"type":"message","role":"user","content":"a"},
				{"type":"mcp_call_output","call_id":"c1","output":"x"}]}`,
			wantUser: false,
			wantMsgs: []string{"a"},
		},
		{
			name:     "responses last item user message is a user turn",
			endpoint: responsesEndpoint,
			body: `{"input":[
				{"type":"message","role":"user","content":[{"type":"input_text","text":"第一句"}]},
				{"type":"message","role":"assistant","content":[{"type":"output_text","text":"好的"}]},
				{"type":"message","role":"user","content":[{"type":"input_text","text":"第二句"}]}]}`,
			wantUser: true,
			wantMsgs: []string{"第一句", "第二句"},
		},
		{
			name:     "responses user message without type field",
			endpoint: responsesEndpoint,
			body:     `{"input":[{"role":"user","content":"裸写法"}]}`,
			wantUser: true,
			wantMsgs: []string{"裸写法"},
		},
		{
			name:     "responses last item assistant is not scored",
			endpoint: responsesEndpoint,
			body: `{"input":[
				{"role":"user","content":"q"},
				{"type":"message","role":"assistant","content":[{"type":"output_text","text":"a"}]}]}`,
			wantUser: false,
			wantMsgs: []string{"q"},
		},
		{
			name:     "chat last role user is a user turn",
			endpoint: chatEndpoint,
			body:     `{"messages":[{"role":"user","content":"hi"}]}`,
			wantUser: true,
			wantMsgs: []string{"hi"},
		},
		{
			name:     "chat last role tool is tool resume",
			endpoint: chatEndpoint,
			body: `{"messages":[
				{"role":"user","content":"查天气"},
				{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"w","arguments":"{}"}}]},
				{"role":"tool","tool_call_id":"c1","content":"晴"}]}`,
			wantUser: false,
			wantMsgs: []string{"查天气"},
		},
		{
			name:     "chat legacy role function is tool resume",
			endpoint: chatEndpoint,
			body: `{"messages":[
				{"role":"user","content":"q"},
				{"role":"function","name":"w","content":"晴"}]}`,
			wantUser: false,
			wantMsgs: []string{"q"},
		},
		{
			name:     "chat last role assistant is not scored",
			endpoint: chatEndpoint,
			body: `{"messages":[
				{"role":"user","content":"q"},
				{"role":"assistant","content":"a"}]}`,
			wantUser: false,
			wantMsgs: []string{"q"},
		},
		{
			name:     "chat array content concatenates text parts only",
			endpoint: chatEndpoint,
			body: `{"messages":[{"role":"user","content":[
				{"type":"text","text":"ab"},
				{"type":"image_url","image_url":{"url":"http://x/y.png"}},
				{"type":"text","text":"cd"}]}]}`,
			wantUser: true,
			wantMsgs: []string{"abcd"},
		},
		{
			name:     "responses array content concatenates input_text parts only",
			endpoint: responsesEndpoint,
			body: `{"input":[{"role":"user","content":[
				{"type":"input_text","text":"ab"},
				{"type":"input_image","image_url":"http://x/y.png"},
				{"type":"input_text","text":"cd"}]}]}`,
			wantUser: true,
			wantMsgs: []string{"abcd"},
		},
		{
			name:     "chat empty messages has no user messages",
			endpoint: chatEndpoint,
			body:     `{"messages":[]}`,
			wantUser: false,
			wantMsgs: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := smartroute.InspectRequest(tt.endpoint, []byte(tt.body))
			require.NoError(t, err)
			assert.Equal(t, tt.wantUser, got.IsUserTurn)
			assert.Equal(t, tt.wantMsgs, got.UserMessages)
			assert.Equal(t, len(tt.wantMsgs), got.Features.TurnCount)
		})
	}
}

func TestInspectRequest_InvalidJSON(t *testing.T) {
	for _, endpoint := range []string{responsesEndpoint, chatEndpoint} {
		t.Run(endpoint, func(t *testing.T) {
			_, err := smartroute.InspectRequest(endpoint, []byte(`{"input":`))
			assert.Error(t, err)
		})
	}
}

func TestInspectRequest_ExcludesNonUserContent(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		body     string
	}{
		{
			name:     "responses",
			endpoint: responsesEndpoint,
			body: `{"instructions":"SECRET-INSTRUCTIONS","input":[
				{"role":"system","content":"SECRET-SYSTEM"},
				{"role":"developer","content":[{"type":"input_text","text":"SECRET-DEVELOPER"}]},
				{"role":"user","content":"用户问题"},
				{"type":"reasoning","summary":[{"type":"summary_text","text":"SECRET-REASONING"}]},
				{"type":"function_call","call_id":"c1","name":"w","arguments":"SECRET-ARGS"},
				{"type":"function_call_output","call_id":"c1","output":"SECRET-TOOL-RESULT"}]}`,
		},
		{
			name:     "chat",
			endpoint: chatEndpoint,
			body: `{"messages":[
				{"role":"system","content":"SECRET-SYSTEM"},
				{"role":"developer","content":"SECRET-DEVELOPER"},
				{"role":"user","content":"用户问题"},
				{"role":"assistant","content":"SECRET-ASSISTANT","reasoning_content":"SECRET-REASONING"},
				{"role":"tool","tool_call_id":"c1","content":"SECRET-TOOL-RESULT"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := smartroute.InspectRequest(tt.endpoint, []byte(tt.body))
			require.NoError(t, err)
			assert.Equal(t, []string{"用户问题"}, got.UserMessages)
			input := smartroute.BuildScorerInput(got, 5)
			assert.NotContains(t, fmt.Sprint(input), "SECRET")
		})
	}
}

func TestInspectRequest_AttachmentOnlyLooksAtLatestMessage(t *testing.T) {
	body := `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"x"}},{"type":"text","text":"看图"}]},{"role":"assistant","content":"a"},{"role":"user","content":"继续"}]}`
	in, err := smartroute.InspectRequest(chatEndpoint, []byte(body))
	require.NoError(t, err)
	assert.False(t, in.Features.HasAttachment)
}

func TestInspectRequest_LocalFeatures(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		body           string
		wantRunes      int
		wantTurns      int
		wantAttachment bool
		wantCodeTable  bool
	}{
		{
			name:      "plain text counts runes not bytes",
			endpoint:  responsesEndpoint,
			body:      `{"input":"你好abc"}`,
			wantRunes: 5,
			wantTurns: 1,
		},
		{
			name:      "turns count user messages only",
			endpoint:  chatEndpoint,
			body:      `{"messages":[{"role":"system","content":"s"},{"role":"user","content":"a"},{"role":"assistant","content":"b"},{"role":"user","content":"cc"}]}`,
			wantRunes: 2,
			wantTurns: 2,
		},
		{
			name:           "responses input_image",
			endpoint:       responsesEndpoint,
			body:           `{"input":[{"role":"user","content":[{"type":"input_text","text":"看图"},{"type":"input_image","image_url":"http://x/y.png"}]}]}`,
			wantRunes:      2,
			wantTurns:      1,
			wantAttachment: true,
		},
		{
			name:           "responses input_file",
			endpoint:       responsesEndpoint,
			body:           `{"input":[{"role":"user","content":[{"type":"input_text","text":"看"},{"type":"input_file","file_id":"f1"}]}]}`,
			wantRunes:      1,
			wantTurns:      1,
			wantAttachment: true,
		},
		{
			name:           "chat image_url",
			endpoint:       chatEndpoint,
			body:           `{"messages":[{"role":"user","content":[{"type":"text","text":"看"},{"type":"image_url","image_url":{"url":"http://x/y.png"}}]}]}`,
			wantRunes:      1,
			wantTurns:      1,
			wantAttachment: true,
		},
		{
			name:           "chat file",
			endpoint:       chatEndpoint,
			body:           `{"messages":[{"role":"user","content":[{"type":"text","text":"看"},{"type":"file","file":{"file_id":"f1"}}]}]}`,
			wantRunes:      1,
			wantTurns:      1,
			wantAttachment: true,
		},
		{
			name:          "code fence in latest message",
			endpoint:      chatEndpoint,
			body:          "{\"messages\":[{\"role\":\"user\",\"content\":\"改一下\\n```go\\nx := 1\\n```\"}]}",
			wantRunes:     len([]rune("改一下\n```go\nx := 1\n```")),
			wantTurns:     1,
			wantCodeTable: true,
		},
		{
			name:          "two table rows",
			endpoint:      responsesEndpoint,
			body:          `{"input":"看表\n|a|b|\n|1|2|"}`,
			wantRunes:     len([]rune("看表\n|a|b|\n|1|2|")),
			wantTurns:     1,
			wantCodeTable: true,
		},
		{
			name:      "single pipe row is not a table",
			endpoint:  responsesEndpoint,
			body:      `{"input":"看\n|a|b|"}`,
			wantRunes: len([]rune("看\n|a|b|")),
			wantTurns: 1,
		},
		{
			name:      "pipes not at line start are not a table",
			endpoint:  responsesEndpoint,
			body:      `{"input":"a|b|\nc|d|"}`,
			wantRunes: len([]rune("a|b|\nc|d|")),
			wantTurns: 1,
		},
		{
			name:      "code fence in an earlier message does not count",
			endpoint:  chatEndpoint,
			body:      "{\"messages\":[{\"role\":\"user\",\"content\":\"```x```\"},{\"role\":\"assistant\",\"content\":\"ok\"},{\"role\":\"user\",\"content\":\"谢谢\"}]}",
			wantRunes: 2,
			wantTurns: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := smartroute.InspectRequest(tt.endpoint, []byte(tt.body))
			require.NoError(t, err)
			assert.Equal(t, tt.wantRunes, got.Features.LatestRuneCount)
			assert.Equal(t, tt.wantTurns, got.Features.TurnCount)
			assert.Equal(t, tt.wantAttachment, got.Features.HasAttachment)
			assert.Equal(t, tt.wantCodeTable, got.Features.HasCodeOrTable)
		})
	}
}

func TestInspectRequest_PromptCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		body     string
		want     string
	}{
		{"responses with key", responsesEndpoint, `{"prompt_cache_key":"k1","input":"hi"}`, "k1"},
		{"chat with key", chatEndpoint, `{"prompt_cache_key":"k2","messages":[{"role":"user","content":"hi"}]}`, "k2"},
		{"responses without key", responsesEndpoint, `{"input":"hi"}`, ""},
		{"non-string key is ignored", chatEndpoint, `{"prompt_cache_key":123,"messages":[{"role":"user","content":"hi"}]}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := smartroute.InspectRequest(tt.endpoint, []byte(tt.body))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.PromptCacheKey)
		})
	}
}

func TestBuildScorerInput(t *testing.T) {
	long := func(n int, r string) string { return strings.Repeat(r, n) }
	chatBody := func(msgs ...string) string {
		parts := make([]string, len(msgs))
		for i, m := range msgs {
			parts[i] = fmt.Sprintf(`{"role":"user","content":"%s"},{"role":"assistant","content":"ok"}`, m)
		}
		last := parts[len(parts)-1]
		parts[len(parts)-1] = last[:strings.LastIndex(last, ",{")]
		return `{"messages":[` + strings.Join(parts, ",") + `]}`
	}

	t.Run("latest over 4000 runes is cut to 4000", func(t *testing.T) {
		in, err := smartroute.InspectRequest(chatEndpoint, []byte(chatBody(long(5000, "字"))))
		require.NoError(t, err)
		got := smartroute.BuildScorerInput(in, 2)
		assert.Equal(t, long(4000, "字"), got.Latest)
		assert.Empty(t, got.History)
	})

	t.Run("latest at exactly 4000 runes is kept whole", func(t *testing.T) {
		in, err := smartroute.InspectRequest(chatEndpoint, []byte(chatBody(long(4000, "字"))))
		require.NoError(t, err)
		assert.Equal(t, long(4000, "字"), smartroute.BuildScorerInput(in, 2).Latest)
	})

	t.Run("history takes the N most recent earlier messages, oldest first, each cut to 200 runes", func(t *testing.T) {
		in, err := smartroute.InspectRequest(chatEndpoint, []byte(chatBody(
			"m1", long(300, "甲"), long(300, "乙"), "m4-latest")))
		require.NoError(t, err)
		got := smartroute.BuildScorerInput(in, 2)
		assert.Equal(t, "m4-latest", got.Latest)
		assert.Equal(t, []string{long(200, "甲"), long(200, "乙")}, got.History)
	})

	t.Run("fewer earlier messages than N returns what exists", func(t *testing.T) {
		in, err := smartroute.InspectRequest(chatEndpoint, []byte(chatBody("early", "late")))
		require.NoError(t, err)
		got := smartroute.BuildScorerInput(in, 5)
		assert.Equal(t, "late", got.Latest)
		assert.Equal(t, []string{"early"}, got.History)
	})

	t.Run("N zero returns latest only", func(t *testing.T) {
		in, err := smartroute.InspectRequest(chatEndpoint, []byte(chatBody("early", "late")))
		require.NoError(t, err)
		got := smartroute.BuildScorerInput(in, 0)
		assert.Equal(t, "late", got.Latest)
		assert.Empty(t, got.History)
	})
}

func TestSessionKey(t *testing.T) {
	inspect := func(t *testing.T, endpoint, body string) smartroute.Inspection {
		t.Helper()
		in, err := smartroute.InspectRequest(endpoint, []byte(body))
		require.NoError(t, err)
		return in
	}

	t.Run("first user message without text gives no session key", func(t *testing.T) {
		in := inspect(t, chatEndpoint, `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"x"}}]},{"role":"assistant","content":"a"},{"role":"user","content":"再看看"}]}`)
		assert.Empty(t, smartroute.SessionKey(7, in))
	})

	t.Run("prompt_cache_key hashes token id and key", func(t *testing.T) {
		in := inspect(t, responsesEndpoint, `{"prompt_cache_key":"k1","input":"hi"}`)
		assert.Equal(t, sha256Hex("7|pck|k1"), smartroute.SessionKey(7, in))
	})

	t.Run("without key hashes token id and first user message", func(t *testing.T) {
		in := inspect(t, chatEndpoint, `{"messages":[{"role":"user","content":"第一条"},{"role":"assistant","content":"a"},{"role":"user","content":"第二条"}]}`)
		assert.Equal(t, sha256Hex("7|first|第一条"), smartroute.SessionKey(7, in))
	})

	t.Run("same token and key give the same key across different messages and endpoints", func(t *testing.T) {
		a := inspect(t, responsesEndpoint, `{"prompt_cache_key":"k1","input":"甲"}`)
		b := inspect(t, chatEndpoint, `{"prompt_cache_key":"k1","messages":[{"role":"user","content":"乙"}]}`)
		assert.Equal(t, smartroute.SessionKey(7, a), smartroute.SessionKey(7, b))
	})

	t.Run("same first message gives the same key as the conversation grows", func(t *testing.T) {
		a := inspect(t, chatEndpoint, `{"messages":[{"role":"user","content":"开头"}]}`)
		b := inspect(t, chatEndpoint, `{"messages":[{"role":"user","content":"开头"},{"role":"assistant","content":"a"},{"role":"user","content":"后续"}]}`)
		assert.Equal(t, smartroute.SessionKey(7, a), smartroute.SessionKey(7, b))
	})

	t.Run("different first message gives different keys", func(t *testing.T) {
		a := inspect(t, chatEndpoint, `{"messages":[{"role":"user","content":"甲"}]}`)
		b := inspect(t, chatEndpoint, `{"messages":[{"role":"user","content":"乙"}]}`)
		assert.NotEqual(t, smartroute.SessionKey(7, a), smartroute.SessionKey(7, b))
	})

	t.Run("different tokens give different keys", func(t *testing.T) {
		withKey := inspect(t, responsesEndpoint, `{"prompt_cache_key":"k1","input":"hi"}`)
		noKey := inspect(t, responsesEndpoint, `{"input":"hi"}`)
		assert.NotEqual(t, smartroute.SessionKey(7, withKey), smartroute.SessionKey(8, withKey))
		assert.NotEqual(t, smartroute.SessionKey(7, noKey), smartroute.SessionKey(8, noKey))
	})

	t.Run("no user messages and no key returns empty", func(t *testing.T) {
		in := inspect(t, chatEndpoint, `{"messages":[{"role":"system","content":"s"}]}`)
		assert.Empty(t, smartroute.SessionKey(7, in))
	})
}

func TestRequestEndpointFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/v1/responses", responsesEndpoint},
		{"/v1/chat/completions", chatEndpoint},
		{"/v1/messages", ""},
		{"/v1/embeddings", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, smartroute.RequestEndpointFromPath(tt.path))
		})
	}
}

const validScorerJSON = `{"task_type":"lookup","reasoning":"low","workload":"one_step","risk":"low","domain":"code","confidence":0.9,"feedback":["neutral","satisfied"]}`

func TestParseScorerOutput(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantFeedback int
		wantErr      bool
	}{
		{name: "valid", raw: validScorerJSON, wantFeedback: 2},
		{name: "wrapped in json code fence", raw: "```json\n" + validScorerJSON + "\n```", wantFeedback: 2},
		{name: "confidence 0 is valid", raw: strings.Replace(validScorerJSON, "0.9", "0", 1), wantFeedback: 2},
		{name: "confidence 1 is valid", raw: strings.Replace(validScorerJSON, "0.9", "1", 1), wantFeedback: 2},
		{name: "unknown task_type", raw: strings.Replace(validScorerJSON, "lookup", "poem", 1), wantFeedback: 2, wantErr: true},
		{name: "unknown reasoning", raw: strings.Replace(validScorerJSON, `"low","workload"`, `"extreme","workload"`, 1), wantFeedback: 2, wantErr: true},
		{name: "unknown workload", raw: strings.Replace(validScorerJSON, "one_step", "zero_step", 1), wantFeedback: 2, wantErr: true},
		{name: "unknown risk", raw: strings.Replace(validScorerJSON, `"risk":"low"`, `"risk":"medium"`, 1), wantFeedback: 2, wantErr: true},
		{name: "unknown domain", raw: strings.Replace(validScorerJSON, `"code"`, `"legal"`, 1), wantFeedback: 2, wantErr: true},
		{name: "domain finance is valid", raw: strings.Replace(validScorerJSON, `"code"`, `"finance"`, 1), wantFeedback: 2},
		{name: "domain operations is valid", raw: strings.Replace(validScorerJSON, `"code"`, `"operations"`, 1), wantFeedback: 2},
		{name: "domain data_analysis is valid", raw: strings.Replace(validScorerJSON, `"code"`, `"data_analysis"`, 1), wantFeedback: 2},
		{name: "domain writing is not an option", raw: strings.Replace(validScorerJSON, `"code"`, `"writing"`, 1), wantFeedback: 2, wantErr: true},
		{name: "confidence above 1", raw: strings.Replace(validScorerJSON, "0.9", "1.2", 1), wantFeedback: 2, wantErr: true},
		{name: "confidence below 0", raw: strings.Replace(validScorerJSON, "0.9", "-0.1", 1), wantFeedback: 2, wantErr: true},
		{name: "confidence missing", raw: `{"task_type":"lookup","reasoning":"low","workload":"one_step","risk":"low","domain":"code","feedback":["neutral","satisfied"]}`, wantFeedback: 2, wantErr: true},
		{name: "feedback shorter than expected", raw: validScorerJSON, wantFeedback: 3, wantErr: true},
		{name: "feedback longer than expected", raw: validScorerJSON, wantFeedback: 1, wantErr: true},
		{name: "feedback value unknown", raw: strings.Replace(validScorerJSON, "satisfied", "happy", 1), wantFeedback: 2, wantErr: true},
		{name: "feedback missing", raw: `{"task_type":"lookup","reasoning":"low","workload":"one_step","risk":"low","domain":"code","confidence":0.9}`, wantFeedback: 2, wantErr: true},
		{name: "not json", raw: "I think this is easy", wantFeedback: 2, wantErr: true},
		{name: "empty", raw: "", wantFeedback: 2, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels, err := smartroute.ParseScorerOutput(tt.raw, tt.wantFeedback)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, labels.Feedback, tt.wantFeedback)
		})
	}
}

func TestParseScorerOutput_Fields(t *testing.T) {
	labels, err := smartroute.ParseScorerOutput(validScorerJSON, 2)
	require.NoError(t, err)
	assert.Equal(t, smartroute.ScorerLabels{
		TaskType:   "lookup",
		Reasoning:  "low",
		Workload:   "one_step",
		Risk:       "low",
		Domain:     "code",
		Confidence: 0.9,
		Feedback:   []string{"neutral", "satisfied"},
	}, labels)
}

func smartRouteCfg() operation_setting.SmartRoute {
	return operation_setting.SmartRoute{
		Name:                      "r",
		DefaultTier:               operation_setting.SmartTierNormal,
		ConfidenceThreshold:       0.6,
		SimpleMaxMessageChars:     2000,
		DowngradeMaxHistoryTokens: 20000,
		Advisor:                   operation_setting.SmartRouteAdvisor{AfterDissatisfied: 2},
	}
}

func labelsOf(taskType, reasoning, workload, risk string, confidence float64) smartroute.ScorerLabels {
	return smartroute.ScorerLabels{TaskType: taskType, Reasoning: reasoning, Workload: workload, Risk: risk, Domain: "other", Confidence: confidence}
}

func TestSelectTier(t *testing.T) {
	tests := []struct {
		name        string
		labels      smartroute.ScorerLabels
		latestChars int
		wantTier    string
		wantReason  string
		wantMatched []string
	}{
		{name: "low confidence uses default tier", labels: labelsOf("plan", "high", "many_steps", "high", 0.5), latestChars: 10, wantTier: "normal", wantReason: "low_confidence"},
		{name: "confidence equal to threshold is trusted", labels: labelsOf("chat_qa", "low", "one_step", "low", 0.6), latestChars: 10, wantTier: "simple", wantReason: "rule_simple"},
		{name: "risk high", labels: labelsOf("lookup", "low", "one_step", "high", 0.9), latestChars: 10, wantTier: "hard", wantReason: "rule_hard", wantMatched: []string{"risk"}},
		{name: "task_type plan", labels: labelsOf("plan", "low", "one_step", "low", 0.9), latestChars: 10, wantTier: "hard", wantReason: "rule_hard", wantMatched: []string{"task_type"}},
		{name: "reasoning high", labels: labelsOf("analyze", "high", "few_steps", "low", 0.9), latestChars: 10, wantTier: "hard", wantReason: "rule_hard", wantMatched: []string{"reasoning"}},
		{name: "several hard conditions all recorded", labels: labelsOf("plan", "high", "many_steps", "high", 0.9), latestChars: 10, wantTier: "hard", wantReason: "rule_hard", wantMatched: []string{"reasoning", "risk", "task_type"}},
		{name: "hard wins over simple-looking labels", labels: labelsOf("lookup", "high", "one_step", "low", 0.9), latestChars: 10, wantTier: "hard", wantReason: "rule_hard", wantMatched: []string{"reasoning"}},
		{name: "rewrite short is simple", labels: labelsOf("rewrite", "low", "one_step", "low", 0.9), latestChars: 1500, wantTier: "simple", wantReason: "rule_simple"},
		{name: "lookup short is simple", labels: labelsOf("lookup", "low", "one_step", "low", 0.9), latestChars: 1, wantTier: "simple", wantReason: "rule_simple"},
		{name: "one char below limit is simple", labels: labelsOf("rewrite", "low", "one_step", "low", 0.9), latestChars: 1999, wantTier: "simple", wantReason: "rule_simple"},
		{name: "length equal to limit is normal", labels: labelsOf("rewrite", "low", "one_step", "low", 0.9), latestChars: 2000, wantTier: "normal", wantReason: "rule_normal"},
		{name: "rewrite long is normal", labels: labelsOf("rewrite", "low", "one_step", "low", 0.9), latestChars: 2500, wantTier: "normal", wantReason: "rule_normal"},
		{name: "lookup many steps is normal", labels: labelsOf("lookup", "low", "many_steps", "low", 0.9), latestChars: 10, wantTier: "normal", wantReason: "rule_normal"},
		{name: "lookup medium reasoning is normal", labels: labelsOf("lookup", "medium", "one_step", "low", 0.9), latestChars: 10, wantTier: "normal", wantReason: "rule_normal"},
		{name: "generate is never simple", labels: labelsOf("generate", "low", "one_step", "low", 0.9), latestChars: 10, wantTier: "normal", wantReason: "rule_normal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := smartroute.SelectTier(smartRouteCfg(), tt.labels, tt.latestChars)
			assert.Equal(t, tt.wantTier, got.Tier)
			assert.Equal(t, tt.wantReason, got.Reason)
			if tt.wantMatched != nil {
				assert.ElementsMatch(t, tt.wantMatched, got.Params["matched"])
			}
		})
	}
}

func TestCountTrailingDissatisfied(t *testing.T) {
	tests := []struct {
		name     string
		feedback []string
		want     int
	}{
		{name: "nil", feedback: nil, want: 0},
		{name: "none", feedback: []string{"satisfied", "neutral"}, want: 0},
		{name: "trailing two", feedback: []string{"neutral", "dissatisfied", "dissatisfied"}, want: 2},
		{name: "interrupted by neutral", feedback: []string{"dissatisfied", "neutral", "dissatisfied"}, want: 1},
		{name: "all", feedback: []string{"dissatisfied", "dissatisfied", "dissatisfied"}, want: 3},
		{name: "latest satisfied", feedback: []string{"dissatisfied", "dissatisfied", "satisfied"}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, smartroute.CountTrailingDissatisfied(tt.feedback))
		})
	}
}

func TestApplyDowngradeGuard(t *testing.T) {
	scored := func(tier string) smartroute.TierChoice {
		return smartroute.TierChoice{Tier: tier, Reason: "rule_simple"}
	}
	tests := []struct {
		name           string
		choice         smartroute.TierChoice
		lastNonAdvisor string
		tokens         int
		wantTier       string
		wantReason     string
	}{
		{name: "no session record", choice: scored("simple"), lastNonAdvisor: "", tokens: 86000, wantTier: "simple", wantReason: "rule_simple"},
		{name: "upgrade ignores token count", choice: smartroute.TierChoice{Tier: "hard", Reason: "rule_hard"}, lastNonAdvisor: "normal", tokens: 86000, wantTier: "hard", wantReason: "rule_hard"},
		{name: "same tier", choice: smartroute.TierChoice{Tier: "normal", Reason: "rule_normal"}, lastNonAdvisor: "normal", tokens: 86000, wantTier: "normal", wantReason: "rule_normal"},
		{name: "downgrade below limit", choice: scored("simple"), lastNonAdvisor: "hard", tokens: 15000, wantTier: "simple", wantReason: "rule_simple"},
		{name: "downgrade one below limit", choice: scored("simple"), lastNonAdvisor: "hard", tokens: 19999, wantTier: "simple", wantReason: "rule_simple"},
		{name: "downgrade at limit is blocked", choice: scored("simple"), lastNonAdvisor: "hard", tokens: 20000, wantTier: "hard", wantReason: "downgrade_blocked"},
		{name: "downgrade above limit is blocked", choice: scored("simple"), lastNonAdvisor: "hard", tokens: 86000, wantTier: "hard", wantReason: "downgrade_blocked"},
		{name: "hard to normal blocked", choice: smartroute.TierChoice{Tier: "normal", Reason: "rule_normal"}, lastNonAdvisor: "hard", tokens: 86000, wantTier: "hard", wantReason: "downgrade_blocked"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := smartroute.ApplyDowngradeGuard(smartRouteCfg(), tt.choice, tt.lastNonAdvisor, tt.tokens)
			assert.Equal(t, tt.wantTier, got.Tier)
			assert.Equal(t, tt.wantReason, got.Reason)
		})
	}
}

func TestApplyDowngradeGuard_BlockedParams(t *testing.T) {
	got := smartroute.ApplyDowngradeGuard(smartRouteCfg(), smartroute.TierChoice{Tier: "simple", Reason: "rule_simple"}, "hard", 86000)
	assert.Equal(t, map[string]any{
		"history_tokens": 86000,
		"limit":          20000,
		"kept_tier":      "hard",
		"scored_tier":    "simple",
	}, got.Params)
}

func TestDecideRoute(t *testing.T) {
	simpleLabels := labelsOf("rewrite", "low", "one_step", "low", 0.9)
	withFeedback := func(l smartroute.ScorerLabels, fb ...string) *smartroute.ScorerLabels {
		l.Feedback = fb
		return &l
	}
	tests := []struct {
		name       string
		in         smartroute.DecideInput
		wantSel    string
		wantReason string
		wantScored string
		wantParams map[string]any
	}{
		{
			name:    "no session uses scored tier",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, Labels: withFeedback(simpleLabels, "neutral"), InputTokens: 86000, LatestChars: 100},
			wantSel: "simple", wantReason: "rule_simple", wantScored: "simple",
		},
		{
			name:    "low confidence default tier",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, Labels: withFeedback(labelsOf("plan", "high", "many_steps", "high", 0.5), "neutral"), LatestChars: 100},
			wantSel: "normal", wantReason: "low_confidence", wantScored: "normal",
		},
		{
			name:    "downgrade allowed under limit",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, Labels: withFeedback(simpleLabels, "neutral"), Session: &smartroute.SessionRecord{CurrentRoute: "hard", LastNonAdvisor: "hard"}, InputTokens: 15000, LatestChars: 100},
			wantSel: "simple", wantReason: "rule_simple", wantScored: "simple",
		},
		{
			name:    "downgrade blocked over limit",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, Labels: withFeedback(simpleLabels, "neutral"), Session: &smartroute.SessionRecord{CurrentRoute: "hard", LastNonAdvisor: "hard"}, InputTokens: 86000, LatestChars: 100},
			wantSel: "hard", wantReason: "downgrade_blocked", wantScored: "simple",
			wantParams: map[string]any{"history_tokens": 86000, "limit": 20000, "kept_tier": "hard", "scored_tier": "simple"},
		},
		{
			name:    "after advisor compares with tier before advisor",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, Labels: withFeedback(simpleLabels, "neutral"), Session: &smartroute.SessionRecord{CurrentRoute: "advisor", LastNonAdvisor: "normal"}, InputTokens: 86000, LatestChars: 100},
			wantSel: "normal", wantReason: "downgrade_blocked", wantScored: "simple",
			wantParams: map[string]any{"history_tokens": 86000, "limit": 20000, "kept_tier": "normal", "scored_tier": "simple"},
		},
		{
			name:    "advisor after consecutive dissatisfied",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, Labels: withFeedback(simpleLabels, "neutral", "dissatisfied", "dissatisfied"), Session: &smartroute.SessionRecord{CurrentRoute: "normal", LastNonAdvisor: "normal"}, InputTokens: 100, LatestChars: 100},
			wantSel: "advisor", wantReason: "advisor", wantScored: "simple",
			wantParams: map[string]any{"count": 2},
		},
		{
			name:    "advisor count reports full trailing run",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, Labels: withFeedback(simpleLabels, "dissatisfied", "dissatisfied", "dissatisfied"), InputTokens: 100, LatestChars: 100},
			wantSel: "advisor", wantReason: "advisor", wantScored: "simple",
			wantParams: map[string]any{"count": 3},
		},
		{
			name:    "interrupted dissatisfied is not advisor",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, Labels: withFeedback(simpleLabels, "dissatisfied", "neutral", "dissatisfied"), InputTokens: 100, LatestChars: 100},
			wantSel: "simple", wantReason: "rule_simple", wantScored: "simple",
		},
		{
			name:    "scorer timeout default tier",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, ScorerFailure: "scorer_timeout", InputTokens: 100, LatestChars: 100},
			wantSel: "normal", wantReason: "scorer_timeout", wantScored: "normal",
		},
		{
			name:    "scorer error default tier",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, ScorerFailure: "scorer_error"},
			wantSel: "normal", wantReason: "scorer_error", wantScored: "normal",
		},
		{
			name:    "scorer invalid default tier",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, ScorerFailure: "scorer_invalid"},
			wantSel: "normal", wantReason: "scorer_invalid", wantScored: "normal",
		},
		{
			name:    "scorer timeout default tier still guarded",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, ScorerFailure: "scorer_timeout", Session: &smartroute.SessionRecord{CurrentRoute: "hard", LastNonAdvisor: "hard"}, InputTokens: 86000},
			wantSel: "hard", wantReason: "downgrade_blocked", wantScored: "normal",
			wantParams: map[string]any{"history_tokens": 86000, "limit": 20000, "kept_tier": "hard", "scored_tier": "normal"},
		},
		{
			name:    "tool continuation keeps advisor",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: false, Session: &smartroute.SessionRecord{CurrentRoute: "advisor", LastNonAdvisor: "normal"}},
			wantSel: "advisor", wantReason: "tool_continuation",
		},
		{
			name:    "tool continuation keeps tier",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: false, Session: &smartroute.SessionRecord{CurrentRoute: "hard", LastNonAdvisor: "hard"}},
			wantSel: "hard", wantReason: "tool_continuation",
		},
		{
			name:    "tool continuation without session uses default tier",
			in:      smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: false},
			wantSel: "normal", wantReason: "tool_continuation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := smartroute.DecideRoute(tt.in)
			assert.Equal(t, tt.wantSel, got.Selected)
			assert.Equal(t, tt.wantReason, got.Reason)
			assert.Equal(t, tt.wantScored, got.ScoredTier)
			for k, v := range tt.wantParams {
				assert.Equal(t, v, got.Params[k], k)
			}
		})
	}
}

func TestDecideRoute_RuleHardMatched(t *testing.T) {
	l := labelsOf("analyze", "high", "few_steps", "high", 0.9)
	l.Feedback = []string{"neutral"}
	got := smartroute.DecideRoute(smartroute.DecideInput{Cfg: smartRouteCfg(), IsUserTurn: true, Labels: &l, LatestChars: 10})
	assert.Equal(t, "hard", got.Selected)
	assert.Equal(t, "rule_hard", got.Reason)
	assert.Equal(t, "hard", got.ScoredTier)
	assert.ElementsMatch(t, []string{"reasoning", "risk"}, got.Params["matched"])
}

func TestDecideRoute_AdvisorEdgeCases(t *testing.T) {
	dissatisfied := func(l smartroute.ScorerLabels, fb ...string) *smartroute.ScorerLabels {
		l.Feedback = fb
		return &l
	}

	t.Run("invalid score result never goes to the advisor", func(t *testing.T) {
		got := smartroute.DecideRoute(smartroute.DecideInput{
			Cfg:           smartRouteCfg(),
			IsUserTurn:    true,
			Labels:        dissatisfied(labelsOf("rewrite", "low", "one_step", "low", 0.9), "dissatisfied", "dissatisfied"),
			ScorerFailure: "scorer_invalid",
			LatestChars:   100,
		})
		assert.Equal(t, "normal", got.Selected)
		assert.Equal(t, "scorer_invalid", got.Reason)
	})

	t.Run("advisor wins over low confidence and keeps a guarded fallback tier", func(t *testing.T) {
		got := smartroute.DecideRoute(smartroute.DecideInput{
			Cfg:         smartRouteCfg(),
			IsUserTurn:  true,
			Labels:      dissatisfied(labelsOf("rewrite", "low", "one_step", "low", 0.5), "neutral", "dissatisfied", "dissatisfied"),
			Session:     &smartroute.SessionRecord{CurrentRoute: "hard", LastNonAdvisor: "hard"},
			InputTokens: 86000,
			LatestChars: 100,
		})
		assert.Equal(t, "advisor", got.Selected)
		assert.Equal(t, "advisor", got.Reason)
		assert.Equal(t, map[string]any{"count": 2}, got.Params)
		assert.Equal(t, "hard", got.Fallback.Tier)
		assert.Equal(t, "downgrade_blocked", got.Fallback.Reason)
	})
}

func TestSessionStore(t *testing.T) {
	common.RedisEnabled = false

	t.Run("empty key is never stored", func(t *testing.T) {
		smartroute.SaveSessionAfterSuccess("", true, "hard", 7)
		_, ok := smartroute.LoadSession("")
		assert.False(t, ok)
	})

	t.Run("unknown key is absent", func(t *testing.T) {
		_, ok := smartroute.LoadSession("session-store-unknown")
		assert.False(t, ok)
	})

	t.Run("user turn writes all three fields", func(t *testing.T) {
		for _, route := range []string{"simple", "normal", "hard"} {
			key := "session-store-user-turn-" + route
			smartroute.SaveSessionAfterSuccess(key, true, route, 11)
			got, ok := smartroute.LoadSession(key)
			require.True(t, ok)
			assert.Equal(t, smartroute.SessionState{CurrentRoute: route, LastNonAdvisor: route, LastChannelID: 11}, got)
		}
	})

	t.Run("later user turn overwrites earlier one", func(t *testing.T) {
		key := "session-store-overwrite"
		smartroute.SaveSessionAfterSuccess(key, true, "simple", 1)
		smartroute.SaveSessionAfterSuccess(key, true, "hard", 2)
		got, ok := smartroute.LoadSession(key)
		require.True(t, ok)
		assert.Equal(t, smartroute.SessionState{CurrentRoute: "hard", LastNonAdvisor: "hard", LastChannelID: 2}, got)
	})

	t.Run("advisor turn keeps last non-advisor tier", func(t *testing.T) {
		key := "session-store-advisor"
		smartroute.SaveSessionAfterSuccess(key, true, "normal", 3)
		smartroute.SaveSessionAfterSuccess(key, true, "advisor", 4)
		got, ok := smartroute.LoadSession(key)
		require.True(t, ok)
		assert.Equal(t, smartroute.SessionState{CurrentRoute: "advisor", LastNonAdvisor: "normal", LastChannelID: 4}, got)
	})

	t.Run("advisor turn without prior record leaves last non-advisor empty", func(t *testing.T) {
		key := "session-store-advisor-first"
		smartroute.SaveSessionAfterSuccess(key, true, "advisor", 5)
		got, ok := smartroute.LoadSession(key)
		require.True(t, ok)
		assert.Equal(t, smartroute.SessionState{CurrentRoute: "advisor", LastNonAdvisor: "", LastChannelID: 5}, got)
	})

	t.Run("tool result continuation only updates channel", func(t *testing.T) {
		key := "session-store-tool-result"
		smartroute.SaveSessionAfterSuccess(key, true, "advisor", 6)
		smartroute.SaveSessionAfterSuccess(key, true, "hard", 6)
		smartroute.SaveSessionAfterSuccess(key, true, "advisor", 6)
		smartroute.SaveSessionAfterSuccess(key, false, "hard", 9)
		got, ok := smartroute.LoadSession(key)
		require.True(t, ok)
		assert.Equal(t, smartroute.SessionState{CurrentRoute: "advisor", LastNonAdvisor: "hard", LastChannelID: 9}, got)
	})

	t.Run("tool result continuation without record writes channel only", func(t *testing.T) {
		key := "session-store-tool-result-first"
		smartroute.SaveSessionAfterSuccess(key, false, "normal", 8)
		got, ok := smartroute.LoadSession(key)
		require.True(t, ok)
		assert.Equal(t, smartroute.SessionState{LastChannelID: 8}, got)
	})

	t.Run("json tags match the stored contract", func(t *testing.T) {
		raw, err := common.Marshal(smartroute.SessionState{CurrentRoute: "hard", LastNonAdvisor: "normal", LastChannelID: 3})
		require.NoError(t, err)
		assert.JSONEq(t, `{"current_route":"hard","last_non_advisor":"normal","last_channel_id":3}`, string(raw))
	})
}

func TestDecideRoute_ToolContinuationWithChannelOnlyRecord(t *testing.T) {
	got := smartroute.DecideRoute(smartroute.DecideInput{
		Cfg:        smartRouteCfg(),
		IsUserTurn: false,
		Session:    &smartroute.SessionRecord{CurrentRoute: "", LastNonAdvisor: ""},
	})
	assert.Equal(t, "normal", got.Selected)
	assert.Equal(t, "tool_continuation", got.Reason)
}

func setupValidateRouteTest(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalRoutes := operation_setting.SmartRoutingRoutesToString()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))
	model.DB = db
	common.MemoryCacheEnabled = true

	channels := []struct {
		id          int
		channelType int
		group       string
		modelName   string
	}{
		{1, constant.ChannelTypeXai, "api", "m-both"},
		{2, constant.ChannelTypeOpenAI, "api", "m-chat"},
		{3, constant.ChannelTypeCodex, "api", "m-resp"},
		{4, constant.ChannelTypeXai, "api", "m-noprice"},
	}
	priority := int64(0)
	weight := uint(100)
	for _, ch := range channels {
		require.NoError(t, db.Create(&model.Channel{
			Id: ch.id, Type: ch.channelType, Key: fmt.Sprintf("key-%d", ch.id),
			Status: common.ChannelStatusEnabled, Name: fmt.Sprintf("channel-%d", ch.id),
			Weight: &weight, Models: ch.modelName, Group: ch.group, Priority: &priority,
		}).Error)
		require.NoError(t, db.Create(&model.Ability{
			Group: ch.group, Model: ch.modelName, ChannelId: ch.id,
			Enabled: true, Priority: &priority, Weight: weight,
		}).Error)
	}
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(
		`{"m-both":0.01,"m-chat":0.01,"m-resp":0.01,"m-priced-only":0.01}`))
	operation_setting.LoadSmartRoutesFromJSONString(`[]`)
	model.InitChannelCache()
	model.RefreshPricing()

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		operation_setting.LoadSmartRoutesFromJSONString(originalRoutes)
		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
}

func validRoute() operation_setting.SmartRoute {
	tier := operation_setting.SmartRouteTier{Model: "m-both", Groups: []string{"api"}}
	return operation_setting.SmartRoute{
		Name: "smart-a",
		Tiers: map[string]operation_setting.SmartRouteTier{
			operation_setting.SmartTierSimple: tier,
			operation_setting.SmartTierNormal: tier,
			operation_setting.SmartTierHard:   tier,
		},
		DefaultTier:               operation_setting.SmartTierNormal,
		Scorer:                    operation_setting.SmartRouteScorer{Model: "m-chat", Groups: []string{"api"}, TimeoutMs: 2000},
		Advisor:                   operation_setting.SmartRouteAdvisor{Model: "m-both", Groups: []string{"api"}, AfterDissatisfied: 3},
		DowngradeMaxHistoryTokens: 4000,
		ConfidenceThreshold:       0.6,
		SimpleMaxMessageChars:     500,
	}
}

func setTier(route *operation_setting.SmartRoute, name string, mutate func(*operation_setting.SmartRouteTier)) {
	tier := route.Tiers[name]
	mutate(&tier)
	route.Tiers[name] = tier
}

func TestValidateRoute_ValidRoute(t *testing.T) {
	setupValidateRouteTest(t)

	errs, endpoints := smartroute.ValidateRoute(validRoute(), "")

	assert.Empty(t, errs)
	assert.ElementsMatch(t, []string{chatEndpoint, responsesEndpoint}, endpoints)
}

func TestValidateRoute_SingleFieldErrors(t *testing.T) {
	const (
		keyNoChannel = "{{model}} has no available channel in group {{group}}"
		keyNoPrice   = "{{model}} has no price yet. Set it in Model Pricing first"
		keySelect    = "Select a model"
		keyGroups    = "Select at least one group"
	)
	tests := []struct {
		name       string
		mutate     func(*operation_setting.SmartRoute)
		original   string
		wantField  string
		wantKey    string
		wantParams map[string]any
	}{
		{"empty name", func(r *operation_setting.SmartRoute) { r.Name = "" }, "", "name", "Enter a name", nil},
		{"default tier is not a tier", func(r *operation_setting.SmartRoute) { r.DefaultTier = "foo" }, "", "default_tier", "Select a default tier", nil},
		{"name equals channel model", func(r *operation_setting.SmartRoute) { r.Name = "m-both" }, "", "name",
			"{{name}} is a real model name. A virtual model cannot use the name of a real model", map[string]any{"name": "m-both"}},
		{"name equals priced model without channel", func(r *operation_setting.SmartRoute) { r.Name = "m-priced-only" }, "", "name",
			"{{name}} is a real model name. A virtual model cannot use the name of a real model", map[string]any{"name": "m-priced-only"}},
		{"name equals other virtual model", func(r *operation_setting.SmartRoute) { r.Name = "smart-taken" }, "", "name",
			"A virtual model named {{name}} already exists", map[string]any{"name": "smart-taken"}},
		{"renamed onto other virtual model", func(r *operation_setting.SmartRoute) { r.Name = "smart-taken" }, "smart-a", "name",
			"A virtual model named {{name}} already exists", map[string]any{"name": "smart-taken"}},
		{"simple model empty", func(r *operation_setting.SmartRoute) {
			setTier(r, operation_setting.SmartTierSimple, func(t *operation_setting.SmartRouteTier) { t.Model = "" })
		}, "", "tiers.simple.model", keySelect, nil},
		{"normal model empty", func(r *operation_setting.SmartRoute) {
			setTier(r, operation_setting.SmartTierNormal, func(t *operation_setting.SmartRouteTier) { t.Model = "" })
		}, "", "tiers.normal.model", keySelect, nil},
		{"hard model empty", func(r *operation_setting.SmartRoute) {
			setTier(r, operation_setting.SmartTierHard, func(t *operation_setting.SmartRouteTier) { t.Model = "" })
		}, "", "tiers.hard.model", keySelect, nil},
		{"scorer model empty", func(r *operation_setting.SmartRoute) { r.Scorer.Model = "" }, "", "scorer.model", keySelect, nil},
		{"advisor model empty", func(r *operation_setting.SmartRoute) { r.Advisor.Model = "" }, "", "advisor.model", keySelect, nil},
		{"simple groups empty", func(r *operation_setting.SmartRoute) {
			setTier(r, operation_setting.SmartTierSimple, func(t *operation_setting.SmartRouteTier) { t.Groups = nil })
		}, "", "tiers.simple.groups", keyGroups, nil},
		{"scorer groups empty", func(r *operation_setting.SmartRoute) { r.Scorer.Groups = []string{} }, "", "scorer.groups", keyGroups, nil},
		{"advisor groups empty", func(r *operation_setting.SmartRoute) { r.Advisor.Groups = nil }, "", "advisor.groups", keyGroups, nil},
		{"tier group auto is invalid", func(r *operation_setting.SmartRoute) {
			setTier(r, operation_setting.SmartTierHard, func(t *operation_setting.SmartRouteTier) { t.Groups = []string{"auto"} })
		}, "", "tiers.hard.model", keyNoChannel, map[string]any{"model": "m-both", "group": "auto"}},
		{"tier model has no channel in group", func(r *operation_setting.SmartRoute) {
			setTier(r, operation_setting.SmartTierHard, func(t *operation_setting.SmartRouteTier) { t.Groups = []string{"api", "vip"} })
		}, "", "tiers.hard.model", keyNoChannel, map[string]any{"model": "m-both", "group": "vip"}},
		{"scorer model has no channel in group", func(r *operation_setting.SmartRoute) { r.Scorer.Groups = []string{"vip"} }, "",
			"scorer.model", keyNoChannel, map[string]any{"model": "m-chat", "group": "vip"}},
		{"advisor model has no channel in group", func(r *operation_setting.SmartRoute) { r.Advisor.Groups = []string{"vip"} }, "",
			"advisor.model", keyNoChannel, map[string]any{"model": "m-both", "group": "vip"}},
		{"scorer model without price", func(r *operation_setting.SmartRoute) { r.Scorer.Model = "m-noprice" }, "",
			"scorer.model", keyNoPrice, map[string]any{"model": "m-noprice"}},
		{"tier model without price", func(r *operation_setting.SmartRoute) {
			setTier(r, operation_setting.SmartTierNormal, func(t *operation_setting.SmartRouteTier) { t.Model = "m-noprice" })
		}, "", "tiers.normal.model", keyNoPrice, map[string]any{"model": "m-noprice"}},
		{"advisor model without price", func(r *operation_setting.SmartRoute) { r.Advisor.Model = "m-noprice" }, "",
			"advisor.model", keyNoPrice, map[string]any{"model": "m-noprice"}},
		{"tier endpoint conflicts with advisor", func(r *operation_setting.SmartRoute) {
			for _, name := range []string{operation_setting.SmartTierSimple, operation_setting.SmartTierNormal, operation_setting.SmartTierHard} {
				setTier(r, name, func(t *operation_setting.SmartRouteTier) { t.Model = "m-chat" })
			}
			r.Advisor.Model = "m-resp"
		}, "", "advisor.model",
			"{{model}} only supports {{endpoint}}. The other models of this virtual model use {{other}}", map[string]any{"model": "m-resp", "endpoint": "openai-response", "other": "openai"}},
		{"endpoint conflict points at the model that empties the shared set", func(r *operation_setting.SmartRoute) {
			setTier(r, operation_setting.SmartTierSimple, func(t *operation_setting.SmartRouteTier) { t.Model = "m-both" })
			setTier(r, operation_setting.SmartTierNormal, func(t *operation_setting.SmartRouteTier) { t.Model = "m-chat" })
			setTier(r, operation_setting.SmartTierHard, func(t *operation_setting.SmartRouteTier) { t.Model = "m-resp" })
			r.Advisor.Model = "m-both"
		}, "", "tiers.hard.model",
			"{{model}} only supports {{endpoint}}. The other models of this virtual model use {{other}}", map[string]any{"model": "m-resp", "endpoint": "openai-response", "other": "openai"}},
		{"unknown reasoning effort", func(r *operation_setting.SmartRoute) {
			setTier(r, operation_setting.SmartTierHard, func(t *operation_setting.SmartRouteTier) { t.Effort = "ultra" })
		}, "", "tiers.hard.effort", "Select a valid reasoning effort", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupValidateRouteTest(t)
			operation_setting.LoadSmartRoutesFromJSONString(`[{"name":"smart-taken"}]`)
			route := validRoute()
			tt.mutate(&route)

			errs, _ := smartroute.ValidateRoute(route, tt.original)

			require.Len(t, errs, 1)
			assert.Equal(t, tt.wantField, errs[0].Field)
			require.NotNil(t, errs[0].Message)
			assert.Equal(t, tt.wantKey, errs[0].Message.Key)
			for name, want := range tt.wantParams {
				assert.Equal(t, want, errs[0].Message.Params[name], "param %s", name)
			}
		})
	}
}

func TestValidateRoute_EndpointMismatchReportsNoEndpoints(t *testing.T) {
	setupValidateRouteTest(t)
	route := validRoute()
	for _, name := range []string{operation_setting.SmartTierSimple, operation_setting.SmartTierNormal, operation_setting.SmartTierHard} {
		setTier(&route, name, func(t *operation_setting.SmartRouteTier) { t.Model = "m-chat" })
	}
	route.Advisor.Model = "m-resp"

	errs, endpoints := smartroute.ValidateRoute(route, "")

	require.NotEmpty(t, errs)
	assert.Empty(t, endpoints)
}

func TestValidateRoute_NameEqualsOriginalNameIsNotDuplicate(t *testing.T) {
	setupValidateRouteTest(t)
	operation_setting.LoadSmartRoutesFromJSONString(`[{"name":"smart-a"}]`)

	errs, _ := smartroute.ValidateRoute(validRoute(), "smart-a")
	assert.Empty(t, errs)

	errs, _ = smartroute.ValidateRoute(validRoute(), "")
	require.Len(t, errs, 1)
	assert.Equal(t, "name", errs[0].Field)
}

func TestValidateRoute_ScorerMayUseDifferentEndpoint(t *testing.T) {
	setupValidateRouteTest(t)
	route := validRoute()
	for _, name := range []string{operation_setting.SmartTierSimple, operation_setting.SmartTierNormal, operation_setting.SmartTierHard} {
		setTier(&route, name, func(t *operation_setting.SmartRouteTier) { t.Model = "m-resp" })
	}
	route.Advisor.Model = "m-resp"
	route.Scorer.Model = "m-chat"

	errs, endpoints := smartroute.ValidateRoute(route, "")

	assert.Empty(t, errs)
	assert.Equal(t, []string{responsesEndpoint}, endpoints)
}

func TestValidateRoute_NumberRanges(t *testing.T) {
	const keyRange = "Enter a value between {{min}} and {{max}}"
	const keyInt = "Enter an integer of 0 or more"
	tests := []struct {
		name      string
		set       func(*operation_setting.SmartRoute)
		wantField string
		wantKey   string
		wantMin   string
		wantMax   string
	}{
		{"timeout at lower bound", func(r *operation_setting.SmartRoute) { r.Scorer.TimeoutMs = 100 }, "", "", "", ""},
		{"timeout at upper bound", func(r *operation_setting.SmartRoute) { r.Scorer.TimeoutMs = 30000 }, "", "", "", ""},
		{"timeout below range", func(r *operation_setting.SmartRoute) { r.Scorer.TimeoutMs = 99 }, "scorer.timeout_ms", keyRange, "100", "30000"},
		{"timeout above range", func(r *operation_setting.SmartRoute) { r.Scorer.TimeoutMs = 30001 }, "scorer.timeout_ms", keyRange, "100", "30000"},
		{"dissatisfied at lower bound", func(r *operation_setting.SmartRoute) { r.Advisor.AfterDissatisfied = 1 }, "", "", "", ""},
		{"dissatisfied at upper bound", func(r *operation_setting.SmartRoute) { r.Advisor.AfterDissatisfied = 10 }, "", "", "", ""},
		{"dissatisfied zero", func(r *operation_setting.SmartRoute) { r.Advisor.AfterDissatisfied = 0 }, "advisor.after_dissatisfied", keyRange, "1", "10"},
		{"dissatisfied negative", func(r *operation_setting.SmartRoute) { r.Advisor.AfterDissatisfied = -1 }, "advisor.after_dissatisfied", keyRange, "1", "10"},
		{"dissatisfied above range", func(r *operation_setting.SmartRoute) { r.Advisor.AfterDissatisfied = 11 }, "advisor.after_dissatisfied", keyRange, "1", "10"},
		{"downgrade zero", func(r *operation_setting.SmartRoute) { r.DowngradeMaxHistoryTokens = 0 }, "", "", "", ""},
		{"downgrade large", func(r *operation_setting.SmartRoute) { r.DowngradeMaxHistoryTokens = 1000000 }, "", "", "", ""},
		{"downgrade negative", func(r *operation_setting.SmartRoute) { r.DowngradeMaxHistoryTokens = -1 }, "downgrade_max_history_tokens", keyInt, "", ""},
		{"confidence zero", func(r *operation_setting.SmartRoute) { r.ConfidenceThreshold = 0 }, "", "", "", ""},
		{"confidence one", func(r *operation_setting.SmartRoute) { r.ConfidenceThreshold = 1 }, "", "", "", ""},
		{"confidence negative", func(r *operation_setting.SmartRoute) { r.ConfidenceThreshold = -0.01 }, "confidence_threshold", keyRange, "0", "1"},
		{"confidence above one", func(r *operation_setting.SmartRoute) { r.ConfidenceThreshold = 1.01 }, "confidence_threshold", keyRange, "0", "1"},
		{"chars at lower bound", func(r *operation_setting.SmartRoute) { r.SimpleMaxMessageChars = 1 }, "", "", "", ""},
		{"chars at upper bound", func(r *operation_setting.SmartRoute) { r.SimpleMaxMessageChars = 100000 }, "", "", "", ""},
		{"chars zero", func(r *operation_setting.SmartRoute) { r.SimpleMaxMessageChars = 0 }, "simple_max_message_chars", keyRange, "1", "100000"},
		{"chars above range", func(r *operation_setting.SmartRoute) { r.SimpleMaxMessageChars = 100001 }, "simple_max_message_chars", keyRange, "1", "100000"},
	}

	setupValidateRouteTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := validRoute()
			tt.set(&route)

			errs, _ := smartroute.ValidateRoute(route, "")

			if tt.wantField == "" {
				assert.Empty(t, errs)
				return
			}
			require.Len(t, errs, 1)
			assert.Equal(t, tt.wantField, errs[0].Field)
			assert.Equal(t, tt.wantKey, errs[0].Message.Key)
			if tt.wantMin != "" {
				assert.Equal(t, tt.wantMin, fmt.Sprint(errs[0].Message.Params["min"]))
				assert.Equal(t, tt.wantMax, fmt.Sprint(errs[0].Message.Params["max"]))
			}
		})
	}
}

func TestRouteEndpoints(t *testing.T) {
	setupValidateRouteTest(t)
	mixed := validRoute()
	for _, name := range []string{operation_setting.SmartTierSimple, operation_setting.SmartTierNormal, operation_setting.SmartTierHard} {
		setTier(&mixed, name, func(t *operation_setting.SmartRouteTier) { t.Model = "m-both" })
	}
	mixed.Advisor.Model = "m-resp"
	conflict := validRoute()
	setTier(&conflict, operation_setting.SmartTierSimple, func(t *operation_setting.SmartRouteTier) { t.Model = "m-chat" })
	conflict.Advisor.Model = "m-resp"

	assert.ElementsMatch(t, []string{chatEndpoint, responsesEndpoint}, smartroute.RouteEndpoints(validRoute()))
	assert.Equal(t, []string{responsesEndpoint}, smartroute.RouteEndpoints(mixed))
	assert.Empty(t, smartroute.RouteEndpoints(conflict))
}

func TestValidateRoutes(t *testing.T) {
	setupValidateRouteTest(t)
	second := validRoute()
	second.Name = "smart-b"
	invalid := validRoute()
	invalid.Name = "smart-bad"
	invalid.Advisor.AfterDissatisfied = -1
	duplicate := validRoute()

	assert.NoError(t, smartroute.ValidateRoutes(nil))
	assert.NoError(t, smartroute.ValidateRoutes([]operation_setting.SmartRoute{validRoute(), second}))
	assert.Error(t, smartroute.ValidateRoutes([]operation_setting.SmartRoute{validRoute(), duplicate}))

	err := smartroute.ValidateRoutes([]operation_setting.SmartRoute{validRoute(), invalid})
	require.Error(t, err)
	var message *common.Message
	require.True(t, errors.As(err, &message))
	assert.Equal(t, "Virtual model {{name}}: {{detail}}", message.Key)
	assert.Equal(t, "smart-bad", message.Params["name"])
}

const wantScorerSystemPrompt = `You are a router that labels a user's latest request so a gateway can pick a model tier. You never answer the request itself.

Read the user messages (oldest first; earlier messages show only their beginning) and the local features, then return exactly one JSON object and nothing else:

{"task_type":"","reasoning":"","workload":"","risk":"","domain":"","confidence":0,"feedback":[]}

Field rules:
- task_type, judged on the LATEST message only, one of:
  chat_qa  = small talk or a simple factual question
  lookup   = find information, files, code locations or facts on the web
  rewrite  = translate, summarize, reformat or polish given text
  generate = write or change content such as code, reports, documents or copy
  analyze  = analyze data, investigate a failure, research and draw a conclusion
  plan     = design a solution, architecture, strategy or multi-part plan
- reasoning: low | medium | high. How much multi-step thinking, calculation or trade-off weighing the latest request needs.
- workload: one_step | few_steps | many_steps. How many actions (reading files, searching, running tools, fetching data) it needs.
- risk: low | high. high when the result will be published externally, used for decisions or bookkeeping, or touches production data, billing or security, and a mistake would be hard to notice.
- domain: code | finance | operations | data_analysis | other.
- confidence: a number from 0 to 1 for how sure you are about these labels.
- feedback: one value per user message shown, in the same order, each satisfied | dissatisfied | neutral. Use dissatisfied only when that message says the previous answer was wrong, did not work or missed the point.

Return only the JSON object.`

func TestBuildScorerMessages(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		history  int
		wantUser string
	}{
		{
			name:    "history lines are numbered and the latest line is last",
			body:    `{"messages":[{"role":"user","content":"first"},{"role":"assistant","content":"x"},{"role":"user","content":"second"},{"role":"user","content":"hello"}]}`,
			history: 2,
			wantUser: "Local features: latest_message_chars=5, user_turns=3, has_attachment=false, has_code_or_table=false\n" +
				"User messages (oldest first):\n[1] first\n[2] second\n[latest] hello",
		},
		{
			name:    "single message has only the latest line",
			body:    `{"messages":[{"role":"user","content":"hi"}]}`,
			history: 2,
			wantUser: "Local features: latest_message_chars=2, user_turns=1, has_attachment=false, has_code_or_table=false\n" +
				"User messages (oldest first):\n[latest] hi",
		},
		{
			name:    "attachment and code flags render as true",
			body:    "{\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"image_url\",\"image_url\":{\"url\":\"u\"}},{\"type\":\"text\",\"text\":\"```go\\nx\\n```\"}]}]}",
			history: 2,
			wantUser: "Local features: latest_message_chars=11, user_turns=1, has_attachment=true, has_code_or_table=true\n" +
				"User messages (oldest first):\n[latest] ```go\nx\n```",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, err := smartroute.InspectRequest(chatEndpoint, []byte(tt.body))
			require.NoError(t, err)

			system, user := smartroute.BuildScorerMessages(smartroute.BuildScorerInput(in, tt.history), in.Features)

			assert.Equal(t, wantScorerSystemPrompt, system)
			assert.Equal(t, tt.wantUser, user)
		})
	}
}

func TestScorerGroups(t *testing.T) {
	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"api":1,"vip":2}`))

	route := operation_setting.SmartRoute{Scorer: operation_setting.SmartRouteScorer{Groups: []string{"vip", "api", "no-ratio"}}}
	tests := []struct {
		name       string
		usable     string
		userGroup  string
		usingGroup string
		want       []string
	}{
		{"auto keeps scorer group order when the user may use all", `{"default":"d","api":"a","vip":"v","no-ratio":"n"}`, "default", "auto", []string{"vip", "api"}},
		{"auto skips groups the user may not use", `{"default":"d","api":"a"}`, "default", "auto", []string{"api"}},
		{"auto skips groups without a group ratio", `{"default":"d","no-ratio":"n"}`, "default", "auto", []string{}},
		{"non-auto token uses only its own group", `{"default":"d","api":"a","vip":"v"}`, "default", "api", []string{"api"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(tt.usable))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			common.SetContextKey(c, constant.ContextKeyUserGroup, tt.userGroup)
			common.SetContextKey(c, constant.ContextKeyUsingGroup, tt.usingGroup)

			got := smartroute.ScorerGroups(c, route)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRunScorer_UnregisteredRunnerFailsAsScorerError(t *testing.T) {
	original := smartroute.ScorerRunner
	smartroute.ScorerRunner = nil
	t.Cleanup(func() { smartroute.ScorerRunner = original })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	outcome, err := smartroute.RunScorer(c, operation_setting.SmartRoute{}, smartroute.Inspection{})

	require.NoError(t, err)
	assert.Equal(t, "scorer_error", outcome.Failure)
	assert.Nil(t, outcome.Labels)
	assert.Nil(t, outcome.Settle)
}
