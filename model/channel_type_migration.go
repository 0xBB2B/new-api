package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

// 上游先后把渠道类型 59 分配给 Sub2API、60 分配给 New API，本仓库的 Claude 订阅渠道相应从 59 迁到 60、再迁到 61。
// 每个阶段必须一次性执行：重复执行会把此后新建的同号上游渠道一并改号。
var claudeSubscriptionRenumberStages = []struct {
	markerOptionKey string
	supersededType  int
}{
	{markerOptionKey: "migration.channel_type.claude_subscription_60", supersededType: 59},
	{markerOptionKey: "migration.channel_type.claude_subscription_61", supersededType: 60},
}

// MigrateClaudeSubscriptionChannelType 把存量 Claude 订阅渠道逐阶段改号到当前的 ChannelTypeClaudeSubscription。
func MigrateClaudeSubscriptionChannelType() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	for _, stage := range claudeSubscriptionRenumberStages {
		err := DB.Transaction(func(tx *gorm.DB) error {
			var marker Option
			err := tx.Where(&Option{Key: stage.markerOptionKey}).First(&marker).Error
			if err == nil {
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			result := tx.Model(&Channel{}).
				Where("type = ?", stage.supersededType).
				Update("type", constant.ChannelTypeClaudeSubscription)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				common.SysLog(fmt.Sprintf("renumbered %d Claude subscription channels from type %d to %d",
					result.RowsAffected, stage.supersededType, constant.ChannelTypeClaudeSubscription))
			}
			return tx.Create(&Option{Key: stage.markerOptionKey, Value: "1"}).Error
		})
		if err != nil {
			return err
		}
	}
	return nil
}
