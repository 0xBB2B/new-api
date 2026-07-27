package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

const (
	claudeSubscriptionRenumberOptionKey = "migration.channel_type.claude_subscription_60"
	// 上游把渠道类型 59 分配给了 Sub2API，本仓库原先用 59 表示 Claude 订阅渠道。
	supersededClaudeSubscriptionChannelType = 59
)

// MigrateClaudeSubscriptionChannelType 把存量 Claude 订阅渠道从 59 改号到 60。
// 必须一次性执行：重复执行会把此后新建的 Sub2API 渠道一并改号。
func MigrateClaudeSubscriptionChannelType() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var marker Option
		err := tx.Where(&Option{Key: claudeSubscriptionRenumberOptionKey}).First(&marker).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result := tx.Model(&Channel{}).
			Where("type = ?", supersededClaudeSubscriptionChannelType).
			Update("type", constant.ChannelTypeClaudeSubscription)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			common.SysLog(fmt.Sprintf("renumbered %d Claude subscription channels from type %d to %d",
				result.RowsAffected, supersededClaudeSubscriptionChannelType, constant.ChannelTypeClaudeSubscription))
		}
		return tx.Create(&Option{Key: claudeSubscriptionRenumberOptionKey, Value: "1"}).Error
	})
}
