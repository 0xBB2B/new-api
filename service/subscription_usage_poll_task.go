package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	subscriptionUsagePollTickInterval = 60 * time.Second
	subscriptionUsagePollBatchSize    = 200
	subscriptionUsagePollConcurrency  = 20

	subscriptionUsageSyncTickInterval = 30 * time.Second
	subscriptionUsageSnapshotRedisKey = "subscription_channel_usage_snapshot"
	subscriptionUsageSnapshotTTL      = 10 * time.Minute
)

var subscriptionUsagePollRequestTimeout = 15 * time.Second

var (
	subscriptionUsagePollOnce    = new(sync.Once)
	subscriptionUsagePollRunning atomic.Bool
	subscriptionUsageSyncOnce    = new(sync.Once)
)

type codexWhamUsageResponse struct {
	RateLimit struct {
		PrimaryWindow *struct {
			UsedPercent *float64 `json:"used_percent"`
		} `json:"primary_window"`
		SecondaryWindow *struct {
			UsedPercent *float64 `json:"used_percent"`
		} `json:"secondary_window"`
	} `json:"rate_limit"`
}

func subscriptionUsageDueChannelTypes(round int) []int {
	if round%3 == 0 {
		return []int{constant.ChannelTypeCodex, constant.ChannelTypeClaudeSubscription}
	}
	return []int{constant.ChannelTypeCodex}
}

func StartSubscriptionUsagePollTask() {
	subscriptionUsagePollOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}

		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("subscription usage poll task started: tick=%s", subscriptionUsagePollTickInterval))

			ticker := time.NewTicker(subscriptionUsagePollTickInterval)
			defer ticker.Stop()

			round := 0
			runSubscriptionUsagePollOnce(subscriptionUsageDueChannelTypes(round))
			round++
			for range ticker.C {
				runSubscriptionUsagePollOnce(subscriptionUsageDueChannelTypes(round))
				round++
			}
		})
	})
}

func runSubscriptionUsagePollOnce(channelTypes []int) {
	if !subscriptionUsagePollRunning.CompareAndSwap(false, true) {
		return
	}
	defer subscriptionUsagePollRunning.Store(false)

	ctx := context.Background()
	offset := 0
	for {
		var channels []*model.Channel
		err := model.DB.
			Where("type IN ? AND status = ?", channelTypes, common.ChannelStatusEnabled).
			Order("id asc").
			Limit(subscriptionUsagePollBatchSize).
			Offset(offset).
			Find(&channels).Error
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("subscription usage poll: query channels failed: %v", err))
			return
		}
		if len(channels) == 0 {
			break
		}
		offset += subscriptionUsagePollBatchSize

		var wg sync.WaitGroup
		sem := make(chan struct{}, subscriptionUsagePollConcurrency)
		for _, ch := range channels {
			wg.Add(1)
			sem <- struct{}{}
			go func(ch *model.Channel) {
				defer wg.Done()
				defer func() { <-sem }()
				if err := pollSubscriptionChannelUsage(ctx, ch); err != nil {
					logger.LogWarn(ctx, fmt.Sprintf("subscription usage poll: channel_id=%d name=%s failed: %v", ch.Id, ch.Name, err))
				}
			}(ch)
		}
		wg.Wait()

		if common.RedisEnabled {
			data, err := model.SubscriptionChannelUsageSnapshotJSON()
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("subscription usage poll: export snapshot failed: %v", err))
			} else if err := common.RedisSet(subscriptionUsageSnapshotRedisKey, string(data), subscriptionUsageSnapshotTTL); err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("subscription usage poll: write snapshot failed: %v", err))
			}
		}
	}
}

func pollSubscriptionChannelUsage(ctx context.Context, ch *model.Channel) error {
	// 不用 ch.GetSetting()：它在解析失败时会清空 Setting 并写回数据库，轮询必须零写副作用
	setting := dto.ChannelSettings{}
	if ch.Setting != nil && *ch.Setting != "" {
		if err := common.UnmarshalJsonStr(*ch.Setting, &setting); err != nil {
			return err
		}
	}
	client, err := NewProxyHttpClient(setting.Proxy)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, subscriptionUsagePollRequestTimeout)
	defer cancel()

	switch ch.Type {
	case constant.ChannelTypeCodex:
		return pollCodexChannelUsage(ctx, client, ch)
	case constant.ChannelTypeClaudeSubscription:
		return pollClaudeChannelUsage(ctx, client, ch)
	default:
		return fmt.Errorf("subscription usage poll: unsupported channel type %d", ch.Type)
	}
}

func pollCodexChannelUsage(ctx context.Context, client *http.Client, ch *model.Channel) error {
	oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return err
	}

	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)

	statusCode, body, err := FetchCodexWhamUsage(ctx, client, ch.GetBaseURL(), accessToken, accountID)
	if err != nil {
		return err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("codex usage poll: upstream status %d", statusCode)
	}

	bottleneckPercent, err := parseCodexWhamUsageBottleneck(body)
	if err != nil {
		return err
	}
	model.CacheSetSubscriptionChannelUsage(ch.Id, bottleneckPercent)
	return nil
}

func pollClaudeChannelUsage(ctx context.Context, client *http.Client, ch *model.Channel) error {
	envelope, err := parseClaudeCredentialEnvelope(strings.TrimSpace(ch.Key))
	if err != nil {
		return err
	}

	accessToken := strings.TrimSpace(envelope.ClaudeAiOauth.AccessToken)
	if accessToken == "" {
		return fmt.Errorf("claude subscription channel: access_token is required")
	}

	statusCode, body, err := FetchClaudeOAuthUsage(ctx, client, ch.GetBaseURL(), accessToken)
	if err != nil {
		return err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("claude subscription usage poll: upstream status %d", statusCode)
	}

	bottleneckPercent, err := parseClaudeOAuthUsageBottleneck(body)
	if err != nil {
		return err
	}
	model.CacheSetSubscriptionChannelUsage(ch.Id, bottleneckPercent)
	return nil
}

func StartSubscriptionUsageSyncTask() {
	subscriptionUsageSyncOnce.Do(func() {
		if common.IsMasterNode {
			return
		}
		if !common.RedisEnabled {
			common.SysLog("订阅渠道用量均衡路由同步未启动：非 master 节点且 Redis 未启用，无法感知渠道触顶状态")
			return
		}

		gopool.Go(func() {
			ticker := time.NewTicker(subscriptionUsageSyncTickInterval)
			defer ticker.Stop()

			syncSubscriptionUsageSnapshotOnce()
			for range ticker.C {
				syncSubscriptionUsageSnapshotOnce()
			}
		})
	})
}

func syncSubscriptionUsageSnapshotOnce() {
	ctx := context.Background()
	data, err := common.RedisGet(subscriptionUsageSnapshotRedisKey)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("subscription usage sync: read snapshot failed: %v", err))
		return
	}
	if err := model.CacheLoadSubscriptionChannelUsageSnapshotJSON([]byte(data)); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("subscription usage sync: load snapshot failed: %v", err))
	}
}

func parseCodexWhamUsageBottleneck(body []byte) (float64, error) {
	var usage codexWhamUsageResponse
	if err := common.Unmarshal(body, &usage); err != nil {
		return 0, err
	}

	primary := usage.RateLimit.PrimaryWindow
	secondary := usage.RateLimit.SecondaryWindow
	if primary == nil && secondary == nil {
		return 0, fmt.Errorf("codex usage poll: missing usage windows")
	}

	var bottleneckPercent float64
	if primary != nil && primary.UsedPercent != nil && *primary.UsedPercent > bottleneckPercent {
		bottleneckPercent = *primary.UsedPercent
	}
	if secondary != nil && secondary.UsedPercent != nil && *secondary.UsedPercent > bottleneckPercent {
		bottleneckPercent = *secondary.UsedPercent
	}
	return bottleneckPercent, nil
}
