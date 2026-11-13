package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestSetChannelOtherInfoEntryPreservesExistingKeys(t *testing.T) {
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}))
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
	})

	channel := &Channel{Name: "codex", OtherInfo: `{"status_reason":"quota","status_time":123}`}
	require.NoError(t, db.Create(channel).Error)

	require.NoError(t, SetChannelOtherInfoEntry(channel.Id, "subscription_usage", map[string]any{"plan_type": "plus"}))

	var stored Channel
	require.NoError(t, db.First(&stored, channel.Id).Error)
	otherInfo := stored.GetOtherInfo()
	assert.Equal(t, "quota", otherInfo["status_reason"])
	assert.Equal(t, float64(123), otherInfo["status_time"])
	assert.Equal(t, map[string]any{"plan_type": "plus"}, otherInfo["subscription_usage"])

	require.NoError(t, SetChannelOtherInfoEntry(channel.Id, "subscription_usage", map[string]any{"plan_type": "pro"}))
	require.NoError(t, db.First(&stored, channel.Id).Error)
	assert.Equal(t, map[string]any{"plan_type": "pro"}, stored.GetOtherInfo()["subscription_usage"])
	assert.Equal(t, "quota", stored.GetOtherInfo()["status_reason"])

	assert.Error(t, SetChannelOtherInfoEntry(channel.Id+100, "subscription_usage", "x"))
}

func TestSetChannelOtherInfoEntryKeepsConcurrentWrite(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(filepath.Join(t.TempDir(), "channels.db"))
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "other_info_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			require.NoError(t, db.Migrator().DropTable(&Channel{}))
			require.NoError(t, db.AutoMigrate(&Channel{}))
			previousDB := DB
			previousType := common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			t.Cleanup(func() {
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				assert.NoError(t, db.Migrator().DropTable(&Channel{}))
				assert.NoError(t, sqlDB.Close())
			})
			versionSQL := "SELECT version()"
			if dialect == "sqlite" {
				versionSQL = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(t, db.Raw(versionSQL).Scan(&version).Error)
			t.Logf("database version: %s", version)

			channel := &Channel{Name: "codex", OtherInfo: `{"status_reason":"quota"}`}
			require.NoError(t, db.Create(channel).Error)

			concurrentWritten := false
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("concurrent_other_info_write", func(tx *gorm.DB) {
				if concurrentWritten {
					return
				}
				concurrentWritten = true
				require.NoError(t, db.Session(&gorm.Session{NewDB: true}).Model(&Channel{}).Where("id = ?", channel.Id).
					Update("other_info", `{"status_reason":"quota","balance_detail":{"amount":5}}`).Error)
			}))

			require.NoError(t, SetChannelOtherInfoEntry(channel.Id, "subscription_usage", map[string]any{"plan_type": "plus"}))

			var stored Channel
			require.NoError(t, db.First(&stored, channel.Id).Error)
			otherInfo := stored.GetOtherInfo()
			assert.Equal(t, map[string]any{"amount": float64(5)}, otherInfo["balance_detail"])
			assert.Equal(t, map[string]any{"plan_type": "plus"}, otherInfo["subscription_usage"])
			assert.Equal(t, "quota", otherInfo["status_reason"])
		})
	}
}
