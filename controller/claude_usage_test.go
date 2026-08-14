package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type claudeUsageEnvelope struct {
	Success        bool            `json:"success"`
	Message        string          `json:"message"`
	UpstreamStatus int             `json:"upstream_status"`
	Data           json.RawMessage `json:"data"`
}

func decodeClaudeUsageResponse(t *testing.T, recorder *httptest.ResponseRecorder) (claudeUsageEnvelope, map[string]json.RawMessage) {
	t.Helper()
	var envelope claudeUsageEnvelope
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &envelope))
	var raw map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &raw))
	return envelope, raw
}

func callGetClaudeChannelUsage(t *testing.T, channelId int) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", channelId)}}
	ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/channel/%d/claude/usage", channelId), nil)

	GetClaudeChannelUsage(ctx)

	return recorder
}

func TestGetClaudeChannelUsage_Success_ReturnsUpstreamDataAndSubscriptionType(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	var gotAuth, gotBeta, gotUA string
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		gotAuth = r.Header.Get("Authorization")
		gotBeta = r.Header.Get("anthropic-beta")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":41}}`))
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    constant.ChannelTypeClaudeSubscription,
		Key:     `{"claudeAiOauth":{"accessToken":"tk","subscriptionType":"max"}}`,
		BaseURL: common.GetPointer(server.URL),
	}
	require.NoError(t, db.Create(channel).Error)

	recorder := callGetClaudeChannelUsage(t, channel.Id)

	require.Equal(t, http.StatusOK, recorder.Code)
	envelope, raw := decodeClaudeUsageResponse(t, recorder)
	assert.True(t, envelope.Success)
	assert.Equal(t, http.StatusOK, envelope.UpstreamStatus)
	assert.JSONEq(t, `{"five_hour":{"utilization":41}}`, string(envelope.Data))

	var subscriptionType string
	require.Contains(t, raw, "subscription_type")
	require.NoError(t, common.Unmarshal(raw["subscription_type"], &subscriptionType))
	assert.Equal(t, "max", subscriptionType)

	assert.Equal(t, 1, requestCount)
	assert.Equal(t, "Bearer tk", gotAuth)
	assert.Equal(t, "oauth-2025-04-20", gotBeta)
	assert.Contains(t, gotUA, "claude-code/")
}

func TestGetClaudeChannelUsage_NoSubscriptionType_OmitsFieldFromResponse(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":10}}`))
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    constant.ChannelTypeClaudeSubscription,
		Key:     `{"claudeAiOauth":{"accessToken":"tk"}}`,
		BaseURL: common.GetPointer(server.URL),
	}
	require.NoError(t, db.Create(channel).Error)

	recorder := callGetClaudeChannelUsage(t, channel.Id)

	_, raw := decodeClaudeUsageResponse(t, recorder)
	_, present := raw["subscription_type"]
	assert.False(t, present)
}

func TestGetClaudeChannelUsage_UpstreamUnauthorizedWithoutRefreshToken_ReturnsFailureAndDoesNotRetry(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    constant.ChannelTypeClaudeSubscription,
		Key:     `{"claudeAiOauth":{"accessToken":"tk"}}`,
		BaseURL: common.GetPointer(server.URL),
	}
	require.NoError(t, db.Create(channel).Error)

	recorder := callGetClaudeChannelUsage(t, channel.Id)

	envelope, _ := decodeClaudeUsageResponse(t, recorder)
	assert.False(t, envelope.Success)
	assert.Equal(t, http.StatusUnauthorized, envelope.UpstreamStatus)
	assert.Equal(t, "upstream status: 401", envelope.Message)
	assert.Equal(t, 1, requestCount)
}

func TestGetClaudeChannelUsage_UpstreamNonJSONBody_ReturnsRawStringData(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    constant.ChannelTypeClaudeSubscription,
		Key:     `{"claudeAiOauth":{"accessToken":"tk"}}`,
		BaseURL: common.GetPointer(server.URL),
	}
	require.NoError(t, db.Create(channel).Error)

	recorder := callGetClaudeChannelUsage(t, channel.Id)

	envelope, _ := decodeClaudeUsageResponse(t, recorder)
	assert.True(t, envelope.Success)
	var data string
	require.NoError(t, common.Unmarshal(envelope.Data, &data))
	assert.Equal(t, "not-json", data)
}

func TestGetClaudeChannelUsage_ChannelNotFound_ReturnsFailure(t *testing.T) {
	setupModelListControllerTestDB(t)

	recorder := callGetClaudeChannelUsage(t, 999999)

	envelope, _ := decodeClaudeUsageResponse(t, recorder)
	assert.False(t, envelope.Success)
	assert.Equal(t, "record not found", envelope.Message)
}

func TestGetClaudeChannelUsage_WrongChannelType_ReturnsFailureWithoutUpstreamRequest(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:    constant.ChannelTypeOpenAI,
		Key:     `{"claudeAiOauth":{"accessToken":"tk"}}`,
		BaseURL: common.GetPointer(server.URL),
	}
	require.NoError(t, db.Create(channel).Error)

	recorder := callGetClaudeChannelUsage(t, channel.Id)

	envelope, _ := decodeClaudeUsageResponse(t, recorder)
	assert.False(t, envelope.Success)
	assert.Equal(t, "channel type is not Claude Subscription", envelope.Message)
	assert.Equal(t, 0, requestCount)
}

func TestGetClaudeChannelUsage_MultiKeyChannel_ReturnsFailureWithoutUpstreamRequest(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel := &model.Channel{
		Type:        constant.ChannelTypeClaudeSubscription,
		Key:         `{"claudeAiOauth":{"accessToken":"tk"}}`,
		BaseURL:     common.GetPointer(server.URL),
		ChannelInfo: model.ChannelInfo{IsMultiKey: true},
	}
	require.NoError(t, db.Create(channel).Error)

	recorder := callGetClaudeChannelUsage(t, channel.Id)

	envelope, _ := decodeClaudeUsageResponse(t, recorder)
	assert.False(t, envelope.Success)
	assert.Equal(t, "multi-key channel is not supported", envelope.Message)
	assert.Equal(t, 0, requestCount)
}

func TestGetClaudeChannelUsage_InvalidCredential_ReturnsSpecificMessageWithoutUpstreamRequest(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		message string
	}{
		{
			name:    "invalid json key",
			key:     `not-json`,
			message: "claude subscription channel: invalid credential key json",
		},
		{
			name:    "missing claudeAiOauth",
			key:     `{"other":{}}`,
			message: "claude subscription channel: credential key json must include claudeAiOauth",
		},
		{
			name:    "empty access token",
			key:     `{"claudeAiOauth":{"accessToken":""}}`,
			message: "claude subscription channel: access_token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupModelListControllerTestDB(t)

			var requestCount int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			channel := &model.Channel{
				Type:    constant.ChannelTypeClaudeSubscription,
				Key:     tt.key,
				BaseURL: common.GetPointer(server.URL),
			}
			require.NoError(t, db.Create(channel).Error)

			recorder := callGetClaudeChannelUsage(t, channel.Id)

			envelope, _ := decodeClaudeUsageResponse(t, recorder)
			assert.False(t, envelope.Success)
			assert.Equal(t, tt.message, envelope.Message)
			assert.Equal(t, 0, requestCount)
		})
	}
}
