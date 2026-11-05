package middleware

import (
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/tokenkit"
	"github.com/gin-gonic/gin"
)

var smartRouteEndpointNames = map[string]string{
	string(constant.EndpointTypeOpenAIResponse): "Responses",
	string(constant.EndpointTypeOpenAI):         "Chat Completions",
	string(constant.EndpointTypeAnthropic):      "Anthropic",
}

// applySmartRoute 返回 false 表示已经写出错误响应，调用方直接结束请求。
func applySmartRoute(c *gin.Context, modelRequest *ModelRequest) bool {
	route, ok := operation_setting.FindSmartRoute(modelRequest.Model)
	if !ok {
		return true
	}

	path := c.Request.URL.Path
	compact := path == "/v1/responses/compact"
	if compact {
		path = "/v1/responses"
	}
	endpoint := smartroute.RequestEndpointFromPath(path)
	supported := route.Endpoints
	if endpoint == "" || !slices.Contains(supported, endpoint) {
		names := make([]string, 0, len(supported))
		for _, e := range supported {
			names = append(names, smartRouteEndpointNames[e])
		}
		abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorSmartRouteEndpointUnsupported, map[string]any{
			"Model":     route.Name,
			"Endpoints": strings.Join(names, ", "),
		}))
		return false
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
		return false
	}
	raw, err := storage.Bytes()
	if err != nil {
		abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
		return false
	}
	inspection, err := smartroute.InspectRequest(endpoint, raw)
	if err != nil {
		abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
		return false
	}
	if countTopLevelJSONKey(raw, "reasoning") > 1 || countTopLevelJSONKey(raw, "reasoning_effort") > 1 {
		abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": "duplicate reasoning field"}))
		return false
	}

	sessionKey := smartroute.SessionKey(common.GetContextKeyInt(c, constant.ContextKeyTokenId), inspection)
	var session *smartroute.SessionRecord
	if state, found := smartroute.LoadSession(sessionKey); found {
		session = &smartroute.SessionRecord{CurrentRoute: state.CurrentRoute, LastNonAdvisor: state.LastNonAdvisor, LastChannelID: state.LastChannelID}
	}

	isUserTurn := inspection.IsUserTurn && !compact
	var outcome smartroute.ScorerOutcome
	if isUserTurn {
		outcome, err = smartroute.RunScorer(c, route, inspection)
		var billingErr *types.NewAPIError
		if errors.Is(err, smartroute.ErrScorerInsufficientQuota) && errors.As(err, &billingErr) {
			logger.LogError(c, common.LogText("relay error: %s", common.LocalLogPreview(billingErr.Error())))
			billingErr.SetMessage(common.MessageWithRequestId(billingErr.Error(), c.GetString(common.RequestIdKey)))
			c.JSON(billingErr.StatusCode, gin.H{"error": billingErr.ToOpenAIError(common.UpstreamAddresses(c)...)})
			c.Abort()
			return false
		}
		if err != nil && outcome.Failure == "" {
			outcome.Failure = smartroute.FailureScorerError
		}
	}

	inputTokens := 0
	if isUserTurn && session != nil {
		inputTokens = tokenkit.Estimate(route.Name, string(raw))
	}
	decision := smartroute.DecideRoute(smartroute.DecideInput{
		Cfg:           route,
		IsUserTurn:    isUserTurn,
		Labels:        outcome.Labels,
		ScorerFailure: outcome.Failure,
		Session:       session,
		InputTokens:   inputTokens,
		LatestChars:   inspection.Features.LatestRuneCount,
	})

	autoToken := common.GetContextKeyString(c, constant.ContextKeyUsingGroup) == "auto"
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	tierModel := func(tier string) (string, []string, string) {
		if tier == smartroute.RouteAdvisor {
			groups := route.Advisor.Groups
			if autoToken {
				groups = service.FilterUserTokenAutoGroups(userGroup, groups)
			}
			return route.Advisor.Model, groups, ""
		}
		t := route.Tiers[tier]
		return t.Model, t.Groups, t.Effort
	}
	available := func(tier string) bool {
		if autoToken && tier != smartroute.RouteAdvisor {
			return true
		}
		modelName, groups, _ := tierModel(tier)
		if !autoToken {
			groups = []string{common.GetContextKeyString(c, constant.ContextKeyUsingGroup)}
		}
		filters := service.GetChannelConstraints(c).Filters
		for _, group := range groups {
			if ch, err := model.GetRandomSatisfiedChannel(group, modelName, 0, filters); err == nil && ch != nil {
				return true
			}
		}
		return false
	}

	finalTier, reason, params := decision.Selected, decision.Reason, decision.Params
	if finalTier == smartroute.RouteAdvisor && !available(finalTier) {
		finalTier, reason, params = decision.Fallback.Tier, decision.Fallback.Reason, decision.Fallback.Params
		if finalTier == "" {
			finalTier = route.DefaultTier
		}
	}
	if finalTier != smartroute.RouteAdvisor && !available(finalTier) {
		finalTier = route.DefaultTier
	}
	realModel, groups, effort := tierModel(finalTier)

	scorerBilled := outcome.Settle != nil
	if scorerBilled {
		outcome.Settle(finalTier, realModel)
	}

	if autoToken {
		common.SetContextKey(c, constant.ContextKeyTokenAutoGroups, groups)
	}
	smartroute.SetRouteResult(c, smartroute.RouteResult{
		VirtualModel: route.Name,
		Tier:         finalTier,
		ScoredTier:   decision.ScoredTier,
		Reason:       reason,
		Params:       params,
		Labels:       outcome.Labels,
		ScorerStatus: scorerStatus(isUserTurn, outcome.Failure),
		ScorerBilled: scorerBilled,
		IsUserTurn:   isUserTurn,
		SessionKey:   sessionKey,
		Session:      session,
	})

	if err := rewriteSmartRouteBody(c, storage, raw, endpoint, realModel, effort); err != nil {
		abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
		return false
	}
	modelRequest.Model = realModel
	return true
}

func scorerStatus(scored bool, failure string) string {
	switch {
	case !scored:
		return smartroute.ScorerStatusSkipped
	case failure == smartroute.FailureScorerTimeout:
		return smartroute.ScorerStatusTimeout
	case failure == smartroute.FailureScorerInvalid:
		return smartroute.ScorerStatusInvalid
	case failure != "":
		return smartroute.ScorerStatusError
	}
	return smartroute.ScorerStatusOK
}

func rewriteSmartRouteBody(c *gin.Context, storage common.BodyStorage, raw []byte, endpoint, realModel, effort string) error {
	patched, err := ojson.SetBytes(raw, "model", realModel)
	if err != nil {
		return err
	}
	if effort != "" && endpoint == string(constant.EndpointTypeOpenAIResponse) {
		reasoning := map[string]any{}
		if existing := ojson.GetBytes(raw, "reasoning"); existing.IsObject() {
			if err := common.UnmarshalJsonStr(existing.Raw, &reasoning); err != nil {
				return err
			}
		}
		reasoning["effort"] = effort
		encoded, err := common.Marshal(reasoning)
		if err != nil {
			return err
		}
		if patched, err = ojson.SetRawBytes(patched, "reasoning", encoded); err != nil {
			return err
		}
	} else if effort != "" {
		// reasoning 对象里的 effort 或 max_tokens 会被部分适配器优先使用，留着就能绕过档位的思考力度。
		if patched, err = ojson.SetBytes(patched, "reasoning_effort", effort); err != nil {
			return err
		}
		if patched, err = ojson.DeleteBytes(patched, "reasoning"); err != nil {
			return err
		}
	}
	newStorage, err := common.CreateBodyStorage(patched)
	if err != nil {
		return err
	}
	_ = storage.Close()
	c.Set(common.KeyBodyStorage, newStorage)
	c.Set(common.KeyRequestBody, nil)
	c.Request.Body = io.NopCloser(newStorage)
	c.Request.ContentLength = int64(len(patched))
	return nil
}

func saveSmartRouteSession(c *gin.Context) {
	result, ok := smartroute.GetRouteResult(c)
	if !ok {
		return
	}
	smartroute.SaveSessionAfterSuccess(result.SessionKey, result.IsUserTurn, result.Tier, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
}
