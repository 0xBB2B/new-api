package relay

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	newapidto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

const (
	scorerRequestPath          = "/v1/chat/completions"
	scorerResponsesRequestPath = "/v1/responses"
)

func init() {
	smartroute.ScorerRunner = runSmartRouteScorer
}

func runSmartRouteScorer(parent *gin.Context, route operation_setting.SmartRoute, in smartroute.Inspection) (smartroute.ScorerOutcome, error) {
	scorerModel := route.Scorer.Model
	scorerFailed := func(format string, args ...any) (smartroute.ScorerOutcome, error) {
		logger.LogWarn(parent, common.LogText("smart routing scorer for %s failed: "+format, append([]any{route.Name}, args...)...))
		return smartroute.ScorerOutcome{Failure: smartroute.FailureScorerError}, nil
	}
	input := smartroute.BuildScorerInput(in, route.Advisor.AfterDissatisfied)
	system, user := smartroute.BuildScorerMessages(input, in.Features)
	var channel *model.Channel
	var group string
	// 打分请求的格式由打分渠道决定，和主请求的格式无关。
	filters := slices.DeleteFunc(slices.Clone(service.GetChannelConstraints(parent).Filters), func(f newapidto.ChannelFilter) bool {
		return f.Kind == newapidto.FilterAcceptsFormat
	})
	filters = append(filters, newapidto.ChannelFilter{Kind: newapidto.FilterScorerCapable})
	for _, g := range smartroute.ScorerGroups(parent, route) {
		if channel, _ = model.GetRandomSatisfiedChannel(g, scorerModel, 0, filters); channel != nil {
			group = g
			break
		}
	}
	if channel == nil {
		return scorerFailed("no available channel for %s in groups %v", scorerModel, smartroute.ScorerGroups(parent, route))
	}
	effort, _ := smartroute.ClampEffortToCapability(route.Scorer.Effort, smartroute.EffectiveThinkingCapability(scorerModel))
	var request dto.Request
	var requestPath string
	var relayFormat types.RelayFormat
	if channel.Type == constant.ChannelTypeCodex {
		// Codex 上游只有 Responses 接口
		instructions, err := common.Marshal(system)
		if err != nil {
			return scorerFailed("building the scorer request: %s", err.Error())
		}
		userInput, err := common.Marshal([]map[string]string{{"role": "user", "content": user}})
		if err != nil {
			return scorerFailed("building the scorer request: %s", err.Error())
		}
		responsesRequest := &dto.OpenAIResponsesRequest{
			Model:        scorerModel,
			Instructions: instructions,
			Input:        userInput,
			Stream:       lo.ToPtr(false),
		}
		if effort != "" {
			responsesRequest.Reasoning = &dto.Reasoning{Effort: effort}
		}
		request = responsesRequest
		requestPath, relayFormat = scorerResponsesRequestPath, types.RelayFormatOpenAIResponses
	} else {
		request = &dto.GeneralOpenAIRequest{
			Model:           scorerModel,
			Messages:        []dto.Message{{Role: "system", Content: system}, {Role: "user", Content: user}},
			ReasoningEffort: effort,
			Temperature:     lo.ToPtr(0.0),
			Stream:          lo.ToPtr(false),
		}
		requestPath, relayFormat = scorerRequestPath, types.RelayFormatOpenAI
	}
	body, err := common.Marshal(request)
	if err != nil {
		return scorerFailed("building the scorer request: %s", err.Error())
	}
	bodyStorage, err := common.CreateBodyStorage(body)
	if err != nil {
		return scorerFailed("buffering the scorer request: %s", err.Error())
	}
	defer bodyStorage.Close()

	timeout := time.Duration(route.Scorer.TimeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(parent.Request.Context(), timeout)
	defer cancel()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Keys = parent.Copy().Keys
	c.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, requestPath, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyBodyStorage, bodyStorage)
	c.Set(common.KeyRequestBody, nil)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, scorerModel)
	common.SetContextKey(c, constant.ContextKeyIsStream, false)

	common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
	common.SetContextKey(c, constant.ContextKeyAutoGroup, group)
	if apiErr := middleware.SetupContextForSelectedChannel(c, channel, scorerModel); apiErr != nil {
		return scorerFailed("setting up the scorer channel: %s", apiErr.Error())
	}

	info, err := relaycommon.GenRelayInfo(c, relayFormat, request, nil)
	if err != nil {
		return scorerFailed("building relay info: %s", err.Error())
	}
	info.IsPlayground = strings.HasPrefix(parent.Request.URL.Path, "/pg")
	// 预扣费按请求 ID 幂等，与主请求共用 ID 会让打分的预扣被当成重复
	info.RequestId += "-scorer"
	refund := func() {
		if session, ok := info.Billing.(*service.BillingSession); ok {
			session.RefundSync(parent)
		}
	}
	if apiErr := PrepareRequestBilling(c, info); apiErr != nil {
		if apiErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
			return smartroute.ScorerOutcome{}, fmt.Errorf("%w: %w", smartroute.ErrScorerInsufficientQuota, apiErr)
		}
		refund()
		return scorerFailed("reserving scorer quota: %s", apiErr.Error())
	}

	// 上游请求不带 context，超时只能靠这里放弃等待；放弃后协程独占 c 和 info，用完即弃
	type upstreamResult struct {
		usage   *dto.Usage
		content string
		apiErr  *types.NewAPIError
	}
	done := make(chan upstreamResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- upstreamResult{apiErr: types.NewError(fmt.Errorf("scorer panic: %v", r), types.ErrorCodeBadResponse)}
			}
		}()
		usage, content, apiErr := callScorerUpstream(c, info, request, recorder)
		done <- upstreamResult{usage, content, apiErr}
	}()
	var result upstreamResult
	select {
	case result = <-done:
	case <-ctx.Done():
		refund()
		return smartroute.ScorerOutcome{Failure: smartroute.FailureScorerTimeout}, nil
	}
	if result.apiErr != nil {
		refund()
		return scorerFailed("calling upstream: %s", result.apiErr.Error())
	}
	usage, content := result.usage, result.content

	// 结算发生在本函数返回之后，此时 ctx 已被取消，日志写入不能再受它影响
	c.Request = c.Request.WithContext(context.WithoutCancel(ctx))
	var settleOnce sync.Once
	outcome := smartroute.ScorerOutcome{
		Settle: func(mainTier, mainModel string) {
			settleOnce.Do(func() { settleSmartRouteScorer(c, info, usage, route.Name, mainTier, mainModel) })
		},
	}
	labels, err := smartroute.ParseScorerOutput(content, len(input.History)+1)
	if err != nil {
		raw := []rune(content)
		if len(raw) > 500 {
			raw = raw[:500]
		}
		rawText := string(raw)
		if rawText == "" {
			rawText = "(empty)"
		}
		logger.LogWarn(parent, common.LogText("smart routing scorer for %s returned unparsable output from %s: %s; raw: %q", route.Name, scorerModel, err.Error(), rawText))
		outcome.Failure = smartroute.FailureScorerInvalid
		return outcome, nil
	}
	outcome.Labels = &labels
	return outcome, nil
}

func callScorerUpstream(c *gin.Context, info *relaycommon.RelayInfo, request dto.Request, recorder *httptest.ResponseRecorder) (*dto.Usage, string, *types.NewAPIError) {
	info.InitChannelMeta(c)
	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return nil, "", types.NewError(err, types.ErrorCodeChannelModelMappedError)
	}
	if err := helper.ApplyReasoningModelSuffix(c, info, request); err != nil {
		return nil, "", types.NewError(err, types.ErrorCodeConvertRequestFailed)
	}
	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, "", types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType)
	}
	adaptor.Init(info)

	var converted any
	var err error
	if responsesRequest, ok := request.(*dto.OpenAIResponsesRequest); ok {
		converted, err = adaptor.ConvertOpenAIResponsesRequest(c, info, *responsesRequest)
	} else {
		converted, err = adaptor.ConvertOpenAIRequest(c, info, request.(*dto.GeneralOpenAIRequest))
	}
	if err != nil {
		return nil, "", types.NewError(err, types.ErrorCodeConvertRequestFailed)
	}
	jsonData, err := common.Marshal(converted)
	if err != nil {
		return nil, "", types.NewError(err, types.ErrorCodeJsonMarshalFailed)
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, "", types.NewError(err, types.ErrorCodeConvertRequestFailed)
	}
	if len(info.ParamOverride) > 0 {
		if jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info); err != nil {
			return nil, "", types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid)
		}
	}

	resp, err := adaptor.DoRequest(c, info, bytes.NewReader(jsonData))
	if err != nil {
		return nil, "", types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			return nil, "", service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		}
	}
	usage, apiErr := adaptor.DoResponse(c, httpResp, info)
	if apiErr != nil {
		return nil, "", apiErr
	}

	if info.RelayFormat == types.RelayFormatOpenAIResponses {
		var response dto.OpenAIResponsesResponse
		if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			return usage.(*dto.Usage), "", nil
		}
		for _, output := range response.Output {
			if output.Type != "message" {
				continue
			}
			for _, part := range output.Content {
				if part.Type == "output_text" {
					return usage.(*dto.Usage), part.Text, nil
				}
			}
		}
		return usage.(*dto.Usage), "", nil
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := common.Unmarshal(recorder.Body.Bytes(), &completion); err != nil || len(completion.Choices) == 0 {
		return usage.(*dto.Usage), "", nil
	}
	return usage.(*dto.Usage), completion.Choices[0].Message.Content, nil
}

func settleSmartRouteScorer(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage, virtualModel, mainTier, mainModel string) {
	common.SetContextKey(c, constant.ContextKeySmartRouteLog, smartroute.ScorerLogFields(virtualModel, mainTier, mainModel))
	service.PostTextConsumeQuota(c, info, usage, nil)
}
