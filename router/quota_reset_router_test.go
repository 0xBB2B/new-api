package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type quotaResetRunAPIResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ResetCount int `json:"reset_count"`
	} `json:"data"`
}

func TestQuotaResetRunRouteRequiresRootRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.AuditLog{}, &model.CasbinRule{}, &model.AuthzRole{}, &model.UserAccessToken{}))

	previousDB, previousLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })

	require.NoError(t, i18n.Init())
	require.NoError(t, authz.Init(db))

	s := operation_setting.GetQuotaResetSetting()
	originalSetting := *s
	s.Enabled = true
	s.Period = operation_setting.QuotaResetPeriodMonthly
	s.ResetValue = 500000
	t.Cleanup(func() { *s = originalSetting })

	admin := model.User{Username: "quota_reset_router_admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Quota: 1234, AffCode: "quota-reset-router-admin"}
	require.NoError(t, db.Create(&admin).Error)
	adminPAT := createQuotaResetRouterAccessToken(t, admin.Id)

	root := model.User{Username: "quota_reset_router_root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 5678, AffCode: "quota-reset-router-root"}
	require.NoError(t, db.Create(&root).Error)
	rootPAT := createQuotaResetRouterAccessToken(t, root.Id)

	engine := gin.New()
	SetApiRouter(engine)

	adminRecorder := httptest.NewRecorder()
	adminRequest := httptest.NewRequest(http.MethodPost, "/api/user/quota_reset/run", nil)
	adminRequest.Header.Set("Authorization", "Bearer "+adminPAT)
	engine.ServeHTTP(adminRecorder, adminRequest)

	assert.Equal(t, http.StatusForbidden, adminRecorder.Code)
	assert.Equal(t, 1234, fetchQuotaResetRouterUser(t, db, admin.Id).Quota)
	assert.Equal(t, 5678, fetchQuotaResetRouterUser(t, db, root.Id).Quota)
	assert.Empty(t, fetchQuotaResetRouterLogs(t, db, admin.Id))
	assert.Empty(t, fetchQuotaResetRouterLogs(t, db, root.Id))

	rootRecorder := httptest.NewRecorder()
	rootRequest := httptest.NewRequest(http.MethodPost, "/api/user/quota_reset/run", nil)
	rootRequest.Header.Set("Authorization", "Bearer "+rootPAT)
	engine.ServeHTTP(rootRecorder, rootRequest)

	require.Equal(t, http.StatusOK, rootRecorder.Code)
	var response quotaResetRunAPIResponse
	require.NoError(t, common.Unmarshal(rootRecorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 2, response.Data.ResetCount)
	assert.Equal(t, 500000, fetchQuotaResetRouterUser(t, db, admin.Id).Quota)
	assert.Equal(t, 500000, fetchQuotaResetRouterUser(t, db, root.Id).Quota)
}

func createQuotaResetRouterAccessToken(t *testing.T, userID int) string {
	t.Helper()
	suffix, err := common.GenerateRandomCharsKey(43)
	require.NoError(t, err)
	raw := model.AccessTokenPrefix + suffix
	token := &model.UserAccessToken{Name: "quota reset router token", TokenHash: model.AccessTokenFingerprint(raw), TokenHint: model.AccessTokenHint(raw)}
	require.NoError(t, token.SetScopes([]string{"user:write"}))
	require.NoError(t, model.CreateUserAccessToken(userID, token, service.AccessTokenMaxPerUser))
	return raw
}

func fetchQuotaResetRouterUser(t *testing.T, db *gorm.DB, id int) *model.User {
	t.Helper()
	var user model.User
	require.NoError(t, db.First(&user, "id = ?", id).Error)
	return &user
}

func fetchQuotaResetRouterLogs(t *testing.T, db *gorm.DB, userId int) []model.Log {
	t.Helper()
	var logs []model.Log
	require.NoError(t, db.Where("user_id = ?", userId).Order("id").Find(&logs).Error)
	return logs
}
