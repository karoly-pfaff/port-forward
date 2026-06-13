package workflow

// Workflow-run explanations: stable codes for the workflow-run-specific failure
// modes (a skipped dependency, an unreachable runtime, an unreadable/malformed
// step input) plus the helpers that select which codes to explain for a run.
// These are ADDITIVE — `workflow run --explain` only adds explanation text; it
// never changes which steps run, their status, the run result, or the exit code.
// Policy finding codes (from a failed policy.check/review/baseline.compare) are
// explained by reusing the canonical policy registry; this file only owns the
// workflow.run.* codes and the selection logic.

import (
	"portier/cli/sources/explain"
	"portier/cli/sources/policy"
)

// Stable workflow-run codes. These are operator-facing identifiers (a CLI/tool
// contract) — do not rename them casually. Each is actually emitted by a run
// failure/skip path (see run.go) and MUST have an explanation (guarded by
// run_explain_internal_test.go). They are deliberately separate from the
// workflow.step.* validation codes (those describe a plan, these describe a run).
const (
	runCodeDependencyFailed   = "workflow.run.dependency_failed"
	runCodeRuntimeUnreachable = "workflow.run.runtime_unreachable"
	runCodeInputFailed        = "workflow.run.input_failed"
)

// runExplanations is the static registry of workflow-run code explanations, keyed
// by the run code constants so the strings cannot drift. Every workflow.run.* code
// MUST have an entry (guarded by run_explain_internal_test.go). The Explanation
// type and rendering helpers live in the shared explain package; this package owns
// only the data. The explanations describe what a run failure means and what an
// operator can do — they never claim Portier enforces, applies, or fixes anything
// (workflow run is read-only).
var runExplanations = map[string]explain.Explanation{
	runCodeDependencyFailed: {
		Code:     runCodeDependencyFailed,
		Title:    "Workflow step skipped: dependency produced no report",
		Meaning:  "A policy.baseline.compare step used \"reportFrom\" to consume the report of an earlier step, but that step failed or produced no policy report — so there was nothing to compare against and this step was skipped. A skipped step fails the run.",
		Action:   "Fix the earlier step so it produces a policy report (for example, resolve its input or runtime error), or give this step a \"report\" file instead of \"reportFrom\".",
		Severity: "error",
	},
	runCodeRuntimeUnreachable: {
		Code:     runCodeRuntimeUnreachable,
		Title:    "Runtime unreachable during workflow run",
		Meaning:  "A policy.check step set \"runtime\": true, but Portier could not reach the management runtime to read its config (read-only). The step could not be evaluated and the run exits 3, matching 'policy check --runtime'.",
		Action:   "Start the Portier runtime (or check --url/--host/--port and that the service is listening), then re-run. Use a local \"config\" file instead of \"runtime\" to evaluate offline.",
		Severity: "error",
	},
	runCodeInputFailed: {
		Code:     runCodeInputFailed,
		Title:    "Workflow step input could not be read",
		Meaning:  "A step referenced a config, policy, baseline, or report file (or the runtime config) that could not be read or parsed during the run, so the step failed. Unlike planning, 'workflow run' opens these files while executing.",
		Action:   "Check that the referenced file exists, is readable, and is valid JSON of the expected kind, then re-run. 'portier workflow plan --file <file>' validates the workflow structure without opening referenced files.",
		Severity: "error",
	},
}

// RunExplanations returns the workflow-run explanation registry (the
// workflow.run.* codes emitted by `workflow run`). It is kept separate from the
// workflow step-validation registry; Explanations() merges both so the `explain`
// command lists and looks up every workflow code.
func RunExplanations() map[string]explain.Explanation {
	return runExplanations
}

// runExplainRegistry is the registry used to render a run's explanations: the
// workflow-run codes plus the policy finding codes (the only codes a run report
// can carry — a failed policy.check/review/baseline.compare surfaces policy
// codes). Doctor codes never appear in a run, so they are not merged in.
func runExplainRegistry() map[string]explain.Explanation {
	return explain.Merge(runExplanations, policy.Explanations())
}

// ExplanationCodesForRun returns the codes worth explaining for a run, in step
// order: a per-step workflow-run failure/skip code (input / runtime / dependency)
// and the policy finding codes of failed policy.check/review/baseline.compare
// steps. Passed steps contribute nothing (a passed step needs no explanation).
// Duplicates are kept — explain.ForReport deduplicates them via its map.
func ExplanationCodesForRun(run WorkflowRun) []string {
	var codes []string
	for _, s := range run.Steps {
		if s.Status == runStatusPassed {
			continue
		}
		codes = append(codes, s.explainCodes...)
	}
	return codes
}

// errorFindingCodes returns the codes of the error-severity (violation) findings
// in a policy report, in order. A failing policy report carries only violations,
// but filtering on severity keeps the intent explicit and excludes the info
// "policy.valid" marker.
func errorFindingCodes(rep policy.Report) []string {
	var codes []string
	for _, f := range rep.Findings {
		if f.Severity == policy.Error {
			codes = append(codes, f.Code)
		}
	}
	return codes
}

// newFindingCodes returns the codes of the NEW findings of a baseline comparison,
// in order — the findings that failed the compare step.
func newFindingCodes(cmp policy.CompareReport) []string {
	codes := make([]string, 0, len(cmp.New))
	for _, f := range cmp.New {
		codes = append(codes, f.Code)
	}
	return codes
}

// dedupeStrings returns in with later duplicates removed, preserving first-seen
// order. Used to avoid rendering the same inline explanation twice under one step.
func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
