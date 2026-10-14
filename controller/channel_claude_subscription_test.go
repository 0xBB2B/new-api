package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestResetClaudeChannelLimit_ErrorMessagesAreWebConsoleKeys(t *testing.T) {
	setupTaskPluginBindChannelTest(t)

	tests := []struct {
		name    string
		id      string
		wantMsg string
	}{
		{"id not integer", "abc", "Invalid parameters"},
		{"channel missing", "999999", "Channel does not exist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Params = gin.Params{{Key: "id", Value: tt.id}}
			c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/"+tt.id+"/claude/usage/reset", strings.NewReader(`{"program":"juniper_tide"}`))
			c.Request.Header.Set("Content-Type", "application/json")

			ResetClaudeChannelLimit(c)

			var resp struct {
				Success    bool   `json:"success"`
				Message    string `json:"message"`
				MessageKey string `json:"message_key"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
			assert.False(t, resp.Success)
			assert.Equal(t, tt.wantMsg, resp.Message)
			assert.Equal(t, tt.wantMsg, resp.MessageKey)
			assert.NotContains(t, resp.Message, "record not found")
		})
	}
}
