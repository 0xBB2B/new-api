package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetClaudeChannelUsage(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if ch.Type != constant.ChannelTypeClaudeSubscription {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Claude Subscription"})
		return
	}
	if ch.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "multi-key channel is not supported"})
		return
	}

	cred, err := service.ClaudeChannelCredential(ch)
	if err != nil {
		common.SysError("failed to parse claude credential: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析凭证失败，请检查渠道配置"})
		return
	}

	statusCode, body, snapshot, hasVersion, err := service.SyncClaudeChannelUsage(c.Request.Context(), ch, cred)
	if err != nil {
		common.SysError("failed to fetch claude usage: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取用量信息失败，请稍后重试"})
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
		resp["message"] = fmt.Sprintf("upstream status: %d", statusCode)
	}
	c.JSON(http.StatusOK, resp)
}

func ResetClaudeChannelLimit(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgChannelIdFormatError)
		return
	}

	ch, err := model.GetChannelById(channelId, true)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiErrorI18n(c, i18n.MsgChannelNotExists)
		return
	}
	if err != nil {
		common.SysError("failed to get channel: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgClaudeLimitResetFailed)
		return
	}
	if ch.Type != constant.ChannelTypeClaudeSubscription {
		common.ApiErrorI18n(c, i18n.MsgClaudeLimitResetChannelTypeInvalid)
		return
	}
	if ch.ChannelInfo.IsMultiKey {
		common.ApiErrorI18n(c, i18n.MsgClaudeLimitResetMultiKeyUnsupported)
		return
	}

	cred, err := service.ClaudeChannelCredential(ch)
	if err != nil {
		common.SysError("failed to parse claude credential: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgClaudeLimitResetCredentialInvalid)
		return
	}

	var req service.ClaudeLimitResetRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgClaudeLimitResetInvalidParams)
		return
	}

	res, err := service.ResetClaudeChannelLimit(c.Request.Context(), ch, cred, req)
	if err != nil {
		common.SysError("failed to reset claude limit: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgClaudeLimitResetFailed)
		return
	}

	var payload any
	if common.Unmarshal(res.Body, &payload) != nil {
		payload = string(res.Body)
	}
	message := i18n.T(c, res.MessageKey, res.MessageArgs)
	c.JSON(http.StatusOK, gin.H{
		"success":         res.Success,
		"message":         message,
		"upstream_status": res.UpstreamStatus,
		"data":            payload,
	})
}
