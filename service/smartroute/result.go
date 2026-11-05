package smartroute

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

type RouteResult struct {
	VirtualModel           string
	Tier                   string
	ScoredTier             string
	Reason                 string
	Params                 map[string]any
	Labels                 *ScorerLabels
	ScorerStatus           string
	ScorerBilled           bool
	IsUserTurn             bool
	SessionKey             string
	Session                *SessionRecord
	StrippedReasoningCount int
}

const (
	ScorerStatusOK      = "ok"
	ScorerStatusTimeout = "timeout"
	ScorerStatusError   = "error"
	ScorerStatusInvalid = "invalid"
	ScorerStatusSkipped = "skipped"
)

func SetRouteResult(c *gin.Context, result RouteResult) {
	common.SetContextKey(c, constant.ContextKeySmartRouteResult, result)
	common.SetContextKey(c, constant.ContextKeySmartRouteLog, MainLogFields(result, result.StrippedReasoningCount))
}

func GetRouteResult(c *gin.Context) (RouteResult, bool) {
	value, ok := common.GetContextKey(c, constant.ContextKeySmartRouteResult)
	if !ok {
		return RouteResult{}, false
	}
	result, ok := value.(RouteResult)
	return result, ok
}
