package controller

import (
	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func GetSubscriptionChannelUsage(c *gin.Context) {
	common.ApiSuccess(c, model.SubscriptionChannelUsageOverview())
}
