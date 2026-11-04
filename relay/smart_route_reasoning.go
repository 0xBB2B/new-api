package relay

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
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

func recordStrippedReasoning(c *gin.Context, count int) {
	if count == 0 {
		return
	}
	result, ok := smartroute.GetRouteResult(c)
	if !ok {
		return
	}
	result.StrippedReasoningCount = count
	smartroute.SetRouteResult(c, result)
}
