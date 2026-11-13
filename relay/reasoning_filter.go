package relay

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/MoeForge/moego/ojson"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/gin-gonic/gin"
)

// 思考记录的 id、签名和加密内容只在签发它的渠道里有效：标记指向目标渠道的还原成原值，其余的清理。
// 清理后带摘要的条目保留摘要，没有摘要的整条丢弃。
func filterResponsesReasoningInput(input json.RawMessage, channelID int) (json.RawMessage, int, bool) {
	var items []json.RawMessage
	if err := common.Unmarshal(input, &items); err != nil {
		return input, 0, false
	}
	kept := make([]json.RawMessage, 0, len(items))
	stripped := 0
	changed := false
	for _, item := range items {
		var fields map[string]json.RawMessage
		if err := common.Unmarshal(item, &fields); err != nil {
			kept = append(kept, item)
			continue
		}
		var itemType string
		if common.Unmarshal(fields["type"], &itemType) != nil || (itemType != "reasoning" && itemType != "item_reference") {
			kept = append(kept, item)
			continue
		}
		var id string
		_ = common.Unmarshal(fields["id"], &id)
		if itemType == "item_reference" && !strings.HasPrefix(id, service.ReasoningTagPrefix) {
			kept = append(kept, item)
			continue
		}
		if tagChannel, original, ok := service.DecodeReasoningTag(id); ok && tagChannel == channelID {
			fields["id"], _ = common.Marshal(original)
			restored, err := common.Marshal(fields)
			if err != nil {
				kept = append(kept, item)
				continue
			}
			kept = append(kept, restored)
			changed = true
			continue
		}
		if itemType == "item_reference" {
			stripped++
			changed = true
			continue
		}
		var summary []json.RawMessage
		if common.Unmarshal(fields["summary"], &summary) != nil || len(summary) == 0 {
			stripped++
			changed = true
			continue
		}
		if _, hasID := fields["id"]; !hasID {
			if _, hasEnc := fields["encrypted_content"]; !hasEnc {
				kept = append(kept, item)
				continue
			}
		}
		delete(fields, "id")
		delete(fields, "encrypted_content")
		cleaned, err := common.Marshal(fields)
		if err != nil {
			kept = append(kept, item)
			continue
		}
		kept = append(kept, cleaned)
		stripped++
		changed = true
	}
	if !changed {
		return input, 0, false
	}
	out, err := common.Marshal(kept)
	if err != nil {
		return input, 0, false
	}
	return out, stripped, true
}

// 思考块的签名只在签发它的渠道里有效：标记指向目标渠道的还原成原值，其余的去掉。去空的 assistant 消息整条删除，上游会合并相邻同角色消息。
// 返回清理的块数，以及这一轮第一条 assistant 消息是否丢了思考块。
func filterClaudeThinkingBlocks(request *dto.ClaudeRequest, channelID int) (int, bool) {
	turnAssistant := claudeTurnFirstAssistant(len(request.Messages),
		func(i int) string { return request.Messages[i].Role },
		func(i int) []string { return claudeBlockTypes(request.Messages[i].Content) })
	stripped := 0
	lostFirst := false
	messages := make([]dto.ClaudeMessage, 0, len(request.Messages))
	for i, message := range request.Messages {
		blocks, ok := message.Content.([]any)
		if !ok || message.Role != "assistant" {
			messages = append(messages, message)
			continue
		}
		kept := make([]any, 0, len(blocks))
		removed := 0
		for _, raw := range blocks {
			block, _ := raw.(map[string]any)
			blockType, _ := block["type"].(string)
			if !isClaudeThinkingBlock(blockType) {
				kept = append(kept, raw)
				continue
			}
			field := "signature"
			if blockType == "redacted_thinking" {
				field = "data"
			}
			tag, _ := block[field].(string)
			if tagChannel, original, ok := service.DecodeReasoningTag(tag); ok && tagChannel == channelID {
				block[field] = original
				kept = append(kept, raw)
				continue
			}
			removed++
		}
		stripped += removed
		if removed > 0 && i == turnAssistant {
			lostFirst = true
		}
		if len(kept) == 0 {
			continue
		}
		if removed > 0 {
			message.Content = kept
		}
		messages = append(messages, message)
	}
	request.Messages = messages
	return stripped, lostFirst
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

func filterClaudeThinkingBlocksInBody(raw []byte, channelID int) ([]byte, int, bool, error) {
	messages := ojson.GetBytes(raw, "messages").Array()
	stripped := 0
	changed := false
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
		messagePath := "messages." + strconv.Itoa(i)
		blocks := messages[i].Get("content").Array()
		var removed []int
		for j, block := range blocks {
			blockType := block.Get("type").String()
			if !isClaudeThinkingBlock(blockType) {
				continue
			}
			field := "signature"
			if blockType == "redacted_thinking" {
				field = "data"
			}
			if tagChannel, original, ok := service.DecodeReasoningTag(block.Get(field).String()); ok && tagChannel == channelID {
				if raw, err = ojson.SetBytes(raw, messagePath+".content."+strconv.Itoa(j)+"."+field, original); err != nil {
					return nil, 0, false, err
				}
				changed = true
				continue
			}
			removed = append(removed, j)
		}
		if len(removed) == 0 {
			continue
		}
		changed = true
		lastTurnLost = lastTurnLost || i == turnAssistant
		stripped += len(removed)
		if len(removed) == len(blocks) {
			if raw, err = ojson.DeleteBytes(raw, messagePath); err != nil {
				return nil, 0, false, err
			}
			continue
		}
		for k := len(removed) - 1; k >= 0; k-- {
			if raw, err = ojson.DeleteBytes(raw, messagePath+".content."+strconv.Itoa(removed[k])); err != nil {
				return nil, 0, false, err
			}
		}
	}
	if lastTurnLost && ojson.GetBytes(raw, "thinking.type").String() == "enabled" {
		if raw, err = ojson.SetRawBytes(raw, "thinking", []byte(`{"type":"disabled"}`)); err != nil {
			return nil, 0, false, err
		}
		if raw, err = ojson.DeleteBytes(raw, "output_config.effort"); err != nil {
			return nil, 0, false, err
		}
	}
	return raw, stripped, changed, nil
}

func recordStrippedReasoning(c *gin.Context, count int) {
	if count > 0 {
		logger.LogInfo(c, common.LogText("stripped %d reasoning segments not issued by this channel", count))
	}
	result, ok := smartroute.GetRouteResult(c)
	if !ok || result.StrippedReasoningCount == count {
		return
	}
	result.StrippedReasoningCount = count
	smartroute.SetRouteResult(c, result)
}
