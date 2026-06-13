package workflow

// White-box tests for the workflow support report: parsing/detecting an existing
// plan/run report, normalizing steps + extracting explainable codes, rendering the
// summary, and writing the bundle. All inputs are report JSON strings — these tests
// never produce a fresh evaluation and never touch a referenced file.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleRunReport = `{
  "workflow": "demo",
  "steps": [
    {"id":"c","type":"policy.check","status":"failed","exitCode":1,"message":"Policy check found 1 violation.",
     "report":{"findings":[{"code":"policy.lan_exposure_forbidden","severity":"error","title":"t","message":"m"}],"summary":{"info":0,"warning":0,"error":1},"result":"failed"}},
    {"id":"cmp","type":"policy.baseline.compare","status":"skipped","exitCode":1,"message":"Skipped: depends on step \"c\"."}
  ],
  "summary": {"total":2,"passed":0,"failed":1,"skipped":1},
  "result": "failed"
}`

const samplePassedRunReport = `{
  "workflow": "ok",
  "steps": [
    {"id":"c","type":"policy.check","status":"passed","exitCode":0,"message":"Policy check passed (no violations).",
     "report":{"findings":[{"code":"policy.valid","severity":"info","title":"t","message":"m"}],"summary":{"info":1,"warning":0,"error":0},"result":"passed"}}
  ],
  "summary": {"total":1,"passed":1,"failed":0,"skipped":0},
  "result": "passed"
}`

const sampleInvalidPlanReport = `{
  "schemaVersion": 1,
  "name": "bad",
  "steps": [
    {"id":"r","type":"policy.review","code":"workflow.step.missing_candidate","status":"invalid","message":"...","inputs":{},"dependsOn":[]}
  ],
  "summary": {"total":1,"valid":0,"invalid":1},
  "result": "invalid"
}`

func TestParseSupportReport_DetectsRun(t *testing.T) {
	r, err := ParseSupportReport([]byte(sampleRunReport))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Kind != ReportKindRun {
		t.Errorf("kind = %q, want %q", r.Kind, ReportKindRun)
	}
	if r.Workflow != "demo" || r.Result != "failed" || len(r.Steps) != 2 {
		t.Errorf("unexpected report: %+v", r)
	}
	// Failed policy.check → its error finding code; skipped → dependency_failed.
	if got := r.Steps[0].Codes; len(got) != 1 || got[0] != "policy.lan_exposure_forbidden" {
		t.Errorf("step 0 codes = %v", got)
	}
	if got := r.Steps[1].Codes; len(got) != 1 || got[0] != runCodeDependencyFailed {
		t.Errorf("step 1 codes = %v", got)
	}
}

func TestParseSupportReport_DetectsPlan(t *testing.T) {
	r, err := ParseSupportReport([]byte(sampleInvalidPlanReport))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Kind != ReportKindPlan {
		t.Errorf("kind = %q, want %q", r.Kind, ReportKindPlan)
	}
	if r.Workflow != "bad" || r.Result != "invalid" {
		t.Errorf("unexpected report: %+v", r)
	}
	if got := r.Steps[0].Codes; len(got) != 1 || got[0] != "workflow.step.missing_candidate" {
		t.Errorf("plan step codes = %v", got)
	}
}

func TestParseSupportReport_PassedRunHasNoStepCodes(t *testing.T) {
	r, err := ParseSupportReport([]byte(samplePassedRunReport))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Steps[0].Codes != nil {
		t.Errorf("passed step should carry no codes, got %v", r.Steps[0].Codes)
	}
	if len(r.ExplanationCodes()) != 0 {
		t.Errorf("passed run should have no explanation codes, got %v", r.ExplanationCodes())
	}
}

func TestParseSupportReport_Errors(t *testing.T) {
	cases := map[string]string{
		"empty":          ``,
		"malformed":      `{not json`,
		"no steps":       `{"workflow":"x","result":"failed","steps":[]}`,
		"policy report":  `{"findings":[],"summary":{},"result":"passed"}`,
		"unknown result": `{"workflow":"x","result":"weird","steps":[{"id":"a","type":"policy.check"}]}`,
	}
	for name, in := range cases {
		if _, err := ParseSupportReport([]byte(in)); err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		}
	}
}

func TestSupportReport_ExplanationsUsesCanonicalRegistryAndDedupes(t *testing.T) {
	r := SupportReport{
		Kind: ReportKindRun,
		Steps: []SupportReportStep{
			{ID: "a", Status: runStatusFailed, Codes: []string{"policy.lan_exposure_forbidden"}},
			{ID: "b", Status: runStatusFailed, Codes: []string{"policy.lan_exposure_forbidden"}}, // dup
			{ID: "c", Status: runStatusSkipped, Codes: []string{runCodeDependencyFailed}},
		},
		inputCodes: []string{runCodeRuntimeUnreachable, "policy.lan_exposure_forbidden", "not.a.real.code"},
	}
	exps := r.Explanations()
	// Deduped, canonical-only (the junk code is dropped).
	if len(exps) != 3 {
		t.Fatalf("explanations = %d, want 3 (%v)", len(exps), exps)
	}
	for _, want := range []string{"policy.lan_exposure_forbidden", runCodeDependencyFailed, runCodeRuntimeUnreachable} {
		if _, ok := exps[want]; !ok {
			t.Errorf("missing explanation %q", want)
		}
	}
	if _, ok := exps["not.a.real.code"]; ok {
		t.Errorf("unknown code must be dropped")
	}
}

func TestWriteSupportSummary_Shape(t *testing.T) {
	r, _ := ParseSupportReport([]byte(sampleRunReport))
	var sb strings.Builder
	WriteSupportSummary(r, &sb)
	out := sb.String()
	for _, want := range []string{
		"Portier Workflow Report",
		"Source: workflow-run",
		"Workflow: demo",
		"Result: failed",
		"[FAILED] c (policy.check)",
		"[SKIPPED] cmp (policy.baseline.compare)",
		"- policy.lan_exposure_forbidden",
		"- workflow.run.dependency_failed",
		"No runtime contact.",
		"No workflow execution.",
		"No logs, environment variables, or process data were collected.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestWriteSupportSummary_NoCodes(t *testing.T) {
	r, _ := ParseSupportReport([]byte(samplePassedRunReport))
	var sb strings.Builder
	WriteSupportSummary(r, &sb)
	if !strings.Contains(sb.String(), "Explanations:\n- none") {
		t.Errorf("expected '- none' for no explanations:\n%s", sb.String())
	}
}

func TestReportStatusTag_AllStatuses(t *testing.T) {
	cases := map[string]string{
		statusValid:      "[VALID]",
		statusInvalid:    "[INVALID]",
		runStatusPassed:  "[PASSED]",
		runStatusFailed:  "[FAILED]",
		runStatusSkipped: "[SKIPPED]",
		"weird":          "[weird]",
	}
	for status, want := range cases {
		if got := reportStatusTag(status); got != want {
			t.Errorf("reportStatusTag(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestEmbeddedFindingCodes_NewFindingsAndMalformed(t *testing.T) {
	// A failed baseline.compare report carries its failing findings under "new".
	withNew := `{"summary":{"new":1,"resolved":0,"unchanged":0},"result":"failed","new":[{"code":"policy.privileged_port_forbidden","fingerprint":"f"}]}`
	codes := embeddedFindingCodes([]byte(withNew))
	if len(codes) != 1 || codes[0] != "policy.privileged_port_forbidden" {
		t.Errorf("new-finding codes = %v", codes)
	}
	// A malformed or empty embedded report yields no codes (no panic).
	if got := embeddedFindingCodes([]byte(`{not json`)); got != nil {
		t.Errorf("malformed embedded report codes = %v, want nil", got)
	}
	if got := embeddedFindingCodes(nil); got != nil {
		t.Errorf("nil embedded report codes = %v, want nil", got)
	}
}

func TestParseSupportReport_FailedCompareNewFindings(t *testing.T) {
	report := `{
      "workflow":"w",
      "steps":[{"id":"cmp","type":"policy.baseline.compare","status":"failed","exitCode":1,"message":"1 new finding compared to the baseline.",
        "report":{"summary":{"new":1,"resolved":0,"unchanged":0},"result":"failed","new":[{"code":"policy.autostart_forbidden","fingerprint":"f"}]}}],
      "summary":{"total":1,"passed":0,"failed":1,"skipped":0},
      "result":"failed"
    }`
	r, err := ParseSupportReport([]byte(report))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := r.Steps[0].Codes; len(got) != 1 || got[0] != "policy.autostart_forbidden" {
		t.Errorf("failed compare codes = %v", got)
	}
}

func TestWriteSupportSummary_Placeholders(t *testing.T) {
	r := SupportReport{
		Kind:     ReportKindPlan,
		Workflow: "",
		Result:   "valid",
		Steps:    []SupportReportStep{{ID: "", Type: "", Status: statusValid}},
	}
	var sb strings.Builder
	WriteSupportSummary(r, &sb)
	out := sb.String()
	for _, want := range []string{"Workflow: (unnamed)", "[VALID] (no id) ((no type))"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestWriteSupportReport_WritesBundle(t *testing.T) {
	r, _ := ParseSupportReport([]byte(sampleRunReport))
	dir := t.TempDir()
	clock := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	manifest, err := WriteSupportReport(dir, r, clock)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if manifest.CreatedAt != "2026-06-13T12:00:00Z" {
		t.Errorf("createdAt = %q", manifest.CreatedAt)
	}
	if manifest.Type != "workflow-report" || manifest.Source != ReportKindRun || manifest.Result != "failed" {
		t.Errorf("manifest = %+v", manifest)
	}
	wantFiles := []string{"summary.txt", "report.json", "explanations.json"}
	if strings.Join(manifest.Files, ",") != strings.Join(wantFiles, ",") {
		t.Errorf("files = %v, want %v", manifest.Files, wantFiles)
	}
	for _, f := range append(wantFiles, "manifest.json") {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	// explanations.json with no codes is a stable empty object.
	r2, _ := ParseSupportReport([]byte(samplePassedRunReport))
	dir2 := t.TempDir()
	if _, err := WriteSupportReport(dir2, r2, clock); err != nil {
		t.Fatalf("write passed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir2, "explanations.json"))
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("explanations.json not valid JSON: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty explanations object, got %v", m)
	}
}
