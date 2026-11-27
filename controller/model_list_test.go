package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type listModelsResponse struct {
	Success bool               `json:"success"`
	Data    []dto.OpenAIModels `json:"data"`
	Object  string             `json:"object"`
}

type userModelsResponse struct {
	Success bool     `json:"success"`
	Data    []string `json:"data"`
}

func setupModelListControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	initModelListColumnNames(t)

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func initModelListColumnNames(t *testing.T) {
	t.Helper()

	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	defer func() {
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	}()

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s_init?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))

	require.NoError(t, model.InitDB())
	if model.DB != nil {
		sqlDB, err := model.DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
}

func withTieredBillingConfig(t *testing.T, modes map[string]string, exprs map[string]string) {
	t.Helper()

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		if strings.HasPrefix(key, "billing_setting.") {
			saved[key] = value
		}
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		model.InvalidatePricingCache()
	})

	modeBytes, err := common.Marshal(modes)
	require.NoError(t, err)
	exprBytes, err := common.Marshal(exprs)
	require.NoError(t, err)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": string(modeBytes),
		"billing_setting.billing_expr": string(exprBytes),
	}))
	model.InvalidatePricingCache()
}

func withSelfUseModeDisabled(t *testing.T) {
	t.Helper()

	original := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = false
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = original
	})
}

func withSelfUseModeEnabled(t *testing.T) {
	t.Helper()

	original := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = original
	})
}

func decodeListModelsPayload(t *testing.T, recorder *httptest.ResponseRecorder) listModelsResponse {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload listModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "list", payload.Object)
	return payload
}

func decodeListModelsResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]struct{} {
	t.Helper()

	payload := decodeListModelsPayload(t, recorder)
	ids := make(map[string]struct{}, len(payload.Data))
	for _, item := range payload.Data {
		ids[item.Id] = struct{}{}
	}
	return ids
}

func pricingByModelName(pricings []model.Pricing) map[string]model.Pricing {
	byName := make(map[string]model.Pricing, len(pricings))
	for _, pricing := range pricings {
		byName[pricing.ModelName] = pricing
	}
	return byName
}

func modelSquareData[T any](t *testing.T, handler gin.HandlerFunc, path, userGroup string) T {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)
	if userGroup != "" {
		common.SetContextKey(ctx, constant.ContextKeyUserGroup, userGroup)
	}
	handler(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    T    `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response.Data
}

func TestModelSquareGroupVisibility(t *testing.T) {
	originalGroups := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
	})

	for _, dialect := range []struct{ kind, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(dialect.kind, func(t *testing.T) {
			if dialect.env != "" && os.Getenv(dialect.env) == "" {
				t.Skip("set " + dialect.env + " to run this database")
			}
			db := modelManagementDB(t, dialect.kind, os.Getenv(dialect.env))
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Public"}`))
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"internal":2}`))
			require.NoError(t, db.Create(&[]model.Ability{
				{Model: "shared", Group: "default", ChannelId: 1, Enabled: true},
				{Model: "shared", Group: "internal", ChannelId: 1, Enabled: true},
				{Model: "private-only", Group: "internal", ChannelId: 1, Enabled: true},
				{Model: "wildcard", Group: "all", ChannelId: 1, Enabled: true},
				{Model: "wildcard", Group: "internal", ChannelId: 1, Enabled: true},
			}).Error)
			model.InvalidatePricingCache()
			t.Cleanup(model.InvalidatePricingCache)

			guest := pricingByModelName(modelSquareData[[]model.Pricing](t, GetPricing, "/api/pricing", ""))
			assert.Equal(t, []string{"default"}, guest["shared"].EnableGroup)
			assert.Equal(t, []string{"all"}, guest["wildcard"].EnableGroup)
			assert.NotContains(t, guest, "private-only")
			member := pricingByModelName(modelSquareData[[]model.Pricing](t, GetPricing, "/api/pricing", "internal"))
			assert.ElementsMatch(t, []string{"default", "internal"}, member["shared"].EnableGroup)
			assert.Equal(t, []string{"internal"}, member["private-only"].EnableGroup)

			require.NoError(t, db.AutoMigrate(&model.PerfMetric{}))
			ts := time.Now().Add(-time.Hour).Unix()
			require.NoError(t, db.Create(&[]model.PerfMetric{
				{ModelName: "shared", Group: "default", BucketTs: ts, RequestCount: 1, SuccessCount: 1},
				{ModelName: "shared", Group: "internal", BucketTs: ts, RequestCount: 1},
				{ModelName: "private-only", Group: "internal", BucketTs: ts, RequestCount: 1},
			}).Error)
			const detailPath = "/api/perf-metrics?model=shared"
			const summaryPath = "/api/perf-metrics/summary?recent_model=private-only"
			detail := modelSquareData[perfmetrics.QueryResult](t, GetPerfMetrics, detailPath, "")
			require.Len(t, detail.Groups, 1)
			assert.Equal(t, "default", detail.Groups[0].Group)
			denied := modelSquareData[perfmetrics.QueryResult](t, GetPerfMetrics, detailPath+"&group=internal", "")
			assert.Empty(t, denied.Groups)
			summary := modelSquareData[perfmetrics.SummaryAllResult](t, GetPerfMetricsSummary, summaryPath, "")
			require.Len(t, summary.Models, 1)
			assert.Equal(t, "shared", summary.Models[0].ModelName)
			assert.Equal(t, 100.0, summary.Models[0].SuccessRate)
			assert.NotContains(t, summary.RecentSuccessSeries, "private-only")
			allowed := modelSquareData[perfmetrics.QueryResult](t, GetPerfMetrics, detailPath+"&group=internal", "internal")
			require.Len(t, allowed.Groups, 1)
			assert.Equal(t, "internal", allowed.Groups[0].Group)

			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{}`))
			assert.Empty(t, modelSquareData[[]model.Pricing](t, GetPricing, "/api/pricing", ""))
			assert.Empty(t, modelSquareData[perfmetrics.QueryResult](t, GetPerfMetrics, detailPath, "").Groups)
			assert.Empty(t, modelSquareData[perfmetrics.SummaryAllResult](t, GetPerfMetricsSummary, summaryPath, "").Models)
			dashboard := modelSquareData[perfmetrics.SummaryAllResult](t, GetDashboardPerfMetricsSummary, "/api/data/perf-metrics/summary", "")
			assert.Len(t, dashboard.Models, 2)
			require.NotNil(t, dashboard.Summary)
			assert.Equal(t, 33.33, dashboard.Summary.SuccessRate)
		})
	}
}

func TestModelSquareOrderFollowsSavedSetting(t *testing.T) {
	originalGroups := setting.UserUsableGroups2JSONString()
	originalOrder := *operation_setting.GetModelSquareSetting()
	originalDataExport := common.DataExportEnabled
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		*operation_setting.GetModelSquareSetting() = originalOrder
		common.DataExportEnabled = originalDataExport
	})

	for _, dialect := range []struct{ kind, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(dialect.kind, func(t *testing.T) {
			if dialect.env != "" && os.Getenv(dialect.env) == "" {
				t.Skip("set " + dialect.env + " to run this database")
			}
			db := modelManagementDB(t, dialect.kind, os.Getenv(dialect.env))
			// The service caches usage across databases, so usage ordering is
			// covered in the service tests and the query is checked directly below.
			common.DataExportEnabled = false
			*operation_setting.GetModelSquareSetting() = operation_setting.ModelSquareSetting{SortMode: "manual", SortMetric: "tokens", SortDays: 7, SortModels: []string{}}
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Public"}`))
			vendor := model.Vendor{Name: "Acme", Icon: "Acme.Color"}
			require.NoError(t, db.Create(&vendor).Error)
			require.NoError(t, db.Create(&model.Model{ModelName: "vip-only", Icon: "Vip.Color", VendorID: vendor.Id, Status: 1}).Error)
			require.NoError(t, db.Create(&[]model.Ability{
				{Model: "alpha", Group: "default", ChannelId: 1, Enabled: true},
				{Model: "gamma", Group: "default", ChannelId: 1, Enabled: true},
				{Model: "vip-only", Group: "vip", ChannelId: 1, Enabled: true},
			}).Error)
			model.InvalidatePricingCache()
			t.Cleanup(model.InvalidatePricingCache)

			manual := []string{"gamma", "alpha"}
			for _, step := range []struct {
				key, value string
				rejected   bool
				want       []string
			}{
				{want: []string{}},
				{key: "model_square_setting.sort_models", value: `["gamma","vip-only","alpha","gamma"]`, want: manual},
				{key: "model_square_setting.sort_mode", value: "random", rejected: true, want: manual},
				{key: "model_square_setting.sort_metric", value: "cost", rejected: true, want: manual},
				{key: "model_square_setting.sort_days", value: "3", rejected: true, want: manual},
				{key: "model_square_setting.sort_models", value: `"gamma"`, rejected: true, want: manual},
			} {
				if step.key != "" {
					err := model.UpdateOption(step.key, step.value)
					if step.rejected {
						require.Error(t, err, step.key)
					} else {
						require.NoError(t, err, step.key)
					}
				}
				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
				GetPricing(ctx)
				require.Equal(t, http.StatusOK, recorder.Code)
				var response struct {
					ModelOrder map[string][]string `json:"model_order"`
				}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
				assert.Equal(t, map[string][]string{"recommended": step.want, "requests": {}, "tokens": {}}, response.ModelOrder, step.key)
			}

			// The admin preview offers the models of every group.
			assert.ElementsMatch(t, []modelSquareUsage{
				{ModelName: "alpha"},
				{ModelName: "gamma"},
				{ModelName: "vip-only", Icon: "Vip.Color", VendorName: "Acme", VendorIcon: "Acme.Color"},
			}, modelSquareData[[]modelSquareUsage](t, GetModelSquareUsage, "/api/models/square_usage?days=7", ""))

			require.NoError(t, db.AutoMigrate(&model.QuotaData{}))
			now := time.Now().Unix()
			require.NoError(t, db.Create(&[]model.QuotaData{
				{UserID: 1, Username: "a", ModelName: "alpha", CreatedAt: now - 3600, Count: 3, TokenUsed: 100},
				{UserID: 2, Username: "b", ModelName: "alpha", CreatedAt: now - 7200, Count: 2},
				{UserID: 1, Username: "a", ModelName: "vip-only", CreatedAt: now - 3600, Count: 1},
				{UserID: 1, Username: "a", ModelName: "gamma", CreatedAt: now - 8*24*3600, Count: 9, TokenUsed: 900},
				{UserID: 1, Username: "a", ModelName: "", CreatedAt: now - 3600, Count: 4, TokenUsed: 40},
			}).Error)
			totals, err := model.GetModelUsageTotals(now-7*24*3600, now)
			require.NoError(t, err)
			assert.ElementsMatch(t, []model.ModelUsageTotal{
				{ModelName: "alpha", Requests: 5, Tokens: 100},
				{ModelName: "vip-only", Requests: 1},
			}, totals)
		})
	}
}

func decodeUserModelsResponse(t *testing.T, recorder *httptest.ResponseRecorder) []string {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload userModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	return payload.Data
}

func TestGetUserModelsFiltersByRequestedGroup(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1002,
		Username: "playground-model-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-default-only-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-disabled-model", ChannelId: 1, Enabled: false},
	}).Error)

	defaultRecorder := httptest.NewRecorder()
	defaultContext, _ := gin.CreateTestContext(defaultRecorder)
	defaultContext.Request = httptest.NewRequest(http.MethodGet, "/api/user/models?group=default", nil)
	defaultContext.Set("id", 1002)

	GetUserModels(defaultContext)

	defaultModels := decodeUserModelsResponse(t, defaultRecorder)
	require.ElementsMatch(t, []string{"zz-default-only-model"}, defaultModels)

	vipRecorder := httptest.NewRecorder()
	vipContext, _ := gin.CreateTestContext(vipRecorder)
	vipContext.Request = httptest.NewRequest(http.MethodGet, "/api/user/models?group=vip", nil)
	vipContext.Set("id", 1002)

	GetUserModels(vipContext)

	require.Empty(t, decodeUserModelsResponse(t, vipRecorder))
}

func TestGetUserModelsExpandsAutoGroupsInConfiguredOrder(t *testing.T) {
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalSpecialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		specialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
		specialGroups.Clear()
		specialGroups.AddAll(originalSpecialGroups)
	})

	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["vip","default","unavailable"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"auto":"自动分组","default":"默认分组","unavailable":"不可用分组"}`))
	specialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	specialGroups.Clear()
	specialGroups.Set("default", map[string]string{
		"+:vip":         "VIP 分组",
		"-:unavailable": "",
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1003,
		Username: "playground-auto-model-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "vip", Model: "zz-vip-model", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "zz-shared-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-default-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-shared-model", ChannelId: 2, Enabled: true},
		{Group: "unavailable", Model: "zz-unavailable-model", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/user/models?group=auto", nil)
	context.Set("id", 1003)

	GetUserModels(context)

	models := decodeUserModelsResponse(t, recorder)
	require.Len(t, models, 3)
	assert.ElementsMatch(t, []string{"zz-vip-model", "zz-shared-model"}, models[:2])
	assert.Equal(t, "zz-default-model", models[2])
}

func TestListModelsIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-tiered-visible-model":      "tiered_expr",
		"zz-tiered-empty-expr-model":   "tiered_expr",
		"zz-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-tiered-empty-expr-model": "   ",
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1001,
		Username: "model-list-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-tiered-visible-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-empty-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-missing-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-unpriced-model", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1001)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-tiered-visible-model")
	require.NotContains(t, ids, "zz-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-unpriced-model")

	pricingByName := pricingByModelName(model.GetPricing())
	visiblePricing, ok := pricingByName["zz-tiered-visible-model"]
	require.True(t, ok)
	require.Equal(t, "tiered_expr", visiblePricing.BillingMode)
	require.NotEmpty(t, visiblePricing.BillingExpr)

	emptyExprPricing, ok := pricingByName["zz-tiered-empty-expr-model"]
	require.True(t, ok)
	require.Empty(t, emptyExprPricing.BillingMode)
	require.Empty(t, emptyExprPricing.BillingExpr)

	missingExprPricing, ok := pricingByName["zz-tiered-missing-expr-model"]
	require.True(t, ok)
	require.Empty(t, missingExprPricing.BillingMode)
	require.Empty(t, missingExprPricing.BillingExpr)
}

func TestListModelsUsesAdvancedCustomEndpointTypesFromPricingCache(t *testing.T) {
	withSelfUseModeEnabled(t)
	db := setupModelListControllerTestDB(t)

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		model.InvalidatePricingCache()
	})

	require.NoError(t, db.Create(&model.User{
		Id:       1003,
		Username: "advanced-custom-model-list-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)

	channel := &model.Channel{
		Id:     701,
		Type:   constant.ChannelTypeAdvancedCustom,
		Key:    "advanced-custom-key",
		Status: common.ChannelStatusEnabled,
		Name:   "advanced-custom-channel",
		Group:  "default",
		Models: "gemini-3.5-flash",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{
			Routes: []dto.AdvancedCustomRoute{
				{
					IncomingPath: "/v1/chat/completions",
					UpstreamPath: "/v1/chat/completions",
				},
				{
					IncomingPath: "/v1/responses",
					UpstreamPath: "/v1beta/models/{model}:generateContent",
					Converter:    "openai_responses_to_gemini_generate_content",
					Models:       []string{"re:^gemini-"},
				},
			},
		},
	})
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "gemini-3.5-flash",
		ChannelId: 701,
		Enabled:   true,
	}).Error)

	model.InitChannelCache()
	model.GetPricing()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1003)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	payload := decodeListModelsPayload(t, recorder)
	require.Len(t, payload.Data, 1)
	require.Equal(t, "gemini-3.5-flash", payload.Data[0].Id)
	require.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}, payload.Data[0].SupportedEndpointTypes)
}

func TestListModelsTokenLimitIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-token-tiered-visible-model":      "tiered_expr",
		"zz-token-tiered-empty-expr-model":   "tiered_expr",
		"zz-token-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-token-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-token-tiered-empty-expr-model": "",
	})
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-token-tiered-visible-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-token-tiered-empty-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-token-tiered-missing-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-token-unpriced-model", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{
		"zz-token-tiered-visible-model":      true,
		"zz-token-tiered-empty-expr-model":   true,
		"zz-token-tiered-missing-expr-model": true,
		"zz-token-unpriced-model":            true,
	})

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-token-tiered-visible-model")
	require.NotContains(t, ids, "zz-token-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-token-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-token-unpriced-model")
}

func TestListModelsTokenLimitUsesResolvedCustomAutoGroups(t *testing.T) {
	withSelfUseModeEnabled(t)
	originalMax := setting.GetMaxTokenAutoGroups()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, setting.UpdateMaxTokenAutoGroups("5"))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateMaxTokenAutoGroups(fmt.Sprintf("%d", originalMax)))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "vip", Model: "zz-vip-allowed", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "zz-vip-denied", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-default-outside-snapshot", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "auto")
	common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"vip"})
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{
		"zz-vip-allowed":              true,
		"zz-default-outside-snapshot": true,
		"zz-not-enabled":              true,
	})

	ListModels(ctx, constant.ChannelTypeOpenAI)
	ids := decodeListModelsResponse(t, recorder)
	require.Equal(t, map[string]struct{}{"zz-vip-allowed": {}}, ids)

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	emptyRecorder := httptest.NewRecorder()
	emptyCtx, _ := gin.CreateTestContext(emptyRecorder)
	emptyCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(emptyCtx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(emptyCtx, constant.ContextKeyTokenGroup, "auto")
	common.SetContextKey(emptyCtx, constant.ContextKeyTokenAutoGroups, []string{"vip"})
	common.SetContextKey(emptyCtx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(emptyCtx, constant.ContextKeyTokenModelLimit, map[string]bool{"zz-vip-allowed": true})

	require.NotPanics(t, func() {
		ListModels(emptyCtx, constant.ChannelTypeAnthropic)
	})
	var anthropicResponse struct {
		Data    []dto.AnthropicModel `json:"data"`
		FirstID string               `json:"first_id"`
		LastID  string               `json:"last_id"`
	}
	require.NoError(t, common.Unmarshal(emptyRecorder.Body.Bytes(), &anthropicResponse))
	require.Empty(t, anthropicResponse.Data)
	require.Empty(t, anthropicResponse.FirstID)
	require.Empty(t, anthropicResponse.LastID)
}

func TestSetupLoginDoesNotTouchPasswordWhenPasswordFieldOmitted(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.AuditLog{}, &model.UserSession{}, &model.TwoFA{}, &model.PasskeyCredential{}))

	hashedPassword, err := common.Password2Hash("CurrentPassword123")
	require.NoError(t, err)
	user := &model.User{
		Username: "twofa-user",
		Password: hashedPassword,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(user).Error)

	router := gin.New()
	router.GET("/", func(c *gin.Context) {
		setupLogin(&model.User{
			Id:          user.Id,
			AuthVersion: user.AuthVersion,
			Username:    user.Username,
			Role:        user.Role,
			Status:      user.Status,
			Group:       user.Group,
		}, nil, c)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	assert.Equal(t, hashedPassword, stored.Password)
}

func setupVirtualModelListTest(t *testing.T) {
	t.Helper()
	withSelfUseModeDisabled(t)
	db := setupModelListControllerTestDB(t)

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalRoutes := operation_setting.SmartRoutingRoutesToString()
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		operation_setting.LoadSmartRoutesFromJSONString(originalRoutes)
		model.InvalidatePricingCache()
	})

	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id: 1, Type: constant.ChannelTypeCodex, Key: "key-1", Status: common.ChannelStatusEnabled,
		Name: "channel-1", Weight: &weight, Models: "m-resp", Group: "api", Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "api", Model: "m-resp", ChannelId: 1, Enabled: true, Priority: &priority, Weight: weight,
	}).Error)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"m-resp":0.01}`))
	operation_setting.LoadSmartRoutesFromJSONString(`[]`)
	model.InitChannelCache()
	model.RefreshPricing()
}

func listVirtualModelTestModels(t *testing.T, tokenModelLimit map[string]bool) []dto.OpenAIModels {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "api")
	if tokenModelLimit != nil {
		common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
		common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, tokenModelLimit)
	}

	ListModels(ctx, constant.ChannelTypeOpenAI)

	return decodeListModelsPayload(t, recorder).Data
}

func TestListModelsVirtualModel(t *testing.T) {
	tier := `{"model":"m-resp","groups":["api"],"effort":""}`
	route := `[{"name":"auto-gpt","endpoints":["openai-response","anthropic"],"tiers":{"simple":` + tier + `,"normal":` + tier + `,"hard":` + tier + `},` +
		`"default_tier":"normal","scorer":{"model":"m-resp","groups":["api"],"timeout_ms":2000},` +
		`"advisor":{"model":"m-resp","groups":["api"],"after_dissatisfied":2},` +
		`"downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500}]`

	tests := []struct {
		name        string
		route       string
		tokenLimit  map[string]bool
		wantVirtual bool
	}{
		{name: "no whitelist shows virtual model", route: route, wantVirtual: true},
		{name: "whitelist without virtual model hides it", route: route, tokenLimit: map[string]bool{"m-resp": true}},
		{name: "whitelist with virtual model shows it", route: route, tokenLimit: map[string]bool{"auto-gpt": true}, wantVirtual: true},
		{name: "no virtual model configured", route: `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupVirtualModelListTest(t)
			operation_setting.LoadSmartRoutesFromJSONString(tt.route)

			models := listVirtualModelTestModels(t, tt.tokenLimit)

			var virtual *dto.OpenAIModels
			ids := make([]string, 0, len(models))
			for i := range models {
				ids = append(ids, models[i].Id)
				if models[i].Id == "auto-gpt" {
					virtual = &models[i]
				}
			}
			if !tt.wantVirtual {
				assert.Nil(t, virtual)
				if tt.tokenLimit == nil {
					assert.Equal(t, []string{"m-resp"}, ids)
				}
				return
			}
			require.NotNil(t, virtual)
			assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIResponse, constant.EndpointTypeAnthropic}, virtual.SupportedEndpointTypes)
		})
	}
}

func TestGetUserModelsAppendsVirtualModels(t *testing.T) {
	originalRoutes := operation_setting.SmartRoutingRoutesToString()
	t.Cleanup(func() { operation_setting.LoadSmartRoutesFromJSONString(originalRoutes) })

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1004,
		Username: "virtual-model-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "default", Model: "zz-real-model", ChannelId: 1, Enabled: true,
	}).Error)

	const routes = `[{"name":"auto-gpt","endpoints":["openai-response","anthropic"]},{"name":"auto-chat","endpoints":["openai"]}]`
	tests := []struct {
		name   string
		routes string
		query  string
		want   []string
	}{
		{name: "no routes keeps real models only", routes: `[]`, query: "", want: []string{"zz-real-model"}},
		{name: "no routes with endpoint keeps real models only", routes: `[]`, query: "?endpoint=openai", want: []string{"zz-real-model"}},
		{name: "appends all virtual models in config order", routes: routes, query: "", want: []string{"zz-real-model", "auto-gpt", "auto-chat"}},
		{name: "group param still appends", routes: routes, query: "?group=default", want: []string{"zz-real-model", "auto-gpt", "auto-chat"}},
		{name: "group without real models still appends", routes: routes, query: "?group=vip", want: []string{"auto-gpt", "auto-chat"}},
		{name: "endpoint openai", routes: routes, query: "?endpoint=openai", want: []string{"zz-real-model", "auto-chat"}},
		{name: "endpoint anthropic", routes: routes, query: "?endpoint=anthropic", want: []string{"zz-real-model", "auto-gpt"}},
		{name: "endpoint openai-response", routes: routes, query: "?endpoint=openai-response", want: []string{"zz-real-model", "auto-gpt"}},
		{name: "endpoint gemini appends nothing", routes: routes, query: "?endpoint=gemini", want: []string{"zz-real-model"}},
		{name: "unknown endpoint appends nothing", routes: routes, query: "?endpoint=xyz", want: []string{"zz-real-model"}},
		{name: "virtual model sharing a real model name appears once", routes: `[{"name":"zz-real-model","endpoints":["openai"]},{"name":"auto-chat","endpoints":["openai"]}]`, query: "", want: []string{"zz-real-model", "auto-chat"}},
		{name: "empty endpoint behaves like no endpoint", routes: routes, query: "?endpoint=", want: []string{"zz-real-model", "auto-gpt", "auto-chat"}},
		{name: "group and endpoint combine", routes: routes, query: "?group=default&endpoint=openai", want: []string{"zz-real-model", "auto-chat"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation_setting.LoadSmartRoutesFromJSONString(tt.routes)

			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/user/models"+tt.query, nil)
			context.Set("id", 1004)

			GetUserModels(context)

			assert.Equal(t, tt.want, decodeUserModelsResponse(t, recorder))
		})
	}
}

const solCapabilities = `{"top_provider":{"max_completion_tokens":128000},"architecture":{"input_modalities":["text","image"],"output_modalities":["text"]},"reasoning":{"supported_efforts":["none","low","medium","high","xhigh","max"]}}`

var capabilityKeys = []string{"context_length", "top_provider", "architecture", "reasoning"}

func setupCapabilityListTest(t *testing.T, names ...string) {
	t.Helper()
	withSelfUseModeEnabled(t)
	db := setupModelListControllerTestDB(t)

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRoutes := operation_setting.SmartRoutingRoutesToString()
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		operation_setting.LoadSmartRoutesFromJSONString(originalRoutes)
		model.InvalidatePricingCache()
	})

	abilities := make([]model.Ability, 0, len(names))
	for _, name := range names {
		abilities = append(abilities, model.Ability{Group: "default", Model: name, ChannelId: 1, Enabled: true})
	}
	require.NoError(t, db.Create(&abilities).Error)
	require.NoError(t, db.Create(&[]model.Model{
		{ModelName: "gpt-6.2-sol", Status: 1, Capabilities: solCapabilities},
		{ModelName: "qwen4-max", Status: 1, Capabilities: `{"reasoning":{"supported_efforts":["none","low","high","xhigh"]}}`},
		{ModelName: "gpt-4o", Status: 1, Capabilities: `{"reasoning":{"supported_efforts":[]}}`},
		{ModelName: "gpt-6", Status: 1, NameRule: model.NameRulePrefix, Capabilities: `{"context_length":1050000}`},
	}).Error)
	operation_setting.LoadSmartRoutesFromJSONString(`[]`)
	model.RefreshPricing()
}

func requestCapabilityList(t *testing.T, modelType int) []byte {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")

	ListModels(ctx, modelType)

	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder.Body.Bytes()
}

func rawModelObjects(t *testing.T, body []byte, key string) map[string]json.RawMessage {
	t.Helper()
	var payload map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(body, &payload))
	var items []json.RawMessage
	require.NoError(t, common.Unmarshal(payload[key], &items))
	byID := make(map[string]json.RawMessage, len(items))
	for _, item := range items {
		var head struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		}
		require.NoError(t, common.Unmarshal(item, &head))
		id := head.Id
		if id == "" {
			id = head.Name
		}
		byID[id] = item
	}
	return byID
}

func objectKeys(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(raw, &fields))
	return fields
}

func TestListModelsCapabilities(t *testing.T) {
	setupCapabilityListTest(t, "gpt-6.2-sol", "qwen4-max", "gpt-4o", "foo-model", "claude-opus-5", "gpt-6.2-luna")
	operation_setting.LoadSmartRoutesFromJSONString(`[{"name":"auto","endpoints":["openai"],"tiers":{"simple":{"model":"gpt-4o","groups":["default"],"effort":""},"normal":{"model":"gpt-4o","groups":["default"],"effort":""},"hard":{"model":"gpt-4o","groups":["default"],"effort":""}},"default_tier":"normal","scorer":{"model":"gpt-4o","groups":["default"],"timeout_ms":2000},"advisor":{"model":"gpt-4o","groups":["default"],"after_dissatisfied":2},"downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500}]`)

	body := requestCapabilityList(t, constant.ChannelTypeOpenAI)
	objects := rawModelObjects(t, body, "data")

	t.Run("whole record applies: sol omits context_length despite prefix record, other fields kept", func(t *testing.T) {
		raw := string(objects["gpt-6.2-sol"])
		fields := objectKeys(t, objects["gpt-6.2-sol"])
		for _, key := range []string{"id", "object", "created", "owned_by", "supported_endpoint_types"} {
			require.Contains(t, fields, key)
		}
		last := strings.Index(raw, `"id"`)
		for _, key := range []string{`"object"`, `"created"`, `"owned_by"`, `"supported_endpoint_types"`, `"top_provider"`} {
			idx := strings.Index(raw, key)
			require.Greater(t, idx, last, key)
			last = idx
		}
		var want map[string]any
		require.NoError(t, common.Unmarshal([]byte(solCapabilities), &want))
		var all map[string]any
		require.NoError(t, common.Unmarshal(objects["gpt-6.2-sol"], &all))
		gotCaps := map[string]any{}
		for k := range want {
			gotCaps[k] = all[k]
		}
		assert.Equal(t, want, gotCaps)
		assert.NotContains(t, fields, "context_length")
	})

	t.Run("effort-only record", func(t *testing.T) {
		fields := objectKeys(t, objects["qwen4-max"])
		assert.Contains(t, fields, "reasoning")
		assert.NotContains(t, fields, "context_length")
		assert.NotContains(t, fields, "top_provider")
		assert.NotContains(t, fields, "architecture")
		assert.JSONEq(t, `{"supported_efforts":["none","low","high","xhigh"]}`, string(fields["reasoning"]))
	})

	t.Run("confirmed no reasoning keeps empty array in raw JSON", func(t *testing.T) {
		assert.Contains(t, string(objects["gpt-4o"]), `"supported_efforts":[]`)
	})

	t.Run("no record and virtual model carry no capability keys", func(t *testing.T) {
		for _, id := range []string{"foo-model", "auto"} {
			require.Contains(t, objects, id)
			fields := objectKeys(t, objects[id])
			for _, key := range capabilityKeys {
				assert.NotContains(t, fields, key, id)
			}
		}
	})

	t.Run("claude 2026 model gets inferred reasoning only", func(t *testing.T) {
		fields := objectKeys(t, objects["claude-opus-5"])
		require.Contains(t, fields, "reasoning")
		want := smartroute.EffectiveThinkingCapability("claude-opus-5")
		require.NotEmpty(t, want.Efforts)
		var reasoning dto.OpenAIModelReasoning
		require.NoError(t, common.Unmarshal(fields["reasoning"], &reasoning))
		assert.Equal(t, want.Efforts, reasoning.SupportedEfforts)
		assert.NotContains(t, fields, "context_length")
		assert.NotContains(t, fields, "top_provider")
		assert.NotContains(t, fields, "architecture")
	})

	t.Run("prefix record applies to channel model", func(t *testing.T) {
		fields := objectKeys(t, objects["gpt-6.2-luna"])
		assert.JSONEq(t, `1050000`, string(fields["context_length"]))
	})
}

func TestListModelsCapabilitiesAbsentInOtherFormats(t *testing.T) {
	setupCapabilityListTest(t, "gpt-6.2-sol")

	tests := []struct {
		name      string
		modelType int
		key       string
	}{
		{name: "anthropic", modelType: constant.ChannelTypeAnthropic, key: "data"},
		{name: "gemini", modelType: constant.ChannelTypeGemini, key: "models"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := rawModelObjects(t, requestCapabilityList(t, tt.modelType), tt.key)
			require.Contains(t, objects, "gpt-6.2-sol")
			fields := objectKeys(t, objects["gpt-6.2-sol"])
			for _, key := range capabilityKeys {
				assert.NotContains(t, fields, key)
			}
		})
	}
}

func TestListModelsCapabilitiesReflectRecordUpdate(t *testing.T) {
	setupCapabilityListTest(t, "gpt-6.2-sol")

	before := objectKeys(t, rawModelObjects(t, requestCapabilityList(t, constant.ChannelTypeOpenAI), "data")["gpt-6.2-sol"])
	assert.NotContains(t, before, "context_length")

	require.NoError(t, model.DB.Model(&model.Model{}).Where("model_name = ?", "gpt-6.2-sol").
		Update("capabilities", `{"context_length":2000000,"top_provider":{"max_completion_tokens":128000}}`).Error)
	model.RefreshPricing()

	after := objectKeys(t, rawModelObjects(t, requestCapabilityList(t, constant.ChannelTypeOpenAI), "data")["gpt-6.2-sol"])
	assert.JSONEq(t, `2000000`, string(after["context_length"]))
}
