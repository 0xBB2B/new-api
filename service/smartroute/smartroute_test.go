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
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/model_setting"
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
		{"/v1/messages", "anthropic"},
		{"/v1/embeddings", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, smartroute.RequestEndpointFromPath(tt.path))
		})
	}
}

const validScorerJSON = `{"task_type":"lookup","reasoning":"low","workload":"one_step","risk":"low","domain":"code","confidence":0.9,"effort_score":0.5,"feedback":["neutral","satisfied"]}`

func TestParseScorerOutput(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantFeedback int
		wantValues   []string
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
		{name: "feedback shorter than expected is kept as is", raw: validScorerJSON, wantFeedback: 3, wantValues: []string{"neutral", "satisfied"}},
		{name: "feedback longer than expected keeps the last values", raw: validScorerJSON, wantFeedback: 1, wantValues: []string{"satisfied"}},
		{name: "live sample with one feedback value for two messages", raw: `{"task_type":"chat_qa","reasoning":"low","workload":"one_step","risk":"low","domain":"other","confidence":0.95,"feedback":["neutral"]}`, wantFeedback: 2, wantValues: []string{"neutral"}},
		{name: "unknown feedback value is rejected even when truncated away", raw: `{"task_type":"lookup","reasoning":"low","workload":"one_step","risk":"low","domain":"code","confidence":0.9,"feedback":["happy","satisfied"]}`, wantFeedback: 1, wantErr: true},
		{name: "empty feedback array is kept empty", raw: `{"task_type":"lookup","reasoning":"low","workload":"one_step","risk":"low","domain":"code","confidence":0.9,"feedback":[]}`, wantFeedback: 2, wantValues: []string{}},
		{name: "null feedback is rejected", raw: `{"task_type":"lookup","reasoning":"low","workload":"one_step","risk":"low","domain":"code","confidence":0.9,"feedback":null}`, wantFeedback: 2, wantErr: true},
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
			if tt.wantValues != nil {
				assert.Equal(t, tt.wantValues, labels.Feedback)
				return
			}
			assert.Len(t, labels.Feedback, tt.wantFeedback)
		})
	}
}

func TestParseScorerOutput_TruncatedFeedbackTrailingCount(t *testing.T) {
	const tpl = `{"task_type":"lookup","reasoning":"low","workload":"one_step","risk":"low","domain":"code","confidence":0.9,"feedback":%s}`
	tests := []struct {
		name     string
		feedback string
		want     int
	}{
		{"leading dissatisfied are truncated away", `["dissatisfied","dissatisfied","neutral","neutral"]`, 0},
		{"trailing dissatisfied survive truncation", `["neutral","dissatisfied","dissatisfied"]`, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels, err := smartroute.ParseScorerOutput(fmt.Sprintf(tpl, tt.feedback), 2)
			require.NoError(t, err)
			assert.Equal(t, tt.want, smartroute.CountTrailingDissatisfied(labels.Feedback))
		})
	}
}

func TestParseScorerOutput_FeedbackMismatchKeepsOtherFields(t *testing.T) {
	labels, err := smartroute.ParseScorerOutput(validScorerJSON, 1)
	require.NoError(t, err)
	assert.Equal(t, 0.9, labels.Confidence)
	require.NotNil(t, labels.EffortScore)
	assert.Equal(t, 0.5, *labels.EffortScore)
	assert.Equal(t, "lookup", string(labels.TaskType))
	assert.Equal(t, "code", string(labels.Domain))
}

func TestParseScorerOutput_EffortScore(t *testing.T) {
	const valid = `"effort_score":0.5`
	tests := []struct {
		name    string
		raw     string
		want    *float64
		wantErr bool
	}{
		{name: "0 is valid", raw: strings.Replace(validScorerJSON, valid, `"effort_score":0`, 1), want: new(0.0)},
		{name: "0.5 is valid", raw: validScorerJSON, want: new(0.5)},
		{name: "1 is valid", raw: strings.Replace(validScorerJSON, valid, `"effort_score":1`, 1), want: new(1.0)},
		{name: "below 0 is invalid", raw: strings.Replace(validScorerJSON, valid, `"effort_score":-0.1`, 1)},
		{name: "above 1 is invalid", raw: strings.Replace(validScorerJSON, valid, `"effort_score":1.5`, 1)},
		{name: "string is invalid", raw: strings.Replace(validScorerJSON, valid, `"effort_score":"0.5"`, 1)},
		{name: "null is invalid", raw: strings.Replace(validScorerJSON, valid, `"effort_score":null`, 1)},
		{name: "missing is invalid", raw: strings.Replace(validScorerJSON, valid+",", "", 1)},
		{name: "valid effort_score does not rescue a bad label", raw: strings.Replace(strings.Replace(validScorerJSON, valid, `"effort_score":0.4`, 1), "one_step", "lots", 1), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels, err := smartroute.ParseScorerOutput(tt.raw, 2)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, labels.EffortScore)
			assert.Equal(t, "lookup", labels.TaskType)
			assert.Equal(t, "one_step", labels.Workload)
			assert.Equal(t, 0.9, labels.Confidence)
			assert.Equal(t, []string{"neutral", "satisfied"}, labels.Feedback)
		})
	}
}

func TestParseScorerOutput_Fields(t *testing.T) {
	labels, err := smartroute.ParseScorerOutput(validScorerJSON, 2)
	require.NoError(t, err)
	assert.Equal(t, smartroute.ScorerLabels{
		TaskType:    "lookup",
		Reasoning:   "low",
		Workload:    "one_step",
		Risk:        "low",
		Domain:      "code",
		Confidence:  0.9,
		EffortScore: new(0.5),
		Feedback:    []string{"neutral", "satisfied"},
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

func TestSelectTier_IgnoresEffortScore(t *testing.T) {
	base := labelsOf("analyze", "high", "few_steps", "low", 0.9)
	for _, tt := range []struct {
		name  string
		score *float64
	}{
		{name: "0", score: new(0.0)},
		{name: "1", score: new(1.0)},
		{name: "invalid", score: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			labels := base
			labels.EffortScore = tt.score
			got := smartroute.SelectTier(smartRouteCfg(), labels, 10)
			want := smartroute.SelectTier(smartRouteCfg(), base, 10)
			assert.Equal(t, "hard", got.Tier)
			assert.Equal(t, want, got)
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
		smartroute.SaveSessionAfterSuccess("", true, "hard", smartroute.EffortDecision{})
		_, ok := smartroute.LoadSession("")
		assert.False(t, ok)
	})

	t.Run("unknown key is absent", func(t *testing.T) {
		_, ok := smartroute.LoadSession("session-store-unknown")
		assert.False(t, ok)
	})

	t.Run("user turn writes both route fields", func(t *testing.T) {
		for _, route := range []string{"simple", "normal", "hard"} {
			key := "session-store-user-turn-" + route
			smartroute.SaveSessionAfterSuccess(key, true, route, smartroute.EffortDecision{})
			got, ok := smartroute.LoadSession(key)
			require.True(t, ok)
			assert.Equal(t, smartroute.SessionState{CurrentRoute: route, LastNonAdvisor: route}, got)
		}
	})

	t.Run("later user turn overwrites earlier one", func(t *testing.T) {
		key := "session-store-overwrite"
		smartroute.SaveSessionAfterSuccess(key, true, "simple", smartroute.EffortDecision{})
		smartroute.SaveSessionAfterSuccess(key, true, "hard", smartroute.EffortDecision{})
		got, ok := smartroute.LoadSession(key)
		require.True(t, ok)
		assert.Equal(t, smartroute.SessionState{CurrentRoute: "hard", LastNonAdvisor: "hard"}, got)
	})

	t.Run("advisor turn keeps last non-advisor tier", func(t *testing.T) {
		key := "session-store-advisor"
		smartroute.SaveSessionAfterSuccess(key, true, "normal", smartroute.EffortDecision{})
		smartroute.SaveSessionAfterSuccess(key, true, "advisor", smartroute.EffortDecision{})
		got, ok := smartroute.LoadSession(key)
		require.True(t, ok)
		assert.Equal(t, smartroute.SessionState{CurrentRoute: "advisor", LastNonAdvisor: "normal"}, got)
	})

	t.Run("advisor turn without prior record leaves last non-advisor empty", func(t *testing.T) {
		key := "session-store-advisor-first"
		smartroute.SaveSessionAfterSuccess(key, true, "advisor", smartroute.EffortDecision{})
		got, ok := smartroute.LoadSession(key)
		require.True(t, ok)
		assert.Equal(t, smartroute.SessionState{CurrentRoute: "advisor", LastNonAdvisor: ""}, got)
	})

	t.Run("tool result continuation keeps the stored routes", func(t *testing.T) {
		key := "session-store-tool-result"
		smartroute.SaveSessionAfterSuccess(key, true, "advisor", smartroute.EffortDecision{})
		smartroute.SaveSessionAfterSuccess(key, true, "hard", smartroute.EffortDecision{})
		smartroute.SaveSessionAfterSuccess(key, true, "advisor", smartroute.EffortDecision{})
		smartroute.SaveSessionAfterSuccess(key, false, "hard", smartroute.EffortDecision{})
		got, ok := smartroute.LoadSession(key)
		require.True(t, ok)
		assert.Equal(t, smartroute.SessionState{CurrentRoute: "advisor", LastNonAdvisor: "hard"}, got)
	})

	t.Run("json tags match the stored contract", func(t *testing.T) {
		raw, err := common.Marshal(smartroute.SessionState{CurrentRoute: "hard", LastNonAdvisor: "normal"})
		require.NoError(t, err)
		assert.JSONEq(t, `{"current_route":"hard","last_non_advisor":"normal"}`, string(raw))
	})
}

func TestDecideRoute_ToolContinuationWithEmptyRecord(t *testing.T) {
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
		Name:      "smart-a",
		Endpoints: []string{responsesEndpoint, chatEndpoint},
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

	assert.Empty(t, smartroute.ValidateRoute(validRoute()))
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
		{"unknown reasoning effort", func(r *operation_setting.SmartRoute) {
			setTier(r, operation_setting.SmartTierHard, func(t *operation_setting.SmartRouteTier) { t.EffortMode = "fixed"; t.Effort = "ultra" })
		}, "", "tiers.hard.effort", "Select a valid reasoning effort", nil},
		{"unknown scorer effort", func(r *operation_setting.SmartRoute) { r.Scorer.Effort = "ultra" }, "",
			"scorer.effort", "Select a valid reasoning effort", nil},
		{"scorer effort is case sensitive", func(r *operation_setting.SmartRoute) { r.Scorer.Effort = "LOW" }, "",
			"scorer.effort", "Select a valid reasoning effort", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupValidateRouteTest(t)
			operation_setting.LoadSmartRoutesFromJSONString(`[{"name":"smart-taken"}]`)
			route := validRoute()
			tt.mutate(&route)

			errs := smartroute.ValidateRoute(route)

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

func TestValidateRoute_ScorerEffort(t *testing.T) {
	for _, effort := range []string{"", "none", "minimal", "low", "medium", "high", "xhigh", "max"} {
		t.Run("accepts "+effort, func(t *testing.T) {
			setupValidateRouteTest(t)
			route := validRoute()
			route.Scorer.Effort = effort

			assert.Empty(t, smartroute.ValidateRoute(route))
		})
	}

	t.Run("old config without effort is valid", func(t *testing.T) {
		setupValidateRouteTest(t)
		var route operation_setting.SmartRoute
		require.NoError(t, common.UnmarshalJsonStr(`{"name":"smart-a","scorer":{"model":"m-chat","groups":["api"],"timeout_ms":2000}}`, &route))

		assert.Equal(t, "", route.Scorer.Effort)
		for _, e := range smartroute.ValidateRoute(route) {
			assert.NotEqual(t, "scorer.effort", e.Field)
		}
	})
}

func TestValidateRoute_SavedRouteWithSameNameIsNotDuplicate(t *testing.T) {
	setupValidateRouteTest(t)
	operation_setting.LoadSmartRoutesFromJSONString(`[{"name":"smart-a"}]`)

	errs := smartroute.ValidateRoute(validRoute())

	assert.Empty(t, errs)
}

func TestValidateRoute_SelfUseModeKeepsVirtualNameUsable(t *testing.T) {
	const realNameKey = "{{name}} is a real model name. A virtual model cannot use the name of a real model"
	tests := []struct {
		name     string
		route    string
		setup    func(t *testing.T)
		wantErrs int
	}{
		{"name without any price config", "auto-gpt", nil, 0},
		{"name with price config", "m-priced-only", nil, 1},
		{"name with only a model ratio config", "ratio-only", func(t *testing.T) {
			prev := ratio_setting.ModelRatio2JSONString()
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"ratio-only":1}`))
			t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(prev)) })
		}, 1},
		{"name with only a billing expression config", "expr-only", func(t *testing.T) {
			keys := []string{"billing_setting.billing_mode", "billing_setting.billing_expr"}
			prev := config.GlobalConfig.ExportAllConfigs()
			require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
				keys[0]: `{"expr-only":"tiered_expr"}`,
				keys[1]: `{"expr-only":"tier(\"base\", p * 1)"}`,
			}))
			t.Cleanup(func() {
				require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{keys[0]: prev[keys[0]], keys[1]: prev[keys[1]]}))
			})
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupValidateRouteTest(t)
			prev := operation_setting.SelfUseModeEnabled
			operation_setting.SelfUseModeEnabled = true
			t.Cleanup(func() { operation_setting.SelfUseModeEnabled = prev })
			if tt.setup != nil {
				tt.setup(t)
			}
			route := validRoute()
			route.Name = tt.route

			errs := smartroute.ValidateRoute(route)

			require.Len(t, errs, tt.wantErrs)
			if tt.wantErrs > 0 {
				assert.Equal(t, "name", errs[0].Field)
				assert.Equal(t, realNameKey, errs[0].Message.Key)
			}
		})
	}
}

func TestValidateRoute_ScorerChannelTypes(t *testing.T) {
	t.Run("only a Codex channel in the group is accepted", func(t *testing.T) {
		setupValidateRouteTest(t)
		route := validRoute()
		route.Scorer.Model = "m-resp"

		errs := smartroute.ValidateRoute(route)

		assert.Empty(t, errs)
	})

	t.Run("only a Replicate channel in the group is rejected", func(t *testing.T) {
		setupValidateRouteTest(t)
		priority, weight := int64(0), uint(100)
		require.NoError(t, model.DB.Create(&model.Channel{
			Id: 8, Type: constant.ChannelTypeReplicate, Key: "key-8", Status: common.ChannelStatusEnabled,
			Name: "channel-8", Weight: &weight, Models: "m-repl", Group: "api", Priority: &priority,
		}).Error)
		require.NoError(t, model.DB.Create(&model.Ability{
			Group: "api", Model: "m-repl", ChannelId: 8, Enabled: true, Priority: &priority, Weight: weight,
		}).Error)
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"m-both":0.01,"m-chat":0.01,"m-resp":0.01,"m-priced-only":0.01,"m-repl":0.01}`))
		model.InitChannelCache()
		route := validRoute()
		route.Scorer.Model = "m-repl"

		errs := smartroute.ValidateRoute(route)

		require.Len(t, errs, 1)
		assert.Equal(t, "scorer.model", errs[0].Field)
		assert.Equal(t, "{{model}} has no available channel in group {{group}}", errs[0].Message.Key)
		assert.Equal(t, "m-repl", errs[0].Message.Params["model"])
		assert.Equal(t, "api", errs[0].Message.Params["group"])
	})

	t.Run("a plain OpenAI channel serves the same model", func(t *testing.T) {
		setupValidateRouteTest(t)
		priority, weight := int64(0), uint(100)
		require.NoError(t, model.DB.Create(&model.Channel{
			Id: 9, Type: constant.ChannelTypeOpenAI, Key: "key-9", Status: common.ChannelStatusEnabled,
			Name: "channel-9", Weight: &weight, Models: "m-resp", Group: "api", Priority: &priority,
		}).Error)
		require.NoError(t, model.DB.Create(&model.Ability{
			Group: "api", Model: "m-resp", ChannelId: 9, Enabled: true, Priority: &priority, Weight: weight,
		}).Error)
		model.InitChannelCache()
		route := validRoute()
		route.Scorer.Model = "m-resp"

		errs := smartroute.ValidateRoute(route)

		assert.Empty(t, errs)
	})

	t.Run("tier and advisor may still use a Codex-only model", func(t *testing.T) {
		setupValidateRouteTest(t)
		route := validRoute()
		for _, name := range []string{operation_setting.SmartTierSimple, operation_setting.SmartTierNormal, operation_setting.SmartTierHard} {
			setTier(&route, name, func(t *operation_setting.SmartRouteTier) { t.Model = "m-resp" })
		}
		route.Advisor.Model = "m-resp"
		route.Endpoints = []string{responsesEndpoint}

		errs := smartroute.ValidateRoute(route)

		assert.Empty(t, errs)
	})
}

func TestValidateRoute_ScorerMayUseDifferentEndpoint(t *testing.T) {
	setupValidateRouteTest(t)
	route := validRoute()
	for _, name := range []string{operation_setting.SmartTierSimple, operation_setting.SmartTierNormal, operation_setting.SmartTierHard} {
		setTier(&route, name, func(t *operation_setting.SmartRouteTier) { t.Model = "m-resp" })
	}
	route.Advisor.Model = "m-resp"
	route.Scorer.Model = "m-chat"
	route.Endpoints = []string{responsesEndpoint}

	assert.Empty(t, smartroute.ValidateRoute(route))
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

			errs := smartroute.ValidateRoute(route)

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

func TestValidateRoutes_DetailIsTranslatableMessage(t *testing.T) {
	setupValidateRouteTest(t)
	noChannel := validRoute()
	noChannel.Name = "smart-nochannel"
	setTier(&noChannel, operation_setting.SmartTierHard, func(tier *operation_setting.SmartRouteTier) {
		tier.Groups = []string{"missing-group"}
	})
	fieldErrs := smartroute.ValidateRoute(noChannel)
	require.NotEmpty(t, fieldErrs)

	err := smartroute.ValidateRoutes([]operation_setting.SmartRoute{noChannel})
	require.Error(t, err)
	var outer *common.Message
	require.True(t, errors.As(err, &outer))
	assert.Equal(t, "Virtual model {{name}}: {{detail}}", outer.Key)
	assert.Equal(t, "smart-nochannel", outer.Params["name"])
	detail, ok := outer.Params["detail"].(*common.Message)
	require.True(t, ok, "detail must be a *common.Message, got %T", outer.Params["detail"])
	assert.Equal(t, fieldErrs[0].Message.Key, detail.Key)
	assert.Equal(t, fieldErrs[0].Message.Params, detail.Params)
	assert.Equal(t, "Virtual model smart-nochannel: "+fieldErrs[0].Message.Error(), err.Error())

	duplicate := smartroute.ValidateRoutes([]operation_setting.SmartRoute{validRoute(), validRoute()})
	require.Error(t, duplicate)
	require.True(t, errors.As(duplicate, &outer))
	detail, ok = outer.Params["detail"].(*common.Message)
	require.True(t, ok, "detail must be a *common.Message, got %T", outer.Params["detail"])
	assert.Equal(t, "A virtual model named {{name}} already exists", detail.Key)
	assert.Equal(t, "smart-a", detail.Params["name"])
	assert.Equal(t, "Virtual model smart-a: A virtual model named smart-a already exists", duplicate.Error())
}

const wantScorerSystemPrompt = `You are a router that labels a user's latest request so a gateway can pick a model tier. You never answer the request itself.

Read the user messages (oldest first; earlier messages show only their beginning) and the local features, then return exactly one JSON object and nothing else:

{"task_type":"","reasoning":"","effort_score":0,"workload":"","risk":"","domain":"","confidence":0,"feedback":[]}

Field rules:
- task_type, judged on the LATEST message only, one of:
  chat_qa  = small talk or a simple factual question
  lookup   = find information, files, code locations or facts on the web
  rewrite  = translate, summarize, reformat or polish given text
  generate = write or change content such as code, reports, documents or copy
  analyze  = analyze data, investigate a failure, research and draw a conclusion
  plan     = design a solution, architecture, strategy or multi-part plan
- reasoning: low | medium | high. How much multi-step thinking, calculation or trade-off weighing the latest request needs.
- effort_score: a number from 0 to 1 for how much thinking the latest request needs. Higher when the goal is vague or the problem is hard; lower when the goal is clear and the task is simple.
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
				"User messages (oldest first):\n[1] first\n[2] second\n[latest] hello\nThe feedback array must have exactly 3 values.",
		},
		{
			name:    "single message has only the latest line",
			body:    `{"messages":[{"role":"user","content":"hi"}]}`,
			history: 2,
			wantUser: "Local features: latest_message_chars=2, user_turns=1, has_attachment=false, has_code_or_table=false\n" +
				"User messages (oldest first):\n[latest] hi\nThe feedback array must have exactly 1 value.",
		},
		{
			name:    "attachment and code flags render as true",
			body:    "{\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"image_url\",\"image_url\":{\"url\":\"u\"}},{\"type\":\"text\",\"text\":\"```go\\nx\\n```\"}]}]}",
			history: 2,
			wantUser: "Local features: latest_message_chars=11, user_turns=1, has_attachment=true, has_code_or_table=true\n" +
				"User messages (oldest first):\n[latest] ```go\nx\n```\nThe feedback array must have exactly 1 value.",
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

func TestMainLogFields(t *testing.T) {
	cases := []struct {
		name            string
		result          smartroute.RouteResult
		cleanedReasoing int
		want            string
	}{
		{
			name: "downgrade blocked keeps full structure",
			result: smartroute.RouteResult{
				VirtualModel: "auto-gpt",
				Tier:         "hard",
				ScoredTier:   "simple",
				Reason:       smartroute.ReasonDowngradeBlocked,
				Params:       map[string]any{"history_tokens": 86420, "limit": 20000, "kept_tier": "hard", "scored_tier": "simple"},
				Labels: &smartroute.ScorerLabels{
					TaskType: "lookup", Reasoning: "low", Workload: "one_step", Risk: "low", Domain: "code",
					Confidence: 0.91, EffortScore: new(0.72), Feedback: []string{"neutral", "satisfied"},
				},
				ScorerStatus: smartroute.ScorerStatusOK,
				ScorerBilled: true,
			},
			cleanedReasoing: 2,
			want: `{
				"role":"main","virtual_model":"auto-gpt","tier":"hard","scored_tier":"simple",
				"reason":"downgrade_blocked",
				"reason_params":{"history_tokens":86420,"limit":20000,"kept_tier":"hard","scored_tier":"simple"},
				"labels":{"task_type":"lookup","reasoning":"low","workload":"one_step","risk":"low","domain":"code","confidence":0.91,"effort_score":0.72,"feedback":"satisfied"},
				"scorer":{"status":"ok","billed":true},
				"cleaned_reasoning":2
			}`,
		},
		{
			name: "tool continuation has no labels and scorer is skipped",
			result: smartroute.RouteResult{
				VirtualModel: "auto-gpt",
				Tier:         "normal",
				Reason:       smartroute.ReasonToolContinuation,
				ScorerStatus: smartroute.ScorerStatusSkipped,
			},
			want: `{
				"role":"main","virtual_model":"auto-gpt","tier":"normal",
				"reason":"tool_continuation","reason_params":{},
				"scorer":{"status":"skipped","billed":false}
			}`,
		},
		{
			name: "scorer timeout records timeout_ms and is not billed",
			result: smartroute.RouteResult{
				VirtualModel: "auto-gpt",
				Tier:         "normal",
				Reason:       smartroute.FailureScorerTimeout,
				Params:       map[string]any{"timeout_ms": 2000},
				ScorerStatus: smartroute.ScorerStatusTimeout,
			},
			want: `{
				"role":"main","virtual_model":"auto-gpt","tier":"normal",
				"reason":"scorer_timeout","reason_params":{"timeout_ms":2000},
				"scorer":{"status":"timeout","billed":false,"timeout_ms":2000}
			}`,
		},
		{
			name: "labels without feedback omit the feedback field",
			result: smartroute.RouteResult{
				VirtualModel: "auto-gpt",
				Tier:         "simple",
				Reason:       smartroute.ReasonRuleSimple,
				Labels:       &smartroute.ScorerLabels{TaskType: "chat_qa", Reasoning: "low", Workload: "one_step", Risk: "low", Domain: "other", Confidence: 0.8},
				ScorerStatus: smartroute.ScorerStatusOK,
				ScorerBilled: true,
			},
			want: `{
				"role":"main","virtual_model":"auto-gpt","tier":"simple",
				"reason":"rule_simple","reason_params":{},
				"labels":{"task_type":"chat_qa","reasoning":"low","workload":"one_step","risk":"low","domain":"other","confidence":0.8,"effort_score":"invalid"},
				"scorer":{"status":"ok","billed":true}
			}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := common.Marshal(smartroute.MainLogFields(tc.result, tc.cleanedReasoing))
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestMainLogFieldsHasNoConversationText(t *testing.T) {
	result := smartroute.RouteResult{
		VirtualModel: "auto-gpt",
		Tier:         "hard",
		Reason:       smartroute.ReasonRuleHard,
		Params:       map[string]any{"matched": []string{"risk"}},
		Labels:       &smartroute.ScorerLabels{TaskType: "plan", Reasoning: "high", Workload: "many_steps", Risk: "high", Domain: "finance", Confidence: 0.9, Feedback: []string{"dissatisfied"}},
		ScorerStatus: smartroute.ScorerStatusOK,
		ScorerBilled: true,
		SessionKey:   "session-secret-key",
	}

	got, err := common.Marshal(smartroute.MainLogFields(result, 0))
	require.NoError(t, err)

	assert.NotContains(t, string(got), "session-secret-key")
	assert.NotContains(t, string(got), "cleaned_reasoning")
}

func TestScorerLogFields(t *testing.T) {
	got, err := common.Marshal(smartroute.ScorerLogFields("auto-gpt", "hard", "gpt-5.6-sol"))
	require.NoError(t, err)

	assert.JSONEq(t, `{"role":"scorer","virtual_model":"auto-gpt","main_tier":"hard","main_model":"gpt-5.6-sol"}`, string(got))
}

func TestSetRouteResultPublishesMainLogFields(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	result := smartroute.RouteResult{
		VirtualModel: "auto-gpt",
		Tier:         "normal",
		Reason:       smartroute.ReasonToolContinuation,
		ScorerStatus: smartroute.ScorerStatusSkipped,
	}

	smartroute.SetRouteResult(ctx, result)

	logFields, ok := common.GetContextKey(ctx, constant.ContextKeySmartRouteLog)
	require.True(t, ok)
	assert.Equal(t, smartroute.MainLogFields(result, 0), logFields)
}

func TestDecideRoute_ScorerTimeoutCarriesTimeoutMs(t *testing.T) {
	cfg := smartRouteCfg()
	cfg.Scorer.TimeoutMs = 2000
	got := smartroute.DecideRoute(smartroute.DecideInput{Cfg: cfg, IsUserTurn: true, ScorerFailure: "scorer_timeout", LatestChars: 10})
	assert.Equal(t, "scorer_timeout", got.Reason)
	assert.Equal(t, map[string]any{"timeout_ms": 2000}, got.Params)
}

var effortLevels = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

func TestValidateRoute_Endpoints(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []string
		wantKey   string
	}{
		{"gemini is rejected", []string{"gemini"}, ""},
		{"valid value mixed with gemini is rejected", []string{"openai", "gemini"}, ""},
		{"all three accepted", []string{"openai-response", "openai", "anthropic"}, "-"},
		{"anthropic only accepted", []string{"anthropic"}, "-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupValidateRouteTest(t)
			route := validRoute()
			for _, name := range []string{operation_setting.SmartTierSimple, operation_setting.SmartTierNormal, operation_setting.SmartTierHard} {
				setTier(&route, name, func(tier *operation_setting.SmartRouteTier) { tier.Model = "m-chat" })
			}
			route.Advisor.Model = "m-chat"
			route.Endpoints = tt.endpoints

			errs := smartroute.ValidateRoute(route)

			if tt.wantKey == "-" {
				assert.Empty(t, errs)
				return
			}
			require.Len(t, errs, 1)
			assert.Equal(t, "endpoints", errs[0].Field)
			if tt.wantKey != "" {
				require.NotNil(t, errs[0].Message)
				assert.Equal(t, tt.wantKey, errs[0].Message.Key)
			}
		})
	}
}

func TestValidateRoute_EmptyEndpointsAreInferredFromModelMetadata(t *testing.T) {
	setupValidateRouteTest(t)
	legacy := validRoute()
	for _, name := range []string{operation_setting.SmartTierSimple, operation_setting.SmartTierNormal, operation_setting.SmartTierHard} {
		setTier(&legacy, name, func(tier *operation_setting.SmartRouteTier) { tier.Model = "m-chat" })
	}
	legacy.Advisor.Model = "m-chat"
	legacy.Endpoints = nil

	disjoint := legacy
	disjoint.Tiers = map[string]operation_setting.SmartRouteTier{}
	for name, tier := range legacy.Tiers {
		disjoint.Tiers[name] = tier
	}
	setTier(&disjoint, operation_setting.SmartTierSimple, func(tier *operation_setting.SmartRouteTier) { tier.Model = "m-resp" })

	t.Run("inferable set passes", func(t *testing.T) {
		assert.Empty(t, smartroute.ValidateRoute(legacy))
	})

	t.Run("empty slice is inferred the same way", func(t *testing.T) {
		route := legacy
		route.Endpoints = []string{}
		assert.Empty(t, smartroute.ValidateRoute(route))
	})

	t.Run("empty inferred set is still rejected", func(t *testing.T) {
		errs := smartroute.ValidateRoute(disjoint)

		require.Len(t, errs, 1)
		assert.Equal(t, "endpoints", errs[0].Field)
		assert.Equal(t, "Select at least one endpoint", errs[0].Message.Key)
	})

	t.Run("a legacy route saved together with a complete route", func(t *testing.T) {
		complete := validRoute()
		complete.Name = "smart-b"

		assert.NoError(t, smartroute.ValidateRoutes([]operation_setting.SmartRoute{legacy, complete}))
	})

	t.Run("a legacy route with an empty inferred set fails the whole batch", func(t *testing.T) {
		complete := validRoute()
		complete.Name = "smart-b"

		assert.Error(t, smartroute.ValidateRoutes([]operation_setting.SmartRoute{disjoint, complete}))
	})
}

func TestValidateRoute_TierModelsMayUseDifferentEndpoints(t *testing.T) {
	setupValidateRouteTest(t)
	addFormatTestChannel(t, 20, constant.ChannelTypeOpenAI, "api", "m-resp")
	route := validRoute()
	setTier(&route, operation_setting.SmartTierSimple, func(tier *operation_setting.SmartRouteTier) { tier.Model = "m-resp" })
	setTier(&route, operation_setting.SmartTierNormal, func(tier *operation_setting.SmartRouteTier) { tier.Model = "m-chat" })
	setTier(&route, operation_setting.SmartTierHard, func(tier *operation_setting.SmartRouteTier) { tier.Model = "m-both" })
	route.Advisor.Model = "m-chat"

	errs := smartroute.ValidateRoute(route)

	for _, fieldErr := range errs {
		require.NotNil(t, fieldErr.Message)
		assert.NotContains(t, fieldErr.Message.Key, "The other models of this virtual model use")
	}
	assert.Empty(t, errs)
}

func TestValidateRoute_EffortModes(t *testing.T) {
	tests := []struct {
		name string
		tier operation_setting.SmartRouteTier
		ok   bool
	}{
		{"no override", operation_setting.SmartRouteTier{}, true},
		{"no override with effort", operation_setting.SmartRouteTier{Effort: "low"}, false},
		{"no override with effort_min", operation_setting.SmartRouteTier{EffortMin: "low"}, false},
		{"no override with effort_max", operation_setting.SmartRouteTier{EffortMax: "high"}, false},
		{"unknown mode", operation_setting.SmartRouteTier{EffortMode: "auto"}, false},
		{"fixed empty effort", operation_setting.SmartRouteTier{EffortMode: "fixed"}, false},
		{"fixed unknown effort", operation_setting.SmartRouteTier{EffortMode: "fixed", Effort: "ultra"}, false},
		{"follow min above max", operation_setting.SmartRouteTier{EffortMode: "follow", EffortMin: "high", EffortMax: "low"}, false},
		{"follow unknown min", operation_setting.SmartRouteTier{EffortMode: "follow", EffortMin: "ultra", EffortMax: "high"}, false},
		{"follow unknown max", operation_setting.SmartRouteTier{EffortMode: "follow", EffortMin: "low", EffortMax: "ultra"}, false},
		{"follow empty min", operation_setting.SmartRouteTier{EffortMode: "follow", EffortMax: "high"}, false},
		{"follow empty max", operation_setting.SmartRouteTier{EffortMode: "follow", EffortMin: "low"}, false},
		{"follow min equals max", operation_setting.SmartRouteTier{EffortMode: "follow", EffortMin: "medium", EffortMax: "medium"}, true},
		{"follow full range", operation_setting.SmartRouteTier{EffortMode: "follow", EffortMin: "none", EffortMax: "max"}, true},
	}
	for _, level := range effortLevels {
		tests = append(tests, struct {
			name string
			tier operation_setting.SmartRouteTier
			ok   bool
		}{"fixed " + level, operation_setting.SmartRouteTier{EffortMode: "fixed", Effort: level}, true})
	}
	for lo, minLevel := range effortLevels {
		for _, maxLevel := range effortLevels[lo:] {
			tests = append(tests, struct {
				name string
				tier operation_setting.SmartRouteTier
				ok   bool
			}{"follow " + minLevel + " to " + maxLevel, operation_setting.SmartRouteTier{EffortMode: "follow", EffortMin: minLevel, EffortMax: maxLevel}, true})
		}
	}

	setupValidateRouteTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := validRoute()
			setTier(&route, operation_setting.SmartTierHard, func(tier *operation_setting.SmartRouteTier) {
				tier.EffortMode = tt.tier.EffortMode
				tier.Effort = tt.tier.Effort
				tier.EffortMin = tt.tier.EffortMin
				tier.EffortMax = tt.tier.EffortMax
			})

			errs := smartroute.ValidateRoute(route)

			if tt.ok {
				assert.Empty(t, errs)
				return
			}
			require.NotEmpty(t, errs)
			assert.True(t, strings.HasPrefix(errs[0].Field, "tiers.hard.effort"), errs[0].Field)
		})
	}
}

func useThinkingOverrides(t *testing.T, value string) {
	t.Helper()
	original := operation_setting.ThinkingCapabilitiesToString()
	operation_setting.LoadThinkingCapabilitiesFromJSONString(value)
	t.Cleanup(func() { operation_setting.LoadThinkingCapabilitiesFromJSONString(original) })
}

func TestBuiltinThinkingCapability(t *testing.T) {
	useThinkingOverrides(t, `{}`)
	tests := []struct {
		name       string
		model      string
		source     string
		noThinking bool
		efforts    []string
	}{
		{"gpt-4.1 非推理模型", "gpt-4.1", "builtin", true, []string{}},
		{"deepseek-v4-pro 可关闭且有 low/high/max", "deepseek-v4-pro", "builtin", false, []string{"none", "low", "high", "max"}},
		{"claude opus 4.6 按 relaykit 能力", "claude-opus-4-6", "builtin", false, nil},
		{"claude sonnet 5.5 按 relaykit 能力", "claude-sonnet-5-5", "builtin", false, nil},
		{"claude haiku 5.5 按 relaykit 能力", "claude-haiku-5-5", "builtin", false, nil},
		{"gemini 3.1 pro 按 relaykit 能力", "gemini-3.1-pro-preview", "builtin", false, nil},
		{"gemini 3.8 flash 按 relaykit 能力", "gemini-3.8-flash", "builtin", false, nil},
		{"glm-5.3", "glm-5.3", "builtin", false, []string{"low", "high", "max"}},
		{"glm-5.3-flash", "glm-5.3-flash", "builtin", false, []string{"low", "high", "max"}},
		{"glm-5.3-flashx", "glm-5.3-flashx", "builtin", false, []string{"low", "high", "max"}},
		{"glm-5.2", "glm-5.2", "builtin", false, []string{"none", "high", "max"}},
		{"gemini 图像模型不支持思考", "gemini-2.5-flash-image", "builtin", true, []string{}},
		{"gpt-4o 非推理", "gpt-4o", "builtin", true, []string{}},
		{"gpt-4.1-mini 非推理", "gpt-4.1-mini", "builtin", true, []string{}},
		{"gpt-5.4", "gpt-5.4", "builtin", false, []string{"none", "low", "medium", "high", "xhigh"}},
		{"gpt-5.4-mini", "gpt-5.4-mini", "builtin", false, []string{"none", "low", "medium", "high", "xhigh"}},
		{"gpt-5.4-nano", "gpt-5.4-nano", "builtin", false, []string{"none", "low", "medium", "high", "xhigh"}},
		{"gpt-5.5", "gpt-5.5", "builtin", false, []string{"none", "low", "medium", "high", "xhigh"}},
		{"gpt-5.4-pro", "gpt-5.4-pro", "builtin", false, []string{"medium", "high", "xhigh"}},
		{"gpt-5.5-pro", "gpt-5.5-pro", "builtin", false, []string{"medium", "high", "xhigh"}},
		{"gpt-5.2-codex", "gpt-5.2-codex", "builtin", false, []string{"low", "medium", "high", "xhigh"}},
		{"gpt-5.3-codex", "gpt-5.3-codex", "builtin", false, []string{"low", "medium", "high", "xhigh"}},
		{"gpt-5.6", "gpt-5.6", "builtin", false, []string{"none", "low", "medium", "high", "xhigh", "max"}},
		{"gpt-5.6-sol", "gpt-5.6-sol", "builtin", false, []string{"none", "low", "medium", "high", "xhigh", "max"}},
		{"gpt-5.6-terra", "gpt-5.6-terra", "builtin", false, []string{"none", "low", "medium", "high", "xhigh", "max"}},
		{"gpt-5.6-luna", "gpt-5.6-luna", "builtin", false, []string{"none", "low", "medium", "high", "xhigh", "max"}},
		{"gpt-6-sol", "gpt-6-sol", "builtin", false, []string{"none", "low", "medium", "high", "xhigh", "max"}},
		{"gpt-6-luna", "gpt-6-luna", "builtin", false, []string{"none", "low", "medium", "high", "xhigh", "max"}},
		{"gpt-6.1-terra", "gpt-6.1-terra", "builtin", false, []string{"none", "low", "medium", "high", "xhigh", "max"}},
		{"gpt-6-astra", "gpt-6-astra", "builtin", false, []string{"low", "medium", "high", "xhigh", "max"}},
		{"gpt-6.1-sol", "gpt-6.1-sol", "builtin", false, []string{"low", "medium", "high", "xhigh", "max"}},
		{"deepseek-v4-flash", "deepseek-v4-flash", "builtin", false, []string{"none", "low", "high", "max"}},
		{"2025 年及以前发布 gpt-5 未确认", "gpt-5", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 gpt-5.1 未确认", "gpt-5.1", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 gpt-5-pro 未确认", "gpt-5-pro", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 gpt-5.2 未确认", "gpt-5.2", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 gpt-5.2-pro 未确认", "gpt-5.2-pro", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 claude-sonnet-4-5 未确认", "claude-sonnet-4-5", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 claude-opus-4-5 未确认", "claude-opus-4-5", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 claude-haiku-4-5 未确认", "claude-haiku-4-5", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 claude-opus-4-1 未确认", "claude-opus-4-1", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 claude-sonnet-4 未确认", "claude-sonnet-4", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 claude-3-7-sonnet 未确认", "claude-3-7-sonnet", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 gemini-2.5-pro 未确认", "gemini-2.5-pro", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 gemini-2.5-flash 未确认", "gemini-2.5-flash", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 gemini-3-pro-preview 未确认", "gemini-3-pro-preview", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 gemini-3-flash-preview 未确认", "gemini-3-flash-preview", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 gemini-flash-latest 未确认", "gemini-flash-latest", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 glm-5 未确认", "glm-5", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"2025 年及以前发布 glm-5.1 未确认", "glm-5.1", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"o3 文档未写明档位，未确认", "o3", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"o4-mini 文档未写明档位，未确认", "o4-mini", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"gpt-5-mini 文档未写明档位，未确认", "gpt-5-mini", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"gpt-5.6-cyber 文档未写明档位，未确认", "gpt-5.6-cyber", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"deepseek-flash 文档未写明档位，未确认", "deepseek-flash", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"日期快照后缀 gpt-5.6-sol-2026-07-09 同 gpt-5.6-sol", "gpt-5.6-sol-2026-07-09", "builtin", false, []string{"none", "low", "medium", "high", "xhigh", "max"}},
		{"日期快照后缀 gpt-5-2025-08-07 同 gpt-5，未确认", "gpt-5-2025-08-07", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"日期快照后缀 gpt-5.4-2026-03-05 同 gpt-5.4", "gpt-5.4-2026-03-05", "builtin", false, []string{"none", "low", "medium", "high", "xhigh"}},
		{"大小写不敏感 GPT-5.5 同 gpt-5.5", "GPT-5.5", "builtin", false, []string{"none", "low", "medium", "high", "xhigh"}},
		{"前缀不误匹配 gpt-5-pro-2025-10-06 同 gpt-5-pro，未确认", "gpt-5-pro-2025-10-06", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"gpt-4o-mini-tts 不是 Gemini 家族，未确认", "gpt-4o-mini-tts", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"qwen3-tts-flash 不是 Gemini 家族，未确认", "qwen3-tts-flash", "unconfirmed", false, []string{"low", "medium", "high"}},
		{"qwen3-livetranslate-flash 不是 Gemini 家族，未确认", "qwen3-livetranslate-flash", "unconfirmed", false, []string{"low", "medium", "high"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := smartroute.BuiltinThinkingCapability(tt.model)
			assert.Equal(t, tt.source, got.Source)
			assert.Equal(t, tt.noThinking, got.NoThinking)
			if tt.efforts != nil {
				assert.Equal(t, tt.efforts, got.Efforts)
			}
		})
	}
}

func TestEffectiveThinkingCapability(t *testing.T) {
	t.Run("无覆盖时等同内置", func(t *testing.T) {
		useThinkingOverrides(t, `{}`)
		assert.Equal(t, smartroute.BuiltinThinkingCapability("gpt-5"), smartroute.EffectiveThinkingCapability("gpt-5"))
	})

	t.Run("覆盖未收录模型", func(t *testing.T) {
		useThinkingOverrides(t, `{"glm-5.1":{"no_thinking":false,"efforts":["low","high"]}}`)
		got := smartroute.EffectiveThinkingCapability("glm-5.1")
		assert.Equal(t, "override", got.Source)
		assert.False(t, got.NoThinking)
		assert.Equal(t, []string{"low", "high"}, got.Efforts)
	})

	t.Run("覆盖优先于内置", func(t *testing.T) {
		useThinkingOverrides(t, `{"gpt-4.1":{"no_thinking":false,"efforts":["medium"]}}`)
		got := smartroute.EffectiveThinkingCapability("gpt-4.1")
		assert.Equal(t, "override", got.Source)
		assert.False(t, got.NoThinking)
		assert.Equal(t, []string{"medium"}, got.Efforts)
	})

	t.Run("覆盖为不支持思考", func(t *testing.T) {
		useThinkingOverrides(t, `{"gpt-5":{"no_thinking":true,"efforts":[]}}`)
		got := smartroute.EffectiveThinkingCapability("gpt-5")
		assert.Equal(t, "override", got.Source)
		assert.True(t, got.NoThinking)
	})

	t.Run("内置查询不受覆盖影响", func(t *testing.T) {
		useThinkingOverrides(t, `{"gpt-5.4":{"no_thinking":true,"efforts":[]}}`)
		assert.Equal(t, "builtin", smartroute.BuiltinThinkingCapability("gpt-5.4").Source)
	})
}

func TestClampEffortToCapability(t *testing.T) {
	capability := func(efforts ...string) smartroute.ThinkingCapability {
		return smartroute.ThinkingCapability{Source: "builtin", Efforts: efforts}
	}
	tests := []struct {
		name       string
		effort     string
		capability smartroute.ThinkingCapability
		want       string
		changed    bool
	}{
		{"支持的档位原样返回", "low", capability("low", "high"), "low", false},
		{"medium 与 low、high 一样近，取更高", "medium", capability("low", "high"), "high", true},
		{"xhigh 超出上限收敛到 high", "xhigh", capability("low", "medium", "high"), "high", true},
		{"高于上限的 max 收敛到最高档", "max", capability("low", "high", "xhigh"), "xhigh", true},
		{"minimal 低于下限收敛到 low", "minimal", capability("low", "medium", "high"), "low", true},
		{"请求 none 但不支持，取最近一档", "none", capability("minimal", "low", "high"), "minimal", true},
		{"请求 none 且支持", "none", capability("none", "low"), "none", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := smartroute.ClampEffortToCapability(tt.effort, tt.capability)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.changed, changed)
		})
	}
}

func TestResolveTierEffort_FollowScoreSegments(t *testing.T) {
	all := smartroute.ThinkingCapability{Source: "builtin", Efforts: []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}}
	follow := func(lo, hi string) operation_setting.SmartRouteTier {
		return operation_setting.SmartRouteTier{EffortMode: smartroute.EffortModeFollow, EffortMin: lo, EffortMax: hi}
	}
	scored := func(score float64) *smartroute.ScorerLabels { return &smartroute.ScorerLabels{EffortScore: &score} }
	tests := []struct {
		name       string
		tier       operation_setting.SmartRouteTier
		labels     *smartroute.ScorerLabels
		failed     bool
		capability smartroute.ThinkingCapability
		want       smartroute.EffortDecision
	}{
		{"3 档: 0 落第 1 段", follow("medium", "xhigh"), scored(0), false, all, smartroute.EffortDecision{Value: "medium", Source: "follow"}},
		{"3 档: 0.33 仍在第 1 段", follow("medium", "xhigh"), scored(0.33), false, all, smartroute.EffortDecision{Value: "medium", Source: "follow"}},
		{"3 档: 0.34 进第 2 段", follow("medium", "xhigh"), scored(0.34), false, all, smartroute.EffortDecision{Value: "high", Source: "follow"}},
		{"3 档: 0.66 仍在第 2 段", follow("medium", "xhigh"), scored(0.66), false, all, smartroute.EffortDecision{Value: "high", Source: "follow"}},
		{"3 档: 0.67 进第 3 段", follow("medium", "xhigh"), scored(0.67), false, all, smartroute.EffortDecision{Value: "xhigh", Source: "follow"}},
		{"3 档: 1 不越界", follow("medium", "xhigh"), scored(1), false, all, smartroute.EffortDecision{Value: "xhigh", Source: "follow"}},
		{"2 档: 0.5 进第 2 段", follow("high", "xhigh"), scored(0.5), false, all, smartroute.EffortDecision{Value: "xhigh", Source: "follow"}},
		{"1 档: 任意分数都是这一档", follow("high", "high"), scored(0.9), false, all, smartroute.EffortDecision{Value: "high", Source: "follow"}},
		{"模型能力裁掉候选后重新分段", follow("none", "max"), scored(0.5), false,
			smartroute.ThinkingCapability{Source: "builtin", Efforts: []string{"low", "high", "max"}},
			smartroute.EffortDecision{Value: "high", Source: "follow"}},
		{"没有分数取候选最高档", follow("medium", "xhigh"), &smartroute.ScorerLabels{}, false, all, smartroute.EffortDecision{Value: "xhigh", Source: "follow"}},
		{"打分失败取候选最高档", follow("medium", "xhigh"), scored(0.1), true, all, smartroute.EffortDecision{Value: "xhigh", Source: "follow"}},
		{"候选为 0 档: 上下一样近取更高", follow("medium", "medium"), scored(0.5), false,
			smartroute.ThinkingCapability{Source: "override", Efforts: []string{"low", "high"}},
			smartroute.EffortDecision{Value: "high", Source: "follow", From: "medium"}},
		{"候选为 0 档且没有分数: 取 EffortMax 再换档", follow("medium", "high"), nil, true,
			smartroute.ThinkingCapability{Source: "override", Efforts: []string{"low", "xhigh"}},
			smartroute.EffortDecision{Value: "xhigh", Source: "follow", From: "high"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := smartroute.ResolveTierEffort(smartroute.EffortInput{
				Tier: tt.tier, IsUserTurn: true, Labels: tt.labels, ScorerFailed: tt.failed, Capability: tt.capability,
			})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClampEffortToCapability_NoThinkingYieldsEmpty(t *testing.T) {
	for _, effort := range []string{"none", "low", "high", "max"} {
		got, _ := smartroute.ClampEffortToCapability(effort, smartroute.ThinkingCapability{Source: "builtin", NoThinking: true})
		assert.Empty(t, got, effort)
	}
}

func TestInspectRequest_Anthropic(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		wantUserTurn   bool
		wantFirst      string
		wantLatest     string
		wantAttachment bool
	}{
		{"string content", `{"messages":[{"role":"user","content":"hello"}]}`, true, "hello", "hello", false},
		{"text blocks are joined", `{"messages":[{"role":"user","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}]}`, true, "ab", "ab", false},
		{"tool result only is a continuation", `{"messages":[{"role":"user","content":"first"},{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"f","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"secret result"}]}]}`, false, "first", "first", false},
		{"tool result with text is a user turn without the result text", `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"secret result"},{"type":"text","text":"thanks, now do more"}]}]}`, true, "thanks, now do more", "thanks, now do more", false},
		{"image is an attachment", `{"messages":[{"role":"user","content":[{"type":"text","text":"look"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AA=="}}]}]}`, true, "look", "look", true},
		{"document is an attachment", `{"messages":[{"role":"user","content":[{"type":"text","text":"read"},{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"AA=="}}]}]}`, true, "read", "read", true},
		{"system and thinking blocks are not user text", `{"system":"be nice","messages":[{"role":"user","content":"q1"},{"role":"assistant","content":[{"type":"thinking","thinking":"hmm","signature":"s"},{"type":"text","text":"a1"}]},{"role":"user","content":"q2"}]}`, true, "q1", "q2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, err := smartroute.InspectRequest("anthropic", []byte(tt.body))

			require.NoError(t, err)
			assert.Equal(t, tt.wantUserTurn, in.IsUserTurn)
			require.NotEmpty(t, in.UserMessages)
			assert.Equal(t, tt.wantFirst, in.UserMessages[0])
			assert.Equal(t, tt.wantLatest, in.UserMessages[len(in.UserMessages)-1])
			assert.Equal(t, tt.wantAttachment, in.Features.HasAttachment)
			assert.NotContains(t, strings.Join(in.UserMessages, "|"), "secret result")
			assert.NotContains(t, strings.Join(in.UserMessages, "|"), "be nice")
			assert.NotContains(t, strings.Join(in.UserMessages, "|"), "hmm")
		})
	}
}

func TestAnthropicScorerInputAndSessionKey(t *testing.T) {
	body := []byte(`{"system":"be nice","messages":[{"role":"user","content":"q1"},{"role":"assistant","content":[{"type":"thinking","thinking":"hmm","signature":"s"},{"type":"text","text":"a1"}]},{"role":"user","content":"q2"}]}`)
	in, err := smartroute.InspectRequest("anthropic", body)
	require.NoError(t, err)

	assert.Equal(t, smartroute.ScorerInput{Latest: "q2", History: []string{"q1"}}, smartroute.BuildScorerInput(in, 3))

	later, err := smartroute.InspectRequest("anthropic", []byte(`{"messages":[{"role":"user","content":"q1"},{"role":"assistant","content":"a1"},{"role":"user","content":"q9"}]}`))
	require.NoError(t, err)
	key := smartroute.SessionKey(7, in)
	require.NotEmpty(t, key)
	assert.Equal(t, key, smartroute.SessionKey(7, later))
	assert.NotEqual(t, key, smartroute.SessionKey(8, in))
}

const keyFormatRejected = "{{model}} has only {{channel_types}} channels in group {{group}}, which cannot accept the {{format}} format"

func addFormatTestChannel(t *testing.T, id, channelType int, group, modelName string) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: id, Type: channelType, Key: fmt.Sprintf("key-%d", id),
		Status: common.ChannelStatusEnabled, Name: fmt.Sprintf("channel-%d", id),
		Weight: &weight, Models: modelName, Group: group, Priority: &priority,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group: group, Model: modelName, ChannelId: id,
		Enabled: true, Priority: &priority, Weight: weight,
	}).Error)
	model.InitChannelCache()
}

func formatRejectionRoute(endpoint string) operation_setting.SmartRoute {
	route := validRoute()
	route.Endpoints = []string{endpoint}
	setTier(&route, operation_setting.SmartTierSimple, func(tier *operation_setting.SmartRouteTier) {
		tier.Model, tier.Groups = "gpt-5.5", []string{"api"}
	})
	setTier(&route, operation_setting.SmartTierNormal, func(tier *operation_setting.SmartRouteTier) {
		tier.Model, tier.Groups = "gpt-5.5", []string{"subscription", "api"}
	})
	setTier(&route, operation_setting.SmartTierHard, func(tier *operation_setting.SmartRouteTier) {
		tier.Model, tier.Groups = "gpt-5.5", []string{"api"}
	})
	route.Scorer = operation_setting.SmartRouteScorer{Model: "m-chat", Groups: []string{"api"}, TimeoutMs: 2000}
	route.Advisor.Model, route.Advisor.Groups = "gpt-5.5", []string{"api"}
	return route
}

func setupFormatRejectionTest(t *testing.T) {
	t.Helper()
	setupValidateRouteTest(t)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(
		`{"m-both":0.01,"m-chat":0.01,"m-resp":0.01,"gpt-5.5":0.01,"claude-x":0.01}`))
	addFormatTestChannel(t, 11, constant.ChannelTypeCodex, "subscription", "gpt-5.5")
	addFormatTestChannel(t, 12, constant.ChannelTypeOpenAI, "api", "gpt-5.5")
}

func formatRejectionErrors(errs []smartroute.FieldError) []smartroute.FieldError {
	var found []smartroute.FieldError
	for _, fieldErr := range errs {
		if fieldErr.Message != nil && fieldErr.Message.Key == keyFormatRejected {
			found = append(found, fieldErr)
		}
	}
	return found
}

func TestValidateRoute_TierGroupOnlyRejectingChannels(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		wantFormat string
	}{
		{"responses accepted by codex", "openai-response", ""},
		{"anthropic rejected by codex", "anthropic", "Anthropic"},
		{"chat completions rejected by codex", "openai", "Chat Completions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupFormatRejectionTest(t)

			found := formatRejectionErrors(smartroute.ValidateRoute(formatRejectionRoute(tt.endpoint)))

			if tt.wantFormat == "" {
				assert.Empty(t, found)
				return
			}
			require.Len(t, found, 1)
			assert.Equal(t, "tiers.normal.groups", found[0].Field)
			assert.Equal(t, "gpt-5.5", found[0].Message.Params["model"])
			assert.Equal(t, "subscription", found[0].Message.Params["group"])
			assert.Equal(t, tt.wantFormat, found[0].Message.Params["format"])
			assert.Contains(t, fmt.Sprint(found[0].Message.Params["channel_types"]), constant.ChannelTypeNames[constant.ChannelTypeCodex])
		})
	}
}

func TestValidateRoute_GroupWithAcceptingChannelIsSavable(t *testing.T) {
	setupFormatRejectionTest(t)
	addFormatTestChannel(t, 13, constant.ChannelTypeOpenAI, "subscription", "gpt-5.5")

	found := formatRejectionErrors(smartroute.ValidateRoute(formatRejectionRoute("anthropic")))

	assert.Empty(t, found)
}

func TestValidateRoute_AdvisorOnlyRejectingChannels(t *testing.T) {
	setupFormatRejectionTest(t)
	addFormatTestChannel(t, 14, constant.ChannelTypeClaudeSubscription, "claude-sub", "claude-x")
	route := formatRejectionRoute("openai-response")
	route.Advisor.Model, route.Advisor.Groups = "claude-x", []string{"claude-sub"}

	found := formatRejectionErrors(smartroute.ValidateRoute(route))

	require.Len(t, found, 1)
	assert.Equal(t, "advisor.groups", found[0].Field)
	assert.Equal(t, "claude-x", found[0].Message.Params["model"])
	assert.Equal(t, "claude-sub", found[0].Message.Params["group"])
	assert.Equal(t, "Responses", found[0].Message.Params["format"])
	assert.Contains(t, fmt.Sprint(found[0].Message.Params["channel_types"]), constant.ChannelTypeNames[constant.ChannelTypeClaudeSubscription])
}

func TestValidateRoute_ScorerIsNotCheckedForFormat(t *testing.T) {
	setupFormatRejectionTest(t)
	addFormatTestChannel(t, 15, constant.ChannelTypeCodex, "scorer-group", "scorer-m")
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(
		`{"m-both":0.01,"m-chat":0.01,"m-resp":0.01,"gpt-5.5":0.01,"scorer-m":0.01}`))
	route := formatRejectionRoute("anthropic")
	route.Scorer.Model, route.Scorer.Groups = "scorer-m", []string{"scorer-group"}

	found := formatRejectionErrors(smartroute.ValidateRoute(route))

	for _, fieldErr := range found {
		assert.NotContains(t, fieldErr.Field, "scorer")
	}
}

func TestValidateRoute_ChatToResponsesPolicyCountsAsAccepting(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		policy   model_setting.ChatCompletionsToResponsesPolicy
		wantErr  bool
	}{
		{"anthropic with policy hit", "anthropic", model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, ChannelTypes: []int{constant.ChannelTypeCodex}, ModelPatterns: []string{"^gpt-5"}}, false},
		{"chat with policy hit", "openai", model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, ChannelTypes: []int{constant.ChannelTypeCodex}, ModelPatterns: []string{"^gpt-5"}}, false},
		{"anthropic without policy", "anthropic", model_setting.ChatCompletionsToResponsesPolicy{}, true},
		{"chat without policy", "openai", model_setting.ChatCompletionsToResponsesPolicy{}, true},
		{"anthropic with model pattern miss", "anthropic", model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, ChannelTypes: []int{constant.ChannelTypeCodex}, ModelPatterns: []string{"^claude"}}, true},
		{"anthropic with other channel type", "anthropic", model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, ChannelTypes: []int{constant.ChannelTypeOpenAI}, ModelPatterns: []string{"^gpt-5"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupFormatRejectionTest(t)
			settings := model_setting.GetGlobalSettings()
			original := settings.ChatCompletionsToResponsesPolicy
			settings.ChatCompletionsToResponsesPolicy = tt.policy
			t.Cleanup(func() { settings.ChatCompletionsToResponsesPolicy = original })

			found := formatRejectionErrors(smartroute.ValidateRoute(formatRejectionRoute(tt.endpoint)))

			if tt.wantErr {
				require.Len(t, found, 1)
				assert.Equal(t, "tiers.normal.groups", found[0].Field)
				return
			}
			assert.Empty(t, found)
		})
	}
}

func TestValidateRoute_RejectingChannelTypesJoinedWithComma(t *testing.T) {
	setupFormatRejectionTest(t)
	addFormatTestChannel(t, 16, constant.ChannelTypeXai, "subscription", "gpt-5.5")

	found := formatRejectionErrors(smartroute.ValidateRoute(formatRejectionRoute("anthropic")))

	require.Len(t, found, 1)
	codex, xai := constant.ChannelTypeNames[constant.ChannelTypeCodex], constant.ChannelTypeNames[constant.ChannelTypeXai]
	assert.Contains(t, []any{codex + ", " + xai, xai + ", " + codex}, found[0].Message.Params["channel_types"])
}

func TestValidateRoute_DisabledChannelsAreNotCounted(t *testing.T) {
	setupFormatRejectionTest(t)
	addFormatTestChannel(t, 17, constant.ChannelTypeOpenAI, "subscription", "gpt-5.5")
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 17).Update("status", common.ChannelStatusManuallyDisabled).Error)
	require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ?", 17).Update("enabled", false).Error)
	model.InitChannelCache()

	found := formatRejectionErrors(smartroute.ValidateRoute(formatRejectionRoute("anthropic")))

	require.Len(t, found, 1)
	assert.Equal(t, "tiers.normal.groups", found[0].Field)
}

func setChatToResponsesPolicy(t *testing.T, policy model_setting.ChatCompletionsToResponsesPolicy, globalPassThrough bool) {
	t.Helper()
	settings := model_setting.GetGlobalSettings()
	originalPolicy, originalPassThrough := settings.ChatCompletionsToResponsesPolicy, settings.PassThroughRequestEnabled
	settings.ChatCompletionsToResponsesPolicy, settings.PassThroughRequestEnabled = policy, globalPassThrough
	t.Cleanup(func() {
		settings.ChatCompletionsToResponsesPolicy, settings.PassThroughRequestEnabled = originalPolicy, originalPassThrough
	})
}

func normalTierGroupErrors(errs []smartroute.FieldError) []smartroute.FieldError {
	var found []smartroute.FieldError
	for _, fieldErr := range errs {
		if fieldErr.Field == "tiers.normal.groups" {
			found = append(found, fieldErr)
		}
	}
	return found
}

func TestValidateRoute_PolicyConvertsToFormatChannelRejects(t *testing.T) {
	allChannels := model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, AllChannels: true, ModelPatterns: []string{".*"}}
	tests := []struct {
		name         string
		endpoint     string
		policy       model_setting.ChatCompletionsToResponsesPolicy
		extraChannel bool
		wantErr      bool
	}{
		{"chat converted to responses that minimax rejects", "openai", allChannels, false, true},
		{"anthropic converted to responses that minimax rejects", "anthropic", allChannels, false, true},
		{"chat without policy is accepted by minimax", "openai", model_setting.ChatCompletionsToResponsesPolicy{}, false, false},
		{"anthropic without policy is accepted by minimax", "anthropic", model_setting.ChatCompletionsToResponsesPolicy{}, false, false},
		{"chat is savable when another channel is not hit by the policy", "openai",
			model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, ChannelTypes: []int{constant.ChannelTypeMiniMax}, ModelPatterns: []string{".*"}}, true, false},
		{"anthropic is savable when another channel is not hit by the policy", "anthropic",
			model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, ChannelTypes: []int{constant.ChannelTypeMiniMax}, ModelPatterns: []string{".*"}}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupFormatRejectionTest(t)
			addFormatTestChannel(t, 30, constant.ChannelTypeMiniMax, "mm", "gpt-5.5")
			if tt.extraChannel {
				addFormatTestChannel(t, 31, constant.ChannelTypeOpenAI, "mm", "gpt-5.5")
			}
			setChatToResponsesPolicy(t, tt.policy, false)
			route := formatRejectionRoute(tt.endpoint)
			setTier(&route, operation_setting.SmartTierNormal, func(tier *operation_setting.SmartRouteTier) { tier.Groups = []string{"mm"} })

			found := normalTierGroupErrors(smartroute.ValidateRoute(route))

			if tt.wantErr {
				assert.NotEmpty(t, found)
				return
			}
			assert.Empty(t, found)
		})
	}
}

func TestValidateRoute_PassThroughSkipsChatToResponsesPolicy(t *testing.T) {
	codexPolicy := model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, ChannelTypes: []int{constant.ChannelTypeCodex}, ModelPatterns: []string{"^gpt-5"}}
	tests := []struct {
		name               string
		globalPassThrough  bool
		channelPassThrough bool
		wantErr            bool
	}{
		{"policy applies without pass-through", false, false, false},
		{"global pass-through bypasses the policy", true, false, true},
		{"channel pass-through bypasses the policy", false, true, true},
	}
	for _, tt := range tests {
		for _, endpoint := range []string{"openai", "anthropic"} {
			t.Run(tt.name+"/"+endpoint, func(t *testing.T) {
				setupFormatRejectionTest(t)
				if tt.channelPassThrough {
					require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 11).
						Update("setting", `{"pass_through_body_enabled":true}`).Error)
					model.InitChannelCache()
				}
				setChatToResponsesPolicy(t, codexPolicy, tt.globalPassThrough)
				route := formatRejectionRoute(endpoint)

				found := normalTierGroupErrors(smartroute.ValidateRoute(route))

				if tt.wantErr {
					require.Len(t, found, 1)
					return
				}
				assert.Empty(t, found)
			})
		}
	}
}
