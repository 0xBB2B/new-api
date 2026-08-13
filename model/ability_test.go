package model

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetChannel_SaturatedCodexIsExcludedFromDBCandidates(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57001, "gpt-5.4", "default", constant.ChannelTypeCodex, 0, 1_000_000)
	insertChannelSelectionCandidate(t, 57002, "gpt-5.4", "default", constant.ChannelTypeOpenAI, 0, 0)
	CacheSetSubscriptionChannelUsage(57001, 95)
	rand.Seed(1)

	channel, err := GetChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 57002, channel.Id)
}

func TestGetChannel_OnlySaturatedCodexDBCandidateReturnsNil(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57003, "gpt-5.4", "default", constant.ChannelTypeCodex, 0, 100)
	CacheSetSubscriptionChannelUsage(57003, 95)

	channel, err := GetChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestGetChannel_SaturatedHighPriorityCodexFallsBackToLowerPriority(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57004, "gpt-5.4", "default", constant.ChannelTypeCodex, 10, 100)
	insertChannelSelectionCandidate(t, 57005, "gpt-5.4", "default", constant.ChannelTypeOpenAI, 1, 100)
	CacheSetSubscriptionChannelUsage(57004, 95)

	channel, err := GetChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 57005, channel.Id)

	var codex Channel
	require.NoError(t, DB.First(&codex, "id = ?", 57004).Error)
	assert.Equal(t, common.ChannelStatusEnabled, codex.Status)
}

func TestGetChannel_AllPriorityLayersSaturatedReturnsNil(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57006, "gpt-5.4", "default", constant.ChannelTypeCodex, 10, 100)
	insertChannelSelectionCandidate(t, 57007, "gpt-5.4", "default", constant.ChannelTypeCodex, 1, 100)
	CacheSetSubscriptionChannelUsage(57006, 95)
	CacheSetSubscriptionChannelUsage(57007, 95)

	channel, err := GetChannel("default", "gpt-5.4", 0, "")

	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestGetChannel_RetryBeyondPriorityLayerCountClampsToLowestLayer(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
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
	resetSubscriptionChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	common.MemoryCacheEnabled = false
	insertChannelSelectionCandidate(t, 57009, "gpt-5.4", "default", constant.ChannelTypeOpenAI, 10, 100)
	insertChannelSelectionCandidate(t, 57010, "gpt-5.4", "default", constant.ChannelTypeCodex, 1, 100)
	CacheSetSubscriptionChannelUsage(57010, 95)

	channel, err := GetChannel("default", "gpt-5.4", 1, "")

	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 57009, channel.Id)
}

func TestGetChannel_NullPriorityDefaultsToZeroInDBSelection(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
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

func TestGetChannel_ChannelTypeLookupErrorReturnsNoChannel(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	clearPreferredOwnerTables(t)
	insertChannelSelectionCandidate(t, 57013, "gpt-5.4", "default", constant.ChannelTypeCodex, 0, 100)
	CacheSetSubscriptionChannelUsage(57013, 95)

	callbackName := "test:fail_first_channel_type_lookup"
	channelQueryCount := 0
	require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "Channel" {
			return
		}
		channelQueryCount++
		if channelQueryCount == 1 {
			tx.AddError(errors.New("channel type lookup failed"))
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, DB.Callback().Query().Remove(callbackName))
	})

	channel, err := GetChannel("default", "gpt-5.4", 0, "")

	require.Error(t, err)
	assert.Nil(t, channel)
	assert.Equal(t, 1, channelQueryCount)
}

func TestFilterSaturatedSubscriptionAbilities_ClaudeSubscriptionSaturatedIsExcluded(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	CacheSetSubscriptionChannelUsage(59001, 95)
	abilities := []Ability{{ChannelId: 59001}, {ChannelId: 59002}}
	channelTypes := map[int]int{59001: constant.ChannelTypeClaudeSubscription, 59002: constant.ChannelTypeClaudeSubscription}

	filtered := filterSaturatedSubscriptionAbilities(abilities, channelTypes)

	require.Len(t, filtered, 1)
	assert.Equal(t, 59002, filtered[0].ChannelId)
}

func TestFilterSaturatedSubscriptionAbilities_ClaudeSubscriptionNotSaturatedIsKept(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	CacheSetSubscriptionChannelUsage(59003, 94.9)
	abilities := []Ability{{ChannelId: 59003}}
	channelTypes := map[int]int{59003: constant.ChannelTypeClaudeSubscription}

	filtered := filterSaturatedSubscriptionAbilities(abilities, channelTypes)

	require.Len(t, filtered, 1)
	assert.Equal(t, 59003, filtered[0].ChannelId)
}

func TestFilterSaturatedSubscriptionAbilities_NonSubscriptionTypeSaturatedCacheIsKept(t *testing.T) {
	resetSubscriptionChannelUsageCache(t)
	CacheSetSubscriptionChannelUsage(59004, 95)
	abilities := []Ability{{ChannelId: 59004}}
	channelTypes := map[int]int{59004: constant.ChannelTypeOpenAI}

	filtered := filterSaturatedSubscriptionAbilities(abilities, channelTypes)

	require.Len(t, filtered, 1)
	assert.Equal(t, 59004, filtered[0].ChannelId)
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
