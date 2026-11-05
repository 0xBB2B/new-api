package smartroute

import (
	"slices"

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
	switch {
	case len(route.Endpoints) == 0:
		add("endpoints", "Select at least one endpoint")
	case slices.ContainsFunc(route.Endpoints, func(endpoint string) bool { return !slices.Contains(routeEndpointTypes, endpoint) }):
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
