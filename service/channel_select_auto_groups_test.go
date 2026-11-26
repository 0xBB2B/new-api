package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelSelectAutoGroupsTest(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRetryTimes := common.RetryTimes
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalMaxTokenAutoGroups := setting.GetMaxTokenAutoGroups()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	common.RetryTimes = 0

	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`[]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
	require.NoError(t, setting.UpdateMaxTokenAutoGroups("2"))

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RetryTimes = originalRetryTimes
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, setting.UpdateMaxTokenAutoGroups(fmt.Sprintf("%d", originalMaxTokenAutoGroups)))

		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	return db
}

func createChannelSelectAutoGroupsChannel(t *testing.T, db *gorm.DB, id int, group, modelName string) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("channel-%d", id),
		Weight:   &weight,
		Models:   modelName,
		Group:    group,
		Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func TestCacheGetRandomSatisfiedChannelUsesTokenAutoGroupsWhenGlobalAutoIsEmpty(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "auto-groups-runtime-model"
	createChannelSelectAutoGroupsChannel(t, db, 2101, "vip", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2102, "default", modelName)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
	common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, true)

	retry := 0
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "auto",
		ModelName:   modelName,
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	first, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, 2101, first.Id)
	assert.Equal(t, "vip", selectedGroup)
	assert.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
	assert.Empty(t, setting.GetAutoGroups(), "the selection must not depend on the global Auto list")

	param.IncreaseRetry()
	second, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, 2102, second.Id)
	assert.Equal(t, "default", selectedGroup)
	assert.Equal(t, "default", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
}

func createFailoverChannel(t *testing.T, db *gorm.DB, id, channelType int, group, modelName string, priority int64) {
	t.Helper()
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id:       id,
		Type:     channelType,
		Key:      fmt.Sprintf("key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("channel-%d", id),
		Weight:   &weight,
		Models:   modelName,
		Group:    group,
		Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func newFailoverContext(autoGroups []string, crossGroup bool, dispatchedChannelID int) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	if autoGroups != nil {
		common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, autoGroups)
		common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, crossGroup)
		common.SetContextKey(ctx, constant.ContextKeyAutoGroupIndex, 0)
	}
	common.SetContextKey(ctx, constant.ContextKeyChannelId, dispatchedChannelID)
	return ctx
}

func TestFailoverLedgerOnlyExistsForStartedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Nil(t, GetFailoverLedger(ctx), "async tasks never start a ledger")

	common.SetContextKey(ctx, constant.ContextKeyChannelId, 3101)
	StartFailoverLedger(ctx)
	assert.NotNil(t, GetFailoverLedger(ctx))
}

func TestPickFailoverChannelSkipsDispatchedChannel(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "failover-model"
	createFailoverChannel(t, db, 3201, constant.ChannelTypeClaudeSubscription, "default", modelName, 0)
	createFailoverChannel(t, db, 3202, constant.ChannelTypeCodex, "default", modelName, 0)
	createFailoverChannel(t, db, 3203, constant.ChannelTypeOpenAI, "default", modelName, 10)
	model.InitChannelCache()

	ctx := newFailoverContext(nil, false, 3201)
	StartFailoverLedger(ctx)
	retry := 0
	param := &RetryParam{Ctx: ctx, TokenGroup: "default", ModelName: modelName, RequestPath: "/v1/chat/completions", Retry: &retry}

	for range 20 {
		channel, group, exhausted, err := PickFailoverChannel(param)
		require.NoError(t, err)
		require.False(t, exhausted)
		require.NotNil(t, channel)
		assert.Equal(t, 3202, channel.Id, "the dispatched subscription channel is tried, the other subscription channel comes before the higher-priority API channel")
		assert.Equal(t, "default", group)
	}
}

func TestPickFailoverChannelExhaustedIsNotAnError(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "failover-model"
	createFailoverChannel(t, db, 3301, constant.ChannelTypeOpenAI, "default", modelName, 0)
	model.InitChannelCache()

	ctx := newFailoverContext(nil, false, 3301)
	StartFailoverLedger(ctx)
	retry := 0
	param := &RetryParam{Ctx: ctx, TokenGroup: "default", ModelName: modelName, RequestPath: "/v1/chat/completions", Retry: &retry}

	channel, _, exhausted, err := PickFailoverChannel(param)
	require.NoError(t, err)
	assert.Nil(t, channel)
	assert.True(t, exhausted)
	assert.False(t, HasFailoverChannel(param))
}

func TestPickFailoverChannelRespectsRequestFilters(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "failover-model"
	createFailoverChannel(t, db, 3401, constant.ChannelTypeClaudeSubscription, "default", modelName, 0)
	createFailoverChannel(t, db, 3402, constant.ChannelTypeCodex, "default", modelName, 0)
	createFailoverChannel(t, db, 3403, constant.ChannelTypeOpenAI, "default", modelName, 10)
	model.InitChannelCache()

	ctx := newFailoverContext(nil, false, 3401)
	GetChannelConstraints(ctx).AddFilter(dto.ChannelFilter{
		Kind: dto.FilterTaskPluginIdentity, TaskPluginKey: "any", TaskPluginChannelTypes: []int{constant.ChannelTypeOpenAI},
	})
	StartFailoverLedger(ctx)
	retry := 0
	param := &RetryParam{Ctx: ctx, TokenGroup: "default", ModelName: modelName, RequestPath: "/v1/chat/completions", Retry: &retry}

	for range 20 {
		channel, _, exhausted, err := PickFailoverChannel(param)
		require.NoError(t, err)
		require.False(t, exhausted)
		require.NotNil(t, channel)
		assert.Equal(t, 3403, channel.Id)
	}
}

func TestHasFailoverChannelMatchesPickAndLeavesAutoGroupState(t *testing.T) {
	const modelName = "failover-model"
	for _, tc := range []struct {
		name       string
		crossGroup bool
		wantID     int
	}{
		{"cross group retry on moves to the next group", true, 3502},
		{"cross group retry off is exhausted", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupChannelSelectAutoGroupsTest(t)
			require.NoError(t, setting.UpdateMaxTokenAutoGroups("5"))
			createFailoverChannel(t, db, 3501, constant.ChannelTypeClaudeSubscription, "default", modelName, 0)
			createFailoverChannel(t, db, 3502, constant.ChannelTypeCodex, "vip", modelName, 0)
			model.InitChannelCache()

			ctx := newFailoverContext([]string{"default", "vip"}, tc.crossGroup, 3501)
			StartFailoverLedger(ctx)
			retry := 0
			param := &RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: modelName, RequestPath: "/v1/chat/completions", Retry: &retry}

			indexBefore, _ := common.GetContextKey(ctx, constant.ContextKeyAutoGroupIndex)
			groupBefore := common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup)
			has := HasFailoverChannel(param)
			indexAfter, _ := common.GetContextKey(ctx, constant.ContextKeyAutoGroupIndex)
			assert.Equal(t, indexBefore, indexAfter)
			assert.Equal(t, groupBefore, common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))

			channel, _, exhausted, err := PickFailoverChannel(param)
			require.NoError(t, err)
			assert.Equal(t, has, channel != nil)
			assert.Equal(t, !has, exhausted)
			if tc.wantID == 0 {
				assert.Nil(t, channel)
				return
			}
			require.NotNil(t, channel)
			assert.Equal(t, tc.wantID, channel.Id)
			assert.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
			assert.Equal(t, 1, common.GetContextKeyInt(ctx, constant.ContextKeyAutoGroupIndex))
		})
	}
}

func requestFailoverContext(autoGroups []string, crossGroup bool) *gin.Context {
	ctx := newFailoverContext(autoGroups, crossGroup, 0)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return ctx
}

func TestFailoverAfterRealFirstSelectionStaysInGroupBeforeCrossing(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "failover-model"
	createFailoverChannel(t, db, 5001, constant.ChannelTypeClaudeSubscription, "default", modelName, 0)
	createFailoverChannel(t, db, 5002, constant.ChannelTypeOpenAI, "default", modelName, 10)
	createFailoverChannel(t, db, 5003, constant.ChannelTypeCodex, "vip", modelName, 0)
	model.InitChannelCache()
	require.Zero(t, common.RetryTimes)

	ctx := requestFailoverContext([]string{"default", "vip"}, true)
	first, group, selectionErr := SelectChannelForRequest(ctx, modelName, &RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: modelName})
	require.Nil(t, selectionErr)
	require.NotNil(t, first)
	require.Equal(t, 5001, first.Id)
	require.Equal(t, "default", group)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, first.Id)
	ledger := StartFailoverLedger(ctx)

	param := &RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: modelName, RequestPath: "/v1/chat/completions"}
	for _, want := range []struct {
		id    int
		group string
	}{{5002, "default"}, {5003, "vip"}} {
		channel, pickedGroup, exhausted, err := PickFailoverChannel(param)
		require.NoError(t, err)
		require.False(t, exhausted)
		require.NotNil(t, channel)
		assert.Equal(t, want.id, channel.Id)
		assert.Equal(t, want.group, pickedGroup)
		assert.Equal(t, want.group, common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
		ledger.Record(channel.Id, 0)
	}
	channel, _, exhausted, err := PickFailoverChannel(param)
	require.NoError(t, err)
	assert.Nil(t, channel)
	assert.True(t, exhausted)
}

func TestFailoverAfterAffinityHitOnlyTriesTheAffinityGroupWithoutCrossGroupRetry(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "failover-affinity-model"
	createFailoverChannel(t, db, 5101, constant.ChannelTypeClaudeSubscription, "default", modelName, 0)
	createFailoverChannel(t, db, 5102, constant.ChannelTypeOpenAI, "default", modelName, 10)
	createFailoverChannel(t, db, 5103, constant.ChannelTypeCodex, "vip", modelName, 0)
	createFailoverChannel(t, db, 5104, constant.ChannelTypeOpenAI, "vip", modelName, 10)
	model.InitChannelCache()

	affinity := operation_setting.GetChannelAffinitySetting()
	previousAffinity := *affinity
	t.Cleanup(func() { *affinity = previousAffinity })
	*affinity = operation_setting.ChannelAffinitySetting{
		Enabled: true, DefaultTTLSeconds: 60, MaxEntries: 100,
		Rules: []operation_setting.ChannelAffinityRule{{
			Name: t.Name(), ModelRegex: []string{"^" + modelName + "$"}, SessionMode: "prefer",
			KeySources: []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "X-Session"}},
		}},
	}

	ctx := requestFailoverContext([]string{"default", "vip"}, false)
	ctx.Request.Header.Set("X-Session", t.Name())
	_, found := GetPreferredChannelByAffinity(ctx, modelName, "auto")
	require.False(t, found)
	RecordChannelAffinity(ctx, 5103)
	t.Cleanup(func() { ClearCurrentChannelAffinityCache(ctx) })

	first, group, selectionErr := SelectChannelForRequest(ctx, modelName, &RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: modelName})
	require.Nil(t, selectionErr)
	require.NotNil(t, first)
	require.Equal(t, 5103, first.Id)
	require.Equal(t, "vip", group)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, first.Id)
	ledger := StartFailoverLedger(ctx)

	param := &RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: modelName, RequestPath: "/v1/chat/completions"}
	channel, pickedGroup, exhausted, err := PickFailoverChannel(param)
	require.NoError(t, err)
	require.False(t, exhausted)
	require.NotNil(t, channel)
	assert.Equal(t, 5104, channel.Id)
	assert.Equal(t, "vip", pickedGroup)
	ledger.Record(channel.Id, 0)

	channel, _, exhausted, err = PickFailoverChannel(param)
	require.NoError(t, err)
	assert.Nil(t, channel, "the default group is never tried after the affinity group")
	assert.True(t, exhausted)
}

func TestFailoverSkipsChannelDisabledMidRequest(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "failover-model"
	createFailoverChannel(t, db, 5201, constant.ChannelTypeClaudeSubscription, "default", modelName, 0)
	createFailoverChannel(t, db, 5202, constant.ChannelTypeOpenAI, "default", modelName, 10)
	createFailoverChannel(t, db, 5203, constant.ChannelTypeOpenAI, "default", modelName, 5)
	model.InitChannelCache()

	ctx := requestFailoverContext(nil, false)
	first, _, selectionErr := SelectChannelForRequest(ctx, modelName, &RetryParam{Ctx: ctx, TokenGroup: "default", ModelName: modelName})
	require.Nil(t, selectionErr)
	require.Equal(t, 5201, first.Id)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, first.Id)
	ledger := StartFailoverLedger(ctx)

	require.True(t, model.UpdateChannelStatus(5202, "", common.ChannelStatusAutoDisabled, "test"))

	param := &RetryParam{Ctx: ctx, TokenGroup: "default", ModelName: modelName, RequestPath: "/v1/chat/completions"}
	for range 20 {
		channel, _, exhausted, err := PickFailoverChannel(param)
		require.NoError(t, err)
		require.False(t, exhausted)
		require.NotNil(t, channel)
		assert.Equal(t, 5203, channel.Id)
	}
	ledger.Record(5203, 0)
	_, _, exhausted, err := PickFailoverChannel(param)
	require.NoError(t, err)
	assert.True(t, exhausted)
}

func TestFailoverMultiKeyChannelIgnoresKeyDisabledMidRequest(t *testing.T) {
	for _, tc := range []struct {
		name       string
		disableKey bool
		wantChosen bool
	}{
		{"untried key still enabled", false, true},
		{"untried key disabled mid request", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupChannelSelectAutoGroupsTest(t)
			const modelName = "failover-model"
			weight := uint(100)
			priority := int64(0)
			multiKey := model.Channel{
				Id: 5301, Type: constant.ChannelTypeClaudeSubscription, Key: "k0\nk1", Status: common.ChannelStatusEnabled,
				Name: "multi", Weight: &weight, Models: modelName, Group: "default", Priority: &priority,
				ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModeRandom},
			}
			require.NoError(t, db.Create(&multiKey).Error)
			require.NoError(t, db.Create(&model.Ability{Group: "default", Model: modelName, ChannelId: 5301, Enabled: true, Priority: &priority, Weight: weight}).Error)
			model.InitChannelCache()

			ctx := requestFailoverContext(nil, false)
			common.SetContextKey(ctx, constant.ContextKeyChannelId, 5301)
			common.SetContextKey(ctx, constant.ContextKeyChannelIsMultiKey, true)
			common.SetContextKey(ctx, constant.ContextKeyChannelMultiKeyIndex, 0)
			StartFailoverLedger(ctx)

			if tc.disableKey {
				require.True(t, model.UpdateChannelStatus(5301, "k1", common.ChannelStatusAutoDisabled, "test"))
			}

			param := &RetryParam{Ctx: ctx, TokenGroup: "default", ModelName: modelName, RequestPath: "/v1/chat/completions"}
			channel, _, exhausted, err := PickFailoverChannel(param)
			require.NoError(t, err)
			assert.Equal(t, tc.wantChosen, channel != nil)
			assert.Equal(t, !tc.wantChosen, exhausted)
		})
	}
}
