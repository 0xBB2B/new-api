package model

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	codexChannelUsageSaturationThreshold = 95
	codexChannelUsageSaturationWindow    = 10 * time.Minute
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
	wasSaturated := exists && codexChannelUsageEntrySaturationValid(oldEntry) && codexChannelUsageBottleneck(oldEntry) >= codexChannelUsageSaturationThreshold
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
	return ok && codexChannelUsageEntrySaturationValid(entry) && codexChannelUsageBottleneck(entry) >= codexChannelUsageSaturationThreshold
}

func codexChannelRemainingRatio(channelID int) (float64, bool) {
	codexChannelUsageCacheLock.RLock()
	entry, ok := codexChannelUsageCache[channelID]
	codexChannelUsageCacheLock.RUnlock()
	if !ok || !codexChannelUsageEntrySaturationValid(entry) {
		return 0, false
	}
	return (100 - codexChannelUsageBottleneck(entry)) / 100, true
}

func codexChannelUsageEntrySaturationValid(entry codexChannelUsageCacheEntry) bool {
	return time.Since(entry.refreshedAt) <= codexChannelUsageSaturationWindow
}

func codexChannelUsageBottleneck(entry codexChannelUsageCacheEntry) float64 {
	if entry.used5hPercent > entry.used7dPercent {
		return entry.used5hPercent
	}
	return entry.used7dPercent
}

type codexChannelUsageSnapshotEntry struct {
	Used5hPercent     float64 `json:"used_5h_percent"`
	Used7dPercent     float64 `json:"used_7d_percent"`
	RefreshedAtUnixMs int64   `json:"refreshed_at_unix_ms"`
}

func CodexChannelUsageSnapshotJSON() ([]byte, error) {
	codexChannelUsageCacheLock.RLock()
	snapshot := make(map[string]codexChannelUsageSnapshotEntry, len(codexChannelUsageCache))
	for channelID, entry := range codexChannelUsageCache {
		snapshot[strconv.Itoa(channelID)] = codexChannelUsageSnapshotEntry{
			Used5hPercent:     entry.used5hPercent,
			Used7dPercent:     entry.used7dPercent,
			RefreshedAtUnixMs: entry.refreshedAt.UnixMilli(),
		}
	}
	codexChannelUsageCacheLock.RUnlock()

	return common.Marshal(snapshot)
}

func CacheLoadCodexChannelUsageSnapshotJSON(data []byte) error {
	var snapshot map[string]codexChannelUsageSnapshotEntry
	if err := common.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	newCache := make(map[int]codexChannelUsageCacheEntry, len(snapshot))
	for key, entry := range snapshot {
		channelID, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("codex usage snapshot: invalid channel id %q: %w", key, err)
		}
		newCache[channelID] = codexChannelUsageCacheEntry{
			used5hPercent: entry.Used5hPercent,
			used7dPercent: entry.Used7dPercent,
			refreshedAt:   time.UnixMilli(entry.RefreshedAtUnixMs),
		}
	}

	codexChannelUsageCacheLock.Lock()
	codexChannelUsageCache = newCache
	codexChannelUsageCacheLock.Unlock()
	return nil
}
