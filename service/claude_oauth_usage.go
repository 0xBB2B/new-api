package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const claudeUsageUserAgent = "claude-code/2.1.229"

func FetchClaudeOAuthUsage(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	accessToken string,
) (statusCode int, body []byte, err error) {
	if client == nil {
		return 0, nil, fmt.Errorf("nil http client")
	}
	bu := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if bu == "" {
		return 0, nil, fmt.Errorf("empty baseURL")
	}
	at := strings.TrimSpace(accessToken)
	if at == "" {
		return 0, nil, fmt.Errorf("empty accessToken")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bu+"/api/oauth/usage", nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+at)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", claudeUsageUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func FetchClaudeChannelUsage(ctx context.Context, ch *model.Channel) (statusCode int, body []byte, subscriptionType string, err error) {
	envelope, err := parseClaudeCredentialEnvelope(strings.TrimSpace(ch.Key))
	if err != nil {
		return 0, nil, "", err
	}
	accessToken := strings.TrimSpace(envelope.ClaudeAiOauth.AccessToken)
	if accessToken == "" {
		return 0, nil, "", fmt.Errorf("claude subscription channel: access_token is required")
	}
	subscriptionType = envelope.ClaudeAiOauth.SubscriptionType

	client, err := GetHttpClientWithProxy(ch.GetSetting().Proxy)
	if err != nil {
		return 0, nil, "", err
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	statusCode, body, err = FetchClaudeOAuthUsage(fetchCtx, client, ch.GetBaseURL(), accessToken)
	if err != nil {
		return 0, nil, "", err
	}

	if (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) && strings.TrimSpace(envelope.ClaudeAiOauth.RefreshToken) != "" {
		newCred, _, refreshErr := RefreshClaudeChannelCredential(ctx, ch.Id, ClaudeCredentialRefreshOptions{ResetCaches: true})
		if refreshErr == nil {
			subscriptionType = newCred.SubscriptionType
			retryCtx, retryCancel := context.WithTimeout(ctx, 15*time.Second)
			defer retryCancel()
			statusCode, body, err = FetchClaudeOAuthUsage(retryCtx, client, ch.GetBaseURL(), newCred.AccessToken)
			if err != nil {
				return 0, nil, "", err
			}
		}
	}

	return statusCode, body, subscriptionType, nil
}

type claudeOAuthUsageWindow struct {
	Utilization *float64 `json:"utilization"`
}

type claudeOAuthUsageLimit struct {
	Percent *float64 `json:"percent"`
}

func isClaudeUsageWindowKey(key string) bool {
	return key == "five_hour" || strings.HasPrefix(key, "seven_day")
}

func parseClaudeOAuthUsageBottleneck(body []byte) (float64, error) {
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(body, &fields); err != nil {
		return 0, err
	}

	found := false
	bottleneck := 0.0
	record := func(value *float64) {
		found = true
		if value != nil && *value > bottleneck {
			bottleneck = *value
		}
	}

	for key, raw := range fields {
		if !isClaudeUsageWindowKey(key) {
			continue
		}
		var window claudeOAuthUsageWindow
		if err := common.Unmarshal(raw, &window); err != nil {
			return 0, err
		}
		record(window.Utilization)
	}

	if raw, ok := fields["limits"]; ok {
		var limits []claudeOAuthUsageLimit
		if err := common.Unmarshal(raw, &limits); err != nil {
			return 0, err
		}
		for _, limit := range limits {
			record(limit.Percent)
		}
	}

	if !found {
		return 0, fmt.Errorf("no usage windows present in claude oauth usage response")
	}
	return bottleneck, nil
}
