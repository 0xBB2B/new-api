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
	"sync/atomic"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/common"
)

const ReasoningTagPrefix = "rtag_"

type reasoningTagCrypto struct {
	secret   string
	gcm      cipher.AEAD
	nonceKey []byte
}

var reasoningTagCache atomic.Pointer[reasoningTagCrypto]

func reasoningTagKey(secret, purpose string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(purpose))
	return mac.Sum(nil)
}

func currentReasoningTagCrypto() *reasoningTagCrypto {
	secret := common.CryptoSecret
	if c := reasoningTagCache.Load(); c != nil && c.secret == secret {
		return c
	}
	block, err := aes.NewCipher(reasoningTagKey(secret, "new-api reasoning channel tag v1 aes"))
	if err != nil {
		panic(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	c := &reasoningTagCrypto{
		secret:   secret,
		gcm:      gcm,
		nonceKey: reasoningTagKey(secret, "new-api reasoning channel tag v1 nonce"),
	}
	reasoningTagCache.Store(c)
	return c
}

func EncodeReasoningTag(channelID int, original string) string {
	plain := binary.BigEndian.AppendUint64(nil, uint64(channelID))
	plain = append(plain, original...)

	// 随机数由明文派生：流式响应里同一个条目出现在多个事件中，必须得到同一个标记
	c := currentReasoningTagCrypto()
	mac := hmac.New(sha256.New, c.nonceKey)
	mac.Write(plain)
	nonce := mac.Sum(nil)[:c.gcm.NonceSize()]

	sealed := c.gcm.Seal(nonce, nonce, plain, nil)
	return ReasoningTagPrefix + base64.RawURLEncoding.EncodeToString(sealed)
}

func DecodeReasoningTag(tag string) (int, string, bool) {
	payload, ok := strings.CutPrefix(tag, ReasoningTagPrefix)
	if !ok {
		return 0, "", false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(payload)
	if err != nil {
		return 0, "", false
	}
	gcm := currentReasoningTagCrypto().gcm
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

func tagValueAt(data []byte, path string, channelID int) []byte {
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

func tagItemAt(data []byte, itemPath string, channelID int) []byte {
	data = tagValueAt(data, itemPath+".id", channelID)
	return tagValueAt(data, itemPath+".approval_request_id", channelID)
}

func tagItemOutput(data []byte, outputPath string, channelID int) []byte {
	for i := range ojson.GetBytes(data, outputPath).Array() {
		data = tagItemAt(data, outputPath+"."+strconv.Itoa(i), channelID)
	}
	return data
}

func TagResponsesItemBody(body []byte, channelID int) []byte {
	if channelID <= 0 || !bytes.Contains(body, []byte("id")) || !ojson.ValidBytes(body) {
		return body
	}
	return tagItemOutput(body, "output", channelID)
}

func TagResponsesItemEvent(data []byte, channelID int) []byte {
	if channelID <= 0 || !bytes.Contains(data, []byte("id")) || !ojson.ValidBytes(data) {
		return data
	}
	eventType := ojson.GetBytes(data, "type").String()
	if ojson.GetBytes(data, "item_id").Exists() {
		return tagValueAt(data, "item_id", channelID)
	}
	switch {
	case eventType == "response.output_item.added" || eventType == "response.output_item.done":
		return tagItemAt(data, "item", channelID)
	case eventType == "response.created", eventType == "response.in_progress",
		eventType == "response.completed", eventType == "response.incomplete", eventType == "response.failed",
		eventType == "response.done", eventType == "response.cancelled", eventType == "response.canceled":
		return tagItemOutput(data, "response.output", channelID)
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
			return tagValueAt(data, "content_block.signature", channelID)
		case "redacted_thinking":
			return tagValueAt(data, "content_block.data", channelID)
		}
	case "content_block_delta":
		if ojson.GetBytes(data, "delta.type").String() == "signature_delta" {
			return tagValueAt(data, "delta.signature", channelID)
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
			body = tagValueAt(body, prefix+".signature", channelID)
		case "redacted_thinking":
			body = tagValueAt(body, prefix+".data", channelID)
		}
	}
	return body
}
