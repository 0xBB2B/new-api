package relay

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
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
	status    int
	content   string
	delay     time.Duration
	cancelled chan struct{}
}

func setupScorerFixture(t *testing.T, userQuota int) *scorerFixture {
	t.Helper()
	require.NoError(t, i18n.Init())

	f := &scorerFixture{status: http.StatusOK, content: scorerLabelsJSON, bodies: make(chan []byte, 4), cancelled: make(chan struct{}, 1)}
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

func TestRunSmartRouteScorer_SendsFixedChatRequest(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	in := scorerInspection(t)

	_, err := runSmartRouteScorer(scorerContext("api"), scorerRoute(5000, "api"), in)

	require.NoError(t, err)
	var sent struct {
		Model       string  `json:"model"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
		Stream      bool    `json:"stream"`
		Messages    []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, common.Unmarshal(<-f.bodies, &sent))
	system, user := smartroute.BuildScorerMessages(smartroute.BuildScorerInput(in, 2), in.Features)
	assert.Equal(t, "scorer-x", sent.Model)
	assert.Equal(t, 300, sent.MaxTokens)
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
	assert.Equal(t, "scorer_invalid", outcome.Failure)
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
		{"upstream 500", 5000, []string{"api"}, func(f *scorerFixture) { f.status = http.StatusInternalServerError }, "scorer_error"},
		{"upstream slower than timeout_ms", 100, []string{"api"}, func(f *scorerFixture) { f.delay = 500 * time.Millisecond }, "scorer_timeout"},
		{"no scorer channel in the token group", 5000, []string{"api"}, func(f *scorerFixture) {
			f.db.Exec("DELETE FROM abilities")
			f.db.Exec("DELETE FROM channels")
			model.InitChannelCache()
		}, "scorer_error"},
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
		require.Equal(t, "scorer_timeout", outcome.Failure)

		settleMainRequest(t, subscriptionFirstContext(), mainActualQuota)

		assert.Equal(t, int64(mainActualQuota), subscriptionAmountUsed(t, f.db))
		assert.Empty(t, scorerConsumeLogs(t, f.db))
	})
}

func TestRunSmartRouteScorer_SkipsChannelsWithoutChatCompletions(t *testing.T) {
	f := setupScorerFixture(t, scorerTestQuota)
	priority, weight := int64(0), uint(100)
	deadURL := "http://127.0.0.1:1"
	require.NoError(t, f.db.Create(&model.Channel{
		Id: 2, Type: constant.ChannelTypeCodex, Key: "sk-placeholder", Status: common.ChannelStatusEnabled,
		Name: "codex-channel", Models: "scorer-x", Group: "default", BaseURL: &deadURL, Weight: &weight, Priority: &priority,
	}).Error)
	require.NoError(t, f.db.Create(&model.Ability{Group: "default", Model: "scorer-x", ChannelId: 2, Enabled: true, Priority: &priority, Weight: weight}).Error)
	model.InitChannelCache()

	outcome, err := runSmartRouteScorer(scorerContext("auto"), scorerRoute(5000, "default", "api"), scorerInspection(t))

	require.NoError(t, err)
	assert.Empty(t, outcome.Failure)
	assert.Equal(t, int32(1), f.hits.Load())
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
	assert.Equal(t, "scorer_timeout", outcome.Failure)
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

func TestStripEncryptedReasoningInput(t *testing.T) {
	const mixedInput = `[
		{"type":"message","role":"user","content":"hi","id":"msg_1"},
		{"type":"reasoning","id":"rs_1","encrypted_content":"enc1","summary":[{"type":"summary_text","text":"a"}]},
		{"type":"reasoning","id":"rs_2","encrypted_content":"enc2"},
		{"type":"function_call","call_id":"c1","name":"f","arguments":"{}","id":"fc_1"},
		{"type":"reasoning","id":"rs_3","encrypted_content":"enc3","summary":[{"type":"summary_text","text":"b"}]}
	]`
	const mixedWant = `[
		{"type":"message","role":"user","content":"hi","id":"msg_1"},
		{"type":"reasoning","summary":[{"type":"summary_text","text":"a"}]},
		{"type":"function_call","call_id":"c1","name":"f","arguments":"{}","id":"fc_1"},
		{"type":"reasoning","summary":[{"type":"summary_text","text":"b"}]}
	]`

	tests := []struct {
		name      string
		input     string
		want      string
		wantCount int
	}{
		{"mixed entries", mixedInput, mixedWant, 3},
		{"empty summary array drops entry", `[{"type":"reasoning","id":"rs_1","encrypted_content":"e","summary":[]},{"type":"message","content":"x"}]`, `[{"type":"message","content":"x"}]`, 1},
		{"extra fields on summarized reasoning kept", `[{"type":"reasoning","id":"rs_1","encrypted_content":"e","status":"completed","summary":[{"type":"summary_text","text":"a"}]}]`, `[{"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"a"}]}]`, 1},
		{"no reasoning entries untouched", `[{"type":"message","content":"x","id":"m"}]`, `[{"type":"message","content":"x","id":"m"}]`, 0},
		{"string input untouched", `"hello"`, `"hello"`, 0},
		{"stored reasoning id with summary loses its id", `[{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"a"}]}]`, `[{"type":"reasoning","summary":[{"type":"summary_text","text":"a"}]}]`, 1},
		{"stored reasoning id without summary is dropped", `[{"type":"reasoning","id":"rs_1"},{"type":"message","content":"x"}]`, `[{"type":"message","content":"x"}]`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, count := stripEncryptedReasoningInput([]byte(tt.input))
			assert.Equal(t, tt.wantCount, count)
			assert.JSONEq(t, tt.want, string(got))
		})
	}
}

func TestShouldStripSmartRouteReasoning(t *testing.T) {
	tests := []struct {
		name   string
		result *smartroute.RouteResult
		want   bool
	}{
		{"no route result", nil, false},
		{"session has no record", &smartroute.RouteResult{}, false},
		{"same channel as last", &smartroute.RouteResult{Session: &smartroute.SessionRecord{LastChannelID: 5}}, false},
		{"different channel from last", &smartroute.RouteResult{Session: &smartroute.SessionRecord{LastChannelID: 9}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tt.result != nil {
				smartroute.SetRouteResult(c, *tt.result)
			}
			assert.Equal(t, tt.want, shouldStripSmartRouteReasoning(c, 5))
		})
	}
}

func TestPrepareResponsesRequestPassthroughStripsReasoningOnWire(t *testing.T) {
	raw := []byte(`{"model":"gpt-4o","input":[{"type":"reasoning","id":"rs_1","encrypted_content":"enc"},{"type":"message","role":"user","content":"hi"}]}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	storage, err := common.CreateBodyStorage(raw)
	require.NoError(t, err)
	defer storage.Close()
	c.Set(common.KeyBodyStorage, storage)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelId, 9)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})
	smartroute.SetRouteResult(c, smartroute.RouteResult{Session: &smartroute.SessionRecord{LastChannelID: 5}})

	request := &dto.OpenAIResponsesRequest{Model: "gpt-4o", Input: []byte(`[{"type":"reasoning","id":"rs_1","encrypted_content":"enc"},{"type":"message","role":"user","content":"hi"}]`)}
	info := relaycommon.GenRelayInfoResponses(c, request)
	_, body, closer, apiErr := PrepareResponsesRequest(c, info, request)
	require.Nil(t, apiErr)
	defer closer.Close()

	sent, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gpt-4o","input":[{"type":"message","role":"user","content":"hi"}]}`, string(sent))
	cached, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	cachedBytes, err := cached.Bytes()
	require.NoError(t, err)
	assert.JSONEq(t, string(raw), string(cachedBytes))
	result, ok := smartroute.GetRouteResult(c)
	require.True(t, ok)
	assert.Equal(t, 1, result.StrippedReasoningCount)
}

func TestPrepareResponsesRequestRetryOnLastChannelDropsStaleCleanedReasoningCount(t *testing.T) {
	const input = `[{"type":"reasoning","id":"rs_1","encrypted_content":"e1"},{"type":"reasoning","id":"rs_2","encrypted_content":"e2"},{"type":"message","role":"user","content":"hi"}]`
	raw := []byte(`{"model":"gpt-4o","input":` + input + `}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	storage, err := common.CreateBodyStorage(raw)
	require.NoError(t, err)
	defer storage.Close()
	c.Set(common.KeyBodyStorage, storage)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})
	smartroute.SetRouteResult(c, smartroute.RouteResult{Session: &smartroute.SessionRecord{LastChannelID: 5}})

	attempt := func(channelID int) {
		common.SetContextKey(c, constant.ContextKeyChannelId, channelID)
		request := &dto.OpenAIResponsesRequest{Model: "gpt-4o", Input: []byte(input)}
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

	attempt(5)
	logFields, ok = common.GetContextKey(c, constant.ContextKeySmartRouteLog)
	require.True(t, ok)
	assert.NotContains(t, logFields.(map[string]any), "cleaned_reasoning")
}
