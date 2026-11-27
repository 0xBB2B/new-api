package middleware

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type streamedBodyStorage struct {
	common.BodyStorage
}

func (streamedBodyStorage) Bytes() ([]byte, error) {
	return nil, errors.New("disk body must be read through readers")
}

func TestJSONModelRoutingUsesReadersForDiskBodies(t *testing.T) {
	previous := common.GetDiskCacheConfig()
	t.Cleanup(func() { common.SetDiskCacheConfig(previous) })
	large := strings.Repeat("image-data", 32<<10)
	prefix := `{"model":"model-a",`
	acrossRead := prefix + strings.Repeat(" ", (64<<10)-len(prefix)-3) + `"\u006d\u006f\u0064\u0065\u006c":"model-b"}`
	for _, disk := range []bool{false, true} {
		storageName := "memory"
		if disk {
			storageName = "file"
		}
		t.Run(storageName, func(t *testing.T) {
			common.SetDiskCacheConfig(common.DiskCacheConfig{Enabled: disk, ThresholdMB: 0, MaxSizeMB: 16, Path: t.TempDir()})
			for _, tt := range []struct {
				name, body, model, group, wantError string
			}{
				{"large body", `{"model":"model-a","group":"g","stream":true,"background":false,"input":"` + large + `"}`, "model-a", "g", ""},
				{"fields after payload", `{"input":"` + large + `","model":"model-a","group":"g"}`, "model-a", "g", ""},
				{"escaped model key", `{"\u006d\u006f\u0064\u0065\u006c":"model-a"}`, "model-a", "", ""},
				{"nested models", `{"model":"model-a","nested":[{"model":"x"},{"model":"y"}],"text":"\"model\":false"}`, "model-a", "", ""},
				{"long unrelated key", `{"` + large + `":"x","model":"model-a"}`, "model-a", "", ""},
				{"duplicate model", `{"model":"model-a","model":"model-b"}`, "", "", "model must be provided once"},
				{"duplicate key across reads", acrossRead, "", "", "model must be provided once"},
				{"wrong model type", `{"model":false}`, "", "", "field model must be a string"},
				{"wrong group type", `{"model":"model-a","group":[]}`, "", "", "field group must be a string"},
				{"trailing JSON", `{"model":"model-a"} {}`, "", "", "invalid JSON request body"},
				{"truncated payload", `{"model":"model-a","input":"` + large, "", "", "invalid JSON request body"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					storage, err := common.CreateBodyStorage([]byte(tt.body))
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, storage.Close()) })
					require.Equal(t, disk, storage.IsDisk())
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(tt.body))
					c.Request.Header.Set("Content-Type", "application/json")
					if disk {
						c.Set(common.KeyBodyStorage, streamedBodyStorage{storage})
					} else {
						c.Set(common.KeyBodyStorage, storage)
					}
					request, err := getModelFromJSONBody(c)
					if tt.wantError != "" {
						require.EqualError(t, err, tt.wantError)
						return
					}
					require.NoError(t, err)
					assert.Equal(t, tt.model, request.Model)
					assert.Equal(t, tt.group, request.Group)
					if tt.name == "large body" {
						stream, background := jsonBodyBoolFlags(c)
						assert.True(t, stream)
						assert.False(t, background)
					}
					var decoded ModelRequest
					require.NoError(t, common.UnmarshalBodyReusableNoCopy(c, &decoded))
					assert.Equal(t, *request, decoded)
					replayed, err := io.ReadAll(c.Request.Body)
					require.NoError(t, err)
					assert.Equal(t, tt.body, string(replayed))
				})
			}
		})
	}
}

func TestChannelMatchesExpectedTaskPluginUsesGenericChannelSetting(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeTaskPlugin}
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "generic-alpha"})

	assert.True(t, channelMatchesExpectedTaskPlugin(nil, channel, "generic-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, "generic-beta"))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, ""))
}

func TestChannelMatchesExpectedTaskPluginUsesPinnedLegacyIndex(t *testing.T) {
	registry := jsplugin.NewRegistry()
	alpha, err := registry.Register(distributorTaskPluginSource("legacy-alpha", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)
	pinnedGeneration := registry.Generation()

	require.NoError(t, registry.Unregister("legacy-alpha"))
	_, err = registry.Register(distributorTaskPluginSource("legacy-beta", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{
		Generation: pinnedGeneration,
		Plugin:     alpha,
	})
	channel := &model.Channel{Type: constant.ChannelTypeKling}

	assert.True(t, channelMatchesExpectedTaskPlugin(c, channel, "legacy-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, channel, "legacy-beta"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "legacy-alpha"))
}

func TestChannelMatchesExpectedTaskPluginRejectsUnindexedLegacyChannel(t *testing.T) {
	registry := jsplugin.NewRegistry()
	plugin, err := registry.Register(distributorTaskPluginSource("legacy-alpha", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{
		Generation: registry.Generation(),
		Plugin:     plugin,
	})

	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "legacy-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: 0}, "legacy-alpha"))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, ""))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, &model.Channel{Type: constant.ChannelTypeKling}, "legacy-alpha"))

	c.Set("expected_task_plugin_key", "legacy-alpha")
	setupErr := SetupContextForSelectedChannel(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "task-model")
	require.NotNil(t, setupErr)
	assert.Contains(t, setupErr.Error(), "does not match")
}

func TestSharedEndpointRebindsToSelectedLegacyProvider(t *testing.T) {
	registry := jsplugin.NewRegistry()
	_, err := registry.Register(distributorEndpointPluginSource("gemini-shared", constant.ChannelTypeGemini), jsplugin.Options{})
	require.NoError(t, err)
	_, err = registry.Register(distributorEndpointPluginSource("vertex-shared", constant.ChannelTypeVertexAi), jsplugin.Options{})
	require.NoError(t, err)
	candidates := registry.Generation().LookupEndpointCandidates("POST", "/v1/responses", "task-model")
	require.Len(t, candidates, 2)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: registry.Generation(), Plugin: candidates[0].Plugin})
	c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{
		Generation: registry.Generation(),
		Plugin:     candidates[0].Plugin,
		Protocol:   candidates[0].Protocol,
		Operation:  candidates[0].Operation,
		Model:      "task-model",
		Candidates: candidates,
	})
	c.Set("expected_task_plugin_key", candidates[0].Plugin.Meta.Key)

	geminiChannel := &model.Channel{Id: 1, Type: constant.ChannelTypeGemini}
	vertexChannel := &model.Channel{Id: 2, Type: constant.ChannelTypeVertexAi}
	assert.True(t, channelMatchesExpectedTaskPlugin(c, geminiChannel, candidates[0].Plugin.Meta.Key))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, vertexChannel, candidates[0].Plugin.Meta.Key))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeKling}, candidates[0].Plugin.Meta.Key))

	require.Nil(t, SetupContextForSelectedChannel(c, vertexChannel, "task-model"))
	pinnedValue, exists := c.Get(jsplugin.ContextKeyPinnedEndpoint)
	require.True(t, exists)
	pinned, ok := pinnedValue.(jsplugin.PinnedEndpoint)
	require.True(t, ok)
	assert.Equal(t, "vertex-shared", pinned.Plugin.Meta.Key)
	assert.Equal(t, "vertex-shared", c.GetString("expected_task_plugin_key"))
	assert.Equal(t, "vertex-shared", c.GetString("task_plugin_key"))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, geminiChannel, "vertex-shared"), "a retry may select another declared provider")
}

func distributorTaskPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["task-model"],
  fetchMode: "per_task",
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
`, key, key, channelType)
}

func distributorEndpointPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["task-model"],
  fetchMode: "per_task",
  protocols: [{name: "openai_responses", supports: ["stream", "sync", "background"]}],
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
export const protocols = {openai_responses: {
  decodeRequest: function(ctx) { return {kind: "submit", model: "task-model", requestBody: ctx.body.value}; },
  renderEvents: function() { return {events: [], state: null, done: false}; },
  renderFinal: function() { return {output: []}; },
}};
`, key, key, channelType)
}

func TestTokenModelLimitAllowsLegacyAliasAndModifierVariant(t *testing.T) {
	aliasOnly := map[string]bool{"claude-3-7-sonnet-thinking": true}
	assert.True(t, TokenModelLimitAllows(aliasOnly, "claude-3-7-sonnet-thinking"))
	assert.False(t, TokenModelLimitAllows(aliasOnly, "claude-3-7-sonnet"))

	baseOnly := map[string]bool{"claude-3-7-sonnet": true}
	assert.True(t, TokenModelLimitAllows(baseOnly, "claude-3-7-sonnet@thinking:on"))
	assert.True(t, TokenModelLimitAllows(baseOnly, "claude-3-7-sonnet-thinking"))

	wildcard := map[string]bool{"gemini-2.5-flash-thinking-*": true}
	assert.True(t, TokenModelLimitAllows(wildcard, "gemini-2.5-flash-thinking-8192"))
}

// /v1/tools/search and /v1/tools/fetch always route and bill by the DeepSeek
// routing names. A model named in the body is ignored: a chat model there
// would price the call at zero tokens.
func TestGetModelRequestAssignsDeepSeekToolRoutingNames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ path, body, want string }{
		{"/v1/tools/search", `{"query":"deepseek","count":3}`, constant.DeepSeekSearchModel},
		{"/v1/tools/fetch", `{"url":"https://example.com/"}`, constant.DeepSeekFetchModel},
		{"/v1/tools/search", `{"query":"deepseek","model":"deepseek-flash"}`, constant.DeepSeekSearchModel},
		{"/v1/tools/fetch", `{"url":"https://example.com/","model":"deepseek-v4-pro"}`, constant.DeepSeekFetchModel},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		c.Request.Header.Set("Content-Type", "application/json")
		modelRequest, shouldSelectChannel, err := getModelRequest(c)
		require.NoError(t, err, tc.body)
		assert.True(t, shouldSelectChannel, tc.body)
		assert.Equal(t, tc.want, modelRequest.Model, tc.body)
	}
}

func TestTokenModelLimitAllowsExemptAtNameByFullName(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	original := append([]string(nil), settings.ThinkingModelBlacklist...)
	t.Cleanup(func() { settings.ThinkingModelBlacklist = original })
	settings.ThinkingModelBlacklist = append(original, "re:.*@sha256:.*")

	fullOnly := map[string]bool{"opaque@sha256:deadbeef": true}
	assert.True(t, TokenModelLimitAllows(fullOnly, "opaque@sha256:deadbeef"))

	baseOnly := map[string]bool{"opaque": true}
	assert.False(t, TokenModelLimitAllows(baseOnly, "opaque@sha256:deadbeef"))
}

func TestDistributeHidesTaskPluginDetailsButLogsDiagnostics(t *testing.T) {
	require.NoError(t, i18n.Init())
	previousCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = previousCacheEnabled })

	const group = "private-plugin-error-test-group"
	for _, locale := range []struct{ language, message string }{
		{"en", "No available channel for model task-model under group " + group + ": the model is claimed by a task plugin, which has no enabled channel serving it (distributor)"},
		{"zh-CN", "分组 " + group + " 下模型 task-model 无可用渠道：该模型由任务插件认领，但当前没有启用的渠道可服务此模型（distributor）"},
		{"zh-TW", "分組 " + group + " 下模型 task-model 無可用管道：該模型由任務插件認領，但目前沒有啟用的管道可服務此模型（distributor）"},
	} {
		for _, providerCount := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s/%d_providers", locale.language, providerCount), func(t *testing.T) {
				registry := jsplugin.NewRegistry()
				keys := []string{"private-provider-alpha", "private-provider-beta"}[:providerCount]
				for index, key := range keys {
					_, err := registry.Register(distributorEndpointPluginSource(key, constant.ChannelTypeKling+index), jsplugin.Options{})
					require.NoError(t, err)
				}
				generation := registry.Generation()
				candidates := generation.LookupEndpointCandidates("POST", "/v1/responses", "task-model")
				require.Len(t, candidates, providerCount)

				var logs bytes.Buffer
				common.LogWriterMu.Lock()
				previousWriter := gin.DefaultErrorWriter
				gin.DefaultErrorWriter = &logs
				common.LogWriterMu.Unlock()
				t.Cleanup(func() {
					common.LogWriterMu.Lock()
					gin.DefaultErrorWriter = previousWriter
					common.LogWriterMu.Unlock()
				})

				router := gin.New()
				router.POST("/v1/responses", RequestId(), func(c *gin.Context) {
					common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
					c.Set("resolved_task_model", "task-model")
					c.Set("expected_task_plugin_key", keys[0])
					c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: generation, Plugin: candidates[0].Plugin})
					if providerCount > 1 {
						c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{
							Generation: generation, Plugin: candidates[0].Plugin,
							Protocol: candidates[0].Protocol, Operation: candidates[0].Operation,
							Model: "task-model", Candidates: candidates,
						})
					}
				}, Distribute(), func(c *gin.Context) {
					t.Error("unavailable requests must stop before the relay handler")
					c.Status(http.StatusNoContent)
				})
				request := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				request.Header.Set("Accept-Language", locale.language)
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)

				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
				requestID := recorder.Header().Get(common.RequestIdKey)
				require.NotEmpty(t, requestID)
				assert.JSONEq(t, fmt.Sprintf(`{"error":{"message":%q,"type":"new_api_error","code":"model_not_found"}}`,
					locale.message+" (request id: "+requestID+")"), recorder.Body.String())
				assert.NotContains(t, recorder.Body.String(), "disable or override")
				for _, key := range keys {
					assert.NotContains(t, recorder.Body.String(), key)
					assert.Contains(t, logs.String(), key)
				}
				assert.Contains(t, logs.String(), requestID)
				assert.Contains(t, logs.String(), `group="`+group+`"`)
				assert.Contains(t, logs.String(), `model="task-model"`)
				assert.Contains(t, logs.String(), "reason=no_eligible_channel")
			})
		}
	}
}

func TestNoAvailableChannelMessageWithoutPlugin(t *testing.T) {
	require.NoError(t, i18n.Init())
	plain, _ := gin.CreateTestContext(nil)
	plain.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	plain.Request.Header.Set("Accept-Language", "en")
	generic := noAvailableChannelMessage(plain, "default", "gpt-4o")
	assert.NotContains(t, generic, "task plugin")
	assert.Contains(t, generic, "gpt-4o")
}

func TestSharedEndpointRebindsToSelectedType61Plugin(t *testing.T) {
	registry := jsplugin.NewRegistry()
	for _, key := range []string{"alpha", "beta"} {
		source := strings.Replace(distributorEndpointPluginSource(key, 0), "channelTypes: [0],", "", 1)
		_, err := registry.Register(source, jsplugin.Options{})
		require.NoError(t, err)
	}
	generation := registry.Generation()
	candidates := generation.LookupEndpointCandidates("POST", "/v1/responses", "task-model")
	require.Len(t, candidates, 2)
	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: generation, Plugin: candidates[0].Plugin})
	c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{Generation: generation, Plugin: candidates[0].Plugin, Protocol: candidates[0].Protocol, Operation: candidates[0].Operation, Model: "task-model", Candidates: candidates})
	c.Set("expected_task_plugin_key", "alpha")
	channel := &model.Channel{Id: 2, Type: constant.ChannelTypeTaskPlugin}
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "unrelated"})
	assert.False(t, channelMatchesExpectedTaskPlugin(c, channel, "alpha"))
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "beta"})
	require.Nil(t, SetupContextForSelectedChannel(c, channel, "task-model"))
	assert.Equal(t, "beta", c.GetString("task_plugin_key"))
	assert.Equal(t, "beta", c.GetString("expected_task_plugin_key"))
	assert.Equal(t, "beta", c.MustGet(jsplugin.ContextKeyPinnedEndpoint).(jsplugin.PinnedEndpoint).Plugin.Meta.Key)
}

// On a native path two plugins declare, the selected channel names the
// executing plugin, which takes over the route and the task identity.
func TestSharedNativeRouteRebindsToSelectedPlugin(t *testing.T) {
	registry := jsplugin.NewRegistry()
	for _, key := range []string{"route-alpha", "route-beta"} {
		_, err := registry.Register(fmt.Sprintf(`
export const meta = {apiVersion: 1, key: %q, name: %q, version: "1.0.0", author: {name: "Test"}, models: ["task-model"], fetchMode: "per_task",
	routes: [{method: "POST", path: "/probe/v1/jobs", type: "submit", decode: "decode", render: "render"}]};
export const native = {decode() { return {}; }, render() { return {}; }};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {}; }
`, key, key), jsplugin.Options{})
		require.NoError(t, err)
	}
	generation := registry.Generation()
	candidates := generation.LookupDeclaredRoutes("POST", "/probe/v1/jobs")
	require.Len(t, candidates, 2)
	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: generation, Plugin: candidates[0].Plugin})
	c.Set(jsplugin.ContextKeyPinnedRoute, jsplugin.PinnedRoute{Generation: generation, Plugin: candidates[0].Plugin, Route: candidates[0].Route, Candidates: candidates})
	c.Set("expected_task_plugin_key", "route-alpha")
	channel := &model.Channel{Id: 3, Type: constant.ChannelTypeTaskPlugin}
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "unrelated"})
	assert.False(t, channelMatchesExpectedTaskPlugin(c, channel, "route-alpha"))
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "route-beta"})

	require.Nil(t, SetupContextForSelectedChannel(c, channel, "task-model"))
	pinned := c.MustGet(jsplugin.ContextKeyPinnedRoute).(jsplugin.PinnedRoute)
	assert.Same(t, candidates[1].Plugin, pinned.Plugin)
	assert.Equal(t, candidates[1].Route, pinned.Route)
	assert.Same(t, candidates[1].Plugin, c.MustGet(jsplugin.ContextKeyPinnedPlugin).(jsplugin.PinnedPlugin).Plugin)
	for _, key := range []string{"expected_task_plugin_key", "task_plugin_key", "platform"} {
		assert.Equal(t, "route-beta", c.GetString(key), key)
	}
}

func TestChannelMatchesExpectedTaskPluginUsesNewAPIExtensions(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeNewAPI}
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "single", TaskExtendPluginKeys: []string{"ext-alpha", "ext-beta"}})

	assert.True(t, channelMatchesExpectedTaskPlugin(nil, channel, "ext-alpha"))
	assert.True(t, channelMatchesExpectedTaskPlugin(nil, channel, "single"), "the single key stays valid on a New API channel")
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, "ext-gamma"))
	assert.True(t, channelMatchesExpectedTaskPlugin(nil, channel, ""), "requests without a pinned plugin still use the gateway")
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, &model.Channel{Type: constant.ChannelTypeNewAPI}, "ext-alpha"))
}

func TestSharedEndpointRebindsToBoundNewAPIExtension(t *testing.T) {
	registry := jsplugin.NewRegistry()
	for _, key := range []string{"alpha", "beta"} {
		source := strings.Replace(distributorEndpointPluginSource(key, 0), "channelTypes: [0],", "", 1)
		_, err := registry.Register(source, jsplugin.Options{})
		require.NoError(t, err)
	}
	generation := registry.Generation()
	candidates := generation.LookupEndpointCandidates("POST", "/v1/responses", "task-model")
	require.Len(t, candidates, 2)
	pin := func(expected string) *gin.Context {
		pinned := candidates[0]
		if candidates[1].Plugin.Meta.Key == expected {
			pinned = candidates[1]
		}
		c, _ := gin.CreateTestContext(nil)
		c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: generation, Plugin: pinned.Plugin})
		c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{Generation: generation, Plugin: pinned.Plugin, Protocol: pinned.Protocol, Operation: pinned.Operation, Model: "task-model", Candidates: candidates})
		c.Set("expected_task_plugin_key", expected)
		return c
	}

	channel := &model.Channel{Id: 3, Type: constant.ChannelTypeNewAPI}
	channel.SetSetting(dto.ChannelSettings{TaskExtendPluginKeys: []string{"unrelated"}})
	assert.False(t, channelMatchesExpectedTaskPlugin(pin("alpha"), channel, "alpha"))

	channel.SetSetting(dto.ChannelSettings{TaskExtendPluginKeys: []string{"beta"}})
	c := pin("alpha")
	require.Nil(t, SetupContextForSelectedChannel(c, channel, "task-model"))
	assert.Equal(t, "beta", c.GetString("task_plugin_key"), "the only bound candidate executes")
	assert.Equal(t, "beta", c.GetString("expected_task_plugin_key"))
	assert.Equal(t, "beta", c.MustGet(jsplugin.ContextKeyPinnedEndpoint).(jsplugin.PinnedEndpoint).Plugin.Meta.Key)

	channel.SetSetting(dto.ChannelSettings{TaskExtendPluginKeys: []string{"alpha", "beta"}})
	c = pin("beta")
	require.Nil(t, SetupContextForSelectedChannel(c, channel, "task-model"))
	assert.Equal(t, "alpha", c.GetString("task_plugin_key"), "the first bound candidate executes regardless of the earlier pin")
	assert.Equal(t, "alpha", c.MustGet(jsplugin.ContextKeyPinnedEndpoint).(jsplugin.PinnedEndpoint).Plugin.Meta.Key)
}

const smartRoutesJSON = `[
 {"name":"auto-gpt","endpoints":["openai-response","openai","anthropic"],"tiers":{
   "simple":{"model":"t-simple","groups":["default"],"effort":""},
   "normal":{"model":"t-normal","groups":["default"],"effort":""},
   "hard":{"model":"t-hard","groups":["vip"],"effort_mode":"fixed","effort":"high"}},
  "default_tier":"normal","scorer":{"model":"m-scorer","groups":["default"],"timeout_ms":2000},
  "advisor":{"model":"t-advisor","groups":["default"],"after_dissatisfied":3},
  "downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500},
 {"name":"auto-keep","endpoints":["openai-response","openai","anthropic"],"tiers":{
   "simple":{"model":"t-simple","groups":["default"],"effort":""},
   "normal":{"model":"t-normal","groups":["default"],"effort":""},
   "hard":{"model":"t-hard","groups":["vip"],"effort":""}},
  "default_tier":"normal","scorer":{"model":"m-scorer","groups":["default"],"timeout_ms":2000},
  "advisor":{"model":"t-advisor","groups":["default"],"after_dissatisfied":1},
  "downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500},
 {"name":"resp-only","endpoints":["openai-response"],"tiers":{
   "simple":{"model":"r-resp","groups":["default"],"effort":""},
   "normal":{"model":"r-resp","groups":["default"],"effort":""},
   "hard":{"model":"r-resp","groups":["default"],"effort":""}},
  "default_tier":"normal","scorer":{"model":"m-scorer","groups":["default"],"timeout_ms":2000},
  "advisor":{"model":"r-resp","groups":["default"],"after_dissatisfied":3},
  "downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500},
 {"name":"auto-adv","endpoints":["openai-response","openai","anthropic"],"tiers":{
   "simple":{"model":"t-simple","groups":["default"],"effort":""},
   "normal":{"model":"t-normal","groups":["default"],"effort":""},
   "hard":{"model":"t-hard","groups":["vip"],"effort":""}},
  "default_tier":"normal","scorer":{"model":"m-scorer","groups":["default"],"timeout_ms":2000},
  "advisor":{"model":"t-advisor-hidden","groups":["hidden"],"after_dissatisfied":1},
  "downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500},
 {"name":"resp-anthropic","endpoints":["openai-response","anthropic"],"tiers":{
   "simple":{"model":"t-simple","groups":["default"],"effort":""},
   "normal":{"model":"t-normal","groups":["default"],"effort":""},
   "hard":{"model":"t-normal","groups":["default"],"effort":""}},
  "default_tier":"normal","scorer":{"model":"m-scorer","groups":["default"],"timeout_ms":2000},
  "advisor":{"model":"t-advisor","groups":["default"],"after_dissatisfied":3},
  "downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500},
 {"name":"chat-only","endpoints":["openai"],"tiers":{
   "simple":{"model":"c-chat","groups":["default"],"effort":""},
   "normal":{"model":"c-chat","groups":["default"],"effort":""},
   "hard":{"model":"c-chat","groups":["default"],"effort":""}},
  "default_tier":"normal","scorer":{"model":"m-scorer","groups":["default"],"timeout_ms":2000},
  "advisor":{"model":"c-chat","groups":["default"],"after_dissatisfied":3},
  "downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500}
]`

const (
	smartHardChannelID    = 103
	smartNormalChannelID  = 102
	smartAdvisorChannelID = 104
)

func setupSmartRouteDistributeTest(t *testing.T) {
	t.Helper()
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)

	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalRoutes := operation_setting.SmartRoutingRoutesToString()
	originalRunner := smartroute.ScorerRunner

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`[]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))

	priority := int64(0)
	weight := uint(100)
	for _, ch := range []struct {
		id          int
		channelType int
		group       string
		modelName   string
	}{
		{101, constant.ChannelTypeOpenAI, "default", "t-simple"},
		{smartNormalChannelID, constant.ChannelTypeOpenAI, "default", "t-normal"},
		{smartHardChannelID, constant.ChannelTypeOpenAI, "vip", "t-hard"},
		{smartAdvisorChannelID, constant.ChannelTypeOpenAI, "default", "t-advisor"},
		{105, constant.ChannelTypeOpenAI, "default", "m-scorer"},
		{106, constant.ChannelTypeCodex, "default", "r-resp"},
		{107, constant.ChannelTypeOpenAI, "hidden", "t-advisor-hidden"},
		{108, constant.ChannelTypeOpenRouter, "default", "c-chat"},
	} {
		require.NoError(t, db.Create(&model.Channel{
			Id: ch.id, Type: ch.channelType, Key: fmt.Sprintf("key-%d", ch.id),
			Status: common.ChannelStatusEnabled, Name: fmt.Sprintf("channel-%d", ch.id),
			Weight: &weight, Models: ch.modelName, Group: ch.group, Priority: &priority,
		}).Error)
		require.NoError(t, db.Create(&model.Ability{
			Group: ch.group, Model: ch.modelName, ChannelId: ch.id,
			Enabled: true, Priority: &priority, Weight: weight,
		}).Error)
	}
	operation_setting.LoadSmartRoutesFromJSONString(smartRoutesJSON)
	model.InitChannelCache()
	model.RefreshPricing()

	t.Cleanup(func() {
		smartroute.ScorerRunner = originalRunner
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		operation_setting.LoadSmartRoutesFromJSONString(originalRoutes)
		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		if sqlDB, err := db.DB(); err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
}

type smartScorerStub struct {
	calls   int
	settles [][2]string
}

func installSmartScorerStub(t *testing.T, labels *smartroute.ScorerLabels, failure string, err error) *smartScorerStub {
	t.Helper()
	stub := &smartScorerStub{}
	smartroute.ScorerRunner = func(*gin.Context, operation_setting.SmartRoute, smartroute.Inspection) (smartroute.ScorerOutcome, error) {
		stub.calls++
		return smartroute.ScorerOutcome{
			Labels:  labels,
			Failure: failure,
			Settle: func(mainTier, mainModel string) {
				stub.settles = append(stub.settles, [2]string{mainTier, mainModel})
			},
		}, err
	}
	return stub
}

func smartLabels(taskType, reasoning string, feedback ...string) *smartroute.ScorerLabels {
	return &smartroute.ScorerLabels{
		TaskType: taskType, Reasoning: reasoning, Workload: "few_steps",
		Risk: "low", Domain: "other", Confidence: 0.9, Feedback: feedback,
	}
}

func smartScored(labels *smartroute.ScorerLabels, score float64) *smartroute.ScorerLabels {
	labels.EffortScore = &score
	return labels
}

type smartDistributeSeen struct {
	ran           bool
	originalModel string
	autoGroups    []string
	channelID     int
	body          []byte
	routeLog      map[string]any
}

type smartRequest struct {
	path    string
	body    string
	auto    bool
	tokenID int
	userID  int
	status  int

	modelLimit map[string]bool
}

func serveSmartRoute(t *testing.T, req smartRequest) (*httptest.ResponseRecorder, *smartDistributeSeen) {
	t.Helper()
	seen := &smartDistributeSeen{}
	router := gin.New()
	router.POST(req.path, RequestId(), func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenId, req.tokenID)
		if req.userID != 0 {
			common.SetContextKey(c, constant.ContextKeyUserId, req.userID)
		}
		if req.modelLimit != nil {
			common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
			common.SetContextKey(c, constant.ContextKeyTokenModelLimit, req.modelLimit)
		}
		if req.auto {
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "auto")
			common.SetContextKey(c, constant.ContextKeyTokenAutoGroups, []string{"default", "vip"})
		} else {
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		}
	}, Distribute(), func(c *gin.Context) {
		seen.ran = true
		seen.originalModel = common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
		seen.autoGroups = common.GetContextKeyStringSlice(c, constant.ContextKeyTokenAutoGroups)
		seen.channelID = common.GetContextKeyInt(c, constant.ContextKeyChannelId)
		if fields, ok := common.GetContextKey(c, constant.ContextKeySmartRouteLog); ok {
			seen.routeLog, _ = fields.(map[string]any)
		}
		storage, err := common.GetBodyStorage(c)
		require.NoError(t, err)
		seen.body, err = storage.Bytes()
		require.NoError(t, err)
		status := req.status
		if status == 0 {
			status = http.StatusOK
		}
		c.Status(status)
	})
	httpReq := httptest.NewRequest(http.MethodPost, req.path, strings.NewReader(req.body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept-Language", "en")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httpReq)
	return recorder, seen
}

func smartBodyField(t *testing.T, body []byte, path ...string) any {
	t.Helper()
	var node any
	require.NoError(t, common.Unmarshal(body, &node))
	for _, key := range path {
		m, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node = m[key]
	}
	return node
}

func TestSmartRouteDistributeAutoTokenHardTierRewritesModelGroupsAndEffort(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		effortPath       []string
	}{
		{"responses", "/v1/responses", `{"model":"auto-gpt","input":"design a billing system","stream":false}`, []string{"reasoning", "effort"}},
		{"chat", "/v1/chat/completions", `{"model":"auto-gpt","messages":[{"role":"user","content":"design a billing system"}]}`, []string{"reasoning_effort"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSmartRouteDistributeTest(t)
			stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{path: tc.path, body: tc.body, auto: true, tokenID: 9101})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.True(t, seen.ran)
			assert.Equal(t, "t-hard", seen.originalModel)
			assert.Equal(t, []string{"vip"}, seen.autoGroups)
			assert.Equal(t, smartHardChannelID, seen.channelID)
			assert.Equal(t, "t-hard", smartBodyField(t, seen.body, "model"))
			assert.Equal(t, "high", smartBodyField(t, seen.body, tc.effortPath...))
			assert.Equal(t, 1, stub.calls)
			assert.Equal(t, [][2]string{{"hard", "t-hard"}}, stub.settles)
		})
	}
}

func TestSmartRouteDistributeEmptyTierEffortKeepsClientValue(t *testing.T) {
	t.Run("tier", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		_, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/responses", auto: true, tokenID: 9102,
			body: `{"model":"auto-keep","input":"design a billing system","reasoning":{"effort":"low"}}`,
		})

		require.True(t, seen.ran)
		assert.Equal(t, "t-hard", seen.originalModel)
		assert.Equal(t, "low", smartBodyField(t, seen.body, "reasoning", "effort"))
	})

	t.Run("advisor", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		installSmartScorerStub(t, smartLabels("chat_qa", "low", "dissatisfied"), "", nil)

		_, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/responses", auto: true, tokenID: 9103,
			body: `{"model":"auto-keep","input":"that answer was wrong","reasoning":{"effort":"low"}}`,
		})

		require.True(t, seen.ran)
		assert.Equal(t, "t-advisor", seen.originalModel)
		assert.Equal(t, smartAdvisorChannelID, seen.channelID)
		assert.Equal(t, "low", smartBodyField(t, seen.body, "reasoning", "effort"))
	})
}

func TestSmartRouteDistributeScorerInvalidStillSettles(t *testing.T) {
	setupSmartRouteDistributeTest(t)
	stub := installSmartScorerStub(t, nil, smartroute.FailureScorerInvalid, nil)

	recorder, seen := serveSmartRoute(t, smartRequest{
		path: "/v1/responses", auto: true, tokenID: 9104,
		body: `{"model":"auto-gpt","input":"hello there"}`,
	})

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "t-normal", seen.originalModel)
	assert.Equal(t, [][2]string{{"normal", "t-normal"}}, stub.settles)
}

func TestSmartRouteDistributeRejectsUnsupportedEndpoint(t *testing.T) {
	setupSmartRouteDistributeTest(t)
	stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

	recorder, seen := serveSmartRoute(t, smartRequest{
		path: "/v1/chat/completions", auto: false, tokenID: 9105,
		body: `{"model":"resp-only","messages":[{"role":"user","content":"hi"}]}`,
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.Contains(t, smartBodyField(t, recorder.Body.Bytes(), "error", "message"), "Virtual model resp-only only supports these endpoints: Responses")
	assert.False(t, seen.ran)
	assert.Zero(t, stub.calls)
	assert.Empty(t, stub.settles)
}

func TestSmartRouteDistributeRejectsEndpointOutsideConfiguredList(t *testing.T) {
	setupSmartRouteDistributeTest(t)
	stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

	recorder, seen := serveSmartRoute(t, smartRequest{
		path: "/v1/chat/completions", auto: false, tokenID: 9106,
		body: `{"model":"resp-anthropic","messages":[{"role":"user","content":"hi"}]}`,
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.Contains(t, smartBodyField(t, recorder.Body.Bytes(), "error", "message"), "Virtual model resp-anthropic only supports these endpoints: Responses, Anthropic")
	assert.False(t, seen.ran)
	assert.Zero(t, stub.calls)
	assert.Empty(t, stub.settles)
}

func TestSmartRouteDistributeNonAutoTokenFallsBackToDefaultTierWithoutChannel(t *testing.T) {
	setupSmartRouteDistributeTest(t)
	stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

	recorder, seen := serveSmartRoute(t, smartRequest{
		path: "/v1/responses", auto: false, tokenID: 9106,
		body: `{"model":"auto-gpt","input":"design a billing system"}`,
	})

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "t-normal", seen.originalModel)
	assert.Equal(t, smartNormalChannelID, seen.channelID)
	assert.Equal(t, "t-normal", smartBodyField(t, seen.body, "model"))
	assert.Equal(t, [][2]string{{"normal", "t-normal"}}, stub.settles)
}

func TestSmartRouteDistributeInsufficientScorerQuotaRejectsRequest(t *testing.T) {
	setupSmartRouteDistributeTest(t)
	stub := installSmartScorerStub(t, nil, "", fmt.Errorf("%w: %w", smartroute.ErrScorerInsufficientQuota,
		types.NewErrorWithStatusCode(errors.New("insufficient quota"), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden)))

	recorder, seen := serveSmartRoute(t, smartRequest{
		path: "/v1/responses", auto: true, tokenID: 9107,
		body: `{"model":"auto-gpt","input":"hello there"}`,
	})

	assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	assert.Equal(t, "insufficient_user_quota", smartBodyField(t, recorder.Body.Bytes(), "error", "code"))
	assert.False(t, seen.ran)
	assert.Empty(t, stub.settles)
}

func TestSmartRouteDistributeToolResultContinuationSkipsScorer(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
	}{
		{"responses", "/v1/responses", `{"model":"auto-gpt","input":[{"role":"user","content":"continue-resp"},{"type":"function_call_output","call_id":"c1","output":"ok"}]}`},
		{"chat", "/v1/chat/completions", `{"model":"auto-gpt","messages":[{"role":"user","content":"continue-chat"},{"role":"tool","tool_call_id":"c1","content":"ok"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSmartRouteDistributeTest(t)
			stub := installSmartScorerStub(t, smartLabels("chat_qa", "low", "neutral"), "", nil)
			const tokenID = 9108
			endpoint := smartroute.RequestEndpointFromPath(tc.path)
			inspection, err := smartroute.InspectRequest(endpoint, []byte(tc.body))
			require.NoError(t, err)
			require.False(t, inspection.IsUserTurn)
			smartroute.SaveSessionAfterSuccess(smartroute.SessionKey(tokenID, inspection), true, "hard", smartroute.EffortDecision{})

			recorder, seen := serveSmartRoute(t, smartRequest{path: tc.path, body: tc.body, auto: true, tokenID: tokenID})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Zero(t, stub.calls)
			assert.Equal(t, "t-hard", seen.originalModel)
			assert.Equal(t, []string{"vip"}, seen.autoGroups)
		})
	}
}

func TestSmartRouteDistributeSavesSessionOnlyAfterSuccess(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		message string
		tokenID int
		saved   bool
	}{
		{"success", http.StatusOK, "session-save-ok", 9109, true},
		{"failure", http.StatusInternalServerError, "session-save-fail", 9110, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSmartRouteDistributeTest(t)
			installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)
			body := fmt.Sprintf(`{"model":"auto-gpt","input":%q}`, tc.message)
			inspection, err := smartroute.InspectRequest(smartroute.RequestEndpointFromPath("/v1/responses"), []byte(body))
			require.NoError(t, err)
			key := smartroute.SessionKey(tc.tokenID, inspection)
			require.NotEmpty(t, key)

			serveSmartRoute(t, smartRequest{path: "/v1/responses", body: body, auto: true, tokenID: tc.tokenID, status: tc.status})

			state, found := smartroute.LoadSession(key)
			require.Equal(t, tc.saved, found)
			if tc.saved {
				assert.Equal(t, "hard", state.CurrentRoute)
				assert.Equal(t, "hard", state.LastNonAdvisor)
			}
		})
	}
}

func TestSmartRouteDistributeOrdinaryModelIsUntouched(t *testing.T) {
	setupSmartRouteDistributeTest(t)
	stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)
	body := `{"model":"t-normal","input":"hello"}`

	recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/responses", body: body, auto: false, tokenID: 9111})

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Zero(t, stub.calls)
	assert.Empty(t, stub.settles)
	assert.Equal(t, "t-normal", seen.originalModel)
	assert.Equal(t, body, string(seen.body))
}

func TestSmartRouteDistributeAdvisorWithoutChannelUsesFallbackTier(t *testing.T) {
	setupSmartRouteDistributeTest(t)
	stub := installSmartScorerStub(t, smartLabels("chat_qa", "low", "dissatisfied"), "", nil)

	recorder, seen := serveSmartRoute(t, smartRequest{
		path: "/v1/responses", auto: true, tokenID: 9112,
		body: `{"model":"auto-adv","input":"that answer was wrong"}`,
	})

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "t-normal", seen.originalModel)
	assert.Equal(t, "t-normal", smartBodyField(t, seen.body, "model"))
	assert.Equal(t, [][2]string{{"normal", "t-normal"}}, stub.settles)
}

func TestSmartRouteDistributeContinuationWithDeadAdvisorUsesLastNonAdvisorTier(t *testing.T) {
	setupSmartRouteDistributeTest(t)
	stub := installSmartScorerStub(t, nil, "", nil)
	const tokenID = 9113
	body := `{"model":"auto-adv","input":[{"role":"user","content":"continue-dead-advisor"},{"type":"function_call_output","call_id":"c1","output":"ok"}]}`
	inspection, err := smartroute.InspectRequest(smartroute.RequestEndpointFromPath("/v1/responses"), []byte(body))
	require.NoError(t, err)
	key := smartroute.SessionKey(tokenID, inspection)
	smartroute.SaveSessionAfterSuccess(key, true, "hard", smartroute.EffortDecision{})
	smartroute.SaveSessionAfterSuccess(key, true, smartroute.RouteAdvisor, smartroute.EffortDecision{})

	recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/responses", body: body, auto: true, tokenID: tokenID})

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Zero(t, stub.calls)
	assert.Equal(t, "t-hard", seen.originalModel)
	assert.Equal(t, "t-hard", smartBodyField(t, seen.body, "model"))
}

func TestSmartRouteDistributeRejectsDuplicateEffortKey(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"responses", "/v1/responses", `{"model":"auto-gpt","input":"design a billing system","reasoning":{"effort":"medium"},"reasoning":{"effort":"low"}}`},
		{"chat", "/v1/chat/completions", `{"model":"auto-gpt","messages":[{"role":"user","content":"design a billing system"}],"reasoning_effort":"medium","reasoning_effort":"low"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSmartRouteDistributeTest(t)
			stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{path: tc.path, body: tc.body, auto: true, tokenID: 9114})

			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			assert.False(t, seen.ran)
			assert.Zero(t, stub.calls)
			assert.Empty(t, stub.settles)
		})
	}
}

func TestSmartRouteDistributeTokenModelLimitChecksVirtualModelName(t *testing.T) {
	for _, tc := range []struct {
		name   string
		limit  map[string]bool
		status int
		model  string
	}{
		{"virtual name only", map[string]bool{"auto-gpt": true}, http.StatusOK, "t-hard"},
		{"real model only", map[string]bool{"t-hard": true}, http.StatusForbidden, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSmartRouteDistributeTest(t)
			installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/responses", auto: true, tokenID: 9115, modelLimit: tc.limit,
				body: `{"model":"auto-gpt","input":"design a billing system"}`,
			})

			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			assert.Equal(t, tc.model, seen.originalModel)
			assert.Equal(t, tc.status == http.StatusOK, seen.ran)
		})
	}
}

func TestSmartRouteDistributeCompactEndpointNeverScores(t *testing.T) {
	const tokenID = 9116
	compactBody := func(cacheKey string) string {
		return fmt.Sprintf(`{"model":"auto-gpt","input":[{"role":"user","content":"compact me"}],"prompt_cache_key":%q}`, cacheKey)
	}
	for _, tc := range []struct {
		name        string
		cacheKey    string
		sessionTier string
		wantModel   string
	}{
		{"session hard", "pck-compact-hard", "hard", "t-hard"},
		{"no session uses default tier", "pck-compact-none", "", "t-normal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSmartRouteDistributeTest(t)
			stub := installSmartScorerStub(t, smartLabels("chat_qa", "low", "neutral"), "", nil)
			body := compactBody(tc.cacheKey)
			if tc.sessionTier != "" {
				inspection, err := smartroute.InspectRequest(smartroute.RequestEndpointFromPath("/v1/responses"), []byte(body))
				require.NoError(t, err)
				smartroute.SaveSessionAfterSuccess(smartroute.SessionKey(tokenID, inspection), true, tc.sessionTier, smartroute.EffortDecision{})
			}

			recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/responses/compact", body: body, auto: true, tokenID: tokenID})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Zero(t, stub.calls)
			assert.Equal(t, tc.wantModel, seen.originalModel)
			assert.Equal(t, tc.wantModel, smartBodyField(t, seen.body, "model"))
			if tc.sessionTier != "" {
				inspection, err := smartroute.InspectRequest(smartroute.RequestEndpointFromPath("/v1/responses"), []byte(body))
				require.NoError(t, err)
				state, found := smartroute.LoadSession(smartroute.SessionKey(tokenID, inspection))
				require.True(t, found)
				assert.Equal(t, tc.sessionTier, state.CurrentRoute)
			}
		})
	}

	t.Run("chat only virtual model", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		stub := installSmartScorerStub(t, smartLabels("chat_qa", "low", "neutral"), "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/responses/compact", auto: false, tokenID: tokenID,
			body: `{"model":"chat-only","input":[{"role":"user","content":"compact me"}]}`,
		})

		assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		assert.False(t, seen.ran)
		assert.Zero(t, stub.calls)
	})
}

func TestSmartRouteDistributeEffortOverrideCoversReasoningObject(t *testing.T) {
	t.Run("chat drops the reasoning object", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/chat/completions", auto: true, tokenID: 9117,
			body: `{"model":"auto-gpt","messages":[{"role":"user","content":"design a billing system"}],"reasoning":{"effort":"low","max_tokens":100}}`,
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, "high", smartBodyField(t, seen.body, "reasoning_effort"))
		assert.NotContains(t, string(seen.body), `"reasoning":`)
	})

	t.Run("chat drops the whole reasoning object even with duplicate keys", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/chat/completions", auto: true, tokenID: 9119,
			body: `{"model":"auto-gpt","messages":[{"role":"user","content":"design a billing system"}],"reasoning":{"effort":"low","effort":"low","max_tokens":100,"exclude":true}}`,
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, "high", smartBodyField(t, seen.body, "reasoning_effort"))
		assert.Nil(t, smartBodyField(t, seen.body, "reasoning"))
		assert.Zero(t, strings.Count(string(seen.body), `"effort"`))
		assert.NotContains(t, string(seen.body), `"max_tokens"`)
		assert.NotContains(t, string(seen.body), `"exclude"`)
	})

	t.Run("chat drops differently cased reasoning keys", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/chat/completions", auto: true, tokenID: 9120,
			body: `{"model":"auto-gpt","messages":[{"role":"user","content":"design a billing system"}],"reasoning":{"MAX_TOKENS":30000,"Effort":"xhigh"}}`,
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, "high", smartBodyField(t, seen.body, "reasoning_effort"))
		assert.Nil(t, smartBodyField(t, seen.body, "reasoning"))
		assert.NotContains(t, strings.ToLower(string(seen.body)), `"max_tokens"`)
		assert.NotContains(t, string(seen.body), `"Effort"`)
	})

	t.Run("responses rewrites the whole reasoning object", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/responses", auto: true, tokenID: 9118,
			body: `{"model":"auto-gpt","input":"design a billing system","reasoning":{"effort":"low","effort":"low","summary":"auto"}}`,
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, 1, strings.Count(string(seen.body), `"effort"`))
		assert.Equal(t, "high", smartBodyField(t, seen.body, "reasoning", "effort"))
		assert.Equal(t, "auto", smartBodyField(t, seen.body, "reasoning", "summary"))
	})
}

var smartEffortTokenSeq = 9300

const (
	effortFollowMediumXhigh = `"effort_mode":"follow","effort_min":"medium","effort_max":"xhigh"`
	effortFixedMedium       = `"effort_mode":"fixed","effort":"medium"`
	effortFixedHigh         = `"effort_mode":"fixed","effort":"high"`
	effortFixedNone         = `"effort_mode":"fixed","effort":"none"`
)

type effortTier struct{ model, fields string }

func setupSmartEffortRoute(t *testing.T, upstreamModel, tierFields, capabilities string, advisorModel ...string) int {
	t.Helper()
	tier := effortTier{upstreamModel, tierFields}
	return setupSmartEffortTiers(t, tier, tier, tier, capabilities, advisorModel...)
}

func setupSmartEffortTiers(t *testing.T, simple, normal, hard effortTier, capabilities string, advisorModel ...string) int {
	t.Helper()
	return setupSmartEffortAdvisor(t, simple, normal, hard, "", capabilities, advisorModel...)
}

func setupSmartEffortAdvisor(t *testing.T, simple, normal, hard effortTier, advisorFields, capabilities string, advisorModel ...string) int {
	t.Helper()
	if advisorFields != "" {
		advisorFields = "," + advisorFields
	}
	advisor := "eff-advisor"
	if len(advisorModel) > 0 {
		advisor = advisorModel[0]
	}
	setupSmartRouteDistributeTest(t)

	priority := int64(0)
	weight := uint(100)
	var models []string
	for _, name := range []string{simple.model, normal.model, hard.model, advisor} {
		if !slices.Contains(models, name) {
			models = append(models, name)
		}
	}
	for i, name := range models {
		id := 201 + i
		require.NoError(t, model.DB.Create(&model.Channel{
			Id: id, Type: constant.ChannelTypeOpenAI, Key: fmt.Sprintf("key-%d", id),
			Status: common.ChannelStatusEnabled, Name: fmt.Sprintf("channel-%d", id),
			Weight: &weight, Models: name, Group: "default", Priority: &priority,
		}).Error)
		require.NoError(t, model.DB.Create(&model.Ability{
			Group: "default", Model: name, ChannelId: id,
			Enabled: true, Priority: &priority, Weight: weight,
		}).Error)
	}
	tierJSON := func(tier effortTier) string {
		return fmt.Sprintf(`{"model":%q,"groups":["default"],%s}`, tier.model, tier.fields)
	}
	operation_setting.LoadSmartRoutesFromJSONString(fmt.Sprintf(`[{"name":"eff","endpoints":["openai-response","openai","anthropic"],
 "tiers":{"simple":%s,"normal":%s,"hard":%s},
 "default_tier":"normal","scorer":{"model":"m-scorer","groups":["default"],"timeout_ms":2000},
 "advisor":{"model":%q,"groups":["default"],"after_dissatisfied":1%s},
 "downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500}]`,
		tierJSON(simple), tierJSON(normal), tierJSON(hard), advisor, advisorFields))

	recordEfforts := map[string][]string{
		"gpt-5.6-sol":  {"none", "low", "medium", "high", "xhigh", "max"},
		"gpt-5.4-mini": {"none", "low", "medium", "high", "xhigh"},
		"gpt-5.5":      {"none", "low", "medium", "high", "xhigh"},
		"glm-5.3":      {"low", "high", "max"},
		"gpt-4.1":      {},
	}
	var overrides map[string][]string
	require.NoError(t, common.UnmarshalJsonStr(capabilities, &overrides))
	maps.Copy(recordEfforts, overrides)
	for name, efforts := range recordEfforts {
		encoded, err := common.Marshal(map[string]any{"reasoning": map[string]any{"supported_efforts": efforts}})
		require.NoError(t, err)
		require.NoError(t, model.DB.Create(&model.Model{ModelName: name, Capabilities: string(encoded)}).Error)
	}
	model.InitChannelCache()
	model.RefreshPricing()

	smartEffortTokenSeq++
	return smartEffortTokenSeq
}

func smartEffortResponsesBody(message, reasoning string) string {
	if reasoning != "" {
		reasoning = "," + reasoning
	}
	return fmt.Sprintf(`{"model":"eff","input":%q%s}`, message, reasoning)
}

func TestSmartRouteDistributeEffortFollowUsesEffortScore(t *testing.T) {
	for _, tc := range []struct {
		name      string
		labels    *smartroute.ScorerLabels
		failure   string
		wantValue string
	}{
		{"score 0 picks lowest candidate", smartScored(smartLabels("plan", "high", "neutral"), 0), "", "medium"},
		{"score 0.30", smartScored(smartLabels("plan", "high", "neutral"), 0.30), "", "medium"},
		{"score 0.50", smartScored(smartLabels("plan", "high", "neutral"), 0.50), "", "high"},
		{"score 0.72", smartScored(smartLabels("plan", "high", "neutral"), 0.72), "", "xhigh"},
		{"score 1 stays on highest candidate", smartScored(smartLabels("plan", "high", "neutral"), 1), "", "xhigh"},
		{"reasoning low does not matter", smartScored(smartLabels("analyze", "low", "neutral"), 0.72), "", "xhigh"},
		{"reasoning low same score", smartScored(smartLabels("analyze", "low", "neutral"), 0.30), "", "medium"},
		{"low confidence default tier still follows score", func() *smartroute.ScorerLabels {
			labels := smartScored(smartLabels("analyze", "low", "neutral"), 0.30)
			labels.Confidence = 0.3
			return labels
		}(), "", "medium"},
		{"missing score uses highest candidate", smartLabels("plan", "low", "neutral"), "", "xhigh"},
		{"scorer timeout uses highest candidate", nil, smartroute.FailureScorerTimeout, "xhigh"},
		{"scorer error uses highest candidate", nil, smartroute.FailureScorerError, "xhigh"},
		{"scorer invalid uses highest candidate", nil, smartroute.FailureScorerInvalid, "xhigh"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-5.6-sol", effortFollowMediumXhigh, `{}`)
			installSmartScorerStub(t, tc.labels, tc.failure, nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/responses", tokenID: tokenID,
				body: smartEffortResponsesBody("follow-"+tc.name, ""),
			})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, tc.wantValue, smartBodyField(t, seen.body, "reasoning", "effort"))
			assert.Equal(t, map[string]any{"value": tc.wantValue, "source": "follow"}, seen.routeLog["effort"])
		})
	}
}

func TestSmartRouteDistributeEffortFollowScoreAgainstCapability(t *testing.T) {
	for _, tc := range []struct {
		name, model, fields, capabilities string
		labels                            *smartroute.ScorerLabels
		failure                           string
		wantValue, wantFrom               string
	}{
		{"gap below picks nearest supported", "gpt-5.6-sol", `"effort_mode":"follow","effort_min":"medium","effort_max":"high"`,
			`{"gpt-5.6-sol":["low","xhigh"]}`, smartScored(smartLabels("plan", "high", "neutral"), 0.3), "", "low", "medium"},
		{"gap above picks nearest supported", "gpt-5.6-sol", `"effort_mode":"follow","effort_min":"medium","effort_max":"high"`,
			`{"gpt-5.6-sol":["low","xhigh"]}`, smartScored(smartLabels("plan", "high", "neutral"), 0.8), "", "xhigh", "high"},
		{"timeout with capped capability takes highest candidate without from", "gpt-5.6-sol", effortFollowMediumXhigh,
			`{"gpt-5.6-sol":["none","low","medium","high"]}`, nil, smartroute.FailureScorerTimeout, "high", ""},
		{"glm low", "glm-5.3", `"effort_mode":"follow","effort_min":"low","effort_max":"max"`, `{}`,
			smartScored(smartLabels("plan", "high", "neutral"), 0.2), "", "low", ""},
		{"glm high", "glm-5.3", `"effort_mode":"follow","effort_min":"low","effort_max":"max"`, `{}`,
			smartScored(smartLabels("plan", "high", "neutral"), 0.5), "", "high", ""},
		{"glm max", "glm-5.3", `"effort_mode":"follow","effort_min":"low","effort_max":"max"`, `{}`,
			smartScored(smartLabels("plan", "high", "neutral"), 0.9), "", "max", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, tc.model, tc.fields, tc.capabilities)
			installSmartScorerStub(t, tc.labels, tc.failure, nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/responses", tokenID: tokenID,
				body: smartEffortResponsesBody("cap-"+tc.name, ""),
			})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, tc.wantValue, smartBodyField(t, seen.body, "reasoning", "effort"))
			want := map[string]any{"value": tc.wantValue, "source": "follow"}
			if tc.wantFrom != "" {
				want["from"] = tc.wantFrom
			}
			assert.Equal(t, want, seen.routeLog["effort"])
		})
	}
}

func TestSmartRouteDistributeEffortFollowScoreSameAcrossFormats(t *testing.T) {
	const budget = `"thinking":{"type":"enabled","budget_tokens":2048}`
	for _, tc := range []struct {
		name, path, body string
		effortPath       []string
	}{
		{"responses", "/v1/responses", smartEffortResponsesBody("fmt-resp", ""), []string{"reasoning", "effort"}},
		{"chat", "/v1/chat/completions", `{"model":"eff","messages":[{"role":"user","content":"fmt-chat"}]}`, []string{"reasoning_effort"}},
		{"messages", "/v1/messages", smartEffortMessagesBody("fmt-msg", budget), []string{"output_config", "effort"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-5.6-sol", effortFollowMediumXhigh, `{}`)
			installSmartScorerStub(t, smartScored(smartLabels("plan", "low", "neutral"), 0.72), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{path: tc.path, tokenID: tokenID, body: tc.body})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, "xhigh", smartBodyField(t, seen.body, tc.effortPath...))
		})
	}
}

func TestSmartRouteDistributeEffortLogShape(t *testing.T) {
	t.Run("no from when the effort kept its level", func(t *testing.T) {
		tokenID := setupSmartEffortRoute(t, "gpt-5.6-sol", effortFollowMediumXhigh, `{}`)
		installSmartScorerStub(t, smartScored(smartLabels("plan", "high", "neutral"), 0.5), "", nil)

		_, seen := serveSmartRoute(t, smartRequest{path: "/v1/responses", tokenID: tokenID, body: smartEffortResponsesBody("log-plain", "")})

		assert.Equal(t, map[string]any{"value": "high", "source": "follow"}, seen.routeLog["effort"])
	})

	t.Run("capability with only low and high raises medium to high", func(t *testing.T) {
		tokenID := setupSmartEffortRoute(t, "gpt-5.4-mini", effortFixedMedium,
			`{"gpt-5.4-mini":["low","high"]}`)
		installSmartScorerStub(t, smartLabels("analyze", "medium", "neutral"), "", nil)

		_, seen := serveSmartRoute(t, smartRequest{path: "/v1/responses", tokenID: tokenID, body: smartEffortResponsesBody("log-gap", "")})

		assert.Equal(t, "high", smartBodyField(t, seen.body, "reasoning", "effort"))
		assert.Equal(t, map[string]any{"value": "high", "source": "fixed", "from": "medium"}, seen.routeLog["effort"])
	})
}

func TestSmartRouteDistributeEffortFixedOverridesClientValue(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		effortPath       []string
	}{
		{"responses", "/v1/responses", smartEffortResponsesBody("fixed-resp", `"reasoning":{"effort":"high"}`), []string{"reasoning", "effort"}},
		{"chat", "/v1/chat/completions", `{"model":"eff","messages":[{"role":"user","content":"fixed-chat"}],"reasoning_effort":"high"}`, []string{"reasoning_effort"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-5.6-sol", effortFixedMedium, `{}`)
			installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{path: tc.path, body: tc.body, tokenID: tokenID})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, "medium", smartBodyField(t, seen.body, tc.effortPath...))
			assert.Equal(t, map[string]any{"value": "medium", "source": "fixed"}, seen.routeLog["effort"])
		})
	}
}

func TestSmartRouteDistributeEffortInheritClientValue(t *testing.T) {
	for _, tc := range []struct {
		name       string
		capability string
		reasoning  string
		wantBody   any
		wantAbsent bool
		wantLog    map[string]any
	}{
		{"client value lowered by capability", `{"gpt-5.4-mini":["none","low","medium","high"]}`,
			`"reasoning":{"effort":"xhigh"}`, "high", false, map[string]any{"value": "high", "source": "client", "from": "xhigh"}},
		{"client did not send", `{}`, "", nil, true, map[string]any{"value": "", "source": "client_unset"}},
		{"upper case client value", `{}`, `"reasoning":{"effort":"HIGH"}`, "high", false,
			map[string]any{"value": "high", "source": "client"}},
		{"unknown client value kept", `{}`, `"reasoning":{"effort":"turbo"}`, "turbo", false,
			map[string]any{"value": "turbo", "source": "client"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-5.4-mini", `"effort_mode":""`, tc.capability)
			installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/responses", tokenID: tokenID,
				body: smartEffortResponsesBody("inherit-"+tc.name, tc.reasoning),
			})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, tc.wantLog, seen.routeLog["effort"])
			if tc.wantAbsent {
				assert.Nil(t, smartBodyField(t, seen.body, "reasoning", "effort"))
				return
			}
			assert.Equal(t, tc.wantBody, smartBodyField(t, seen.body, "reasoning", "effort"))
		})
	}
}

func TestSmartRouteDistributeEffortNoThinkingModelDropsThinkingParams(t *testing.T) {
	for _, mode := range []struct{ name, fields string }{
		{"fixed", `"effort_mode":"fixed","effort":"high"`},
		{"follow", effortFollowMediumXhigh},
		{"inherit", `"effort_mode":""`},
	} {
		t.Run(mode.name+" responses", func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-4.1", mode.fields, `{}`)
			installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/responses", tokenID: tokenID,
				body: smartEffortResponsesBody("nothink-resp-"+mode.name, `"reasoning":{"effort":"high","summary":"auto"}`),
			})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Nil(t, smartBodyField(t, seen.body, "reasoning", "effort"))
			assert.Equal(t, map[string]any{"value": "", "source": "no_thinking"}, seen.routeLog["effort"])
		})
		t.Run(mode.name+" chat", func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-4.1", mode.fields, `{}`)
			installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/chat/completions", tokenID: tokenID,
				body: fmt.Sprintf(`{"model":"eff","messages":[{"role":"user","content":"nothink-chat-%s"}],"reasoning_effort":"high","reasoning":{"effort":"high"}}`, mode.name),
			})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Nil(t, smartBodyField(t, seen.body, "reasoning_effort"))
			assert.Nil(t, smartBodyField(t, seen.body, "reasoning"))
			assert.Equal(t, map[string]any{"value": "", "source": "no_thinking"}, seen.routeLog["effort"])
		})
	}
}

func TestSmartRouteDistributeEffortContinuationReusesSessionEffort(t *testing.T) {
	t.Run("tool result continuation reuses the last sent effort", func(t *testing.T) {
		tokenID := setupSmartEffortRoute(t, "gpt-5.6-sol", effortFollowMediumXhigh, `{}`)
		stub := installSmartScorerStub(t, smartScored(smartLabels("plan", "high", "neutral"), 0.5), "", nil)

		first, _ := serveSmartRoute(t, smartRequest{path: "/v1/responses", tokenID: tokenID, body: smartEffortResponsesBody("continue-reuse", "")})
		require.Equal(t, http.StatusOK, first.Code, first.Body.String())
		require.Equal(t, 1, stub.calls)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/responses", tokenID: tokenID,
			body: `{"model":"eff","input":[{"role":"user","content":"continue-reuse"},{"type":"function_call_output","call_id":"c1","output":"ok"}]}`,
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, 1, stub.calls)
		assert.Equal(t, "high", smartBodyField(t, seen.body, "reasoning", "effort"))
		effort, ok := seen.routeLog["effort"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "high", effort["value"])
		assert.Equal(t, "follow", effort["source"])
	})

	t.Run("compact request reuses the last sent effort", func(t *testing.T) {
		tokenID := setupSmartEffortRoute(t, "gpt-5.6-sol", effortFollowMediumXhigh, `{}`)
		installSmartScorerStub(t, smartScored(smartLabels("plan", "high", "neutral"), 0.5), "", nil)
		body := `{"model":"eff","input":[{"role":"user","content":"compact-reuse"}],"prompt_cache_key":"pck-effort-compact"}`

		first, _ := serveSmartRoute(t, smartRequest{path: "/v1/responses", tokenID: tokenID, body: body})
		require.Equal(t, http.StatusOK, first.Code, first.Body.String())

		recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/responses/compact", tokenID: tokenID, body: body})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, "high", smartBodyField(t, seen.body, "reasoning", "effort"))
		effort, ok := seen.routeLog["effort"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "follow", effort["source"])
	})

	for _, tc := range []struct {
		name, fields, reasoning string
		wantBody                any
	}{
		{"no record follow uses max", effortFollowMediumXhigh, `"reasoning":{"effort":"low"}`, "xhigh"},
		{"no record fixed uses fixed value", effortFixedMedium, `"reasoning":{"effort":"low"}`, "medium"},
		{"no record inherit uses client value", `"effort_mode":""`, `"reasoning":{"effort":"low"}`, "low"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-5.6-sol", tc.fields, `{}`)
			stub := installSmartScorerStub(t, smartLabels("chat_qa", "low", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/responses", tokenID: tokenID,
				body: fmt.Sprintf(`{"model":"eff","input":[{"role":"user","content":"continue-fresh-%s"},{"type":"function_call_output","call_id":"c1","output":"ok"}],%s}`,
					tc.name, tc.reasoning),
			})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Zero(t, stub.calls)
			assert.Equal(t, tc.wantBody, smartBodyField(t, seen.body, "reasoning", "effort"))
		})
	}
}

func TestSmartRouteDistributeEffortAdvisorClampsClientValue(t *testing.T) {
	const lowToMax = `{"eff-advisor":["low","medium","high","xhigh","max"]}`
	for _, tc := range []struct {
		name, capabilities, path, extra string
		wantEffort                      any
		wantLog                         map[string]any
	}{
		{"none moves to the lowest supported", lowToMax, "/v1/responses", `"reasoning":{"effort":"none"}`,
			"low", map[string]any{"value": "low", "source": "client", "from": "none"}},
		{"anthropic disabled thinking moves to the lowest supported", lowToMax, "/v1/messages", `"thinking":{"type":"disabled"}`,
			"low", map[string]any{"value": "low", "source": "client", "from": "none"}},
		{"supported value kept", lowToMax, "/v1/responses", `"reasoning":{"effort":"high"}`,
			"high", map[string]any{"value": "high", "source": "client"}},
		{"minimal moves to low", lowToMax, "/v1/responses", `"reasoning":{"effort":"minimal"}`,
			"low", map[string]any{"value": "low", "source": "client", "from": "minimal"}},
		{"max moves to the highest supported", `{"eff-advisor":["low","medium","high"]}`, "/v1/responses", `"reasoning":{"effort":"max"}`,
			"high", map[string]any{"value": "high", "source": "client", "from": "max"}},
		{"tie between two supported picks the higher", `{"eff-advisor":["low","high"]}`, "/v1/responses", `"reasoning":{"effort":"medium"}`,
			"high", map[string]any{"value": "high", "source": "client", "from": "medium"}},
		{"model without thinking drops the effort", `{"eff-advisor":[]}`, "/v1/responses", `"reasoning":{"effort":"medium"}`,
			nil, map[string]any{"value": "", "source": "no_thinking"}},
		{"client did not send", lowToMax, "/v1/responses", ``,
			nil, map[string]any{"value": "", "source": "client_unset"}},
		{"unknown client value kept", lowToMax, "/v1/responses", `"reasoning":{"effort":"turbo"}`,
			"turbo", map[string]any{"value": "turbo", "source": "client"}},
		{"unconfirmed capability falls back to low medium high", `{}`, "/v1/responses", `"reasoning":{"effort":"xhigh"}`,
			"high", map[string]any{"value": "high", "source": "client", "from": "xhigh"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-5.6-sol", effortFollowMediumXhigh, tc.capabilities)
			installSmartScorerStub(t, smartLabels("chat_qa", "low", "dissatisfied"), "", nil)
			body := smartEffortResponsesBody("advisor-effort-"+tc.name, tc.extra)
			effortPath := []string{"reasoning", "effort"}
			if tc.path == "/v1/messages" {
				body = smartEffortMessagesBody("advisor-effort-"+tc.name, tc.extra)
				effortPath = []string{"output_config", "effort"}
			}

			recorder, seen := serveSmartRoute(t, smartRequest{path: tc.path, tokenID: tokenID, body: body})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, "eff-advisor", seen.originalModel)
			assert.Equal(t, tc.wantEffort, smartBodyField(t, seen.body, effortPath...))
			assert.Equal(t, tc.wantLog, seen.routeLog["effort"])
			if tc.path == "/v1/messages" {
				assert.Nil(t, smartBodyField(t, seen.body, "thinking"))
			}
		})
	}
}

func TestSmartRouteDistributeEffortAdvisorOwnMode(t *testing.T) {
	const allEfforts = `{"eff-advisor":["none","low","medium","high","xhigh","max"]}`
	const noMax = `{"eff-advisor":["low","medium","high","xhigh"]}`
	followMediumMax := `"effort_mode":"follow","effort_min":"medium","effort_max":"max"`
	disabled := `"reasoning":{"effort":"none"}`
	dissatisfied := func(score *float64) *smartroute.ScorerLabels {
		labels := smartLabels("chat_qa", "low", "dissatisfied")
		labels.EffortScore = score
		return labels
	}
	score := func(v float64) *float64 { return &v }
	for _, tc := range []struct {
		name, advisorFields, capabilities, extra string
		score                                    *float64
		wantEffort                               any
		wantLog                                  map[string]any
	}{
		{"follow high score picks max", followMediumMax, allEfforts, "", score(0.9), "max", map[string]any{"value": "max", "source": "follow"}},
		{"follow low score picks medium", followMediumMax, allEfforts, "", score(0.1), "medium", map[string]any{"value": "medium", "source": "follow"}},
		{"follow without score picks the highest", followMediumMax, allEfforts, "", nil, "max", map[string]any{"value": "max", "source": "follow"}},
		{"follow only counts levels the model supports", followMediumMax, noMax, "", score(0.9), "xhigh", map[string]any{"value": "xhigh", "source": "follow"}},
		{"fixed high replaces the client disabled thinking", effortFixedHigh, allEfforts, disabled, nil, "high", map[string]any{"value": "high", "source": "fixed"}},
		{"fixed none moves to the lowest supported", effortFixedNone, `{"eff-advisor":["low","medium"]}`, disabled, nil, "low", map[string]any{"value": "low", "source": "fixed", "from": "none"}},
		{"fixed high on a model without thinking drops the effort", effortFixedHigh, `{"eff-advisor":[]}`, "", nil, nil, map[string]any{"value": "", "source": "no_thinking"}},
		{"empty mode keeps the client value", `"effort_mode":""`, allEfforts, `"reasoning":{"effort":"xhigh"}`, nil, "xhigh", map[string]any{"value": "xhigh", "source": "client"}},
		{"empty mode without client value adds nothing", `"effort_mode":""`, allEfforts, "", nil, nil, map[string]any{"value": "", "source": "client_unset"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := effortTier{"gpt-5.6-sol", `"effort_mode":""`}
			tokenID := setupSmartEffortAdvisor(t, base, base, base, tc.advisorFields, tc.capabilities)
			installSmartScorerStub(t, dissatisfied(tc.score), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/responses", tokenID: tokenID,
				body: smartEffortResponsesBody("advisor-own-"+tc.name, tc.extra),
			})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, "eff-advisor", seen.originalModel)
			assert.Equal(t, tc.wantEffort, smartBodyField(t, seen.body, "reasoning", "effort"))
			assert.Equal(t, tc.wantLog, seen.routeLog["effort"])
			if tc.wantEffort == nil {
				assert.Nil(t, smartBodyField(t, seen.body, "reasoning"))
			}
		})
	}
}

func TestSmartRouteDistributeEffortAdvisorToolContinuationKeepsTurnEffort(t *testing.T) {
	base := effortTier{"gpt-5.6-sol", `"effort_mode":""`}
	tokenID := setupSmartEffortAdvisor(t, base, base, base,
		`"effort_mode":"follow","effort_min":"medium","effort_max":"max"`,
		`{"eff-advisor":["low","medium","high","xhigh","max"]}`)
	labels := smartLabels("chat_qa", "low", "dissatisfied")
	score := 0.9
	labels.EffortScore = &score
	installSmartScorerStub(t, labels, "", nil)

	first, firstSeen := serveSmartRoute(t, smartRequest{
		path: "/v1/responses", tokenID: tokenID,
		body: smartEffortResponsesBody("advisor-own-continue", ""),
	})
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Equal(t, "max", smartBodyField(t, firstSeen.body, "reasoning", "effort"))

	recorder, seen := serveSmartRoute(t, smartRequest{
		path: "/v1/responses", tokenID: tokenID,
		body: `{"model":"eff","input":[{"role":"user","content":"advisor-own-continue"},{"type":"function_call_output","call_id":"c1","output":"ok"}]}`,
	})

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "eff-advisor", seen.originalModel)
	assert.Equal(t, "max", smartBodyField(t, seen.body, "reasoning", "effort"))
}

func TestSmartRouteDistributeEffortAdvisorRecomputesOnNewUserMessage(t *testing.T) {
	base := effortTier{"gpt-5.6-sol", `"effort_mode":""`}
	tokenID := setupSmartEffortAdvisor(t, base, base, base,
		`"effort_mode":"follow","effort_min":"medium","effort_max":"max"`,
		`{"eff-advisor":["low","medium","high","xhigh","max"]}`)
	for _, tc := range []struct {
		message string
		score   float64
		want    string
	}{{"advisor-recompute", 0.9, "max"}, {"advisor-recompute", 0.1, "medium"}} {
		labels := smartLabels("chat_qa", "low", "dissatisfied")
		labels.EffortScore = &tc.score
		installSmartScorerStub(t, labels, "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/responses", tokenID: tokenID,
			body: `{"model":"eff","input":[{"role":"user","content":"advisor-recompute"},{"role":"assistant","content":"a"},{"role":"user","content":"again ` + tc.want + `"}]}`,
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, "eff-advisor", seen.originalModel)
		assert.Equal(t, tc.want, smartBodyField(t, seen.body, "reasoning", "effort"))
	}
}

func TestSmartRouteDistributeEffortAdvisorFixedOnAnthropicDisabledThinking(t *testing.T) {
	base := effortTier{"gpt-5.6-sol", `"effort_mode":""`}
	tokenID := setupSmartEffortAdvisor(t, base, base, base, effortFixedHigh, `{"eff-advisor":["low","medium","high","max"]}`)
	installSmartScorerStub(t, smartLabels("chat_qa", "low", "dissatisfied"), "", nil)

	recorder, seen := serveSmartRoute(t, smartRequest{
		path: "/v1/messages", tokenID: tokenID,
		body: smartEffortMessagesBody("advisor-fixed-anthropic", `"thinking":{"type":"disabled"}`),
	})

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "eff-advisor", seen.originalModel)
	assert.Equal(t, "high", smartBodyField(t, seen.body, "output_config", "effort"))
	assert.NotEqual(t, "disabled", smartBodyField(t, seen.body, "thinking", "type"))
}

func TestSmartRouteDistributeEffortChatReasoningObjectClamped(t *testing.T) {
	const lowToMax = `{"eff-advisor":["low","medium","high","xhigh","max"],"gpt-5.6-sol":["low","medium","high","xhigh","max"]}`
	const noThinking = `{"eff-advisor":[],"gpt-5.6-sol":[]}`
	fromNone := map[string]any{"value": "low", "source": "client", "from": "none"}
	noThinkingLog := map[string]any{"value": "", "source": "no_thinking"}
	for _, tc := range []struct {
		name, tierFields, capabilities, extra string
		advisor                               bool
		wantEffort                            any
		wantLog                               map[string]any
		wantReasoning                         map[string]any
	}{
		{"advisor none moves to low", effortFollowMediumXhigh, lowToMax, `"reasoning":{"effort":"none"}`, true, "low", fromNone, nil},
		{"inherit none moves to low", `"effort_mode":""`, lowToMax, `"reasoning":{"effort":"none"}`, false, "low", fromNone, nil},
		{"advisor disabled reasoning counts as none", effortFollowMediumXhigh, lowToMax, `"reasoning":{"enabled":false}`, true, "low", fromNone, nil},
		{"inherit disabled reasoning counts as none", `"effort_mode":""`, lowToMax, `"reasoning":{"enabled":false}`, false, "low", fromNone, nil},
		{"advisor zero token budget counts as none", effortFollowMediumXhigh, lowToMax, `"reasoning":{"max_tokens":0}`, true, "low", fromNone, nil},
		{"advisor large token budget counts as high and moves to medium", effortFollowMediumXhigh, `{"eff-advisor":["low","medium"]}`, `"reasoning":{"max_tokens":20000}`, true, "medium",
			map[string]any{"value": "medium", "source": "client", "from": "high"}, nil},
		{"advisor without thinking drops the reasoning object", effortFollowMediumXhigh, noThinking, `"reasoning":{"effort":"medium"}`, true, nil, noThinkingLog, nil},
		{"inherit without thinking drops the reasoning object", `"effort_mode":""`, noThinking, `"reasoning":{"effort":"medium"}`, false, nil, noThinkingLog, nil},
		{"advisor enabled reasoning counts as high and drops the whole reasoning object", effortFollowMediumXhigh, `{"eff-advisor":["low","medium"]}`, `"reasoning":{"enabled":true,"exclude":true}`, true, "medium",
			map[string]any{"value": "medium", "source": "client", "from": "high"}, nil},
		{"advisor supported effort leaves the reasoning object untouched", effortFollowMediumXhigh, lowToMax, `"reasoning":{"enabled":true,"exclude":true}`, true, nil,
			map[string]any{"value": "high", "source": "client"}, map[string]any{"enabled": true, "exclude": true}},
		{"inherit zero token budget counts as none", `"effort_mode":""`, lowToMax, `"reasoning":{"max_tokens":0}`, false, "low", fromNone, nil},
		{"advisor without thinking drops the token budget", effortFollowMediumXhigh, noThinking, `"reasoning":{"max_tokens":2000}`, true, nil, noThinkingLog, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-5.6-sol", tc.tierFields, tc.capabilities)
			scorer := smartLabels("plan", "high", "neutral")
			if tc.advisor {
				scorer = smartLabels("chat_qa", "low", "dissatisfied")
			}
			installSmartScorerStub(t, scorer, "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/chat/completions", tokenID: tokenID,
				body: fmt.Sprintf(`{"model":"eff","messages":[{"role":"user","content":%q}],%s}`, "chat-reasoning-"+tc.name, tc.extra),
			})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			if tc.advisor {
				assert.Equal(t, "eff-advisor", seen.originalModel)
			}
			assert.Equal(t, tc.wantEffort, smartBodyField(t, seen.body, "reasoning_effort"))
			if tc.wantReasoning == nil {
				assert.Nil(t, smartBodyField(t, seen.body, "reasoning"))
			} else {
				assert.Equal(t, tc.wantReasoning, smartBodyField(t, seen.body, "reasoning"))
			}
			assert.Equal(t, tc.wantLog, seen.routeLog["effort"])
			assert.NotContains(t, string(seen.body), `"none"`)
		})
	}
}

func TestSmartRouteDistributeEffortAnthropicOutputConfigOnly(t *testing.T) {
	const sonnetCaps = `{"eff-advisor":["low","medium","high","max"],"claude-sonnet-4-6":["low","medium","high","max"],"gpt-5.6-sol":["low","medium","high","max"]}`
	const extra = `"output_config":{"effort":"xhigh"},"temperature":0.3`
	for _, tc := range []struct {
		name, model, tierFields string
		advisorModel            string
		wantEffort              any
		wantThinking            any
		wantTemperature         any
	}{
		{"advisor moves xhigh to max", "gpt-5.6-sol", effortFollowMediumXhigh, "claude-sonnet-4-6", "max", nil, 0.3},
		{"inherit moves xhigh to max", "claude-sonnet-4-6", `"effort_mode":""`, "", "max", nil, 0.3},
		{"fixed keeps the pre-change handling", "claude-sonnet-4-6", effortFixedMedium, "", "medium", map[string]any{"type": "adaptive"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var advisorModel []string
			scorer := smartLabels("plan", "high", "neutral")
			if tc.advisorModel != "" {
				advisorModel = []string{tc.advisorModel}
				scorer = smartLabels("chat_qa", "low", "dissatisfied")
			}
			tokenID := setupSmartEffortRoute(t, tc.model, tc.tierFields, sonnetCaps, advisorModel...)
			installSmartScorerStub(t, scorer, "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/messages", tokenID: tokenID,
				body: smartEffortMessagesBody("anthropic-output-config-"+tc.name, extra),
			})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			if tc.advisorModel != "" {
				assert.Equal(t, tc.advisorModel, seen.originalModel)
			}
			assert.Equal(t, tc.wantEffort, smartBodyField(t, seen.body, "output_config", "effort"))
			assert.Equal(t, tc.wantThinking, smartBodyField(t, seen.body, "thinking"))
			assert.Equal(t, tc.wantTemperature, smartBodyField(t, seen.body, "temperature"))
		})
	}
}

func TestSmartRouteDistributeEffortAdvisorNotOverriddenContinuationKeepsTurnEffort(t *testing.T) {
	tokenID := setupSmartEffortRoute(t, "gpt-5.6-sol", effortFollowMediumXhigh, `{"eff-advisor":["low","medium","high"]}`)
	installSmartScorerStub(t, smartLabels("chat_qa", "low", "dissatisfied"), "", nil)

	first, _ := serveSmartRoute(t, smartRequest{
		path: "/v1/responses", tokenID: tokenID,
		body: smartEffortResponsesBody("advisor-continue-clamp", `"reasoning":{"effort":"high"}`),
	})
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	recorder, seen := serveSmartRoute(t, smartRequest{
		path: "/v1/responses", tokenID: tokenID,
		body: `{"model":"eff","input":[{"role":"user","content":"advisor-continue-clamp"},{"type":"function_call_output","call_id":"c1","output":"ok"}],"reasoning":{"effort":"none"}}`,
	})

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "eff-advisor", seen.originalModel)
	assert.Equal(t, "high", smartBodyField(t, seen.body, "reasoning", "effort"))
	assert.Equal(t, map[string]any{"value": "high", "source": "client"}, seen.routeLog["effort"])
}

func TestSmartRouteDistributeEffortContinuationRecomputesWhenTierDiffersFromSession(t *testing.T) {
	normal := effortTier{"gpt-5.4-mini", `"effort_mode":"fixed","effort":"low"`}
	hard := effortTier{"gpt-5.6-sol", effortFollowMediumXhigh}
	tokenID := setupSmartEffortTiers(t, normal, normal, hard, `{}`)
	installSmartScorerStub(t, smartScored(smartLabels("plan", "high", "neutral"), 0.5), "", nil)
	hardBody := smartEffortResponsesBody("continue-tier-differs-hard", "")
	hardFirst, hardSeen := serveSmartRoute(t, smartRequest{path: "/v1/responses", tokenID: tokenID, body: hardBody})
	require.Equal(t, http.StatusOK, hardFirst.Code, hardFirst.Body.String())
	require.Equal(t, "gpt-5.6-sol", hardSeen.originalModel)
	require.Equal(t, "high", smartBodyField(t, hardSeen.body, "reasoning", "effort"))

	require.NoError(t, model.DB.Model(&model.Ability{}).Where("model = ?", "gpt-5.6-sol").Update("enabled", false).Error)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("models = ?", "gpt-5.6-sol").Update("status", common.ChannelStatusManuallyDisabled).Error)
	model.InitChannelCache()

	recorder, seen := serveSmartRoute(t, smartRequest{
		path: "/v1/responses", tokenID: tokenID,
		body: `{"model":"eff","input":[{"role":"user","content":"continue-tier-differs-hard"},{"type":"function_call_output","call_id":"c1","output":"ok"}]}`,
	})

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "gpt-5.4-mini", seen.originalModel)
	assert.Equal(t, "low", smartBodyField(t, seen.body, "reasoning", "effort"))
	effort, ok := seen.routeLog["effort"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "low", effort["value"])
	assert.Equal(t, "fixed", effort["source"])
}

func TestSmartRouteDistributePlaygroundUsesSmartRouting(t *testing.T) {
	t.Run("chat virtual model routes to the tier model", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		installSmartScorerStub(t, smartLabels("chat_qa", "low", "neutral"), "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/pg/chat/completions", userID: 7001,
			body: `{"model":"chat-only","messages":[{"role":"user","content":"playground hello"}]}`,
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, "c-chat", seen.originalModel)
		assert.Equal(t, "c-chat", smartBodyField(t, seen.body, "model"))
	})

	t.Run("responses only virtual model is rejected", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		stub := installSmartScorerStub(t, smartLabels("chat_qa", "low", "neutral"), "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/pg/chat/completions", userID: 7002,
			body: `{"model":"resp-only","messages":[{"role":"user","content":"playground hello"}]}`,
		})

		require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		assert.Contains(t, smartBodyField(t, recorder.Body.Bytes(), "error", "message"), "only supports these endpoints: Responses")
		assert.False(t, seen.ran)
		assert.Zero(t, stub.calls)
	})

	t.Run("sessions are not shared between users", func(t *testing.T) {
		normal := effortTier{"gpt-5.4-mini", `"effort_mode":""`}
		hard := effortTier{"gpt-5.6-sol", `"effort_mode":""`}
		setupSmartEffortTiers(t, normal, normal, hard, `{}`)
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)
		const message = "playground same first message"

		first, firstSeen := serveSmartRoute(t, smartRequest{
			path: "/pg/chat/completions", userID: 7003,
			body: fmt.Sprintf(`{"model":"eff","messages":[{"role":"user","content":%q}]}`, message),
		})
		require.Equal(t, http.StatusOK, first.Code, first.Body.String())
		require.Equal(t, "gpt-5.6-sol", firstSeen.originalModel)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/pg/chat/completions", userID: 7004,
			body: fmt.Sprintf(`{"model":"eff","messages":[{"role":"user","content":%q},{"role":"tool","tool_call_id":"c1","content":"ok"}]}`, message),
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, "gpt-5.4-mini", seen.originalModel)
	})
}

func smartEffortMessagesBody(message, extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return fmt.Sprintf(`{"model":"eff","max_tokens":64000,"messages":[{"role":"user","content":%q}]%s}`, message, extra)
}

func TestSmartRouteDistributeMessagesEndpoint(t *testing.T) {
	t.Run("virtual model without anthropic is rejected before scoring", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/messages", tokenID: 9401,
			body: `{"model":"resp-only","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`,
		})

		require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		assert.Contains(t, recorder.Body.String(), "Virtual model resp-only only supports these endpoints: Responses")
		assert.False(t, seen.ran)
		assert.Zero(t, stub.calls)
		assert.Empty(t, stub.settles)
	})

	t.Run("user message is scored and routed to the tier model", func(t *testing.T) {
		normal := effortTier{"gpt-5.4-mini", `"effort_mode":""`}
		hard := effortTier{"gpt-5.6-sol", `"effort_mode":""`}
		tokenID := setupSmartEffortTiers(t, normal, normal, hard, `{}`)
		stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/messages", tokenID: tokenID, body: smartEffortMessagesBody("design a billing system", "")})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, 1, stub.calls)
		assert.Equal(t, "gpt-5.6-sol", seen.originalModel)
		assert.Equal(t, "gpt-5.6-sol", smartBodyField(t, seen.body, "model"))
		assert.Equal(t, "main", seen.routeLog["role"])
	})

	t.Run("tool result continuation skips the scorer and keeps the session tier", func(t *testing.T) {
		normal := effortTier{"gpt-5.4-mini", `"effort_mode":""`}
		hard := effortTier{"gpt-5.6-sol", `"effort_mode":""`}
		tokenID := setupSmartEffortTiers(t, normal, normal, hard, `{}`)
		stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)
		first, _ := serveSmartRoute(t, smartRequest{path: "/v1/messages", tokenID: tokenID, body: smartEffortMessagesBody("continue anthropic", "")})
		require.Equal(t, http.StatusOK, first.Code, first.Body.String())
		require.Equal(t, 1, stub.calls)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/messages", tokenID: tokenID,
			body: `{"model":"eff","max_tokens":64,"messages":[{"role":"user","content":"continue anthropic"},{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"f","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}]}`,
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, 1, stub.calls)
		assert.Equal(t, "gpt-5.6-sol", seen.originalModel)
		assert.Equal(t, "tool_continuation", seen.routeLog["reason"])
		assert.NotContains(t, seen.routeLog, "labels")
	})
}

func TestSmartRouteDistributeMessagesRewritesThinking(t *testing.T) {
	budget := `"thinking":{"type":"enabled","budget_tokens":2048}`
	for _, tc := range []struct {
		name, model, fields, extra string
		check                      func(t *testing.T, body []byte)
	}{
		{"manual budget claude keeps a valid budget", "claude-sonnet-4-5", effortFixedHigh, budget, func(t *testing.T, body []byte) {
			assert.Equal(t, "enabled", smartBodyField(t, body, "thinking", "type"))
			budgetTokens, ok := smartBodyField(t, body, "thinking", "budget_tokens").(float64)
			require.True(t, ok, "budget_tokens missing: %s", body)
			assert.GreaterOrEqual(t, budgetTokens, 1024.0)
		}},
		{"claude that disables with between_tools", "claude-sonnet-5-5", effortFixedNone, "", func(t *testing.T, body []byte) {
			assert.Equal(t, "between_tools", smartBodyField(t, body, "thinking", "type"))
		}},
		{"claude with effort", "claude-opus-5-5", effortFixedHigh, "", func(t *testing.T, body []byte) {
			assert.Equal(t, "high", smartBodyField(t, body, "output_config", "effort"))
		}},
		{"non claude high converts the budget and writes output_config effort", "gpt-5.5", effortFixedHigh, budget, func(t *testing.T, body []byte) {
			assert.Equal(t, "high", smartBodyField(t, body, "output_config", "effort"))
			assert.Equal(t, "enabled", smartBodyField(t, body, "thinking", "type"))
			budgetTokens, ok := smartBodyField(t, body, "thinking", "budget_tokens").(float64)
			require.True(t, ok, "budget_tokens missing: %s", body)
			assert.GreaterOrEqual(t, budgetTokens, 1024.0)
			assert.NotEqual(t, 2048.0, budgetTokens)
		}},
		{"non claude none disables thinking", "gpt-5.5", effortFixedNone, `"output_config":{"effort":"high"}`, func(t *testing.T, body []byte) {
			assert.JSONEq(t, `{"type":"disabled"}`, mustJSON(t, smartBodyField(t, body, "thinking")))
			assert.Nil(t, smartBodyField(t, body, "output_config", "effort"))
		}},
		{"client disabled thinking with high drops thinking", "gpt-5.5", effortFixedHigh, `"thinking":{"type":"disabled"}`, func(t *testing.T, body []byte) {
			assert.Nil(t, smartBodyField(t, body, "thinking"))
			assert.Equal(t, "high", smartBodyField(t, body, "output_config", "effort"))
		}},
		{"model without thinking drops both", "gpt-4.1", effortFixedHigh, budget + `,"output_config":{"effort":"high"}`, func(t *testing.T, body []byte) {
			assert.Nil(t, smartBodyField(t, body, "thinking"))
			assert.Nil(t, smartBodyField(t, body, "output_config", "effort"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, tc.model, tc.fields, `{}`)
			installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/messages", tokenID: tokenID, body: smartEffortMessagesBody("rewrite-"+tc.name, tc.extra)})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, tc.model, smartBodyField(t, seen.body, "model"))
			tc.check(t, seen.body)
		})
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := common.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

func TestSmartRouteDistributeMessagesRejectsDuplicateKeys(t *testing.T) {
	for _, tc := range []struct{ name, extra string }{
		{"thinking twice", `"thinking":{"type":"enabled","budget_tokens":2048},"thinking":{"type":"disabled"}`},
		{"output_config twice", `"output_config":{"effort":"low"},"output_config":{"effort":"high"}`},
		{"effort twice", `"output_config":{"effort":"low","effort":"high"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-5.5", effortFixedHigh, `{}`)
			stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/messages", tokenID: tokenID, body: smartEffortMessagesBody("dup-"+tc.name, tc.extra)})

			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			assert.False(t, seen.ran)
			assert.Zero(t, stub.calls)
		})
	}
}

func TestSmartRouteDistributeMessagesThinkingRewriteDetails(t *testing.T) {
	t.Run("missing max_tokens gets the default and the budget follows it", func(t *testing.T) {
		tokenID := setupSmartEffortRoute(t, "claude-sonnet-4-5", effortFixedHigh, `{}`)
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/messages", tokenID: tokenID,
			body: `{"model":"eff","messages":[{"role":"user","content":"no max tokens"}]}`,
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		maxTokens, ok := smartBodyField(t, seen.body, "max_tokens").(float64)
		require.True(t, ok, "max_tokens missing: %s", seen.body)
		assert.GreaterOrEqual(t, int(maxTokens), model_setting.GetClaudeSettings().GetDefaultMaxTokens("claude-sonnet-4-5"))
		budget, ok := smartBodyField(t, seen.body, "thinking", "budget_tokens").(float64)
		require.True(t, ok, "budget_tokens missing: %s", seen.body)
		assert.Greater(t, budget, 1280.0)
		assert.Less(t, budget, maxTokens)
	})

	t.Run("manual thinking constrains sampling", func(t *testing.T) {
		tokenID := setupSmartEffortRoute(t, "claude-sonnet-4-5", effortFixedHigh, `{}`)
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		_, seen := serveSmartRoute(t, smartRequest{path: "/v1/messages", tokenID: tokenID, body: smartEffortMessagesBody("sampling-manual", `"temperature":0.2,"top_k":5`)})

		assert.Equal(t, "enabled", smartBodyField(t, seen.body, "thinking", "type"))
		assert.Nil(t, smartBodyField(t, seen.body, "temperature"))
		assert.Nil(t, smartBodyField(t, seen.body, "top_k"))
	})

	t.Run("strict sampling model clears sampling", func(t *testing.T) {
		tokenID := setupSmartEffortRoute(t, "claude-opus-5-5", effortFixedHigh, `{}`)
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		_, seen := serveSmartRoute(t, smartRequest{path: "/v1/messages", tokenID: tokenID, body: smartEffortMessagesBody("sampling-strict", `"temperature":0.2,"top_p":0.5,"top_k":5`)})

		assert.Nil(t, smartBodyField(t, seen.body, "temperature"))
		assert.Nil(t, smartBodyField(t, seen.body, "top_p"))
		assert.Nil(t, smartBodyField(t, seen.body, "top_k"))
	})

	t.Run("display and block_binding survive an adaptive rewrite", func(t *testing.T) {
		tokenID := setupSmartEffortRoute(t, "claude-opus-4-6", effortFixedHigh, `{}`)
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

		_, seen := serveSmartRoute(t, smartRequest{path: "/v1/messages", tokenID: tokenID, body: smartEffortMessagesBody("keep-display",
			`"thinking":{"type":"adaptive","display":"summarized","block_binding":{"prefix_mismatch_behavior":"drop_block"}},"output_config":{"effort":"low"}`)})

		assert.Equal(t, "adaptive", smartBodyField(t, seen.body, "thinking", "type"))
		assert.Equal(t, "summarized", smartBodyField(t, seen.body, "thinking", "display"))
		assert.Equal(t, map[string]any{"prefix_mismatch_behavior": "drop_block"}, smartBodyField(t, seen.body, "thinking", "block_binding"))
		assert.Equal(t, "high", smartBodyField(t, seen.body, "output_config", "effort"))
	})

	for _, tc := range []struct{ name, extra string }{
		{"thinking type twice", `"thinking":{"type":"enabled","type":"disabled"}`},
		{"thinking budget_tokens twice", `"thinking":{"type":"enabled","budget_tokens":2048,"budget_tokens":4096}`},
	} {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "gpt-5.5", effortFixedHigh, `{}`)
			stub := installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/messages", tokenID: tokenID, body: smartEffortMessagesBody("dup-inner-"+tc.name, tc.extra)})

			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			assert.False(t, seen.ran)
			assert.Zero(t, stub.calls)
		})
	}
}

func addSmartRouteChannel(t *testing.T, id, channelType int, priority int64, group, modelName string) {
	t.Helper()
	weight := uint(100)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: id, Type: channelType, Key: fmt.Sprintf("key-%d", id),
		Status: common.ChannelStatusEnabled, Name: fmt.Sprintf("channel-%d", id),
		Weight: &weight, Models: modelName, Group: group, Priority: &priority,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group: group, Model: modelName, ChannelId: id,
		Enabled: true, Priority: &priority, Weight: weight,
	}).Error)
}

func loadSingleModelSmartRoute(name, endpointsField, modelName string) {
	tier := fmt.Sprintf(`{"model":%q,"groups":["default"]}`, modelName)
	operation_setting.LoadSmartRoutesFromJSONString(fmt.Sprintf(`[{"name":%q,%s"tiers":{"simple":%s,"normal":%s,"hard":%s},
 "default_tier":"normal","scorer":{"model":"m-scorer","groups":["default"],"timeout_ms":2000},
 "advisor":{"model":%q,"groups":["default"],"after_dissatisfied":3},
 "downgrade_max_history_tokens":4000,"confidence_threshold":0.6,"simple_max_message_chars":500}]`,
		name, endpointsField, tier, tier, tier, modelName))
}

func TestSmartRouteDistributeRouteWithoutEndpointsInfersFromModelMetadata(t *testing.T) {
	const chatPath, responsesPath = "/v1/chat/completions", "/v1/responses"
	for _, tc := range []struct {
		name        string
		channelType int
		path, body  string
		wantStatus  int
	}{
		{"chat on a plain openai model", constant.ChannelTypeOpenAI, chatPath, `{"model":"legacy-auto","messages":[{"role":"user","content":"hi"}]}`, http.StatusOK},
		{"responses on a plain openai model is rejected", constant.ChannelTypeOpenAI, responsesPath, `{"model":"legacy-auto","input":"hi"}`, http.StatusBadRequest},
		{"responses on a codex model", constant.ChannelTypeCodex, responsesPath, `{"model":"legacy-auto","input":"hi"}`, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSmartRouteDistributeTest(t)
			addSmartRouteChannel(t, 301, tc.channelType, 0, "default", "legacy-m")
			loadSingleModelSmartRoute("legacy-auto", "", "legacy-m")
			model.InitChannelCache()
			model.RefreshPricing()
			installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{path: tc.path, body: tc.body, tokenID: 9801})

			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			if tc.wantStatus == http.StatusBadRequest {
				assert.Contains(t, recorder.Body.String(), "only supports these endpoints: Chat Completions")
				assert.False(t, seen.ran)
				return
			}
			assert.True(t, seen.ran)
			assert.Equal(t, "legacy-m", seen.originalModel)
		})
	}
}

func TestSmartRouteDistributeAnthropicUnknownModelConvertsBudgetFromTierEffort(t *testing.T) {
	budgets := map[string]float64{}
	for _, tc := range []struct{ effort, tierFields string }{
		{"high", effortFixedHigh},
		{"low", `"effort_mode":"fixed","effort":"low"`},
	} {
		t.Run(tc.effort, func(t *testing.T) {
			tokenID := setupSmartEffortRoute(t, "my-claude", tc.tierFields, `{}`)
			installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)

			recorder, seen := serveSmartRoute(t, smartRequest{
				path: "/v1/messages", tokenID: tokenID,
				body: smartEffortMessagesBody("alias-claude", `"thinking":{"type":"enabled","budget_tokens":10000}`),
			})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, "my-claude", smartBodyField(t, seen.body, "model"))
			assert.Equal(t, tc.effort, smartBodyField(t, seen.body, "output_config", "effort"))
			assert.Equal(t, "enabled", smartBodyField(t, seen.body, "thinking", "type"))
			budget, ok := smartBodyField(t, seen.body, "thinking", "budget_tokens").(float64)
			require.True(t, ok, "enabled thinking without budget_tokens: %s", seen.body)
			assert.GreaterOrEqual(t, budget, 1024.0)
			assert.NotEqual(t, 10000.0, budget)
			budgets[tc.effort] = budget
		})
	}
	if high, low := budgets["high"], budgets["low"]; high != 0 && low != 0 {
		assert.Less(t, low, high)
	}
}

func TestSmartRouteDistributeSkipsChannelsThatRejectTheRequestFormat(t *testing.T) {
	const codexID, openAIID = 311, 312
	setup := func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		addSmartRouteChannel(t, codexID, constant.ChannelTypeCodex, 10, "default", "fmt-m")
		addSmartRouteChannel(t, openAIID, constant.ChannelTypeOpenAI, 0, "default", "fmt-m")
		addSmartRouteChannel(t, 313, constant.ChannelTypeCodex, 0, "default", "fmt-codex-only")
		loadSingleModelSmartRoute("fmt-route", `"endpoints":["openai-response","openai"],`, "fmt-m")
		model.InitChannelCache()
		model.RefreshPricing()
		installSmartScorerStub(t, smartLabels("plan", "high", "neutral"), "", nil)
	}
	chatBody := func(name string) string {
		return fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, name)
	}

	t.Run("chat request picks the channel that accepts chat every time", func(t *testing.T) {
		setup(t)
		for i := range 10 {
			recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/chat/completions", body: chatBody("fmt-route"), tokenID: 9810 + i})

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, openAIID, seen.channelID, "attempt %d", i)
		}
	})

	t.Run("responses request keeps the higher priority channel", func(t *testing.T) {
		setup(t)

		recorder, seen := serveSmartRoute(t, smartRequest{
			path: "/v1/responses", tokenID: 9830,
			body: `{"model":"fmt-route","input":"hi"}`,
		})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, codexID, seen.channelID)
	})

	t.Run("chat request converted to responses by policy counts codex as accepting", func(t *testing.T) {
		setup(t)
		settings := model_setting.GetGlobalSettings()
		original := settings.ChatCompletionsToResponsesPolicy
		t.Cleanup(func() { settings.ChatCompletionsToResponsesPolicy = original })
		settings.ChatCompletionsToResponsesPolicy = model_setting.ChatCompletionsToResponsesPolicy{
			Enabled: true, ChannelTypes: []int{constant.ChannelTypeCodex}, ModelPatterns: []string{"^fmt-"},
		}

		recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/chat/completions", body: chatBody("fmt-route"), tokenID: 9831})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, codexID, seen.channelID)
	})

	t.Run("group with only rejecting channels reports no available channel", func(t *testing.T) {
		setup(t)
		loadSingleModelSmartRoute("fmt-route", `"endpoints":["openai-response","openai"],`, "fmt-codex-only")

		recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/chat/completions", body: chatBody("fmt-route"), tokenID: 9832})

		assert.NotEqual(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Contains(t, recorder.Body.String(), "No available channel for model fmt-codex-only under group default")
		assert.False(t, seen.ran)
	})

	t.Run("direct request for the real model is not filtered", func(t *testing.T) {
		setup(t)

		recorder, seen := serveSmartRoute(t, smartRequest{path: "/v1/chat/completions", body: chatBody("fmt-m"), tokenID: 9833})

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, codexID, seen.channelID)
	})
}

const smartDowngradeBigBase64Len = 1 << 20

func smartDowngradeBodies(imageBase64, longText string) map[string]struct{ path, body string } {
	return map[string]struct{ path, body string }{
		"chat":      {"/v1/chat/completions", fmt.Sprintf(`{"model":"auto-gpt","messages":[{"role":"user","content":"downgrade chat first turn"},{"role":"assistant","content":"ok"},{"role":"user","content":[{"type":"text","text":"look %s"},{"type":"image_url","image_url":{"url":"data:image/png;base64,%s"}}]}]}`, longText, imageBase64)},
		"responses": {"/v1/responses", fmt.Sprintf(`{"model":"auto-gpt","input":[{"role":"user","content":"downgrade responses first turn"},{"role":"assistant","content":"ok"},{"role":"user","content":[{"type":"input_text","text":"look %s"},{"type":"input_image","image_url":"data:image/png;base64,%s"}]}]}`, longText, imageBase64)},
		"messages":  {"/v1/messages", fmt.Sprintf(`{"model":"auto-gpt","max_tokens":64,"messages":[{"role":"user","content":"downgrade messages first turn"},{"role":"assistant","content":"ok"},{"role":"user","content":[{"type":"text","text":"look %s"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"%s"}}]}]}`, longText, imageBase64)},
	}
}

func smartDowngradeBase64(n int) string {
	raw := make([]byte, n*3/4)
	for i := range raw {
		raw[i] = byte(i*7919 ^ i>>3 ^ i*i)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func smartSimpleLabels() *smartroute.ScorerLabels {
	labels := smartLabels("rewrite", "low", "neutral")
	labels.Workload = "one_step"
	return labels
}

func serveSmartDowngrade(t *testing.T, tokenID int, path, body string) (*httptest.ResponseRecorder, *smartDistributeSeen) {
	t.Helper()
	inspection, err := smartroute.InspectRequest(smartroute.RequestEndpointFromPath(path), []byte(body))
	require.NoError(t, err)
	key := smartroute.SessionKey(tokenID, inspection)
	require.NotEmpty(t, key)
	smartroute.SaveSessionAfterSuccess(key, true, "hard", smartroute.EffortDecision{})
	return serveSmartRoute(t, smartRequest{path: path, body: body, auto: true, tokenID: tokenID})
}

func TestSmartRouteDistributeDowngradeIgnoresBinaryPayloads(t *testing.T) {
	bigBase64 := smartDowngradeBase64(smartDowngradeBigBase64Len)
	for _, format := range []string{"chat", "responses", "messages"} {
		t.Run(format+" large image does not block downgrade", func(t *testing.T) {
			setupSmartRouteDistributeTest(t)
			installSmartScorerStub(t, smartSimpleLabels(), "", nil)
			req := smartDowngradeBodies(bigBase64, "short")[format]

			recorder, seen := serveSmartDowngrade(t, 9801, req.path, req.body)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, "t-simple", seen.originalModel)
			assert.NotEqual(t, "downgrade_blocked", seen.routeLog["reason"])
		})
	}

	t.Run("responses reasoning encrypted_content does not block downgrade", func(t *testing.T) {
		setupSmartRouteDistributeTest(t)
		installSmartScorerStub(t, smartSimpleLabels(), "", nil)
		body := fmt.Sprintf(`{"model":"auto-gpt","input":[{"role":"user","content":"downgrade encrypted first turn"},{"type":"reasoning","encrypted_content":%q,"summary":[]},{"role":"assistant","content":"ok"},{"role":"user","content":"look short"}]}`, bigBase64)

		recorder, seen := serveSmartDowngrade(t, 9802, "/v1/responses", body)

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, "t-simple", seen.originalModel)
		assert.NotEqual(t, "downgrade_blocked", seen.routeLog["reason"])
	})
}

func TestSmartRouteDistributeDowngradeCountsConversationText(t *testing.T) {
	longText := strings.Repeat("word ", 30000)
	historyTokens := map[string]int{}
	for _, format := range []string{"chat", "responses", "messages"} {
		for _, imageLen := range []int{16, smartDowngradeBigBase64Len} {
			t.Run(fmt.Sprintf("%s long text with %d byte image stays blocked", format, imageLen), func(t *testing.T) {
				setupSmartRouteDistributeTest(t)
				installSmartScorerStub(t, smartSimpleLabels(), "", nil)
				req := smartDowngradeBodies(smartDowngradeBase64(imageLen), longText)[format]

				recorder, seen := serveSmartDowngrade(t, 9803, req.path, req.body)

				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				assert.Equal(t, "t-hard", seen.originalModel)
				assert.Equal(t, "downgrade_blocked", seen.routeLog["reason"])
				params, ok := seen.routeLog["reason_params"].(map[string]any)
				require.True(t, ok)
				tokens, ok := params["history_tokens"].(int)
				require.True(t, ok)
				assert.GreaterOrEqual(t, tokens, 4000)
				key := fmt.Sprintf("%s/%d", format, imageLen)
				historyTokens[key] = tokens
			})
		}
	}

	for _, format := range []string{"chat", "responses", "messages"} {
		small, big := historyTokens[fmt.Sprintf("%s/16", format)], historyTokens[fmt.Sprintf("%s/%d", format, smartDowngradeBigBase64Len)]
		assert.NotZero(t, small, format)
		assert.Equal(t, small, big, "%s: image base64 length must not change the estimate", format)
	}
}

func TestSmartRouteDistributeDowngradeCountsSystemAndToolText(t *testing.T) {
	longText := strings.Repeat("word ", 30000)
	user := func(text string) map[string]any { return map[string]any{"role": "user", "content": text} }
	assistant := map[string]any{"role": "assistant", "content": "ok"}
	cases := []struct {
		name string
		path string
		body map[string]any
	}{
		{"chat system message", "/v1/chat/completions", map[string]any{"model": "auto-gpt", "messages": []any{
			map[string]any{"role": "system", "content": longText}, user("sys chat first turn"), assistant, user("short")}}},
		{"chat tool_calls arguments", "/v1/chat/completions", map[string]any{"model": "auto-gpt", "messages": []any{
			user("args chat first turn"),
			map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"id": "c1", "type": "function", "function": map[string]any{"name": "f", "arguments": longText}}}},
			map[string]any{"role": "tool", "tool_call_id": "c1", "content": "done"}, user("short")}}},
		{"chat tool message content", "/v1/chat/completions", map[string]any{"model": "auto-gpt", "messages": []any{
			user("result chat first turn"),
			map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"id": "c1", "type": "function", "function": map[string]any{"name": "f", "arguments": "{}"}}}},
			map[string]any{"role": "tool", "tool_call_id": "c1", "content": longText}, user("short")}}},
		{"chat tools definition", "/v1/chat/completions", map[string]any{"model": "auto-gpt", "messages": []any{
			user("tools chat first turn"), assistant, user("short")},
			"tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": "f", "description": longText, "parameters": map[string]any{"type": "object"}}}}}},

		{"responses instructions", "/v1/responses", map[string]any{"model": "auto-gpt", "instructions": longText, "input": []any{
			user("sys responses first turn"), assistant, user("short")}}},
		{"responses function_call arguments", "/v1/responses", map[string]any{"model": "auto-gpt", "input": []any{
			user("args responses first turn"),
			map[string]any{"type": "function_call", "call_id": "c1", "name": "f", "arguments": longText},
			map[string]any{"type": "function_call_output", "call_id": "c1", "output": "done"}, user("short")}}},
		{"responses function_call_output", "/v1/responses", map[string]any{"model": "auto-gpt", "input": []any{
			user("result responses first turn"),
			map[string]any{"type": "function_call", "call_id": "c1", "name": "f", "arguments": "{}"},
			map[string]any{"type": "function_call_output", "call_id": "c1", "output": longText}, user("short")}}},
		{"responses tools definition", "/v1/responses", map[string]any{"model": "auto-gpt", "input": []any{
			user("tools responses first turn"), assistant, user("short")},
			"tools": []any{map[string]any{"type": "function", "name": "f", "description": longText, "parameters": map[string]any{"type": "object"}}}}},

		{"messages system", "/v1/messages", map[string]any{"model": "auto-gpt", "max_tokens": 64, "system": longText, "messages": []any{
			user("sys messages first turn"), assistant, user("short")}}},
		{"messages tool_use input", "/v1/messages", map[string]any{"model": "auto-gpt", "max_tokens": 64, "messages": []any{
			user("args messages first turn"),
			map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "id": "c1", "name": "f", "input": map[string]any{"q": longText}}}},
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "c1", "content": "done"}, map[string]any{"type": "text", "text": "short"}}}}}},
		{"messages tool_result content", "/v1/messages", map[string]any{"model": "auto-gpt", "max_tokens": 64, "messages": []any{
			user("result messages first turn"),
			map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "id": "c1", "name": "f", "input": map[string]any{}}}},
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "c1", "content": longText}, map[string]any{"type": "text", "text": "short"}}}}}},
		{"messages tools definition", "/v1/messages", map[string]any{"model": "auto-gpt", "max_tokens": 64, "messages": []any{
			user("tools messages first turn"), assistant, user("short")},
			"tools": []any{map[string]any{"name": "f", "description": longText, "input_schema": map[string]any{"type": "object"}}}}},
	}
	for i, tc := range cases {
		t.Run(tc.name+" stays blocked", func(t *testing.T) {
			setupSmartRouteDistributeTest(t)
			installSmartScorerStub(t, smartSimpleLabels(), "", nil)
			body, err := common.Marshal(tc.body)
			require.NoError(t, err)

			recorder, seen := serveSmartDowngrade(t, 9810+i, tc.path, string(body))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, "t-hard", seen.originalModel)
			assert.Equal(t, "downgrade_blocked", seen.routeLog["reason"])
			params, ok := seen.routeLog["reason_params"].(map[string]any)
			require.True(t, ok)
			tokens, ok := params["history_tokens"].(int)
			require.True(t, ok)
			assert.GreaterOrEqual(t, tokens, 4000)
		})
	}
}

func TestSmartRouteDistributeDowngradeCountsFileAttachments(t *testing.T) {
	user := func(text string) map[string]any { return map[string]any{"role": "user", "content": text} }
	assistant := map[string]any{"role": "assistant", "content": "ok"}
	repeat := func(n int, item map[string]any) []any {
		items := []any{map[string]any{"type": "text", "text": "short"}}
		for range n {
			items = append(items, item)
		}
		return items
	}
	cases := []struct {
		name string
		path string
		body func(n int, payload string) map[string]any
	}{
		{"messages document base64", "/v1/messages", func(n int, payload string) map[string]any {
			doc := map[string]any{"type": "document", "source": map[string]any{"type": "base64", "media_type": "application/pdf", "data": payload}}
			return map[string]any{"model": "auto-gpt", "max_tokens": 64, "messages": []any{
				user("doc base64 first turn"), assistant, map[string]any{"role": "user", "content": repeat(n, doc)}}}
		}},
		{"messages document text", "/v1/messages", func(n int, payload string) map[string]any {
			doc := map[string]any{"type": "document", "source": map[string]any{"type": "text", "media_type": "text/plain", "data": payload}}
			return map[string]any{"model": "auto-gpt", "max_tokens": 64, "messages": []any{
				user("doc text first turn"), assistant, map[string]any{"role": "user", "content": repeat(n, doc)}}}
		}},
		{"responses input_file file_data", "/v1/responses", func(n int, payload string) map[string]any {
			file := map[string]any{"type": "input_file", "filename": "a.pdf", "file_data": "data:application/pdf;base64," + payload}
			return map[string]any{"model": "auto-gpt", "input": []any{
				user("file_data first turn"), assistant, map[string]any{"role": "user", "content": repeat(n, file)}}}
		}},
	}
	smallPayload, bigPayload := smartDowngradeBase64(16), smartDowngradeBase64(smartDowngradeBigBase64Len)
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			measure := func(t *testing.T, count int, payload string) (string, int) {
				setupSmartRouteDistributeTest(t)
				installSmartScorerStub(t, smartSimpleLabels(), "", nil)
				body, err := common.Marshal(tc.body(count, payload))
				require.NoError(t, err)

				recorder, seen := serveSmartDowngrade(t, 9830+i, tc.path, string(body))

				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				reason, _ := seen.routeLog["reason"].(string)
				params, _ := seen.routeLog["reason_params"].(map[string]any)
				tokens, _ := params["history_tokens"].(int)
				return reason, tokens
			}
			var small, big, two int
			t.Run("one attachment blocks downgrade", func(t *testing.T) {
				var reason string
				reason, small = measure(t, 1, smallPayload)
				assert.Equal(t, "downgrade_blocked", reason)
				assert.GreaterOrEqual(t, small, 4096)
			})
			t.Run("1MB attachment", func(t *testing.T) {
				_, big = measure(t, 1, bigPayload)
			})
			t.Run("two attachments", func(t *testing.T) {
				_, two = measure(t, 2, smallPayload)
			})
			assert.Equal(t, small, big, "attachment size must not change the estimate")
			assert.GreaterOrEqual(t, two, small+4096)
		})
	}
}

const failoverModel = "failover-m"

func setupMultiKeyChannelTest(t *testing.T, id int, mode constant.MultiKeyMode, keyStatuses map[int]int) *model.Channel {
	t.Helper()
	gin.SetMode(gin.TestMode)

	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = true

	weight := uint(100)
	priority := int64(0)
	require.NoError(t, db.Create(&model.Channel{
		Id: id, Type: constant.ChannelTypeClaudeSubscription, Key: "k0\nk1\nk2", Status: common.ChannelStatusEnabled,
		Name: "multi", Weight: &weight, Models: failoverModel, Group: "default", Priority: &priority,
		ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 3, MultiKeyMode: mode, MultiKeyStatusList: keyStatuses},
	}).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: failoverModel, ChannelId: id, Enabled: true, Priority: &priority, Weight: weight}).Error)
	model.InitChannelCache()

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		if sqlDB, err := db.DB(); err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	channel, err := model.CacheGetChannel(id)
	require.NoError(t, err)
	return channel
}

func TestSetupContextForSelectedChannelSkipsTriedKeysAndRecordsSelection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		keyStatuses map[int]int
		wantIndexes []int
	}{
		{"three enabled keys", nil, []int{1, 2}},
		{"a disabled key does not count", map[int]int{1: common.ChannelStatusManuallyDisabled}, []int{2}},
	} {
		for _, mode := range []constant.MultiKeyMode{constant.MultiKeyModeRandom, constant.MultiKeyModePolling} {
			t.Run(fmt.Sprintf("%s/%s", tc.name, mode), func(t *testing.T) {
				channel := setupMultiKeyChannelTest(t, 4201, mode, tc.keyStatuses)
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				common.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
				common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, 0)
				service.StartFailoverLedger(c)

				var usedIndexes []int
				for range tc.wantIndexes {
					require.Nil(t, SetupContextForSelectedChannel(c, channel, failoverModel))
					usedIndexes = append(usedIndexes, common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex))
				}
				assert.ElementsMatch(t, tc.wantIndexes, usedIndexes, "each selection takes a different untried enabled key")

				apiErr := SetupContextForSelectedChannel(c, channel, failoverModel)
				require.NotNil(t, apiErr)
				assert.Equal(t, types.ErrorCodeAllKeysTried, apiErr.GetErrorCode())
			})
		}
	}
}

func TestSetupContextForSelectedChannelWithoutLedgerKeepsPollingBehavior(t *testing.T) {
	channel := setupMultiKeyChannelTest(t, 4401, constant.MultiKeyModePolling, nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.Nil(t, service.GetFailoverLedger(c))

	var indexes []int
	for range 4 {
		require.Nil(t, SetupContextForSelectedChannel(c, channel, failoverModel))
		indexes = append(indexes, common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex))
	}
	assert.Equal(t, []int{0, 1, 2, 0}, indexes)
	assert.Equal(t, 1, channel.ChannelInfo.MultiKeyPollingIndex)
	assert.Nil(t, service.GetFailoverLedger(c), "selecting a channel never creates a ledger")
}
