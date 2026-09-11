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

const codexUsagePollInterval = time.Hour

var (
	codexUsagePollOnce    sync.Once
	codexUsagePollRunning atomic.Bool
)

func StartCodexUsagePollTask() {
	codexUsagePollOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("codex usage poll task started: interval=%s", codexUsagePollInterval))
			ticker := time.NewTicker(codexUsagePollInterval)
			defer ticker.Stop()

			runCodexUsagePollOnce()
			for range ticker.C {
				runCodexUsagePollOnce()
			}
		})
	})
}

func runCodexUsagePollOnce() {
	if !codexUsagePollRunning.CompareAndSwap(false, true) {
		return
	}
	defer codexUsagePollRunning.Store(false)

	ctx := context.Background()
	var channels []*model.Channel
	err := model.DB.
		Where("type = ? AND (status = ? OR status = ?)",
			constant.ChannelTypeCodex,
			common.ChannelStatusEnabled,
			common.ChannelStatusAutoDisabled,
		).
		Order("id asc").
		Find(&channels).Error
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("codex usage poll: query channels failed: %v", err))
		return
	}

	synced := 0
	for _, ch := range channels {
		if ch.ChannelInfo.IsMultiKey {
			continue
		}
		oauthKey, err := CodexChannelCredential(ch)
		if err != nil {
			continue
		}
		statusCode, _, err := SyncCodexChannelUsage(ctx, ch, oauthKey)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("codex usage poll: channel_id=%d name=%s fetch failed: %v", ch.Id, ch.Name, err))
			continue
		}
		if statusCode < 200 || statusCode >= 300 {
			logger.LogWarn(ctx, fmt.Sprintf("codex usage poll: channel_id=%d name=%s upstream status %d", ch.Id, ch.Name, statusCode))
			continue
		}
		synced++
	}
	if common.DebugEnabled {
		logger.LogDebug(ctx, "codex usage poll: scanned=%d synced=%d", len(channels), synced)
	}
}
