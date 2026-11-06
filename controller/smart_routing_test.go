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

func TestThinkingCapabilitiesOption(t *testing.T) {
	const key = "smart_routing_setting.thinking_capabilities"
	const baseline = `{}`
	const valid = `{"gpt-5.5":{"no_thinking":false,"efforts":["low","high","xhigh"]},"gpt-4o":{"no_thinking":true,"efforts":[]}}`
	tests := []struct {
		name  string
		value string
		ok    bool
	}{
		{"valid table", valid, true},
		{"empty table", `{}`, true},
		{"no_thinking false with empty efforts", `{"gpt-5.5":{"no_thinking":false,"efforts":[]}}`, false},
		{"efforts contain unknown level", `{"gpt-5.5":{"no_thinking":false,"efforts":["low","ultra"]}}`, false},
		{"empty model name", `{"":{"no_thinking":false,"efforts":["low"]}}`, false},
		{"whitespace-only model name", `{" ":{"no_thinking":false,"efforts":["low"]}}`, false},
		{"not an object", `["gpt-5.5"]`, false},
	}
	viaRequestPolicy := func(t *testing.T, value string) bool {
		t.Helper()
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		body, err := common.Marshal(map[string]any{"options": map[string]string{key: value}})
		require.NoError(t, err)
		ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/option/request_policy", bytes.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		UpdateRequestPolicy(ctx)
		var response map[string]any
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		return response["success"] == true
	}
	viaUpdateOption := func(t *testing.T, value string) bool {
		t.Helper()
		seedQuotaResetOption(t, key, baseline)
		recorder := putQuotaResetOption(t, key, value)
		require.Equal(t, http.StatusOK, recorder.Code)
		return decodeQuotaResetOptionResponse(t, recorder).Success
	}
	channels := map[string]func(*testing.T, string) bool{
		"PATCH request_policy": viaRequestPolicy,
		"PUT option":           viaUpdateOption,
	}

	for channelName, save := range channels {
		for _, tt := range tests {
			t.Run(channelName+"/"+tt.name, func(t *testing.T) {
				setupSmartRoutingControllerTest(t)
				require.NoError(t, model.UpdateOption(key, baseline))
				t.Cleanup(func() { require.NoError(t, model.UpdateOption(key, baseline)) })
				before := operation_setting.GetThinkingCapabilityOverrides()

				success := save(t, tt.value)

				assert.Equal(t, tt.ok, success)
				if !tt.ok {
					assert.Equal(t, before, operation_setting.GetThinkingCapabilityOverrides())
					return
				}
				var want map[string]operation_setting.ThinkingCapabilityOverride
				require.NoError(t, common.UnmarshalJsonStr(tt.value, &want))
				got := operation_setting.GetThinkingCapabilityOverrides()
				assert.Len(t, got, len(want))
				for name, capability := range want {
					assert.Equal(t, capability.NoThinking, got[name].NoThinking, name)
					assert.ElementsMatch(t, capability.Efforts, got[name].Efforts, name)
				}
			})
		}
	}
}
