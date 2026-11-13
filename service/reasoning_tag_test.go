package service

import (
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const reasoningTagTestSignature = "msg_f836e1c0-0b4b-47c7-b1f0-3f05f59b2880"

func swapCryptoSecret(t *testing.T, secret string) {
	t.Helper()
	original := common.CryptoSecret
	common.CryptoSecret = secret
	t.Cleanup(func() { common.CryptoSecret = original })
}

func TestReasoningTag_RoundTrip(t *testing.T) {
	tag := EncodeReasoningTag(5, reasoningTagTestSignature)

	assert.True(t, strings.HasPrefix(tag, "rtag_"))
	assert.NotEqual(t, reasoningTagTestSignature, tag)
	assert.NotContains(t, tag, reasoningTagTestSignature)

	channelID, original, ok := DecodeReasoningTag(tag)
	require.True(t, ok)
	assert.Equal(t, 5, channelID)
	assert.Equal(t, reasoningTagTestSignature, original)
}

func TestReasoningTag_NoPlainChannelID(t *testing.T) {
	tag := EncodeReasoningTag(123456, reasoningTagTestSignature)
	assert.NotContains(t, strings.TrimPrefix(tag, "rtag_"), strconv.Itoa(123456))
}

func TestReasoningTag_Deterministic(t *testing.T) {
	assert.Equal(t,
		EncodeReasoningTag(5, reasoningTagTestSignature),
		EncodeReasoningTag(5, reasoningTagTestSignature))

	assert.NotEqual(t,
		EncodeReasoningTag(5, "sig-a"),
		EncodeReasoningTag(5, "sig-b"))

	assert.NotEqual(t,
		EncodeReasoningTag(5, reasoningTagTestSignature),
		EncodeReasoningTag(6, reasoningTagTestSignature))
}

func TestReasoningTag_PreservesOriginalBytes(t *testing.T) {
	for _, original := range []string{"sig+/==-_中", "", "a", "+++///===", "---___", "多字节😀"} {
		channelID, got, ok := DecodeReasoningTag(EncodeReasoningTag(9, original))
		require.True(t, ok, "original=%q", original)
		assert.Equal(t, 9, channelID)
		assert.Equal(t, original, got)
	}
}

func TestReasoningTag_SecretChange(t *testing.T) {
	swapCryptoSecret(t, "secret-one")
	tag := EncodeReasoningTag(5, reasoningTagTestSignature)

	swapCryptoSecret(t, "secret-two")
	_, _, ok := DecodeReasoningTag(tag)
	assert.False(t, ok)

	swapCryptoSecret(t, "secret-one")
	channelID, original, ok := DecodeReasoningTag(tag)
	require.True(t, ok)
	assert.Equal(t, 5, channelID)
	assert.Equal(t, reasoningTagTestSignature, original)
}

func TestReasoningTag_NonPositiveChannelIDRejected(t *testing.T) {
	for _, channelID := range []int{0, -1} {
		_, _, ok := DecodeReasoningTag(EncodeReasoningTag(channelID, "x"))
		assert.False(t, ok, "channelID=%d", channelID)
	}
}

func TestReasoningTag_InvalidTagRejected(t *testing.T) {
	valid := EncodeReasoningTag(5, reasoningTagTestSignature)
	last := valid[len(valid)-1]
	swapped := "A"
	if last == 'A' {
		swapped = "B"
	}

	tests := []struct {
		name string
		tag  string
	}{
		{"last char changed", valid[:len(valid)-1] + swapped},
		{"first payload char changed", "rtag_" + swapped + valid[len("rtag_")+1:]},
		{"prefix stripped", strings.TrimPrefix(valid, "rtag_")},
		{"empty", ""},
		{"prefix only", "rtag_"},
		{"non base64 payload", "rtag_!!!"},
		{"other vendor signature", "rs_abc"},
		{"truncated", valid[:len(valid)-4]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				_, _, ok := DecodeReasoningTag(tt.tag)
				assert.False(t, ok)
			})
		})
	}
}

func decodeTaggedID(t *testing.T, tag string, wantChannel int, wantOriginal string) {
	t.Helper()
	channelID, original, ok := DecodeReasoningTag(tag)
	require.True(t, ok, "tag=%q", tag)
	assert.Equal(t, wantChannel, channelID)
	assert.Equal(t, wantOriginal, original)
}

func TestTagResponsesReasoningBody(t *testing.T) {
	swapCryptoSecret(t, "tag-body-secret")
	body := `{"id":"resp_1","output":[{"type":"reasoning","id":"rs_y","summary":[]},{"type":"message","id":"msg_1","content":[]}]}`

	var got map[string]any
	require.NoError(t, common.Unmarshal(TagResponsesReasoningBody([]byte(body), 5), &got))
	output := got["output"].([]any)
	reasoning := output[0].(map[string]any)
	decodeTaggedID(t, reasoning["id"].(string), 5, "rs_y")
	assert.Equal(t, "msg_1", output[1].(map[string]any)["id"])

	reasoning["id"] = "rs_y"
	expected := map[string]any{}
	require.NoError(t, common.Unmarshal([]byte(body), &expected))
	assert.Equal(t, expected, got)
}

func TestTagResponsesReasoningEventSequence(t *testing.T) {
	swapCryptoSecret(t, "tag-event-secret")
	events := []string{
		`{"type":"response.output_item.added","item":{"type":"reasoning","id":"rs_y"}}`,
		`{"type":"response.reasoning_summary_text.delta","item_id":"rs_y","delta":"x"}`,
		`{"type":"response.output_text.delta","item_id":"msg_1","delta":"hi"}`,
		`{"type":"response.output_item.done","item":{"type":"reasoning","id":"rs_y"}}`,
		`{"type":"response.completed","response":{"output":[{"type":"reasoning","id":"rs_y"},{"type":"message","id":"msg_1"}]}}`,
	}
	var parsed []map[string]any
	for _, event := range events {
		var m map[string]any
		require.NoError(t, common.Unmarshal(TagResponsesReasoningEvent([]byte(event), 5), &m))
		parsed = append(parsed, m)
	}

	tags := []string{
		parsed[0]["item"].(map[string]any)["id"].(string),
		parsed[1]["item_id"].(string),
		parsed[3]["item"].(map[string]any)["id"].(string),
		parsed[4]["response"].(map[string]any)["output"].([]any)[0].(map[string]any)["id"].(string),
	}
	for _, tag := range tags {
		assert.Equal(t, tags[0], tag)
	}
	decodeTaggedID(t, tags[0], 5, "rs_y")

	assert.Equal(t, "msg_1", parsed[2]["item_id"])
	assert.Equal(t, "msg_1", parsed[4]["response"].(map[string]any)["output"].([]any)[1].(map[string]any)["id"])
}

func TestTagResponsesReasoningEventLeavesBytesUntouched(t *testing.T) {
	swapCryptoSecret(t, "tag-untouched-secret")
	for _, tc := range []struct{ name, data string }{
		{"no reasoning", `{"type":"response.output_text.delta",  "item_id":"msg_1","delta":"hi"}`},
		{"not json", `data: not json {`},
		{"truncated json with reasoning keyword", `{"type":"response.output_item.added","item":{"type":"reasoning","id":"rs_x"`},
		{"empty reasoning id", `{"type":"response.output_item.added", "item":{"type":"reasoning","id":""}}`},
		{"message item", `{"type":"response.output_item.added", "item":{"type":"message","id":"msg_1"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, []byte(tc.data), TagResponsesReasoningEvent([]byte(tc.data), 5))
		})
	}
	body := `{"output":[ {"type":"reasoning","id":""} ]}`
	assert.Equal(t, []byte(body), TagResponsesReasoningBody([]byte(body), 5))
	assert.Equal(t, []byte("nope"), TagResponsesReasoningBody([]byte("nope"), 5))
	truncated := `{"output":[{"type":"reasoning","id":"rs_x"}`
	assert.Equal(t, []byte(truncated), TagResponsesReasoningBody([]byte(truncated), 5))
}

func TestTagResponsesReasoningZeroChannelLeavesBytesUntouched(t *testing.T) {
	swapCryptoSecret(t, "tag-zero-secret")
	event := `{"type":"response.completed",  "response":{"output":[ {"type":"reasoning","id":"rs_y"} ]}}`
	item := `{"type":"response.output_item.added", "item":{"type":"reasoning","id":"rs_y"}}`
	body := `{"output":[ {"type":"reasoning","id":"rs_y"} ]}`

	assert.Equal(t, []byte(event), TagResponsesReasoningEvent([]byte(event), 0))
	assert.Equal(t, []byte(item), TagResponsesReasoningEvent([]byte(item), 0))
	assert.Equal(t, []byte(body), TagResponsesReasoningBody([]byte(body), 0))
}

func TestTagResponsesReasoningEventDoneAndCancelled(t *testing.T) {
	swapCryptoSecret(t, "tag-terminal-secret")
	for _, eventType := range []string{"response.done", "response.cancelled", "response.canceled"} {
		t.Run(eventType, func(t *testing.T) {
			event := `{"type":"` + eventType + `","response":{"output":[{"type":"reasoning","id":"rs_y"},{"type":"message","id":"msg_1"}]}}`
			var got map[string]any
			require.NoError(t, common.Unmarshal(TagResponsesReasoningEvent([]byte(event), 5), &got))
			output := got["response"].(map[string]any)["output"].([]any)
			decodeTaggedID(t, output[0].(map[string]any)["id"].(string), 5, "rs_y")
			assert.Equal(t, "msg_1", output[1].(map[string]any)["id"])
		})
	}
}

func TestTagClaudeThinkingBody(t *testing.T) {
	body := `{"type":"message","content":[{"type":"thinking","thinking":"T","signature":"sig_z"},{"type":"redacted_thinking","data":"d+/="},{"type":"text","text":"hi"}]}`

	var got struct {
		Content []map[string]any `json:"content"`
	}
	require.NoError(t, common.Unmarshal(TagClaudeThinkingBody([]byte(body), 7), &got))
	require.Len(t, got.Content, 3)

	channelID, original, ok := DecodeReasoningTag(got.Content[0]["signature"].(string))
	require.True(t, ok)
	assert.Equal(t, 7, channelID)
	assert.Equal(t, "sig_z", original)
	assert.Equal(t, "T", got.Content[0]["thinking"])

	channelID, original, ok = DecodeReasoningTag(got.Content[1]["data"].(string))
	require.True(t, ok)
	assert.Equal(t, 7, channelID)
	assert.Equal(t, "d+/=", original)

	assert.Equal(t, "hi", got.Content[2]["text"])
}

func TestTagClaudeThinkingEvent(t *testing.T) {
	tagged := func(t *testing.T, out []byte, path ...string) (int, string, bool) {
		t.Helper()
		var node any
		require.NoError(t, common.Unmarshal(out, &node))
		for _, key := range path {
			node = node.(map[string]any)[key]
		}
		return DecodeReasoningTag(node.(string))
	}

	t.Run("signature_delta", func(t *testing.T) {
		out := TagClaudeThinkingEvent([]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig_z"}}`), 7)
		channelID, original, ok := tagged(t, out, "delta", "signature")
		require.True(t, ok)
		assert.Equal(t, 7, channelID)
		assert.Equal(t, "sig_z", original)
	})

	t.Run("redacted_thinking start", func(t *testing.T) {
		out := TagClaudeThinkingEvent([]byte(`{"type":"content_block_start","index":1,"content_block":{"type":"redacted_thinking","data":"d1"}}`), 7)
		channelID, original, ok := tagged(t, out, "content_block", "data")
		require.True(t, ok)
		assert.Equal(t, 7, channelID)
		assert.Equal(t, "d1", original)
	})

	unchanged := []struct {
		name      string
		data      string
		channelID int
	}{
		{"empty signature", `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`, 7},
		{"thinking_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"T"}}`, 7},
		{"no thinking block", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`, 7},
		{"not json", `event: ping`, 7},
		{"truncated json with signature keyword", `{"type":"content_block_delta","delta":{"type":"signature_delta","signature":"x"`, 7},
		{"channel zero", `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig_z"}}`, 0},
	}
	for _, tt := range unchanged {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.data, string(TagClaudeThinkingEvent([]byte(tt.data), tt.channelID)))
		})
	}
}

func TestTagClaudeThinkingBody_Unchanged(t *testing.T) {
	cases := map[string]struct {
		body      string
		channelID int
	}{
		"no thinking":  {`{"type":"message","content":[{"type":"text","text":"hi"}]}`, 7},
		"not json":     {`not json`, 7},
		"channel zero": {`{"type":"message","content":[{"type":"thinking","thinking":"T","signature":"sig_z"}]}`, 0},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.body, string(TagClaudeThinkingBody([]byte(tt.body), tt.channelID)))
		})
	}
}
