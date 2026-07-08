package model

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexChannelUsageCache_MissingChannel(t *testing.T) {
	resetCodexChannelUsageCache(t)

	assert.False(t, CacheIsCodexChannelSaturated(40401))
	remaining, ok := codexChannelRemainingRatio(40401)
	require.False(t, ok)
	assert.Zero(t, remaining)
}

func TestCodexChannelUsageCache_ClampPercentBeforeStore(t *testing.T) {
	resetCodexChannelUsageCache(t)

	tests := []struct {
		name         string
		used5h       float64
		used7d       float64
		want5h       float64
		want7d       float64
		wantRemain   float64
		wantSaturate bool
	}{
		{name: "above upper bound", used5h: 120, used7d: -3, want5h: 100, want7d: 0, wantRemain: 0, wantSaturate: true},
		{name: "below lower bound", used5h: -3, used7d: -4, want5h: 0, want7d: 0, wantRemain: 1, wantSaturate: false},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channelID := 51000 + index
			CacheSetCodexChannelUsage(channelID, tt.used5h, tt.used7d)

			codexChannelUsageCacheLock.Lock()
			entry, exists := codexChannelUsageCache[channelID]
			codexChannelUsageCacheLock.Unlock()
			require.True(t, exists)
			assert.Equal(t, tt.want5h, entry.used5hPercent)
			assert.Equal(t, tt.want7d, entry.used7dPercent)

			assert.Equal(t, tt.wantSaturate, CacheIsCodexChannelSaturated(channelID))
			remaining, ok := codexChannelRemainingRatio(channelID)
			require.True(t, ok)
			assert.Equal(t, tt.wantRemain, remaining)
		})
	}
}

func TestCodexChannelUsageCache_SaturationThreshold(t *testing.T) {
	resetCodexChannelUsageCache(t)

	tests := []struct {
		name       string
		channelID  int
		used5h     float64
		used7d     float64
		wantResult bool
	}{
		{name: "at threshold", channelID: 52001, used5h: 95.0, used7d: 10, wantResult: true},
		{name: "below threshold", channelID: 52002, used5h: 94.9, used7d: 10, wantResult: false},
		{name: "seven day bottleneck", channelID: 52003, used5h: 10, used7d: 95.0, wantResult: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CacheSetCodexChannelUsage(tt.channelID, tt.used5h, tt.used7d)

			assert.Equal(t, tt.wantResult, CacheIsCodexChannelSaturated(tt.channelID))
		})
	}
}

func TestCodexChannelUsageCache_ExpiredEntryIsIgnoredByReaders(t *testing.T) {
	resetCodexChannelUsageCache(t)
	channelID := 53001
	CacheSetCodexChannelUsage(channelID, 50, 40)

	codexChannelUsageCacheLock.Lock()
	entry := codexChannelUsageCache[channelID]
	entry.used5hPercent = 96
	entry.refreshedAt = time.Now().Add(-6 * time.Minute)
	codexChannelUsageCache[channelID] = entry
	codexChannelUsageCacheLock.Unlock()

	assert.False(t, CacheIsCodexChannelSaturated(channelID))
	remaining, ok := codexChannelRemainingRatio(channelID)
	require.False(t, ok)
	assert.Zero(t, remaining)

	codexChannelUsageCacheLock.Lock()
	_, exists := codexChannelUsageCache[channelID]
	codexChannelUsageCacheLock.Unlock()
	assert.True(t, exists)
}

func TestCodexChannelUsageCache_SaturationTransitionLogsOnce(t *testing.T) {
	resetCodexChannelUsageCache(t)
	var logBuffer bytes.Buffer
	originalWriter := gin.DefaultWriter
	gin.DefaultWriter = &logBuffer
	t.Cleanup(func() {
		gin.DefaultWriter = originalWriter
	})

	channelID := 54001
	CacheSetCodexChannelUsage(channelID, 50, 10)
	assert.Empty(t, logBuffer.String())

	CacheSetCodexChannelUsage(channelID, 96, 10)
	saturatedLog := logBuffer.String()
	require.NotEmpty(t, saturatedLog)
	assert.Len(t, strings.Split(strings.TrimSpace(saturatedLog), "\n"), 1)
	assert.Contains(t, saturatedLog, "触顶")
	assert.Contains(t, saturatedLog, "channel_id=54001")
	assert.Contains(t, saturatedLog, "bottleneck_usage=96.00")

	CacheSetCodexChannelUsage(channelID, 97, 20)
	assert.Equal(t, saturatedLog, logBuffer.String())

	CacheSetCodexChannelUsage(channelID, 10, 10)
	recoveredLog := strings.TrimPrefix(logBuffer.String(), saturatedLog)
	require.NotEmpty(t, recoveredLog)
	assert.Len(t, strings.Split(strings.TrimSpace(logBuffer.String()), "\n"), 2)
	assert.Len(t, strings.Split(strings.TrimSpace(recoveredLog), "\n"), 1)
	assert.Contains(t, recoveredLog, "回归")
	assert.Contains(t, recoveredLog, "channel_id=54001")
	assert.Contains(t, recoveredLog, "bottleneck_usage=10.00")

	allLogs := logBuffer.String()
	CacheSetCodexChannelUsage(channelID, 20, 20)
	assert.Equal(t, allLogs, logBuffer.String())
}

func resetCodexChannelUsageCache(t *testing.T) {
	t.Helper()
	codexChannelUsageCacheLock.Lock()
	codexChannelUsageCache = make(map[int]codexChannelUsageCacheEntry)
	codexChannelUsageCacheLock.Unlock()
	t.Cleanup(func() {
		codexChannelUsageCacheLock.Lock()
		codexChannelUsageCache = make(map[int]codexChannelUsageCacheEntry)
		codexChannelUsageCacheLock.Unlock()
	})
}
