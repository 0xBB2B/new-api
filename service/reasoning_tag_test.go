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
