package operation_setting

import (
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
)

type ThinkingCapabilityOverride struct {
	NoThinking bool     `json:"no_thinking"`
	Efforts    []string `json:"efforts"`
}

const SmartThinkingCapabilitiesOptionKey = "smart_routing_setting.thinking_capabilities"

var thinkingCapabilityOverrides atomic.Pointer[map[string]ThinkingCapabilityOverride]

func init() {
	thinkingCapabilityOverrides.Store(&map[string]ThinkingCapabilityOverride{})
}

func LoadThinkingCapabilitiesFromJSONString(value string) {
	overrides := map[string]ThinkingCapabilityOverride{}
	if err := common.UnmarshalJsonStr(value, &overrides); err != nil {
		common.SysError(common.LogText("failed to load smart routing thinking capabilities, keeping the previous table: %s", err.Error()))
		return
	}
	thinkingCapabilityOverrides.Store(&overrides)
}

func GetThinkingCapabilityOverrides() map[string]ThinkingCapabilityOverride {
	return *thinkingCapabilityOverrides.Load()
}

func ThinkingCapabilitiesToString() string {
	raw, err := common.Marshal(*thinkingCapabilityOverrides.Load())
	if err != nil {
		return "{}"
	}
	return string(raw)
}
