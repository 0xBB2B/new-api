package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	newapidto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	scorerTestUserID   = 7001
	scorerTestQuota    = 1_000_000
	scorerTestRequest  = "req-scorer-test"
	scorerLabelsJSON   = `{"task_type":"plan","reasoning":"high","workload":"few_steps","risk":"low","domain":"code","confidence":0.9,"feedback":["neutral"]}`
	scorerChargeAtRate = 10000 // 0.01 美元 × QuotaPerUnit(500000) × 分组倍率 2
)

type scorerFixture struct {
	db        *gorm.DB
	upstream  *httptest.Server
	hits      atomic.Int32
	bodies    chan []byte
	paths     chan string
	failEvent bool
	status    int
	content   string
	delay     time.Duration
	cancelled chan struct{}
}

func setupScorerFixture(t *testing.T, userQuota int) *scorerFixture {
	t.Helper()
	require.NoError(t, i18n.Init())

	f := &scorerFixture{status: http.StatusOK, content: scorerLabelsJSON, bodies: make(chan []byte, 4), paths: make(chan string, 4), cancelled: make(chan struct{}, 1)}
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.Channel{}, &model.Ability{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.SubscriptionPreConsumeRecord{}))
	f.db = db

	f.upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		f.bodies <- body
		f.paths <- r.URL.Path
		select {
		case <-time.After(f.delay):
		case <-r.Context().Done():
			select {
			case f.cancelled <- struct{}{}:
			default:
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.status)
		if f.status != http.StatusOK {
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
			return
		}
		content, err := common.Marshal(f.content)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/responses") {
			w.Header().Set("Content-Type", "text/event-stream")
			if f.failEvent {
				_, _ = fmt.Fprint(w, "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_1\",\"status\":\"failed\",\"error\":{\"code\":\"server_error\",\"message\":\"boom\"}}}\n\n")
				return
			}
			_, _ = fmt.Fprintf(w, "data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":{\"type\":\"reasoning\",\"id\":\"rs_1\",\"summary\":[]}}\n\n")
			_, _ = fmt.Fprintf(w, "data: {\"type\":\"response.output_item.done\",\"output_index\":1,\"item\":{\"type\":\"message\",\"id\":\"msg_1\",\"status\":\"completed\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":%s}]}}\n\n", content)
			_, _ = fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"created_at\":1,\"status\":\"completed\",\"model\":\"scorer-x\",\"output\":[],\"usage\":{\"input_tokens\":100,\"output_tokens\":20,\"total_tokens\":120}}}\n\n")
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":"x","object":"chat.completion","created":1,"model":"scorer-x","choices":[{"index":0,"message":{"role":"assistant","content":%s},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120}}`, content)
	}))

	priority, weight := int64(0), uint(100)
	baseURL := f.upstream.URL
	require.NoError(t, db.Create(&model.Channel{
		Id: 1, Type: constant.ChannelTypeOpenAI, Key: "sk-placeholder", Status: common.ChannelStatusEnabled,
		Name: "scorer-channel", Models: "scorer-x", Group: "api", BaseURL: &baseURL, Weight: &weight, Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "api", Model: "scorer-x", ChannelId: 1, Enabled: true, Priority: &priority, Weight: weight}).Error)
	require.NoError(t, db.Create(&model.User{Id: scorerTestUserID, Username: "scorer-user", Status: common.UserStatusEnabled, Quota: userQuota, Group: "default"}).Error)

	prevDB, prevLogDB := model.DB, model.LOG_DB
	prevMainType := common.MainDatabaseType()
	prevCache := common.MemoryCacheEnabled
	prevBatch, prevLogConsume := common.BatchUpdateEnabled, common.LogConsumeEnabled
	prevUsable, prevRatios, prevPrices := setting.UserUsableGroups2JSONString(), ratio_setting.GroupRatio2JSONString(), ratio_setting.ModelPrice2JSONString()
	model.DB, model.LOG_DB = db, db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled, common.LogConsumeEnabled = false, true
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"d","api":"a"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"api":2,"vip":3}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"scorer-x":0.01}`))
	model.InitChannelCache()
	service.InitHttpClient()
	t.Cleanup(func() {
		model.DB, model.LOG_DB = prevDB, prevLogDB
		common.SetMainDatabaseType(prevMainType)
		common.MemoryCacheEnabled = prevCache
		common.BatchUpdateEnabled, common.LogConsumeEnabled = prevBatch, prevLogConsume
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(prevUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(prevRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(prevPrices))
		require.NoError(t, sqlDB.Close())
	})
	t.Cleanup(f.upstream.Close)
	return f
}

func scorerContext(usingGroup string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(common.RequestIdKey, scorerTestRequest)
	common.SetContextKey(c, constant.ContextKeyUserId, scorerTestUserID)
	common.SetContextKey(c, constant.ContextKeyTokenId, 1)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
	common.SetContextKey(c, constant.ContextKeyTokenUnlimited, true)
	return c
}

func scorerRoute(timeoutMs int, groups ...string) operation_setting.SmartRoute {
	return operation_setting.SmartRoute{
		Name:    "smart-a",
		Scorer:  operation_setting.SmartRouteScorer{Model: "scorer-x", Groups: groups, TimeoutMs: timeoutMs},
		Advisor: operation_setting.SmartRouteAdvisor{AfterDissatisfied: 2},
	}
}

func scorerInspection(t *testing.T) smartroute.Inspection {
	t.Helper()
	in, err := smartroute.InspectRequest("openai", []byte(`{"messages":[{"role":"user","content":"design a billing system"}],"temperature":1.5,"max_tokens":99999,"stream":true}`))
	require.NoError(t, err)
	return in
}

func scorerUserQuota(t *testing.T) int {
	t.Helper()
	quota, err := model.GetUserQuota(scorerTestUserID, true)
	require.NoError(t, err)
	return quota
}

func scorerConsumeLogs(t *testing.T, db *gorm.DB) []model.Log {
	t.Helper()
	var logs []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeConsume).Find(&logs).Error)
	return logs
}

func TestSmartRouteScorerIsRegistered(t *testing.T) {
	assert.NotNil(t, smartroute.ScorerRunner)
}

func TestRunSmartRouteScorer_ValidLabelsAreBilledAtSettle(t *testing.T) {
	tests := []struct {
		name       string
		usingGroup string
		groups     []string
	}{
		{"fixed token group", "api", []string{"api"}},
		{"auto token skips groups the user may not use", "auto", []string{"vip", "api"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setupScorerFixture(t, scorerTestQuota)

			outcome, err := runSmartRouteScorer(scorerContext(tt.usingGroup), scorerRoute(5000, tt.groups...), scorerInspection(t))

			require.NoError(t, err)
			require.Empty(t, outcome.Failure)
			require.NotNil(t, outcome.Labels)
			assert.Equal(t, "plan", outcome.Labels.TaskType)
			assert.Equal(t, 0.9, outcome.Labels.Confidence)
			require.NotNil(t, outcome.Settle)

			outcome.Settle("hard", "gpt-x")

			logs := scorerConsumeLogs(t, f.db)
			require.Len(t, logs, 1)
			assert.Equal(t, "scorer-x", logs[0].ModelName)
			assert.Equal(t, scorerTestRequest, logs[0].RequestId)
			assert.Equal(t, scorerChargeAtRate, logs[0].Quota)
			assert.Equal(t, scorerTestQuota-scorerChargeAtRate, scorerUserQuota(t))
			var other map[string]any
			require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
			assert.Equal(t, map[string]any{"role": "scorer", "virtual_model": "smart-a", "main_tier": "hard", "main_model": "gpt-x"}, other["smart_routing"])
		})
	}
}

func TestRunSmartRouteScorer_PlaygroundWithoutTokenIsBilledToTheUser(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	c := scorerContext("api")
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	common.SetContextKey(c, constant.ContextKeyTokenId, 0)
	common.SetContextKey(c, constant.ContextKeyTokenUnlimited, false)

	outcome, err := runSmartRouteScorer(c, scorerRoute(5000, "api"), scorerInspection(t))

	require.NoError(t, err)
	require.Empty(t, outcome.Failure)
	require.NotNil(t, outcome.Labels)
	require.NotNil(t, outcome.Settle)

	outcome.Settle("hard", "gpt-x")

	logs := scorerConsumeLogs(t, f.db)
	require.Len(t, logs, 1)
	assert.Equal(t, "scorer-x", logs[0].ModelName)
	assert.Equal(t, scorerChargeAtRate, logs[0].Quota)
	assert.Equal(t, scorerTestQuota-scorerChargeAtRate, scorerUserQuota(t))
}

func TestRunSmartRouteScorer_SendsFixedChatRequest(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	in := scorerInspection(t)

	_, err := runSmartRouteScorer(scorerContext("api"), scorerRoute(5000, "api"), in)

	require.NoError(t, err)
	raw := <-f.bodies
	var fields map[string]any
	require.NoError(t, common.Unmarshal(raw, &fields))
	assert.NotContains(t, fields, "max_tokens")
	assert.NotContains(t, fields, "max_completion_tokens")
	assert.NotContains(t, fields, "reasoning_effort")
	var sent struct {
		Model       string  `json:"model"`
		Temperature float64 `json:"temperature"`
		Stream      bool    `json:"stream"`
		Messages    []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, common.Unmarshal(raw, &sent))
	system, user := smartroute.BuildScorerMessages(smartroute.BuildScorerInput(in, 2), in.Features)
	assert.Equal(t, "scorer-x", sent.Model)
	assert.Equal(t, 0.0, sent.Temperature)
	assert.False(t, sent.Stream)
	require.Len(t, sent.Messages, 2)
	assert.Equal(t, "system", sent.Messages[0].Role)
	assert.Equal(t, system, sent.Messages[0].Content)
	assert.Equal(t, "user", sent.Messages[1].Role)
	assert.Equal(t, user, sent.Messages[1].Content)
}

func TestRunSmartRouteScorer_NonJSONContentIsInvalidButStillBilled(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	f.content = "I think this is a planning task."

	outcome, err := runSmartRouteScorer(scorerContext("api"), scorerRoute(5000, "api"), scorerInspection(t))

	require.NoError(t, err)
	assert.Equal(t, smartroute.FailureScorerInvalid, outcome.Failure)
	assert.Nil(t, outcome.Labels)
	require.NotNil(t, outcome.Settle)

	outcome.Settle("normal", "gpt-x")

	logs := scorerConsumeLogs(t, f.db)
	require.Len(t, logs, 1)
	assert.Equal(t, scorerChargeAtRate, logs[0].Quota)
	assert.Equal(t, scorerTestQuota-scorerChargeAtRate, scorerUserQuota(t))
}

func TestRunSmartRouteScorer_NoUsageMeansRefundAndNoLog(t *testing.T) {
	tests := []struct {
		name        string
		timeoutMs   int
		groups      []string
		setup       func(f *scorerFixture)
		wantFailure string
	}{
		{"upstream 500", 5000, []string{"api"}, func(f *scorerFixture) { f.status = http.StatusInternalServerError }, smartroute.FailureScorerError},
		{"upstream slower than timeout_ms", 100, []string{"api"}, func(f *scorerFixture) { f.delay = 500 * time.Millisecond }, smartroute.FailureScorerTimeout},
		{"no scorer channel in the token group", 5000, []string{"api"}, func(f *scorerFixture) {
			f.db.Exec("DELETE FROM abilities")
			f.db.Exec("DELETE FROM channels")
			model.InitChannelCache()
		}, smartroute.FailureScorerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setupScorerFixture(t, scorerTestQuota)
			tt.setup(f)

			outcome, err := runSmartRouteScorer(scorerContext("api"), scorerRoute(tt.timeoutMs, tt.groups...), scorerInspection(t))

			require.NoError(t, err)
			assert.Equal(t, tt.wantFailure, outcome.Failure)
			assert.Nil(t, outcome.Labels)
			assert.Nil(t, outcome.Settle)
			assert.Empty(t, scorerConsumeLogs(t, f.db))
			assert.Equal(t, scorerTestQuota, scorerUserQuota(t))
		})
	}
}

func seedScorerSubscription(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	require.NoError(t, db.Create(&model.SubscriptionPlan{Id: 1, Title: "plan"}).Error)
	require.NoError(t, db.Create(&model.UserSubscription{
		Id: 1, UserId: scorerTestUserID, PlanId: 1, AmountTotal: scorerTestQuota, Status: "active",
		StartTime: time.Now().Unix(), EndTime: time.Now().Add(time.Hour).Unix(),
	}).Error)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"scorer-x":0.01,"main-x":0.03}`))
}

func subscriptionFirstContext() *gin.Context {
	c := scorerContext("api")
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "subscription_first"})
	return c
}

func settleMainRequest(t *testing.T, c *gin.Context, actualQuota int) {
	t.Helper()
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "main-x")
	request := &dto.GeneralOpenAIRequest{Model: "main-x"}
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, request, nil)
	require.NoError(t, err)
	apiErr := PrepareRequestBilling(c, info)
	if apiErr != nil {
		require.FailNow(t, apiErr.Error())
	}
	require.NoError(t, service.SettleBilling(c, info, actualQuota))
}

func subscriptionAmountUsed(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var sub model.UserSubscription
	require.NoError(t, db.First(&sub, 1).Error)
	return sub.AmountUsed
}

func TestRunSmartRouteScorer_SubscriptionBillingIsIndependentFromMainRequest(t *testing.T) {
	const mainActualQuota = 25000

	t.Run("scorer and main request each reserve and settle", func(t *testing.T) {
		f := setupScorerFixture(t, scorerTestQuota)
		seedScorerSubscription(t, f.db)
		outcome, err := runSmartRouteScorer(subscriptionFirstContext(), scorerRoute(5000, "api"), scorerInspection(t))
		require.NoError(t, err)
		require.NotNil(t, outcome.Settle)

		settleMainRequest(t, subscriptionFirstContext(), mainActualQuota)
		outcome.Settle("hard", "main-x")

		assert.Equal(t, int64(scorerChargeAtRate+mainActualQuota), subscriptionAmountUsed(t, f.db))
		logs := scorerConsumeLogs(t, f.db)
		require.Len(t, logs, 1)
		assert.Equal(t, scorerTestRequest, logs[0].RequestId)
	})

	t.Run("scorer timeout refunds without blocking the main request", func(t *testing.T) {
		f := setupScorerFixture(t, scorerTestQuota)
		seedScorerSubscription(t, f.db)
		f.delay = 500 * time.Millisecond
		outcome, err := runSmartRouteScorer(subscriptionFirstContext(), scorerRoute(100, "api"), scorerInspection(t))
		require.NoError(t, err)
		require.Equal(t, smartroute.FailureScorerTimeout, outcome.Failure)

		settleMainRequest(t, subscriptionFirstContext(), mainActualQuota)

		assert.Equal(t, int64(mainActualQuota), subscriptionAmountUsed(t, f.db))
		assert.Empty(t, scorerConsumeLogs(t, f.db))
	})
}

func addScorerChannel(t *testing.T, f *scorerFixture, id, channelType int, key, group, baseURL string) {
	t.Helper()
	priority, weight := int64(0), uint(100)
	require.NoError(t, f.db.Create(&model.Channel{
		Id: id, Type: channelType, Key: key, Status: common.ChannelStatusEnabled,
		Name: fmt.Sprintf("channel-%d", id), Models: "scorer-x", Group: group, BaseURL: &baseURL, Weight: &weight, Priority: &priority,
	}).Error)
	require.NoError(t, f.db.Create(&model.Ability{Group: group, Model: "scorer-x", ChannelId: id, Enabled: true, Priority: &priority, Weight: weight}).Error)
	model.InitChannelCache()
}

func TestRunSmartRouteScorer_SkipsChannelsWithoutAnyScorerEndpoint(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	addScorerChannel(t, f, 2, constant.ChannelTypeReplicate, "sk-placeholder", "default", "http://127.0.0.1:1")

	outcome, err := runSmartRouteScorer(scorerContext("auto"), scorerRoute(5000, "default", "api"), scorerInspection(t))

	require.NoError(t, err)
	assert.Empty(t, outcome.Failure)
	assert.Equal(t, int32(1), f.hits.Load())
	assert.Equal(t, "/v1/chat/completions", <-f.paths)
}

func TestRunSmartRouteScorer_CodexChannelGetsResponsesRequest(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	addScorerChannel(t, f, 2, constant.ChannelTypeCodex, `{"access_token":"tok-placeholder","account_id":"acct-placeholder"}`, "default", f.upstream.URL)
	in := scorerInspection(t)

	outcome, err := runSmartRouteScorer(scorerContext("auto"), scorerRoute(5000, "default", "api"), in)

	require.NoError(t, err)
	require.Empty(t, outcome.Failure)
	require.NotNil(t, outcome.Labels)
	assert.Equal(t, "plan", outcome.Labels.TaskType)
	assert.Equal(t, 0.9, outcome.Labels.Confidence)
	assert.Equal(t, int32(1), f.hits.Load())
	assert.True(t, strings.HasSuffix(<-f.paths, "/responses"))

	var sent struct {
		Model           string  `json:"model"`
		MaxOutputTokens *int    `json:"max_output_tokens"`
		Reasoning       any     `json:"reasoning"`
		Stream          bool    `json:"stream"`
		Messages        []any   `json:"messages"`
		MaxTokens       *int    `json:"max_tokens"`
		Input           any     `json:"input"`
		Instructions    string  `json:"instructions"`
		Temperature     float64 `json:"temperature"`
	}
	require.NoError(t, common.Unmarshal(<-f.bodies, &sent))
	system, user := smartroute.BuildScorerMessages(smartroute.BuildScorerInput(in, 2), in.Features)
	assert.Equal(t, "scorer-x", sent.Model)
	assert.Nil(t, sent.MaxOutputTokens)
	assert.Nil(t, sent.Reasoning)
	assert.Empty(t, sent.Messages)
	assert.Nil(t, sent.MaxTokens)
	assert.Contains(t, fmt.Sprint(sent.Input, sent.Instructions), user)
	assert.Contains(t, fmt.Sprint(sent.Input, sent.Instructions), system)
	items, isList := sent.Input.([]any)
	require.True(t, isList, "input must be a list of messages, got %T", sent.Input)
	require.NotEmpty(t, items)
	first, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", first["role"])
	assert.IsType(t, "", first["content"])

	require.NotNil(t, outcome.Settle)
	outcome.Settle("hard", "gpt-x")

	logs := scorerConsumeLogs(t, f.db)
	require.Len(t, logs, 1)
	assert.Equal(t, "scorer-x", logs[0].ModelName)
	assert.Equal(t, scorerTestRequest, logs[0].RequestId)
	assert.Equal(t, 100, logs[0].PromptTokens)
	assert.Equal(t, 20, logs[0].CompletionTokens)
	assert.Equal(t, scorerChargeAtRate/2, logs[0].Quota) // default 分组倍率 1，是 api 分组倍率 2 的一半
}

func TestRunSmartRouteScorer_CodexUpstreamFailureRefundsAndWritesNoLog(t *testing.T) {
	failures := []struct {
		name  string
		setup func(f *scorerFixture)
	}{
		{"response.failed event", func(f *scorerFixture) { f.failEvent = true }},
		{"HTTP 500", func(f *scorerFixture) { f.status = http.StatusInternalServerError }},
	}
	for _, failure := range failures {
		t.Run(failure.name+" with wallet billing", func(t *testing.T) {
			f := setupScorerFixture(t, scorerTestQuota)
			addScorerChannel(t, f, 2, constant.ChannelTypeCodex, `{"access_token":"tok-placeholder","account_id":"acct-placeholder"}`, "default", f.upstream.URL)
			failure.setup(f)

			outcome, err := runSmartRouteScorer(scorerContext("auto"), scorerRoute(5000, "default", "api"), scorerInspection(t))

			require.NoError(t, err)
			assert.Equal(t, smartroute.FailureScorerError, outcome.Failure)
			assert.Nil(t, outcome.Labels)
			assert.Nil(t, outcome.Settle)
			assert.True(t, strings.HasSuffix(<-f.paths, "/responses"))
			assert.Empty(t, scorerConsumeLogs(t, f.db))
			assert.Equal(t, scorerTestQuota, scorerUserQuota(t))
		})

		t.Run(failure.name+" with subscription billing", func(t *testing.T) {
			const mainActualQuota = 25000
			f := setupScorerFixture(t, scorerTestQuota)
			seedScorerSubscription(t, f.db)
			require.NoError(t, f.db.Exec("DELETE FROM abilities WHERE channel_id = 1").Error)
			require.NoError(t, f.db.Exec("DELETE FROM channels WHERE id = 1").Error)
			addScorerChannel(t, f, 2, constant.ChannelTypeCodex, `{"access_token":"tok-placeholder","account_id":"acct-placeholder"}`, "api", f.upstream.URL)
			failure.setup(f)

			outcome, err := runSmartRouteScorer(subscriptionFirstContext(), scorerRoute(5000, "api"), scorerInspection(t))

			require.NoError(t, err)
			assert.Equal(t, smartroute.FailureScorerError, outcome.Failure)
			assert.True(t, strings.HasSuffix(<-f.paths, "/responses"))
			assert.Nil(t, outcome.Settle)
			settleMainRequest(t, subscriptionFirstContext(), mainActualQuota)
			assert.Equal(t, int64(mainActualQuota), subscriptionAmountUsed(t, f.db))
			assert.Empty(t, scorerConsumeLogs(t, f.db))
		})
	}
}

func TestRunSmartRouteScorer_StripsThinkingSuffixFromUpstreamModel(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		wantModel  string
		wantEffort string
	}{
		{"@thinking modifier", "scorer-x@thinking:off", "scorer-x", ""},
		{"OpenAI effort suffix", "gpt-5.4-mini-low", "gpt-5.4-mini", "low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setupScorerFixture(t, scorerTestQuota)
			priority, weight := int64(0), uint(100)
			require.NoError(t, f.db.Model(&model.Channel{}).Where("id = ?", 1).Update("models", "scorer-x,"+tt.model).Error)
			require.NoError(t, f.db.Create(&model.Ability{Group: "api", Model: tt.model, ChannelId: 1, Enabled: true, Priority: &priority, Weight: weight}).Error)
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(fmt.Sprintf(`{"scorer-x":0.01,%q:0.01}`, tt.model)))
			model.InitChannelCache()
			route := scorerRoute(5000, "api")
			route.Scorer.Model = tt.model

			_, err := runSmartRouteScorer(scorerContext("api"), route, scorerInspection(t))

			require.NoError(t, err)
			var sent struct {
				Model           string `json:"model"`
				ReasoningEffort string `json:"reasoning_effort"`
			}
			require.NoError(t, common.Unmarshal(<-f.bodies, &sent))
			assert.Equal(t, tt.wantModel, sent.Model)
			if tt.wantEffort != "" {
				assert.Equal(t, tt.wantEffort, sent.ReasoningEffort)
			}
		})
	}
}

func TestRunSmartRouteScorer_InsufficientQuotaSkipsUpstream(t *testing.T) {
	f := setupScorerFixture(t, 10)

	outcome, err := runSmartRouteScorer(scorerContext("api"), scorerRoute(5000, "api"), scorerInspection(t))

	require.ErrorIs(t, err, smartroute.ErrScorerInsufficientQuota)
	ordinaryRequest := &dto.GeneralOpenAIRequest{Model: "scorer-x"}
	ordinaryCtx := scorerContext("api")
	common.SetContextKey(ordinaryCtx, constant.ContextKeyOriginalModel, "scorer-x")
	ordinaryInfo, infoErr := relaycommon.GenRelayInfo(ordinaryCtx, types.RelayFormatOpenAI, ordinaryRequest, nil)
	require.NoError(t, infoErr)
	ordinaryErr := PrepareRequestBilling(ordinaryCtx, ordinaryInfo)
	require.NotNil(t, ordinaryErr)
	assert.Contains(t, ordinaryErr.Error(), "remaining")
	assert.Contains(t, err.Error(), ordinaryErr.Error())
	assert.Nil(t, outcome.Settle)
	assert.Zero(t, f.hits.Load())
	assert.Empty(t, scorerConsumeLogs(t, f.db))
	assert.Equal(t, 10, scorerUserQuota(t))
}

func TestRunSmartRouteScorer_TimeoutCancelsUpstreamRequest(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	f.delay = 5 * time.Second

	outcome, err := runSmartRouteScorer(scorerContext("api"), scorerRoute(100, "api"), scorerInspection(t))

	require.NoError(t, err)
	assert.Equal(t, smartroute.FailureScorerTimeout, outcome.Failure)
	require.Eventually(t, func() bool {
		select {
		case <-f.cancelled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond, "upstream request was not cancelled after the scorer timeout")
}

func TestRunSmartRouteScorer_HistoryFollowsConfiguredN(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	in, err := smartroute.InspectRequest("openai", []byte(`{"messages":[{"role":"user","content":"one"},{"role":"assistant","content":"a"},{"role":"user","content":"two"},{"role":"assistant","content":"a"},{"role":"user","content":"three"},{"role":"assistant","content":"a"},{"role":"user","content":"four"}]}`))
	require.NoError(t, err)

	_, err = runSmartRouteScorer(scorerContext("api"), scorerRoute(5000, "api"), in)
	require.NoError(t, err)

	var sent struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, common.Unmarshal(<-f.bodies, &sent))
	require.Len(t, sent.Messages, 2)
	user := sent.Messages[1].Content
	assert.Contains(t, user, "[1] two")
	assert.Contains(t, user, "[2] three")
	assert.Contains(t, user, "[latest] four")
	assert.NotContains(t, user, "one")
}

func prepareResponsesSent(t *testing.T, raw string, channelID int, setting dto.ChannelSettings, result *smartroute.RouteResult) ([]byte, *gin.Context) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	storage, err := common.CreateBodyStorage([]byte(raw))
	require.NoError(t, err)
	t.Cleanup(func() { storage.Close() })
	c.Set(common.KeyBodyStorage, storage)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelId, channelID)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, setting)
	if result != nil {
		smartroute.SetRouteResult(c, *result)
	}

	request := &dto.OpenAIResponsesRequest{}
	require.NoError(t, common.UnmarshalJsonStr(raw, request))
	info := relaycommon.GenRelayInfoResponses(c, request)
	_, body, closer, apiErr := PrepareResponsesRequest(c, info, request)
	require.Nil(t, apiErr)
	defer closer.Close()
	sent, err := io.ReadAll(body)
	require.NoError(t, err)
	return sent, c
}

func responsesInput(items ...string) string {
	return `{"model":"gpt-4o","input":[` + strings.Join(items, ",") + `]}`
}

func reasoningItem(id, rest string) string {
	return `{"type":"reasoning","id":"` + id + `"` + rest + `}`
}

const (
	summaryEnc  = `,"encrypted_content":"enc","summary":[{"type":"summary_text","text":"s"}]`
	summaryOnly = `{"type":"reasoning","summary":[{"type":"summary_text","text":"s"}]}`
	userMessage = `{"type":"message","role":"user","content":"hi"}`
)

func TestPrepareResponsesRequestDecidesReasoningByItsOwnChannelTag(t *testing.T) {
	modes := []struct {
		name    string
		setting dto.ChannelSettings
	}{
		{"converted", dto.ChannelSettings{}},
		{"passthrough", dto.ChannelSettings{PassThroughBodyEnabled: true}},
	}
	tag := func(channelID int, id string) string { return service.EncodeReasoningTag(channelID, id) }
	sentInput := func(sent []byte) string { return string(ojson.GetBytes(sent, "input").Raw) }

	for _, mode := range modes {
		t.Run(mode.name+" same channel restores ids and keeps every other field", func(t *testing.T) {
			var tagged, original []string
			for _, id := range []string{"rs_a", "rs_b", "rs_c", "rs_d"} {
				tagged = append(tagged, reasoningItem(tag(5, id), summaryEnc+`,"status":"completed"`))
				original = append(original, reasoningItem(id, summaryEnc+`,"status":"completed"`))
			}
			raw := responsesInput(append(tagged, userMessage)...)

			sent, c := prepareResponsesSent(t, raw, 5, mode.setting, &smartroute.RouteResult{})

			assert.JSONEq(t, "["+strings.Join(append(original, userMessage), ",")+"]", sentInput(sent))
			result, _ := smartroute.GetRouteResult(c)
			assert.Equal(t, 0, result.StrippedReasoningCount)
		})

		t.Run(mode.name+" other channel keeps only summaries and drops entries without one", func(t *testing.T) {
			items := []string{
				userMessage,
				reasoningItem(tag(8, "rs_1"), summaryEnc),
				reasoningItem(tag(8, "rs_2"), summaryEnc),
				reasoningItem(tag(8, "rs_3"), `,"encrypted_content":"enc","summary":[]`),
				reasoningItem(tag(8, "rs_4"), `,"content":[{"type":"reasoning_text","text":"t1"}],"summary":[]`),
				reasoningItem(tag(8, "rs_5"), `,"content":[{"type":"reasoning_text","text":"t2"}],"summary":[]`),
				reasoningItem(tag(8, "rs_6"), `,"content":[{"type":"reasoning_text","text":"t3"}],"summary":[]`),
				userMessage,
			}

			sent, c := prepareResponsesSent(t, responsesInput(items...), 9, mode.setting, &smartroute.RouteResult{})

			assert.JSONEq(t, "["+strings.Join([]string{userMessage, summaryOnly, summaryOnly, userMessage}, ",")+"]", sentInput(sent))
			result, _ := smartroute.GetRouteResult(c)
			assert.Equal(t, 6, result.StrippedReasoningCount)
			fields, ok := common.GetContextKey(c, constant.ContextKeySmartRouteLog)
			require.True(t, ok)
			assert.Equal(t, 6, fields.(map[string]any)["cleaned_reasoning"])
		})

		t.Run(mode.name+" untagged and tampered ids are cleaned", func(t *testing.T) {
			valid := tag(9, "rs_x")
			middle := len(valid) / 2
			flipped := byte('A')
			if valid[middle] == 'A' {
				flipped = 'B'
			}
			tampered := valid[:middle] + string(flipped) + valid[middle+1:]
			raw := responsesInput(
				reasoningItem("msg_f836e1c0-0b4b-47c7-b1f0-3f05f59b2880", `,"summary":[]`),
				reasoningItem(tampered, summaryEnc),
				userMessage,
			)

			sent, _ := prepareResponsesSent(t, raw, 9, mode.setting, &smartroute.RouteResult{})

			assert.JSONEq(t, "["+summaryOnly+","+userMessage+"]", sentInput(sent))
		})
	}
}

func TestPrepareResponsesRequestDecidesByTagWithOrWithoutRouteResult(t *testing.T) {
	tests := []struct {
		name        string
		tagChannel  int
		result      *smartroute.RouteResult
		wantCleaned bool
	}{
		{"auto route and tag differs", 8, &smartroute.RouteResult{}, true},
		{"auto route and tag equals target", 9, &smartroute.RouteResult{}, false},
		{"no route result and tag equals target", 9, nil, false},
		{"no route result and tag differs", 8, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := responsesInput(reasoningItem(service.EncodeReasoningTag(tt.tagChannel, "rs_1"), summaryEnc), userMessage)
			want := "[" + summaryOnly + "," + userMessage + "]"
			if !tt.wantCleaned {
				want = "[" + reasoningItem("rs_1", summaryEnc) + "," + userMessage + "]"
			}

			sent, _ := prepareResponsesSent(t, raw, 9, dto.ChannelSettings{}, tt.result)

			assert.JSONEq(t, want, string(ojson.GetBytes(sent, "input").Raw))
		})
	}
}

func TestPrepareResponsesRequestCountsOnlyChangedReasoningItems(t *testing.T) {
	const bare = `{"type":"reasoning","summary":[{"type":"summary_text","text":"residue"}]}`
	foreign := reasoningItem(service.EncodeReasoningTag(8, "rs_1"), summaryEnc)
	raw := responsesInput(bare, foreign, userMessage)

	sent, c := prepareResponsesSent(t, raw, 9, dto.ChannelSettings{}, &smartroute.RouteResult{})

	assert.JSONEq(t, "["+bare+","+summaryOnly+","+userMessage+"]", string(ojson.GetBytes(sent, "input").Raw))
	result, ok := smartroute.GetRouteResult(c)
	require.True(t, ok)
	assert.Equal(t, 1, result.StrippedReasoningCount)
	fields, ok := common.GetContextKey(c, constant.ContextKeySmartRouteLog)
	require.True(t, ok)
	assert.Equal(t, 1, fields.(map[string]any)["cleaned_reasoning"])
}

func TestPrepareResponsesRequestServerLogsStrippedCount(t *testing.T) {
	capture := func(t *testing.T, raw string) string {
		var buf bytes.Buffer
		common.LogWriterMu.Lock()
		origOut, origErr := gin.DefaultWriter, gin.DefaultErrorWriter
		gin.DefaultWriter, gin.DefaultErrorWriter = &buf, &buf
		common.LogWriterMu.Unlock()
		t.Cleanup(func() {
			common.LogWriterMu.Lock()
			gin.DefaultWriter, gin.DefaultErrorWriter = origOut, origErr
			common.LogWriterMu.Unlock()
		})
		prepareResponsesSent(t, raw, 9, dto.ChannelSettings{}, &smartroute.RouteResult{})
		common.LogWriterMu.RLock()
		defer common.LogWriterMu.RUnlock()
		return buf.String()
	}
	foreign := reasoningItem(service.EncodeReasoningTag(8, "rs_1"), summaryEnc)

	t.Run("stripped segments are logged", func(t *testing.T) {
		out := capture(t, responsesInput(foreign, foreign, userMessage))

		assert.Contains(t, out, "stripped 2 reasoning segments not issued by this channel")
	})

	t.Run("nothing stripped logs nothing", func(t *testing.T) {
		own := reasoningItem(service.EncodeReasoningTag(9, "rs_1"), summaryEnc)

		out := capture(t, responsesInput(own, userMessage))

		assert.NotContains(t, out, "stripped")
	})
}

func TestPrepareResponsesRequestPassthroughReasoningOnTheWire(t *testing.T) {
	passthrough := dto.ChannelSettings{PassThroughBodyEnabled: true}

	t.Run("no reasoning sends the client body byte for byte", func(t *testing.T) {
		const raw = `{ "model":"gpt-4o",  "big":12345678901234567890,"input":[{"type":"message","role":"user","content":"hi"}]}`

		sent, _ := prepareResponsesSent(t, raw, 6, passthrough, &smartroute.RouteResult{})

		assert.Equal(t, raw, string(sent))
	})

	raw := responsesInput(reasoningItem(service.EncodeReasoningTag(5, "rs_1"), summaryEnc), userMessage)

	t.Run("issuing channel restores the id", func(t *testing.T) {
		sent, _ := prepareResponsesSent(t, raw, 5, passthrough, &smartroute.RouteResult{})

		assert.JSONEq(t, responsesInput(reasoningItem("rs_1", summaryEnc), userMessage), string(sent))
	})

	t.Run("other channel cleans and leaves the cached body alone", func(t *testing.T) {
		sent, c := prepareResponsesSent(t, raw, 6, passthrough, &smartroute.RouteResult{})

		assert.JSONEq(t, responsesInput(summaryOnly, userMessage), string(sent))
		cached, err := common.GetBodyStorage(c)
		require.NoError(t, err)
		cachedBytes, err := cached.Bytes()
		require.NoError(t, err)
		assert.Equal(t, raw, string(cachedBytes))
	})
}

func TestPrepareResponsesRequestRetryAcrossChannelsRecomputesCleanedCount(t *testing.T) {
	raw := responsesInput(
		reasoningItem(service.EncodeReasoningTag(5, "rs_1"), summaryEnc),
		reasoningItem(service.EncodeReasoningTag(5, "rs_2"), summaryEnc),
		userMessage,
	)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	storage, err := common.CreateBodyStorage([]byte(raw))
	require.NoError(t, err)
	defer storage.Close()
	c.Set(common.KeyBodyStorage, storage)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})
	smartroute.SetRouteResult(c, smartroute.RouteResult{})

	attempt := func(channelID int) {
		common.SetContextKey(c, constant.ContextKeyChannelId, channelID)
		request := &dto.OpenAIResponsesRequest{}
		require.NoError(t, common.UnmarshalJsonStr(raw, request))
		info := relaycommon.GenRelayInfoResponses(c, request)
		_, body, closer, apiErr := PrepareResponsesRequest(c, info, request)
		require.Nil(t, apiErr)
		defer closer.Close()
		_, err := io.ReadAll(body)
		require.NoError(t, err)
	}

	attempt(9)
	logFields, ok := common.GetContextKey(c, constant.ContextKeySmartRouteLog)
	require.True(t, ok)
	assert.Equal(t, 2, logFields.(map[string]any)["cleaned_reasoning"])
	result, _ := smartroute.GetRouteResult(c)
	assert.Equal(t, 2, result.StrippedReasoningCount)

	attempt(5)
	logFields, ok = common.GetContextKey(c, constant.ContextKeySmartRouteLog)
	require.True(t, ok)
	assert.NotContains(t, logFields.(map[string]any), "cleaned_reasoning")
	result, _ = smartroute.GetRouteResult(c)
	assert.Equal(t, 0, result.StrippedReasoningCount)
}

func TestRunSmartRouteScorer_IgnoresTheMainRequestFormatFilter(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		channelType int
		key         string
		wantSuffix  string
	}{
		{"chat request with a codex-only scorer group", "openai", constant.ChannelTypeCodex, `{"access_token":"tok-placeholder","account_id":"acct-placeholder"}`, "/responses"},
		{"anthropic request with an xai-only scorer group", "anthropic", constant.ChannelTypeXai, "sk-placeholder", "/chat/completions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setupScorerFixture(t, scorerTestQuota)
			addScorerChannel(t, f, 2, tt.channelType, tt.key, "default", f.upstream.URL)
			parent := scorerContext("auto")
			service.GetChannelConstraints(parent).AddFilter(newapidto.ChannelFilter{
				Kind: newapidto.FilterAcceptsFormat, Format: tt.format, ChatResponsesPolicy: service.ShouldChatCompletionsUseResponsesGlobal,
			})

			outcome, err := runSmartRouteScorer(parent, scorerRoute(5000, "default"), scorerInspection(t))

			require.NoError(t, err)
			require.Empty(t, outcome.Failure)
			require.Equal(t, int32(1), f.hits.Load())
			assert.True(t, strings.HasSuffix(<-f.paths, tt.wantSuffix))
		})
	}
}

func setScorerCapabilities(t *testing.T, overrides string) {
	t.Helper()
	prev := operation_setting.ThinkingCapabilitiesToString()
	operation_setting.LoadThinkingCapabilitiesFromJSONString(overrides)
	t.Cleanup(func() { operation_setting.LoadThinkingCapabilitiesFromJSONString(prev) })
}

func captureServerWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	common.LogWriterMu.Lock()
	prev := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &buf
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = prev
		common.LogWriterMu.Unlock()
	})
	return &buf
}

func TestRunSmartRouteScorer_EffortIsSnappedToModelCapability(t *testing.T) {
	tests := []struct {
		name         string
		capabilities string
		effort       string
		want         any
	}{
		{"supported level is sent as is", `{"scorer-x":{"efforts":["none","low","high","max"]}}`, "none", "none"},
		{"unsupported level snaps to the nearest lower one", `{"scorer-x":{"efforts":["none","low"]}}`, "high", "low"},
		{"tie between two levels picks the higher", `{"scorer-x":{"efforts":["none","high"]}}`, "low", "high"},
		{"model without thinking gets no parameter", `{"scorer-x":{"no_thinking":true}}`, "low", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setupScorerFixture(t, scorerTestQuota)
			setScorerCapabilities(t, tt.capabilities)
			route := scorerRoute(5000, "api")
			route.Scorer.Effort = tt.effort

			_, err := runSmartRouteScorer(scorerContext("api"), route, scorerInspection(t))

			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, common.Unmarshal(<-f.bodies, &fields))
			if tt.want == nil {
				assert.NotContains(t, fields, "reasoning_effort")
			} else {
				assert.Equal(t, tt.want, fields["reasoning_effort"])
			}
			assert.NotContains(t, fields, "max_tokens")
			assert.NotContains(t, fields, "max_completion_tokens")
		})
	}
}

func TestRunSmartRouteScorer_CodexChannelSendsConfiguredEffort(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	setScorerCapabilities(t, `{"scorer-x":{"efforts":["low","medium","high"]}}`)
	addScorerChannel(t, f, 2, constant.ChannelTypeCodex, `{"access_token":"tok-placeholder","account_id":"acct-placeholder"}`, "default", f.upstream.URL)
	route := scorerRoute(5000, "default", "api")
	route.Scorer.Effort = "low"

	_, err := runSmartRouteScorer(scorerContext("auto"), route, scorerInspection(t))

	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(<-f.paths, "/responses"))
	var fields map[string]any
	require.NoError(t, common.Unmarshal(<-f.bodies, &fields))
	assert.Equal(t, map[string]any{"effort": "low"}, fields["reasoning"])
	assert.NotContains(t, fields, "max_output_tokens")
}

func TestRunSmartRouteScorer_InvalidReplyWritesServerWarning(t *testing.T) {
	head := strings.Repeat("甲", 500)
	var parseProbe map[string]any
	nonJSONErr := common.UnmarshalJsonStr("I think this is a planning task.", &parseProbe)
	require.Error(t, nonJSONErr)
	tests := []struct {
		name         string
		content      string
		wantInLog    []string
		wantNotInLog []string
	}{
		{"non-JSON text", "I think this is a planning task.", []string{"smart-a", "scorer-x", "I think this is a planning task.", nonJSONErr.Error()}, nil},
		{"label outside the allowed options", `{"task_type":"nonsense","reasoning":"high","workload":"few_steps","risk":"low","domain":"code","confidence":0.9,"feedback":["neutral"]}`, []string{"smart-a", "scorer-x", "nonsense", "unknown label value"}, nil},
		{"long reply is cut to 500 characters", head + strings.Repeat("乙", 1500), []string{head}, []string{"乙"}},
		{"empty reply", "", []string{"smart-a", "scorer-x", "(empty)"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setupScorerFixture(t, scorerTestQuota)
			f.content = tt.content
			logs := captureServerWarnings(t)

			outcome, err := runSmartRouteScorer(scorerContext("api"), scorerRoute(5000, "api"), scorerInspection(t))

			require.NoError(t, err)
			assert.Equal(t, smartroute.FailureScorerInvalid, outcome.Failure)
			assert.Contains(t, logs.String(), "returned unparsable output")
			for _, want := range tt.wantInLog {
				assert.Contains(t, logs.String(), want)
			}
			for _, unwanted := range tt.wantNotInLog {
				assert.NotContains(t, logs.String(), unwanted)
			}

			require.NotNil(t, outcome.Settle)
			outcome.Settle("normal", "gpt-x")
			consumeLogs := scorerConsumeLogs(t, f.db)
			require.Len(t, consumeLogs, 1)
			if tt.content != "" {
				assert.NotContains(t, fmt.Sprint(consumeLogs[0].Content, consumeLogs[0].Other), tt.content)
			}
		})
	}
}

func TestRunSmartRouteScorer_NoUnparsableWarningUnlessReplyIsInvalid(t *testing.T) {
	tests := []struct {
		name      string
		timeoutMs int
		setup     func(f *scorerFixture)
	}{
		{"valid labels", 5000, func(f *scorerFixture) {}},
		{"upstream 500", 5000, func(f *scorerFixture) { f.status = http.StatusInternalServerError }},
		{"timeout", 100, func(f *scorerFixture) { f.delay = 500 * time.Millisecond }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setupScorerFixture(t, scorerTestQuota)
			tt.setup(f)
			logs := captureServerWarnings(t)

			_, err := runSmartRouteScorer(scorerContext("api"), scorerRoute(tt.timeoutMs, "api"), scorerInspection(t))

			require.NoError(t, err)
			assert.NotContains(t, logs.String(), "returned unparsable output")
		})
	}
}

func TestRunSmartRouteScorer_InvalidReplyCannotForgeLogLines(t *testing.T) {
	const forged = "[ERR] 2026/11/09 - 12:00:00 | SYSTEM | fake line"
	f := setupScorerFixture(t, scorerTestQuota)
	f.content = "oops\n" + forged
	logs := captureServerWarnings(t)

	outcome, err := runSmartRouteScorer(scorerContext("api"), scorerRoute(5000, "api"), scorerInspection(t))

	require.NoError(t, err)
	assert.Equal(t, smartroute.FailureScorerInvalid, outcome.Failure)
	assert.Contains(t, logs.String(), `oops\n`)
	for line := range strings.SplitSeq(logs.String(), "\n") {
		assert.False(t, strings.HasPrefix(line, forged), "forged log line: %q", line)
	}
}

func TestRunSmartRouteScorer_CodexChannelSnapsEffortToCapability(t *testing.T) {
	tests := []struct {
		name          string
		capabilities  string
		effort        string
		wantReasoning any
	}{
		{"unsupported level snaps down", `{"scorer-x":{"efforts":["none","low"]}}`, "high", map[string]any{"effort": "low"}},
		{"model without thinking gets no reasoning field", `{"scorer-x":{"no_thinking":true}}`, "low", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setupScorerFixture(t, scorerTestQuota)
			setScorerCapabilities(t, tt.capabilities)
			addScorerChannel(t, f, 2, constant.ChannelTypeCodex, `{"access_token":"tok-placeholder","account_id":"acct-placeholder"}`, "default", f.upstream.URL)
			route := scorerRoute(5000, "default", "api")
			route.Scorer.Effort = tt.effort

			_, err := runSmartRouteScorer(scorerContext("auto"), route, scorerInspection(t))

			require.NoError(t, err)
			assert.True(t, strings.HasSuffix(<-f.paths, "/responses"))
			var fields map[string]any
			require.NoError(t, common.Unmarshal(<-f.bodies, &fields))
			if tt.wantReasoning == nil {
				assert.NotContains(t, fields, "reasoning")
			} else {
				assert.Equal(t, tt.wantReasoning, fields["reasoning"])
			}
		})
	}
}

func TestRunSmartRouteScorer_BuiltInNoThinkingModelGetsNoEffort(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	priority, weight := int64(0), uint(100)
	require.NoError(t, f.db.Model(&model.Channel{}).Where("id = ?", 1).Update("models", "scorer-x,gpt-4o").Error)
	require.NoError(t, f.db.Create(&model.Ability{Group: "api", Model: "gpt-4o", ChannelId: 1, Enabled: true, Priority: &priority, Weight: weight}).Error)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"scorer-x":0.01,"gpt-4o":0.01}`))
	model.InitChannelCache()
	setScorerCapabilities(t, `{}`)
	route := scorerRoute(5000, "api")
	route.Scorer.Model = "gpt-4o"
	route.Scorer.Effort = "low"

	_, err := runSmartRouteScorer(scorerContext("api"), route, scorerInspection(t))

	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, common.Unmarshal(<-f.bodies, &fields))
	assert.Equal(t, "gpt-4o", fields["model"])
	assert.NotContains(t, fields, "reasoning_effort")
}

func TestPrepareResponsesRequestItemReferenceFollowsChannelTag(t *testing.T) {
	reference := func(id string) string { return `{"type":"item_reference","id":"` + id + `"}` }
	valid := service.EncodeReasoningTag(5, "rs_r")
	flipped := byte('A')
	if valid[len(valid)/2] == 'A' {
		flipped = 'B'
	}
	tampered := valid[:len(valid)/2] + string(flipped) + valid[len(valid)/2+1:]

	tests := []struct {
		name        string
		id          string
		channelID   int
		want        []string
		wantCleaned int
	}{
		{"same channel restores the id", valid, 5, []string{reference("rs_r"), userMessage}, 0},
		{"other channel drops the reference", valid, 6, []string{userMessage}, 1},
		{"untagged id is kept", "msg_1", 5, []string{reference("msg_1"), userMessage}, 0},
		{"tampered tag drops the reference", tampered, 5, []string{userMessage}, 1},
	}
	modes := []struct {
		name    string
		setting dto.ChannelSettings
	}{
		{"converted", dto.ChannelSettings{}},
		{"passthrough", dto.ChannelSettings{PassThroughBodyEnabled: true}},
	}
	for _, mode := range modes {
		for _, tt := range tests {
			t.Run(mode.name+" "+tt.name, func(t *testing.T) {
				raw := responsesInput(reference(tt.id), userMessage)

				sent, c := prepareResponsesSent(t, raw, tt.channelID, mode.setting, &smartroute.RouteResult{})

				assert.JSONEq(t, "["+strings.Join(tt.want, ",")+"]", string(ojson.GetBytes(sent, "input").Raw))
				result, _ := smartroute.GetRouteResult(c)
				assert.Equal(t, tt.wantCleaned, result.StrippedReasoningCount)
			})
		}
	}
}
