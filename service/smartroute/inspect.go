package smartroute

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/tokenkit"
)

const (
	endpointResponses = string(constant.EndpointTypeOpenAIResponse)
	endpointChat      = string(constant.EndpointTypeOpenAI)
	endpointAnthropic = string(constant.EndpointTypeAnthropic)

	maxLatestRunes  = 4000
	maxHistoryRunes = 200
)

type LocalFeatures struct {
	LatestRuneCount int
	TurnCount       int
	HasAttachment   bool
	HasCodeOrTable  bool
}

type Inspection struct {
	IsUserTurn     bool
	UserMessages   []string
	Features       LocalFeatures
	PromptCacheKey string
}

type ScorerInput struct {
	Latest  string
	History []string
}

func RequestEndpointFromPath(path string) string {
	switch path {
	case "/v1/responses":
		return endpointResponses
	case "/v1/chat/completions":
		return endpointChat
	case "/v1/messages":
		return endpointAnthropic
	}
	return ""
}

func InspectRequest(endpoint string, body []byte) (Inspection, error) {
	var req map[string]any
	if err := common.Unmarshal(body, &req); err != nil {
		return Inspection{}, err
	}

	var items []any
	if endpoint == endpointResponses {
		switch input := req["input"].(type) {
		case string:
			items = []any{map[string]any{"role": "user", "content": input}}
		case []any:
			items = input
		}
	} else {
		items, _ = req["messages"].([]any)
	}

	var in Inspection
	in.PromptCacheKey, _ = req["prompt_cache_key"].(string)
	lastIsUser := false
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		role, _ := item["role"].(string)
		itemType, _ := item["type"].(string)
		lastIsUser = role == "user" && (itemType == "" || itemType == "message")
		if !lastIsUser {
			continue
		}
		if endpoint == endpointAnthropic && isToolResultOnly(item["content"]) {
			lastIsUser = false
			continue
		}
		text, attachment := messageText(item["content"])
		in.UserMessages = append(in.UserMessages, text)
		in.Features.HasAttachment = attachment
	}
	in.IsUserTurn = lastIsUser

	in.Features.TurnCount = len(in.UserMessages)
	if n := len(in.UserMessages); n > 0 {
		latest := in.UserMessages[n-1]
		in.Features.LatestRuneCount = utf8.RuneCountInString(latest)
		in.Features.HasCodeOrTable = strings.Contains(latest, "```") || tableRows(latest) >= 2
	}
	return in, nil
}

func messageText(content any) (text string, attachment bool) {
	switch c := content.(type) {
	case string:
		return c, false
	case []any:
		var sb strings.Builder
		for _, raw := range c {
			part, _ := raw.(map[string]any)
			switch part["type"] {
			case "text", "input_text":
				s, _ := part["text"].(string)
				sb.WriteString(s)
			case "image_url", "input_image", "input_file", "file", "image", "document":
				attachment = true
			}
		}
		return sb.String(), attachment
	}
	return "", false
}

func isToolResultOnly(content any) bool {
	blocks, _ := content.([]any)
	if len(blocks) == 0 {
		return false
	}
	for _, raw := range blocks {
		part, _ := raw.(map[string]any)
		if part["type"] != "tool_result" {
			return false
		}
	}
	return true
}

func tableRows(s string) int {
	rows := 0
	for line := range strings.SplitSeq(s, "\n") {
		if strings.HasPrefix(line, "|") {
			rows++
		}
	}
	return rows
}

func truncateRunes(s string, limit int) string {
	count := 0
	for i := range s {
		if count == limit {
			return s[:i]
		}
		count++
	}
	return s
}

func BuildScorerInput(in Inspection, n int) ScorerInput {
	last := len(in.UserMessages) - 1
	if last < 0 {
		return ScorerInput{}
	}
	out := ScorerInput{Latest: truncateRunes(in.UserMessages[last], maxLatestRunes)}
	for _, m := range in.UserMessages[max(0, last-n):last] {
		out.History = append(out.History, truncateRunes(m, maxHistoryRunes))
	}
	return out
}

func SessionKey(tokenID int, in Inspection) string {
	return sessionKey(strconv.Itoa(tokenID), in)
}

// SessionKeyForUser 给没有令牌的请求（演练场）用，按用户区分会话。
func SessionKeyForUser(userID int, in Inspection) string {
	return sessionKey("u"+strconv.Itoa(userID), in)
}

func sessionKey(owner string, in Inspection) string {
	var seed string
	switch {
	case in.PromptCacheKey != "":
		seed = owner + "|pck|" + in.PromptCacheKey
	case len(in.UserMessages) > 0 && in.UserMessages[0] != "":
		seed = owner + "|first|" + in.UserMessages[0]
	default:
		return ""
	}
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func EstimateInputTokens(endpoint string, body []byte, model string) int {
	var req interface{ GetTokenCountMeta() *types.TokenCountMeta }
	switch endpoint {
	case endpointChat:
		req = &dto.GeneralOpenAIRequest{}
	case endpointResponses:
		req = &dto.OpenAIResponsesRequest{}
	case endpointAnthropic:
		req = &dto.ClaudeRequest{}
	default:
		return 0
	}
	if err := common.Unmarshal(body, req); err != nil {
		return 0
	}
	meta := req.GetTokenCountMeta()
	tokens := tokenkit.Estimate(model, meta.CombineText)
	// meta 漏记的文件附件：Claude 的 document 块、Responses 没有 file_url 的 input_file
	switch r := req.(type) {
	case *dto.ClaudeRequest:
		for _, m := range r.Messages {
			parts, err := m.ParseContent()
			if err != nil {
				continue
			}
			for _, part := range parts {
				if part.Type == "document" {
					meta.Files = append(meta.Files, &types.FileMeta{FileType: types.FileTypeFile})
				}
			}
		}
	case *dto.OpenAIResponsesRequest:
		for _, in := range r.ParseInput() {
			if in.Type == "input_file" && in.FileUrl == "" {
				meta.Files = append(meta.Files, &types.FileMeta{FileType: types.FileTypeFile})
			}
		}
	}
	for _, file := range meta.Files {
		switch file.FileType {
		case types.FileTypeImage:
			tokens += tokenkit.ImageTokens(model, tokenkit.Image{Detail: file.Detail})
		case types.FileTypeAudio:
			tokens += 256
		case types.FileTypeVideo:
			tokens += 4096 * 2
		default:
			tokens += 4096
		}
	}
	return tokens
}
