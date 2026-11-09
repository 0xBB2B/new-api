package model

import (
	"errors"
	"maps"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestRequestPolicyDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "policy_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			previousDB, previousType, previousSnapshot := DB, common.MainDatabaseType(), CurrentRequestPolicy()
			common.OptionMapRWMutex.Lock()
			previousOptions := maps.Clone(common.OptionMap)
			common.OptionMap = maps.Clone(previousSnapshot.Options)
			common.OptionMapRWMutex.Unlock()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&Option{}))
				for k, v := range previousSnapshot.Options {
					require.NoError(t, updateOptionMap(k, v))
				}
				requestPolicySnapshot.Store(previousSnapshot)
				common.OptionMapRWMutex.Lock()
				common.OptionMap = previousOptions
				common.OptionMapRWMutex.Unlock()
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				initCol()
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&Option{}))
			var version string
			query := "SELECT version()"
			if dialect == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("database version: %s", version)
			rules := `[{"name":"session","model_regex":[".*"],"key_sources":[{"type":"request_header","key":"X-Session"}],"ttl_seconds":0,"skip_retry_on_failure":false,"include_using_group":false,"param_override_template":{"temperature":0},"future_field":{"enabled":false}}]`
			require.NoError(t, UpdateRequestPolicyOptions(map[string]string{"RetryTimes": "2", "channel_affinity_setting.rules": rules}))
			assert.Equal(t, 2, CurrentRequestPolicy().RetryTimes)
			assert.Equal(t, 2, common.RetryTimes, "the runtime global follows the same write")
			require.NoError(t, UpdateOption("AutomaticRetryStatusCodes", "429,500-503"))
			assert.Equal(t, "429,500-503", CurrentRequestPolicy().Options["AutomaticRetryStatusCodes"])
			assert.Equal(t, "429,500-503", operation_setting.AutomaticRetryStatusCodesToString())
			assert.Error(t, UpdateRequestPolicyOptions(map[string]string{"request_policy_setting.mode": "off"}), "the removed mode switch is not a policy option")
			assert.False(t, CurrentRequestPolicy().Affinity.Rules[0].SkipRetryOnFailure)
			assert.Equal(t, rules, CurrentRequestPolicy().Options["channel_affinity_setting.rules"])
			loadOptionsFromDatabase()
			loadOptionsFromDatabase()
			assert.Equal(t, rules, CurrentRequestPolicy().Options["channel_affinity_setting.rules"], "legacy rule JSON survives reloads without dropping extension fields")
			require.NoError(t, UpdateRequestPolicyOptions(map[string]string{"channel_affinity_setting.session_mode": "strict"}))
			loadOptionsFromDatabase()
			loadOptionsFromDatabase()
			assert.Equal(t, "strict", CurrentRequestPolicy().Affinity.SessionMode)
			assert.Equal(t, rules, CurrentRequestPolicy().Options["channel_affinity_setting.rules"], "a global mode never rewrites rule fields")
			snapshot := CurrentRequestPolicy()
			assert.Error(t, UpdateRequestPolicyOptions(map[string]string{"channel_affinity_setting.session_mode": "unknown"}))
			assert.Same(t, snapshot, CurrentRequestPolicy())
			for _, value := range []string{"-1", "1.5", "bad", strconv.Itoa(math.MaxInt)} {
				assert.Error(t, UpdateRequestPolicyOptions(map[string]string{"RetryTimes": value}))
				assert.Same(t, snapshot, CurrentRequestPolicy())
			}
			assert.Error(t, UpdateRequestPolicyOptions(map[string]string{"channel_affinity_setting.rules": "[", "RetryTimes": "8"}))
			assert.Same(t, snapshot, CurrentRequestPolicy())
			var option Option
			require.NoError(t, db.Where(map[string]any{"key": "RetryTimes"}).First(&option).Error)
			assert.Equal(t, "2", option.Value)
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("policy_fail", func(tx *gorm.DB) {
				if tx.Statement.Table == "policy_test_options" {
					tx.AddError(errors.New("save rejected"))
				}
			}))
			assert.Error(t, UpdateRequestPolicyOptions(map[string]string{"RetryTimes": "8", "channel_affinity_setting.session_mode": "off"}))
			assert.Same(t, snapshot, CurrentRequestPolicy())
			require.NoError(t, db.Callback().Update().Remove("policy_fail"))
			option = Option{}
			require.NoError(t, db.Where(map[string]any{"key": "RetryTimes"}).First(&option).Error)
			assert.Equal(t, "2", option.Value)
			loadOptionsFromDatabase()
			assert.Equal(t, "strict", CurrentRequestPolicy().Affinity.SessionMode, "a failed save keeps the persisted global mode")
		})
	}
}

func TestIsRequestPolicyOption_SmartRouting(t *testing.T) {
	assert.True(t, IsRequestPolicyOption("smart_routing_setting.routes"))
	assert.False(t, IsRequestPolicyOption("smart_routing_setting.route"))
	assert.False(t, IsRequestPolicyOption("smart_routing_x"))
}

func TestSmartRoutingRoutesOptionReload(t *testing.T) {
	const key = "smart_routing_setting.routes"
	const fullRoutes = `[{"name":"auto-gpt","endpoints":["openai-response","openai"],"tiers":{"simple":{"model":"gpt-5.4-mini","groups":["api"],"effort_mode":"follow","effort":"","effort_min":"low","effort_max":"high"},"normal":{"model":"gpt-5.5","groups":["subscription","api"],"effort_mode":"","effort":"","effort_min":"","effort_max":""},"hard":{"model":"gpt-5.6-sol","groups":["subscription","api"],"effort_mode":"fixed","effort":"high","effort_min":"","effort_max":""}},"default_tier":"normal","scorer":{"model":"gpt-5.6-luna","groups":["api"],"timeout_ms":1500,"effort":""},"advisor":{"model":"gpt-5.6-pro","groups":["subscription"],"after_dissatisfied":3},"downgrade_max_history_tokens":30000,"confidence_threshold":0.7,"simple_max_message_chars":1500}]`
	const otherRoutes = `[{"name":"auto-chat","tiers":{"simple":{"model":"gpt-5.4-mini","groups":["api"]}}}]`

	originalMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		require.NoError(t, updateOptionMap(key, "[]"))
		common.OptionMap = originalMap
	})

	require.NoError(t, updateOptionMap(key, fullRoutes))
	route, ok := operation_setting.FindSmartRoute("auto-gpt")
	require.True(t, ok)
	assert.Equal(t, "gpt-5.6-sol", route.Tiers["hard"].Model)
	assert.Equal(t, []string{"subscription", "api"}, route.Tiers["hard"].Groups)
	assert.Equal(t, []string{"openai-response", "openai"}, route.Endpoints)
	assert.Equal(t, "fixed", route.Tiers["hard"].EffortMode)
	assert.Equal(t, "high", route.Tiers["hard"].Effort)
	assert.Equal(t, "", route.Tiers["normal"].EffortMode)
	assert.Equal(t, "", route.Tiers["normal"].Effort)
	assert.Equal(t, "follow", route.Tiers["simple"].EffortMode)
	assert.Equal(t, "low", route.Tiers["simple"].EffortMin)
	assert.Equal(t, "high", route.Tiers["simple"].EffortMax)
	assert.Equal(t, "normal", route.DefaultTier)
	assert.Equal(t, "gpt-5.6-luna", route.Scorer.Model)
	assert.Equal(t, 1500, route.Scorer.TimeoutMs)
	assert.Equal(t, "gpt-5.6-pro", route.Advisor.Model)
	assert.Equal(t, 3, route.Advisor.AfterDissatisfied)
	assert.Equal(t, 30000, route.DowngradeMaxHistoryTokens)
	assert.InDelta(t, 0.7, route.ConfidenceThreshold, 1e-9)
	assert.Equal(t, 1500, route.SimpleMaxMessageChars)
	assert.JSONEq(t, fullRoutes, requestPolicyDefaultOptions()[key])

	tests := []struct {
		name  string
		model string
		found bool
	}{
		{name: "exact name", model: "auto-gpt", found: true},
		{name: "name is case sensitive", model: "Auto-GPT", found: false},
		{name: "real model name is not a route", model: "gpt-5.5", found: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := operation_setting.FindSmartRoute(tt.model)
			assert.Equal(t, tt.found, ok)
		})
	}

	require.NoError(t, updateOptionMap(key, otherRoutes))
	_, ok = operation_setting.FindSmartRoute("auto-gpt")
	assert.False(t, ok)
	replaced, ok := operation_setting.FindSmartRoute("auto-chat")
	require.True(t, ok)
	assert.NotContains(t, replaced.Tiers, "hard")
	assert.Zero(t, replaced.Scorer.TimeoutMs)

	require.NoError(t, updateOptionMap(key, "{broken"))
	_, ok = operation_setting.FindSmartRoute("auto-chat")
	assert.True(t, ok)
}

func TestSmartTierRankOrder(t *testing.T) {
	assert.Less(t, operation_setting.SmartTierRank("simple"), operation_setting.SmartTierRank("normal"))
	assert.Less(t, operation_setting.SmartTierRank("normal"), operation_setting.SmartTierRank("hard"))
	assert.Equal(t, -1, operation_setting.SmartTierRank("advisor"))
}
