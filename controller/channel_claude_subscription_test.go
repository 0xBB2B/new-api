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

func TestClaudeChannelEndpoints_ErrorMessagesAreWebConsoleKeys(t *testing.T) {
	setupTaskPluginBindChannelTest(t)

	validKey := `{"claudeAiOauth":{"accessToken":"tok-placeholder"}}`
	channels := map[string]*model.Channel{
		"wrong type": {Id: 101, Type: constant.ChannelTypeOpenAI, Key: validKey, Name: "wrong-type"},
		"multi key": {Id: 102, Type: constant.ChannelTypeClaudeSubscription, Key: validKey, Name: "multi-key",
			ChannelInfo: model.ChannelInfo{IsMultiKey: true}},
		"bad credentials": {Id: 103, Type: constant.ChannelTypeClaudeSubscription, Key: "not-json", Name: "bad-credentials"},
		"valid":           {Id: 104, Type: constant.ChannelTypeClaudeSubscription, Key: validKey, Name: "valid"},
	}
	for _, channel := range channels {
		require.NoError(t, model.DB.Create(channel).Error)
	}

	tests := []struct {
		name     string
		handler  gin.HandlerFunc
		id       string
		body     string
		wantText string
	}{
		{"reset id not integer", ResetClaudeChannelLimit, "abc", `{"program":"juniper_tide"}`, "Invalid parameters"},
		{"reset channel missing", ResetClaudeChannelLimit, "999999", `{"program":"juniper_tide"}`, "Channel does not exist"},
		{"reset wrong channel type", ResetClaudeChannelLimit, "101", `{"program":"juniper_tide"}`, "Channel type is not Claude Subscription"},
		{"reset multi key", ResetClaudeChannelLimit, "102", `{"program":"juniper_tide"}`, "Multi-key channels are not supported"},
		{"reset bad credentials", ResetClaudeChannelLimit, "103", `{"program":"juniper_tide"}`, "Failed to parse credentials. Please check the channel configuration"},
		{"reset body not json", ResetClaudeChannelLimit, "104", `not json`, "Invalid reset parameters"},
		{"reset unsupported program", ResetClaudeChannelLimit, "104", `{"program":"foo"}`, "Unsupported reset type"},
		{"usage id not integer", GetClaudeChannelUsage, "abc", "", "Invalid parameters"},
		{"usage channel missing", GetClaudeChannelUsage, "999999", "", "Channel does not exist"},
		{"usage wrong channel type", GetClaudeChannelUsage, "101", "", "Channel type is not Claude Subscription"},
		{"usage multi key", GetClaudeChannelUsage, "102", "", "Multi-key channels are not supported"},
		{"usage bad credentials", GetClaudeChannelUsage, "103", "", "Failed to parse credentials. Please check the channel configuration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Params = gin.Params{{Key: "id", Value: tt.id}}
			c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/"+tt.id+"/claude/usage", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			tt.handler(c)

			var resp struct {
				Success        bool           `json:"success"`
				Message        string         `json:"message"`
				MessageKey     string         `json:"message_key"`
				MessageParams  map[string]any `json:"message_params"`
				UpstreamStatus int            `json:"upstream_status"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
			assert.False(t, resp.Success)
			assert.Equal(t, tt.wantText, resp.Message)
			assert.Equal(t, tt.wantText, resp.MessageKey)
			assert.Empty(t, resp.MessageParams)
			assert.Zero(t, resp.UpstreamStatus)
			assert.Equal(t, http.StatusOK, recorder.Code)
		})
	}
}
