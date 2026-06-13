package workflow

// Workflow execution (read-only): runs a VALID workflow's steps in order by
// calling the existing safe domain helpers directly (policy evaluate / review /
// baseline compare), and collects a deterministic execution report. It is
// strictly read-only — it NEVER mutates runtime/config/policy/baseline/report
// files, applies/imports configs, enforces a policy, runs a shell command, or
// executes the runbook display text. The only runtime contact is the injected
// read-only runtime-config fetch (for a policy.check runtime step); it is injected
// as a function so this package stays free of client/runtime coupling.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"portier/cli/sources/config"
	"portier/cli/sources/explain"
	"portier/cli/sources/output"
	"portier/cli/sources/policy"
)

// Run step/result status values.
const (
	runStatusPassed  = "passed"
	runStatusFailed  = "failed"
	runStatusSkipped = "skipped"
	runResultPassed  = "passed"
	runResultFailed  = "failed"
)

// ErrRuntimeUnreachable marks a runtime-config fetch failure that must map to the
// runtime-unreachable exit code (3), matching `policy check --runtime`. The
// command layer (which knows about the API client) wraps a connection error with
// this sentinel; execution detects it via errors.Is and never needs to import the
// client package.
var ErrRuntimeUnreachable = errors.New("runtime unreachable")

// RunDeps are the injected side-effecting dependencies workflow execution needs.
// Keeping them injected lets the workflow package stay free of os/client/runtime
// coupling — the command layer wires the real implementations (os.ReadFile and a
// read-only runtime-config fetch). ReadFile is used for the local files a step
// references (config/policy/baseline/report); RuntimeRules fetches the live
// runtime config as evaluator rules for a policy.check runtime step.
type RunDeps struct {
	ReadFile     func(path string) ([]byte, error)
	RuntimeRules func() ([]config.Rule, error)
}

// WorkflowRunStep is the execution result for one step: its id/type, a
// passed/failed/skipped status, the per-step exit code, a human message, and an
// optional compact per-step report (the policy/review/compare result it produced;
// omitted for skipped or input-error steps). The report intentionally carries
// only what the existing policy reports already include — no raw configs, no
// secrets.
type WorkflowRunStep struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	ExitCode int    `json:"exitCode"`
	Message  string `json:"message"`
	Report   any    `json:"report,omitempty"`

	// explainCodes are the codes worth explaining for this step under --explain: a
	// workflow.run.* failure/skip code and/or the policy finding codes of a failed
	// step. Unexported: it drives --explain rendering only and is NOT part of the
	// JSON contract (the additive top-level "explanations" map carries the data).
	explainCodes []string
}

// WorkflowRunSummary counts the steps in a run by status.
type WorkflowRunSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// WorkflowRun is the deterministic result of executing a workflow. It is the JSON
// contract emitted by `workflow run` under --json, kept separate from the Plan,
// Runbook, and policy/doctor report models. The result is "passed" only when
// every step passed; any failed OR skipped step makes it "failed".
type WorkflowRun struct {
	Workflow string             `json:"workflow"`
	Steps    []WorkflowRunStep  `json:"steps"`
	Summary  WorkflowRunSummary `json:"summary"`
	Result   string             `json:"result"`

	// runtimeUnreachable records that a runtime step could not reach the runtime
	// (an operational failure that maps to exit 3, not 1). Unexported: it drives
	// the exit code only and is not part of the JSON contract.
	runtimeUnreachable bool
}

// stepOutcome is the internal result of executing one step before it is folded
// into a WorkflowRunStep + the run tally.
type stepOutcome struct {
	status       string
	exitCode     int
	message      string
	report       any            // the per-step Report field (nil when none)
	snapshot     *policy.Report // produced report for a later reportFrom (nil when none)
	unreachable  bool           // runtime was unreachable
	explainCodes []string       // codes to explain for this step under --explain
}

// Run executes a valid workflow's steps in order and returns a deterministic run
// report. It MUST be called only after BuildPlan has confirmed the workflow is
// valid (the caller enforces this). It reuses the existing policy/config helpers
// for each step and never mutates anything. Steps run in file order; a
// policy.baseline.compare step using reportFrom consumes the IN-MEMORY report
// produced by the referenced earlier step (no intermediate file is read or
// written). If a referenced step produced no usable report, the dependent step is
// SKIPPED (which fails the run).
func Run(f File, deps RunDeps) WorkflowRun {
	run := WorkflowRun{Workflow: f.Name, Steps: make([]WorkflowRunStep, len(f.Steps))}
	reports := map[string]*policy.Report{} // producing-step reports, for reportFrom

	for i, s := range f.Steps {
		out := executeStep(s, deps, reports)
		run.Steps[i] = WorkflowRunStep{
			ID:           s.ID,
			Type:         s.Type,
			Status:       out.status,
			ExitCode:     out.exitCode,
			Message:      out.message,
			Report:       out.report,
			explainCodes: out.explainCodes,
		}
		if out.snapshot != nil {
			reports[s.ID] = out.snapshot
		}
		if out.unreachable {
			run.runtimeUnreachable = true
		}
		switch out.status {
		case runStatusPassed:
			run.Summary.Passed++
		case runStatusSkipped:
			run.Summary.Skipped++
		default:
			run.Summary.Failed++
		}
	}

	run.Summary.Total = len(run.Steps)
	run.Result = runResultPassed
	if run.Summary.Failed > 0 || run.Summary.Skipped > 0 {
		run.Result = runResultFailed
	}
	return run
}

// executeStep dispatches one validated step to its read-only executor.
func executeStep(s Step, deps RunDeps, reports map[string]*policy.Report) stepOutcome {
	switch s.Type {
	case stepPolicyCheck:
		return execPolicyCheck(s, deps)
	case stepPolicyReview:
		return execPolicyReview(s, deps)
	case stepPolicyBaselineCompare:
		return execBaselineCompare(s, deps, reports)
	default:
		// Unreachable for a valid plan (every step type is validated); defensive.
		return failOutcome(fmt.Sprintf("Unsupported step type %q.", s.Type))
	}
}

// execPolicyCheck evaluates a local config file (offline) or the live runtime
// config (read-only, injected) against a policy file. It reads the referenced
// files HERE (during run), unlike plan/runbook.
func execPolicyCheck(s Step, deps RunDeps) stepOutcome {
	var rules []config.Rule
	if s.Runtime {
		r, err := deps.RuntimeRules()
		if err != nil {
			if errors.Is(err, ErrRuntimeUnreachable) {
				out := failOutcome(fmt.Sprintf("Runtime config could not be read: %v", err))
				out.unreachable = true
				out.explainCodes = []string{runCodeRuntimeUnreachable}
				return out
			}
			return failInputOutcome(fmt.Sprintf("Runtime config could not be read: %v", err))
		}
		rules = r
	} else {
		data, err := deps.ReadFile(s.Config)
		if err != nil {
			return failInputOutcome(fmt.Sprintf("Error reading config %s: %v", s.Config, err))
		}
		parsed, perr := config.ParseLocal(data)
		if perr != nil {
			return failInputOutcome(fmt.Sprintf("Invalid config %s: %v", s.Config, perr))
		}
		rules = parsed
	}

	pol, err := loadPolicy(s.Policy, deps)
	if err != nil {
		return failInputOutcome(err.Error())
	}

	rep := policy.Evaluate(rules, pol)
	return policyReportOutcome(rep, "Policy check")
}

// execPolicyReview compares current vs candidate config summaries and evaluates
// the candidate against a policy (reusing policy.BuildReview). Fully offline.
func execPolicyReview(s Step, deps RunDeps) stepOutcome {
	current, err := loadRules(s.Current, "current config", deps)
	if err != nil {
		return failInputOutcome(err.Error())
	}
	candidate, err := loadRules(s.Candidate, "candidate config", deps)
	if err != nil {
		return failInputOutcome(err.Error())
	}
	pol, err := loadPolicy(s.Policy, deps)
	if err != nil {
		return failInputOutcome(err.Error())
	}

	review := policy.BuildReview(current, candidate, pol)
	out := stepOutcome{report: review, snapshot: &review.Report}
	if policy.ExitCode(review.Report) == 0 {
		out.status = runStatusPassed
		out.exitCode = 0
		out.message = "Policy review passed (candidate has no violations)."
	} else {
		n := review.Report.Summary.Error
		out.status = runStatusFailed
		out.exitCode = 1
		out.message = fmt.Sprintf("Policy review found %d %s in the candidate.", n, output.PluralWord(n, "violation", "violations"))
		out.explainCodes = errorFindingCodes(review.Report)
	}
	return out
}

// execBaselineCompare compares a baseline against a policy report — either a
// report file or the in-memory report produced by an earlier step (reportFrom).
// A reportFrom whose producing step yielded no report SKIPS this step (it never
// reads the baseline in that case).
func execBaselineCompare(s Step, deps RunDeps, reports map[string]*policy.Report) stepOutcome {
	// Resolve the report source first.
	var snap policy.ReportSnapshot
	if s.Report != "" {
		data, err := deps.ReadFile(s.Report)
		if err != nil {
			return failInputOutcome(fmt.Sprintf("Error reading report %s: %v", s.Report, err))
		}
		sn, serr := policy.ParseReportSnapshot(data)
		if serr != nil {
			return failInputOutcome(fmt.Sprintf("Invalid report %s: %v", s.Report, serr))
		}
		snap = sn
	} else {
		producing := reports[s.ReportFrom]
		if producing == nil {
			return skipDependencyOutcome(fmt.Sprintf("Skipped: depends on step %q, which produced no policy report.", s.ReportFrom))
		}
		sn, err := snapshotFromReport(*producing)
		if err != nil {
			// Unreachable in practice (a concrete report always marshals).
			return failInputOutcome(fmt.Sprintf("Could not use the report from step %q: %v", s.ReportFrom, err))
		}
		snap = sn
	}

	data, err := deps.ReadFile(s.Baseline)
	if err != nil {
		return failInputOutcome(fmt.Sprintf("Error reading baseline %s: %v", s.Baseline, err))
	}
	baseline, berr := policy.ParseBaseline(data)
	if berr != nil {
		return failInputOutcome(fmt.Sprintf("Invalid baseline %s: %v", s.Baseline, berr))
	}

	cmp := policy.Compare(baseline, snap)
	out := stepOutcome{report: cmp}
	if policy.CompareExitCode(cmp) == 0 {
		out.status = runStatusPassed
		out.exitCode = 0
		out.message = "No new findings compared to the baseline."
	} else {
		n := cmp.Summary.New
		out.status = runStatusFailed
		out.exitCode = 1
		out.message = fmt.Sprintf("%d new %s compared to the baseline.", n, output.PluralWord(n, "finding", "findings"))
		out.explainCodes = newFindingCodes(cmp)
	}
	return out
}

// policyReportOutcome builds the outcome for a step that evaluated a policy.Report
// (policy.check). The report is also stored as the step's snapshot for a later
// reportFrom.
func policyReportOutcome(rep policy.Report, label string) stepOutcome {
	out := stepOutcome{report: rep, snapshot: &rep}
	if policy.ExitCode(rep) == 0 {
		out.status = runStatusPassed
		out.exitCode = 0
		out.message = label + " passed (no violations)."
	} else {
		n := rep.Summary.Error
		out.status = runStatusFailed
		out.exitCode = 1
		out.message = fmt.Sprintf("%s found %d %s.", label, n, output.PluralWord(n, "violation", "violations"))
		out.explainCodes = errorFindingCodes(rep)
	}
	return out
}

// loadRules reads and parses a local config file via the injected reader. role is
// used only in the error message.
func loadRules(path, role string, deps RunDeps) ([]config.Rule, error) {
	data, err := deps.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Error reading %s %s: %v", role, path, err)
	}
	rules, perr := config.ParseLocal(data)
	if perr != nil {
		return nil, fmt.Errorf("Invalid %s %s: %v", role, path, perr)
	}
	return rules, nil
}

// loadPolicy reads and parses a policy file via the injected reader.
func loadPolicy(path string, deps RunDeps) (policy.Policy, error) {
	data, err := deps.ReadFile(path)
	if err != nil {
		return policy.Policy{}, fmt.Errorf("Error reading policy %s: %v", path, err)
	}
	pol, perr := policy.Parse(data)
	if perr != nil {
		return policy.Policy{}, fmt.Errorf("Invalid policy %s: %v", path, perr)
	}
	return pol, nil
}

// snapshotFromReport converts an in-memory policy.Report into a ReportSnapshot by
// round-tripping it through JSON. This normalises finding details to the generic
// map form ParseReportSnapshot produces, so a reportFrom report fingerprints
// identically to the same report written to a file and re-read — keeping baseline
// matching consistent regardless of source.
func snapshotFromReport(rep policy.Report) (policy.ReportSnapshot, error) {
	data, err := json.Marshal(rep)
	if err != nil {
		return policy.ReportSnapshot{}, err
	}
	return policy.ParseReportSnapshot(data)
}

// failOutcome builds a failed step outcome with no report/snapshot.
func failOutcome(message string) stepOutcome {
	return stepOutcome{status: runStatusFailed, exitCode: 1, message: message}
}

// failInputOutcome builds a failed step outcome for an unreadable/malformed step
// input (config/policy/baseline/report file, or the runtime config), tagging it
// with the workflow.run.input_failed explanation code.
func failInputOutcome(message string) stepOutcome {
	out := failOutcome(message)
	out.explainCodes = []string{runCodeInputFailed}
	return out
}

// skipOutcome builds a skipped step outcome. A skip fails the overall run, so its
// exit code is 1 (it did not succeed).
func skipOutcome(message string) stepOutcome {
	return stepOutcome{status: runStatusSkipped, exitCode: 1, message: message}
}

// skipDependencyOutcome builds a skipped step outcome for a reportFrom dependency
// that produced no usable report, tagging it with the
// workflow.run.dependency_failed explanation code.
func skipDependencyOutcome(message string) stepOutcome {
	out := skipOutcome(message)
	out.explainCodes = []string{runCodeDependencyFailed}
	return out
}

// RunExitCode maps a run to a CLI exit code: 3 when a runtime step could not reach
// the runtime (matching `policy check --runtime`), else 1 when any step failed or
// was skipped, else 0.
func RunExitCode(run WorkflowRun) int {
	if run.runtimeUnreachable {
		return 3
	}
	if run.Summary.Failed > 0 || run.Summary.Skipped > 0 {
		return 1
	}
	return 0
}

// runStatusTag returns the fixed-width ASCII tag for a run step status.
func runStatusTag(status string) string {
	switch status {
	case runStatusPassed:
		return "[PASSED]"
	case runStatusSkipped:
		return "[SKIPPED]"
	default:
		return "[FAILED]"
	}
}

// PrintRunHuman renders a workflow run as a deterministic, human-readable list of
// step results ending in a summary and Result line. When withExplain is set, each
// FAILED or SKIPPED step is followed by inline explanation blocks (code, meaning,
// next action) for its codes — a workflow.run.* failure/skip code and/or the
// policy finding codes that failed it. Passed steps are never explained (a passed
// step needs no explanation), so passed-only runs stay quiet under --explain.
func PrintRunHuman(run WorkflowRun, withExplain bool, w io.Writer) {
	fmt.Fprintln(w, "Portier Workflow Run")
	fmt.Fprintln(w)
	name := run.Workflow
	if strings.TrimSpace(name) == "" {
		name = "(unnamed)"
	}
	fmt.Fprintf(w, "Workflow: %s\n\n", name)

	reg := runExplainRegistry()
	for _, s := range run.Steps {
		label := s.ID
		typeLabel := s.Type
		if typeLabel == "" {
			typeLabel = "(no type)"
		}
		fmt.Fprintf(w, "%-9s %s  (%s)\n", runStatusTag(s.Status), label, typeLabel)
		fmt.Fprintf(w, "          %s\n", s.Message)
		if withExplain && s.Status != runStatusPassed {
			for _, code := range dedupeStrings(s.explainCodes) {
				explain.PrintInline(reg, code, w)
			}
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  %d total\n", run.Summary.Total)
	fmt.Fprintf(w, "  %d passed\n", run.Summary.Passed)
	fmt.Fprintf(w, "  %d failed\n", run.Summary.Failed)
	fmt.Fprintf(w, "  %d skipped\n", run.Summary.Skipped)
	fmt.Fprintf(w, "\nResult: %s\n", run.Result)
}

// runJSON is the JSON encoding of a workflow run plus an optional additive
// explanations map. WorkflowRun is embedded so workflow/steps/summary/result stay
// at the top level; Explanations is populated ONLY with --explain (and only for
// the codes the run emitted that need explaining). It is omitempty, so without
// --explain — and when --explain emits no explainable codes — the output is
// byte-identical to encoding the WorkflowRun directly.
type runJSON struct {
	WorkflowRun
	Explanations map[string]explain.Explanation `json:"explanations,omitempty"`
}

// EmitRun prints a run report (JSON when jsonOutput is set, otherwise human) and,
// when outPath is non-empty, also writes the exact same JSON report to that file
// (byte-identical to the --json stdout output). With explain it adds inline
// explanations (human blocks under failed/skipped steps; an additive
// `explanations` map in JSON, deduplicated by code) — WITHOUT changing the steps,
// summary, result, or exit code. It returns the run's exit code, except that a
// JSON-encode or file-write failure overrides it with 1 (an operation failure). It
// never mutates the workflow file and never contacts the runtime (the run already
// happened).
func EmitRun(run WorkflowRun, jsonOutput, explainOut bool, outPath string, stdout, stderr io.Writer) int {
	payload := runJSON{WorkflowRun: run}
	if explainOut {
		payload.Explanations = explain.ForReport(runExplainRegistry(), ExplanationCodesForRun(run))
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, payload); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		PrintRunHuman(run, explainOut, stdout)
	}

	if outPath != "" {
		if err := output.WritePrettyJSON(outPath, payload); err != nil {
			fmt.Fprintf(stderr, "Error writing %s: %v\n", outPath, err)
			return 1
		}
		if !jsonOutput {
			fmt.Fprintf(stdout, "\nReport written to %s\n", outPath)
		}
	}

	return RunExitCode(run)
}
