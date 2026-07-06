package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
)

func TestChannelType2APIType_ClaudeSubscription(t *testing.T) {
	apiType, ok := ChannelType2APIType(constant.ChannelTypeClaudeSubscription)

	assert.True(t, ok)
	assert.Equal(t, constant.APITypeClaudeSubscription, apiType)
}

func TestChannelType2APIType_ClaudeSubscriptionDiffersFromAnthropicAndCodex(t *testing.T) {
	apiType, ok := ChannelType2APIType(constant.ChannelTypeClaudeSubscription)
	assert.True(t, ok)

	anthropicAPIType, _ := ChannelType2APIType(constant.ChannelTypeAnthropic)
	codexAPIType, _ := ChannelType2APIType(constant.ChannelTypeCodex)

	assert.NotEqual(t, anthropicAPIType, apiType)
	assert.NotEqual(t, codexAPIType, apiType)
}
