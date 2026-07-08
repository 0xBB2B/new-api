package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	codexChannelUsageSaturationThreshold = 95
	codexChannelUsageCacheExpiration     = 5 * time.Minute
)

type codexChannelUsageCacheEntry struct {
	used5hPercent float64
	used7dPercent float64
	refreshedAt   time.Time
}

var codexChannelUsageCache map[int]codexChannelUsageCacheEntry
var codexChannelUsageCacheLock sync.RWMutex

func CacheSetCodexChannelUsage(channelID int, used5hPercent float64, used7dPercent float64) {
	if used5hPercent < 0 {
		used5hPercent = 0
	}
	if used5hPercent > 100 {
		used5hPercent = 100
	}
	if used7dPercent < 0 {
		used7dPercent = 0
	}
	if used7dPercent > 100 {
		used7dPercent = 100
	}
	entry := codexChannelUsageCacheEntry{
		used5hPercent: used5hPercent,
		used7dPercent: used7dPercent,
		refreshedAt:   time.Now(),
	}

	codexChannelUsageCacheLock.Lock()
	if codexChannelUsageCache == nil {
		codexChannelUsageCache = make(map[int]codexChannelUsageCacheEntry)
	}
	oldEntry, exists := codexChannelUsageCache[channelID]
	wasSaturated := exists && codexChannelUsageEntryFresh(oldEntry) && codexChannelUsageBottleneck(oldEntry) >= codexChannelUsageSaturationThreshold
	codexChannelUsageCache[channelID] = entry
	isSaturated := codexChannelUsageBottleneck(entry) >= codexChannelUsageSaturationThreshold
	codexChannelUsageCacheLock.Unlock()

	if wasSaturated == isSaturated {
		return
	}
	status := "回归"
	if isSaturated {
		status = "触顶"
	}
	common.SysLog(fmt.Sprintf("Codex 渠道使用率%s: channel_id=%d, bottleneck_usage=%.2f", status, channelID, codexChannelUsageBottleneck(entry)))
}

func CacheIsCodexChannelSaturated(channelID int) bool {
	codexChannelUsageCacheLock.RLock()
	entry, ok := codexChannelUsageCache[channelID]
	codexChannelUsageCacheLock.RUnlock()
	return ok && codexChannelUsageEntryFresh(entry) && codexChannelUsageBottleneck(entry) >= codexChannelUsageSaturationThreshold
}

func codexChannelRemainingRatio(channelID int) (float64, bool) {
	codexChannelUsageCacheLock.RLock()
	entry, ok := codexChannelUsageCache[channelID]
	codexChannelUsageCacheLock.RUnlock()
	if !ok || !codexChannelUsageEntryFresh(entry) {
		return 0, false
	}
	return (100 - codexChannelUsageBottleneck(entry)) / 100, true
}

func codexChannelUsageEntryFresh(entry codexChannelUsageCacheEntry) bool {
	return time.Since(entry.refreshedAt) <= codexChannelUsageCacheExpiration
}

func codexChannelUsageBottleneck(entry codexChannelUsageCacheEntry) float64 {
	if entry.used5hPercent > entry.used7dPercent {
		return entry.used5hPercent
	}
	return entry.used7dPercent
}
