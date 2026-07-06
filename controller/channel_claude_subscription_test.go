package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestValidateChannel_ClaudeSubscription_RejectsNonJSONKey(t *testing.T) {
	channel := &model.Channel{
		Type:   constant.ChannelTypeClaudeSubscription,
		Key:    "sk-plain-string",
		Models: "claude-3-5-sonnet",
	}

	err := validateChannel(channel, true)

	assert.Error(t, err)
}

func TestValidateChannel_ClaudeSubscription_RejectsMissingAccessToken(t *testing.T) {
	channel := &model.Channel{
		Type:   constant.ChannelTypeClaudeSubscription,
		Key:    `{"claudeAiOauth":{"refreshToken":"rt"}}`,
		Models: "claude-3-5-sonnet",
	}

	err := validateChannel(channel, true)

	assert.Error(t, err)
}

func TestValidateChannel_ClaudeSubscription_AcceptsValidOAuthKey(t *testing.T) {
	channel := &model.Channel{
		Type:   constant.ChannelTypeClaudeSubscription,
		Key:    `{"claudeAiOauth":{"accessToken":"x"}}`,
		Models: "claude-3-5-sonnet",
	}

	err := validateChannel(channel, true)

	assert.NoError(t, err)
}
