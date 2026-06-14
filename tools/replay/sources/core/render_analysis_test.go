package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderAnalysisHuman_RunReportShape(t *testing.T) {
	data := []byte(`{
		"workflow": "policy-baseline-check", "result": "failed",
		"steps": [
			{"id":"check","type":"policy.check","status":"passed"},
			{"id":"compare","type":"policy.baseline.compare","status":"failed",
				"report":{"findings":[{"code":"policy.lan_exposure_forbidden","severity":"error"}]}}
		]
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	RenderAnalysisHuman(&buf, a)
	out := buf.String()
	for _, want := range []string{
		"Portier Replay Analysis",
		"Source: workflow-run-report",
		"Workflow: policy-baseline-check",
		"Result: failed",
		"Step summary:",
		"- 2 total",
		"- 1 passed",
		"- 1 failed",
		"- 0 skipped",
		"Codes:",
		"- policy.lan_exposure_forbidden: 1",
		"Findings:",
		"- Workflow has failed steps.",
		"Insights:",
		"- Workflow failed because one or more steps failed.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderAnalysisHuman_PlanShowsValidInvalid(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1, "name": "p", "result": "invalid",
		"steps": [{"id":"a","type":"policy.review","status":"invalid","code":"workflow.step.missing_policy"}]
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	RenderAnalysisHuman(&buf, a)
	out := buf.String()
	if !strings.Contains(out, "- 1 invalid") {
		t.Errorf("plan output should show invalid count\n%s", out)
	}
	if strings.Contains(out, "passed") || strings.Contains(out, "skipped") {
		t.Errorf("plan output should not show run-style step counts\n%s", out)
	}
}

func TestRenderAnalysisHuman_HistoryShowsRunsAndWorkflows(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1, "source": "workflow-history", "runCount": 2, "runs": [
			{"id":"a","workflow":"w1","result":"passed"},
			{"id":"b","workflow":"w2","result":"failed","codes":["workflow.run.input_failed"]}
		], "safety": {}
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	RenderAnalysisHuman(&buf, a)
	out := buf.String()
	for _, want := range []string{"Runs:", "- 2 total", "- 1 passed", "- 1 failed", "Workflows:", "- w1: 1", "- w2: 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("history output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "Step summary:") {
		t.Errorf("history output should not show a step summary\n%s", out)
	}
}

func TestRenderAnalysisHuman_NoneSections(t *testing.T) {
	data := []byte(`{
		"workflow": "ok", "result": "passed",
		"steps": [{"id":"s","type":"policy.check","status":"passed"}]
	}`)
	a, err := AnalyzeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	RenderAnalysisHuman(&buf, a)
	out := buf.String()
	// Codes and Findings should both render "- none" for a clean passed run.
	codes := out[strings.Index(out, "Codes:"):strings.Index(out, "Findings:")]
	if !strings.Contains(codes, "- none") {
		t.Errorf("expected '- none' under Codes\n%s", out)
	}
	findings := out[strings.Index(out, "Findings:"):strings.Index(out, "Insights:")]
	if !strings.Contains(findings, "- none") {
		t.Errorf("expected '- none' under Findings\n%s", out)
	}
}

func TestMarshalAnalysis_ValidAndNewlineTerminated(t *testing.T) {
	a, err := AnalyzeJSON([]byte(`{"workflow":"w","result":"passed","steps":[{"id":"s","type":"t","status":"passed"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	data, err := MarshalAnalysis(a)
	if err != nil {
		t.Fatalf("MarshalAnalysis: %v", err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Error("MarshalAnalysis output should end with a newline")
	}
	var decoded Analysis
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if decoded.SchemaVersion != AnalysisSchemaVersion {
		t.Errorf("decoded schemaVersion = %d", decoded.SchemaVersion)
	}
	// codes/findings/insights must serialize as arrays, never null
	if bytes.Contains(data, []byte(`"codes": null`)) ||
		bytes.Contains(data, []byte(`"findings": null`)) ||
		bytes.Contains(data, []byte(`"insights": null`)) {
		t.Errorf("slices should serialize as [] not null\n%s", data)
	}
}
