package service

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/common"
)

const reasoningTagPrefix = "rtag_"

func reasoningTagKey(purpose string) []byte {
	mac := hmac.New(sha256.New, []byte(common.CryptoSecret))
	mac.Write([]byte(purpose))
	return mac.Sum(nil)
}

func newReasoningTagGCM() cipher.AEAD {
	block, err := aes.NewCipher(reasoningTagKey("new-api reasoning channel tag v1 aes"))
	if err != nil {
		panic(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return gcm
}

func EncodeReasoningTag(channelID int, original string) string {
	plain := binary.BigEndian.AppendUint64(nil, uint64(channelID))
	plain = append(plain, original...)

	// 随机数由明文派生：流式响应里同一条思考记录出现在多个事件中，必须得到同一个标记
	mac := hmac.New(sha256.New, reasoningTagKey("new-api reasoning channel tag v1 nonce"))
	mac.Write(plain)
	gcm := newReasoningTagGCM()
	nonce := mac.Sum(nil)[:gcm.NonceSize()]

	sealed := gcm.Seal(nonce, nonce, plain, nil)
	return reasoningTagPrefix + base64.RawURLEncoding.EncodeToString(sealed)
}

func DecodeReasoningTag(tag string) (int, string, bool) {
	payload, ok := strings.CutPrefix(tag, reasoningTagPrefix)
	if !ok {
		return 0, "", false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(payload)
	if err != nil {
		return 0, "", false
	}
	gcm := newReasoningTagGCM()
	if len(raw) < gcm.NonceSize() {
		return 0, "", false
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil || len(plain) < 8 {
		return 0, "", false
	}
	channelID := int(binary.BigEndian.Uint64(plain[:8]))
	if channelID <= 0 {
		return 0, "", false
	}
	return channelID, string(plain[8:]), true
}

func tagReasoningIDAt(data []byte, path string, channelID int) []byte {
	id := ojson.GetBytes(data, path)
	if id.Type != ojson.String || id.String() == "" {
		return data
	}
	out, err := ojson.SetBytes(data, path, EncodeReasoningTag(channelID, id.String()))
	if err != nil {
		return data
	}
	return out
}

func tagReasoningOutput(data []byte, outputPath string, channelID int) []byte {
	for i, item := range ojson.GetBytes(data, outputPath).Array() {
		if item.Get("type").String() != "reasoning" {
			continue
		}
		data = tagReasoningIDAt(data, outputPath+"."+strconv.Itoa(i)+".id", channelID)
	}
	return data
}

func TagResponsesReasoningBody(body []byte, channelID int) []byte {
	if channelID <= 0 || !bytes.Contains(body, []byte(`"reasoning"`)) || !ojson.ValidBytes(body) {
		return body
	}
	return tagReasoningOutput(body, "output", channelID)
}

func TagResponsesReasoningEvent(data []byte, channelID int) []byte {
	if channelID <= 0 || !bytes.Contains(data, []byte("reasoning")) || !ojson.ValidBytes(data) {
		return data
	}
	eventType := ojson.GetBytes(data, "type").String()
	switch {
	case eventType == "response.output_item.added" || eventType == "response.output_item.done":
		if ojson.GetBytes(data, "item.type").String() != "reasoning" {
			return data
		}
		return tagReasoningIDAt(data, "item.id", channelID)
	case strings.HasPrefix(eventType, "response.reasoning_"):
		return tagReasoningIDAt(data, "item_id", channelID)
	case eventType == "response.created", eventType == "response.in_progress",
		eventType == "response.completed", eventType == "response.incomplete", eventType == "response.failed",
		eventType == "response.done", eventType == "response.cancelled", eventType == "response.canceled":
		return tagReasoningOutput(data, "response.output", channelID)
	}
	return data
}

func TagClaudeThinkingEvent(data []byte, channelID int) []byte {
	if channelID <= 0 || !(bytes.Contains(data, []byte("signature")) || bytes.Contains(data, []byte("redacted_thinking"))) || !ojson.ValidBytes(data) {
		return data
	}
	switch ojson.GetBytes(data, "type").String() {
	case "content_block_start":
		switch ojson.GetBytes(data, "content_block.type").String() {
		case "thinking":
			return tagReasoningIDAt(data, "content_block.signature", channelID)
		case "redacted_thinking":
			return tagReasoningIDAt(data, "content_block.data", channelID)
		}
	case "content_block_delta":
		if ojson.GetBytes(data, "delta.type").String() == "signature_delta" {
			return tagReasoningIDAt(data, "delta.signature", channelID)
		}
	}
	return data
}

func TagClaudeThinkingBody(body []byte, channelID int) []byte {
	if channelID <= 0 || !(bytes.Contains(body, []byte("signature")) || bytes.Contains(body, []byte("redacted_thinking"))) || !ojson.ValidBytes(body) {
		return body
	}
	for i, block := range ojson.GetBytes(body, "content").Array() {
		prefix := "content." + strconv.Itoa(i)
		switch block.Get("type").String() {
		case "thinking":
			body = tagReasoningIDAt(body, prefix+".signature", channelID)
		case "redacted_thinking":
			body = tagReasoningIDAt(body, prefix+".data", channelID)
		}
	}
	return body
}
