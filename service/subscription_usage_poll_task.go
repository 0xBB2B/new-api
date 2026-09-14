package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	subscriptionUsagePollInterval    = time.Minute
	subscriptionUsageMinPollInterval = time.Hour
)

var (
	subscriptionUsagePollOnce    sync.Once
	subscriptionUsagePollRunning atomic.Bool
	subscriptionUsageLastAttempt = make(map[int]time.Time)
)

func shouldPollSubscriptionUsage(otherInfo string, lastAttempt time.Time, now time.Time) bool {
	var otherInfoData struct {
		SubscriptionUsage SubscriptionUsageSnapshot `json:"subscription_usage"`
	}
	_ = common.UnmarshalJsonStr(otherInfo, &otherInfoData)
	updatedAt := otherInfoData.SubscriptionUsage.UpdatedAt

	snapshotStale := updatedAt == 0 || now.Sub(time.Unix(updatedAt, 0)) >= subscriptionUsageMinPollInterval
	if !snapshotStale {
		return false
	}
	return lastAttempt.IsZero() || now.Sub(lastAttempt) >= subscriptionUsageMinPollInterval
}

func StartSubscriptionUsagePollTask() {
	subscriptionUsagePollOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("subscription usage poll task started: interval=%s", subscriptionUsagePollInterval))
			ticker := time.NewTicker(subscriptionUsagePollInterval)
			defer ticker.Stop()

			runSubscriptionUsagePollOnce()
			for range ticker.C {
				runSubscriptionUsagePollOnce()
			}
		})
	})
}

func runSubscriptionUsagePollOnce() {
	if !subscriptionUsagePollRunning.CompareAndSwap(false, true) {
		return
	}
	defer subscriptionUsagePollRunning.Store(false)

	ctx := context.Background()
	var channels []*model.Channel
	err := model.DB.
		Where("type IN ? AND status = ?",
			[]int{constant.ChannelTypeCodex, constant.ChannelTypeClaudeSubscription},
			common.ChannelStatusEnabled,
		).
		Order("id asc").
		Find(&channels).Error
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("subscription usage poll: query channels failed: %v", err))
		return
	}

	now := time.Now()
	synced := 0
	for _, ch := range channels {
		if ch.ChannelInfo.IsMultiKey {
			continue
		}
		if !shouldPollSubscriptionUsage(ch.OtherInfo, subscriptionUsageLastAttempt[ch.Id], now) {
			continue
		}
		subscriptionUsageLastAttempt[ch.Id] = time.Now()
		statusCode, err := syncSubscriptionChannelUsage(ctx, ch)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("subscription usage poll: channel_id=%d name=%s fetch failed: %v", ch.Id, ch.Name, err))
			continue
		}
		if statusCode < 200 || statusCode >= 300 {
			logger.LogWarn(ctx, fmt.Sprintf("subscription usage poll: channel_id=%d name=%s upstream status %d", ch.Id, ch.Name, statusCode))
			continue
		}
		synced++
	}
	if common.DebugEnabled {
		logger.LogDebug(ctx, "subscription usage poll: scanned=%d synced=%d", len(channels), synced)
	}
}

func syncSubscriptionChannelUsage(ctx context.Context, ch *model.Channel) (int, error) {
	switch ch.Type {
	case constant.ChannelTypeCodex:
		oauthKey, err := CodexChannelCredential(ch)
		if err != nil {
			return 0, err
		}
		statusCode, _, err := SyncCodexChannelUsage(ctx, ch, oauthKey)
		return statusCode, err
	case constant.ChannelTypeClaudeSubscription:
		cred, err := ClaudeChannelCredential(ch)
		if err != nil {
			return 0, err
		}
		statusCode, _, _, err := SyncClaudeChannelUsage(ctx, ch, cred)
		return statusCode, err
	}
	return 0, fmt.Errorf("unsupported channel type %d", ch.Type)
}
