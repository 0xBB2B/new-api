package smartroute

import (
	"regexp"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/relayconvert/reasoning"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	ThinkingSourceBuiltin     = "builtin"
	ThinkingSourceOverride    = "override"
	ThinkingSourceUnconfirmed = "unconfirmed"
)

type ThinkingCapability struct {
	Source     string
	NoThinking bool
	Efforts    []string
}

// 内置表只收录 2026 年发布的型号，2025 年及以前的按未确认处理；已确认不支持思考的老模型（gpt-4o、gpt-4.1、gpt-4.1-mini、gemini-2.5-flash-image）例外，
// 收录它们是为了避免给不会思考的模型发思考参数。其余（o 系列、gpt-5-mini、gpt-5-nano、gpt-5.6-cyber 等）同样按未确认处理。
// 发布日期与档位的官方出处：
//   - OpenAI 发布日期: https://developers.openai.com/api/docs/changelog
//   - Claude 档位: https://platform.claude.com/docs/en/build-with-claude/effort 和 https://platform.claude.com/docs/en/build-with-claude/thinking-troubleshooting#supported-models
//   - Claude 发布日期: https://platform.claude.com/docs/en/release-notes/overview
//   - Gemini 档位: https://docs.cloud.google.com/vertex-ai/generative-ai/docs/thinking
//   - Gemini 发布日期: https://aistudio.google.com/docs/changelog
//   - GLM 档位: https://docs.bigmodel.cn/cn/guide/capabilities/thinking（GLM-5.3 系列仅支持 max、high、low；GLM-5.2 的 low/medium 映射为 high，xhigh 映射为 max）
//   - GLM 发布日期: https://docs.bigmodel.cn/cn/update/new-releases
//   - Kimi 档位: https://platform.kimi.com/docs/guide/use-reasoning-effort（kimi-k3.1-pro 始终思考，只有 low、high、max）
//   - Qwen 档位: https://www.alibabacloud.com/help/en/model-studio/qwen-api-via-openai-chat-completions（none 关闭思考，其余值映射到下列档位）
//   - Qwen 发布日期: https://www.alibabacloud.com/blog/qwen3-8-max-a-new-bar-for-coding-and-cowork_603421 、https://www.alibabacloud.com/blog/alibaba-unveils-qwen4-max-new-ai-teammate-for-work-creativity-and-discovery_603783
//
// 依据（OpenAI 型号文档为 https://developers.openai.com/api/docs/models/<型号>）：
//   - gpt-4.1 不支持思考: https://developers.openai.com/api/docs/models/gpt-4.1
//   - gpt-4o 不支持思考: https://developers.openai.com/api/docs/models/gpt-4o
//   - gpt-4.1-mini 不支持思考: https://developers.openai.com/api/docs/models/gpt-4.1-mini
//   - gpt-5.4 none/low/medium/high/xhigh: https://developers.openai.com/api/docs/models/gpt-5.4
//   - gpt-5.4-mini none/low/medium/high/xhigh: https://developers.openai.com/api/docs/models/gpt-5.4-mini
//   - gpt-5.4-nano none/low/medium/high/xhigh: https://developers.openai.com/api/docs/models/gpt-5.4-nano
//   - gpt-5.5 none/low/medium/high/xhigh: https://developers.openai.com/api/docs/models/gpt-5.5
//   - gpt-5.4-pro medium/high/xhigh: https://developers.openai.com/api/docs/models/gpt-5.4-pro
//   - gpt-5.5-pro medium/high/xhigh: https://developers.openai.com/api/docs/models/gpt-5.5-pro
//   - gpt-5.2-codex low/medium/high/xhigh: https://developers.openai.com/api/docs/models/gpt-5.2-codex
//   - gpt-5.3-codex low/medium/high/xhigh: https://developers.openai.com/api/docs/models/gpt-5.3-codex
//   - gpt-5.6 none/low/medium/high/xhigh/max: https://developers.openai.com/api/docs/models/gpt-5.6
//   - gpt-5.6-sol none/low/medium/high/xhigh/max: https://developers.openai.com/api/docs/models/gpt-5.6-sol
//   - gpt-5.6-terra none/low/medium/high/xhigh/max: https://developers.openai.com/api/docs/models/gpt-5.6-terra
//   - gpt-5.6-luna none/low/medium/high/xhigh/max: https://developers.openai.com/api/docs/models/gpt-5.6-luna
//   - gpt-6-sol none/low/medium/high/xhigh/max: https://developers.openai.com/api/docs/models/gpt-6-sol
//   - gpt-6-luna none/low/medium/high/xhigh/max: https://developers.openai.com/api/docs/models/gpt-6-luna
//   - gpt-6.1-terra none/low/medium/high/xhigh/max: https://developers.openai.com/api/docs/models/gpt-6.1-terra
//   - gpt-6-astra low/medium/high/xhigh/max: https://developers.openai.com/api/docs/models/gpt-6-astra
//   - gpt-6.1-sol low/medium/high/xhigh/max: https://developers.openai.com/api/docs/models/gpt-6.1-sol
//   - gpt-6.2-sol、gpt-6.2-astra low/medium/high/xhigh/max：官方型号页只写明拒绝 none；2026-11-18 经 Codex 渠道实测这五档都被接受（该渠道也接受 none，与公开 API 不同）:
//     https://developers.openai.com/api/docs/models/gpt-6.2-sol 、https://developers.openai.com/api/docs/models/gpt-6.2-astra
//   - glm-5.3、glm-5.3-flash、glm-5.3-flashx low/high/max: https://docs.bigmodel.cn/cn/guide/capabilities/thinking
//   - glm-5.2 none/high/max: https://docs.bigmodel.cn/cn/guide/capabilities/thinking
//   - deepseek-v4-pro none/low/high/max，thinking 可关闭: https://api-docs.deepseek.com/guides/thinking_mode
//   - deepseek-v4-flash none/low/high/max: https://api-docs.deepseek.com/updates（2026-08-13）
//   - deepseek-v4.1-flash none/low/high/max: https://api-docs.deepseek.com/updates（2026-09-10）和 https://api-docs.deepseek.com/guides/thinking_mode
//   - kimi-k3.1-pro low/high/max: https://platform.kimi.com/docs/guide/use-reasoning-effort
//   - qwen3.8-max none/low/medium/xhigh（high、max 映射为 xhigh）: https://www.alibabacloud.com/help/en/model-studio/qwen-api-via-openai-chat-completions
//   - qwen4-max none/low/high/xhigh（medium 映射为 high，max 映射为 xhigh）: https://www.alibabacloud.com/help/en/model-studio/qwen-api-via-openai-chat-completions
//
// 查表前先去掉结尾的日期快照后缀，再按完整型号名精确匹配，不做前缀匹配（gpt-5-pro 不能当成 gpt-5）。
var builtinThinkingCapabilities = map[string]ThinkingCapability{
	"gpt-4.1":             {Source: ThinkingSourceBuiltin, NoThinking: true, Efforts: []string{}},
	"gpt-4o":              {Source: ThinkingSourceBuiltin, NoThinking: true, Efforts: []string{}},
	"gpt-4.1-mini":        {Source: ThinkingSourceBuiltin, NoThinking: true, Efforts: []string{}},
	"gpt-5.4":             {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh"}},
	"gpt-5.4-mini":        {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh"}},
	"gpt-5.4-nano":        {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh"}},
	"gpt-5.5":             {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh"}},
	"gpt-5.4-pro":         {Source: ThinkingSourceBuiltin, Efforts: []string{"medium", "high", "xhigh"}},
	"gpt-5.5-pro":         {Source: ThinkingSourceBuiltin, Efforts: []string{"medium", "high", "xhigh"}},
	"gpt-5.2-codex":       {Source: ThinkingSourceBuiltin, Efforts: []string{"low", "medium", "high", "xhigh"}},
	"gpt-5.3-codex":       {Source: ThinkingSourceBuiltin, Efforts: []string{"low", "medium", "high", "xhigh"}},
	"gpt-5.6":             {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh", "max"}},
	"gpt-5.6-sol":         {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh", "max"}},
	"gpt-5.6-terra":       {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh", "max"}},
	"gpt-5.6-luna":        {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh", "max"}},
	"gpt-6-sol":           {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh", "max"}},
	"gpt-6-luna":          {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh", "max"}},
	"gpt-6.1-terra":       {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "high", "xhigh", "max"}},
	"gpt-6-astra":         {Source: ThinkingSourceBuiltin, Efforts: []string{"low", "medium", "high", "xhigh", "max"}},
	"gpt-6.1-sol":         {Source: ThinkingSourceBuiltin, Efforts: []string{"low", "medium", "high", "xhigh", "max"}},
	"gpt-6.2-sol":         {Source: ThinkingSourceBuiltin, Efforts: []string{"low", "medium", "high", "xhigh", "max"}},
	"gpt-6.2-astra":       {Source: ThinkingSourceBuiltin, Efforts: []string{"low", "medium", "high", "xhigh", "max"}},
	"glm-5.3":             {Source: ThinkingSourceBuiltin, Efforts: []string{"low", "high", "max"}},
	"glm-5.3-flash":       {Source: ThinkingSourceBuiltin, Efforts: []string{"low", "high", "max"}},
	"glm-5.3-flashx":      {Source: ThinkingSourceBuiltin, Efforts: []string{"low", "high", "max"}},
	"glm-5.2":             {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "high", "max"}},
	"deepseek-v4-pro":     {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "high", "max"}},
	"deepseek-v4-flash":   {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "high", "max"}},
	"deepseek-v4.1-flash": {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "high", "max"}},
	"kimi-k3.1-pro":       {Source: ThinkingSourceBuiltin, Efforts: []string{"low", "high", "max"}},
	"qwen3.8-max":         {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "medium", "xhigh"}},
	"qwen4-max":           {Source: ThinkingSourceBuiltin, Efforts: []string{"none", "low", "high", "xhigh"}},
}

var dateSnapshotSuffix = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}$`)

var claudeReleasedIn2026 = []string{
	"claude-opus-4-6", "claude-sonnet-4-6", "claude-opus-4-7", "claude-opus-4-8", "claude-mythos-preview",
	"claude-fable-5", "claude-mythos-5", "claude-opus-5", "claude-sonnet-5", "claude-haiku-5",
}

func BuiltinThinkingCapability(model string) ThinkingCapability {
	lower := strings.ToLower(model)
	if slices.ContainsFunc(claudeReleasedIn2026, func(prefix string) bool { return strings.HasPrefix(lower, prefix) }) {
		if efforts, known := reasoning.ClaudeSupportedEfforts(model); known {
			return ThinkingCapability{Source: ThinkingSourceBuiltin, Efforts: effortStrings(efforts)}
		}
	}
	if efforts, noThinking, known := reasoning.GeminiSupportedEfforts(model); known && (noThinking || strings.HasPrefix(lower, "gemini-3.")) {
		return ThinkingCapability{Source: ThinkingSourceBuiltin, NoThinking: noThinking, Efforts: effortStrings(efforts)}
	}
	if capability, ok := builtinThinkingCapabilities[dateSnapshotSuffix.ReplaceAllString(strings.ToLower(model), "")]; ok {
		return capability
	}
	return ThinkingCapability{Source: ThinkingSourceUnconfirmed, Efforts: []string{"low", "medium", "high"}}
}

func EffectiveThinkingCapability(model string) ThinkingCapability {
	if override, ok := operation_setting.GetThinkingCapabilityOverrides()[model]; ok {
		efforts := override.Efforts
		if efforts == nil {
			efforts = []string{}
		}
		return ThinkingCapability{Source: ThinkingSourceOverride, NoThinking: override.NoThinking, Efforts: efforts}
	}
	return BuiltinThinkingCapability(model)
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
