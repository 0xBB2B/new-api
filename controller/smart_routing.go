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

func GetSmartRouteThinkingCapabilities(c *gin.Context) {
	data := gin.H{}
	for name := range strings.SplitSeq(c.Query("models"), ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		capability := smartroute.EffectiveThinkingCapability(name)
		data[name] = gin.H{
			"known":       capability.Source != smartroute.ThinkingSourceUnconfirmed,
			"no_thinking": capability.NoThinking,
			"efforts":     capability.Efforts,
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
