package service

import (
	"strconv"
	"strings"
	"sync"
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

func TestTagResponsesItemBody(t *testing.T) {
	swapCryptoSecret(t, "tag-body-secret")
	body := `{"id":"resp_1","output":[{"type":"reasoning","id":"rs_y","summary":[]},{"type":"message","id":"msg_1","content":[]}]}`

	var got map[string]any
	require.NoError(t, common.Unmarshal(TagResponsesItemBody([]byte(body), 5), &got))
	output := got["output"].([]any)
	reasoning := output[0].(map[string]any)
	decodeTaggedID(t, reasoning["id"].(string), 5, "rs_y")
	message := output[1].(map[string]any)
	decodeTaggedID(t, message["id"].(string), 5, "msg_1")

	reasoning["id"] = "rs_y"
	message["id"] = "msg_1"
	expected := map[string]any{}
	require.NoError(t, common.Unmarshal([]byte(body), &expected))
	assert.Equal(t, expected, got)
}

func TestTagResponsesItemEventSequence(t *testing.T) {
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
		require.NoError(t, common.Unmarshal(TagResponsesItemEvent([]byte(event), 5), &m))
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

	decodeTaggedID(t, parsed[2]["item_id"].(string), 5, "msg_1")
	decodeTaggedID(t, parsed[4]["response"].(map[string]any)["output"].([]any)[1].(map[string]any)["id"].(string), 5, "msg_1")
}

func TestTagResponsesItemEventLeavesBytesUntouched(t *testing.T) {
	swapCryptoSecret(t, "tag-untouched-secret")
	for _, tc := range []struct{ name, data string }{
		{"no id or item_id", `{"type":"response.in_progress",  "sequence_number":1}`},
		{"empty item_id", `{"type":"response.output_text.delta", "item_id":"","delta":"hi"}`},
		{"not json", `data: not json {`},
		{"truncated json with reasoning keyword", `{"type":"response.output_item.added","item":{"type":"reasoning","id":"rs_x"`},
		{"empty reasoning id", `{"type":"response.output_item.added", "item":{"type":"reasoning","id":""}}`},
		{"empty message id", `{"type":"response.output_item.added", "item":{"type":"message","id":""}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, []byte(tc.data), TagResponsesItemEvent([]byte(tc.data), 5))
		})
	}
	body := `{"output":[ {"type":"reasoning","id":""} ]}`
	assert.Equal(t, []byte(body), TagResponsesItemBody([]byte(body), 5))
	assert.Equal(t, []byte("nope"), TagResponsesItemBody([]byte("nope"), 5))
	truncated := `{"output":[{"type":"reasoning","id":"rs_x"}`
	assert.Equal(t, []byte(truncated), TagResponsesItemBody([]byte(truncated), 5))
}

func TestTagResponsesItemZeroChannelLeavesBytesUntouched(t *testing.T) {
	swapCryptoSecret(t, "tag-zero-secret")
	event := `{"type":"response.completed",  "response":{"output":[ {"type":"reasoning","id":"rs_y"} ]}}`
	item := `{"type":"response.output_item.added", "item":{"type":"reasoning","id":"rs_y"}}`
	body := `{"output":[ {"type":"reasoning","id":"rs_y"} ]}`

	assert.Equal(t, []byte(event), TagResponsesItemEvent([]byte(event), 0))
	assert.Equal(t, []byte(item), TagResponsesItemEvent([]byte(item), 0))
	assert.Equal(t, []byte(body), TagResponsesItemBody([]byte(body), 0))
}

func TestTagResponsesItemBodyAllTypes(t *testing.T) {
	swapCryptoSecret(t, "tag-all-types-secret")
	body := `{"id":"resp_1","output":[{"type":"reasoning","id":"rs_r","summary":[]},{"type":"custom_tool_call","id":"msg_aaa","call_id":"call_1","name":"exec","input":"x"},{"type":"message","id":"msg_bbb","role":"assistant","content":[]},{"type":"web_search_call","id":"ws_ccc","status":"completed"}]}`

	var got map[string]any
	require.NoError(t, common.Unmarshal(TagResponsesItemBody([]byte(body), 5), &got))
	assert.Equal(t, "resp_1", got["id"])
	output := got["output"].([]any)
	require.Len(t, output, 4)
	for i, original := range []string{"rs_r", "msg_aaa", "msg_bbb", "ws_ccc"} {
		item := output[i].(map[string]any)
		decodeTaggedID(t, item["id"].(string), 5, original)
		item["id"] = original
	}
	assert.Equal(t, "call_1", output[1].(map[string]any)["call_id"])

	expected := map[string]any{}
	require.NoError(t, common.Unmarshal([]byte(body), &expected))
	assert.Equal(t, expected, got)
}

func TestTagResponsesItemEventAllTypesSequence(t *testing.T) {
	swapCryptoSecret(t, "tag-all-events-secret")
	events := []string{
		`{"type":"response.output_item.added","item":{"type":"custom_tool_call","id":"msg_aaa","call_id":"call_1"}}`,
		`{"type":"response.custom_tool_call_input.delta","item_id":"msg_aaa","delta":"x"}`,
		`{"type":"response.output_text.delta","item_id":"msg_bbb","delta":"hi"}`,
		`{"type":"response.content_part.added","item_id":"msg_bbb","part":{"type":"output_text"}}`,
		`{"type":"response.output_item.done","item":{"type":"message","id":"msg_bbb"}}`,
		`{"type":"response.completed","response":{"id":"resp_1","output":[{"type":"custom_tool_call","id":"msg_aaa","call_id":"call_1"},{"type":"message","id":"msg_bbb"}]}}`,
	}
	var parsed []map[string]any
	for _, event := range events {
		var m map[string]any
		require.NoError(t, common.Unmarshal(TagResponsesItemEvent([]byte(event), 5), &m))
		parsed = append(parsed, m)
	}
	completed := parsed[5]["response"].(map[string]any)
	completedOutput := completed["output"].([]any)

	aaa := []string{
		parsed[0]["item"].(map[string]any)["id"].(string),
		parsed[1]["item_id"].(string),
		completedOutput[0].(map[string]any)["id"].(string),
	}
	bbb := []string{
		parsed[2]["item_id"].(string),
		parsed[3]["item_id"].(string),
		parsed[4]["item"].(map[string]any)["id"].(string),
		completedOutput[1].(map[string]any)["id"].(string),
	}
	for _, tag := range aaa {
		assert.Equal(t, aaa[0], tag)
	}
	for _, tag := range bbb {
		assert.Equal(t, bbb[0], tag)
	}
	decodeTaggedID(t, aaa[0], 5, "msg_aaa")
	decodeTaggedID(t, bbb[0], 5, "msg_bbb")
	assert.Equal(t, "resp_1", completed["id"])
	assert.Equal(t, "call_1", completedOutput[0].(map[string]any)["call_id"])
}

func TestTagResponsesItemEventDoneAndCancelled(t *testing.T) {
	swapCryptoSecret(t, "tag-terminal-secret")
	for _, eventType := range []string{"response.done", "response.cancelled", "response.canceled"} {
		t.Run(eventType, func(t *testing.T) {
			event := `{"type":"` + eventType + `","response":{"output":[{"type":"reasoning","id":"rs_y"},{"type":"message","id":"msg_1"}]}}`
			var got map[string]any
			require.NoError(t, common.Unmarshal(TagResponsesItemEvent([]byte(event), 5), &got))
			output := got["response"].(map[string]any)["output"].([]any)
			decodeTaggedID(t, output[0].(map[string]any)["id"].(string), 5, "rs_y")
			decodeTaggedID(t, output[1].(map[string]any)["id"].(string), 5, "msg_1")
		})
	}
}

func TestTagResponsesItemBodyApprovalRequestID(t *testing.T) {
	swapCryptoSecret(t, "tag-approval-body-secret")
	body := `{"id":"resp_1","output":[{"type":"mcp_approval_request","id":"mcpr_1","name":"tool"},{"type":"mcp_call","id":"mcp_2","approval_request_id":"mcpr_1","name":"tool"}]}`

	var got map[string]any
	require.NoError(t, common.Unmarshal(TagResponsesItemBody([]byte(body), 5), &got))
	output := got["output"].([]any)
	request := output[0].(map[string]any)
	call := output[1].(map[string]any)
	assert.Equal(t, request["id"], call["approval_request_id"])
	decodeTaggedID(t, request["id"].(string), 5, "mcpr_1")
	decodeTaggedID(t, call["id"].(string), 5, "mcp_2")
}

func TestTagResponsesItemEventApprovalRequestID(t *testing.T) {
	swapCryptoSecret(t, "tag-approval-event-secret")
	events := []string{
		`{"type":"response.output_item.added","item":{"type":"mcp_approval_request","id":"mcpr_1"}}`,
		`{"type":"response.output_item.done","item":{"type":"mcp_call","id":"mcp_2","approval_request_id":"mcpr_1"}}`,
	}
	var parsed []map[string]any
	for _, event := range events {
		var m map[string]any
		require.NoError(t, common.Unmarshal(TagResponsesItemEvent([]byte(event), 5), &m))
		parsed = append(parsed, m)
	}
	request := parsed[0]["item"].(map[string]any)
	call := parsed[1]["item"].(map[string]any)
	assert.Equal(t, request["id"], call["approval_request_id"])
	decodeTaggedID(t, request["id"].(string), 5, "mcpr_1")
	decodeTaggedID(t, call["id"].(string), 5, "mcp_2")
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

func TestReasoningTag_ConcurrentEncodeDecode(t *testing.T) {
	swapCryptoSecret(t, "concurrent-secret")

	inputs := []struct {
		channelID int
		original  string
	}{
		{1, reasoningTagTestSignature},
		{5, "rs_0123456789abcdef"},
		{123456, "msg_other"},
		{42, "x"},
	}
	want := make([]string, len(inputs))
	for i, in := range inputs {
		want[i] = EncodeReasoningTag(in.channelID, in.original)
	}

	const workers = 8
	var wg sync.WaitGroup
	tags := make([][]string, workers)
	decoded := make([][]bool, workers)
	for w := range workers {
		tags[w] = make([]string, len(inputs))
		decoded[w] = make([]bool, len(inputs))
		wg.Go(func() {
			for i, in := range inputs {
				tag := EncodeReasoningTag(in.channelID, in.original)
				tags[w][i] = tag
				channelID, original, ok := DecodeReasoningTag(tag)
				decoded[w][i] = ok && channelID == in.channelID && original == in.original
			}
		})
	}
	wg.Wait()

	for w := range workers {
		for i := range inputs {
			assert.Equal(t, want[i], tags[w][i], "worker %d input %d", w, i)
			assert.True(t, decoded[w][i], "worker %d input %d", w, i)
		}
	}
}
