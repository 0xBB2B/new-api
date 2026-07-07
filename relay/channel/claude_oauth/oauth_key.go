package claude_oauth

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
)

type OAuthKey struct {
	AccessToken      string   `json:"accessToken,omitempty"`
	RefreshToken     string   `json:"refreshToken,omitempty"`
	ExpiresAt        int64    `json:"expiresAt,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
	SubscriptionType string   `json:"subscriptionType,omitempty"`
}

type oauthKeyEnvelope struct {
	ClaudeAiOauth *OAuthKey `json:"claudeAiOauth"`
}

func ParseOAuthKey(raw string) (*OAuthKey, error) {
	if raw == "" {
		return nil, errors.New("claude subscription channel: empty oauth key")
	}
	var envelope oauthKeyEnvelope
	if err := common.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, errors.New("claude subscription channel: invalid oauth key json")
	}
	if envelope.ClaudeAiOauth == nil {
		return nil, errors.New("claude subscription channel: missing claudeAiOauth field")
	}
	if envelope.ClaudeAiOauth.AccessToken == "" {
		return nil, errors.New("claude subscription channel: access_token is required")
	}
	return envelope.ClaudeAiOauth, nil
}
