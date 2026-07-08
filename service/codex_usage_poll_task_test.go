package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodexWhamUsageWindows(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want5h  float64
		want7d  float64
		wantErr bool
	}{
		{
			name:   "both windows",
			body:   `{"rate_limit":{"primary_window":{"used_percent":12.5},"secondary_window":{"used_percent":34.75}}}`,
			want5h: 12.5,
			want7d: 34.75,
		},
		{
			name:   "primary only",
			body:   `{"rate_limit":{"primary_window":{"used_percent":12.5}}}`,
			want5h: 12.5,
			want7d: 0,
		},
		{
			name:   "secondary only",
			body:   `{"rate_limit":{"secondary_window":{"used_percent":34.75}}}`,
			want5h: 0,
			want7d: 34.75,
		},
		{
			name:    "missing both windows",
			body:    `{"rate_limit":{}}`,
			wantErr: true,
		},
		{
			name:   "window present without used percent",
			body:   `{"rate_limit":{"primary_window":{}}}`,
			want5h: 0,
			want7d: 0,
		},
		{
			name:   "explicit zero",
			body:   `{"rate_limit":{"primary_window":{"used_percent":0},"secondary_window":{"used_percent":0}}}`,
			want5h: 0,
			want7d: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got5h, got7d, err := parseCodexWhamUsageWindows([]byte(tt.body))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want5h, got5h)
			assert.Equal(t, tt.want7d, got7d)
		})
	}
}

func TestPollCodexChannelUsage_SuccessWritesCache(t *testing.T) {
	channelID := 65001
	model.CacheSetCodexChannelUsage(channelID, 96, 20)

	var logBuffer bytes.Buffer
	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultWriter
	gin.DefaultWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/backend-api/wham/usage", r.URL.Path)
		assert.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
		assert.Equal(t, "account-id", r.Header.Get("chatgpt-account-id"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, err := common.Marshal(map[string]any{
			"rate_limit": map[string]any{
				"primary_window":   map[string]any{"used_percent": 12.34},
				"secondary_window": map[string]any{"used_percent": 20.5},
			},
		})
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	baseURL := server.URL
	channel := &model.Channel{
		Id:      channelID,
		Key:     `{"access_token":"access-token","account_id":"account-id"}`,
		BaseURL: &baseURL,
	}

	err := pollCodexChannelUsage(context.Background(), server.Client(), channel)
	require.NoError(t, err)

	assert.False(t, model.CacheIsCodexChannelSaturated(channelID))
	assert.Contains(t, logBuffer.String(), "channel_id=65001")
	assert.Contains(t, logBuffer.String(), "bottleneck_usage=20.50")

	logBuffer.Reset()
	secondChannelID := 65007
	model.CacheSetCodexChannelUsage(secondChannelID, 96, 20)
	secondServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body, err := common.Marshal(map[string]any{
			"rate_limit": map[string]any{
				"primary_window":   map[string]any{"used_percent": 33.33},
				"secondary_window": map[string]any{"used_percent": 20.0},
			},
		})
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	defer secondServer.Close()
	channel.Id = secondChannelID
	channel.BaseURL = &secondServer.URL
	err = pollCodexChannelUsage(context.Background(), secondServer.Client(), channel)
	require.NoError(t, err)
	assert.False(t, model.CacheIsCodexChannelSaturated(secondChannelID))
	assert.Contains(t, logBuffer.String(), "channel_id=65007")
	assert.Contains(t, logBuffer.String(), "bottleneck_usage=33.33")
}

func TestPollCodexChannelUsage_NonSuccessStatusKeepsOldCache(t *testing.T) {
	channelID := 65002
	model.CacheSetCodexChannelUsage(channelID, 96, 20)

	var logBuffer bytes.Buffer
	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultWriter
	gin.DefaultWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	baseURL := server.URL
	channel := &model.Channel{
		Id:      channelID,
		Key:     `{"access_token":"access-token","account_id":"account-id"}`,
		BaseURL: &baseURL,
	}

	err := pollCodexChannelUsage(context.Background(), server.Client(), channel)
	assert.Error(t, err)
	assert.True(t, model.CacheIsCodexChannelSaturated(channelID))
	assert.NotContains(t, logBuffer.String(), "Codex 渠道使用率回归")
}

func TestPollCodexChannelUsage_UnauthorizedKeepsOldCacheAndKey(t *testing.T) {
	channelID := 65004
	model.CacheSetCodexChannelUsage(channelID, 96, 20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	baseURL := server.URL
	originalKey := `{"access_token":"access-token","account_id":"account-id","refresh_token":"refresh-token"}`
	channel := &model.Channel{Id: channelID, Key: originalKey, BaseURL: &baseURL}

	err := pollCodexChannelUsage(context.Background(), server.Client(), channel)
	assert.Error(t, err)
	assert.True(t, model.CacheIsCodexChannelSaturated(channelID))
	assert.Equal(t, originalKey, channel.Key)
}

func TestPollCodexChannelUsage_InvalidResponseKeepsOldCache(t *testing.T) {
	channelID := 65005
	model.CacheSetCodexChannelUsage(channelID, 96, 20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":`))
	}))
	defer server.Close()

	baseURL := server.URL
	channel := &model.Channel{
		Id:      channelID,
		Key:     `{"access_token":"access-token","account_id":"account-id"}`,
		BaseURL: &baseURL,
	}

	err := pollCodexChannelUsage(context.Background(), server.Client(), channel)
	assert.Error(t, err)
	assert.True(t, model.CacheIsCodexChannelSaturated(channelID))
}

func TestPollCodexChannelUsage_MissingCredentialDoesNotSendRequest(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseURL := server.URL
	channel := &model.Channel{Id: 65006, Key: `{}`, BaseURL: &baseURL}

	err := pollCodexChannelUsage(context.Background(), server.Client(), channel)
	assert.Error(t, err)
	assert.False(t, called)
}

func TestPollCodexChannelUsage_NetworkErrorKeepsOldCache(t *testing.T) {
	channelID := 65008
	model.CacheSetCodexChannelUsage(channelID, 96, 20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	baseURL := server.URL
	client := server.Client()
	server.Close()
	channel := &model.Channel{
		Id:      channelID,
		Key:     `{"access_token":"access-token","account_id":"account-id"}`,
		BaseURL: &baseURL,
	}

	err := pollCodexChannelUsage(context.Background(), client, channel)
	assert.Error(t, err)
	assert.True(t, model.CacheIsCodexChannelSaturated(channelID))
}

func TestPollCodexChannelUsage_SlowUpstreamFailsWithinRequestTimeout(t *testing.T) {
	channelID := 65009
	model.CacheSetCodexChannelUsage(channelID, 96, 20)

	originalTimeout := codexUsagePollRequestTimeout
	codexUsagePollRequestTimeout = 50 * time.Millisecond
	t.Cleanup(func() { codexUsagePollRequestTimeout = originalTimeout })

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })

	baseURL := server.URL
	channel := &model.Channel{
		Id:      channelID,
		Key:     `{"access_token":"access-token","account_id":"account-id"}`,
		BaseURL: &baseURL,
	}

	done := make(chan error, 1)
	go func() {
		done <- pollCodexChannelUsage(context.Background(), server.Client(), channel)
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.True(t, model.CacheIsCodexChannelSaturated(channelID))
	case <-time.After(2 * time.Second):
		t.Fatal("pollCodexChannelUsage 未在请求时限内返回，轮询会被慢上游卡死")
	}
}

func TestRunCodexUsagePollOnce_FiltersEnabledCodexAndContinuesAfterFailure(t *testing.T) {
	truncate(t)
	codexUsagePollRunning.Store(false)

	goodCalls := 0
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodCalls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":12},"secondary_window":{"used_percent":20}}}`))
	}))
	defer goodServer.Close()

	badCalls := 0
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		badCalls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":`))
	}))
	defer badServer.Close()

	disabledCalls := 0
	disabledServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		disabledCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer disabledServer.Close()

	autoDisabledCalls := 0
	autoDisabledServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		autoDisabledCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer autoDisabledServer.Close()

	nonCodexCalls := 0
	nonCodexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonCodexCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer nonCodexServer.Close()

	channels := []*model.Channel{
		{Id: 65101, Name: "bad", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &badServer.URL},
		{Id: 65102, Name: "good", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &goodServer.URL},
		{Id: 65103, Name: "disabled", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusManuallyDisabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &disabledServer.URL},
		{Id: 65104, Name: "auto-disabled", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusAutoDisabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &autoDisabledServer.URL},
		{Id: 65105, Name: "openai", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &nonCodexServer.URL},
	}
	for _, channel := range channels {
		require.NoError(t, model.DB.Create(channel).Error)
	}

	model.CacheSetCodexChannelUsage(65102, 96, 20)
	runCodexUsagePollOnce()

	assert.Equal(t, 1, badCalls)
	assert.Equal(t, 1, goodCalls)
	assert.Equal(t, 0, disabledCalls)
	assert.Equal(t, 0, autoDisabledCalls)
	assert.Equal(t, 0, nonCodexCalls)
	assert.False(t, model.CacheIsCodexChannelSaturated(65102))
}

func TestStartCodexUsagePollTask_NonMasterDoesNotStart(t *testing.T) {
	originalMaster := common.IsMasterNode
	originalOnce := codexUsagePollOnce
	common.IsMasterNode = false
	codexUsagePollOnce = sync.Once{}
	t.Cleanup(func() {
		common.IsMasterNode = originalMaster
		codexUsagePollOnce = originalOnce
	})

	StartCodexUsagePollTask()

	assert.False(t, codexUsagePollRunning.Load())
}

func TestCodexUsagePollTickInterval(t *testing.T) {
	assert.Equal(t, 60*time.Second, codexUsagePollTickInterval)
}

func TestStartCodexUsageSyncTask_NonMasterWithoutRedisLogsWarning(t *testing.T) {
	originalMaster := common.IsMasterNode
	originalRedis := common.RedisEnabled
	common.IsMasterNode = false
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.IsMasterNode = originalMaster
		common.RedisEnabled = originalRedis
	})

	var logBuffer bytes.Buffer
	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultWriter
	gin.DefaultWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	StartCodexUsageSyncTask()

	warning := logBuffer.String()
	require.NotEmpty(t, warning)
	assert.Contains(t, warning, "均衡")
	assert.Contains(t, warning, "触顶")
}

func TestPollCodexChannelUsage_InvalidKeyDoesNotSendRequest(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseURL := server.URL
	channel := &model.Channel{
		Id:      65003,
		Key:     `not-json`,
		BaseURL: &baseURL,
	}

	err := pollCodexChannelUsage(context.Background(), server.Client(), channel)
	assert.Error(t, err)
	assert.False(t, called)
}
