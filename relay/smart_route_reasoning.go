package relay

import (
	"encoding/json"
	"slices"
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
			if isClaudeThinkingBlock(block["type"]) {
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

func isClaudeThinkingBlock(blockType any) bool {
	return blockType == "thinking" || blockType == "redacted_thinking"
}

func isToolResultOnlyBlocks(blockTypes []string) bool {
	return len(blockTypes) > 0 && !slices.ContainsFunc(blockTypes, func(t string) bool { return t != "tool_result" })
}

// 一轮从最近一条真正的用户提问开始，包括其中所有工具调用。只有请求以 tool_result 结尾（轮还没结束）时，
// 这一轮第一条 assistant 消息里的思考块被清掉才需要关思考；新提问开启新一轮，不受影响。
func claudeTurnFirstAssistant(n int, role func(int) string, blockTypes func(int) []string) int {
	if n == 0 || role(n-1) != "user" || !isToolResultOnlyBlocks(blockTypes(n-1)) {
		return -1
	}
	start := 0
	for i := n - 1; i >= 0; i-- {
		if role(i) == "user" && !isToolResultOnlyBlocks(blockTypes(i)) {
			start = i + 1
			break
		}
	}
	for j := start; j < n; j++ {
		if role(j) == "assistant" {
			return j
		}
	}
	return -1
}

func claudeBlockTypes(content any) []string {
	blocks, _ := content.([]any)
	types := make([]string, 0, len(blocks))
	for _, raw := range blocks {
		block, _ := raw.(map[string]any)
		blockType, _ := block["type"].(string)
		types = append(types, blockType)
	}
	return types
}

func claudeTurnLostThinking(request *dto.ClaudeRequest) bool {
	messages := request.Messages
	idx := claudeTurnFirstAssistant(len(messages),
		func(i int) string { return messages[i].Role },
		func(i int) []string { return claudeBlockTypes(messages[i].Content) })
	return idx >= 0 && slices.ContainsFunc(claudeBlockTypes(messages[idx].Content), func(t string) bool { return isClaudeThinkingBlock(t) })
}

// 关思考时把这次请求整体退回到客户端没设力度的状态，避免档位力度跟着一个已经关掉思考的请求一起发出去。
func disableClaudeThinking(request *dto.ClaudeRequest) error {
	if request.Thinking == nil || request.Thinking.Type != "enabled" {
		return nil
	}
	request.Thinking = &dto.Thinking{Type: "disabled"}
	if len(request.OutputConfig) == 0 {
		return nil
	}
	config, err := ojson.DeleteBytes(request.OutputConfig, "effort")
	if err != nil {
		return err
	}
	request.OutputConfig = config
	return nil
}

func stripClaudeThinkingBlocksInBody(raw []byte) ([]byte, int, error) {
	messages := ojson.GetBytes(raw, "messages").Array()
	stripped := 0
	lastTurnLost := false
	turnAssistant := claudeTurnFirstAssistant(len(messages),
		func(i int) string { return messages[i].Get("role").String() },
		func(i int) []string {
			blocks := messages[i].Get("content").Array()
			types := make([]string, 0, len(blocks))
			for _, block := range blocks {
				types = append(types, block.Get("type").String())
			}
			return types
		})
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
		lastTurnLost = lastTurnLost || i == turnAssistant
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
	if lastTurnLost && ojson.GetBytes(raw, "thinking.type").String() == "enabled" {
		if raw, err = ojson.SetRawBytes(raw, "thinking", []byte(`{"type":"disabled"}`)); err != nil {
			return nil, 0, err
		}
		if raw, err = ojson.DeleteBytes(raw, "output_config.effort"); err != nil {
			return nil, 0, err
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
