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
	codexUsagePollTickInterval = 60 * time.Second
	codexUsagePollBatchSize    = 200
	codexUsagePollConcurrency  = 20

	codexUsageSyncTickInterval = 30 * time.Second
	codexUsageSnapshotRedisKey = "codex_channel_usage_snapshot"
	codexUsageSnapshotTTL      = 10 * time.Minute
)

var codexUsagePollRequestTimeout = 15 * time.Second

var (
	codexUsagePollOnce    sync.Once
	codexUsagePollRunning atomic.Bool
	codexUsageSyncOnce    sync.Once
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

func StartCodexUsagePollTask() {
	codexUsagePollOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}

		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("codex usage poll task started: tick=%s", codexUsagePollTickInterval))

			ticker := time.NewTicker(codexUsagePollTickInterval)
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
	offset := 0
	for {
		var channels []*model.Channel
		err := model.DB.
			Where("type = ? AND status = ?", constant.ChannelTypeCodex, common.ChannelStatusEnabled).
			Order("id asc").
			Limit(codexUsagePollBatchSize).
			Offset(offset).
			Find(&channels).Error
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("codex usage poll: query channels failed: %v", err))
			return
		}
		if len(channels) == 0 {
			break
		}
		offset += codexUsagePollBatchSize

		var wg sync.WaitGroup
		sem := make(chan struct{}, codexUsagePollConcurrency)
		for _, ch := range channels {
			wg.Add(1)
			sem <- struct{}{}
			go func(ch *model.Channel) {
				defer wg.Done()
				defer func() { <-sem }()
				if err := pollCodexChannelUsage(ctx, nil, ch); err != nil {
					logger.LogWarn(ctx, fmt.Sprintf("codex usage poll: channel_id=%d name=%s failed: %v", ch.Id, ch.Name, err))
				}
			}(ch)
		}
		wg.Wait()

		if common.RedisEnabled {
			data, err := model.CodexChannelUsageSnapshotJSON()
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("codex usage poll: export snapshot failed: %v", err))
			} else if err := common.RedisSet(codexUsageSnapshotRedisKey, string(data), codexUsageSnapshotTTL); err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("codex usage poll: write snapshot failed: %v", err))
			}
		}
	}
}

func pollCodexChannelUsage(ctx context.Context, client *http.Client, ch *model.Channel) error {
	oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return err
	}

	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)
	if client == nil {
		// 不用 ch.GetSetting()：它在解析失败时会清空 Setting 并写回数据库，轮询必须零写副作用
		setting := dto.ChannelSettings{}
		if ch.Setting != nil && *ch.Setting != "" {
			if err := common.UnmarshalJsonStr(*ch.Setting, &setting); err != nil {
				return err
			}
		}
		client, err = NewProxyHttpClient(setting.Proxy)
		if err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, codexUsagePollRequestTimeout)
	defer cancel()
	statusCode, body, err := FetchCodexWhamUsage(ctx, client, ch.GetBaseURL(), accessToken, accountID)
	if err != nil {
		return err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("codex usage poll: upstream status %d", statusCode)
	}

	used5hPercent, used7dPercent, err := parseCodexWhamUsageWindows(body)
	if err != nil {
		return err
	}
	model.CacheSetCodexChannelUsage(ch.Id, used5hPercent, used7dPercent)
	return nil
}

func StartCodexUsageSyncTask() {
	codexUsageSyncOnce.Do(func() {
		if common.IsMasterNode {
			return
		}
		if !common.RedisEnabled {
			common.SysLog("Codex 渠道用量均衡路由同步未启动：非 master 节点且 Redis 未启用，无法感知渠道触顶状态")
			return
		}

		gopool.Go(func() {
			ticker := time.NewTicker(codexUsageSyncTickInterval)
			defer ticker.Stop()

			syncCodexUsageSnapshotOnce()
			for range ticker.C {
				syncCodexUsageSnapshotOnce()
			}
		})
	})
}

func syncCodexUsageSnapshotOnce() {
	ctx := context.Background()
	data, err := common.RedisGet(codexUsageSnapshotRedisKey)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("codex usage sync: read snapshot failed: %v", err))
		return
	}
	if err := model.CacheLoadCodexChannelUsageSnapshotJSON([]byte(data)); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("codex usage sync: load snapshot failed: %v", err))
	}
}

func parseCodexWhamUsageWindows(body []byte) (float64, float64, error) {
	var usage codexWhamUsageResponse
	if err := common.Unmarshal(body, &usage); err != nil {
		return 0, 0, err
	}

	primary := usage.RateLimit.PrimaryWindow
	secondary := usage.RateLimit.SecondaryWindow
	if primary == nil && secondary == nil {
		return 0, 0, fmt.Errorf("codex usage poll: missing usage windows")
	}

	var used5hPercent float64
	if primary != nil && primary.UsedPercent != nil {
		used5hPercent = *primary.UsedPercent
	}
	var used7dPercent float64
	if secondary != nil && secondary.UsedPercent != nil {
		used7dPercent = *secondary.UsedPercent
	}
	return used5hPercent, used7dPercent, nil
}
