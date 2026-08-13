package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSubscriptionChannel(t *testing.T) {
	tests := []struct {
		name        string
		channelType int
		want        bool
	}{
		{name: "codex", channelType: ChannelTypeCodex, want: true},
		{name: "claude subscription", channelType: ChannelTypeClaudeSubscription, want: true},
		{name: "zero value", channelType: 0, want: false},
		{name: "openai", channelType: ChannelTypeOpenAI, want: false},
		{name: "anthropic", channelType: ChannelTypeAnthropic, want: false},
		{name: "negative", channelType: -1, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsSubscriptionChannel(tt.channelType))
		})
	}
}
