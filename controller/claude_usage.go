package controller

import (
	"errors"
	"maps"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetClaudeChannelUsage(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}

	ch, err := model.GetChannelById(channelId, true)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiErrorT(c, "Channel does not exist")
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch.Type != constant.ChannelTypeClaudeSubscription {
		common.ApiErrorT(c, "Channel type is not Claude Subscription")
		return
	}
	if ch.ChannelInfo.IsMultiKey {
		common.ApiErrorT(c, "Multi-key channels are not supported")
		return
	}

	cred, err := service.ClaudeChannelCredential(ch)
	if err != nil {
		common.SysError("failed to parse claude credential: " + err.Error())
		common.ApiErrorT(c, "Failed to parse credentials. Please check the channel configuration")
		return
	}

	statusCode, body, snapshot, hasVersion, err := service.SyncClaudeChannelUsage(c.Request.Context(), ch, cred)
	if err != nil {
		common.SysError("failed to fetch claude usage: " + err.Error())
		common.ApiErrorT(c, "Failed to fetch usage. Please try again later")
		return
	}

	var payload any
	if common.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}

	ok := statusCode >= 200 && statusCode < 300
	resp := gin.H{
		"success":         ok,
		"message":         "",
		"upstream_status": statusCode,
		"data":            payload,
		"usage":           snapshot,
	}
	if !hasVersion {
		resp["limit_reset_unavailable"] = "client_version"
	}
	if !ok {
		msg := common.NewMessage("Upstream returned HTTP {{status}}", map[string]any{"status": statusCode})
		resp["message"] = msg.Error()
		maps.Copy(resp, msg.Fields())
	}
	c.JSON(http.StatusOK, resp)
}

func ResetClaudeChannelLimit(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}

	ch, err := model.GetChannelById(channelId, true)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiErrorT(c, "Channel does not exist")
		return
	}
	if err != nil {
		common.SysError("failed to get channel: " + err.Error())
		common.ApiErrorT(c, "Reset failed, please try again later")
		return
	}
	if ch.Type != constant.ChannelTypeClaudeSubscription {
		common.ApiErrorT(c, "Channel type is not Claude Subscription")
		return
	}
	if ch.ChannelInfo.IsMultiKey {
		common.ApiErrorT(c, "Multi-key channels are not supported")
		return
	}

	cred, err := service.ClaudeChannelCredential(ch)
	if err != nil {
		common.SysError("failed to parse claude credential: " + err.Error())
		common.ApiErrorT(c, "Failed to parse credentials. Please check the channel configuration")
		return
	}

	var req service.ClaudeLimitResetRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorT(c, "Invalid reset parameters")
		return
	}

	res, err := service.ResetClaudeChannelLimit(c.Request.Context(), ch, cred, req)
	if err != nil {
		common.SysError("failed to reset claude limit: " + err.Error())
		common.ApiErrorT(c, "Reset failed, please try again later")
		return
	}

	var payload any
	if common.Unmarshal(res.Body, &payload) != nil {
		payload = string(res.Body)
	}
	resp := gin.H{
		"success":         res.Success,
		"message":         res.Message.Error(),
		"upstream_status": res.UpstreamStatus,
		"data":            payload,
	}
	maps.Copy(resp, res.Message.Fields())
	c.JSON(http.StatusOK, resp)
}
