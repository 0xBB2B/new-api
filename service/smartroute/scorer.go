package smartroute

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

const (
	FailureScorerError   = "scorer_error"
	FailureScorerTimeout = "scorer_timeout"
	FailureScorerInvalid = "scorer_invalid"

	scorerAutoGroup = "auto"
)

const scorerSystemPrompt = `You are a router that labels a user's latest request so a gateway can pick a model tier. You never answer the request itself.

Read the user messages (oldest first; earlier messages show only their beginning) and the local features, then return exactly one JSON object and nothing else:

{"task_type":"","reasoning":"","effort_score":0,"workload":"","risk":"","domain":"","confidence":0,"feedback":[]}

Field rules:
- task_type, judged on the LATEST message only, one of:
  chat_qa  = small talk or a simple factual question
  lookup   = find information, files, code locations or facts on the web
  rewrite  = translate, summarize, reformat or polish given text
  generate = write or change content such as code, reports, documents or copy
  analyze  = analyze data, investigate a failure, research and draw a conclusion
  plan     = design a solution, architecture, strategy or multi-part plan
- reasoning: low | medium | high. How much multi-step thinking, calculation or trade-off weighing the latest request needs.
- effort_score: a number from 0 to 1 for how much thinking the latest request needs. Higher when the goal is vague or the problem is hard; lower when the goal is clear and the task is simple.
- workload: one_step | few_steps | many_steps. How many actions (reading files, searching, running tools, fetching data) it needs.
- risk: low | high. high when the result will be published externally, used for decisions or bookkeeping, or touches production data, billing or security, and a mistake would be hard to notice.
- domain: code | finance | operations | data_analysis | other.
- confidence: a number from 0 to 1 for how sure you are about these labels.
- feedback: one value per user message shown, in the same order, each satisfied | dissatisfied | neutral. Use dissatisfied only when that message says the previous answer was wrong, did not work or missed the point.

Return only the JSON object.`

var ErrScorerInsufficientQuota = errors.New("insufficient quota for the smart routing scorer")

type ScorerOutcome struct {
	Labels  *ScorerLabels
	Failure string
	Settle  func(mainTier, mainModel string)
}

var ScorerRunner func(c *gin.Context, route operation_setting.SmartRoute, in Inspection) (ScorerOutcome, error)

func RunScorer(c *gin.Context, route operation_setting.SmartRoute, in Inspection) (ScorerOutcome, error) {
	if ScorerRunner == nil {
		return ScorerOutcome{Failure: FailureScorerError}, nil
	}
	return ScorerRunner(c, route, in)
}

func BuildScorerMessages(input ScorerInput, features LocalFeatures) (system, user string) {
	var sb strings.Builder
	sb.WriteString("Local features: latest_message_chars=")
	sb.WriteString(strconv.Itoa(features.LatestRuneCount))
	sb.WriteString(", user_turns=")
	sb.WriteString(strconv.Itoa(features.TurnCount))
	sb.WriteString(", has_attachment=")
	sb.WriteString(strconv.FormatBool(features.HasAttachment))
	sb.WriteString(", has_code_or_table=")
	sb.WriteString(strconv.FormatBool(features.HasCodeOrTable))
	sb.WriteString("\nUser messages (oldest first):")
	for i, m := range input.History {
		sb.WriteString("\n[" + strconv.Itoa(i+1) + "] " + m)
	}
	sb.WriteString("\n[latest] " + input.Latest)
	sb.WriteString("\nThe feedback array must have exactly " + strconv.Itoa(len(input.History)+1) + " values.")
	return scorerSystemPrompt, sb.String()
}

func ScorerGroups(c *gin.Context, route operation_setting.SmartRoute) []string {
	usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	groups := []string{}
	if usingGroup != scorerAutoGroup {
		return []string{usingGroup}
	}
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	for _, g := range route.Scorer.Groups {
		if service.IsUserSelectableGroup(userGroup, g) {
			groups = append(groups, g)
		}
	}
	return groups
}
