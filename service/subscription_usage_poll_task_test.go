package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	redis "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedRedisSet struct {
	key   string
	value string
}

func claudeOAuthKeyJSON(accessToken string) string {
	return `{"claudeAiOauth":{"accessToken":"` + accessToken + `","refreshToken":"refresh-token","expiresAt":1234567890123}}`
}

func TestSubscriptionUsageDueChannelTypes(t *testing.T) {
	tests := []struct {
		round int
		want  []int
	}{
		{round: 0, want: []int{constant.ChannelTypeCodex, constant.ChannelTypeClaudeSubscription}},
		{round: 1, want: []int{constant.ChannelTypeCodex}},
		{round: 2, want: []int{constant.ChannelTypeCodex}},
		{round: 3, want: []int{constant.ChannelTypeCodex, constant.ChannelTypeClaudeSubscription}},
		{round: 4, want: []int{constant.ChannelTypeCodex}},
		{round: 5, want: []int{constant.ChannelTypeCodex}},
		{round: 6, want: []int{constant.ChannelTypeCodex, constant.ChannelTypeClaudeSubscription}},
	}

	for _, tt := range tests {
		t.Run("round_"+strconv.Itoa(tt.round), func(t *testing.T) {
			got := subscriptionUsageDueChannelTypes(tt.round)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestPollClaudeChannelUsage_SuccessWritesCache(t *testing.T) {
	channelID := 66001
	model.CacheSetSubscriptionChannelUsage(channelID, 5)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/oauth/usage", r.URL.Path)
		assert.Equal(t, "Bearer claude-access-token", r.Header.Get("Authorization"))
		assert.Equal(t, "oauth-2025-04-20", r.Header.Get("anthropic-beta"))
		assert.Equal(t, "claude-code/2.1.229", r.Header.Get("User-Agent"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":23},"seven_day":{"utilization":77}}`))
	}))
	defer server.Close()

	baseURL := server.URL
	channel := &model.Channel{
		Id:      channelID,
		Type:    constant.ChannelTypeClaudeSubscription,
		Key:     claudeOAuthKeyJSON("claude-access-token"),
		BaseURL: &baseURL,
	}

	err := pollClaudeChannelUsage(context.Background(), server.Client(), channel)
	require.NoError(t, err)

	snapshotJSON, err := model.SubscriptionChannelUsageSnapshotJSON()
	require.NoError(t, err)
	var snapshot map[string]struct {
		BottleneckPercent float64 `json:"bottleneck_percent"`
	}
	require.NoError(t, common.Unmarshal(snapshotJSON, &snapshot))
	entry, ok := snapshot[strconv.Itoa(channelID)]
	require.True(t, ok)
	assert.Equal(t, 77.0, entry.BottleneckPercent)
}

func TestPollClaudeChannelUsage_UnauthorizedKeepsOldCache(t *testing.T) {
	channelID := 66002
	model.CacheSetSubscriptionChannelUsage(channelID, 96)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	baseURL := server.URL
	channel := &model.Channel{
		Id:      channelID,
		Type:    constant.ChannelTypeClaudeSubscription,
		Key:     claudeOAuthKeyJSON("claude-access-token"),
		BaseURL: &baseURL,
	}

	originalKey := channel.Key
	err := pollClaudeChannelUsage(context.Background(), server.Client(), channel)
	assert.Error(t, err)
	assert.True(t, model.CacheIsSubscriptionChannelSaturated(channelID))
	assert.Equal(t, originalKey, channel.Key)
}

func TestRunSubscriptionUsagePollOnce_OnlyPollsGivenChannelTypes(t *testing.T) {
	truncate(t)

	codexCalls := 0
	codexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		codexCalls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":10},"secondary_window":{"used_percent":15}}}`))
	}))
	defer codexServer.Close()

	claudeCalls := 0
	claudeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claudeCalls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":10},"seven_day":{"utilization":15}}`))
	}))
	defer claudeServer.Close()

	codexChannel := &model.Channel{Id: 66101, Name: "codex", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &codexServer.URL}
	claudeChannel := &model.Channel{Id: 66102, Name: "claude", Type: constant.ChannelTypeClaudeSubscription, Status: common.ChannelStatusEnabled, Key: claudeOAuthKeyJSON("claude-access-token"), BaseURL: &claudeServer.URL}
	require.NoError(t, model.DB.Create(codexChannel).Error)
	require.NoError(t, model.DB.Create(claudeChannel).Error)

	runSubscriptionUsagePollOnce([]int{constant.ChannelTypeCodex})
	assert.Equal(t, 1, codexCalls)
	assert.Equal(t, 0, claudeCalls)

	runSubscriptionUsagePollOnce([]int{constant.ChannelTypeCodex, constant.ChannelTypeClaudeSubscription})
	assert.Equal(t, 2, codexCalls)
	assert.Equal(t, 1, claudeCalls)
}

func TestRunSubscriptionUsagePollOnce_SkipsDisabledClaudeChannel(t *testing.T) {
	truncate(t)

	claudeCalls := 0
	claudeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claudeCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer claudeServer.Close()

	claudeChannel := &model.Channel{Id: 66103, Name: "claude-disabled", Type: constant.ChannelTypeClaudeSubscription, Status: common.ChannelStatusManuallyDisabled, Key: claudeOAuthKeyJSON("claude-access-token"), BaseURL: &claudeServer.URL}
	require.NoError(t, model.DB.Create(claudeChannel).Error)

	runSubscriptionUsagePollOnce([]int{constant.ChannelTypeCodex, constant.ChannelTypeClaudeSubscription})
	assert.Equal(t, 0, claudeCalls)
}

func TestRunSubscriptionUsagePollOnce_ClaudeFailureDoesNotBlockCodexInSameRound(t *testing.T) {
	truncate(t)

	codexCalls := 0
	codexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		codexCalls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":10},"secondary_window":{"used_percent":15}}}`))
	}))
	defer codexServer.Close()

	claudeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer claudeServer.Close()

	codexChannel := &model.Channel{Id: 66104, Name: "codex", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &codexServer.URL}
	claudeChannel := &model.Channel{Id: 66105, Name: "claude-401", Type: constant.ChannelTypeClaudeSubscription, Status: common.ChannelStatusEnabled, Key: claudeOAuthKeyJSON("claude-access-token"), BaseURL: &claudeServer.URL}
	require.NoError(t, model.DB.Create(codexChannel).Error)
	require.NoError(t, model.DB.Create(claudeChannel).Error)

	runSubscriptionUsagePollOnce([]int{constant.ChannelTypeCodex, constant.ChannelTypeClaudeSubscription})

	assert.Equal(t, 1, codexCalls)
	snapshotJSON, err := model.SubscriptionChannelUsageSnapshotJSON()
	require.NoError(t, err)
	var snapshot map[string]struct {
		BottleneckPercent float64 `json:"bottleneck_percent"`
	}
	require.NoError(t, common.Unmarshal(snapshotJSON, &snapshot))
	codexEntry, ok := snapshot[strconv.Itoa(codexChannel.Id)]
	require.True(t, ok)
	assert.Equal(t, 15.0, codexEntry.BottleneckPercent)
}

func TestRunSubscriptionUsagePollOnce_PublishesRedisSnapshotUnderSubscriptionKey(t *testing.T) {
	truncate(t)
	require.NoError(t, model.CacheLoadSubscriptionChannelUsageSnapshotJSON([]byte(`{}`)))
	redisSets := captureSubscriptionUsageRedisSets(t)

	claudeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":18},"seven_day":{"utilization":42}}`))
	}))
	defer claudeServer.Close()

	claudeChannel := &model.Channel{Id: 66106, Name: "claude", Type: constant.ChannelTypeClaudeSubscription, Status: common.ChannelStatusEnabled, Key: claudeOAuthKeyJSON("claude-access-token"), BaseURL: &claudeServer.URL}
	require.NoError(t, model.DB.Create(claudeChannel).Error)

	runSubscriptionUsagePollOnce([]int{constant.ChannelTypeClaudeSubscription})

	var snapshotSet capturedRedisSet
	select {
	case snapshotSet = <-redisSets:
	default:
		t.Fatal("订阅渠道用量轮询未发布 Redis 快照")
	}

	assert.Equal(t, "subscription_channel_usage_snapshot", snapshotSet.key)
	var snapshot map[string]struct {
		BottleneckPercent float64 `json:"bottleneck_percent"`
	}
	require.NoError(t, common.Unmarshal([]byte(snapshotSet.value), &snapshot))
	entry, ok := snapshot[strconv.Itoa(claudeChannel.Id)]
	require.True(t, ok)
	assert.Equal(t, 42.0, entry.BottleneckPercent)
}

func TestParseCodexWhamUsageBottleneck(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    float64
		wantErr bool
	}{
		{
			name: "both windows",
			body: `{"rate_limit":{"primary_window":{"used_percent":12.5},"secondary_window":{"used_percent":34.75}}}`,
			want: 34.75,
		},
		{
			name: "primary only",
			body: `{"rate_limit":{"primary_window":{"used_percent":12.5}}}`,
			want: 12.5,
		},
		{
			name: "secondary only",
			body: `{"rate_limit":{"secondary_window":{"used_percent":34.75}}}`,
			want: 34.75,
		},
		{
			name:    "missing both windows",
			body:    `{"rate_limit":{}}`,
			wantErr: true,
		},
		{
			name: "window present without used percent",
			body: `{"rate_limit":{"primary_window":{}}}`,
			want: 0,
		},
		{
			name: "explicit zero",
			body: `{"rate_limit":{"primary_window":{"used_percent":0},"secondary_window":{"used_percent":0}}}`,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCodexWhamUsageBottleneck([]byte(tt.body))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPollCodexChannelUsage_SuccessWritesCache(t *testing.T) {
	channelID := 65001
	model.CacheSetSubscriptionChannelUsage(channelID, 96)

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

	assert.False(t, model.CacheIsSubscriptionChannelSaturated(channelID))
	assert.Contains(t, logBuffer.String(), "channel_id=65001")
	assert.Contains(t, logBuffer.String(), "bottleneck_usage=20.50")

	logBuffer.Reset()
	secondChannelID := 65007
	model.CacheSetSubscriptionChannelUsage(secondChannelID, 96)
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
	assert.False(t, model.CacheIsSubscriptionChannelSaturated(secondChannelID))
	assert.Contains(t, logBuffer.String(), "channel_id=65007")
	assert.Contains(t, logBuffer.String(), "bottleneck_usage=33.33")
}

func TestPollCodexChannelUsage_NonSuccessStatusKeepsOldCache(t *testing.T) {
	channelID := 65002
	model.CacheSetSubscriptionChannelUsage(channelID, 96)

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
	assert.True(t, model.CacheIsSubscriptionChannelSaturated(channelID))
	assert.NotContains(t, logBuffer.String(), "订阅渠道使用率回归")
}

func TestPollCodexChannelUsage_UnauthorizedKeepsOldCacheAndKey(t *testing.T) {
	channelID := 65004
	model.CacheSetSubscriptionChannelUsage(channelID, 96)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	baseURL := server.URL
	originalKey := `{"access_token":"access-token","account_id":"account-id","refresh_token":"refresh-token"}`
	channel := &model.Channel{Id: channelID, Key: originalKey, BaseURL: &baseURL}

	err := pollCodexChannelUsage(context.Background(), server.Client(), channel)
	assert.Error(t, err)
	assert.True(t, model.CacheIsSubscriptionChannelSaturated(channelID))
	assert.Equal(t, originalKey, channel.Key)
}

func TestPollCodexChannelUsage_InvalidResponseKeepsOldCache(t *testing.T) {
	channelID := 65005
	model.CacheSetSubscriptionChannelUsage(channelID, 96)
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
	assert.True(t, model.CacheIsSubscriptionChannelSaturated(channelID))
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
	model.CacheSetSubscriptionChannelUsage(channelID, 96)
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
	assert.True(t, model.CacheIsSubscriptionChannelSaturated(channelID))
}

func TestPollSubscriptionChannelUsage_SlowUpstreamFailsWithinRequestTimeout(t *testing.T) {
	tests := []struct {
		name        string
		channelID   int
		channelType int
		key         string
	}{
		{
			name:        "codex channel",
			channelID:   65009,
			channelType: constant.ChannelTypeCodex,
			key:         `{"access_token":"access-token","account_id":"account-id"}`,
		},
		{
			name:        "claude subscription channel",
			channelID:   65010,
			channelType: constant.ChannelTypeClaudeSubscription,
			key:         claudeOAuthKeyJSON("claude-access-token"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.CacheSetSubscriptionChannelUsage(tt.channelID, 96)

			originalTimeout := subscriptionUsagePollRequestTimeout
			subscriptionUsagePollRequestTimeout = 50 * time.Millisecond
			t.Cleanup(func() { subscriptionUsagePollRequestTimeout = originalTimeout })

			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				<-release
			}))
			t.Cleanup(server.Close)
			t.Cleanup(func() { close(release) })

			baseURL := server.URL
			channel := &model.Channel{
				Id:      tt.channelID,
				Type:    tt.channelType,
				Key:     tt.key,
				BaseURL: &baseURL,
			}

			done := make(chan error, 1)
			go func() {
				done <- pollSubscriptionChannelUsage(context.Background(), channel)
			}()

			select {
			case err := <-done:
				require.Error(t, err)
				assert.True(t, model.CacheIsSubscriptionChannelSaturated(tt.channelID))
			case <-time.After(2 * time.Second):
				t.Fatal("pollSubscriptionChannelUsage 未在请求时限内返回，轮询会被慢上游卡死")
			}
		})
	}
}

func TestRunSubscriptionUsagePollOnce_FiltersEnabledCodexAndContinuesAfterFailure(t *testing.T) {
	truncate(t)
	subscriptionUsagePollRunning.Store(false)

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

	model.CacheSetSubscriptionChannelUsage(65102, 96)
	runSubscriptionUsagePollOnce([]int{constant.ChannelTypeCodex})

	assert.Equal(t, 1, badCalls)
	assert.Equal(t, 1, goodCalls)
	assert.Equal(t, 0, disabledCalls)
	assert.Equal(t, 0, autoDisabledCalls)
	assert.Equal(t, 0, nonCodexCalls)
	assert.False(t, model.CacheIsSubscriptionChannelSaturated(65102))
}

func TestRunSubscriptionUsagePollOnce_PublishesRedisSnapshotAfterEachBatch(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, model.CacheLoadSubscriptionChannelUsageSnapshotJSON([]byte(`{}`)))
	subscriptionUsagePollRunning.Store(false)
	redisSets := captureSubscriptionUsageRedisSets(t)

	var firstBatchCalls atomic.Int64
	firstBatchDone := make(chan struct{})
	var closeFirstBatchDone sync.Once
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if firstBatchCalls.Add(1) == int64(subscriptionUsagePollBatchSize) {
			closeFirstBatchDone.Do(func() { close(firstBatchDone) })
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":12},"secondary_window":{"used_percent":20}}}`))
	}))
	defer successServer.Close()

	blockedStarted := make(chan struct{})
	releaseBlocked := make(chan struct{})
	var closeBlockedStarted sync.Once
	var closeReleaseBlocked sync.Once
	blockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		closeBlockedStarted.Do(func() { close(blockedStarted) })
		<-releaseBlocked
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":22},"secondary_window":{"used_percent":30}}}`))
	}))
	t.Cleanup(func() { closeReleaseBlocked.Do(func() { close(releaseBlocked) }) })
	defer blockServer.Close()

	baseID := 65200
	for i := 0; i < subscriptionUsagePollBatchSize; i++ {
		channel := &model.Channel{Id: baseID + i, Name: fmt.Sprintf("codex-%d", i), Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &successServer.URL}
		require.NoError(t, model.DB.Create(channel).Error)
	}
	blockedChannel := &model.Channel{Id: baseID + subscriptionUsagePollBatchSize, Name: "blocked", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &blockServer.URL}
	require.NoError(t, model.DB.Create(blockedChannel).Error)

	done := make(chan struct{})
	go func() {
		runSubscriptionUsagePollOnce([]int{constant.ChannelTypeCodex})
		close(done)
	}()

	select {
	case <-firstBatchDone:
	case <-time.After(2 * time.Second):
		t.Fatal("第一批 Codex 渠道未完成")
	}
	select {
	case <-blockedStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("第二批 Codex 渠道未开始")
	}

	var firstSnapshot capturedRedisSet
	select {
	case firstSnapshot = <-redisSets:
	case <-time.After(300 * time.Millisecond):
		closeReleaseBlocked.Do(func() { close(releaseBlocked) })
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		t.Fatal("第一批完成后未发布 Redis 快照")
	}

	assert.Equal(t, subscriptionUsageSnapshotRedisKey, firstSnapshot.key)
	var snapshot map[string]struct {
		BottleneckPercent float64 `json:"bottleneck_percent"`
	}
	require.NoError(t, common.Unmarshal([]byte(firstSnapshot.value), &snapshot))
	firstEntry, ok := snapshot[strconv.Itoa(baseID)]
	require.True(t, ok)
	assert.Equal(t, 20.0, firstEntry.BottleneckPercent)

	closeReleaseBlocked.Do(func() { close(releaseBlocked) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Codex 用量轮询未结束")
	}

	var secondSnapshot capturedRedisSet
	select {
	case secondSnapshot = <-redisSets:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("第二批完成后未发布 Redis 快照")
	}

	assert.Equal(t, subscriptionUsageSnapshotRedisKey, secondSnapshot.key)
	require.NoError(t, common.Unmarshal([]byte(secondSnapshot.value), &snapshot))
	secondEntry, ok := snapshot[strconv.Itoa(blockedChannel.Id)]
	require.True(t, ok)
	assert.Equal(t, 30.0, secondEntry.BottleneckPercent)
}

func captureSubscriptionUsageRedisSets(t *testing.T) <-chan capturedRedisSet {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	sets := make(chan capturedRedisSet, 4)

	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	client := redis.NewClient(&redis.Options{Addr: listener.Addr().String()})
	common.RedisEnabled = true
	common.RDB = client
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
		_ = client.Close()
		_ = listener.Close()
	})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveSubscriptionUsageRedisConn(conn, sets)
		}
	}()
	return sets
}

func serveSubscriptionUsageRedisConn(conn net.Conn, sets chan<- capturedRedisSet) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		command, err := readRedisCommand(reader)
		if err != nil {
			return
		}
		if len(command) >= 3 && strings.EqualFold(command[0], "set") {
			select {
			case sets <- capturedRedisSet{key: command[1], value: command[2]}:
			default:
			}
		}
		_, _ = conn.Write([]byte("+OK\r\n"))
	}
}

func readRedisCommand(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[0] != '*' {
		return nil, fmt.Errorf("invalid redis array")
	}
	count, err := strconv.Atoi(line[1 : len(line)-2])
	if err != nil {
		return nil, err
	}
	command := make([]string, 0, count)
	for i := 0; i < count; i++ {
		bulkHeader, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(bulkHeader[1 : len(bulkHeader)-2])
		if err != nil {
			return nil, err
		}
		buf := make([]byte, length+2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		command = append(command, string(buf[:length]))
	}
	return command, nil
}

func TestStartSubscriptionUsagePollTask_NonMasterDoesNotStart(t *testing.T) {
	originalMaster := common.IsMasterNode
	originalOnce := subscriptionUsagePollOnce
	common.IsMasterNode = false
	subscriptionUsagePollOnce = new(sync.Once)
	t.Cleanup(func() {
		common.IsMasterNode = originalMaster
		subscriptionUsagePollOnce = originalOnce
	})

	StartSubscriptionUsagePollTask()

	assert.False(t, subscriptionUsagePollRunning.Load())
}

func TestStartSubscriptionUsageSyncTask_NonMasterWithoutRedisLogsWarning(t *testing.T) {
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

	StartSubscriptionUsageSyncTask()

	warning := logBuffer.String()
	require.NotEmpty(t, warning)
	assert.Contains(t, warning, "均衡")
	assert.Contains(t, warning, "触顶")
}

func TestRunSubscriptionUsagePollOnce_EmptyBaseURLFallsBackToChannelTypeDefault(t *testing.T) {
	tests := []struct {
		name    string
		baseURL *string
	}{
		{name: "nil base url", baseURL: nil},
		{name: "empty string base url", baseURL: common.GetPointer[string]("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncate(t)
			subscriptionUsagePollRunning.Store(false)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				body, err := common.Marshal(map[string]any{
					"rate_limit": map[string]any{
						"primary_window":   map[string]any{"used_percent": 41.5},
						"secondary_window": map[string]any{"used_percent": 62.75},
					},
				})
				require.NoError(t, err)
				_, _ = w.Write(body)
			}))
			defer server.Close()

			originalDefaultBaseURL := constant.ChannelBaseURLs[constant.ChannelTypeCodex]
			constant.ChannelBaseURLs[constant.ChannelTypeCodex] = server.URL
			t.Cleanup(func() { constant.ChannelBaseURLs[constant.ChannelTypeCodex] = originalDefaultBaseURL })

			channel := &model.Channel{
				Id:      65301,
				Name:    "no-base-url",
				Type:    constant.ChannelTypeCodex,
				Status:  common.ChannelStatusEnabled,
				Key:     `{"access_token":"access-token","account_id":"account-id"}`,
				BaseURL: tt.baseURL,
			}
			require.NoError(t, model.DB.Create(channel).Error)

			runSubscriptionUsagePollOnce([]int{constant.ChannelTypeCodex})

			snapshotJSON, err := model.SubscriptionChannelUsageSnapshotJSON()
			require.NoError(t, err)
			var snapshot map[string]struct {
				BottleneckPercent float64 `json:"bottleneck_percent"`
			}
			require.NoError(t, common.Unmarshal(snapshotJSON, &snapshot))
			entry, ok := snapshot[strconv.Itoa(channel.Id)]
			require.True(t, ok, "channel usage entry should be cached after polling via type default base URL")
			assert.Equal(t, 62.75, entry.BottleneckPercent)
		})
	}
}

func TestRunSubscriptionUsagePollOnce_InvalidSettingDoesNotCorruptChannelRecord(t *testing.T) {
	truncate(t)
	subscriptionUsagePollRunning.Store(false)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":10},"secondary_window":{"used_percent":15}}}`))
	}))
	defer server.Close()

	baseURL := server.URL
	invalidSetting := "{invalid"
	weight := common.GetPointer[uint](7)
	priority := common.GetPointer[int64](3)
	channel := &model.Channel{
		Id:       65302,
		Name:     "bad-setting",
		Type:     constant.ChannelTypeCodex,
		Status:   common.ChannelStatusEnabled,
		Key:      `{"access_token":"access-token","account_id":"account-id"}`,
		BaseURL:  &baseURL,
		Setting:  &invalidSetting,
		Models:   "gpt-5-codex",
		Group:    "vip",
		Priority: priority,
		Weight:   weight,
	}
	require.NoError(t, model.DB.Create(channel).Error)

	runSubscriptionUsagePollOnce([]int{constant.ChannelTypeCodex})

	var reloaded model.Channel
	require.NoError(t, model.DB.First(&reloaded, "id = ?", channel.Id).Error)
	assert.Equal(t, constant.ChannelTypeCodex, reloaded.Type)
	assert.Equal(t, "gpt-5-codex", reloaded.Models)
	assert.Equal(t, "vip", reloaded.Group)
	require.NotNil(t, reloaded.Priority)
	assert.Equal(t, int64(3), *reloaded.Priority)
	require.NotNil(t, reloaded.Weight)
	assert.Equal(t, uint(7), *reloaded.Weight)
	require.NotNil(t, reloaded.Setting)
	assert.Equal(t, invalidSetting, *reloaded.Setting)
}

func TestRunSubscriptionUsagePollOnce_BatchPollsChannelsConcurrently(t *testing.T) {
	truncate(t)
	subscriptionUsagePollRunning.Store(false)

	aStarted := make(chan struct{})
	releaseA := make(chan struct{})
	var closeReleaseA sync.Once
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(aStarted)
		<-releaseA
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":12},"secondary_window":{"used_percent":20}}}`))
	}))
	t.Cleanup(serverA.Close)
	t.Cleanup(func() { closeReleaseA.Do(func() { close(releaseA) }) })

	bCalled := make(chan struct{})
	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(bCalled)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":12},"secondary_window":{"used_percent":20}}}`))
	}))
	t.Cleanup(serverB.Close)

	channelA := &model.Channel{Id: 65401, Name: "a", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &serverA.URL}
	channelB := &model.Channel{Id: 65402, Name: "b", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Key: `{"access_token":"access-token","account_id":"account-id"}`, BaseURL: &serverB.URL}
	require.NoError(t, model.DB.Create(channelA).Error)
	require.NoError(t, model.DB.Create(channelB).Error)

	done := make(chan struct{})
	go func() {
		runSubscriptionUsagePollOnce([]int{constant.ChannelTypeCodex})
		close(done)
	}()

	select {
	case <-aStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("渠道 A 未被轮询到")
	}

	select {
	case <-bCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("同批渠道被串行阻塞，未并发轮询")
	}

	closeReleaseA.Do(func() { close(releaseA) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Codex 用量轮询未结束")
	}
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

func TestSyncSubscriptionUsageSnapshotOnce_ReplacesLocalCacheWholesale(t *testing.T) {
	require.NoError(t, model.CacheLoadSubscriptionChannelUsageSnapshotJSON([]byte(`{}`)))
	model.CacheSetSubscriptionChannelUsage(71002, 97)
	payload, err := model.SubscriptionChannelUsageSnapshotJSON()
	require.NoError(t, err)

	require.NoError(t, model.CacheLoadSubscriptionChannelUsageSnapshotJSON([]byte(`{}`)))
	model.CacheSetSubscriptionChannelUsage(71001, 40)

	startSubscriptionUsageFakeRedisGet(t, string(payload))
	syncSubscriptionUsageSnapshotOnce()

	assert.True(t, model.CacheIsSubscriptionChannelSaturated(71002))
	snapshotJSON, err := model.SubscriptionChannelUsageSnapshotJSON()
	require.NoError(t, err)
	var snapshot map[string]struct {
		BottleneckPercent float64 `json:"bottleneck_percent"`
	}
	require.NoError(t, common.Unmarshal(snapshotJSON, &snapshot))
	assert.Len(t, snapshot, 1)
	_, hasOld := snapshot["71001"]
	assert.False(t, hasOld)
}

func TestSyncSubscriptionUsageSnapshotOnce_ReadFailureKeepsLocalCache(t *testing.T) {
	require.NoError(t, model.CacheLoadSubscriptionChannelUsageSnapshotJSON([]byte(`{}`)))
	model.CacheSetSubscriptionChannelUsage(71003, 96)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	client := redis.NewClient(&redis.Options{Addr: addr})
	common.RedisEnabled = true
	common.RDB = client
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
		_ = client.Close()
	})

	syncSubscriptionUsageSnapshotOnce()

	assert.True(t, model.CacheIsSubscriptionChannelSaturated(71003))
}

func startSubscriptionUsageFakeRedisGet(t *testing.T, payload string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	client := redis.NewClient(&redis.Options{Addr: listener.Addr().String()})
	common.RedisEnabled = true
	common.RDB = client
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
		_ = client.Close()
		_ = listener.Close()
	})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveSubscriptionUsageRedisGetConn(conn, payload)
		}
	}()
}

func serveSubscriptionUsageRedisGetConn(conn net.Conn, payload string) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		command, err := readRedisCommand(reader)
		if err != nil {
			return
		}
		if len(command) >= 2 && strings.EqualFold(command[0], "get") {
			_, _ = fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(payload), payload)
			continue
		}
		_, _ = conn.Write([]byte("+OK\r\n"))
	}
}
