package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchClaudeOAuthUsage_Success_SendsCorrectRequest(t *testing.T) {
	var receivedPath, receivedMethod string
	var receivedAuth, receivedBeta, receivedUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedMethod = r.Method
		receivedAuth = r.Header.Get("Authorization")
		receivedBeta = r.Header.Get("anthropic-beta")
		receivedUA = r.Header.Get("User-Agent")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":10}}`))
	}))
	defer server.Close()

	statusCode, body, err := FetchClaudeOAuthUsage(context.Background(), &http.Client{}, server.URL, "my-access-token")
	require.NoError(t, err)

	assert.Equal(t, "/api/oauth/usage", receivedPath)
	assert.Equal(t, http.MethodGet, receivedMethod)
	assert.Equal(t, "Bearer my-access-token", receivedAuth)
	assert.Equal(t, "oauth-2025-04-20", receivedBeta)
	assert.Equal(t, "claude-code/2.1.229", receivedUA)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, string(body), "five_hour")
}

func TestFetchClaudeOAuthUsage_EmptyBaseURL_ReturnsErrorWithoutRequest(t *testing.T) {
	_, _, err := FetchClaudeOAuthUsage(context.Background(), &http.Client{}, "", "my-access-token")
	assert.Error(t, err)
}

func TestFetchClaudeOAuthUsage_EmptyAccessToken_ReturnsErrorWithoutRequest(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, _, err := FetchClaudeOAuthUsage(context.Background(), &http.Client{}, server.URL, "")
	assert.Error(t, err)
	assert.Equal(t, 0, requestCount)
}

func TestParseClaudeOAuthUsageBottleneck(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		want    float64
		wantErr bool
	}{
		{
			name: "top-level windows return max utilization",
			body: []byte(`{"five_hour":{"utilization":23},"seven_day":{"utilization":12},"seven_day_opus":{"utilization":68}}`),
			want: 68,
		},
		{
			name: "limits array returns max percent",
			body: []byte(`{"limits":[{"kind":"session","percent":40},{"kind":"weekly_all","percent":85}]}`),
			want: 85,
		},
		{
			name: "top-level windows and limits combined return global max",
			body: []byte(`{"five_hour":{"utilization":23},"limits":[{"kind":"weekly_all","percent":85}]}`),
			want: 85,
		},
		{
			name: "missing utilization on present window defaults to zero",
			body: []byte(`{"five_hour":{}}`),
			want: 0,
		},
		{
			name: "seven_day prefixed scoped window counts toward bottleneck",
			body: []byte(`{"five_hour":{"utilization":10},"seven_day_sonnet":{"utilization":91}}`),
			want: 91,
		},
		{
			name: "extra_usage object is not a usage window",
			body: []byte(`{"seven_day":{"utilization":12},"extra_usage":{"is_enabled":true,"utilization":99}}`),
			want: 12,
		},
		{
			name:    "no usage windows present is invalid",
			body:    []byte(`{}`),
			wantErr: true,
		},
		{
			name:    "non-JSON body fails to parse",
			body:    []byte(`not json`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseClaudeOAuthUsageBottleneck(tt.body)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
