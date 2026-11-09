package smartroute

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type FieldError struct {
	Field   string
	Message *common.Message
}

var tierOrder = []string{
	operation_setting.SmartTierSimple,
	operation_setting.SmartTierNormal,
	operation_setting.SmartTierHard,
}

var routeEndpointTypes = []string{
	string(constant.EndpointTypeOpenAIResponse),
	string(constant.EndpointTypeOpenAI),
	string(constant.EndpointTypeAnthropic),
}

var formatDisplayNames = map[string]string{
	string(constant.EndpointTypeOpenAIResponse): "Responses",
	string(constant.EndpointTypeOpenAI):         "Chat Completions",
	string(constant.EndpointTypeAnthropic):      "Anthropic",
}

func ValidateRoute(route operation_setting.SmartRoute) []FieldError {
	errs := []FieldError{}
	add := func(field, key string, params ...map[string]any) {
		errs = append(errs, FieldError{Field: field, Message: common.NewMessage(key, params...)})
	}

	switch {
	case route.Name == "":
		add("name", "Enter a name")
	case slices.Contains(model.GetEnabledModels(), route.Name) || billing_setting.HasPriceOrRatioEntry(route.Name):
		add("name", "{{name}} is a real model name. A virtual model cannot use the name of a real model", map[string]any{"name": route.Name})
	}

	endpoints := route.Endpoints
	if len(endpoints) == 0 {
		endpoints = model.InferSmartRouteEndpoints(route)
	}

	checkSlot := func(field, groupsField string, modelName string, groups []string, checkFormat bool) {
		if modelName == "" {
			add(field, "Select a model")
		}
		if len(groups) == 0 {
			add(groupsField, "Select at least one group")
		}
		if modelName == "" || len(groups) == 0 {
			return
		}
		var filters []dto.ChannelFilter
		if field == "scorer.model" {
			filters = []dto.ChannelFilter{{Kind: dto.FilterScorerCapable}}
		}
		for _, group := range groups {
			if channel, err := model.GetRandomSatisfiedChannel(group, modelName, 0, filters); err != nil || channel == nil {
				add(field, "{{model}} has no available channel in group {{group}}", map[string]any{"model": modelName, "group": group})
				return
			}
		}
		if !billing_setting.HasModelBillingConfig(modelName) {
			add(field, "{{model}} has no price yet. Set it in Model Pricing first", map[string]any{"model": modelName})
		}
		if !checkFormat {
			return
		}
		groupChannels := make(map[string][]model.GroupChannel, len(groups))
		for _, group := range groups {
			groupChannels[group] = model.GroupModelChannels(group, modelName)
		}
		passThrough := map[int]bool{}
		channelPassThrough := func(id int) bool {
			if known, ok := passThrough[id]; ok {
				return known
			}
			channel, err := model.CacheGetChannel(id)
			passThrough[id] = err != nil || channel.GetSetting().PassThroughBodyEnabled
			return passThrough[id]
		}
		for _, format := range endpoints {
			for _, group := range groups {
				rejectedTypes := []string{}
				accepted := false
				for _, ch := range groupChannels[group] {
					converts := model.RequestConvertsToResponses(format, ch.Id, ch.Type, modelName, service.ShouldChatCompletionsUseResponsesGlobal, func() bool { return channelPassThrough(ch.Id) })
					if model.ChannelAcceptsFormat(ch.Type, format, converts) {
						accepted = true
						break
					}
					if name := constant.GetChannelTypeName(ch.Type); !slices.Contains(rejectedTypes, name) {
						rejectedTypes = append(rejectedTypes, name)
					}
				}
				if accepted {
					continue
				}
				if len(rejectedTypes) == 0 {
					add(groupsField, "{{model}} has no channel in group {{group}} that can accept the {{format}} format", map[string]any{
						"model": modelName, "group": group, "format": formatDisplayNames[format],
					})
					continue
				}
				add(groupsField, "{{model}} has only {{channel_types}} channels in group {{group}}, which cannot accept the {{format}} format", map[string]any{
					"model": modelName, "channel_types": strings.Join(rejectedTypes, ", "), "group": group, "format": formatDisplayNames[format],
				})
			}
		}
	}
	for _, name := range tierOrder {
		tier := route.Tiers[name]
		checkSlot("tiers."+name+".model", "tiers."+name+".groups", tier.Model, tier.Groups, true)
	}
	checkSlot("scorer.model", "scorer.groups", route.Scorer.Model, route.Scorer.Groups, false)
	checkSlot("advisor.model", "advisor.groups", route.Advisor.Model, route.Advisor.Groups, true)

	outOfRange := func(field string, min, max any) {
		add(field, "Enter a value between {{min}} and {{max}}", map[string]any{"min": min, "max": max})
	}
	if route.Scorer.TimeoutMs < 100 || route.Scorer.TimeoutMs > 30000 {
		outOfRange("scorer.timeout_ms", 100, 30000)
	}
	if route.Scorer.Effort != "" && operation_setting.SmartEffortRank(route.Scorer.Effort) < 0 {
		add("scorer.effort", "Select a valid reasoning effort")
	}
	if route.Advisor.AfterDissatisfied < 1 || route.Advisor.AfterDissatisfied > 10 {
		outOfRange("advisor.after_dissatisfied", 1, 10)
	}
	if route.DowngradeMaxHistoryTokens < 0 {
		add("downgrade_max_history_tokens", "Enter an integer of 0 or more")
	}
	if !(route.ConfidenceThreshold >= 0 && route.ConfidenceThreshold <= 1) {
		outOfRange("confidence_threshold", 0, 1)
	}
	if route.SimpleMaxMessageChars < 1 || route.SimpleMaxMessageChars > 100000 {
		outOfRange("simple_max_message_chars", 1, 100000)
	}

	if operation_setting.SmartTierRank(route.DefaultTier) < 0 {
		add("default_tier", "Select a default tier", nil)
	}
	switch {
	case len(endpoints) == 0:
		add("endpoints", "Select at least one endpoint")
	case slices.ContainsFunc(endpoints, func(endpoint string) bool { return !slices.Contains(routeEndpointTypes, endpoint) }):
		add("endpoints", "Select a valid endpoint")
	}
	for _, name := range tierOrder {
		validateTierEffort(add, "tiers."+name, route.Tiers[name])
	}
	return errs
}

func validateTierEffort(add func(field, key string, params ...map[string]any), prefix string, tier operation_setting.SmartRouteTier) {
	const invalid = "Select a valid reasoning effort"
	switch tier.EffortMode {
	case "":
		if tier.Effort != "" {
			add(prefix+".effort", invalid)
		}
		if tier.EffortMin != "" {
			add(prefix+".effort_min", invalid)
		}
		if tier.EffortMax != "" {
			add(prefix+".effort_max", invalid)
		}
	case "fixed":
		if operation_setting.SmartEffortRank(tier.Effort) < 0 {
			add(prefix+".effort", invalid)
		}
		if tier.EffortMin != "" {
			add(prefix+".effort_min", invalid)
		}
		if tier.EffortMax != "" {
			add(prefix+".effort_max", invalid)
		}
	case "follow":
		if tier.Effort != "" {
			add(prefix+".effort", invalid)
		}
		minKnown, maxKnown := operation_setting.SmartEffortRank(tier.EffortMin) >= 0, operation_setting.SmartEffortRank(tier.EffortMax) >= 0
		if !minKnown {
			add(prefix+".effort_min", invalid)
		}
		if !maxKnown {
			add(prefix+".effort_max", invalid)
		}
		if minKnown && maxKnown && operation_setting.SmartEffortRank(tier.EffortMin) > operation_setting.SmartEffortRank(tier.EffortMax) {
			add(prefix+".effort_max", "The minimum effort cannot be higher than the maximum")
		}
	default:
		add(prefix+".effort", invalid)
	}
}

func ValidateRoutes(routes []operation_setting.SmartRoute) error {
	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		fail := func(detail *common.Message) error {
			return common.NewMessage("Virtual model {{name}}: {{detail}}", map[string]any{"name": route.Name, "detail": detail})
		}
		if errs := ValidateRoute(route); len(errs) > 0 {
			return fail(errs[0].Message)
		}
		if _, dup := seen[route.Name]; dup {
			return fail(common.NewMessage("A virtual model named {{name}} already exists", map[string]any{"name": route.Name}))
		}
		seen[route.Name] = struct{}{}
	}
	return nil
}
