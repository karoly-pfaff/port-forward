package replay

import (
	"path/filepath"
	"testing"
)

func TestDetectJSON_WorkflowRunReport(t *testing.T) {
	data := []byte(`{
		"workflow": "policy-baseline-check",
		"result": "failed",
		"steps": [
			{"id": "check", "type": "policy.check", "status": "passed", "exitCode": 0},
			{"id": "compare", "type": "policy.baseline.compare", "status": "failed", "exitCode": 1}
		],
		"summary": {"total": 2, "passed": 1, "failed": 1, "skipped": 0}
	}`)
	in, err := DetectJSON(data)
	if err != nil {
		t.Fatalf("DetectJSON: %v", err)
	}
	if in.Kind != SourceWorkflowRunReport {
		t.Errorf("Kind = %q, want %q", in.Kind, SourceWorkflowRunReport)
	}
	if in.Workflow != "policy-baseline-check" {
		t.Errorf("Workflow = %q", in.Workflow)
	}
	if in.Result != "failed" {
		t.Errorf("Result = %q", in.Result)
	}
	if in.Steps != 2 {
		t.Errorf("Steps = %d, want 2", in.Steps)
	}
	if !in.HasCodes {
		t.Errorf("HasCodes = false, want true (a failed step implies explainable codes)")
	}
}

func TestDetectJSON_WorkflowRunReportPassedNoCodes(t *testing.T) {
	data := []byte(`{
		"workflow": "all-good",
		"result": "passed",
		"steps": [{"id": "check", "type": "policy.check", "status": "passed", "exitCode": 0}]
	}`)
	in, err := DetectJSON(data)
	if err != nil {
		t.Fatalf("DetectJSON: %v", err)
	}
	if in.Kind != SourceWorkflowRunReport {
		t.Errorf("Kind = %q, want %q", in.Kind, SourceWorkflowRunReport)
	}
	if in.HasCodes {
		t.Errorf("HasCodes = true, want false (all steps passed, no explanations map)")
	}
}

func TestDetectJSON_WorkflowRunReportExplanationsMap(t *testing.T) {
	// A passed run that still carries an explanations map (produced with --explain)
	// should report HasCodes.
	data := []byte(`{
		"workflow": "explained",
		"result": "passed",
		"steps": [{"id": "check", "type": "policy.check", "status": "passed", "exitCode": 0}],
		"explanations": {"policy.lan_exposure_forbidden": {"code": "policy.lan_exposure_forbidden"}}
	}`)
	in, err := DetectJSON(data)
	if err != nil {
		t.Fatalf("DetectJSON: %v", err)
	}
	if !in.HasCodes {
		t.Errorf("HasCodes = false, want true (explanations map present)")
	}
}

func TestDetectJSON_WorkflowPlanReport(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"name": "policy-review",
		"result": "valid",
		"steps": [{"id": "review", "type": "policy.review", "status": "valid", "code": "workflow.step.valid"}],
		"summary": {"total": 1, "valid": 1, "invalid": 0}
	}`)
	in, err := DetectJSON(data)
	if err != nil {
		t.Fatalf("DetectJSON: %v", err)
	}
	if in.Kind != SourceWorkflowPlanReport {
		t.Errorf("Kind = %q, want %q", in.Kind, SourceWorkflowPlanReport)
	}
	if in.Workflow != "policy-review" {
		t.Errorf("Workflow = %q", in.Workflow)
	}
	if in.Result != "valid" {
		t.Errorf("Result = %q", in.Result)
	}
	if in.Steps != 1 {
		t.Errorf("Steps = %d, want 1", in.Steps)
	}
	if in.HasCodes {
		t.Errorf("HasCodes = true, want false (a valid plan has no explainable codes)")
	}
}

func TestDetectJSON_WorkflowPlanReportInvalidHasCodes(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"name": "broken",
		"result": "invalid",
		"steps": [{"id": "review", "type": "policy.review", "status": "invalid", "code": "workflow.step.missing_policy"}]
	}`)
	in, err := DetectJSON(data)
	if err != nil {
		t.Fatalf("DetectJSON: %v", err)
	}
	if in.Kind != SourceWorkflowPlanReport {
		t.Errorf("Kind = %q", in.Kind)
	}
	if !in.HasCodes {
		t.Errorf("HasCodes = false, want true (an invalid step has an explainable code)")
	}
}

func TestDetectJSON_WorkflowHistoryExport(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"createdAt": "2026-06-14T10:15:30Z",
		"source": "workflow-history",
		"runCount": 2,
		"runs": [
			{"id": "a", "workflow": "w1", "result": "passed"},
			{"id": "b", "workflow": "w2", "result": "failed"}
		],
		"safety": {"containsRawConfigs": false}
	}`)
	in, err := DetectJSON(data)
	if err != nil {
		t.Fatalf("DetectJSON: %v", err)
	}
	if in.Kind != SourceWorkflowHistoryExport {
		t.Errorf("Kind = %q, want %q", in.Kind, SourceWorkflowHistoryExport)
	}
	if in.Runs != 2 {
		t.Errorf("Runs = %d, want 2", in.Runs)
	}
	if in.Workflow != "" || in.Result != "" {
		t.Errorf("history export should have no Workflow/Result, got %q/%q", in.Workflow, in.Result)
	}
}

func TestDetectJSON_EmptyHistoryExport(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"createdAt": "2026-06-14T10:15:30Z",
		"source": "workflow-history",
		"runCount": 0,
		"runs": [],
		"safety": {}
	}`)
	in, err := DetectJSON(data)
	if err != nil {
		t.Fatalf("DetectJSON: %v", err)
	}
	if in.Kind != SourceWorkflowHistoryExport {
		t.Errorf("Kind = %q", in.Kind)
	}
	if in.Runs != 0 {
		t.Errorf("Runs = %d, want 0", in.Runs)
	}
}

func TestDetectJSON_MalformedJSON(t *testing.T) {
	_, err := DetectJSON([]byte(`{ not valid json `))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestDetectJSON_Empty(t *testing.T) {
	if _, err := DetectJSON([]byte("   ")); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestDetectJSON_UnsupportedShape(t *testing.T) {
	// Valid JSON but not a recognizable artifact (e.g. a bare policy report).
	data := []byte(`{"findings": [], "summary": {}, "result": "passed-but-no-steps"}`)
	if _, err := DetectJSON(data); err == nil {
		t.Fatal("expected error for unsupported shape")
	}
}

func TestDetectJSON_RunReportNoSteps(t *testing.T) {
	data := []byte(`{"workflow": "x", "result": "passed", "steps": []}`)
	if _, err := DetectJSON(data); err == nil {
		t.Fatal("expected error for run report with no steps")
	}
}

func TestDetectJSON_PlanReportNoSteps(t *testing.T) {
	data := []byte(`{"schemaVersion": 1, "name": "x", "result": "valid", "steps": []}`)
	if _, err := DetectJSON(data); err == nil {
		t.Fatal("expected error for plan report with no steps")
	}
}

func TestDetectJSON_HistorySourceWithoutRuns(t *testing.T) {
	// `source: workflow-history` without runs/runCount is not a history export.
	data := []byte(`{"source": "workflow-history"}`)
	if _, err := DetectJSON(data); err == nil {
		t.Fatal("expected error: source alone is not a history export")
	}
}

func TestDetectBundleManifest(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"type": "workflow-report",
		"createdAt": "2026-06-14T10:15:30Z",
		"source": "workflow-run",
		"workflow": "policy-baseline-check",
		"result": "failed",
		"files": ["summary.txt", "report.json", "explanations.json"]
	}`)
	in, err := DetectBundleManifest(data)
	if err != nil {
		t.Fatalf("DetectBundleManifest: %v", err)
	}
	if in.Kind != SourceWorkflowReportBundle {
		t.Errorf("Kind = %q, want %q", in.Kind, SourceWorkflowReportBundle)
	}
	if in.Workflow != "policy-baseline-check" {
		t.Errorf("Workflow = %q", in.Workflow)
	}
	if in.Result != "failed" {
		t.Errorf("Result = %q", in.Result)
	}
}

func TestDetectBundleManifest_WrongType(t *testing.T) {
	data := []byte(`{"schemaVersion": 1, "type": "something-else"}`)
	if _, err := DetectBundleManifest(data); err == nil {
		t.Fatal("expected error for non-workflow-report manifest")
	}
}

func TestDetectBundleManifest_NoSchemaVersion(t *testing.T) {
	data := []byte(`{"type": "workflow-report"}`)
	if _, err := DetectBundleManifest(data); err == nil {
		t.Fatal("expected error for manifest without schemaVersion")
	}
}

func TestDetectBundleManifest_Malformed(t *testing.T) {
	if _, err := DetectBundleManifest([]byte("{bad")); err == nil {
		t.Fatal("expected error for malformed manifest")
	}
}

func TestDetectBundleManifest_Empty(t *testing.T) {
	if _, err := DetectBundleManifest([]byte("   ")); err == nil {
		t.Fatal("expected error for empty manifest")
	}
}

func TestDetectInput_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.json")
	writeFile(t, path, `{"workflow": "w", "result": "passed", "steps": [{"id":"s","type":"policy.check","status":"passed"}]}`)
	in, err := DetectInput(path)
	if err != nil {
		t.Fatalf("DetectInput: %v", err)
	}
	if in.Kind != SourceWorkflowRunReport {
		t.Errorf("Kind = %q", in.Kind)
	}
}

func TestDetectInput_BundleDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{"schemaVersion":1,"type":"workflow-report","workflow":"w","result":"failed"}`)
	in, err := DetectInput(dir)
	if err != nil {
		t.Fatalf("DetectInput: %v", err)
	}
	if in.Kind != SourceWorkflowReportBundle {
		t.Errorf("Kind = %q", in.Kind)
	}
}

func TestDetectInput_BundleDirNoManifest(t *testing.T) {
	dir := t.TempDir()
	if _, err := DetectInput(dir); err == nil {
		t.Fatal("expected error for directory without manifest.json")
	}
}

func TestDetectInput_MissingPath(t *testing.T) {
	if _, err := DetectInput(filepath.Join(t.TempDir(), "does-not-exist.json")); err == nil {
		t.Fatal("expected error for missing path")
	}
}
