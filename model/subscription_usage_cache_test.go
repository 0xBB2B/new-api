package model

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
)

func TestSubscriptionChannelUsageCache_MissingChannel(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)

	assert.False(t, CacheIsSubscriptionChannelSaturated(60401))
	remaining, ok := subscriptionChannelRemainingRatio(60401)
	require.False(t, ok)
	assert.Zero(t, remaining)
}

func TestSubscriptionChannelUsageCache_ClampBottleneckBeforeStore(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)

	tests := []struct {
		name         string
		bottleneck   float64
		wantStored   float64
		wantRemain   float64
		wantSaturate bool
	}{
		{name: "above upper bound", bottleneck: 120, wantStored: 100, wantRemain: 0, wantSaturate: true},
		{name: "below lower bound", bottleneck: -5, wantStored: 0, wantRemain: 1, wantSaturate: false},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channelID := 61000 + index
			CacheSetSubscriptionChannelUsage(channelID, tt.bottleneck)

			subscriptionChannelUsageCacheLock.Lock()
			entry, exists := subscriptionChannelUsageCache[channelID]
			subscriptionChannelUsageCacheLock.Unlock()
			require.True(t, exists)
			assert.Equal(t, tt.wantStored, entry.bottleneckPercent)

			assert.Equal(t, tt.wantSaturate, CacheIsSubscriptionChannelSaturated(channelID))
			remaining, ok := subscriptionChannelRemainingRatio(channelID)
			require.True(t, ok)
			assert.Equal(t, tt.wantRemain, remaining)
		})
	}
}

func TestSubscriptionChannelUsageCache_SaturationThreshold(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)

	tests := []struct {
		name       string
		channelID  int
		bottleneck float64
		want       bool
	}{
		{name: "at threshold", channelID: 62001, bottleneck: 95.0, want: true},
		{name: "below threshold", channelID: 62002, bottleneck: 94.9, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CacheSetSubscriptionChannelUsage(tt.channelID, tt.bottleneck)
			assert.Equal(t, tt.want, CacheIsSubscriptionChannelSaturated(tt.channelID))
		})
	}
}

func TestSubscriptionChannelUsageCache_WithinWindowRemainingRatioAvailable(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	channelID := 63001
	CacheSetSubscriptionChannelUsage(channelID, 90)
	ageSubscriptionChannelUsageEntry(t, channelID, -6*time.Minute)

	assert.False(t, CacheIsSubscriptionChannelSaturated(channelID))
	remaining, ok := subscriptionChannelRemainingRatio(channelID)
	require.True(t, ok)
	assert.Equal(t, 0.10, remaining)
}

func TestSubscriptionChannelUsageCache_BeyondWindowRemainingRatioUnavailable(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	channelID := 63002
	CacheSetSubscriptionChannelUsage(channelID, 90)
	ageSubscriptionChannelUsageEntry(t, channelID, -11*time.Minute)

	assert.False(t, CacheIsSubscriptionChannelSaturated(channelID))
	remaining, ok := subscriptionChannelRemainingRatio(channelID)
	require.False(t, ok)
	assert.Zero(t, remaining)
}

func TestSubscriptionChannelUsageCache_SaturatedEntryStaysSaturatedWithinFreshWindow(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	channelID := 64001
	CacheSetSubscriptionChannelUsage(channelID, 96)

	ageSubscriptionChannelUsageEntry(t, channelID, -6*time.Minute)
	assert.True(t, CacheIsSubscriptionChannelSaturated(channelID))

	ageSubscriptionChannelUsageEntry(t, channelID, -11*time.Minute)
	assert.False(t, CacheIsSubscriptionChannelSaturated(channelID))
}

func TestSubscriptionChannelUsageCache_SaturationTransitionLogsOnce(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	var logBuffer bytes.Buffer
	originalWriter := gin.DefaultWriter
	gin.DefaultWriter = &logBuffer
	t.Cleanup(func() {
		gin.DefaultWriter = originalWriter
	})

	channelID := 65001
	CacheSetSubscriptionChannelUsage(channelID, 50)
	assert.Empty(t, logBuffer.String())

	CacheSetSubscriptionChannelUsage(channelID, 96)
	saturatedLog := logBuffer.String()
	require.NotEmpty(t, saturatedLog)
	assert.Len(t, strings.Split(strings.TrimSpace(saturatedLog), "\n"), 1)
	assert.Contains(t, saturatedLog, "65001")

	CacheSetSubscriptionChannelUsage(channelID, 97)
	assert.Equal(t, saturatedLog, logBuffer.String())

	CacheSetSubscriptionChannelUsage(channelID, 10)
	recoveredLog := strings.TrimPrefix(logBuffer.String(), saturatedLog)
	require.NotEmpty(t, recoveredLog)
	assert.Len(t, strings.Split(strings.TrimSpace(logBuffer.String()), "\n"), 2)
	assert.Contains(t, recoveredLog, "65001")

	allLogs := logBuffer.String()
	CacheSetSubscriptionChannelUsage(channelID, 20)
	assert.Equal(t, allLogs, logBuffer.String())
}

func TestSubscriptionChannelUsageSnapshot_RoundTrip(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	channelID := 66001
	CacheSetSubscriptionChannelUsage(channelID, 96)

	data, err := SubscriptionChannelUsageSnapshotJSON()
	require.NoError(t, err)

	subscriptionChannelUsageCacheLock.Lock()
	subscriptionChannelUsageCache = make(map[int]subscriptionChannelUsageCacheEntry)
	subscriptionChannelUsageCacheLock.Unlock()

	require.NoError(t, CacheLoadSubscriptionChannelUsageSnapshotJSON(data))
	assert.True(t, CacheIsSubscriptionChannelSaturated(channelID))
	remaining, ok := subscriptionChannelRemainingRatio(channelID)
	require.True(t, ok)
	assert.Equal(t, 0.04, remaining)
}

func TestSubscriptionChannelUsageSnapshot_PreservesRefreshedAtAcrossRoundTrip(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)

	expiredID := 66101
	CacheSetSubscriptionChannelUsage(expiredID, 96)
	ageSubscriptionChannelUsageEntry(t, expiredID, -11*time.Minute)

	freshID := 66102
	CacheSetSubscriptionChannelUsage(freshID, 96)

	data, err := SubscriptionChannelUsageSnapshotJSON()
	require.NoError(t, err)

	subscriptionChannelUsageCacheLock.Lock()
	subscriptionChannelUsageCache = make(map[int]subscriptionChannelUsageCacheEntry)
	subscriptionChannelUsageCacheLock.Unlock()

	require.NoError(t, CacheLoadSubscriptionChannelUsageSnapshotJSON(data))
	assert.False(t, CacheIsSubscriptionChannelSaturated(expiredID))
	assert.True(t, CacheIsSubscriptionChannelSaturated(freshID))
}

func TestSubscriptionChannelUsageSnapshot_InvalidJSONKeepsCache(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	channelID := 66201
	CacheSetSubscriptionChannelUsage(channelID, 96)

	err := CacheLoadSubscriptionChannelUsageSnapshotJSON([]byte(`{invalid`))
	assert.Error(t, err)
	assert.True(t, CacheIsSubscriptionChannelSaturated(channelID))
}

func TestSubscriptionChannelUsageSnapshot_InvalidChannelIDKeepsCache(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	channelID := 66301
	CacheSetSubscriptionChannelUsage(channelID, 96)

	data, err := SubscriptionChannelUsageSnapshotJSON()
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(data, &raw))
	require.Len(t, raw, 1)

	tampered := make(map[string]json.RawMessage, 1)
	for _, entryJSON := range raw {
		tampered["not-a-channel-id"] = entryJSON
	}
	tamperedData, err := common.Marshal(tampered)
	require.NoError(t, err)

	err = CacheLoadSubscriptionChannelUsageSnapshotJSON(tamperedData)
	assert.Error(t, err)
	assert.True(t, CacheIsSubscriptionChannelSaturated(channelID))
}

type subscriptionChannelUsageOverviewEntryView struct {
	BottleneckPercent float64 `json:"bottleneck_percent"`
	RefreshedAt       int64   `json:"refreshed_at"`
	Saturated         bool    `json:"saturated"`
}

func TestSubscriptionChannelUsageOverview_FreshEntries(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	before := time.Now().UnixMilli()
	CacheSetSubscriptionChannelUsage(7, 62.4)
	CacheSetSubscriptionChannelUsage(12, 96.2)
	after := time.Now().UnixMilli()

	parsed := subscriptionChannelUsageOverviewView(t)

	require.Len(t, parsed, 2)
	require.Contains(t, parsed, "7")
	assert.Equal(t, 62.4, parsed["7"].BottleneckPercent)
	assert.False(t, parsed["7"].Saturated)
	assert.GreaterOrEqual(t, parsed["7"].RefreshedAt, before)
	assert.LessOrEqual(t, parsed["7"].RefreshedAt, after)

	require.Contains(t, parsed, "12")
	assert.Equal(t, 96.2, parsed["12"].BottleneckPercent)
	assert.True(t, parsed["12"].Saturated)
}

func TestSubscriptionChannelUsageOverview_SaturatedEntryStaysSaturatedWithinFreshWindow(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	channelID := 71001
	CacheSetSubscriptionChannelUsage(channelID, 96.2)
	ageSubscriptionChannelUsageEntry(t, channelID, -8*time.Minute)

	parsed := subscriptionChannelUsageOverviewView(t)
	require.Contains(t, parsed, strconv.Itoa(channelID))
	assert.True(t, parsed[strconv.Itoa(channelID)].Saturated)
}

func TestSubscriptionChannelUsageOverview_StaleBeyondWindowNotSaturated(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	channelID := 71002
	CacheSetSubscriptionChannelUsage(channelID, 96.2)
	ageSubscriptionChannelUsageEntry(t, channelID, -11*time.Minute)

	parsed := subscriptionChannelUsageOverviewView(t)
	require.Contains(t, parsed, strconv.Itoa(channelID))
	assert.False(t, parsed[strconv.Itoa(channelID)].Saturated)
}

func TestSubscriptionChannelUsageOverview_NeverSaturatedStaleEntryPresent(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	channelID := 71003
	CacheSetSubscriptionChannelUsage(channelID, 41)
	ageSubscriptionChannelUsageEntry(t, channelID, -11*time.Minute)

	parsed := subscriptionChannelUsageOverviewView(t)
	require.Contains(t, parsed, strconv.Itoa(channelID))
	assert.False(t, parsed[strconv.Itoa(channelID)].Saturated)
}

func TestSubscriptionChannelUsageOverview_EmptyCacheProducesEmptyJSONObject(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)

	data, err := common.Marshal(SubscriptionChannelUsageOverview())
	require.NoError(t, err)
	assert.Equal(t, "{}", string(data))
}

func subscriptionChannelUsageOverviewView(t *testing.T) map[string]subscriptionChannelUsageOverviewEntryView {
	t.Helper()
	data, err := common.Marshal(SubscriptionChannelUsageOverview())
	require.NoError(t, err)
	var parsed map[string]subscriptionChannelUsageOverviewEntryView
	require.NoError(t, common.Unmarshal(data, &parsed))
	return parsed
}

func resetSubscriptionChannelUsageCache(t *testing.T) {
	t.Helper()
	subscriptionChannelUsageCacheLock.Lock()
	subscriptionChannelUsageCache = make(map[int]subscriptionChannelUsageCacheEntry)
	subscriptionChannelUsageCacheLock.Unlock()
	t.Cleanup(func() {
		subscriptionChannelUsageCacheLock.Lock()
		subscriptionChannelUsageCache = make(map[int]subscriptionChannelUsageCacheEntry)
		subscriptionChannelUsageCacheLock.Unlock()
	})
}

func ageSubscriptionChannelUsageEntry(t *testing.T, channelID int, offset time.Duration) {
	t.Helper()
	subscriptionChannelUsageCacheLock.Lock()
	entry := subscriptionChannelUsageCache[channelID]
	entry.refreshedAt = time.Now().Add(offset)
	subscriptionChannelUsageCache[channelID] = entry
	subscriptionChannelUsageCacheLock.Unlock()
}
