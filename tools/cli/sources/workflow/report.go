package workflow

// Workflow support report: packages an EXISTING workflow plan/run JSON report
// (from `workflow plan --json --out` / `workflow run --json --out`) into a small,
// local, offline diagnostic bundle for manual review or AI handoff. It is strictly
// a packaging step over an already-produced report — it NEVER executes a workflow,
// contacts the runtime, runs a shell command, reads the config/policy/baseline/
// report files a step refers to, collects logs/env/process data, mutates the input,
// or uploads anything. It only parses the provided report and re-derives explanation
// metadata from the canonical registry.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"portier/cli/sources/explain"
	"portier/cli/sources/output"
	"portier/cli/sources/policy"
)

// supportReportSchemaVersion is the schema version of the report bundle manifest.
const supportReportSchemaVersion = 1

// Report kinds the support report understands.
const (
	ReportKindRun  = "workflow-run"
	ReportKindPlan = "workflow-plan"
)

// SupportReport is the normalized, safe view of an input workflow report. It is
// the `report.json` payload: step identity + status + message + the explainable
// codes for each step — deliberately NOT a verbatim copy. It drops each run step's
// embedded policy/compare report (which can carry rule host/port detail) so the
// bundle stays compact and unambiguously free of config payloads; the codes needed
// to explain a step are preserved instead.
type SupportReport struct {
	Kind     string              `json:"kind"`
	Workflow string              `json:"workflow"`
	Result   string              `json:"result"`
	Summary  map[string]int      `json:"summary,omitempty"`
	Steps    []SupportReportStep `json:"steps"`

	// inputCodes are codes carried by the input report's own `explanations` map
	// (present only when the report was produced with --explain). They feed the
	// bundle's global explanations set but are not attributed to a single step.
	// Unexported: not part of the report.json contract.
	inputCodes []string
}

// SupportReportStep is one normalized step in a support report.
type SupportReportStep struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Status   string   `json:"status"`
	Code     string   `json:"code,omitempty"`     // plan: the step's validation code
	ExitCode *int     `json:"exitCode,omitempty"` // run: the step's exit code
	Message  string   `json:"message,omitempty"`
	Codes    []string `json:"codes,omitempty"` // explainable codes for this step
}

// SupportReportManifest is the bundle's manifest.json. It carries only metadata —
// no secrets, env, process, or log data.
type SupportReportManifest struct {
	SchemaVersion int      `json:"schemaVersion"`
	Type          string   `json:"type"`
	CreatedAt     string   `json:"createdAt"`
	Source        string   `json:"source"`
	Workflow      string   `json:"workflow"`
	Result        string   `json:"result"`
	Files         []string `json:"files"`
}

// rawReport is the lenient on-disk shape of a workflow plan/run report. Unknown
// fields are tolerated (a report may carry additional fields like inputs/dependsOn
// for plans or an embedded report for runs); detection uses the fields below.
type rawReport struct {
	SchemaVersion *int                       `json:"schemaVersion"` // plan only
	Name          string                     `json:"name"`          // plan
	Workflow      string                     `json:"workflow"`      // run
	Result        string                     `json:"result"`
	Steps         []rawReportStep            `json:"steps"`
	Summary       map[string]int             `json:"summary"`
	Explanations  map[string]json.RawMessage `json:"explanations"`
}

// rawReportStep is the lenient on-disk shape of one step (plan or run).
type rawReportStep struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Status   string          `json:"status"`
	Code     string          `json:"code"`     // plan
	ExitCode *int            `json:"exitCode"` // run
	Message  string          `json:"message"`
	Report   json.RawMessage `json:"report"` // run: embedded policy/compare report
}

// embeddedReport is the subset of a run step's embedded report the packager reads:
// policy findings (policy.check / policy.review) and baseline-compare new findings.
// It is parsed from the existing report JSON only — never from a fresh evaluation.
type embeddedReport struct {
	Findings []struct {
		Code     string `json:"code"`
		Severity string `json:"severity"`
	} `json:"findings"`
	New []struct {
		Code string `json:"code"`
	} `json:"new"`
}

// ParseSupportReport parses and detects an existing workflow plan/run report,
// returning the normalized support view. It never reads any file a step refers to
// and never evaluates anything — it only decodes the provided report JSON. A
// report that is not a recognizable workflow plan/run report is rejected.
func ParseSupportReport(data []byte) (SupportReport, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return SupportReport{}, fmt.Errorf("report file is empty")
	}

	var raw rawReport
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return SupportReport{}, fmt.Errorf("not a valid workflow report: %w", err)
	}
	if len(raw.Steps) == 0 {
		return SupportReport{}, fmt.Errorf("not a workflow report (no steps)")
	}

	kind, err := detectKind(raw)
	if err != nil {
		return SupportReport{}, err
	}

	name := raw.Workflow
	if kind == ReportKindPlan {
		name = raw.Name
	}

	steps := make([]SupportReportStep, len(raw.Steps))
	for i, s := range raw.Steps {
		steps[i] = normalizeStep(kind, s)
	}

	return SupportReport{
		Kind:       kind,
		Workflow:   name,
		Result:     raw.Result,
		Summary:    raw.Summary,
		Steps:      steps,
		inputCodes: sortedKeys(raw.Explanations),
	}, nil
}

// detectKind classifies a report as a workflow plan or run from its result label
// and schemaVersion. A plan carries schemaVersion and a valid/invalid result; a
// run carries no schemaVersion and a passed/failed result. Anything else (e.g. a
// bare policy report) is unsupported.
func detectKind(raw rawReport) (string, error) {
	switch {
	case raw.SchemaVersion != nil && (raw.Result == statusValid || raw.Result == statusInvalid):
		return ReportKindPlan, nil
	case raw.SchemaVersion == nil && (raw.Result == runResultPassed || raw.Result == runResultFailed):
		return ReportKindRun, nil
	default:
		return "", fmt.Errorf("unsupported report kind (expected a workflow plan or run report)")
	}
}

// normalizeStep builds the normalized step + its explainable codes. Plan invalid
// steps carry their validation code; run skipped steps map to dependency_failed;
// run failed steps surface the policy finding codes embedded in their report. The
// embedded report is read for codes only and then dropped from the bundle.
func normalizeStep(kind string, s rawReportStep) SupportReportStep {
	step := SupportReportStep{
		ID:      s.ID,
		Type:    s.Type,
		Status:  s.Status,
		Code:    s.Code,
		Message: s.Message,
	}
	if kind == ReportKindRun {
		step.ExitCode = s.ExitCode
		// Plan-only "code" is not meaningful for a run step.
		step.Code = ""
	}

	var codes []string
	switch kind {
	case ReportKindPlan:
		if s.Status == statusInvalid && s.Code != "" {
			codes = append(codes, s.Code)
		}
	case ReportKindRun:
		switch s.Status {
		case runStatusSkipped:
			codes = append(codes, runCodeDependencyFailed)
		case runStatusFailed:
			codes = append(codes, embeddedFindingCodes(s.Report)...)
		}
	}
	step.Codes = dedupeStrings(codes)
	if len(step.Codes) == 0 {
		step.Codes = nil // keep report.json compact (omitempty)
	}
	return step
}

// embeddedFindingCodes extracts the policy finding codes from a run step's embedded
// report: error-severity findings (policy.check / policy.review) and baseline new
// findings. It reads the existing report JSON only.
func embeddedFindingCodes(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var rep embeddedReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return nil
	}
	var codes []string
	for _, f := range rep.Findings {
		if f.Severity == string(policy.Error) && f.Code != "" {
			codes = append(codes, f.Code)
		}
	}
	for _, n := range rep.New {
		if n.Code != "" {
			codes = append(codes, n.Code)
		}
	}
	return codes
}

// sortedKeys returns the sorted, deduplicated keys of a map (used for the input
// report's explanations map).
func sortedKeys(m map[string]json.RawMessage) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// reportExplainRegistry is the registry used to explain a support report's codes:
// the workflow step + run codes plus the policy finding codes — the only families
// a workflow report can carry. Doctor codes never appear.
func reportExplainRegistry() map[string]explain.Explanation {
	return explain.Merge(explanations, runExplanations, policy.Explanations())
}

// ExplanationCodes returns the union of every step's explainable codes and the
// input report's own explanation-map codes, in step order then input order
// (duplicates kept — explain.ForReport deduplicates via its map).
func (r SupportReport) ExplanationCodes() []string {
	var codes []string
	for _, s := range r.Steps {
		codes = append(codes, s.Codes...)
	}
	codes = append(codes, r.inputCodes...)
	return codes
}

// Explanations returns the canonical explanations for the report's codes,
// deduplicated (unknown codes are safely omitted). An empty map is returned when
// there are no explainable codes (the bundle writes it as `{}` for a stable shape).
func (r SupportReport) Explanations() map[string]explain.Explanation {
	return explain.ForReport(reportExplainRegistry(), r.ExplanationCodes())
}

// reportStatusTag returns the fixed-width ASCII tag for any plan/run step status.
func reportStatusTag(status string) string {
	switch status {
	case statusValid:
		return "[VALID]"
	case statusInvalid:
		return "[INVALID]"
	case runStatusPassed:
		return "[PASSED]"
	case runStatusSkipped:
		return "[SKIPPED]"
	case runStatusFailed:
		return "[FAILED]"
	default:
		return "[" + status + "]"
	}
}

// WriteSupportSummary renders the human-readable summary.txt content: the source,
// workflow, result, the step list, the explained codes, and a fixed safety block
// documenting exactly what was and was not collected.
func WriteSupportSummary(r SupportReport, w io.Writer) {
	fmt.Fprintln(w, "Portier Workflow Report")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Source: %s\n", r.Kind)
	name := r.Workflow
	if strings.TrimSpace(name) == "" {
		name = "(unnamed)"
	}
	fmt.Fprintf(w, "Workflow: %s\n", name)
	fmt.Fprintf(w, "Result: %s\n", r.Result)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Steps:")
	for _, s := range r.Steps {
		typeLabel := s.Type
		if typeLabel == "" {
			typeLabel = "(no type)"
		}
		label := s.ID
		if label == "" {
			label = "(no id)"
		}
		fmt.Fprintf(w, "- %s %s (%s)\n", reportStatusTag(s.Status), label, typeLabel)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Explanations:")
	codes := explain.SortedCodes(r.Explanations())
	if len(codes) == 0 {
		fmt.Fprintln(w, "- none")
	} else {
		for _, c := range codes {
			fmt.Fprintf(w, "- %s\n", c)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Safety:")
	fmt.Fprintln(w, "- Generated from an existing report file.")
	fmt.Fprintln(w, "- No runtime contact.")
	fmt.Fprintln(w, "- No workflow execution.")
	fmt.Fprintln(w, "- No config/policy/baseline/report input files were read.")
	fmt.Fprintln(w, "- No logs, environment variables, or process data were collected.")
}

// WriteSupportReport writes the support-report bundle into dir: summary.txt,
// report.json (the normalized report), explanations.json (canonical explanations
// for the emitted codes, `{}` when none), and manifest.json (written last so its
// presence marks a complete bundle). dir must already exist and be writable (the
// caller prepares it). now stamps the manifest createdAt (injected for tests). It
// reads nothing and contacts nothing — it only serializes the parsed report.
func WriteSupportReport(dir string, r SupportReport, now time.Time) (SupportReportManifest, error) {
	var summary bytes.Buffer
	WriteSupportSummary(r, &summary)

	files := []bundleFile{
		{"summary.txt", func(p string) error { return os.WriteFile(p, summary.Bytes(), 0o644) }},
		{"report.json", func(p string) error { return output.WritePrettyJSON(p, r) }},
		{"explanations.json", func(p string) error { return output.WritePrettyJSON(p, r.Explanations()) }},
	}

	written := make([]string, 0, len(files))
	for _, f := range files {
		if err := f.write(filepath.Join(dir, f.name)); err != nil {
			return SupportReportManifest{}, fmt.Errorf("writing %s: %w", f.name, err)
		}
		written = append(written, f.name)
	}

	manifest := SupportReportManifest{
		SchemaVersion: supportReportSchemaVersion,
		Type:          "workflow-report",
		CreatedAt:     now.UTC().Format(time.RFC3339),
		Source:        r.Kind,
		Workflow:      r.Workflow,
		Result:        r.Result,
		Files:         written,
	}
	if err := output.WritePrettyJSON(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		return SupportReportManifest{}, fmt.Errorf("writing manifest.json: %w", err)
	}
	return manifest, nil
}

// bundleFile is one file to write into the report directory.
type bundleFile struct {
	name  string
	write func(path string) error
}
