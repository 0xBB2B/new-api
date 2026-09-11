package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const subscriptionUsageOtherInfoKey = "subscription_usage"

type SubscriptionUsageWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	ResetAt            int64   `json:"reset_at,omitempty"`
	LimitWindowSeconds int64   `json:"limit_window_seconds,omitempty"`
}

type SubscriptionUsageSnapshot struct {
	PlanType        string                   `json:"plan_type,omitempty"`
	LimitReached    bool                     `json:"limit_reached"`
	PrimaryWindow   *SubscriptionUsageWindow `json:"primary_window,omitempty"`
	SecondaryWindow *SubscriptionUsageWindow `json:"secondary_window,omitempty"`
	UpdatedAt       int64                    `json:"updated_at"`
}

func clampUsagePercent(v float64) float64 {
	return min(max(v, 0), 100)
}

func saveSubscriptionUsageSnapshot(channelID int, snapshot *SubscriptionUsageSnapshot) {
	if err := model.SetChannelOtherInfoEntry(channelID, subscriptionUsageOtherInfoKey, snapshot); err != nil {
		common.SysError(fmt.Sprintf("subscription usage: save snapshot failed: channel_id=%d err=%v", channelID, err))
	}
}
