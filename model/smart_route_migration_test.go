package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const smartRoutesOptionKey = "smart_routing_setting.routes"

func legacyRouteJSON(name string, tierModel string, advisorModel string, simpleEffort string, normalEffort string, hardEffort string) string {
	tier := func(modelName string, effort string) string {
		return `{"model":"` + modelName + `","groups":["default"],"effort":"` + effort + `"}`
	}
	return `{"name":"` + name + `","tiers":{"simple":` + tier(tierModel, simpleEffort) +
		`,"normal":` + tier(tierModel, normalEffort) + `,"hard":` + tier(tierModel, hardEffort) + `},` +
		`"default_tier":"normal","scorer":{"model":"` + tierModel + `","groups":["default"],"timeout_ms":2000},` +
		`"advisor":{"model":"` + advisorModel + `","groups":["default"],"after_dissatisfied":3},` +
		`"downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500}`
}

func setupSmartRouteMigrationTest(t *testing.T, routesJSON string) {
	t.Helper()
	resetPricingEndpointTestTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Exec("DELETE FROM options").Error)

	originalRoutes := operation_setting.SmartRoutingRoutesToString()
	originalMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		operation_setting.LoadSmartRoutesFromJSONString(originalRoutes)
		common.OptionMap = originalMap
		require.NoError(t, DB.Exec("DELETE FROM options").Error)
	})

	insertPricingEndpointChannel(t, 1, constant.ChannelTypeXai, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 1, "m-both")
	insertPricingEndpointChannel(t, 2, constant.ChannelTypeCodex, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 2, "m-resp")
	InitChannelCache()
	RefreshPricing()

	require.NoError(t, DB.Create(&Option{Key: smartRoutesOptionKey, Value: routesJSON}).Error)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[smartRoutesOptionKey] = routesJSON
	common.OptionMapRWMutex.Unlock()
	operation_setting.LoadSmartRoutesFromJSONString(routesJSON)
}

func storedSmartRoutes(t *testing.T) string {
	t.Helper()
	var option Option
	require.NoError(t, DB.Where("`key` = ?", smartRoutesOptionKey).First(&option).Error)
	return option.Value
}

func parseRoutes(t *testing.T, value string) []operation_setting.SmartRoute {
	t.Helper()
	var routes []operation_setting.SmartRoute
	require.NoError(t, common.UnmarshalJsonStr(value, &routes))
	return routes
}

func TestMigrateSmartRouteSettings_AddsEndpointsAndFixedEffort(t *testing.T) {
	legacy := "[" + legacyRouteJSON("auto-gpt", "m-both", "m-both", "", "", "high") + "]"
	setupSmartRouteMigrationTest(t, legacy)

	require.NoError(t, MigrateSmartRouteSettings())

	stored := parseRoutes(t, storedSmartRoutes(t))
	require.Len(t, stored, 1)
	assert.Equal(t, []string{"openai-response", "openai"}, stored[0].Endpoints)
	hard := stored[0].Tiers["hard"]
	assert.Equal(t, "fixed", hard.EffortMode)
	assert.Equal(t, "high", hard.Effort)
	assert.Equal(t, "", stored[0].Tiers["simple"].EffortMode)
	assert.Equal(t, "", stored[0].Tiers["simple"].Effort)

	live, ok := operation_setting.FindSmartRoute("auto-gpt")
	require.True(t, ok)
	assert.Equal(t, []string{"openai-response", "openai"}, live.Endpoints)
	assert.Equal(t, "fixed", live.Tiers["hard"].EffortMode)
	assert.Equal(t, "high", live.Tiers["hard"].Effort)
}

func TestMigrateSmartRouteSettings_IsIdempotent(t *testing.T) {
	legacy := "[" + legacyRouteJSON("auto-gpt", "m-both", "m-both", "low", "medium", "high") + "]"
	setupSmartRouteMigrationTest(t, legacy)

	require.NoError(t, MigrateSmartRouteSettings())
	first := storedSmartRoutes(t)
	require.NoError(t, MigrateSmartRouteSettings())

	assert.Equal(t, first, storedSmartRoutes(t))
	routes := parseRoutes(t, first)
	require.Len(t, routes, 1)
	for name, want := range map[string]string{"simple": "low", "normal": "medium", "hard": "high"} {
		assert.Equal(t, "fixed", routes[0].Tiers[name].EffortMode, name)
		assert.Equal(t, want, routes[0].Tiers[name].Effort, name)
	}
}

func TestMigrateSmartRouteSettings_EndpointIntersection(t *testing.T) {
	tests := []struct {
		name         string
		tierModel    string
		advisorModel string
		want         []string
	}{
		{"models share both endpoints", "m-both", "m-both", []string{"openai-response", "openai"}},
		{"advisor narrows to responses", "m-both", "m-resp", []string{"openai-response"}},
		{"empty intersection falls back to responses", "m-both", "m-unknown", []string{"openai-response"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSmartRouteMigrationTest(t, "["+legacyRouteJSON("auto-gpt", tt.tierModel, tt.advisorModel, "", "", "")+"]")

			require.NoError(t, MigrateSmartRouteSettings())

			routes := parseRoutes(t, storedSmartRoutes(t))
			require.Len(t, routes, 1)
			assert.Equal(t, tt.want, routes[0].Endpoints)
		})
	}
}

func TestMigrateSmartRouteSettings_KeepsMigratedConfigUntouched(t *testing.T) {
	migrated := `[{"name":"auto-gpt","endpoints":["anthropic"],"tiers":{` +
		`"simple":{"model":"m-both","groups":["default"],"effort_mode":"follow","effort_min":"low","effort_max":"high"},` +
		`"normal":{"model":"m-both","groups":["default"],"effort_mode":"fixed","effort":"medium"},` +
		`"hard":{"model":"m-both","groups":["default"],"effort_mode":""}},` +
		`"default_tier":"normal","scorer":{"model":"m-both","groups":["default"],"timeout_ms":2000},` +
		`"advisor":{"model":"m-both","groups":["default"],"after_dissatisfied":3},` +
		`"downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500}]`
	setupSmartRouteMigrationTest(t, migrated)

	require.NoError(t, MigrateSmartRouteSettings())

	assert.Equal(t, migrated, storedSmartRoutes(t))
}

func TestMigrateSmartRouteSettings_WithoutLoadedOptionDoesNothing(t *testing.T) {
	legacy := "[" + legacyRouteJSON("auto-gpt", "m-both", "m-both", "low", "", "") + "]"
	setupSmartRouteMigrationTest(t, legacy)
	common.OptionMapRWMutex.Lock()
	delete(common.OptionMap, smartRoutesOptionKey)
	common.OptionMapRWMutex.Unlock()

	before := operation_setting.SmartRoutingRoutesToString()
	require.NoError(t, MigrateSmartRouteSettings())

	assert.Equal(t, legacy, storedSmartRoutes(t))
	assert.Equal(t, before, operation_setting.SmartRoutingRoutesToString())
	live, ok := operation_setting.FindSmartRoute("auto-gpt")
	require.True(t, ok)
	assert.Empty(t, live.Endpoints)
	assert.Equal(t, "", live.Tiers["simple"].EffortMode)
}

func TestMigrateSmartRouteSettings_WithoutOptionDoesNotCreateIt(t *testing.T) {
	setupSmartRouteMigrationTest(t, "[]")
	require.NoError(t, DB.Exec("DELETE FROM options").Error)
	common.OptionMapRWMutex.Lock()
	delete(common.OptionMap, smartRoutesOptionKey)
	common.OptionMapRWMutex.Unlock()

	require.NoError(t, MigrateSmartRouteSettings())

	var count int64
	require.NoError(t, DB.Model(&Option{}).Where("`key` = ?", smartRoutesOptionKey).Count(&count).Error)
	assert.Zero(t, count)
}

func TestMigrateSmartRouteSettings_EmptyPricingFailsWithoutWriting(t *testing.T) {
	legacy := "[" + legacyRouteJSON("auto-gpt", "m-both", "m-both", "low", "", "") + "]"
	setupSmartRouteMigrationTest(t, legacy)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	InitChannelCache()
	RefreshPricing()
	require.Empty(t, GetPricing())

	before := operation_setting.SmartRoutingRoutesToString()
	require.Error(t, MigrateSmartRouteSettings())

	assert.Equal(t, legacy, storedSmartRoutes(t))
	assert.Equal(t, before, operation_setting.SmartRoutingRoutesToString())
	live, ok := operation_setting.FindSmartRoute("auto-gpt")
	require.True(t, ok)
	assert.Empty(t, live.Endpoints)
	assert.Equal(t, "", live.Tiers["simple"].EffortMode)
}

func TestMigrateSmartRouteSettings_CorruptJSONFailsWithoutWriting(t *testing.T) {
	const corrupt = `[{"name":"auto-gpt",`
	setupSmartRouteMigrationTest(t, corrupt)

	require.Error(t, MigrateSmartRouteSettings())

	assert.Equal(t, corrupt, storedSmartRoutes(t))
}

func TestMigrateSmartRouteSettings_PartialMigrationKeepsEndpoints(t *testing.T) {
	partial := `[{"name":"auto-gpt","endpoints":["anthropic"],"tiers":{` +
		`"simple":{"model":"m-both","groups":["default"],"effort":"low"},` +
		`"normal":{"model":"m-both","groups":["default"],"effort":""},` +
		`"hard":{"model":"m-both","groups":["default"],"effort_mode":"follow","effort_min":"low","effort_max":"high"}},` +
		`"default_tier":"normal","scorer":{"model":"m-both","groups":["default"],"timeout_ms":2000},` +
		`"advisor":{"model":"m-both","groups":["default"],"after_dissatisfied":3},` +
		`"downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500}]`
	setupSmartRouteMigrationTest(t, partial)

	require.NoError(t, MigrateSmartRouteSettings())

	routes := parseRoutes(t, storedSmartRoutes(t))
	require.Len(t, routes, 1)
	assert.Equal(t, []string{"anthropic"}, routes[0].Endpoints)
	assert.Equal(t, "fixed", routes[0].Tiers["simple"].EffortMode)
	assert.Equal(t, "low", routes[0].Tiers["simple"].Effort)
	assert.Equal(t, "", routes[0].Tiers["normal"].EffortMode)
	assert.Equal(t, "follow", routes[0].Tiers["hard"].EffortMode)
	assert.Equal(t, "low", routes[0].Tiers["hard"].EffortMin)
	assert.Equal(t, "high", routes[0].Tiers["hard"].EffortMax)
}
