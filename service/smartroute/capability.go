package smartroute

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/reasoning"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	ThinkingSourceRecord      = "record"
	ThinkingSourceInferred    = "inferred"
	ThinkingSourceUnconfirmed = "unconfirmed"
)

type ThinkingCapability struct {
	Source     string
	NoThinking bool
	Efforts    []string
}

// 推算规则覆盖 2026 年发布的 Claude、Gemini 3.x，以及 relaykit 判定不支持思考的 Gemini 模型；其余型号以模型记录为准。
// 档位的官方出处：
//   - Claude 档位: https://platform.claude.com/docs/en/build-with-claude/effort 和 https://platform.claude.com/docs/en/build-with-claude/thinking-troubleshooting#supported-models
//   - Claude 发布日期: https://platform.claude.com/docs/en/release-notes/overview
//   - Gemini 档位: https://docs.cloud.google.com/vertex-ai/generative-ai/docs/thinking
//   - Gemini 发布日期: https://aistudio.google.com/docs/changelog
var claudeReleasedIn2026 = []string{
	"claude-opus-4-6", "claude-sonnet-4-6", "claude-opus-4-7", "claude-opus-4-8", "claude-mythos-preview",
	"claude-fable-5", "claude-mythos-5", "claude-opus-5", "claude-sonnet-5", "claude-haiku-5",
}

func inferredThinkingCapability(name string) ThinkingCapability {
	lower := strings.ToLower(name)
	if slices.ContainsFunc(claudeReleasedIn2026, func(prefix string) bool { return strings.HasPrefix(lower, prefix) }) {
		if efforts, known := reasoning.ClaudeSupportedEfforts(name); known {
			return ThinkingCapability{Source: ThinkingSourceInferred, Efforts: effortStrings(efforts)}
		}
	}
	if efforts, noThinking, known := reasoning.GeminiSupportedEfforts(name); known && (noThinking || strings.HasPrefix(lower, "gemini-3.")) {
		return ThinkingCapability{Source: ThinkingSourceInferred, NoThinking: noThinking, Efforts: effortStrings(efforts)}
	}
	return ThinkingCapability{Source: ThinkingSourceUnconfirmed, Efforts: []string{"low", "medium", "high"}}
}

func EffectiveThinkingCapability(name string) ThinkingCapability {
	return ThinkingCapabilityFromRecord(name, model.GetModelCapabilityByName(name))
}

func ThinkingCapabilityFromRecord(name string, capability model.ModelCapability) ThinkingCapability {
	if efforts, configured := model.ModelThinkingSupport(capability); configured {
		return ThinkingCapability{Source: ThinkingSourceRecord, NoThinking: len(efforts) == 0, Efforts: efforts}
	}
	return inferredThinkingCapability(name)
}

// ClampEffortToCapability 把档位收敛到能力里按七档顺序最近的一档，距离相同取更高。
func ClampEffortToCapability(effort string, capability ThinkingCapability) (string, bool) {
	if effort == "" || capability.NoThinking || len(capability.Efforts) == 0 {
		return "", false
	}
	if slices.Contains(capability.Efforts, effort) {
		return effort, false
	}
	target := operation_setting.SmartEffortRank(effort)
	best, bestDistance := "", -1
	for _, candidate := range capability.Efforts {
		distance := target - operation_setting.SmartEffortRank(candidate)
		if distance < 0 {
			distance = -distance
		}
		rank := operation_setting.SmartEffortRank(candidate)
		if bestDistance < 0 || distance < bestDistance || (distance == bestDistance && rank > operation_setting.SmartEffortRank(best)) {
			best, bestDistance = candidate, distance
		}
	}
	return best, true
}

func effortStrings(efforts []reasoning.Effort) []string {
	result := make([]string, 0, len(efforts))
	for _, effort := range efforts {
		result = append(result, string(effort))
	}
	return result
}
