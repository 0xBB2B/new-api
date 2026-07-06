package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshClaudeOAuthToken_Success_RotatesTokenAndParsesExpiry(t *testing.T) {
	var receivedMethod, receivedContentType string
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &receivedBody))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		respBody, err := common.Marshal(map[string]any{
			"access_token":  "new-at",
			"refresh_token": "new-rt",
			"expires_in":    3600,
		})
		require.NoError(t, err)
		_, _ = w.Write(respBody)
	}))
	defer server.Close()

	before := time.Now()
	result, err := refreshClaudeOAuthToken(context.Background(), &http.Client{}, server.URL, "test-client-id", "my-rt")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, http.MethodPost, receivedMethod)
	assert.Contains(t, receivedContentType, "application/json")
	assert.Equal(t, "refresh_token", receivedBody["grant_type"])
	assert.Equal(t, "my-rt", receivedBody["refresh_token"])
	assert.Equal(t, "test-client-id", receivedBody["client_id"])

	assert.Equal(t, "new-at", result.AccessToken)
	assert.Equal(t, "new-rt", result.RefreshToken)
	assert.True(t, result.ExpiresAt.After(before.Add(3500*time.Second)))
	assert.True(t, result.ExpiresAt.Before(before.Add(3700*time.Second)))
}

func TestRefreshClaudeOAuthToken_MissingFields_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		respBody, err := common.Marshal(map[string]any{
			"access_token": "new-at",
		})
		require.NoError(t, err)
		_, _ = w.Write(respBody)
	}))
	defer server.Close()

	result, err := refreshClaudeOAuthToken(context.Background(), &http.Client{}, server.URL, "test-client-id", "my-rt")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestRefreshClaudeOAuthToken_NonSuccessStatus_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	result, err := refreshClaudeOAuthToken(context.Background(), &http.Client{}, server.URL, "test-client-id", "my-rt")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestRefreshClaudeOAuthToken_EmptyRefreshToken_ReturnsError(t *testing.T) {
	result, err := refreshClaudeOAuthToken(context.Background(), &http.Client{}, "http://unused.invalid", "test-client-id", "")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestClaudeCredentialNeedsRefresh(t *testing.T) {
	now := time.Now()
	threshold := 24 * time.Hour

	tests := []struct {
		name            string
		expiresAtMillis int64
		want            bool
	}{
		{
			name:            "expires in 20h under threshold needs refresh",
			expiresAtMillis: now.Add(20 * time.Hour).UnixMilli(),
			want:            true,
		},
		{
			name:            "expires in 40h beyond threshold does not need refresh",
			expiresAtMillis: now.Add(40 * time.Hour).UnixMilli(),
			want:            false,
		},
		{
			name:            "zero expiresAt is invalid and needs refresh",
			expiresAtMillis: 0,
			want:            true,
		},
		{
			name:            "already expired needs refresh",
			expiresAtMillis: now.Add(-1 * time.Hour).UnixMilli(),
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := claudeCredentialNeedsRefresh(tt.expiresAtMillis, now, threshold)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildRefreshedCredentialKey_PreservesNestedStructureAndOtherFields(t *testing.T) {
	oldKeyJSON := `{"claudeAiOauth":{"accessToken":"old-at","refreshToken":"old-rt","expiresAt":1000,"subscriptionType":"max"}}`
	result := &ClaudeOAuthTokenResult{
		AccessToken:  "new-at",
		RefreshToken: "new-rt",
		ExpiresAt:    time.UnixMilli(9999000),
	}

	newKeyJSON, err := buildRefreshedCredentialKey(oldKeyJSON, result)
	require.NoError(t, err)

	var parsed struct {
		ClaudeAiOauth struct {
			AccessToken      string `json:"accessToken"`
			RefreshToken     string `json:"refreshToken"`
			ExpiresAt        int64  `json:"expiresAt"`
			SubscriptionType string `json:"subscriptionType"`
		} `json:"claudeAiOauth"`
	}
	require.NoError(t, common.UnmarshalJsonStr(newKeyJSON, &parsed))

	assert.Equal(t, "new-at", parsed.ClaudeAiOauth.AccessToken)
	assert.Equal(t, "new-rt", parsed.ClaudeAiOauth.RefreshToken)
	assert.Equal(t, int64(9999000), parsed.ClaudeAiOauth.ExpiresAt)
	assert.Equal(t, "max", parsed.ClaudeAiOauth.SubscriptionType)
}

func TestBuildRefreshedCredentialKey_PreservesUnmodeledFields(t *testing.T) {
	oldKeyJSON := `{"claudeAiOauth":{"accessToken":"old-at","refreshToken":"old-rt","expiresAt":1000,"subscriptionType":"max","isMax":true,"account":{"uuid":"u-123"}}}`
	result := &ClaudeOAuthTokenResult{
		AccessToken:  "new-at",
		RefreshToken: "new-rt",
		ExpiresAt:    time.UnixMilli(9999000),
	}

	newKeyJSON, err := buildRefreshedCredentialKey(oldKeyJSON, result)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, common.UnmarshalJsonStr(newKeyJSON, &parsed))

	oauth, ok := parsed["claudeAiOauth"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "new-at", oauth["accessToken"])
	assert.Equal(t, "new-rt", oauth["refreshToken"])
	assert.Equal(t, int64(9999000), int64(oauth["expiresAt"].(float64)))
	assert.Equal(t, "max", oauth["subscriptionType"])

	assert.Equal(t, true, oauth["isMax"])
	account, ok := oauth["account"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "u-123", account["uuid"])
}
