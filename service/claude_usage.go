package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	claudeFiveHourWindowSeconds = 5 * 60 * 60
	claudeSevenDayWindowSeconds = 7 * 24 * 60 * 60
)

func ClaudeChannelCredential(ch *model.Channel) (*claudeOAuthCredential, error) {
	env, err := parseClaudeCredentialEnvelope(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(env.ClaudeAiOauth.AccessToken) == "" {
		return nil, errors.New("claude subscription channel: accessToken is required")
	}
	return env.ClaudeAiOauth, nil
}

func fetchClaudeOAuthUsage(ctx context.Context, client *http.Client, baseURL string, accessToken string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/oauth/usage", nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

// SyncClaudeChannelUsage 拉取 Claude 订阅用量；access token 失效时刷新凭证重试一次，成功后把摘要写入 other_info。
func SyncClaudeChannelUsage(ctx context.Context, ch *model.Channel, cred *claudeOAuthCredential) (int, []byte, *SubscriptionUsageSnapshot, error) {
	client, err := GetHttpClientWithProxy(ch.GetSetting().Proxy)
	if err != nil {
		return 0, nil, nil, err
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	statusCode, body, err := fetchClaudeOAuthUsage(fetchCtx, client, ch.GetBaseURL(), cred.AccessToken)
	if err != nil {
		return 0, nil, nil, err
	}
	if (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) && strings.TrimSpace(cred.RefreshToken) != "" {
		if refreshed, _, refreshErr := RefreshClaudeChannelCredential(ctx, ch.Id, ClaudeCredentialRefreshOptions{ResetCaches: true}); refreshErr == nil {
			cred = refreshed
			retryCtx, cancelRetry := context.WithTimeout(ctx, 15*time.Second)
			defer cancelRetry()
			statusCode, body, err = fetchClaudeOAuthUsage(retryCtx, client, ch.GetBaseURL(), cred.AccessToken)
			if err != nil {
				return 0, nil, nil, err
			}
		}
	}
	if statusCode < 200 || statusCode >= 300 {
		return statusCode, body, nil, nil
	}
	snapshot, err := parseClaudeUsageSnapshot(body, cred.SubscriptionType, time.Now())
	if err != nil {
		return statusCode, body, nil, nil
	}
	saveSubscriptionUsageSnapshot(ch.Id, snapshot)
	return statusCode, body, snapshot, nil
}

func parseClaudeUsageSnapshot(body []byte, planType string, now time.Time) (*SubscriptionUsageSnapshot, error) {
	type window struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	}
	var payload struct {
		FiveHour *window `json:"five_hour"`
		SevenDay *window `json:"seven_day"`
	}
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.FiveHour == nil && payload.SevenDay == nil {
		return nil, errors.New("claude usage: no rate limit window in payload")
	}
	toWindow := func(w *window, seconds int64) *SubscriptionUsageWindow {
		if w == nil {
			return nil
		}
		out := &SubscriptionUsageWindow{UsedPercent: clampUsagePercent(w.Utilization), LimitWindowSeconds: seconds}
		if resetAt, err := time.Parse(time.RFC3339, strings.TrimSpace(w.ResetsAt)); err == nil {
			out.ResetAt = resetAt.Unix()
		}
		return out
	}
	snapshot := &SubscriptionUsageSnapshot{
		PlanType:        strings.TrimSpace(planType),
		PrimaryWindow:   toWindow(payload.FiveHour, claudeFiveHourWindowSeconds),
		SecondaryWindow: toWindow(payload.SevenDay, claudeSevenDayWindowSeconds),
		UpdatedAt:       now.Unix(),
	}
	for _, w := range []*SubscriptionUsageWindow{snapshot.PrimaryWindow, snapshot.SecondaryWindow} {
		if w != nil && w.UsedPercent >= 100 {
			snapshot.LimitReached = true
		}
	}
	return snapshot, nil
}
