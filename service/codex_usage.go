package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const codexUsageOtherInfoKey = "codex_usage"

type CodexUsageWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	ResetAt            int64   `json:"reset_at,omitempty"`
	LimitWindowSeconds int64   `json:"limit_window_seconds,omitempty"`
}

type CodexUsageSnapshot struct {
	PlanType        string            `json:"plan_type,omitempty"`
	LimitReached    bool              `json:"limit_reached"`
	PrimaryWindow   *CodexUsageWindow `json:"primary_window,omitempty"`
	SecondaryWindow *CodexUsageWindow `json:"secondary_window,omitempty"`
	UpdatedAt       int64             `json:"updated_at"`
}

type CodexWhamFetchFunc func(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	accessToken string,
	accountID string,
) (statusCode int, body []byte, err error)

func CodexChannelCredential(ch *model.Channel) (*CodexOAuthKey, error) {
	oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(oauthKey.AccessToken) == "" {
		return nil, errors.New("codex channel: access_token is required")
	}
	if strings.TrimSpace(oauthKey.AccountID) == "" {
		return nil, errors.New("codex channel: account_id is required")
	}
	return oauthKey, nil
}

// FetchCodexChannelWham 调用 wham 接口；access token 失效且持有 refresh token 时刷新凭证后重试一次。
func FetchCodexChannelWham(ctx context.Context, ch *model.Channel, oauthKey *CodexOAuthKey, fetch CodexWhamFetchFunc) (int, []byte, error) {
	client, err := GetHttpClientWithProxy(ch.GetSetting().Proxy)
	if err != nil {
		return 0, nil, err
	}
	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)

	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	statusCode, body, err := fetch(fetchCtx, client, ch.GetBaseURL(), accessToken, accountID)
	if err != nil {
		return 0, nil, err
	}
	if (statusCode != http.StatusUnauthorized && statusCode != http.StatusForbidden) || strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return statusCode, body, nil
	}

	refreshed, _, err := RefreshCodexChannelCredential(ctx, ch.Id, CodexCredentialRefreshOptions{ResetCaches: true})
	if err != nil {
		return statusCode, body, nil
	}
	retryCtx, cancelRetry := context.WithTimeout(ctx, 15*time.Second)
	defer cancelRetry()
	return fetch(retryCtx, client, ch.GetBaseURL(), strings.TrimSpace(refreshed.AccessToken), accountID)
}

// SyncCodexChannelUsage 拉取用量并把摘要写入渠道 other_info，供列表页的进度条展示。
func SyncCodexChannelUsage(ctx context.Context, ch *model.Channel, oauthKey *CodexOAuthKey) (int, []byte, error) {
	statusCode, body, err := FetchCodexChannelWham(ctx, ch, oauthKey, FetchCodexWhamUsage)
	if err != nil {
		return 0, nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return statusCode, body, nil
	}
	snapshot, err := parseCodexUsageSnapshot(body, time.Now())
	if err != nil {
		return statusCode, body, nil
	}
	if err := model.SetChannelOtherInfoEntry(ch.Id, codexUsageOtherInfoKey, snapshot); err != nil {
		common.SysError(fmt.Sprintf("codex usage: save snapshot failed: channel_id=%d err=%v", ch.Id, err))
	}
	return statusCode, body, nil
}

func parseCodexUsageSnapshot(body []byte, now time.Time) (*CodexUsageSnapshot, error) {
	var payload struct {
		PlanType  string `json:"plan_type"`
		RateLimit struct {
			LimitReached    bool              `json:"limit_reached"`
			PrimaryWindow   *CodexUsageWindow `json:"primary_window"`
			SecondaryWindow *CodexUsageWindow `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.RateLimit.PrimaryWindow == nil && payload.RateLimit.SecondaryWindow == nil {
		return nil, errors.New("codex usage: no rate limit window in payload")
	}
	for _, w := range []*CodexUsageWindow{payload.RateLimit.PrimaryWindow, payload.RateLimit.SecondaryWindow} {
		if w != nil {
			w.UsedPercent = min(max(w.UsedPercent, 0), 100)
		}
	}
	return &CodexUsageSnapshot{
		PlanType:        payload.PlanType,
		LimitReached:    payload.RateLimit.LimitReached,
		PrimaryWindow:   payload.RateLimit.PrimaryWindow,
		SecondaryWindow: payload.RateLimit.SecondaryWindow,
		UpdatedAt:       now.Unix(),
	}, nil
}
