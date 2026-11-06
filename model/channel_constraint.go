package model

import (
	"slices"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

var filterEvalOrder = []dto.ChannelFilterKind{
	dto.FilterRequestPath,
	dto.FilterTaskPluginEndpoint,
	dto.FilterTaskPluginIdentity,
	dto.FilterResponsesWebSocket,
	dto.FilterScorerCapable,
	dto.FilterAcceptsFormat,
}

// ChannelSatisfiesFilters reports whether ch passes every filter.
// On false, it returns the first violated filter in filterEvalOrder for error
// attribution.
func ChannelSatisfiesFilters(ch *Channel, modelName string, filters []dto.ChannelFilter) (bool, dto.ChannelFilterKind) {
	if ch == nil {
		return false, ""
	}
	for _, kind := range filterEvalOrder {
		for _, filter := range filters {
			if filter.Kind != kind {
				continue
			}
			if !channelMatchesFilter(ch, modelName, filter) {
				return false, kind
			}
		}
	}
	return true, ""
}

// filterCandidateIDs applies filters to a cached candidate id list.
// Caller must hold channelSyncLock (read lock). The input slice is never mutated.
// A missing id in channelsIDM is kept for request_path (downstream consistency
// error) and dropped for the other filters.
func filterCandidateIDs(ids []int, modelName string, filters []dto.ChannelFilter) (kept []int, emptiedBy dto.ChannelFilterKind) {
	if len(ids) == 0 {
		return ids, ""
	}
	kept = ids
	for _, kind := range filterEvalOrder {
		kindFilters := filtersByKind(filters, kind)
		if len(kindFilters) == 0 {
			continue
		}
		next := make([]int, 0, len(kept))
		for _, id := range kept {
			channel, exists := channelsIDM[id]
			if candidatePassesKindFilters(channel, exists, modelName, kind, kindFilters) {
				next = append(next, id)
			}
		}
		if len(kept) > 0 && len(next) == 0 {
			return next, kind
		}
		kept = next
	}
	return kept, ""
}

func filtersByKind(filters []dto.ChannelFilter, kind dto.ChannelFilterKind) []dto.ChannelFilter {
	var matched []dto.ChannelFilter
	for _, filter := range filters {
		if filter.Kind == kind {
			matched = append(matched, filter)
		}
	}
	return matched
}

func candidatePassesKindFilters(ch *Channel, exists bool, modelName string, kind dto.ChannelFilterKind, filters []dto.ChannelFilter) bool {
	if kind == dto.FilterRequestPath && !exists {
		return true
	}
	if !exists || ch == nil {
		return false
	}
	for _, filter := range filters {
		if !channelMatchesFilter(ch, modelName, filter) {
			return false
		}
	}
	return true
}

func channelMatchesFilter(ch *Channel, modelName string, filter dto.ChannelFilter) bool {
	switch filter.Kind {
	case dto.FilterRequestPath:
		if filter.RequestPath == "" {
			return true
		}
		if !constant.IsAdvancedCustomChannel(ch.Type) {
			return true
		}
		config := ch.GetOtherSettings().AdvancedCustom
		return config != nil && config.SupportsPathForModel(filter.RequestPath, modelName)
	case dto.FilterTaskPluginIdentity:
		if filter.TaskPluginKey == "" {
			return ch.Type != constant.ChannelTypeTaskPlugin
		}
		if ch.Type == constant.ChannelTypeTaskPlugin || ch.Type == constant.ChannelTypeNewAPI {
			// A New API channel serves every plugin it is extended with; the
			// pinned plugin or any shared-model candidate may execute there.
			setting := ch.GetSetting()
			return setting.BindsTaskPlugin(filter.TaskPluginKey) || slices.ContainsFunc(filter.TaskPluginKeys, setting.BindsTaskPlugin)
		}
		return slices.Contains(filter.TaskPluginChannelTypes, ch.Type)
	case dto.FilterTaskPluginEndpoint:
		pluginChannel := slices.Contains(filter.TaskPluginChannelTypes, ch.Type)
		if ch.Type == constant.ChannelTypeTaskPlugin || ch.Type == constant.ChannelTypeNewAPI {
			pluginChannel = slices.ContainsFunc(filter.TaskPluginKeys, ch.GetSetting().BindsTaskPlugin)
		}
		if filter.TaskPluginRelayOnly {
			return ch.Type != constant.ChannelTypeTaskPlugin && !pluginChannel
		}
		// Ordinary channels and matching plugin channels compete using the same
		// group, priority, weight, affinity, and pin rules. Unrelated type-61
		// plugins cannot execute this endpoint.
		return ch.Type != constant.ChannelTypeTaskPlugin || pluginChannel
	case dto.FilterResponsesWebSocket:
		if !ch.GetSetting().ResponsesWebSocketEnabled {
			return false
		}
		switch ch.Type {
		case constant.ChannelTypeOpenAI, constant.ChannelTypeAzure, constant.ChannelTypeCodex, constant.ChannelTypeSub2API, constant.ChannelTypeNewAPI:
			return true
		case constant.ChannelTypeAdvancedCustom:
			// The session forwards native Responses events without protocol
			// conversion, so only a converter-free /v1/responses route qualifies.
			route, ok := ch.GetOtherSettings().AdvancedCustom.MatchPathForModel("/v1/responses", modelName)
			return ok && route.IsNative()
		default:
			return false
		}
	case dto.FilterScorerCapable:
		return ch.Type != constant.ChannelTypeReplicate
	case dto.FilterAcceptsFormat:
		converts := RequestConvertsToResponses(filter.Format, ch.Id, ch.Type, modelName, filter.ChatResponsesPolicy,
			func() bool { return ch.GetSetting().PassThroughBodyEnabled })
		return ChannelAcceptsFormat(ch.Type, filter.Format, converts)
	default:
		return true
	}
}

var formatRejectingChannelTypes = map[string][]int{
	string(constant.EndpointTypeAnthropic): {
		constant.ChannelTypeCodex, constant.ChannelTypeCoze, constant.ChannelTypeJimeng,
		constant.ChannelTypeReplicate, constant.ChannelTypeSubmodel, constant.ChannelTypeXai,
		constant.ChannelTypePaLM, constant.ChannelTypeBaidu, constant.ChannelTypeZhipu,
		constant.ChannelTypeXunfei, constant.ChannelTypeCohere, constant.ChannelTypeDify,
		constant.ChannelTypeJina, constant.ChannelCloudflare, constant.ChannelTypeMistral,
		constant.ChannelTypeMokaAI,
	},
	string(constant.EndpointTypeOpenAI): {constant.ChannelTypeCodex, constant.ChannelTypeReplicate, constant.ChannelTypeMokaAI},
	string(constant.EndpointTypeOpenAIResponse): {
		constant.ChannelTypeAws, constant.ChannelTypeBaidu, constant.ChannelTypeBaiduV2,
		constant.ChannelTypeClaudeSubscription, constant.ChannelTypeCohere, constant.ChannelTypeCoze,
		constant.ChannelTypeDify, constant.ChannelTypeJimeng, constant.ChannelTypeJina,
		constant.ChannelTypeMiniMax, constant.ChannelTypeMistral, constant.ChannelTypeMokaAI,
		constant.ChannelTypePaLM, constant.ChannelTypeReplicate, constant.ChannelTypeSiliconFlow,
		constant.ChannelTypeSubmodel, constant.ChannelTypeVertexAi, constant.ChannelTypeXunfei,
		constant.ChannelTypeZhipu,
	},
}

// ChannelTypeRejectsFormat reports whether the channel type's adaptor
// explicitly returns an error for the request format.
func ChannelTypeRejectsFormat(channelType int, format string) bool {
	return slices.Contains(formatRejectingChannelTypes[format], channelType)
}

// RequestConvertsToResponses reports whether a Chat Completions or Anthropic
// request sent to this channel is first converted to a Responses request.
// channelPassThrough is only evaluated once the policy hits, so callers that
// must load the channel setting pay for it only then.
func RequestConvertsToResponses(format string, channelID, channelType int, modelName string, policy dto.ChatResponsesPolicyFunc, channelPassThrough func() bool) bool {
	return format != string(constant.EndpointTypeOpenAIResponse) &&
		!model_setting.GetGlobalSettings().PassThroughRequestEnabled &&
		policy != nil && policy(channelID, channelType, modelName) &&
		!channelPassThrough()
}

// ChannelAcceptsFormat judges the format the adaptor actually receives: a
// request converted to Responses is accepted or rejected as Responses.
func ChannelAcceptsFormat(channelType int, format string, convertsToResponses bool) bool {
	if convertsToResponses {
		format = string(constant.EndpointTypeOpenAIResponse)
	}
	return !ChannelTypeRejectsFormat(channelType, format)
}
