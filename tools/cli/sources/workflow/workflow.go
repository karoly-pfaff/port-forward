// Package workflow holds the Portier CLI's local workflow model and its
// deterministic, fully offline dry-run planner: parsing a small JSON workflow
// file into an ordered list of steps, validating the schema and step references,
// and producing a deterministic plan/report. It composes the existing safe
// Portier operations (policy check / review / baseline compare) by reference
// only — it never executes a step, never contacts the runtime, never mutates any
// file, and never enforces a policy. It contains no command/dispatch code; the
// CLI command handlers in package commands read the file and call these helpers.
package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"portier/cli/sources/output"
)

// schemaVersion is the only supported workflow file schema version.
const schemaVersion = 1

// Supported step types. These are stable, operator-facing identifiers — do not
// rename them casually (they are a CLI/tool contract). Each names an existing
// safe Portier operation that a later execution feature could run; in this slice
// they are validated only, never executed.
const (
	stepPolicyCheck           = "policy.check"
	stepPolicyReview          = "policy.review"
	stepPolicyBaselineCompare = "policy.baseline.compare"
)

// supportedStepTypes lists the step types this slice understands, in a stable
// order, for error messages.
var supportedStepTypes = []string{stepPolicyCheck, stepPolicyReview, stepPolicyBaselineCompare}

// Status labels for a planned step and the overall plan result.
const (
	statusValid   = "valid"
	statusInvalid = "invalid"
)

// File is a parsed workflow file after schema validation. It carries the raw
// fields; step-level semantic validation happens in BuildPlan so that invalid
// steps surface as plan findings (exit 1) rather than parse failures (exit 2).
type File struct {
	SchemaVersion int
	Name          string
	Steps         []Step
}

// Step is one parsed workflow step (all fields as written in the file). The
// validator interprets the subset relevant to the step's Type.
type Step struct {
	ID         string
	Type       string
	Config     string
	Runtime    bool
	Policy     string
	Current    string
	Candidate  string
	Baseline   string
	Report     string
	ReportFrom string
}

// fileRaw is the on-disk workflow JSON. SchemaVersion is a pointer so a missing
// field is distinguishable from an explicit 0. Steps is a pointer so a missing
// "steps" key is distinguishable from an explicit empty array (both are rejected,
// but with a clearer message).
type fileRaw struct {
	SchemaVersion *int       `json:"schemaVersion"`
	Name          string     `json:"name"`
	Steps         *[]stepRaw `json:"steps"`
}

// stepRaw is the on-disk shape of a single step. All recognised fields are
// declared here so DisallowUnknownFields rejects typo'd keys anywhere in the
// file. Field relevance is enforced per step type by the validator.
type stepRaw struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Config     string `json:"config"`
	Runtime    bool   `json:"runtime"`
	Policy     string `json:"policy"`
	Current    string `json:"current"`
	Candidate  string `json:"candidate"`
	Baseline   string `json:"baseline"`
	Report     string `json:"report"`
	ReportFrom string `json:"reportFrom"`
}

// Parse decodes and structurally validates a workflow file. Unknown fields are
// REJECTED (DisallowUnknownFields) so a typo'd key fails loudly rather than being
// silently ignored. The supported schemaVersion is required and must equal
// schemaVersion, and at least one step is required. These are structural errors
// (the caller maps them to exit 2); per-step semantic validation is deferred to
// BuildPlan (exit 1), so step problems appear as plan findings rather than parse
// failures. Parse never reads referenced files and never contacts the runtime.
func Parse(data []byte) (File, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return File{}, fmt.Errorf("workflow file is empty")
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var raw fileRaw
	if err := dec.Decode(&raw); err != nil {
		return File{}, fmt.Errorf("not a valid workflow file: %w", err)
	}
	if dec.More() {
		return File{}, fmt.Errorf("workflow file has trailing data after the JSON object")
	}

	if raw.SchemaVersion == nil {
		return File{}, fmt.Errorf("workflow file is missing the required \"schemaVersion\" field")
	}
	if *raw.SchemaVersion != schemaVersion {
		return File{}, fmt.Errorf("unsupported workflow schemaVersion %d (expected %d)", *raw.SchemaVersion, schemaVersion)
	}
	if raw.Steps == nil || len(*raw.Steps) == 0 {
		return File{}, fmt.Errorf("workflow file must contain at least one step")
	}

	steps := make([]Step, len(*raw.Steps))
	for i, s := range *raw.Steps {
		steps[i] = Step{
			ID:         s.ID,
			Type:       s.Type,
			Config:     s.Config,
			Runtime:    s.Runtime,
			Policy:     s.Policy,
			Current:    s.Current,
			Candidate:  s.Candidate,
			Baseline:   s.Baseline,
			Report:     s.Report,
			ReportFrom: s.ReportFrom,
		}
	}

	return File{
		SchemaVersion: *raw.SchemaVersion,
		Name:          raw.Name,
		Steps:         steps,
	}, nil
}

// StepPlan is the validated plan for one workflow step: its id/type, a
// valid/invalid status with an explanatory message, the resolved inputs the step
// would use (deterministic, map keys sorted in JSON), and the IDs of steps it
// depends on (only `reportFrom` creates a dependency in this slice).
type StepPlan struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Inputs    map[string]string `json:"inputs"`
	DependsOn []string          `json:"dependsOn"`
}

// PlanSummary counts the steps in a plan by validity.
type PlanSummary struct {
	Total   int `json:"total"`
	Valid   int `json:"valid"`
	Invalid int `json:"invalid"`
}

// Plan is the full deterministic result of planning a workflow: the schema
// version, name, per-step plans, a summary, and a valid/invalid result. It is the
// JSON contract emitted by `workflow plan` under --json. It is intentionally kept
// separate from the policy and doctor report models.
type Plan struct {
	SchemaVersion int         `json:"schemaVersion"`
	Name          string      `json:"name"`
	Steps         []StepPlan  `json:"steps"`
	Summary       PlanSummary `json:"summary"`
	Result        string      `json:"result"`
}

// BuildPlan validates each step of a parsed workflow and produces a deterministic
// plan. It is pure and offline: it inspects only the parsed file contents, never
// reads the referenced files, never contacts the runtime, and never executes a
// step. Step problems become invalid step plans with explanatory messages (the
// caller maps an invalid plan to exit 1) rather than aborting. Ordering is fixed:
// steps appear in file order. The only inter-step dependency is `reportFrom`,
// which must reference an EARLIER step — so the dependency graph is inherently
// acyclic (a step can only depend on one that precedes it) and no cycle detection
// is needed.
func BuildPlan(f File) Plan {
	steps := make([]StepPlan, len(f.Steps))

	// Track IDs seen so far (earlier steps) and all IDs, to distinguish a
	// reportFrom that points at a later/self step from one that points nowhere.
	earlierIDs := map[string]bool{}
	allIDs := map[string]int{} // id → count, to flag duplicates
	for _, s := range f.Steps {
		if s.ID != "" {
			allIDs[s.ID]++
		}
	}

	seenID := map[string]bool{}
	for i, s := range f.Steps {
		sp := validateStep(s, i, seenID, earlierIDs, allIDs)
		steps[i] = sp
		if s.ID != "" {
			seenID[s.ID] = true
			earlierIDs[s.ID] = true
		}
	}

	summary := PlanSummary{Total: len(steps)}
	for _, sp := range steps {
		if sp.Status == statusValid {
			summary.Valid++
		} else {
			summary.Invalid++
		}
	}
	result := statusValid
	if summary.Invalid > 0 {
		result = statusInvalid
	}

	return Plan{
		SchemaVersion: f.SchemaVersion,
		Name:          f.Name,
		Steps:         steps,
		Summary:       summary,
		Result:        result,
	}
}

// validateStep validates a single step against the common rules (id present,
// id unique, type supported) and the per-type rules, returning its plan. inputs
// and dependsOn are populated even for some invalid steps so the plan stays
// informative; a nil dependsOn is normalised to an empty slice for a stable JSON
// array.
func validateStep(s Step, index int, seenID, earlierIDs map[string]bool, allIDs map[string]int) StepPlan {
	sp := StepPlan{
		ID:        s.ID,
		Type:      s.Type,
		Status:    statusValid,
		Message:   "Step is valid.",
		Inputs:    map[string]string{},
		DependsOn: []string{},
	}

	invalid := func(msg string) StepPlan {
		sp.Status = statusInvalid
		sp.Message = msg
		return sp
	}

	if s.ID == "" {
		return invalid("Step is missing a required id.")
	}
	if seenID[s.ID] {
		return invalid(fmt.Sprintf("Duplicate step id %q; step ids must be unique.", s.ID))
	}

	switch s.Type {
	case stepPolicyCheck:
		return validatePolicyCheck(s, sp, invalid)
	case stepPolicyReview:
		return validatePolicyReview(s, sp, invalid)
	case stepPolicyBaselineCompare:
		return validateBaselineCompare(s, sp, invalid, earlierIDs, allIDs)
	case "":
		return invalid("Step is missing a required type.")
	default:
		return invalid(fmt.Sprintf("Unsupported step type %q; supported types: %s.", s.Type, strings.Join(supportedStepTypes, ", ")))
	}
}

// validatePolicyCheck enforces the policy.check rules: exactly one config source
// (--config file OR --runtime marker) and a required policy file.
func validatePolicyCheck(s Step, sp StepPlan, invalid func(string) StepPlan) StepPlan {
	hasConfig := s.Config != ""
	if hasConfig && s.Runtime {
		return invalid("A policy.check step cannot set both \"config\" and \"runtime\".")
	}
	if !hasConfig && !s.Runtime {
		return invalid("A policy.check step requires one of \"config\" or \"runtime\".")
	}
	if s.Policy == "" {
		return invalid("A policy.check step requires a \"policy\" file.")
	}
	if hasConfig {
		sp.Inputs["config"] = s.Config
	} else {
		sp.Inputs["runtime"] = "true"
	}
	sp.Inputs["policy"] = s.Policy
	return sp
}

// validatePolicyReview enforces the policy.review rules: current, candidate, and
// policy are all required.
func validatePolicyReview(s Step, sp StepPlan, invalid func(string) StepPlan) StepPlan {
	if s.Current == "" {
		return invalid("A policy.review step requires a \"current\" config file.")
	}
	if s.Candidate == "" {
		return invalid("A policy.review step requires a \"candidate\" config file.")
	}
	if s.Policy == "" {
		return invalid("A policy.review step requires a \"policy\" file.")
	}
	sp.Inputs["current"] = s.Current
	sp.Inputs["candidate"] = s.Candidate
	sp.Inputs["policy"] = s.Policy
	return sp
}

// validateBaselineCompare enforces the policy.baseline.compare rules: a required
// baseline, and exactly one report source (a "report" file OR a "reportFrom"
// reference to an EARLIER step). A reportFrom dependency is recorded in DependsOn.
func validateBaselineCompare(s Step, sp StepPlan, invalid func(string) StepPlan, earlierIDs map[string]bool, allIDs map[string]int) StepPlan {
	if s.Baseline == "" {
		return invalid("A policy.baseline.compare step requires a \"baseline\" file.")
	}
	hasReport := s.Report != ""
	hasReportFrom := s.ReportFrom != ""
	if hasReport && hasReportFrom {
		return invalid("A policy.baseline.compare step cannot set both \"report\" and \"reportFrom\".")
	}
	if !hasReport && !hasReportFrom {
		return invalid("A policy.baseline.compare step requires one of \"report\" or \"reportFrom\".")
	}

	sp.Inputs["baseline"] = s.Baseline
	if hasReport {
		sp.Inputs["report"] = s.Report
		return sp
	}

	// reportFrom must reference an earlier step.
	if !earlierIDs[s.ReportFrom] {
		if allIDs[s.ReportFrom] > 0 {
			return invalid(fmt.Sprintf("\"reportFrom\" references step %q, which does not appear before this step.", s.ReportFrom))
		}
		return invalid(fmt.Sprintf("\"reportFrom\" references unknown step id %q.", s.ReportFrom))
	}
	sp.Inputs["reportFrom"] = s.ReportFrom
	sp.DependsOn = []string{s.ReportFrom}
	return sp
}

// PlanExitCode maps a plan to a CLI exit code: 1 when the plan is invalid (one or
// more invalid steps), otherwise 0.
func PlanExitCode(p Plan) int {
	if p.Result == statusInvalid {
		return 1
	}
	return 0
}

// inputOrder is the canonical display order for a step's inputs in human output
// (the JSON map is key-sorted by the encoder regardless). Keys not present in a
// given step are skipped.
var inputOrder = []string{"config", "runtime", "current", "candidate", "policy", "baseline", "report", "reportFrom"}

// inputLabels maps an input key to its human-readable "Uses …" description.
var inputLabels = map[string]string{
	"config":     "Uses config file",
	"runtime":    "Uses runtime config",
	"current":    "Uses current config",
	"candidate":  "Uses candidate config",
	"policy":     "Uses policy file",
	"baseline":   "Uses baseline",
	"report":     "Uses report file",
	"reportFrom": "Uses report from",
}

// statusTag returns the fixed-width ASCII tag for a step status, matching the
// CLI's symbol-free human output style.
func statusTag(status string) string {
	if status == statusValid {
		return "[VALID]"
	}
	return "[INVALID]"
}

// PrintHuman renders a workflow plan in deterministic human-readable form. Valid
// steps list the inputs they would use; invalid steps show the reason. The output
// ends with a summary and a Result: valid/invalid line.
func PrintHuman(p Plan, w io.Writer) {
	fmt.Fprintln(w, "Portier Workflow Plan")
	fmt.Fprintln(w)
	name := p.Name
	if strings.TrimSpace(name) == "" {
		name = "(unnamed)"
	}
	fmt.Fprintf(w, "Workflow: %s\n\n", name)

	for i, sp := range p.Steps {
		label := sp.ID
		if label == "" {
			label = fmt.Sprintf("(step %d)", i+1)
		}
		typeLabel := sp.Type
		if typeLabel == "" {
			typeLabel = "(no type)"
		}
		fmt.Fprintf(w, "%-9s %s  (%s)\n", statusTag(sp.Status), label, typeLabel)
		if sp.Status == statusValid {
			for _, key := range inputOrder {
				val, ok := sp.Inputs[key]
				if !ok {
					continue
				}
				lbl := inputLabels[key]
				if key == "runtime" {
					fmt.Fprintf(w, "          %s\n", lbl)
				} else {
					fmt.Fprintf(w, "          %s: %s\n", lbl, val)
				}
			}
		} else {
			fmt.Fprintf(w, "          %s\n", sp.Message)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  %d total\n", p.Summary.Total)
	fmt.Fprintf(w, "  %d valid\n", p.Summary.Valid)
	fmt.Fprintf(w, "  %d invalid\n", p.Summary.Invalid)
	fmt.Fprintf(w, "\nResult: %s\n", p.Result)
}

// EmitOptions groups the presentation flags for a workflow plan. They affect ONLY
// how the plan is rendered/exported — never the steps, summary, result, or exit
// code. OutPath, when non-empty, also writes the JSON plan to that file.
type EmitOptions struct {
	JSON    bool
	OutPath string
}

// Emit prints a workflow plan (JSON when opts.JSON is set, otherwise human) and,
// when opts.OutPath is non-empty, also writes the exact same JSON plan to that
// file (byte-identical to the --json stdout output). It returns the plan's exit
// code, except that a JSON-encode or file-write failure overrides it with 1 (an
// operation failure, not a plan finding). It never mutates the workflow file and
// never contacts the runtime.
func Emit(p Plan, opts EmitOptions, stdout, stderr io.Writer) int {
	if opts.JSON {
		if err := output.PrintJSON(stdout, p); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		PrintHuman(p, stdout)
	}

	if opts.OutPath != "" {
		if err := output.WritePrettyJSON(opts.OutPath, p); err != nil {
			fmt.Fprintf(stderr, "Error writing %s: %v\n", opts.OutPath, err)
			return 1
		}
		// In human mode, confirm the export on stdout (policy/doctor --out do the
		// same). In JSON mode stdout must stay valid JSON, so stay silent.
		if !opts.JSON {
			fmt.Fprintf(stdout, "\nReport written to %s\n", opts.OutPath)
		}
	}

	return PlanExitCode(p)
}
