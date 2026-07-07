package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetChannelTypeName_ClaudeSubscription(t *testing.T) {
	assert.Equal(t, "Claude Subscription", GetChannelTypeName(ChannelTypeClaudeSubscription))
}

func TestChannelBaseURLs_ClaudeSubscriptionDefault(t *testing.T) {
	require.Less(t, ChannelTypeClaudeSubscription, len(ChannelBaseURLs))
	require.GreaterOrEqual(t, ChannelTypeClaudeSubscription, 0)

	assert.Equal(t, "https://api.anthropic.com", ChannelBaseURLs[ChannelTypeClaudeSubscription])
}

func TestChannelTypeClaudeSubscription_IsDistinctType(t *testing.T) {
	assert.NotEqual(t, ChannelTypeAnthropic, ChannelTypeClaudeSubscription)
	assert.NotEqual(t, ChannelTypeCodex, ChannelTypeClaudeSubscription)
}
