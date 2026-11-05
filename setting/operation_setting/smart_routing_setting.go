package operation_setting

import (
	"slices"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
)

const (
	SmartTierSimple = "simple"
	SmartTierNormal = "normal"
	SmartTierHard   = "hard"
)

type SmartRouteTier struct {
	Model      string   `json:"model"`
	Groups     []string `json:"groups"`
	EffortMode string   `json:"effort_mode"` // "" 不覆盖客户端的值；fixed 只用 Effort；follow 只用 EffortMin / EffortMax
	Effort     string   `json:"effort"`
	EffortMin  string   `json:"effort_min"`
	EffortMax  string   `json:"effort_max"`
}

type SmartRouteScorer struct {
	Model     string   `json:"model"`
	Groups    []string `json:"groups"`
	TimeoutMs int      `json:"timeout_ms"`
}

type SmartRouteAdvisor struct {
	Model             string   `json:"model"`
	Groups            []string `json:"groups"`
	AfterDissatisfied int      `json:"after_dissatisfied"`
}

type SmartRoute struct {
	Name                      string                    `json:"name"`
	Endpoints                 []string                  `json:"endpoints"`
	Tiers                     map[string]SmartRouteTier `json:"tiers"` // 键只允许 simple / normal / hard
	DefaultTier               string                    `json:"default_tier"`
	Scorer                    SmartRouteScorer          `json:"scorer"`
	Advisor                   SmartRouteAdvisor         `json:"advisor"`
	DowngradeMaxHistoryTokens int                       `json:"downgrade_max_history_tokens"`
	ConfidenceThreshold       float64                   `json:"confidence_threshold"`
	SimpleMaxMessageChars     int                       `json:"simple_max_message_chars"`
}

var SmartEffortLevels = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

type SmartRoutingSetting struct {
	Routes []SmartRoute `json:"routes"`
}

const SmartRoutingRoutesOptionKey = "smart_routing_setting.routes"

var smartRoutes atomic.Pointer[[]SmartRoute]

func init() {
	smartRoutes.Store(&[]SmartRoute{})
}

func LoadSmartRoutesFromJSONString(value string) {
	routes := []SmartRoute{}
	if err := common.UnmarshalJsonStr(value, &routes); err != nil {
		common.SysError(common.LogText("failed to load smart routing routes, keeping the previous routes: %s", err.Error()))
		return
	}
	smartRoutes.Store(&routes)
}

func GetSmartRoutingSetting() SmartRoutingSetting {
	return SmartRoutingSetting{Routes: *smartRoutes.Load()}
}

func FindSmartRoute(name string) (SmartRoute, bool) {
	for _, route := range *smartRoutes.Load() {
		if route.Name == name {
			return route, true
		}
	}
	return SmartRoute{}, false
}

func SmartTierRank(tier string) int {
	switch tier {
	case SmartTierSimple:
		return 0
	case SmartTierNormal:
		return 1
	case SmartTierHard:
		return 2
	}
	return -1
}

func SmartEffortRank(effort string) int {
	return slices.Index(SmartEffortLevels, effort)
}

func SmartRoutingRoutesToString() string {
	raw, err := common.Marshal(*smartRoutes.Load())
	if err != nil {
		return "[]"
	}
	return string(raw)
}
