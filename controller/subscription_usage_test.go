package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type subscriptionChannelUsageOverviewEntryView struct {
	BottleneckPercent float64 `json:"bottleneck_percent"`
	RefreshedAt       int64   `json:"refreshed_at"`
	Saturated         bool    `json:"saturated"`
}

func TestGetSubscriptionChannelUsage_ReturnsOverviewEnvelope(t *testing.T) {
	channelID := 72001
	model.CacheSetSubscriptionChannelUsage(channelID, 88.5)

	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/subscription_usage", nil)

	GetSubscriptionChannelUsage(ctx)

	assert.Equal(t, http.StatusOK, response.Code)

	var payload struct {
		Success bool                                                 `json:"success"`
		Message string                                               `json:"message"`
		Data    map[string]subscriptionChannelUsageOverviewEntryView `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	assert.Empty(t, payload.Message)
	require.Contains(t, payload.Data, strconv.Itoa(channelID))
	assert.Equal(t, 88.5, payload.Data[strconv.Itoa(channelID)].BottleneckPercent)
}

func TestGetSubscriptionChannelUsage_DataMatchesModelOverview(t *testing.T) {
	channelID := 72002
	model.CacheSetSubscriptionChannelUsage(channelID, 30)

	overviewJSON, err := common.Marshal(model.SubscriptionChannelUsageOverview())
	require.NoError(t, err)

	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/subscription_usage", nil)

	GetSubscriptionChannelUsage(ctx)

	var payload struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	dataJSON, err := common.Marshal(payload.Data)
	require.NoError(t, err)
	assert.JSONEq(t, string(overviewJSON), string(dataJSON))
}

func TestGetSubscriptionChannelUsage_NoWriteSideEffects(t *testing.T) {
	channelID := 72003
	model.CacheSetSubscriptionChannelUsage(channelID, 55)

	before, err := common.Marshal(model.SubscriptionChannelUsageOverview())
	require.NoError(t, err)

	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/subscription_usage", nil)

	GetSubscriptionChannelUsage(ctx)

	after, err := common.Marshal(model.SubscriptionChannelUsageOverview())
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(after))
}
