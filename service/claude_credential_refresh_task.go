package service

import (
	"context"
	"fmt"
	"strings"
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
	claudeCredentialRefreshTickInterval = 10 * time.Minute
	claudeCredentialRefreshThreshold    = 24 * time.Hour
	claudeCredentialRefreshBatchSize    = 200
	claudeCredentialRefreshTimeout      = 15 * time.Second
)

var (
	claudeCredentialRefreshOnce    sync.Once
	claudeCredentialRefreshRunning atomic.Bool
)

func StartClaudeCredentialAutoRefreshTask() {
	claudeCredentialRefreshOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}

		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("claude subscription credential auto-refresh task started: tick=%s threshold=%s", claudeCredentialRefreshTickInterval, claudeCredentialRefreshThreshold))

			ticker := time.NewTicker(claudeCredentialRefreshTickInterval)
			defer ticker.Stop()

			runClaudeCredentialAutoRefreshOnce()
			for range ticker.C {
				runClaudeCredentialAutoRefreshOnce()
			}
		})
	})
}

func runClaudeCredentialAutoRefreshOnce() {
	if !claudeCredentialRefreshRunning.CompareAndSwap(false, true) {
		return
	}
	defer claudeCredentialRefreshRunning.Store(false)

	ctx := context.Background()
	now := time.Now()

	var refreshed int
	var scanned int

	offset := 0
	for {
		var channels []*model.Channel
		err := model.DB.
			Select("id", "name", "key", "status", "channel_info").
			Where("type = ? AND (status = ? OR status = ?)",
				constant.ChannelTypeClaudeSubscription,
				common.ChannelStatusEnabled,
				common.ChannelStatusAutoDisabled,
			).
			Order("id asc").
			Limit(claudeCredentialRefreshBatchSize).
			Offset(offset).
			Find(&channels).Error
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("claude credential auto-refresh: query channels failed: %v", err))
			return
		}
		if len(channels) == 0 {
			break
		}
		offset += claudeCredentialRefreshBatchSize

		for _, ch := range channels {
			if ch == nil {
				continue
			}
			scanned++
			if ch.ChannelInfo.IsMultiKey {
				continue
			}

			rawKey := strings.TrimSpace(ch.Key)
			if rawKey == "" {
				continue
			}

			env, err := parseClaudeCredentialEnvelope(rawKey)
			if err != nil {
				continue
			}

			refreshToken := strings.TrimSpace(env.ClaudeAiOauth.RefreshToken)
			if refreshToken == "" {
				continue
			}

			if !claudeCredentialNeedsRefresh(env.ClaudeAiOauth.ExpiresAt, now, claudeCredentialRefreshThreshold) {
				continue
			}

			refreshCtx, cancel := context.WithTimeout(ctx, claudeCredentialRefreshTimeout)
			newCred, _, err := RefreshClaudeChannelCredential(refreshCtx, ch.Id, ClaudeCredentialRefreshOptions{ResetCaches: false})
			cancel()
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("claude credential auto-refresh: channel_id=%d name=%s refresh failed: %v", ch.Id, ch.Name, err))
				continue
			}

			refreshed++
			logger.LogInfo(ctx, fmt.Sprintf("claude credential auto-refresh: channel_id=%d name=%s refreshed, expires_at=%d", ch.Id, ch.Name, newCred.ExpiresAt))
		}
	}

	if refreshed > 0 {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.LogWarn(ctx, fmt.Sprintf("claude credential auto-refresh: InitChannelCache panic: %v", r))
				}
			}()
			model.InitChannelCache()
		}()
		ResetProxyClientCache()
	}

	if common.DebugEnabled {
		logger.LogDebug(ctx, "claude credential auto-refresh: scanned=%d refreshed=%d", scanned, refreshed)
	}
}
