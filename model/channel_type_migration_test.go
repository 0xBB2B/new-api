package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useChannelTypeMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}, &Channel{}))
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
	return db
}

func requireChannelType(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	var channel Channel
	require.NoError(t, db.Where(&Channel{Name: name}).First(&channel).Error)
	return channel.Type
}

func TestMigrateClaudeSubscriptionChannelTypeRenumbersOnlyOnce(t *testing.T) {
	db := useChannelTypeMigrationDB(t)
	require.NoError(t, db.Create(&[]Channel{
		{Name: "legacy-claude", Type: supersededClaudeSubscriptionChannelType},
		{Name: "anthropic", Type: constant.ChannelTypeAnthropic},
	}).Error)

	require.NoError(t, MigrateClaudeSubscriptionChannelType())
	assert.Equal(t, constant.ChannelTypeClaudeSubscription, requireChannelType(t, db, "legacy-claude"))
	assert.Equal(t, constant.ChannelTypeAnthropic, requireChannelType(t, db, "anthropic"))

	require.NoError(t, db.Create(&Channel{Name: "sub2api", Type: constant.ChannelTypeSub2API}).Error)
	require.NoError(t, MigrateClaudeSubscriptionChannelType())
	assert.Equal(t, constant.ChannelTypeSub2API, requireChannelType(t, db, "sub2api"))
}
