package relay

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const (
	formatResponses = string(constant.EndpointTypeOpenAIResponse)
	formatChat      = string(constant.EndpointTypeOpenAI)
	formatAnthropic = string(constant.EndpointTypeAnthropic)
)

// 这些类型是否拒收取决于密钥或渠道配置，没有固定的「渠道类型 → 拒收」关系，不进拒收表
var configDependentChannelTypes = map[int]string{
	constant.ChannelTypeTencent:        "按密钥格式分流：ak/sk 走原生适配器，单段 TokenHub 密钥走 OpenAI 适配器",
	constant.ChannelTypeVertexAi:       "Chat Completions 是否报错取决于模型",
	constant.ChannelTypeAdvancedCustom: "由渠道配置决定走哪个协议",
	constant.ChannelTypeVLLM:           "由渠道配置决定走哪个协议",
	constant.ChannelTypeSGLang:         "由渠道配置决定走哪个协议",
}

func adaptorRejectsFormat(t *testing.T, channelType int, format string) (rejected bool) {
	t.Helper()
	apiType, ok := common.ChannelType2APIType(channelType)
	if !ok {
		return false
	}
	adaptor := GetAdaptor(apiType)
	if adaptor == nil {
		return false
	}
	defer func() {
		if recover() != nil {
			rejected = true
		}
	}()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: channelType, ApiType: apiType}}
	adaptor.Init(info)
	var err error
	switch format {
	case formatResponses:
		_, err = adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{Model: "m"})
	case formatChat:
		_, err = adaptor.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{Model: "m"})
	default:
		_, err = adaptor.ConvertClaudeRequest(c, info, &dto.ClaudeRequest{Model: "m"})
	}
	return err != nil
}

func TestChannelTypeRejectsFormat_MatchesAdaptors(t *testing.T) {
	for channelType := 1; channelType <= constant.ChannelTypeDummy; channelType++ {
		if _, excluded := configDependentChannelTypes[channelType]; excluded {
			continue
		}
		for _, format := range []string{formatResponses, formatChat, formatAnthropic} {
			t.Run(fmt.Sprintf("%d/%s", channelType, format), func(t *testing.T) {
				assert.Equal(t, adaptorRejectsFormat(t, channelType, format), model.ChannelTypeRejectsFormat(channelType, format))
			})
		}
	}
}

func TestChannelTypeRejectsFormat_AcceptingTypes(t *testing.T) {
	assert.False(t, model.ChannelTypeRejectsFormat(constant.ChannelTypeOpenAI, formatAnthropic))
	assert.False(t, model.ChannelTypeRejectsFormat(constant.ChannelTypeCodex, formatResponses))
	assert.False(t, model.ChannelTypeRejectsFormat(constant.ChannelTypeTencent, formatResponses))
}
