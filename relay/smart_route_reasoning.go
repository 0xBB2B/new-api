package relay

import (
	"encoding/json"
	"strconv"

	"github.com/MoeForge/moego/ojson"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/gin-gonic/gin"
)

func shouldStripSmartRouteReasoning(c *gin.Context, channelID int) bool {
	result, ok := smartroute.GetRouteResult(c)
	if !ok || result.Session == nil {
		return false
	}
	return result.Session.LastChannelID != channelID
}

// 思考条目的加密内容和服务端 id 只在签发它的账号里有效：带摘要的条目去掉 id 和 encrypted_content 保留摘要，没有摘要的整条丢弃。
func stripEncryptedReasoningInput(input json.RawMessage) (json.RawMessage, int) {
	var items []json.RawMessage
	if err := common.Unmarshal(input, &items); err != nil {
		return input, 0
	}
	kept := make([]json.RawMessage, 0, len(items))
	stripped := 0
	for _, item := range items {
		var fields map[string]json.RawMessage
		if err := common.Unmarshal(item, &fields); err != nil {
			kept = append(kept, item)
			continue
		}
		var itemType string
		if common.Unmarshal(fields["type"], &itemType) != nil || itemType != "reasoning" {
			kept = append(kept, item)
			continue
		}
		stripped++
		var summary []json.RawMessage
		if common.Unmarshal(fields["summary"], &summary) != nil || len(summary) == 0 {
			continue
		}
		delete(fields, "id")
		delete(fields, "encrypted_content")
		cleaned, err := common.Marshal(fields)
		if err != nil {
			kept = append(kept, item)
			stripped--
			continue
		}
		kept = append(kept, cleaned)
	}
	if stripped == 0 {
		return input, 0
	}
	out, err := common.Marshal(kept)
	if err != nil {
		return input, 0
	}
	return out, stripped
}

// 带签名的思考块只在签发它的账号里有效，换渠道后转发会被上游拒绝。去空的 assistant 消息整条删除，上游会合并相邻同角色消息。
func stripClaudeThinkingBlocks(request *dto.ClaudeRequest) int {
	stripped := 0
	messages := make([]dto.ClaudeMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		blocks, ok := message.Content.([]any)
		if !ok || message.Role != "assistant" {
			messages = append(messages, message)
			continue
		}
		kept := make([]any, 0, len(blocks))
		for _, raw := range blocks {
			block, _ := raw.(map[string]any)
			if t := block["type"]; t == "thinking" || t == "redacted_thinking" {
				stripped++
				continue
			}
			kept = append(kept, raw)
		}
		if len(kept) == len(blocks) {
			messages = append(messages, message)
			continue
		}
		if len(kept) == 0 {
			continue
		}
		message.Content = kept
		messages = append(messages, message)
	}
	request.Messages = messages
	return stripped
}

func stripClaudeThinkingBlocksInBody(raw []byte) ([]byte, int, error) {
	messages := ojson.GetBytes(raw, "messages").Array()
	stripped := 0
	var err error
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Get("role").String() != "assistant" {
			continue
		}
		blocks := messages[i].Get("content").Array()
		var thinking []int
		for j, block := range blocks {
			if t := block.Get("type").String(); t == "thinking" || t == "redacted_thinking" {
				thinking = append(thinking, j)
			}
		}
		if len(thinking) == 0 {
			continue
		}
		stripped += len(thinking)
		messagePath := "messages." + strconv.Itoa(i)
		if len(thinking) == len(blocks) {
			if raw, err = ojson.DeleteBytes(raw, messagePath); err != nil {
				return nil, 0, err
			}
			continue
		}
		for k := len(thinking) - 1; k >= 0; k-- {
			if raw, err = ojson.DeleteBytes(raw, messagePath+".content."+strconv.Itoa(thinking[k])); err != nil {
				return nil, 0, err
			}
		}
	}
	return raw, stripped, nil
}

func recordStrippedReasoning(c *gin.Context, count int) {
	result, ok := smartroute.GetRouteResult(c)
	if !ok || result.StrippedReasoningCount == count {
		return
	}
	result.StrippedReasoningCount = count
	smartroute.SetRouteResult(c, result)
}
