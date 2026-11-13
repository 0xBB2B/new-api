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
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/reasoning"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting/model_setting"
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
	if path == "/pg/chat/completions" {
		path = "/v1/chat/completions"
	}
	endpoint := smartroute.RequestEndpointFromPath(path)
	supported := route.Endpoints
	if len(supported) == 0 {
		supported = model.InferSmartRouteEndpoints(route)
	}
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

	service.GetChannelConstraints(c).AddFilter(dto.ChannelFilter{
		Kind: dto.FilterAcceptsFormat, Format: endpoint, ChatResponsesPolicy: service.ShouldChatCompletionsUseResponsesGlobal,
	})

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
	if countTopLevelJSONKey(raw, "reasoning") > 1 || countTopLevelJSONKey(raw, "reasoning_effort") > 1 ||
		countTopLevelJSONKey(raw, "thinking") > 1 || countTopLevelJSONKey(raw, "output_config") > 1 ||
		countTopLevelJSONKey([]byte(ojson.GetBytes(raw, "output_config").Raw), "effort") > 1 ||
		countTopLevelJSONKey([]byte(ojson.GetBytes(raw, "thinking").Raw), "type") > 1 ||
		countTopLevelJSONKey([]byte(ojson.GetBytes(raw, "thinking").Raw), "budget_tokens") > 1 {
		abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": "duplicate reasoning field"}))
		return false
	}

	var sessionKey string
	if tokenID := common.GetContextKeyInt(c, constant.ContextKeyTokenId); tokenID != 0 {
		sessionKey = smartroute.SessionKey(tokenID, inspection)
	} else {
		sessionKey = smartroute.SessionKeyForUser(common.GetContextKeyInt(c, constant.ContextKeyUserId), inspection)
	}
	var session *smartroute.SessionRecord
	state, found := smartroute.LoadSession(sessionKey)
	if found {
		session = &smartroute.SessionRecord{CurrentRoute: state.CurrentRoute, LastNonAdvisor: state.LastNonAdvisor}
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
	tierModel := func(tier string) (string, []string) {
		if tier == smartroute.RouteAdvisor {
			groups := route.Advisor.Groups
			if autoToken {
				groups = service.FilterUserTokenAutoGroups(userGroup, groups)
			}
			return route.Advisor.Model, groups
		}
		t := route.Tiers[tier]
		return t.Model, t.Groups
	}
	available := func(tier string) bool {
		if autoToken && tier != smartroute.RouteAdvisor {
			return true
		}
		modelName, groups := tierModel(tier)
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
	realModel, groups := tierModel(finalTier)

	client := clientEffort(endpoint, raw)
	var effort smartroute.EffortDecision
	if finalTier == smartroute.RouteAdvisor {
		effort = smartroute.EffortDecision{Value: client, Source: smartroute.EffortSourceClient}
		if client == "" {
			effort.Source = smartroute.EffortSourceUnset
		}
	} else {
		sessionEffort, sessionSource := "", ""
		if state.CurrentRoute == finalTier {
			sessionEffort, sessionSource = state.Effort, state.EffortSource
		}
		effort = smartroute.ResolveTierEffort(smartroute.EffortInput{
			Tier:          route.Tiers[finalTier],
			IsUserTurn:    isUserTurn,
			Labels:        outcome.Labels,
			ScorerFailed:  outcome.Failure != "",
			ClientEffort:  client,
			SessionEffort: sessionEffort,
			SessionSource: sessionSource,
			Capability:    smartroute.EffectiveThinkingCapability(realModel),
		})
	}

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
		Effort:       effort,
	})

	if err := rewriteSmartRouteBody(c, storage, raw, endpoint, realModel, effort, client); err != nil {
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

func clientEffort(endpoint string, raw []byte) string {
	switch endpoint {
	case string(constant.EndpointTypeOpenAIResponse):
		return ojson.GetBytes(raw, "reasoning.effort").String()
	case string(constant.EndpointTypeAnthropic):
		if ojson.GetBytes(raw, "thinking.type").String() == "disabled" {
			return "none"
		}
		return ojson.GetBytes(raw, "output_config.effort").String()
	}
	return ojson.GetBytes(raw, "reasoning_effort").String()
}

func rewriteSmartRouteBody(c *gin.Context, storage common.BodyStorage, raw []byte, endpoint, realModel string, effort smartroute.EffortDecision, client string) error {
	patched, err := ojson.SetBytes(raw, "model", realModel)
	if err != nil {
		return err
	}
	if effort.Drop || (effort.Value != "" && !(effort.Source == smartroute.EffortSourceClient && effort.Value == client)) {
		if patched, err = rewriteSmartRouteEffort(patched, endpoint, realModel, effort); err != nil {
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

func rewriteSmartRouteEffort(body []byte, endpoint, realModel string, effort smartroute.EffortDecision) ([]byte, error) {
	var err error
	switch endpoint {
	case string(constant.EndpointTypeOpenAIResponse):
		reasoning := map[string]any{}
		if existing := ojson.GetBytes(body, "reasoning"); existing.IsObject() {
			if err := common.UnmarshalJsonStr(existing.Raw, &reasoning); err != nil {
				return nil, err
			}
		} else if effort.Drop {
			return body, nil
		}
		if effort.Drop {
			delete(reasoning, "effort")
		} else {
			reasoning["effort"] = effort.Value
		}
		encoded, err := common.Marshal(reasoning)
		if err != nil {
			return nil, err
		}
		return ojson.SetRawBytes(body, "reasoning", encoded)
	case string(constant.EndpointTypeAnthropic):
		if !effort.Drop && reasoning.IsKnownClaudeModel(realModel) {
			return renderClaudeEffort(body, realModel, effort.Value)
		}
		switch {
		case effort.Drop:
			if body, err = ojson.DeleteBytes(body, "thinking"); err != nil {
				return nil, err
			}
			return ojson.DeleteBytes(body, "output_config.effort")
		case effort.Value == "none":
			if body, err = ojson.SetRawBytes(body, "thinking", []byte(`{"type":"disabled"}`)); err != nil {
				return nil, err
			}
			return ojson.DeleteBytes(body, "output_config.effort")
		}
		if body, err = ojson.SetBytes(body, "output_config.effort", effort.Value); err != nil {
			return nil, err
		}
		if ojson.GetBytes(body, "thinking.type").String() == "disabled" {
			return ojson.DeleteBytes(body, "thinking")
		}
		if ojson.GetBytes(body, "thinking.type").String() != "enabled" {
			return body, nil
		}
		// 名字认不出的模型可能只支持手动思考，别名映射后收到 adaptive 会被拒，所以保持 enabled 并按档位力度重算 budget_tokens。
		if body, err = renderClaudeEffort(body, realModel, effort.Value); err != nil {
			return nil, err
		}
		return ojson.SetBytes(body, "output_config.effort", effort.Value)

	}
	// reasoning 对象里的 effort 或 max_tokens 会被部分适配器优先使用，留着就能绕过档位的思考力度。
	if !effort.Drop {
		if body, err = ojson.SetBytes(body, "reasoning_effort", effort.Value); err != nil {
			return nil, err
		}
	} else if body, err = ojson.DeleteBytes(body, "reasoning_effort"); err != nil {
		return nil, err
	}
	return ojson.DeleteBytes(body, "reasoning")
}

func renderClaudeEffort(body []byte, realModel, value string) ([]byte, error) {
	intent := reasoning.Intent{Mode: reasoning.ModeEnabled, Effort: reasoning.Effort(value)}
	if value == "none" {
		intent.Mode = reasoning.ModeDisabled
	}
	thinking := ojson.GetBytes(body, "thinking")
	switch thinking.Get("display").String() {
	case "summarized":
		include := true
		intent.IncludeThoughts = &include
	case "omitted":
		include := false
		intent.IncludeThoughts = &include
	}

	settings := model_setting.GetClaudeSettings()
	var maxTokens *uint
	if n := ojson.GetBytes(body, "max_tokens"); n.Exists() && n.Int() > 0 {
		v := uint(n.Int())
		maxTokens = &v
	} else if n := settings.GetDefaultMaxTokens(realModel); n > 0 {
		v := uint(n)
		maxTokens = &v
	}
	rendered, err := reasoning.RenderClaude(realModel, intent, maxTokens, settings.ThinkingAdapterBudgetTokensPercentage)
	if err != nil {
		return nil, err
	}
	if rendered.MaxTokens != nil {
		maxTokens = rendered.MaxTokens
	}

	if rendered.Thinking == nil {
		body, err = ojson.DeleteBytes(body, "thinking")
	} else {
		if rendered.Thinking.Type == "adaptive" {
			if binding := thinking.Get("block_binding"); binding.Exists() {
				rendered.Thinking.BlockBinding = []byte(binding.Raw)
			}
		}
		var encoded []byte
		if encoded, err = common.Marshal(rendered.Thinking); err == nil {
			body, err = ojson.SetRawBytes(body, "thinking", encoded)
		}
	}
	if err != nil {
		return nil, err
	}
	if rendered.OutputEffort == "" {
		body, err = ojson.DeleteBytes(body, "output_config.effort")
	} else {
		body, err = ojson.SetBytes(body, "output_config.effort", string(rendered.OutputEffort))
	}
	if err != nil {
		return nil, err
	}

	var removed []string
	switch {
	case rendered.ClearSampling:
		removed = []string{"temperature", "top_p", "top_k"}
	case rendered.ConstrainThinkingSampling:
		removed = []string{"temperature", "top_k"}
		if topP := ojson.GetBytes(body, "top_p"); topP.Exists() && (topP.Float() < 0.95 || topP.Float() > 1) {
			removed = append(removed, "top_p")
		}
	}
	for _, key := range removed {
		if body, err = ojson.DeleteBytes(body, key); err != nil {
			return nil, err
		}
	}
	if maxTokens == nil {
		return body, nil
	}
	return ojson.SetBytes(body, "max_tokens", *maxTokens)
}

func saveSmartRouteSession(c *gin.Context) {
	result, ok := smartroute.GetRouteResult(c)
	if !ok {
		return
	}
	smartroute.SaveSessionAfterSuccess(result.SessionKey, result.IsUserTurn, result.Tier, result.Effort)
}
