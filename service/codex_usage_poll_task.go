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

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	codexUsagePollTickInterval = 60 * time.Second
	codexUsagePollBatchSize    = 200
)

var (
	codexUsagePollOnce    sync.Once
	codexUsagePollRunning atomic.Bool
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
			Select("id", "name", "key", "status", "setting", "base_url").
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

		for _, ch := range channels {
			if err := pollCodexChannelUsage(ctx, nil, ch); err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("codex usage poll: channel_id=%d name=%s failed: %v", ch.Id, ch.Name, err))
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
		client, err = NewProxyHttpClient(ch.GetSetting().Proxy)
		if err != nil {
			return err
		}
	}

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
