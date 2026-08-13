package model

import (
	"math/rand"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRandomSatisfiedChannel_SaturatedCodexIsExcludedFromMemoryCandidates(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroup2Model2Channels := group2model2channels
	originalChannelsIDM := channelsIDM
	originalChannel2AdvancedCustomConfig := channel2advancedCustomConfig
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		group2model2channels = originalGroup2Model2Channels
		channelsIDM = originalChannelsIDM
		channel2advancedCustomConfig = originalChannel2AdvancedCustomConfig
	})

	common.MemoryCacheEnabled = true
	priority := int64(0)
	codexWeight := uint(1_000_000)
	openAIWeight := uint(0)
	group2model2channels = map[string]map[string][]int{
		"default": {"gpt-5.4": {56001, 56002}},
	}
	channelsIDM = map[int]*Channel{
		56001: {Id: 56001, Type: constant.ChannelTypeCodex, Name: "codex", Weight: &codexWeight, Priority: &priority, Status: common.ChannelStatusEnabled},
		56002: {Id: 56002, Type: constant.ChannelTypeOpenAI, Name: "openai", Weight: &openAIWeight, Priority: &priority, Status: common.ChannelStatusEnabled},
	}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{}
	CacheSetSubscriptionChannelUsage(56001, 95)
	rand.Seed(1)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 56002, channel.Id)
}

func TestGetRandomSatisfiedChannel_RecentlySaturatedStaleCodexIsExcluded(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroup2Model2Channels := group2model2channels
	originalChannelsIDM := channelsIDM
	originalChannel2AdvancedCustomConfig := channel2advancedCustomConfig
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		group2model2channels = originalGroup2Model2Channels
		channelsIDM = originalChannelsIDM
		channel2advancedCustomConfig = originalChannel2AdvancedCustomConfig
	})

	common.MemoryCacheEnabled = true
	priority := int64(0)
	codexWeight := uint(1_000_000)
	openAIWeight := uint(0)
	group2model2channels = map[string]map[string][]int{
		"default": {"gpt-5.4": {56010, 56011}},
	}
	channelsIDM = map[int]*Channel{
		56010: {Id: 56010, Type: constant.ChannelTypeCodex, Name: "codex", Weight: &codexWeight, Priority: &priority, Status: common.ChannelStatusEnabled},
		56011: {Id: 56011, Type: constant.ChannelTypeOpenAI, Name: "openai", Weight: &openAIWeight, Priority: &priority, Status: common.ChannelStatusEnabled},
	}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{}
	CacheSetSubscriptionChannelUsage(56010, 96)
	subscriptionChannelUsageCacheLock.Lock()
	entry := subscriptionChannelUsageCache[56010]
	entry.refreshedAt = time.Now().Add(-6 * time.Minute)
	subscriptionChannelUsageCache[56010] = entry
	subscriptionChannelUsageCacheLock.Unlock()
	rand.Seed(1)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 56011, channel.Id)

	subscriptionChannelUsageCacheLock.Lock()
	entry = subscriptionChannelUsageCache[56010]
	entry.refreshedAt = time.Now().Add(-11 * time.Minute)
	subscriptionChannelUsageCache[56010] = entry
	subscriptionChannelUsageCacheLock.Unlock()
	rand.Seed(1)

	channel, err = GetRandomSatisfiedChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 56010, channel.Id)
}

func TestGetRandomSatisfiedChannel_OnlySaturatedCodexMemoryCandidateReturnsNil(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroup2Model2Channels := group2model2channels
	originalChannelsIDM := channelsIDM
	originalChannel2AdvancedCustomConfig := channel2advancedCustomConfig
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		group2model2channels = originalGroup2Model2Channels
		channelsIDM = originalChannelsIDM
		channel2advancedCustomConfig = originalChannel2AdvancedCustomConfig
	})

	common.MemoryCacheEnabled = true
	priority := int64(0)
	weight := uint(100)
	group2model2channels = map[string]map[string][]int{
		"default": {"gpt-5.4": {56003}},
	}
	channelsIDM = map[int]*Channel{
		56003: {Id: 56003, Type: constant.ChannelTypeCodex, Name: "codex-only", Weight: &weight, Priority: &priority, Status: common.ChannelStatusEnabled},
	}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{}
	CacheSetSubscriptionChannelUsage(56003, 95)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestGetRandomSatisfiedChannel_SaturatedHighPriorityCodexFallsBackToLowerPriority(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroup2Model2Channels := group2model2channels
	originalChannelsIDM := channelsIDM
	originalChannel2AdvancedCustomConfig := channel2advancedCustomConfig
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		group2model2channels = originalGroup2Model2Channels
		channelsIDM = originalChannelsIDM
		channel2advancedCustomConfig = originalChannel2AdvancedCustomConfig
	})

	common.MemoryCacheEnabled = true
	highPriority := int64(10)
	lowPriority := int64(1)
	weight := uint(100)
	group2model2channels = map[string]map[string][]int{
		"default": {"gpt-5.4": {56004, 56005}},
	}
	channelsIDM = map[int]*Channel{
		56004: {Id: 56004, Type: constant.ChannelTypeCodex, Name: "codex-high", Weight: &weight, Priority: &highPriority, Status: common.ChannelStatusEnabled},
		56005: {Id: 56005, Type: constant.ChannelTypeOpenAI, Name: "openai-low", Weight: &weight, Priority: &lowPriority, Status: common.ChannelStatusEnabled},
	}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{}
	CacheSetSubscriptionChannelUsage(56004, 95)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 56005, channel.Id)
	assert.Equal(t, common.ChannelStatusEnabled, channelsIDM[56004].Status)
}

func TestGetRandomSatisfiedChannel_AllPriorityLayersSaturatedReturnsNil(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroup2Model2Channels := group2model2channels
	originalChannelsIDM := channelsIDM
	originalChannel2AdvancedCustomConfig := channel2advancedCustomConfig
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		group2model2channels = originalGroup2Model2Channels
		channelsIDM = originalChannelsIDM
		channel2advancedCustomConfig = originalChannel2AdvancedCustomConfig
	})

	common.MemoryCacheEnabled = true
	highPriority := int64(10)
	lowPriority := int64(1)
	weight := uint(100)
	group2model2channels = map[string]map[string][]int{
		"default": {"gpt-5.4": {56006, 56007}},
	}
	channelsIDM = map[int]*Channel{
		56006: {Id: 56006, Type: constant.ChannelTypeCodex, Name: "codex-high", Weight: &weight, Priority: &highPriority, Status: common.ChannelStatusEnabled},
		56007: {Id: 56007, Type: constant.ChannelTypeCodex, Name: "codex-low", Weight: &weight, Priority: &lowPriority, Status: common.ChannelStatusEnabled},
	}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{}
	CacheSetSubscriptionChannelUsage(56006, 95)
	CacheSetSubscriptionChannelUsage(56007, 95)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestGetRandomSatisfiedChannel_RetryClampsToSurvivingLayerWhenLowerLayerCodexSaturated(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroup2Model2Channels := group2model2channels
	originalChannelsIDM := channelsIDM
	originalChannel2AdvancedCustomConfig := channel2advancedCustomConfig
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		group2model2channels = originalGroup2Model2Channels
		channelsIDM = originalChannelsIDM
		channel2advancedCustomConfig = originalChannel2AdvancedCustomConfig
	})

	common.MemoryCacheEnabled = true
	highPriority := int64(10)
	lowPriority := int64(1)
	weight := uint(100)
	group2model2channels = map[string]map[string][]int{
		"default": {"gpt-5.4": {56008, 56009}},
	}
	channelsIDM = map[int]*Channel{
		56008: {Id: 56008, Type: constant.ChannelTypeOpenAI, Name: "openai-high", Weight: &weight, Priority: &highPriority, Status: common.ChannelStatusEnabled},
		56009: {Id: 56009, Type: constant.ChannelTypeCodex, Name: "codex-low", Weight: &weight, Priority: &lowPriority, Status: common.ChannelStatusEnabled},
	}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{}
	CacheSetSubscriptionChannelUsage(56009, 95)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 1, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 56008, channel.Id)
}

func TestFilterSaturatedSubscriptionChannels_ClaudeSubscriptionSaturatedIsExcluded(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	originalChannelsIDM := channelsIDM
	t.Cleanup(func() {
		channelsIDM = originalChannelsIDM
	})
	weight := uint(100)
	channelsIDM = map[int]*Channel{
		56020: {Id: 56020, Type: constant.ChannelTypeClaudeSubscription, Weight: &weight},
		56021: {Id: 56021, Type: constant.ChannelTypeClaudeSubscription, Weight: &weight},
	}
	CacheSetSubscriptionChannelUsage(56020, 95)

	filtered := filterSaturatedSubscriptionChannels([]int{56020, 56021})

	require.Equal(t, []int{56021}, filtered)
}

func TestSubscriptionEffectiveWeight(t *testing.T) {
	tests := []struct {
		name        string
		channelID   int
		channelType int
		weight      int
		setupCache  func(t *testing.T)
		want        int
	}{
		{
			name:        "codex with fresh 60 percent bottleneck scales weight",
			channelID:   57020,
			channelType: constant.ChannelTypeCodex,
			weight:      100,
			setupCache: func(t *testing.T) {
				CacheSetSubscriptionChannelUsage(57020, 60)
			},
			want: 40,
		},
		{
			name:        "claude subscription with fresh 60 percent bottleneck scales weight",
			channelID:   61020,
			channelType: constant.ChannelTypeClaudeSubscription,
			weight:      100,
			setupCache: func(t *testing.T) {
				CacheSetSubscriptionChannelUsage(61020, 60)
			},
			want: 40,
		},
		{
			name:        "non-subscription channel type returns original weight",
			channelID:   58020,
			channelType: constant.ChannelTypeOpenAI,
			weight:      100,
			setupCache: func(t *testing.T) {
				CacheSetSubscriptionChannelUsage(58020, 60)
			},
			want: 100,
		},
		{
			name:        "missing cache entry falls back to original weight",
			channelID:   61021,
			channelType: constant.ChannelTypeClaudeSubscription,
			weight:      100,
			setupCache:  func(t *testing.T) {},
			want:        100,
		},
		{
			name:        "stale entry beyond ten minutes falls back to original weight",
			channelID:   61023,
			channelType: constant.ChannelTypeClaudeSubscription,
			weight:      100,
			setupCache: func(t *testing.T) {
				CacheSetSubscriptionChannelUsage(61023, 60)
				ageSubscriptionChannelUsageEntry(t, 61023, -11*time.Minute)
			},
			want: 100,
		},
		{
			name:        "entry refreshed six minutes ago still scales weight",
			channelID:   61024,
			channelType: constant.ChannelTypeClaudeSubscription,
			weight:      100,
			setupCache: func(t *testing.T) {
				CacheSetSubscriptionChannelUsage(61024, 90)
				ageSubscriptionChannelUsageEntry(t, 61024, -6*time.Minute)
			},
			want: 10,
		},
		{
			name:        "scaled weight floors to minimum of 1",
			channelID:   61022,
			channelType: constant.ChannelTypeClaudeSubscription,
			weight:      2,
			setupCache: func(t *testing.T) {
				CacheSetSubscriptionChannelUsage(61022, 60)
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetSubscriptionChannelUsageCache(t)
			tt.setupCache(t)

			got := subscriptionEffectiveWeight(tt.channelID, tt.channelType, tt.weight)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSubscriptionEffectiveWeight_CodexAndClaudeSubscriptionMatchUnderSameCacheState(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	CacheSetSubscriptionChannelUsage(57030, 42)
	CacheSetSubscriptionChannelUsage(61030, 42)

	codexWeight := subscriptionEffectiveWeight(57030, constant.ChannelTypeCodex, 250)
	claudeWeight := subscriptionEffectiveWeight(61030, constant.ChannelTypeClaudeSubscription, 250)

	assert.Equal(t, codexWeight, claudeWeight)
}
