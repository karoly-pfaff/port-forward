package workflow

import "portier/cli/sources/explain"

// explanations is the static registry of workflow step-validation code
// explanations, keyed by the workflow code constants so the strings cannot
// drift. Every workflow validation code MUST have an entry here (guarded by
// explain_internal_test.go). The Explanation type and rendering helpers live in
// the shared explain package; this package owns only the workflow explanation
// data. Explanations describe what a validation outcome means and what an
// operator can do to the workflow file — they never claim that Portier executes,
// enforces, or fixes anything (workflow planning is dry-run and offline).
var explanations = map[string]explain.Explanation{
	codeStepValid: {
		Code:     codeStepValid,
		Title:    "Workflow step is valid",
		Meaning:  "The step names a supported operation and has coherent inputs. This is an informational result — planning validates the step, it does not run it.",
		Action:   "No action needed.",
		Severity: "info",
	},
	codeStepDuplicateID: {
		Code:     codeStepDuplicateID,
		Title:    "Duplicate workflow step id",
		Meaning:  "Two steps share the same id. Step ids must be unique so steps can reference one another (for example via reportFrom) unambiguously.",
		Action:   "Give each step a unique id.",
		Severity: "error",
	},
	codeStepUnknownType: {
		Code:     codeStepUnknownType,
		Title:    "Unsupported workflow step type",
		Meaning:  "The step's type is not one of the supported operations. A workflow composes existing safe operations by reference; an unrecognised type cannot be planned.",
		Action:   "Use one of the supported step types: policy.check, policy.review, or policy.baseline.compare.",
		Severity: "error",
		Related:  []string{codeStepMissingType},
	},
	codeStepMissingID: {
		Code:     codeStepMissingID,
		Title:    "Workflow step is missing an id",
		Meaning:  "The step has no id. Every step needs a non-empty, unique id so it can be identified in the plan and referenced by later steps.",
		Action:   "Add a non-empty \"id\" to the step.",
		Severity: "error",
	},
	codeStepMissingType: {
		Code:     codeStepMissingType,
		Title:    "Workflow step is missing a type",
		Meaning:  "The step has no type, so it does not name an operation to plan.",
		Action:   "Set \"type\" to one of: policy.check, policy.review, or policy.baseline.compare.",
		Severity: "error",
		Related:  []string{codeStepUnknownType},
	},
	codeStepMissingConfigSource: {
		Code:     codeStepMissingConfigSource,
		Title:    "policy.check step is missing a config source",
		Meaning:  "A policy.check step must say what config to evaluate — either a local config file or the live runtime config — but neither was provided.",
		Action:   "Add a \"config\" file path, or set \"runtime\": true (exactly one of the two).",
		Severity: "error",
		Related:  []string{codeStepConflictingConfigSources},
	},
	codeStepConflictingConfigSources: {
		Code:     codeStepConflictingConfigSources,
		Title:    "policy.check step has conflicting config sources",
		Meaning:  "A policy.check step set both \"config\" and \"runtime\". A step evaluates exactly one config source, so these conflict.",
		Action:   "Keep only one: a \"config\" file path OR \"runtime\": true.",
		Severity: "error",
		Related:  []string{codeStepMissingConfigSource},
	},
	codeStepMissingPolicy: {
		Code:     codeStepMissingPolicy,
		Title:    "step is missing a policy file",
		Meaning:  "A policy.check or policy.review step needs a policy file to evaluate the config against, but \"policy\" was not provided.",
		Action:   "Add a \"policy\" file path to the step.",
		Severity: "error",
	},
	codeStepMissingCurrent: {
		Code:     codeStepMissingCurrent,
		Title:    "policy.review step is missing the current config",
		Meaning:  "A policy.review step compares a current config with a candidate config, but \"current\" was not provided.",
		Action:   "Add a \"current\" config file path to the step.",
		Severity: "error",
	},
	codeStepMissingCandidate: {
		Code:     codeStepMissingCandidate,
		Title:    "policy.review step is missing the candidate config",
		Meaning:  "A policy.review step evaluates a candidate config against the policy, but \"candidate\" was not provided.",
		Action:   "Add a \"candidate\" config file path to the step.",
		Severity: "error",
	},
	codeStepMissingBaseline: {
		Code:     codeStepMissingBaseline,
		Title:    "policy.baseline.compare step is missing a baseline",
		Meaning:  "A policy.baseline.compare step compares a report against an accepted baseline, but \"baseline\" was not provided.",
		Action:   "Add a \"baseline\" file path to the step.",
		Severity: "error",
	},
	codeStepMissingReportSource: {
		Code:     codeStepMissingReportSource,
		Title:    "policy.baseline.compare step is missing a report source",
		Meaning:  "A policy.baseline.compare step needs a report to compare against the baseline — either a report file or a report produced by an earlier step — but neither was provided.",
		Action:   "Add a \"report\" file path, or a \"reportFrom\" referencing an earlier step id (exactly one of the two).",
		Severity: "error",
		Related:  []string{codeStepConflictingReportSources},
	},
	codeStepConflictingReportSources: {
		Code:     codeStepConflictingReportSources,
		Title:    "policy.baseline.compare step has conflicting report sources",
		Meaning:  "A policy.baseline.compare step set both \"report\" and \"reportFrom\". A step takes exactly one report source, so these conflict.",
		Action:   "Keep only one: a \"report\" file path OR a \"reportFrom\" referencing an earlier step.",
		Severity: "error",
		Related:  []string{codeStepMissingReportSource},
	},
	codeStepUnknownReportFrom: {
		Code:     codeStepUnknownReportFrom,
		Title:    "reportFrom references an unknown step",
		Meaning:  "The step's reportFrom names a step id that does not exist anywhere in the workflow.",
		Action:   "Fix reportFrom to reference an existing earlier step id, or provide a \"report\" file instead.",
		Severity: "error",
		Related:  []string{codeStepFutureReportFrom},
	},
	codeStepFutureReportFrom: {
		Code:     codeStepFutureReportFrom,
		Title:    "reportFrom references a later step",
		Meaning:  "The step's reportFrom names a step that appears AFTER it in the workflow. A step can only use a report produced by a step that runs before it, so the reference must point earlier (this also keeps the workflow acyclic).",
		Action:   "Reorder the steps so the referenced step appears first, or reference an earlier step id, or provide a \"report\" file instead.",
		Severity: "error",
		Related:  []string{codeStepUnknownReportFrom},
	},
}

// Explanations returns the workflow explanation registry. The `explain` command
// merges this with other domains' registries (doctor, policy) for unified lookup
// and listing.
func Explanations() map[string]explain.Explanation {
	return explanations
}
