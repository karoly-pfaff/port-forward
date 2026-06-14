package core

// Offline artifact analysis for the replay tool. `analyze` inspects a saved
// Portier workflow artifact and produces deterministic analysis derived ONLY from
// the artifact's own contents — step/run tallies, emitted-code distributions,
// findings, and plain-language insights.
//
// Like the rest of the replay tool it is strictly offline and read-only: it parses
// only the supplied `--from` JSON file (or, for a support-report bundle directory,
// the bundle's own manifest.json / report.json / explanations.json) and NEVER
// executes workflows, contacts the runtime, reads the config/policy/baseline/report
// files an artifact refers to, mutates inputs, applies/imports, enforces policy,
// runs shell commands, uploads, or collects logs/environment/process data.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// AnalysisSchemaVersion is the schema version of the replay analysis. It is a
// local tool schema (not a REST/API contract), independent of the workflow
// artifact schemas it analyzes.
const AnalysisSchemaVersion = 1

// Finding severities.
const (
	severityInfo    = "info"
	severityWarning = "warning"
	severityError   = "error"
)

// Run-level codes the run-report analyzer recognizes for findings/insights.
const (
	codeDependencyFailed   = "workflow.run.dependency_failed"
	codeRuntimeUnreachable = "workflow.run.runtime_unreachable"
	codeInputFailed        = "workflow.run.input_failed"
)

// StepSummary tallies a report's steps. Run-style reports populate
// passed/failed/skipped; plan-style reports populate valid/invalid. Total is
// always the step count. All fields are always present for a stable shape.
type StepSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Valid   int `json:"valid"`
	Invalid int `json:"invalid"`
}

// RunSummary tallies a history export's runs.
type RunSummary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

// AnalysisSummary holds both tallies; the irrelevant one stays zero.
type AnalysisSummary struct {
	Steps StepSummary `json:"steps"`
	Runs  RunSummary  `json:"runs"`
}

// CodeCount is the number of times an emitted code appears in the artifact (step
// emissions for a single report; runs-containing for a history export).
type CodeCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

// WorkflowCount is a per-workflow-name run count (history exports).
type WorkflowCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Finding is a higher-signal observation derived from the artifact.
type Finding struct {
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
	Related  []string `json:"related,omitempty"`
}

// Analysis is the deterministic offline analysis of one artifact. Workflow/Result
// are omitted when the artifact has none; Workflows appears only for history
// exports. Codes/Findings/Insights are always present (`[]` when empty).
type Analysis struct {
	SchemaVersion int             `json:"schemaVersion"`
	Source        string          `json:"source"`
	Workflow      string          `json:"workflow,omitempty"`
	Result        string          `json:"result,omitempty"`
	Summary       AnalysisSummary `json:"summary"`
	Codes         []CodeCount     `json:"codes"`
	Workflows     []WorkflowCount `json:"workflows,omitempty"`
	Findings      []Finding       `json:"findings"`
	Insights      []string        `json:"insights"`
}

// --- lenient raw shapes (analysis-specific; richer than detection) ---

// analyzeRawReport is the lenient shape of a run OR plan report. Run reports use
// `workflow`+passed/failed/skipped step status + an embedded `report`; plan reports
// use `name`+valid/invalid step status + a validation `code`.
type analyzeRawReport struct {
	Workflow     string                     `json:"workflow"`
	Name         string                     `json:"name"`
	Result       string                     `json:"result"`
	Steps        []analyzeRawStep           `json:"steps"`
	Explanations map[string]json.RawMessage `json:"explanations"`
}

type analyzeRawStep struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Status string          `json:"status"`
	Code   string          `json:"code"`   // plan: validation code
	Report json.RawMessage `json:"report"` // run: embedded policy/compare report
}

// analyzeEmbeddedReport is the subset of a run step's embedded report read for
// codes: error-severity findings and baseline-compare new findings. It is parsed
// from the artifact JSON only — never from a fresh evaluation.
type analyzeEmbeddedReport struct {
	Findings []struct {
		Code     string `json:"code"`
		Severity string `json:"severity"`
	} `json:"findings"`
	New []struct {
		Code string `json:"code"`
	} `json:"new"`
}

// analyzeRawHistory is the lenient shape of a history export.
type analyzeRawHistory struct {
	Runs []analyzeRawHistoryRun `json:"runs"`
}

type analyzeRawHistoryRun struct {
	ID       string   `json:"id"`
	Workflow string   `json:"workflow"`
	Result   string   `json:"result"`
	Codes    []string `json:"codes"`
}

// analyzeBundleManifest / analyzeBundleReport are the lenient shapes of a support
// bundle's manifest.json and report.json.
type analyzeBundleManifest struct {
	Source   string `json:"source"`
	Workflow string `json:"workflow"`
	Result   string `json:"result"`
}

type analyzeBundleReport struct {
	Kind     string                    `json:"kind"`
	Workflow string                    `json:"workflow"`
	Result   string                    `json:"result"`
	Steps    []analyzeBundleReportStep `json:"steps"`
}

type analyzeBundleReportStep struct {
	ID     string   `json:"id"`
	Type   string   `json:"type"`
	Status string   `json:"status"`
	Codes  []string `json:"codes"`
}

// AnalyzeInput resolves an artifact path (a file or a bundle directory) into a
// deterministic Analysis. A directory is treated as a support-report bundle (only
// its own manifest/report/explanations files are read); a file is parsed as a
// single-file JSON artifact. It reads nothing else and contacts nothing.
func AnalyzeInput(path string) (Analysis, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Analysis{}, err
	}
	if info.IsDir() {
		return analyzeBundleDir(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Analysis{}, err
	}
	return AnalyzeJSON(data)
}

// AnalyzeJSON classifies a single-file artifact (reusing detection) and builds its
// analysis. It is pure (no filesystem access) so it is independently testable.
func AnalyzeJSON(data []byte) (Analysis, error) {
	in, err := DetectJSON(data)
	if err != nil {
		return Analysis{}, err
	}
	switch in.Kind {
	case SourceWorkflowRunReport:
		return analyzeRunReport(data)
	case SourceWorkflowPlanReport:
		return analyzePlanReport(data)
	case SourceWorkflowHistoryExport:
		return analyzeHistoryExport(data)
	default:
		// DetectJSON never classifies a bundle from bytes; this is defensive.
		return Analysis{}, fmt.Errorf("unsupported artifact for analysis")
	}
}

func analyzeRunReport(data []byte) (Analysis, error) {
	var raw analyzeRawReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return Analysis{}, fmt.Errorf("not a valid workflow run report: %w", err)
	}
	a := newAnalysis(SourceWorkflowRunReport, raw.Workflow, raw.Result)

	var failedIDs, skippedIDs []string
	var perStepCodes [][]string
	for _, s := range raw.Steps {
		tallyStep(&a.Summary.Steps, s.Status)
		switch s.Status {
		case statusFailed:
			failedIDs = append(failedIDs, s.ID)
			perStepCodes = append(perStepCodes, embeddedFindingCodes(s.Report))
		case statusSkipped:
			skippedIDs = append(skippedIDs, s.ID)
			perStepCodes = append(perStepCodes, []string{codeDependencyFailed})
		}
	}
	a.Codes = countCodes(perStepCodes, sortedMapKeys(raw.Explanations))
	sort.Strings(failedIDs)
	sort.Strings(skippedIDs)
	present := codeSet(a.Codes)

	if len(failedIDs) > 0 {
		a.Findings = append(a.Findings, Finding{Severity: severityWarning, Message: "Workflow has failed steps.", Related: failedIDs})
	}
	if len(skippedIDs) > 0 {
		a.Findings = append(a.Findings, Finding{Severity: severityInfo, Message: "Workflow has skipped steps.", Related: skippedIDs})
	}
	if present[codeRuntimeUnreachable] {
		a.Findings = append(a.Findings, Finding{Severity: severityWarning, Message: "Runtime unreachable was reported."})
	}
	if present[codeInputFailed] {
		a.Findings = append(a.Findings, Finding{Severity: severityWarning, Message: "Input failure was reported."})
	}

	switch {
	case a.Result == statusPassed:
		a.Insights = append(a.Insights, "Workflow completed successfully.")
	case len(failedIDs) > 0:
		a.Insights = append(a.Insights, "Workflow failed because one or more steps failed.")
	default:
		a.Insights = append(a.Insights, "Workflow did not complete successfully.")
	}
	if len(skippedIDs) > 0 {
		a.Insights = append(a.Insights, "Workflow has skipped steps, likely caused by a failed dependency or missing prior report.")
	}
	if present[codeRuntimeUnreachable] {
		a.Insights = append(a.Insights, "Runtime unreachable was reported.")
	}
	if present[codeInputFailed] {
		a.Insights = append(a.Insights, "Input failure was reported.")
	}
	return a, nil
}

func analyzePlanReport(data []byte) (Analysis, error) {
	var raw analyzeRawReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return Analysis{}, fmt.Errorf("not a valid workflow plan report: %w", err)
	}
	a := newAnalysis(SourceWorkflowPlanReport, raw.Name, raw.Result)

	var invalidIDs []string
	var perStepCodes [][]string
	for _, s := range raw.Steps {
		tallyStep(&a.Summary.Steps, s.Status)
		if s.Status == statusInvalid {
			invalidIDs = append(invalidIDs, s.ID)
			if s.Code != "" {
				perStepCodes = append(perStepCodes, []string{s.Code})
			}
		}
	}
	a.Codes = countCodes(perStepCodes, sortedMapKeys(raw.Explanations))
	sort.Strings(invalidIDs)

	if len(invalidIDs) > 0 {
		a.Findings = append(a.Findings, Finding{Severity: severityError, Message: "Workflow plan has invalid steps.", Related: invalidIDs})
		a.Insights = append(a.Insights, "Workflow plan has invalid steps and should not be run until fixed.")
	} else {
		a.Insights = append(a.Insights, "Workflow plan is valid.")
	}
	return a, nil
}

func analyzeHistoryExport(data []byte) (Analysis, error) {
	var raw analyzeRawHistory
	if err := json.Unmarshal(data, &raw); err != nil {
		return Analysis{}, fmt.Errorf("not a valid workflow history export: %w", err)
	}
	a := newAnalysis(SourceWorkflowHistoryExport, "", "")

	var failedRunIDs []string
	workflows := map[string]int{}
	for _, r := range raw.Runs {
		a.Summary.Runs.Total++
		switch r.Result {
		case statusPassed:
			a.Summary.Runs.Passed++
		case statusFailed:
			a.Summary.Runs.Failed++
			failedRunIDs = append(failedRunIDs, r.ID)
		}
		if r.Workflow != "" {
			workflows[r.Workflow]++
		}
	}
	a.Codes = historyCodeCounts(raw.Runs)
	a.Workflows = workflowCounts(workflows)
	sort.Strings(failedRunIDs)

	if a.Summary.Runs.Total == 0 {
		a.Insights = append(a.Insights, "History export contains no runs.")
		return a, nil
	}
	if len(failedRunIDs) > 0 {
		a.Findings = append(a.Findings, Finding{Severity: severityWarning, Message: "Failed runs are present.", Related: failedRunIDs})
		a.Insights = append(a.Insights, "Failed runs are present.")
	} else {
		a.Insights = append(a.Insights, "All recorded runs passed.")
	}
	if len(a.Codes) > 0 {
		a.Insights = append(a.Insights, "Most common emitted code is "+a.Codes[0].Code+".")
	}
	// The export preserves the store's newest-first order, so runs[0] is most recent.
	recent := raw.Runs[0]
	a.Insights = append(a.Insights, fmt.Sprintf("Most recent run %s (%s).", recent.ID, recent.Result))
	return a, nil
}

// analyzeBundleDir analyzes a support-report bundle by reading ONLY its own
// manifest.json (required), report.json, and explanations.json. No other file —
// and in particular no file the report refers to — is read.
func analyzeBundleDir(dir string) (Analysis, error) {
	manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Analysis{}, fmt.Errorf("not a workflow report bundle (no readable manifest.json): %w", err)
	}
	if _, err := DetectBundleManifest(manifestData); err != nil {
		return Analysis{}, err
	}
	var man analyzeBundleManifest
	_ = json.Unmarshal(manifestData, &man) // shape already validated by DetectBundleManifest

	var rep analyzeBundleReport
	if data, err := os.ReadFile(filepath.Join(dir, "report.json")); err == nil {
		_ = json.Unmarshal(data, &rep) // tolerate a missing/malformed report; manifest suffices
	}

	var explKeys []string
	if data, err := os.ReadFile(filepath.Join(dir, "explanations.json")); err == nil {
		var m map[string]json.RawMessage
		if json.Unmarshal(data, &m) == nil {
			explKeys = sortedMapKeys(m)
		}
	}

	return buildBundleAnalysis(man, rep, explKeys), nil
}

func buildBundleAnalysis(man analyzeBundleManifest, rep analyzeBundleReport, explKeys []string) Analysis {
	workflow := firstNonEmpty(man.Workflow, rep.Workflow)
	result := firstNonEmpty(man.Result, rep.Result)
	a := newAnalysis(SourceWorkflowReportBundle, workflow, result)

	var perStepCodes [][]string
	var failedIDs []string
	for _, s := range rep.Steps {
		tallyStep(&a.Summary.Steps, s.Status)
		if len(s.Codes) > 0 {
			perStepCodes = append(perStepCodes, dedupeStrings(s.Codes))
		}
		if s.Status == statusFailed || s.Status == statusInvalid {
			failedIDs = append(failedIDs, s.ID)
		}
	}
	a.Codes = countCodes(perStepCodes, explKeys)
	sort.Strings(failedIDs)

	if len(failedIDs) > 0 {
		a.Findings = append(a.Findings, Finding{Severity: severityWarning, Message: "Bundle reports failed or invalid steps.", Related: failedIDs})
	}
	if man.Source != "" {
		a.Insights = append(a.Insights, fmt.Sprintf("Bundle was produced from a %s report.", man.Source))
	}
	a.Insights = append(a.Insights, "Support bundle is normalized and suitable for offline review.")
	if len(a.Codes) > 0 {
		a.Insights = append(a.Insights, "Bundle contains explanations for emitted codes.")
	}
	return a
}

// --- helpers ---

// newAnalysis returns an Analysis with non-nil Codes/Findings/Insights slices so
// they serialize as `[]` rather than `null`.
func newAnalysis(source, workflow, result string) Analysis {
	return Analysis{
		SchemaVersion: AnalysisSchemaVersion,
		Source:        source,
		Workflow:      workflow,
		Result:        result,
		Codes:         []CodeCount{},
		Findings:      []Finding{},
		Insights:      []string{},
	}
}

// tallyStep increments the right StepSummary counter for a step status.
func tallyStep(sum *StepSummary, status string) {
	sum.Total++
	switch status {
	case statusPassed:
		sum.Passed++
	case statusFailed:
		sum.Failed++
	case statusSkipped:
		sum.Skipped++
	case statusValid:
		sum.Valid++
	case statusInvalid:
		sum.Invalid++
	}
}

// embeddedFindingCodes extracts the policy finding codes from a run step's embedded
// report (error-severity findings + baseline new findings), deduped. It reads the
// embedded JSON only — never a referenced file or a fresh evaluation.
func embeddedFindingCodes(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var rep analyzeEmbeddedReport
	if json.Unmarshal(raw, &rep) != nil {
		return nil
	}
	var codes []string
	for _, f := range rep.Findings {
		if f.Severity == severityError && f.Code != "" {
			codes = append(codes, f.Code)
		}
	}
	for _, n := range rep.New {
		if n.Code != "" {
			codes = append(codes, n.Code)
		}
	}
	return dedupeStrings(codes)
}

// countCodes counts code occurrences across per-step code lists, then adds any
// explanation-map key not already counted (count 1) so a code the report explicitly
// explained still appears. Result sorted by count desc, then code asc.
func countCodes(perStepCodes [][]string, explanationKeys []string) []CodeCount {
	counts := map[string]int{}
	for _, codes := range perStepCodes {
		for _, c := range codes {
			counts[c]++
		}
	}
	for _, k := range explanationKeys {
		if _, ok := counts[k]; !ok {
			counts[k] = 1
		}
	}
	return toSortedCodeCounts(counts)
}

// historyCodeCounts counts, for each code, the number of RUNS containing it (codes
// are deduped per run). Result sorted by count desc, then code asc.
func historyCodeCounts(runs []analyzeRawHistoryRun) []CodeCount {
	counts := map[string]int{}
	for _, r := range runs {
		seen := map[string]bool{}
		for _, c := range r.Codes {
			if seen[c] {
				continue
			}
			seen[c] = true
			counts[c]++
		}
	}
	return toSortedCodeCounts(counts)
}

func toSortedCodeCounts(counts map[string]int) []CodeCount {
	out := make([]CodeCount, 0, len(counts))
	for c, n := range counts {
		out = append(out, CodeCount{Code: c, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// workflowCounts turns a name→count map into a sorted slice (count desc, name asc).
func workflowCounts(counts map[string]int) []WorkflowCount {
	out := make([]WorkflowCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, WorkflowCount{Name: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// codeSet returns the set of codes present in a CodeCount slice.
func codeSet(codes []CodeCount) map[string]bool {
	set := make(map[string]bool, len(codes))
	for _, c := range codes {
		set[c.Code] = true
	}
	return set
}

// dedupeStrings returns the unique values of in, preserving first-seen order.
func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
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

// sortedMapKeys returns the sorted, deduplicated keys of a JSON map.
func sortedMapKeys(m map[string]json.RawMessage) []string {
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

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
