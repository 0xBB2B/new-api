package smartroute

import (
	"errors"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	ReasonLowConfidence    = "low_confidence"
	ReasonRuleHard         = "rule_hard"
	ReasonRuleSimple       = "rule_simple"
	ReasonRuleNormal       = "rule_normal"
	ReasonAdvisor          = "advisor"
	ReasonDowngradeBlocked = "downgrade_blocked"
	ReasonToolContinuation = "tool_continuation"
	RouteAdvisor           = "advisor"
	feedbackDissatisfied   = "dissatisfied"
)

var (
	taskTypes       = []string{"chat_qa", "lookup", "rewrite", "generate", "analyze", "plan"}
	reasoningLevels = []string{"low", "medium", "high"}
	workloads       = []string{"one_step", "few_steps", "many_steps"}
	riskLevels      = []string{"low", "high"}
	domains         = []string{"code", "finance", "operations", "data_analysis", "other"}
	feedbackValues  = []string{"satisfied", "neutral", "dissatisfied"}
	simpleTaskTypes = []string{"chat_qa", "lookup", "rewrite"}
)

type ScorerLabels struct {
	TaskType   string
	Reasoning  string
	Workload   string
	Risk       string
	Domain     string
	Confidence float64
	Feedback   []string
	// EffortScore 为 nil 表示打分没给出有效的 0～1 数字
	EffortScore *float64
}

type TierChoice struct {
	Tier   string
	Reason string
	Params map[string]any
}

type SessionRecord struct {
	CurrentRoute   string
	LastNonAdvisor string
	LastChannelID  int
}

type DecideInput struct {
	Cfg           operation_setting.SmartRoute
	IsUserTurn    bool
	Labels        *ScorerLabels
	ScorerFailure string
	Session       *SessionRecord
	InputTokens   int
	LatestChars   int
}

type Decision struct {
	Selected   string
	Reason     string
	Params     map[string]any
	ScoredTier string
	Fallback   TierChoice
}

func ParseScorerOutput(raw string, expectedFeedback int) (ScorerLabels, error) {
	text := strings.TrimSpace(raw)
	if rest, ok := strings.CutPrefix(text, "```json"); ok {
		text = rest
	} else if rest, ok := strings.CutPrefix(text, "```"); ok {
		text = rest
	}
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "```"))

	var parsed struct {
		TaskType    string            `json:"task_type"`
		Reasoning   string            `json:"reasoning"`
		Workload    string            `json:"workload"`
		Risk        string            `json:"risk"`
		Domain      string            `json:"domain"`
		Confidence  *float64          `json:"confidence"`
		Feedback    []string          `json:"feedback"`
		EffortScore common.RawMessage `json:"effort_score"`
	}
	if err := common.UnmarshalJsonStr(text, &parsed); err != nil {
		return ScorerLabels{}, err
	}
	if !slices.Contains(taskTypes, parsed.TaskType) ||
		!slices.Contains(reasoningLevels, parsed.Reasoning) ||
		!slices.Contains(workloads, parsed.Workload) ||
		!slices.Contains(riskLevels, parsed.Risk) ||
		!slices.Contains(domains, parsed.Domain) {
		return ScorerLabels{}, errors.New("unknown label value")
	}
	if parsed.Confidence == nil || *parsed.Confidence < 0 || *parsed.Confidence > 1 {
		return ScorerLabels{}, errors.New("confidence missing or out of range")
	}
	if len(parsed.Feedback) != expectedFeedback {
		return ScorerLabels{}, errors.New("feedback length mismatch")
	}
	for _, f := range parsed.Feedback {
		if !slices.Contains(feedbackValues, f) {
			return ScorerLabels{}, errors.New("unknown feedback value")
		}
	}
	var effortScore *float64
	if common.GetJsonType(parsed.EffortScore) == "number" {
		var v float64
		if err := common.Unmarshal(parsed.EffortScore, &v); err == nil && v >= 0 && v <= 1 {
			effortScore = &v
		}
	}
	return ScorerLabels{
		TaskType:    parsed.TaskType,
		Reasoning:   parsed.Reasoning,
		Workload:    parsed.Workload,
		Risk:        parsed.Risk,
		Domain:      parsed.Domain,
		Confidence:  *parsed.Confidence,
		Feedback:    parsed.Feedback,
		EffortScore: effortScore,
	}, nil
}

func SelectTier(cfg operation_setting.SmartRoute, labels ScorerLabels, latestChars int) TierChoice {
	if labels.Confidence < cfg.ConfidenceThreshold {
		return TierChoice{Tier: cfg.DefaultTier, Reason: ReasonLowConfidence}
	}
	var matched []string
	if labels.Risk == "high" {
		matched = append(matched, "risk")
	}
	if labels.TaskType == "plan" {
		matched = append(matched, "task_type")
	}
	if labels.Reasoning == "high" {
		matched = append(matched, "reasoning")
	}
	if len(matched) > 0 {
		return TierChoice{Tier: operation_setting.SmartTierHard, Reason: ReasonRuleHard, Params: map[string]any{"matched": matched}}
	}
	if slices.Contains(simpleTaskTypes, labels.TaskType) && labels.Reasoning == "low" &&
		labels.Workload == "one_step" && latestChars < cfg.SimpleMaxMessageChars {
		return TierChoice{Tier: operation_setting.SmartTierSimple, Reason: ReasonRuleSimple}
	}
	return TierChoice{Tier: operation_setting.SmartTierNormal, Reason: ReasonRuleNormal}
}

func CountTrailingDissatisfied(feedback []string) int {
	n := 0
	for i := len(feedback) - 1; i >= 0 && feedback[i] == feedbackDissatisfied; i-- {
		n++
	}
	return n
}

func ApplyDowngradeGuard(cfg operation_setting.SmartRoute, choice TierChoice, lastNonAdvisor string, inputTokens int) TierChoice {
	if lastNonAdvisor == "" ||
		operation_setting.SmartTierRank(choice.Tier) >= operation_setting.SmartTierRank(lastNonAdvisor) ||
		inputTokens < cfg.DowngradeMaxHistoryTokens {
		return choice
	}
	return TierChoice{
		Tier:   lastNonAdvisor,
		Reason: ReasonDowngradeBlocked,
		Params: map[string]any{
			"history_tokens": inputTokens,
			"limit":          cfg.DowngradeMaxHistoryTokens,
			"kept_tier":      lastNonAdvisor,
			"scored_tier":    choice.Tier,
		},
	}
}

func DecideRoute(in DecideInput) Decision {
	if !in.IsUserTurn {
		selected, fallback := in.Cfg.DefaultTier, in.Cfg.DefaultTier
		if in.Session != nil {
			if in.Session.CurrentRoute != "" {
				selected = in.Session.CurrentRoute
			}
			if in.Session.LastNonAdvisor != "" {
				fallback = in.Session.LastNonAdvisor
			}
		}
		return Decision{Selected: selected, Reason: ReasonToolContinuation, Fallback: TierChoice{Tier: fallback, Reason: ReasonToolContinuation}}
	}

	var choice TierChoice
	if in.ScorerFailure != "" || in.Labels == nil {
		choice = TierChoice{Tier: in.Cfg.DefaultTier, Reason: in.ScorerFailure}
		if in.ScorerFailure == FailureScorerTimeout {
			choice.Params = map[string]any{"timeout_ms": in.Cfg.Scorer.TimeoutMs}
		}
	} else {
		choice = SelectTier(in.Cfg, *in.Labels, in.LatestChars)
	}
	scored := choice.Tier
	if in.Session != nil {
		choice = ApplyDowngradeGuard(in.Cfg, choice, in.Session.LastNonAdvisor, in.InputTokens)
	}

	if in.Labels != nil && in.ScorerFailure == "" {
		if n := CountTrailingDissatisfied(in.Labels.Feedback); n >= in.Cfg.Advisor.AfterDissatisfied {
			return Decision{Selected: RouteAdvisor, Reason: ReasonAdvisor, Params: map[string]any{"count": n}, ScoredTier: scored, Fallback: choice}
		}
	}
	return Decision{Selected: choice.Tier, Reason: choice.Reason, Params: choice.Params, ScoredTier: scored, Fallback: choice}
}
