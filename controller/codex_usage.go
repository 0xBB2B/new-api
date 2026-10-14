package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetCodexChannelUsage(c *gin.Context) {
	fetchCodexChannelWhamData(
		c,
		service.SyncCodexChannelUsage,
		common.LogText("failed to fetch codex usage"),
		"Failed to fetch usage. Please try again later",
	)
}

func GetCodexChannelRateLimitResetCredits(c *gin.Context) {
	fetchCodexChannelWhamData(
		c,
		func(ctx context.Context, ch *model.Channel, oauthKey *service.CodexOAuthKey) (int, []byte, error) {
			return service.FetchCodexChannelWham(ctx, ch, oauthKey, service.FetchCodexWhamRateLimitResetCredits)
		},
		common.LogText("failed to fetch codex reset credits"),
		"Failed to fetch reset credit details. Please try again later",
	)
}

func ResetCodexChannelUsage(c *gin.Context) {
	fetchCodexChannelWhamData(
		c,
		func(ctx context.Context, ch *model.Channel, oauthKey *service.CodexOAuthKey) (int, []byte, error) {
			return service.FetchCodexChannelWham(ctx, ch, oauthKey, service.ConsumeCodexWhamRateLimitResetCredit)
		},
		common.LogText("failed to reset codex usage"),
		"Failed to reset usage. Please try again later",
	)
}

func fetchCodexChannelWhamData(
	c *gin.Context,
	fetch func(ctx context.Context, ch *model.Channel, oauthKey *service.CodexOAuthKey) (int, []byte, error),
	logPrefix string,
	userMessage string,
) {
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
	if ch.Type != constant.ChannelTypeCodex {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Codex"})
		return
	}
	if ch.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "multi-key channel is not supported"})
		return
	}

	oauthKey, err := service.CodexChannelCredential(ch)
	if err != nil {
		common.SysError(common.LogText("failed to parse oauth key: %s", err.Error()))
		common.ApiErrorT(c, "Failed to parse credentials. Please check the channel configuration")
		return
	}

	statusCode, body, err := fetch(c.Request.Context(), ch, oauthKey)
	if err != nil {
		common.SysError(logPrefix + ": " + err.Error())
		common.ApiErrorT(c, userMessage)
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
	}
	if !ok {
		resp["message"] = fmt.Sprintf("upstream status: %d", statusCode)
	}
	c.JSON(http.StatusOK, resp)
}
