package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderTimelineHuman_RunReportShape(t *testing.T) {
	data := []byte(`{
		"workflow": "policy-baseline-check", "result": "failed",
		"steps": [
			{"id":"check-config","type":"policy.check","status":"passed","exitCode":0,"message":"ok"},
			{"id":"compare-baseline","type":"policy.baseline.compare","status":"failed","exitCode":1,
				"report":{"new":[{"code":"policy.lan_exposure_forbidden"}]}}
		]
	}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	RenderTimelineHuman(&buf, tl)
	out := buf.String()
	for _, want := range []string{
		"Portier Replay Timeline",
		"Source: workflow-run-report",
		"Workflow: policy-baseline-check",
		"Result: failed",
		"1. workflow-start [started]",
		"Workflow replay timeline reconstructed from saved report.",
		"2. check-config [passed] policy.check",
		"   Exit code: 0",
		"3. compare-baseline [failed] policy.baseline.compare",
		"   Exit code: 1",
		"   Codes: policy.lan_exposure_forbidden",
		"4. workflow-result [failed]",
		"Summary:",
		"- 4 events",
		"- 1 passed",
		"- 1 failed",
		"- 0 skipped",
		"- 2 synthetic",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderTimelineHuman_PlanShowsValidInvalid(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1, "name": "p", "result": "invalid",
		"steps": [{"id":"a","type":"policy.review","status":"invalid","code":"workflow.step.missing_policy"}]
	}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	RenderTimelineHuman(&buf, tl)
	out := buf.String()
	if !strings.Contains(out, "- 1 invalid") {
		t.Errorf("plan timeline should show invalid count\n%s", out)
	}
}

func TestRenderTimelineHuman_HistoryShowsRunFields(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1, "source": "workflow-history", "runCount": 1, "runs": [
			{"id":"r1","createdAt":"2026-06-14T12:00:00Z","workflow":"w1","result":"failed",
				"summary":{"total":1,"passed":0,"failed":1,"skipped":0},"codes":["workflow.run.input_failed"]}
		], "safety": {}
	}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	RenderTimelineHuman(&buf, tl)
	out := buf.String()
	for _, want := range []string{
		"1. history-start [started]",
		"2. r1 [failed] workflow.run",
		"   Workflow: w1",
		"   Created: 2026-06-14T12:00:00Z",
		"   Codes: workflow.run.input_failed",
		"3. history-end [reconstructed]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("history timeline missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "Result:") {
		t.Errorf("history timeline should not show a top-level Result line\n%s", out)
	}
}

func TestMarshalTimeline_ValidAndNewlineTerminated(t *testing.T) {
	tl, err := TimelineJSON([]byte(`{"workflow":"w","result":"passed","steps":[{"id":"s","type":"t","status":"passed"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	data, err := MarshalTimeline(tl)
	if err != nil {
		t.Fatalf("MarshalTimeline: %v", err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Error("MarshalTimeline output should end with a newline")
	}
	var decoded Timeline
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if decoded.SchemaVersion != TimelineSchemaVersion {
		t.Errorf("decoded schemaVersion = %d", decoded.SchemaVersion)
	}
	if bytes.Contains(data, []byte(`"events": null`)) {
		t.Errorf("events should serialize as [] not null\n%s", data)
	}
	// Every event explicitly carries the synthetic marker.
	if !bytes.Contains(data, []byte(`"synthetic": true`)) || !bytes.Contains(data, []byte(`"synthetic": false`)) {
		t.Errorf("events must explicitly mark synthetic true/false\n%s", data)
	}
}
