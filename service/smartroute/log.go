package smartroute

import "maps"

func MainLogFields(result RouteResult, cleanedReasoning int) map[string]any {
	params := map[string]any{}
	maps.Copy(params, result.Params)

	fields := map[string]any{
		"role":          "main",
		"virtual_model": result.VirtualModel,
		"tier":          result.Tier,
		"reason":        result.Reason,
		"reason_params": params,
	}
	if result.Reason == ReasonDowngradeBlocked && result.ScoredTier != "" {
		fields["scored_tier"] = result.ScoredTier
	}

	if labels := result.Labels; labels != nil {
		logLabels := map[string]any{
			"task_type":  labels.TaskType,
			"reasoning":  labels.Reasoning,
			"workload":   labels.Workload,
			"risk":       labels.Risk,
			"domain":     labels.Domain,
			"confidence": labels.Confidence,
		}
		if n := len(labels.Feedback); n > 0 {
			logLabels["feedback"] = labels.Feedback[n-1]
		}
		fields["labels"] = logLabels
	}

	scorer := map[string]any{
		"status": result.ScorerStatus,
		"billed": result.ScorerBilled,
	}
	if result.ScorerStatus == ScorerStatusTimeout {
		if timeoutMs, ok := result.Params["timeout_ms"]; ok {
			scorer["timeout_ms"] = timeoutMs
		}
	}
	fields["scorer"] = scorer

	if result.Effort.Source != "" {
		effort := map[string]any{"value": result.Effort.Value, "source": result.Effort.Source}
		if result.Effort.From != "" {
			effort["from"] = result.Effort.From
		}
		fields["effort"] = effort
	}

	if cleanedReasoning > 0 {
		fields["cleaned_reasoning"] = cleanedReasoning
	}
	return fields
}

func ScorerLogFields(virtualModel, mainTier, mainModel string) map[string]any {
	return map[string]any{
		"role":          "scorer",
		"virtual_model": virtualModel,
		"main_tier":     mainTier,
		"main_model":    mainModel,
	}
}
