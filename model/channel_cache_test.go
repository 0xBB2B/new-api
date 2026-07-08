package model

import (
	"math/rand"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRandomSatisfiedChannel_SaturatedCodexIsExcludedFromMemoryCandidates(t *testing.T) {
	resetCodexChannelUsageCache(t)
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
	CacheSetCodexChannelUsage(56001, 95, 10)
	rand.Seed(1)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 56002, channel.Id)
}

func TestGetRandomSatisfiedChannel_OnlySaturatedCodexMemoryCandidateReturnsNil(t *testing.T) {
	resetCodexChannelUsageCache(t)
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
	CacheSetCodexChannelUsage(56003, 95, 10)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestGetRandomSatisfiedChannel_SaturatedHighPriorityCodexFallsBackToLowerPriority(t *testing.T) {
	resetCodexChannelUsageCache(t)
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
	CacheSetCodexChannelUsage(56004, 95, 10)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 56005, channel.Id)
	assert.Equal(t, common.ChannelStatusEnabled, channelsIDM[56004].Status)
}

func TestGetRandomSatisfiedChannel_AllPriorityLayersSaturatedReturnsNil(t *testing.T) {
	resetCodexChannelUsageCache(t)
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
	CacheSetCodexChannelUsage(56006, 95, 10)
	CacheSetCodexChannelUsage(56007, 10, 95)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestGetRandomSatisfiedChannel_RetryClampsToSurvivingLayerWhenLowerLayerCodexSaturated(t *testing.T) {
	resetCodexChannelUsageCache(t)
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
	CacheSetCodexChannelUsage(56009, 95, 10)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-5.4", 1, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 56008, channel.Id)
}
