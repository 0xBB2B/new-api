package smartroute

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type FieldError struct {
	Field   string
	Message *common.Message
}

type routeModelSlot struct {
	field string
	model string
}

var tierOrder = []string{
	operation_setting.SmartTierSimple,
	operation_setting.SmartTierNormal,
	operation_setting.SmartTierHard,
}

var tierEfforts = []string{"", "low", "medium", "high"}

func modelEndpoints(modelName string) []string {
	model.GetPricing()
	types := model.GetModelSupportEndpointTypes(modelName)
	endpoints := make([]string, 0, len(types))
	for _, t := range types {
		if t == constant.EndpointTypeOpenAI || t == constant.EndpointTypeOpenAIResponse {
			endpoints = append(endpoints, string(t))
		}
	}
	return endpoints
}

func endpointDisplayNames(endpoints []string) string {
	names := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		switch constant.EndpointType(endpoint) {
		case constant.EndpointTypeOpenAIResponse:
			names = append(names, "Responses")
		case constant.EndpointTypeOpenAI:
			names = append(names, "Chat Completions")
		}
	}
	return strings.Join(names, ", ")
}

// 打分模型由 new-api 自己按 Chat Completions 调用，不受虚拟模型接口限制，所以不在这里。
func endpointSlots(route operation_setting.SmartRoute) []routeModelSlot {
	slots := make([]routeModelSlot, 0, len(tierOrder)+1)
	for _, name := range tierOrder {
		slots = append(slots, routeModelSlot{field: "tiers." + name + ".model", model: route.Tiers[name].Model})
	}
	return append(slots, routeModelSlot{field: "advisor.model", model: route.Advisor.Model})
}

// 冲突时返回让交集变空的那个模型，以及它之前各模型的交集，用来告诉管理员"其他模型走什么接口"。
func sharedEndpoints(route operation_setting.SmartRoute) (shared []string, conflict *routeModelSlot, before []string) {
	first := true
	for _, slot := range endpointSlots(route) {
		if slot.model == "" {
			continue
		}
		endpoints := modelEndpoints(slot.model)
		next := endpoints
		if !first {
			next = []string{}
			for _, endpoint := range shared {
				if slices.Contains(endpoints, endpoint) {
					next = append(next, endpoint)
				}
			}
		}
		if len(next) == 0 {
			return []string{}, &slot, shared
		}
		shared, first = next, false
	}
	if shared == nil {
		return []string{}, nil, nil
	}
	return shared, nil, nil
}

func RouteEndpoints(route operation_setting.SmartRoute) []string {
	shared, _, _ := sharedEndpoints(route)
	return shared
}

func ValidateRoute(route operation_setting.SmartRoute) ([]FieldError, []string) {
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

	checkSlot := func(field, groupsField string, modelName string, groups []string) {
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
	}
	for _, name := range tierOrder {
		tier := route.Tiers[name]
		checkSlot("tiers."+name+".model", "tiers."+name+".groups", tier.Model, tier.Groups)
	}
	checkSlot("scorer.model", "scorer.groups", route.Scorer.Model, route.Scorer.Groups)
	checkSlot("advisor.model", "advisor.groups", route.Advisor.Model, route.Advisor.Groups)

	outOfRange := func(field string, min, max any) {
		add(field, "Enter a value between {{min}} and {{max}}", map[string]any{"min": min, "max": max})
	}
	if route.Scorer.TimeoutMs < 100 || route.Scorer.TimeoutMs > 30000 {
		outOfRange("scorer.timeout_ms", 100, 30000)
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
	for _, name := range tierOrder {
		if !slices.Contains(tierEfforts, route.Tiers[name].Effort) {
			add("tiers."+name+".effort", "Select a valid reasoning effort")
		}
	}

	if len(errs) == 0 {
		if _, conflict, before := sharedEndpoints(route); conflict != nil {
			endpoints := modelEndpoints(conflict.model)
			if len(endpoints) == 0 {
				add(conflict.field, "{{model}} supports neither Responses nor Chat Completions", map[string]any{"model": conflict.model})
			} else {
				add(conflict.field, "{{model}} only supports {{endpoint}}. The other models of this virtual model use {{other}}",
					map[string]any{"model": conflict.model, "endpoint": endpointDisplayNames(endpoints), "other": endpointDisplayNames(before)})
			}
		}
	}
	if len(errs) > 0 {
		return errs, []string{}
	}
	return errs, RouteEndpoints(route)
}

func ValidateRoutes(routes []operation_setting.SmartRoute) error {
	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		fail := func(detail *common.Message) error {
			return common.NewMessage("Virtual model {{name}}: {{detail}}", map[string]any{"name": route.Name, "detail": detail})
		}
		if errs, _ := ValidateRoute(route); len(errs) > 0 {
			return fail(errs[0].Message)
		}
		if _, dup := seen[route.Name]; dup {
			return fail(common.NewMessage("A virtual model named {{name}} already exists", map[string]any{"name": route.Name}))
		}
		seen[route.Name] = struct{}{}
	}
	return nil
}
