package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	model.DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	if err := db.AutoMigrate(&model.Channel{}, &model.Ability{}); err != nil {
		panic("failed to migrate: " + err.Error())
	}
	os.Exit(m.Run())
}

func TestDistribute_SaturatedCodexAffinityFallsBackAndKeepsCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM channels").Error)
	resetCodexUsageForMiddlewareTest(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})

	insertDistributorChannel(t, 58001, constant.ChannelTypeCodex)
	insertDistributorChannel(t, 58002, constant.ChannelTypeOpenAI)
	model.InitChannelCache()
	seedDistributorAffinity(t, "pc-saturated", 58001)
	model.CacheSetSubscriptionChannelUsage(58001, 95)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		c.Next()
	})
	router.Use(Distribute())
	router.POST("/v1/responses", func(c *gin.Context) {
		assert.Equal(t, 58002, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
		c.Status(http.StatusOK)
	})

	body := `{"model":"gpt-5","prompt_cache_key":"pc-saturated"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	_, found := preferredDistributorChannel(t, "pc-saturated")
	assert.True(t, found)
}

func insertDistributorChannel(t *testing.T, channelID int, channelType int) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id:     channelID,
		Type:   channelType,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "channel",
		Models: "gpt-5",
		Group:  "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     "default",
		Model:     "gpt-5",
		ChannelId: channelID,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func seedDistributorAffinity(t *testing.T, affinityValue string, channelID int) {
	t.Helper()
	ctx := distributorAffinityContext(affinityValue)
	_, found := service.GetPreferredChannelByAffinity(ctx, "gpt-5", "default")
	require.False(t, found)
	service.RecordChannelAffinity(ctx, channelID)
}

func preferredDistributorChannel(t *testing.T, affinityValue string) (int, bool) {
	t.Helper()
	return service.GetPreferredChannelByAffinity(distributorAffinityContext(affinityValue), "gpt-5", "default")
}

func distributorAffinityContext(affinityValue string) *gin.Context {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	body := `{"model":"gpt-5","prompt_cache_key":"` + affinityValue + `"}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx
}

func resetCodexUsageForMiddlewareTest(t *testing.T) {
	t.Helper()
	model.CacheSetSubscriptionChannelUsage(58001, 0)
	t.Cleanup(func() {
		model.CacheSetSubscriptionChannelUsage(58001, 0)
	})
}
