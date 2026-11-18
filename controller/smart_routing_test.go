package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const smartRoutingBaseRoutes = `[]`

func smartRouteJSON(name string, afterDissatisfied int) string {
	tier := `{"model":"m-both","groups":["api"],"effort":""}`
	return fmt.Sprintf(`{"name":%q,"endpoints":["openai-response","openai"],"tiers":{"simple":%s,"normal":%s,"hard":%s},"default_tier":"normal",`+
		`"scorer":{"model":"m-both","groups":["api"],"timeout_ms":2000},`+
		`"advisor":{"model":"m-both","groups":["api"],"after_dissatisfied":%d},`+
		`"downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500}`,
		name, tier, tier, tier, afterDissatisfied)
}

func setupSmartRoutingControllerTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalRoutes := operation_setting.SmartRoutingRoutesToString()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}, &model.Option{}))
	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = true
	common.RedisEnabled = false
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMapRWMutex.Unlock()

	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id: 1, Type: constant.ChannelTypeXai, Key: "key-1", Status: common.ChannelStatusEnabled,
		Name: "channel-1", Weight: &weight, Models: "m-both", Group: "api", Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "api", Model: "m-both", ChannelId: 1, Enabled: true, Priority: &priority, Weight: weight,
	}).Error)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"m-both":0.01}`))
	operation_setting.LoadSmartRoutesFromJSONString(smartRoutingBaseRoutes)
	model.InitChannelCache()
	model.RefreshPricing()

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		operation_setting.LoadSmartRoutesFromJSONString(originalRoutes)
		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

type validateSmartRouteResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Errors []struct {
			Field   string `json:"field"`
			Message struct {
				Key    string         `json:"key"`
				Params map[string]any `json:"params"`
			} `json:"message"`
		} `json:"errors"`
	} `json:"data"`
}

func postValidateSmartRoute(t *testing.T, body string) (*httptest.ResponseRecorder, validateSmartRouteResponse) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/option/smart_routing/validate", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	ValidateSmartRoute(ctx)

	var response validateSmartRouteResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return recorder, response
}

func TestValidateSmartRoute_Response(t *testing.T) {
	setupSmartRoutingControllerTest(t)

	recorder, valid := postValidateSmartRoute(t, fmt.Sprintf(`{"route":%s}`, smartRouteJSON("smart-a", 3)))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, valid.Success)
	assert.Contains(t, recorder.Body.String(), `"errors":[]`)
	assert.NotContains(t, recorder.Body.String(), `"endpoints"`)

	recorder, invalid := postValidateSmartRoute(t, fmt.Sprintf(`{"route":%s}`, smartRouteJSON("m-both", 0)))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, invalid.Success)
	assert.NotContains(t, recorder.Body.String(), `"endpoints"`)
	require.Len(t, invalid.Data.Errors, 2)
	assert.Equal(t, "name", invalid.Data.Errors[0].Field)
	assert.Equal(t, "{{name}} is a real model name. A virtual model cannot use the name of a real model", invalid.Data.Errors[0].Message.Key)
	assert.Equal(t, "m-both", invalid.Data.Errors[0].Message.Params["name"])
	assert.Equal(t, "advisor.after_dissatisfied", invalid.Data.Errors[1].Field)
	assert.Equal(t, "Enter a value between {{min}} and {{max}}", invalid.Data.Errors[1].Message.Key)
}

func TestUpdateRequestPolicy_SmartRoutesValidation(t *testing.T) {
	patch := func(routes string) (*httptest.ResponseRecorder, map[string]any) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		body, err := common.Marshal(map[string]any{"options": map[string]string{operation_setting.SmartRoutingRoutesOptionKey: routes}})
		require.NoError(t, err)
		ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/option/request_policy", bytes.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		UpdateRequestPolicy(ctx)
		var response map[string]any
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		return recorder, response
	}

	t.Run("invalid route is rejected and option unchanged", func(t *testing.T) {
		setupSmartRoutingControllerTest(t)
		before := model.CurrentRequestPolicy().Options[operation_setting.SmartRoutingRoutesOptionKey]

		_, response := patch("[" + smartRouteJSON("smart-a", -1) + "]")

		assert.Equal(t, false, response["success"])
		assert.Contains(t, fmt.Sprint(response["message"]), "smart-a")
		assert.Equal(t, before, model.CurrentRequestPolicy().Options[operation_setting.SmartRoutingRoutesOptionKey])
		assert.Equal(t, smartRoutingBaseRoutes, operation_setting.SmartRoutingRoutesToString())
		var count int64
		require.NoError(t, model.DB.Model(&model.Option{}).Where("key = ?", operation_setting.SmartRoutingRoutesOptionKey).Count(&count).Error)
		assert.Zero(t, count)
	})

	t.Run("valid route is saved", func(t *testing.T) {
		setupSmartRoutingControllerTest(t)

		_, response := patch("[" + smartRouteJSON("smart-a", 3) + "]")

		assert.Equal(t, true, response["success"])
		_, ok := operation_setting.FindSmartRoute("smart-a")
		assert.True(t, ok)
	})
}

func TestUpdateOption_SmartRoutesValidation(t *testing.T) {
	setupSmartRoutingControllerTest(t)
	seedQuotaResetOption(t, operation_setting.SmartRoutingRoutesOptionKey, smartRoutingBaseRoutes)

	recorder := putQuotaResetOption(t, operation_setting.SmartRoutingRoutesOptionKey, "["+smartRouteJSON("smart-a", -1)+"]")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.False(t, decodeQuotaResetOptionResponse(t, recorder).Success)
	assert.Equal(t, smartRoutingBaseRoutes, readOptionMapValue(t, operation_setting.SmartRoutingRoutesOptionKey))
	assert.Equal(t, smartRoutingBaseRoutes, operation_setting.SmartRoutingRoutesToString())
}

const removedThinkingCapabilitiesKey = "smart_routing_setting.thinking_capabilities"

func countOptionRows(t *testing.T, key string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.Option{}).Where("key = ?", key).Count(&count).Error)
	return count
}

func TestUpdateOption_RemovedThinkingCapabilitiesKeyRejected(t *testing.T) {
	setupSmartRoutingControllerTest(t)

	recorder := putQuotaResetOption(t, removedThinkingCapabilitiesKey, `{"gpt-6.2-sol":{"supported_efforts":["low"]}}`)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.False(t, decodeQuotaResetOptionResponse(t, recorder).Success)
	assert.Zero(t, countOptionRows(t, removedThinkingCapabilitiesKey))
}

func TestUpdateRequestPolicy_RemovedThinkingCapabilitiesKeyRejected(t *testing.T) {
	setupSmartRoutingControllerTest(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body, err := common.Marshal(map[string]any{"options": map[string]string{removedThinkingCapabilitiesKey: `{}`}})
	require.NoError(t, err)
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/option/request_policy", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	UpdateRequestPolicy(ctx)

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, false, response["success"])
	assert.Zero(t, countOptionRows(t, removedThinkingCapabilitiesKey))
}

func addSmartRoutingModel(t *testing.T, id, channelType int, modelName string) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: id, Type: channelType, Key: fmt.Sprintf("key-%d", id), Status: common.ChannelStatusEnabled,
		Name: fmt.Sprintf("channel-%d", id), Weight: &weight, Models: modelName, Group: "api", Priority: &priority,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group: "api", Model: modelName, ChannelId: id, Enabled: true, Priority: &priority, Weight: weight,
	}).Error)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"m-both":0.01,"m-chat":0.01,"m-resp":0.01}`))
	model.InitChannelCache()
	model.RefreshPricing()
}

func legacyRouteWithoutEndpoints(name, simpleModel string) string {
	route := strings.Replace(smartRouteJSON(name, 3), `"endpoints":["openai-response","openai"],`, "", 1)
	return strings.Replace(route, `"simple":{"model":"m-both"`, fmt.Sprintf(`"simple":{"model":%q`, simpleModel), 1)
}

func TestValidateSmartRoute_EndpointsFieldErrors(t *testing.T) {
	setupSmartRoutingControllerTest(t)
	addSmartRoutingModel(t, 2, constant.ChannelTypeOpenRouter, "m-chat")
	addSmartRoutingModel(t, 3, constant.ChannelTypeCodex, "m-resp")
	disjoint := strings.Replace(smartRouteJSON("smart-a", 3), `"endpoints":["openai-response","openai"]`, `"endpoints":[]`, 1)
	disjoint = strings.Replace(disjoint, `"simple":{"model":"m-both"`, `"simple":{"model":"m-resp"`, 1)
	disjoint = strings.Replace(disjoint, `"normal":{"model":"m-both"`, `"normal":{"model":"m-chat"`, 1)

	_, response := postValidateSmartRoute(t, fmt.Sprintf(`{"route":%s}`, disjoint))

	require.Len(t, response.Data.Errors, 1)
	assert.Equal(t, "endpoints", response.Data.Errors[0].Field)
	assert.Equal(t, "Select at least one endpoint", response.Data.Errors[0].Message.Key)
}

func TestUpdateRequestPolicy_SavesLegacyRouteWithoutEndpointsAlongsideCompleteRoute(t *testing.T) {
	setupSmartRoutingControllerTest(t)
	routes := "[" + legacyRouteWithoutEndpoints("smart-old", "m-both") + "," + smartRouteJSON("smart-new", 3) + "]"
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body, err := common.Marshal(map[string]any{"options": map[string]string{operation_setting.SmartRoutingRoutesOptionKey: routes}})
	require.NoError(t, err)
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/option/request_policy", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	UpdateRequestPolicy(ctx)

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, true, response["success"], recorder.Body.String())
	_, ok := operation_setting.FindSmartRoute("smart-old")
	assert.True(t, ok)
}

func TestUpdateOption_SavesLegacyRouteWithoutEndpointsAlongsideCompleteRoute(t *testing.T) {
	setupSmartRoutingControllerTest(t)
	seedQuotaResetOption(t, operation_setting.SmartRoutingRoutesOptionKey, smartRoutingBaseRoutes)
	routes := "[" + legacyRouteWithoutEndpoints("smart-old", "m-both") + "," + smartRouteJSON("smart-new", 3) + "]"

	recorder := putQuotaResetOption(t, operation_setting.SmartRoutingRoutesOptionKey, routes)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, decodeQuotaResetOptionResponse(t, recorder).Success, recorder.Body.String())
}

func TestGetSmartRouteThinkingCapabilities(t *testing.T) {
	setupSmartRoutingControllerTest(t)
	require.NoError(t, model.DB.Create(&model.Model{ModelName: "gpt-6.2-sol",
		Capabilities: `{"reasoning":{"supported_efforts":["none","low","medium","high","xhigh","max"]}}`}).Error)
	require.NoError(t, model.DB.Create(&model.Model{ModelName: "gpt-4o",
		Capabilities: `{"reasoning":{"supported_efforts":[]}}`}).Error)
	model.RefreshPricing()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/option/smart_routing/thinking_capabilities?models=gpt-6.2-sol,gpt-4o,foo-model,glm-5.3,claude-opus-5", nil)
	GetSmartRouteThinkingCapabilities(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    map[string]struct {
			Known      bool     `json:"known"`
			NoThinking bool     `json:"no_thinking"`
			Efforts    []string `json:"efforts"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 5)
	sol := response.Data["gpt-6.2-sol"]
	assert.True(t, sol.Known)
	assert.False(t, sol.NoThinking)
	assert.Equal(t, []string{"none", "low", "medium", "high", "xhigh", "max"}, sol.Efforts)
	assert.True(t, response.Data["gpt-4o"].Known)
	assert.True(t, response.Data["gpt-4o"].NoThinking)
	assert.False(t, response.Data["foo-model"].Known)
	assert.False(t, response.Data["glm-5.3"].Known)
	assert.Equal(t, []string{"low", "medium", "high"}, response.Data["glm-5.3"].Efforts)
	assert.True(t, response.Data["claude-opus-5"].Known)
	assert.NotEmpty(t, response.Data["claude-opus-5"].Efforts)
}
