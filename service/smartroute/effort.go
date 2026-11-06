package smartroute

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	EffortSourceFollow     = "follow"
	EffortSourceFixed      = "fixed"
	EffortSourceClient     = "client"
	EffortSourceUnset      = "client_unset"
	EffortSourceNoThinking = "no_thinking"

	EffortModeFixed  = "fixed"
	EffortModeFollow = "follow"
)

type EffortDecision struct {
	Value  string
	Source string
	// From 只在按模型能力换档时有值，是换档前的值
	From string
	Drop bool
}

type EffortInput struct {
	Tier          operation_setting.SmartRouteTier
	IsUserTurn    bool
	Labels        *ScorerLabels
	ScorerFailed  bool
	ClientEffort  string
	SessionEffort string
	SessionSource string
	Capability    ThinkingCapability
}

// scoredEffort 把打分标签里「要想多深」映射成力度，没有标签时返回空串。
func scoredEffort(labels *ScorerLabels) string {
	if labels == nil {
		return ""
	}
	switch labels.Reasoning {
	case "low", "medium", "high":
		return labels.Reasoning
	}
	return ""
}

func ResolveTierEffort(in EffortInput) EffortDecision {
	tier := in.Tier
	value, source := "", ""

	switch {
	case !in.IsUserTurn && in.SessionEffort != "":
		value, source = in.SessionEffort, in.SessionSource
	case tier.EffortMode == EffortModeFixed:
		value, source = tier.Effort, EffortSourceFixed
	case tier.EffortMode == EffortModeFollow:
		source = EffortSourceFollow
		scored := scoredEffort(in.Labels)
		if !in.IsUserTurn || in.ScorerFailed || scored == "" {
			value = tier.EffortMax
			break
		}
		value = scored
		if operation_setting.SmartEffortRank(value) < operation_setting.SmartEffortRank(tier.EffortMin) {
			value = tier.EffortMin
		}
		if operation_setting.SmartEffortRank(value) > operation_setting.SmartEffortRank(tier.EffortMax) {
			value = tier.EffortMax
		}
	default:
		if in.ClientEffort == "" {
			source = EffortSourceUnset
			break
		}
		value, source = strings.ToLower(in.ClientEffort), EffortSourceClient
		if operation_setting.SmartEffortRank(value) < 0 {
			value = in.ClientEffort
		}
	}

	if in.Capability.NoThinking {
		return EffortDecision{Source: EffortSourceNoThinking, Drop: true}
	}
	decision := EffortDecision{Value: value, Source: source}
	if value == "" || operation_setting.SmartEffortRank(value) < 0 {
		return decision
	}
	if clamped, changed := ClampEffortToCapability(value, in.Capability); clamped != "" && changed {
		decision.Value, decision.From = clamped, value
	}
	return decision
}
