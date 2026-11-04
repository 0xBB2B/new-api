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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.Channel{}, &model.Ability{}, &model.UserSubscription{}))
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

func TestRunSmartRouteScorer_InsufficientQuotaSkipsUpstream(t *testing.T) {
	f := setupScorerFixture(t, 10)

	outcome, err := runSmartRouteScorer(scorerContext("api"), scorerRoute(5000, "api"), scorerInspection(t))

	require.ErrorIs(t, err, smartroute.ErrScorerInsufficientQuota)
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
	select {
	case <-f.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream request was not cancelled after the scorer timeout")
	}
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
