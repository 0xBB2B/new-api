package service

import (
	"errors"
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const ginKeyFailoverLedger = "failover_ledger"

type FailoverLedger struct {
	tried map[int]map[int]bool
}

// StartFailoverLedger 创建尝试记录，并补记分发中间件已选定的渠道和密钥下标。
func StartFailoverLedger(c *gin.Context) *FailoverLedger {
	ledger := &FailoverLedger{tried: make(map[int]map[int]bool)}
	ledger.Record(common.GetContextKeyInt(c, constant.ContextKeyChannelId), common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex))
	c.Set(ginKeyFailoverLedger, ledger)
	return ledger
}

// GetFailoverLedger 异步任务没有尝试记录，返回 nil。
func GetFailoverLedger(c *gin.Context) *FailoverLedger {
	ledger, _ := c.Get(ginKeyFailoverLedger)
	l, _ := ledger.(*FailoverLedger)
	return l
}

func (l *FailoverLedger) Record(channelID, keyIndex int) {
	if l.tried[channelID] == nil {
		l.tried[channelID] = make(map[int]bool)
	}
	l.tried[channelID][keyIndex] = true
}

func (l *FailoverLedger) ExhaustChannel(channel *model.Channel) {
	for index := range channel.GetKeys() {
		l.Record(channel.Id, index)
	}
}

func (l *FailoverLedger) TriedKeys(channelID int) map[int]bool {
	return l.tried[channelID]
}

// ChannelExhausted 在渠道的启用密钥都试过时返回 true；读密钥状态要和自动禁用密钥的写入互斥。
func (l *FailoverLedger) ChannelExhausted(channel *model.Channel) bool {
	lock := model.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()
	tried := l.tried[channel.Id]
	for _, index := range channel.EnabledKeyIndexes() {
		if !tried[index] {
			return false
		}
	}
	return true
}

// findFailoverChannel 只读 context，不改任何状态；auto 分组返回所选分组及其下标。
func findFailoverChannel(param *RetryParam) (*model.Channel, string, int, error) {
	filters := GetChannelConstraints(param.Ctx).Filters
	ledger := GetFailoverLedger(param.Ctx)

	if param.TokenGroup != "auto" {
		channel, err := model.GetUntriedSatisfiedChannel(param.TokenGroup, param.ModelName, filters, ledger.ChannelExhausted)
		return channel, param.TokenGroup, 0, err
	}

	autoGroups := GetRequestAutoGroups(param.Ctx, common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup))
	if len(autoGroups) == 0 {
		return nil, param.TokenGroup, 0, errors.New("auto groups is not enabled")
	}
	startIndex := max(slices.Index(autoGroups, common.GetContextKeyString(param.Ctx, constant.ContextKeyAutoGroup)), 0)
	crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)
	for i := startIndex; i < len(autoGroups); i++ {
		if i > startIndex && !crossGroupRetry {
			break
		}
		channel, err := model.GetUntriedSatisfiedChannel(autoGroups[i], param.ModelName, filters, ledger.ChannelExhausted)
		if err != nil {
			return nil, autoGroups[i], i, err
		}
		if channel != nil {
			return channel, autoGroups[i], i, nil
		}
	}
	return nil, param.TokenGroup, 0, nil
}

// PickFailoverChannel 选一个还没试完的渠道；全部试完时返回 exhausted=true 且不算错误。
func PickFailoverChannel(param *RetryParam) (*model.Channel, string, bool, error) {
	channel, group, groupIndex, err := findFailoverChannel(param)
	if err != nil {
		return nil, group, false, err
	}
	if channel == nil {
		return nil, group, true, nil
	}
	if param.TokenGroup == "auto" {
		common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, groupIndex)
		common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, group)
	}
	return channel, group, false, nil
}

func HasFailoverChannel(param *RetryParam) bool {
	channel, _, _, err := findFailoverChannel(param)
	return err == nil && channel != nil
}
