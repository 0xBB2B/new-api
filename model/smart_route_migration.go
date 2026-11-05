package model

import (
	"errors"
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// 旧格式没有 endpoints，可用接口由三档和顾问模型支持接口的交集推出；这里按同样规则写进配置，升级后客户端能用的接口不变。
func MigrateSmartRouteSettings() error {
	common.OptionMapRWMutex.RLock()
	value, ok := common.OptionMap[operation_setting.SmartRoutingRoutesOptionKey]
	common.OptionMapRWMutex.RUnlock()
	if !ok {
		return nil
	}

	var routes []operation_setting.SmartRoute
	if err := common.UnmarshalJsonStr(value, &routes); err != nil {
		return err
	}

	changed, pricingChecked := false, false
	for i := range routes {
		route := &routes[i]
		if len(route.Endpoints) == 0 {
			if !pricingChecked {
				if len(GetPricing()) == 0 {
					return errors.New("pricing is empty, cannot infer endpoints for smart routing migration")
				}
				pricingChecked = true
			}
			route.Endpoints = legacySmartRouteEndpoints(*route)
			changed = true
		}
		for name, tier := range route.Tiers {
			if tier.Effort != "" && tier.EffortMode == "" {
				tier.EffortMode = "fixed"
				route.Tiers[name] = tier
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}

	raw, err := common.Marshal(routes)
	if err != nil {
		return err
	}
	return UpdateOption(operation_setting.SmartRoutingRoutesOptionKey, string(raw))
}

func legacySmartRouteEndpoints(route operation_setting.SmartRoute) []string {
	endpoints := []constant.EndpointType{constant.EndpointTypeOpenAIResponse, constant.EndpointTypeOpenAI}
	for _, modelName := range []string{
		route.Tiers[operation_setting.SmartTierSimple].Model,
		route.Tiers[operation_setting.SmartTierNormal].Model,
		route.Tiers[operation_setting.SmartTierHard].Model,
		route.Advisor.Model,
	} {
		if modelName == "" {
			continue
		}
		supported := GetModelSupportEndpointTypes(modelName)
		endpoints = slices.DeleteFunc(endpoints, func(endpoint constant.EndpointType) bool {
			return !slices.Contains(supported, endpoint)
		})
	}
	if len(endpoints) == 0 {
		return []string{string(constant.EndpointTypeOpenAIResponse)}
	}
	result := make([]string, len(endpoints))
	for i, endpoint := range endpoints {
		result[i] = string(endpoint)
	}
	return result
}
