package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
