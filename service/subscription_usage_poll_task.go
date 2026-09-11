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

const subscriptionUsagePollInterval = time.Hour

var (
	subscriptionUsagePollOnce    sync.Once
	subscriptionUsagePollRunning atomic.Bool
)

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
		Where("type IN ? AND (status = ? OR status = ?)",
			[]int{constant.ChannelTypeCodex, constant.ChannelTypeClaudeSubscription},
			common.ChannelStatusEnabled,
			common.ChannelStatusAutoDisabled,
		).
		Order("id asc").
		Find(&channels).Error
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("subscription usage poll: query channels failed: %v", err))
		return
	}

	synced := 0
	for _, ch := range channels {
		if ch.ChannelInfo.IsMultiKey {
			continue
		}
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
