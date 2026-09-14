package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func seedUserNamesLookupUsers(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Create(&User{Id: 7, Username: "zhangsan", DisplayName: "张三", Password: "password", AffCode: "aff-7"}).Error)
	require.NoError(t, DB.Create(&User{Id: 8, Username: "lisi", DisplayName: "", Password: "password", AffCode: "aff-8"}).Error)
	require.NoError(t, DB.Create(&User{Id: 9, Username: "deleted", Password: "password", AffCode: "aff-9"}).Error)
	require.NoError(t, DB.Delete(&User{Id: 9}).Error)
}

func TestGetUserNamesByIdsReturnsExistingNonDeletedUsersDeduped(t *testing.T) {
	truncateTables(t)
	seedUserNamesLookupUsers(t)

	result, err := GetUserNamesByIds([]int{7, 8, 9, 7, 0})
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, UserNames{Username: "zhangsan", DisplayName: "张三"}, result[7])
	assert.Equal(t, UserNames{Username: "lisi", DisplayName: ""}, result[8])
	_, hasDeleted := result[9]
	assert.False(t, hasDeleted)
	_, hasZero := result[0]
	assert.False(t, hasZero)
}

func TestGetUserNamesByIdsWithoutValidIdsReturnsEmptyMap(t *testing.T) {
	truncateTables(t)
	seedUserNamesLookupUsers(t)

	cases := []struct {
		name string
		ids  []int
	}{
		{name: "all zero", ids: []int{0, 0}},
		{name: "nil", ids: nil},
		{name: "empty slice", ids: []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := GetUserNamesByIds(tc.ids)
			require.NoError(t, err)
			assert.Len(t, result, 0)
		})
	}
}

func seedDisplayNameLogs(t *testing.T) {
	t.Helper()
	for i, log := range []*Log{
		{UserId: 7, Username: "zhangsan", CreatedAt: 1001, Type: LogTypeConsume},
		{UserId: 8, Username: "lisi", CreatedAt: 1002, Type: LogTypeConsume},
		{UserId: 9, Username: "deleted", CreatedAt: 1003, Type: LogTypeConsume},
		{UserId: 7, Username: "zhangsan", CreatedAt: 1004, Type: LogTypeConsume},
	} {
		require.NoErrorf(t, DB.Create(log).Error, "seed log %d", i)
	}
}

func TestGetAllLogsFillsDisplayNameFromUsersTable(t *testing.T) {
	truncateTables(t)
	seedUserNamesLookupUsers(t)
	seedDisplayNameLogs(t)

	logs, total, err := GetAllLogs(0, 0, 0, "", "", "", 0, 10, 0, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	require.Len(t, logs, 4)

	require.Equal(t, 7, logs[0].UserId)
	assert.Equal(t, "张三", logs[0].DisplayName)
	require.Equal(t, 9, logs[1].UserId)
	assert.Equal(t, "", logs[1].DisplayName)
	require.Equal(t, 8, logs[2].UserId)
	assert.Equal(t, "", logs[2].DisplayName)
	require.Equal(t, 7, logs[3].UserId)
	assert.Equal(t, "张三", logs[3].DisplayName)
}

func TestGetUserLogsDoesNotFillDisplayName(t *testing.T) {
	truncateTables(t)
	seedUserNamesLookupUsers(t)
	seedDisplayNameLogs(t)

	logs, total, err := GetUserLogs(7, 0, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, logs, 2)
	for _, log := range logs {
		assert.Equal(t, "", log.DisplayName)
	}
}

func seedDisplayNameAuditLogs(t *testing.T) {
	t.Helper()
	for i, entry := range []*AuditLog{
		{EventId: "display-name-e1", UserId: 7, Username: "zhangsan", ActorRole: common.RoleCommonUser, CreatedAt: 2001, Category: AuditCategoryOperation, Action: "x"},
		{EventId: "display-name-e2", UserId: 9, Username: "deleted", ActorRole: common.RoleCommonUser, CreatedAt: 2002, Category: AuditCategoryOperation, Action: "x"},
		{EventId: "display-name-e3", UserId: 0, Username: "", ActorRole: common.RoleCommonUser, CreatedAt: 2003, Category: AuditCategoryOperation, Action: "x"},
	} {
		require.NoErrorf(t, DB.Create(entry).Error, "seed audit log %d", i)
	}
}

func TestGetAuditLogsFillsDisplayNameForAdminView(t *testing.T) {
	truncateTables(t)
	seedUserNamesLookupUsers(t)
	seedDisplayNameAuditLogs(t)

	logs, total, err := GetAuditLogs(AuditLogFilter{}, 0, 20, common.RoleAdminUser)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, logs, 3)

	byUserId := make(map[int]string, len(logs))
	for _, entry := range logs {
		byUserId[entry.UserId] = entry.DisplayName
	}
	assert.Equal(t, "张三", byUserId[7])
	assert.Equal(t, "", byUserId[9])
	assert.Equal(t, "", byUserId[0])
}

func TestGetAuditLogsSelfViewDoesNotFillDisplayName(t *testing.T) {
	truncateTables(t)
	seedUserNamesLookupUsers(t)
	seedDisplayNameAuditLogs(t)

	logs, total, err := GetAuditLogs(AuditLogFilter{SelfView: true, UserId: 7}, 0, 20, common.RoleCommonUser)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	assert.Equal(t, "", logs[0].DisplayName)
}

func seedDisplayNameQuotaData(t *testing.T) {
	t.Helper()
	seedFlowQuotaData(t, QuotaData{UserID: 7, Username: "zhangsan", UseGroup: "vip", ModelName: "gpt-a", ChannelID: 1, CreatedAt: 1000, Count: 1, Quota: 10, TokenUsed: 5})
	seedFlowQuotaData(t, QuotaData{UserID: 8, Username: "lisi", UseGroup: "vip", ModelName: "gpt-a", ChannelID: 1, CreatedAt: 1000, Count: 1, Quota: 10, TokenUsed: 5})
}

func TestGetFlowQuotaDataFillsDisplayNameForAdminRole(t *testing.T) {
	truncateTables(t)
	seedUserNamesLookupUsers(t)
	seedDisplayNameQuotaData(t)

	rows, err := GetFlowQuotaData(0, 2000, "", 0, common.RoleAdminUser)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byUserId := make(map[int]string, len(rows))
	for _, row := range rows {
		byUserId[row.UserID] = row.DisplayName
	}
	assert.Equal(t, "张三", byUserId[7])
	assert.Equal(t, "", byUserId[8])
}

func TestGetFlowQuotaDataSelfViewDoesNotFillDisplayName(t *testing.T) {
	truncateTables(t)
	seedUserNamesLookupUsers(t)
	seedDisplayNameQuotaData(t)

	rows, err := GetFlowQuotaData(0, 2000, "", 7, common.RoleCommonUser)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "", rows[0].DisplayName)
}

func TestLogJSONOmitsDisplayNameWhenEmptyIncludesWhenSet(t *testing.T) {
	empty, err := common.Marshal(&Log{UserId: 8, DisplayName: ""})
	require.NoError(t, err)
	assert.NotContains(t, string(empty), `"display_name"`)

	filled, err := common.Marshal(&Log{UserId: 7, DisplayName: "张三"})
	require.NoError(t, err)
	assert.Contains(t, string(filled), `"display_name":"张三"`)
}

func TestDisplayNameFillOnRealDatabases(t *testing.T) {
	cases := []struct {
		dialect string
		env     string
		dbType  common.DatabaseType
		open    func(string) gorm.Dialector
	}{
		{dialect: "mysql", env: "TEST_MYSQL_DSN", dbType: common.DatabaseTypeMySQL, open: mysql.Open},
		{dialect: "postgres", env: "TEST_POSTGRES_DSN", dbType: common.DatabaseTypePostgreSQL, open: postgres.Open},
	}
	for _, tc := range cases {
		t.Run(tc.dialect, func(t *testing.T) {
			dsn := os.Getenv(tc.env)
			if dsn == "" {
				t.Skipf("%s is not configured", tc.env)
			}
			db, err := gorm.Open(tc.open(dsn), &gorm.Config{})
			require.NoError(t, err)
			tables := []any{&User{}, &Log{}, &AuditLog{}, &QuotaData{}, &Channel{}, &Token{}}
			require.NoError(t, db.Migrator().DropTable(tables...))
			require.NoError(t, db.AutoMigrate(tables...))

			previousDB, previousLogDB := DB, LOG_DB
			previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
			DB, LOG_DB = db, db
			common.SetDatabaseTypes(tc.dbType, tc.dbType)
			t.Cleanup(func() {
				DB, LOG_DB = previousDB, previousLogDB
				common.SetDatabaseTypes(previousMainType, previousLogType)
				_ = db.Migrator().DropTable(tables...)
				if sqlDB, err := db.DB(); err == nil {
					_ = sqlDB.Close()
				}
			})

			seedUserNamesLookupUsers(t)
			seedDisplayNameLogs(t)
			seedDisplayNameAuditLogs(t)
			seedDisplayNameQuotaData(t)

			names, err := GetUserNamesByIds([]int{7, 8, 9, 7, 0})
			require.NoError(t, err)
			require.Len(t, names, 2)
			assert.Equal(t, UserNames{Username: "zhangsan", DisplayName: "张三"}, names[7])
			assert.Equal(t, UserNames{Username: "lisi", DisplayName: ""}, names[8])

			wantDisplayName := map[int]string{7: "张三", 8: "", 9: "", 0: ""}
			logs, total, err := GetAllLogs(0, 0, 0, "", "", "", 0, 10, 0, "", "", "")
			require.NoError(t, err)
			require.EqualValues(t, 4, total)
			for _, log := range logs {
				assert.Equal(t, wantDisplayName[log.UserId], log.DisplayName, "log user %d", log.UserId)
			}

			audits, _, err := GetAuditLogs(AuditLogFilter{}, 0, 20, common.RoleAdminUser)
			require.NoError(t, err)
			require.Len(t, audits, 3)
			for _, entry := range audits {
				assert.Equal(t, wantDisplayName[entry.UserId], entry.DisplayName, "audit user %d", entry.UserId)
			}

			flows, err := GetFlowQuotaData(0, 2000, "", 0, common.RoleAdminUser)
			require.NoError(t, err)
			require.Len(t, flows, 2)
			for _, row := range flows {
				assert.Equal(t, wantDisplayName[row.UserID], row.DisplayName, "flow user %d", row.UserID)
			}
		})
	}
}
