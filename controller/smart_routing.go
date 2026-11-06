package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/smartroute"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func ValidateSmartRoute(c *gin.Context) {
	var request struct {
		Route operation_setting.SmartRoute `json:"route"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Invalid parameters"))
		return
	}
	fieldErrors := smartroute.ValidateRoute(request.Route)
	errs := make([]gin.H, 0, len(fieldErrors))
	for _, fieldError := range fieldErrors {
		errs = append(errs, gin.H{"field": fieldError.Field, "message": fieldError.Message})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"errors": errs}})
}

func validateSmartRoutesOption(value string) error {
	var routes []operation_setting.SmartRoute
	if err := common.UnmarshalJsonStr(value, &routes); err != nil {
		return common.NewMessage("Invalid parameters")
	}
	return smartroute.ValidateRoutes(routes)
}

func validateThinkingCapabilitiesOption(value string) error {
	var table map[string]operation_setting.ThinkingCapabilityOverride
	if err := common.UnmarshalJsonStr(value, &table); err != nil || table == nil {
		return common.NewMessage("Invalid parameters")
	}
	for name, capability := range table {
		if strings.TrimSpace(name) == "" {
			return common.NewMessage("Enter a model name")
		}
		if capability.NoThinking {
			continue
		}
		if len(capability.Efforts) == 0 {
			return common.NewMessage("Model {{model}} must either disable thinking or list at least one effort", map[string]any{"model": name})
		}
		for _, effort := range capability.Efforts {
			if operation_setting.SmartEffortRank(effort) < 0 {
				return common.NewMessage("Model {{model}} has an unknown reasoning effort {{effort}}", map[string]any{"model": name, "effort": effort})
			}
		}
	}
	return nil
}

func GetSmartRouteThinkingCapabilities(c *gin.Context) {
	data := gin.H{}
	for name := range strings.SplitSeq(c.Query("models"), ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		capability := smartroute.BuiltinThinkingCapability(name)
		data[name] = gin.H{
			"known":       capability.Source == smartroute.ThinkingSourceBuiltin,
			"no_thinking": capability.NoThinking,
			"efforts":     capability.Efforts,
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
