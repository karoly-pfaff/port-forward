package core

import (
	"path/filepath"
	"testing"
)

func findingMessages(a Analysis) []string {
	out := make([]string, 0, len(a.Findings))
	for _, f := range a.Findings {
		out = append(out, f.Message)
	}
	return out
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func codeCount(t *testing.T, a Analysis, code string) int {
	t.Helper()
	for _, c := range a.Codes {
		if c.Code == code {
			return c.Count
		}
	}
	return 0
}

func TestAnalyze_PassedRunReport(t *testing.T) {
	data := []byte(`{
		"workflow": "all-good",
		"result": "passed",
		"steps": [
			{"id": "check", "type": "policy.check", "status": "passed", "exitCode": 0},
			{"id": "review", "type": "policy.review", "status": "passed", "exitCode": 0}
		]
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatalf("AnalyzeJSON: %v", err)
	}
	if a.Source != SourceWorkflowRunReport {
		t.Errorf("Source = %q", a.Source)
	}
	if a.Summary.Steps.Total != 2 || a.Summary.Steps.Passed != 2 || a.Summary.Steps.Failed != 0 {
		t.Errorf("step summary = %+v", a.Summary.Steps)
	}
	if len(a.Codes) != 0 {
		t.Errorf("expected no codes, got %+v", a.Codes)
	}
	if len(a.Findings) != 0 {
		t.Errorf("expected no findings, got %v", findingMessages(a))
	}
	if !containsStr(a.Insights, "Workflow completed successfully.") {
		t.Errorf("missing success insight: %v", a.Insights)
	}
}

func TestAnalyze_FailedRunReportWithCodes(t *testing.T) {
	data := []byte(`{
		"workflow": "policy-baseline-check",
		"result": "failed",
		"steps": [
			{"id": "check", "type": "policy.check", "status": "passed", "exitCode": 0},
			{"id": "compare", "type": "policy.baseline.compare", "status": "failed", "exitCode": 1,
				"report": {"findings": [{"code": "policy.lan_exposure_forbidden", "severity": "error"}]}}
		]
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatalf("AnalyzeJSON: %v", err)
	}
	if a.Summary.Steps.Failed != 1 {
		t.Errorf("expected 1 failed, got %+v", a.Summary.Steps)
	}
	if codeCount(t, a, "policy.lan_exposure_forbidden") != 1 {
		t.Errorf("expected lan_exposure code count 1, got %+v", a.Codes)
	}
	if !containsStr(findingMessages(a), "Workflow has failed steps.") {
		t.Errorf("missing failed-steps finding: %v", findingMessages(a))
	}
	// the failed finding's related must list the failed step id
	var related []string
	for _, f := range a.Findings {
		if f.Message == "Workflow has failed steps." {
			related = f.Related
		}
	}
	if len(related) != 1 || related[0] != "compare" {
		t.Errorf("failed finding related = %v, want [compare]", related)
	}
	if !containsStr(a.Insights, "Workflow failed because one or more steps failed.") {
		t.Errorf("missing failure insight: %v", a.Insights)
	}
}

func TestAnalyze_RunReportWithSkippedStep(t *testing.T) {
	data := []byte(`{
		"workflow": "deps",
		"result": "failed",
		"steps": [
			{"id": "compare", "type": "policy.baseline.compare", "status": "failed", "exitCode": 1},
			{"id": "dependent", "type": "policy.check", "status": "skipped", "exitCode": 0}
		]
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatalf("AnalyzeJSON: %v", err)
	}
	if a.Summary.Steps.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %+v", a.Summary.Steps)
	}
	if codeCount(t, a, codeDependencyFailed) != 1 {
		t.Errorf("expected dependency_failed code, got %+v", a.Codes)
	}
	if !containsStr(a.Insights, "Workflow has skipped steps, likely caused by a failed dependency or missing prior report.") {
		t.Errorf("missing skipped insight: %v", a.Insights)
	}
}

func TestAnalyze_RunReportRuntimeUnreachableAndInputFailed(t *testing.T) {
	// A failed runtime step plus an explanations map carrying the run-level codes.
	data := []byte(`{
		"workflow": "rt",
		"result": "failed",
		"steps": [{"id": "check", "type": "policy.check", "status": "failed", "exitCode": 3}],
		"explanations": {
			"workflow.run.runtime_unreachable": {"code": "workflow.run.runtime_unreachable"},
			"workflow.run.input_failed": {"code": "workflow.run.input_failed"}
		}
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatalf("AnalyzeJSON: %v", err)
	}
	if !containsStr(findingMessages(a), "Runtime unreachable was reported.") {
		t.Errorf("missing runtime-unreachable finding: %v", findingMessages(a))
	}
	if !containsStr(findingMessages(a), "Input failure was reported.") {
		t.Errorf("missing input-failed finding: %v", findingMessages(a))
	}
	if !containsStr(a.Insights, "Runtime unreachable was reported.") {
		t.Errorf("missing runtime-unreachable insight: %v", a.Insights)
	}
}

func TestAnalyze_ValidPlanReport(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"name": "policy-review",
		"result": "valid",
		"steps": [{"id": "review", "type": "policy.review", "status": "valid", "code": "workflow.step.valid"}]
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatalf("AnalyzeJSON: %v", err)
	}
	if a.Source != SourceWorkflowPlanReport {
		t.Errorf("Source = %q", a.Source)
	}
	if a.Summary.Steps.Valid != 1 || a.Summary.Steps.Invalid != 0 {
		t.Errorf("step summary = %+v", a.Summary.Steps)
	}
	if !containsStr(a.Insights, "Workflow plan is valid.") {
		t.Errorf("missing valid-plan insight: %v", a.Insights)
	}
	if len(a.Findings) != 0 {
		t.Errorf("valid plan should have no findings, got %v", findingMessages(a))
	}
}

func TestAnalyze_InvalidPlanReport(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"name": "broken",
		"result": "invalid",
		"steps": [
			{"id": "a", "type": "policy.review", "status": "invalid", "code": "workflow.step.missing_policy"},
			{"id": "b", "type": "policy.check", "status": "valid", "code": "workflow.step.valid"}
		]
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatalf("AnalyzeJSON: %v", err)
	}
	if a.Summary.Steps.Invalid != 1 || a.Summary.Steps.Valid != 1 {
		t.Errorf("step summary = %+v", a.Summary.Steps)
	}
	if codeCount(t, a, "workflow.step.missing_policy") != 1 {
		t.Errorf("expected missing_policy code, got %+v", a.Codes)
	}
	if !containsStr(findingMessages(a), "Workflow plan has invalid steps.") {
		t.Errorf("missing invalid-plan finding: %v", findingMessages(a))
	}
	if !containsStr(a.Insights, "Workflow plan has invalid steps and should not be run until fixed.") {
		t.Errorf("missing invalid-plan insight: %v", a.Insights)
	}
}

func TestAnalyze_EmptyHistoryExport(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"source": "workflow-history",
		"runCount": 0,
		"runs": [],
		"safety": {}
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatalf("AnalyzeJSON: %v", err)
	}
	if a.Source != SourceWorkflowHistoryExport {
		t.Errorf("Source = %q", a.Source)
	}
	if a.Summary.Runs.Total != 0 {
		t.Errorf("expected 0 runs, got %+v", a.Summary.Runs)
	}
	if len(a.Workflows) != 0 {
		t.Errorf("empty export should have no workflows, got %+v", a.Workflows)
	}
	if !containsStr(a.Insights, "History export contains no runs.") {
		t.Errorf("missing empty insight: %v", a.Insights)
	}
}

func TestAnalyze_PopulatedHistoryExport(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"source": "workflow-history",
		"runCount": 3,
		"runs": [
			{"id": "20260614T120000Z-w1-aaaa", "workflow": "w1", "result": "failed", "codes": ["workflow.run.input_failed"]},
			{"id": "20260614T110000Z-w2-bbbb", "workflow": "w2", "result": "passed"},
			{"id": "20260614T100000Z-w1-cccc", "workflow": "w1", "result": "failed", "codes": ["workflow.run.input_failed", "policy.lan_exposure_forbidden"]}
		],
		"safety": {}
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatalf("AnalyzeJSON: %v", err)
	}
	if a.Summary.Runs.Total != 3 || a.Summary.Runs.Passed != 1 || a.Summary.Runs.Failed != 2 {
		t.Errorf("run summary = %+v", a.Summary.Runs)
	}
	// workflow distribution: w1=2, w2=1, sorted count desc then name asc
	if len(a.Workflows) != 2 || a.Workflows[0].Name != "w1" || a.Workflows[0].Count != 2 {
		t.Errorf("workflows = %+v", a.Workflows)
	}
	// code distribution counts RUNS containing each code: input_failed=2, lan=1
	if codeCount(t, a, "workflow.run.input_failed") != 2 {
		t.Errorf("input_failed should count 2 runs, got %+v", a.Codes)
	}
	if codeCount(t, a, "policy.lan_exposure_forbidden") != 1 {
		t.Errorf("lan_exposure should count 1 run, got %+v", a.Codes)
	}
	// most common code first
	if a.Codes[0].Code != "workflow.run.input_failed" {
		t.Errorf("most common code = %q, want workflow.run.input_failed", a.Codes[0].Code)
	}
	if !containsStr(findingMessages(a), "Failed runs are present.") {
		t.Errorf("missing failed-runs finding: %v", findingMessages(a))
	}
	if !containsStr(a.Insights, "Most common emitted code is workflow.run.input_failed.") {
		t.Errorf("missing most-common-code insight: %v", a.Insights)
	}
	// most recent run is runs[0] (newest-first)
	if !containsStr(a.Insights, "Most recent run 20260614T120000Z-w1-aaaa (failed).") {
		t.Errorf("missing most-recent insight: %v", a.Insights)
	}
}

func TestHistoryCodeCounts_DedupePerRun(t *testing.T) {
	// A code listed twice within one run counts that run once.
	runs := []analyzeRawHistoryRun{
		{ID: "a", Codes: []string{"x", "x"}},
		{ID: "b", Codes: []string{"x"}},
	}
	counts := historyCodeCounts(runs)
	if len(counts) != 1 || counts[0].Code != "x" || counts[0].Count != 2 {
		t.Errorf("counts = %+v, want x:2 (runs-containing)", counts)
	}
}

func TestCountCodes_OrderingAndExplanationUnion(t *testing.T) {
	per := [][]string{{"b"}, {"b"}, {"a"}}
	counts := countCodes(per, []string{"c", "b"}) // c is explanation-only; b already counted
	// b=2, a=1, c=1; sorted count desc then code asc -> b, a, c
	if len(counts) != 3 {
		t.Fatalf("counts = %+v", counts)
	}
	if counts[0].Code != "b" || counts[0].Count != 2 {
		t.Errorf("first = %+v, want b:2", counts[0])
	}
	if counts[1].Code != "a" || counts[2].Code != "c" {
		t.Errorf("tie-break order wrong: %+v", counts)
	}
}

func TestAnalyze_BundleDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"),
		`{"schemaVersion":1,"type":"workflow-report","source":"workflow-run","workflow":"policy-baseline-check","result":"failed"}`)
	writeFile(t, filepath.Join(dir, "report.json"), `{
		"kind": "workflow-run",
		"workflow": "policy-baseline-check",
		"result": "failed",
		"steps": [
			{"id": "check", "type": "policy.check", "status": "passed"},
			{"id": "compare", "type": "policy.baseline.compare", "status": "failed", "codes": ["policy.lan_exposure_forbidden"]}
		]
	}`)
	writeFile(t, filepath.Join(dir, "explanations.json"), `{"policy.lan_exposure_forbidden": {"code": "policy.lan_exposure_forbidden"}}`)

	a, err := AnalyzeInput(dir)
	if err != nil {
		t.Fatalf("AnalyzeInput: %v", err)
	}
	if a.Source != SourceWorkflowReportBundle {
		t.Errorf("Source = %q", a.Source)
	}
	if a.Workflow != "policy-baseline-check" || a.Result != "failed" {
		t.Errorf("workflow/result = %q/%q", a.Workflow, a.Result)
	}
	if a.Summary.Steps.Total != 2 || a.Summary.Steps.Failed != 1 {
		t.Errorf("step summary = %+v", a.Summary.Steps)
	}
	if codeCount(t, a, "policy.lan_exposure_forbidden") != 1 {
		t.Errorf("expected code from bundle, got %+v", a.Codes)
	}
	if !containsStr(a.Insights, "Support bundle is normalized and suitable for offline review.") {
		t.Errorf("missing bundle insight: %v", a.Insights)
	}
	if !containsStr(a.Insights, "Bundle contains explanations for emitted codes.") {
		t.Errorf("missing explanations insight: %v", a.Insights)
	}
}

func TestAnalyze_BundleDirManifestOnly(t *testing.T) {
	// A bundle with only a manifest still analyzes (no report → minimal analysis).
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"),
		`{"schemaVersion":1,"type":"workflow-report","source":"workflow-plan","workflow":"w","result":"valid"}`)
	a, err := AnalyzeInput(dir)
	if err != nil {
		t.Fatalf("AnalyzeInput: %v", err)
	}
	if a.Summary.Steps.Total != 0 {
		t.Errorf("expected 0 steps without report.json, got %+v", a.Summary.Steps)
	}
	if !containsStr(a.Insights, "Support bundle is normalized and suitable for offline review.") {
		t.Errorf("missing bundle insight: %v", a.Insights)
	}
}

func TestAnalyze_BundleDirNoManifest(t *testing.T) {
	if _, err := AnalyzeInput(t.TempDir()); err == nil {
		t.Fatal("expected error for bundle dir without manifest.json")
	}
}

func TestAnalyzeJSON_Malformed(t *testing.T) {
	if _, err := AnalyzeJSON([]byte("{ not json")); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestAnalyzeJSON_Unsupported(t *testing.T) {
	if _, err := AnalyzeJSON([]byte(`{"hello":"world"}`)); err == nil {
		t.Fatal("expected error for unsupported shape")
	}
}

func TestAnalyzeInput_MissingPath(t *testing.T) {
	if _, err := AnalyzeInput(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestAnalyzeInput_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.json")
	writeFile(t, path, `{"workflow":"w","result":"passed","steps":[{"id":"s","type":"policy.check","status":"passed"}]}`)
	a, err := AnalyzeInput(path)
	if err != nil {
		t.Fatalf("AnalyzeInput: %v", err)
	}
	if a.Source != SourceWorkflowRunReport {
		t.Errorf("Source = %q", a.Source)
	}
}

func TestAnalyze_FailedRunWithOnlySkippedStep(t *testing.T) {
	// A run that "failed" with no failed step (only a skipped one) hits the
	// generic did-not-complete insight rather than the failed-steps insight.
	data := []byte(`{
		"workflow": "deps", "result": "failed",
		"steps": [{"id":"dependent","type":"policy.check","status":"skipped","exitCode":0}]
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(a.Insights, "Workflow did not complete successfully.") {
		t.Errorf("missing generic did-not-complete insight: %v", a.Insights)
	}
	if containsStr(findingMessages(a), "Workflow has failed steps.") {
		t.Errorf("should have no failed-steps finding: %v", findingMessages(a))
	}
}

func TestAnalyze_HistoryAllPassed(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1, "source": "workflow-history", "runCount": 2, "runs": [
			{"id":"b","workflow":"w","result":"passed","codes":["x"]},
			{"id":"a","workflow":"w","result":"passed"}
		], "safety": {}
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if a.Summary.Runs.Failed != 0 {
		t.Errorf("expected 0 failed, got %+v", a.Summary.Runs)
	}
	if !containsStr(a.Insights, "All recorded runs passed.") {
		t.Errorf("missing all-passed insight: %v", a.Insights)
	}
	if len(a.Findings) != 0 {
		t.Errorf("all-passed history should have no findings: %v", findingMessages(a))
	}
}

func TestAnalyze_BundleWrongManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{"schemaVersion":1,"type":"not-a-report"}`)
	if _, err := AnalyzeInput(dir); err == nil {
		t.Fatal("expected error for a non-workflow-report manifest")
	}
}

func TestAnalyze_BundleMalformedReportTolerated(t *testing.T) {
	// A malformed report.json is tolerated: the manifest alone yields a minimal
	// analysis (no steps), never an error.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"),
		`{"schemaVersion":1,"type":"workflow-report","source":"workflow-run","workflow":"w","result":"failed"}`)
	writeFile(t, filepath.Join(dir, "report.json"), `{ not valid json`)
	a, err := AnalyzeInput(dir)
	if err != nil {
		t.Fatalf("AnalyzeInput: %v", err)
	}
	if a.Summary.Steps.Total != 0 {
		t.Errorf("malformed report should yield 0 steps, got %+v", a.Summary.Steps)
	}
}

func TestEmbeddedFindingCodes(t *testing.T) {
	// No report → nil.
	if got := embeddedFindingCodes(nil); got != nil {
		t.Errorf("empty report = %v, want nil", got)
	}
	// Malformed embedded report → nil.
	if got := embeddedFindingCodes([]byte("{ bad")); got != nil {
		t.Errorf("malformed report = %v, want nil", got)
	}
	// Error-severity findings + baseline new findings; non-error and empty-code dropped.
	raw := []byte(`{
		"findings": [
			{"code":"policy.a","severity":"error"},
			{"code":"policy.b","severity":"warning"},
			{"code":"","severity":"error"}
		],
		"new": [{"code":"policy.c"},{"code":""}]
	}`)
	got := embeddedFindingCodes(raw)
	if len(got) != 2 || got[0] != "policy.a" || got[1] != "policy.c" {
		t.Errorf("embeddedFindingCodes = %v, want [policy.a policy.c]", got)
	}
}

func TestDedupeStrings(t *testing.T) {
	if got := dedupeStrings(nil); got != nil {
		t.Errorf("empty = %v, want nil", got)
	}
	got := dedupeStrings([]string{"a", "b", "a", "c", "b"})
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("dedupeStrings = %v, want [a b c]", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("x", "y"); got != "x" {
		t.Errorf("got %q, want x", got)
	}
	if got := firstNonEmpty("", "y"); got != "y" {
		t.Errorf("got %q, want y", got)
	}
}

func TestAnalyze_Deterministic(t *testing.T) {
	data := []byte(`{
		"workflow": "w", "result": "failed",
		"steps": [{"id":"s","type":"policy.check","status":"failed","exitCode":1,
			"report": {"findings": [{"code":"policy.privileged_port_forbidden","severity":"error"},{"code":"policy.lan_exposure_forbidden","severity":"error"}]}}]
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	da, _ := MarshalAnalysis(a)
	db, _ := MarshalAnalysis(b)
	if string(da) != string(db) {
		t.Error("analysis is not deterministic across calls")
	}
}
