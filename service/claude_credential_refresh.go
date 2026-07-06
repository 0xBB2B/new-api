package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type ClaudeCredentialRefreshOptions struct {
	ResetCaches bool
}

type claudeOAuthCredential struct {
	AccessToken      string   `json:"accessToken,omitempty"`
	RefreshToken     string   `json:"refreshToken,omitempty"`
	ExpiresAt        int64    `json:"expiresAt,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
	SubscriptionType string   `json:"subscriptionType,omitempty"`
}

type claudeCredentialEnvelope struct {
	ClaudeAiOauth *claudeOAuthCredential `json:"claudeAiOauth"`
}

func parseClaudeCredentialEnvelope(raw string) (*claudeCredentialEnvelope, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("claude subscription channel: empty credential key")
	}
	var env claudeCredentialEnvelope
	if err := common.Unmarshal([]byte(raw), &env); err != nil {
		return nil, errors.New("claude subscription channel: invalid credential key json")
	}
	if env.ClaudeAiOauth == nil {
		return nil, errors.New("claude subscription channel: credential key json must include claudeAiOauth")
	}
	return &env, nil
}

func buildRefreshedCredentialKey(oldKeyJSON string, result *ClaudeOAuthTokenResult) (string, error) {
	var envelope map[string]any
	if err := common.Unmarshal([]byte(oldKeyJSON), &envelope); err != nil {
		return "", errors.New("claude subscription channel: invalid oauth key json")
	}
	oauth, ok := envelope["claudeAiOauth"].(map[string]any)
	if !ok || oauth == nil {
		return "", errors.New("claude subscription channel: missing claudeAiOauth")
	}
	oauth["accessToken"] = result.AccessToken
	oauth["refreshToken"] = result.RefreshToken
	oauth["expiresAt"] = result.ExpiresAt.UnixMilli()
	envelope["claudeAiOauth"] = oauth

	encoded, err := common.Marshal(envelope)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func RefreshClaudeChannelCredential(ctx context.Context, channelID int, opts ClaudeCredentialRefreshOptions) (*claudeOAuthCredential, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeClaudeSubscription {
		return nil, nil, fmt.Errorf("channel type is not Claude Subscription")
	}

	env, err := parseClaudeCredentialEnvelope(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, nil, err
	}
	refreshToken := strings.TrimSpace(env.ClaudeAiOauth.RefreshToken)
	if refreshToken == "" {
		return nil, nil, fmt.Errorf("claude subscription channel: refresh_token is required to refresh credential")
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := RefreshClaudeOAuthTokenWithProxy(refreshCtx, refreshToken, ch.GetSetting().Proxy)
	if err != nil {
		return nil, nil, err
	}

	newKey, err := buildRefreshedCredentialKey(ch.Key, res)
	if err != nil {
		return nil, nil, err
	}

	if err := model.DB.Model(&model.Channel{}).Where("id = ?", ch.Id).Update("key", newKey).Error; err != nil {
		return nil, nil, err
	}

	if opts.ResetCaches {
		model.InitChannelCache()
		ResetProxyClientCache()
	}

	env.ClaudeAiOauth.AccessToken = res.AccessToken
	env.ClaudeAiOauth.RefreshToken = res.RefreshToken
	env.ClaudeAiOauth.ExpiresAt = res.ExpiresAt.UnixMilli()

	return env.ClaudeAiOauth, ch, nil
}
