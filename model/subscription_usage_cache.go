package model

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	subscriptionChannelUsageSaturationThreshold = 95
	subscriptionChannelUsageSaturationWindow    = 10 * time.Minute
)

type subscriptionChannelUsageCacheEntry struct {
	bottleneckPercent float64
	refreshedAt       time.Time
}

var subscriptionChannelUsageCache map[int]subscriptionChannelUsageCacheEntry
var subscriptionChannelUsageCacheLock sync.RWMutex

func CacheSetSubscriptionChannelUsage(channelID int, bottleneckPercent float64) {
	if bottleneckPercent < 0 {
		bottleneckPercent = 0
	}
	if bottleneckPercent > 100 {
		bottleneckPercent = 100
	}
	entry := subscriptionChannelUsageCacheEntry{
		bottleneckPercent: bottleneckPercent,
		refreshedAt:       time.Now(),
	}

	subscriptionChannelUsageCacheLock.Lock()
	if subscriptionChannelUsageCache == nil {
		subscriptionChannelUsageCache = make(map[int]subscriptionChannelUsageCacheEntry)
	}
	oldEntry, exists := subscriptionChannelUsageCache[channelID]
	wasSaturated := exists && subscriptionChannelUsageEntrySaturated(oldEntry)
	subscriptionChannelUsageCache[channelID] = entry
	isSaturated := entry.bottleneckPercent >= subscriptionChannelUsageSaturationThreshold
	subscriptionChannelUsageCacheLock.Unlock()

	if wasSaturated == isSaturated {
		return
	}
	status := "回归"
	if isSaturated {
		status = "触顶"
	}
	common.SysLog(fmt.Sprintf("订阅渠道使用率%s: channel_id=%d, bottleneck_usage=%.2f", status, channelID, entry.bottleneckPercent))
}

func CacheIsSubscriptionChannelSaturated(channelID int) bool {
	subscriptionChannelUsageCacheLock.RLock()
	entry, ok := subscriptionChannelUsageCache[channelID]
	subscriptionChannelUsageCacheLock.RUnlock()
	return ok && subscriptionChannelUsageEntrySaturated(entry)
}

func subscriptionChannelRemainingRatio(channelID int) (float64, bool) {
	subscriptionChannelUsageCacheLock.RLock()
	entry, ok := subscriptionChannelUsageCache[channelID]
	subscriptionChannelUsageCacheLock.RUnlock()
	if !ok || !subscriptionChannelUsageEntryFresh(entry) {
		return 0, false
	}
	return (100 - entry.bottleneckPercent) / 100, true
}

func subscriptionChannelUsageEntryFresh(entry subscriptionChannelUsageCacheEntry) bool {
	return time.Since(entry.refreshedAt) <= subscriptionChannelUsageSaturationWindow
}

func subscriptionChannelUsageEntrySaturated(entry subscriptionChannelUsageCacheEntry) bool {
	return subscriptionChannelUsageEntryFresh(entry) && entry.bottleneckPercent >= subscriptionChannelUsageSaturationThreshold
}

type subscriptionChannelUsageSnapshotEntry struct {
	BottleneckPercent float64 `json:"bottleneck_percent"`
	RefreshedAtUnixMs int64   `json:"refreshed_at_unix_ms"`
}

func SubscriptionChannelUsageSnapshotJSON() ([]byte, error) {
	subscriptionChannelUsageCacheLock.RLock()
	snapshot := make(map[string]subscriptionChannelUsageSnapshotEntry, len(subscriptionChannelUsageCache))
	for channelID, entry := range subscriptionChannelUsageCache {
		snapshot[strconv.Itoa(channelID)] = subscriptionChannelUsageSnapshotEntry{
			BottleneckPercent: entry.bottleneckPercent,
			RefreshedAtUnixMs: entry.refreshedAt.UnixMilli(),
		}
	}
	subscriptionChannelUsageCacheLock.RUnlock()

	return common.Marshal(snapshot)
}

type SubscriptionChannelUsageView struct {
	BottleneckPercent float64 `json:"bottleneck_percent"`
	RefreshedAt       int64   `json:"refreshed_at"`
	Saturated         bool    `json:"saturated"`
}

func SubscriptionChannelUsageOverview() map[string]SubscriptionChannelUsageView {
	subscriptionChannelUsageCacheLock.RLock()
	defer subscriptionChannelUsageCacheLock.RUnlock()

	overview := make(map[string]SubscriptionChannelUsageView, len(subscriptionChannelUsageCache))
	for channelID, entry := range subscriptionChannelUsageCache {
		overview[strconv.Itoa(channelID)] = SubscriptionChannelUsageView{
			BottleneckPercent: entry.bottleneckPercent,
			RefreshedAt:       entry.refreshedAt.UnixMilli(),
			Saturated:         subscriptionChannelUsageEntrySaturated(entry),
		}
	}
	return overview
}

func CacheLoadSubscriptionChannelUsageSnapshotJSON(data []byte) error {
	var snapshot map[string]subscriptionChannelUsageSnapshotEntry
	if err := common.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	newCache := make(map[int]subscriptionChannelUsageCacheEntry, len(snapshot))
	for key, entry := range snapshot {
		channelID, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("subscription usage snapshot: invalid channel id %q: %w", key, err)
		}
		newCache[channelID] = subscriptionChannelUsageCacheEntry{
			bottleneckPercent: entry.BottleneckPercent,
			refreshedAt:       time.UnixMilli(entry.RefreshedAtUnixMs),
		}
	}

	subscriptionChannelUsageCacheLock.Lock()
	subscriptionChannelUsageCache = newCache
	subscriptionChannelUsageCacheLock.Unlock()
	return nil
}
