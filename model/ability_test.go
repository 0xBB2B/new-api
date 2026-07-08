package model

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetChannel_SaturatedCodexIsExcludedFromDBCandidates(t *testing.T) {
	resetCodexChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57001, "gpt-5.4", "default", constant.ChannelTypeCodex, 0, 1_000_000)
	insertChannelSelectionCandidate(t, 57002, "gpt-5.4", "default", constant.ChannelTypeOpenAI, 0, 0)
	CacheSetCodexChannelUsage(57001, 95, 10)
	rand.Seed(1)

	channel, err := GetChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 57002, channel.Id)
}

func TestGetChannel_OnlySaturatedCodexDBCandidateReturnsNil(t *testing.T) {
	resetCodexChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57003, "gpt-5.4", "default", constant.ChannelTypeCodex, 0, 100)
	CacheSetCodexChannelUsage(57003, 95, 10)

	channel, err := GetChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestGetChannel_SaturatedHighPriorityCodexFallsBackToLowerPriority(t *testing.T) {
	resetCodexChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57004, "gpt-5.4", "default", constant.ChannelTypeCodex, 10, 100)
	insertChannelSelectionCandidate(t, 57005, "gpt-5.4", "default", constant.ChannelTypeOpenAI, 1, 100)
	CacheSetCodexChannelUsage(57004, 95, 10)

	channel, err := GetChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 57005, channel.Id)

	var codex Channel
	require.NoError(t, DB.First(&codex, "id = ?", 57004).Error)
	assert.Equal(t, common.ChannelStatusEnabled, codex.Status)
}

func TestGetChannel_AllPriorityLayersSaturatedReturnsNil(t *testing.T) {
	resetCodexChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57006, "gpt-5.4", "default", constant.ChannelTypeCodex, 10, 100)
	insertChannelSelectionCandidate(t, 57007, "gpt-5.4", "default", constant.ChannelTypeCodex, 1, 100)
	CacheSetCodexChannelUsage(57006, 95, 10)
	CacheSetCodexChannelUsage(57007, 10, 95)

	channel, err := GetChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestGetChannel_RetryBeyondPriorityLayerCountClampsToLowestLayer(t *testing.T) {
	resetCodexChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57008, "gpt-5.4", "default", constant.ChannelTypeOpenAI, 0, 100)

	channel, err := GetChannel("default", "gpt-5.4", 5, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 57008, channel.Id)
}

func TestGetChannel_RetryClampsToSurvivingLayerWhenLowerLayerCodexSaturated(t *testing.T) {
	resetCodexChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57009, "gpt-5.4", "default", constant.ChannelTypeOpenAI, 10, 100)
	insertChannelSelectionCandidate(t, 57010, "gpt-5.4", "default", constant.ChannelTypeCodex, 1, 100)
	CacheSetCodexChannelUsage(57010, 95, 10)

	channel, err := GetChannel("default", "gpt-5.4", 1, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 57009, channel.Id)
}

func TestGetChannel_NullPriorityDefaultsToZeroInDBSelection(t *testing.T) {
	resetCodexChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57011, "gpt-5.4", "default", constant.ChannelTypeOpenAI, 0, 100)
	insertChannelSelectionCandidate(t, 57012, "gpt-5.4", "default", constant.ChannelTypeOpenAI, 1, 100)
	require.NoError(t, DB.Model(&Ability{}).
		Where("channel_id = ?", 57011).
		Updates(map[string]any{"priority": nil}).Error)

	channel, err := GetChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 57012, channel.Id)

	channel, err = GetChannel("default", "gpt-5.4", 1, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 57011, channel.Id)
}

func insertChannelSelectionCandidate(t *testing.T, channelID int, modelName string, group string, channelType int, priority int64, weight uint) {
	t.Helper()
	require.NoError(t, DB.Create(&Channel{
		Id:     channelID,
		Type:   channelType,
		Key:    fmt.Sprintf("key-%d", channelID),
		Status: common.ChannelStatusEnabled,
		Name:   fmt.Sprintf("channel-%d", channelID),
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channelID,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}
