package core

// Replay plan model + builder. A replay plan answers a single question:
// "What offline replay/analysis can this saved artifact support?" It does NOT
// replay anything by re-executing it. The plan is a deterministic capability
// description derived only from the detected Input — every artifact also lists the
// fixed set of analyses the replay tool deliberately never performs (execution,
// runtime probing, referenced-file reread, mutation/enforcement).

// PlanSchemaVersion is the schema version of the replay plan. It is a local tool
// schema (not a REST/API contract) and is independent of the workflow artifact
// schema versions it analyzes.
const PlanSchemaVersion = 1

// Item statuses.
const (
	ItemAvailable   = "available"
	ItemUnavailable = "unavailable"
)

// PlanItem is one offline analysis the artifact either supports (available) or
// that replay deliberately never performs (unavailable). Type is a stable local
// identifier; Inputs names the artifact fields an available analysis would read.
type PlanItem struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Inputs  []string `json:"inputs"`
}

// PlanSummary counts available vs unavailable items.
type PlanSummary struct {
	Available   int `json:"available"`
	Unavailable int `json:"unavailable"`
}

// Plan is the deterministic replay plan for one artifact. Workflow and Result are
// omitted when the artifact has none (e.g. a history export).
type Plan struct {
	SchemaVersion int         `json:"schemaVersion"`
	Source        string      `json:"source"`
	Workflow      string      `json:"workflow,omitempty"`
	Result        string      `json:"result,omitempty"`
	Items         []PlanItem  `json:"items"`
	Summary       PlanSummary `json:"summary"`
}

// BuildPlan builds the deterministic replay plan for a detected Input. The
// available analyses depend on the source kind and the presence of the inputs they
// need; the four execution/runtime/reread/mutation items are always appended as
// unavailable so every plan states the replay tool's safety boundary explicitly.
func BuildPlan(in Input) Plan {
	var items []PlanItem

	switch in.Kind {
	case SourceWorkflowRunReport:
		items = append(items,
			availIf(in.Steps > 0, "step-timeline", "replay.analysis.timeline",
				"Step timeline can be reconstructed from saved step records.",
				"No step records are present in this report.",
				[]string{"steps"}),
			availIf(in.Steps > 0, "step-status-summary", "replay.analysis.step_status",
				"Failed and skipped steps can be summarized from saved step records.",
				"No step records are present in this report.",
				[]string{"steps"}),
			availIf(in.HasCodes, "explanation-lookup", "replay.analysis.explanations",
				"Emitted codes can be looked up offline for explanations.",
				"No explainable codes are present in this report.",
				[]string{"steps", "explanations"}),
		)

	case SourceWorkflowPlanReport:
		items = append(items,
			availIf(in.Steps > 0, "step-validation-summary", "replay.analysis.step_validation",
				"Step validation results can be summarized from the saved plan.",
				"No steps are present in this plan.",
				[]string{"steps"}),
			availIf(in.HasCodes, "explanation-lookup", "replay.analysis.explanations",
				"Emitted validation codes can be looked up offline for explanations.",
				"No explainable codes are present in this plan.",
				[]string{"steps", "explanations"}),
		)

	case SourceWorkflowHistoryExport:
		hasRuns := in.Runs > 0
		items = append(items,
			availIf(hasRuns, "result-distribution", "replay.analysis.result_distribution",
				"Run result distribution can be computed from the exported runs.",
				"The export contains no runs.",
				[]string{"runs"}),
			availIf(hasRuns, "workflow-distribution", "replay.analysis.workflow_distribution",
				"Workflow distribution can be computed from the exported runs.",
				"The export contains no runs.",
				[]string{"runs"}),
			availIf(hasRuns, "step-status-distribution", "replay.analysis.step_status_distribution",
				"Step status distribution can be computed from the exported runs.",
				"The export contains no runs.",
				[]string{"runs"}),
			availIf(hasRuns, "step-type-distribution", "replay.analysis.step_type_distribution",
				"Step type distribution can be computed from the exported runs.",
				"The export contains no runs.",
				[]string{"runs"}),
			availIf(hasRuns, "code-distribution", "replay.analysis.code_distribution",
				"Emitted code distribution can be computed from the exported runs.",
				"The export contains no runs.",
				[]string{"runs"}),
			availIf(hasRuns, "failed-run-shortlist", "replay.analysis.failed_runs",
				"A shortlist of failed runs can be built from the exported runs.",
				"The export contains no runs.",
				[]string{"runs"}),
		)

	case SourceWorkflowReportBundle:
		items = append(items,
			avail("summary-reconstruction", "replay.analysis.summary",
				"The report summary can be reconstructed from the bundle.",
				[]string{"manifest", "report"}),
			avail("normalized-report-inspection", "replay.analysis.report_inspection",
				"The normalized report can be inspected from the bundle.",
				[]string{"report"}),
			avail("explanation-lookup", "replay.analysis.explanations",
				"Emitted codes can be looked up offline for explanations.",
				[]string{"explanations"}),
		)
	}

	items = append(items, unavailableSafetyItems()...)

	summary := PlanSummary{}
	for _, it := range items {
		if it.Status == ItemAvailable {
			summary.Available++
		} else {
			summary.Unavailable++
		}
	}

	return Plan{
		SchemaVersion: PlanSchemaVersion,
		Source:        in.Kind,
		Workflow:      in.Workflow,
		Result:        in.Result,
		Items:         items,
		Summary:       summary,
	}
}

// unavailableSafetyItems are the analyses the replay tool deliberately never
// performs. They appear in every plan so the offline, read-only safety boundary is
// always explicit.
func unavailableSafetyItems() []PlanItem {
	return []PlanItem{
		item(ItemUnavailable, "workflow-rerun", "replay.execution",
			"Replay analysis does not re-execute workflows.", nil),
		item(ItemUnavailable, "runtime-probing", "replay.runtime",
			"Replay analysis does not contact the runtime.", nil),
		item(ItemUnavailable, "config-policy-reread", "replay.input.reread",
			"Replay analysis does not re-read referenced config, policy, baseline, or report files.", nil),
		item(ItemUnavailable, "mutation-enforcement", "replay.mutation",
			"Replay analysis does not mutate inputs or enforce policy.", nil),
	}
}

// item builds a PlanItem, ensuring Inputs is always a non-nil slice so it
// serializes as `[]` rather than `null`.
func item(status, id, typ, message string, inputs []string) PlanItem {
	if inputs == nil {
		inputs = []string{}
	}
	return PlanItem{ID: id, Type: typ, Status: status, Message: message, Inputs: inputs}
}

// avail builds an available PlanItem.
func avail(id, typ, message string, inputs []string) PlanItem {
	return item(ItemAvailable, id, typ, message, inputs)
}

// availIf builds an available item with availMsg/inputs when cond holds, otherwise
// an unavailable item with unavailMsg and no inputs.
func availIf(cond bool, id, typ, availMsg, unavailMsg string, inputs []string) PlanItem {
	if cond {
		return item(ItemAvailable, id, typ, availMsg, inputs)
	}
	return item(ItemUnavailable, id, typ, unavailMsg, nil)
}
