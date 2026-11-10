package smartroute

import (
	"slices"
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
		minRank := operation_setting.SmartEffortRank(tier.EffortMin)
		maxRank := operation_setting.SmartEffortRank(tier.EffortMax)
		var inRange, candidates []string
		for _, level := range operation_setting.SmartEffortLevels {
			rank := operation_setting.SmartEffortRank(level)
			if rank < minRank || rank > maxRank {
				continue
			}
			inRange = append(inRange, level)
			if slices.Contains(in.Capability.Efforts, level) {
				candidates = append(candidates, level)
			}
		}
		pool := candidates
		if len(pool) == 0 {
			pool = inRange
		}
		hasScore := in.IsUserTurn && !in.ScorerFailed && in.Labels != nil && in.Labels.EffortScore != nil
		switch {
		case len(pool) > 0 && hasScore:
			value = pool[min(int(*in.Labels.EffortScore*float64(len(pool))), len(pool)-1)]
		case len(candidates) > 0:
			value = candidates[len(candidates)-1]
		default:
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
