package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"strings"

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
	raw, err := base64.RawURLEncoding.DecodeString(payload)
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
