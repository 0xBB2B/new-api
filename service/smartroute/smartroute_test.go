package smartroute_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
