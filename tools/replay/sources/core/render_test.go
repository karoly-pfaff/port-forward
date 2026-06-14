package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderHuman_Shape(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowRunReport, Workflow: "policy-baseline-check", Result: "failed", Steps: 2, HasCodes: true})
	var buf bytes.Buffer
	RenderHuman(&buf, p)
	out := buf.String()

	for _, want := range []string{
		"Portier Replay Plan",
		"Source: workflow-run-report",
		"Workflow: policy-baseline-check",
		"Result: failed",
		"Available:",
		"Unavailable:",
		"Summary:",
		"Replay analysis does not re-execute workflows.",
		"Replay analysis does not contact the runtime.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderHuman_OmitsWorkflowResultForHistoryExport(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowHistoryExport, Runs: 3})
	var buf bytes.Buffer
	RenderHuman(&buf, p)
	out := buf.String()
	if strings.Contains(out, "Workflow:") {
		t.Errorf("history export human output should not print Workflow line\n%s", out)
	}
	if strings.Contains(out, "Result:") {
		t.Errorf("history export human output should not print Result line\n%s", out)
	}
}

func TestRenderHuman_NoneWhenNoAvailable(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowHistoryExport, Runs: 0})
	var buf bytes.Buffer
	RenderHuman(&buf, p)
	out := buf.String()
	// The Available section should render "- none".
	avail := out[strings.Index(out, "Available:"):strings.Index(out, "Unavailable:")]
	if !strings.Contains(avail, "- none") {
		t.Errorf("expected '- none' in Available section\n%s", avail)
	}
}

func TestMarshalPlan_DeterministicAndValid(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowRunReport, Workflow: "w", Result: "failed", Steps: 2, HasCodes: true})
	data, err := MarshalPlan(p)
	if err != nil {
		t.Fatalf("MarshalPlan: %v", err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Error("MarshalPlan output should end with a newline")
	}

	var decoded Plan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if decoded.Source != SourceWorkflowRunReport {
		t.Errorf("decoded Source = %q", decoded.Source)
	}
	if decoded.SchemaVersion != PlanSchemaVersion {
		t.Errorf("decoded SchemaVersion = %d", decoded.SchemaVersion)
	}
}

func TestMarshalPlan_InputsAreArraysNotNull(t *testing.T) {
	p := BuildPlan(Input{Kind: SourceWorkflowRunReport, Workflow: "w", Result: "passed", Steps: 1, HasCodes: false})
	data, err := MarshalPlan(p)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"inputs": null`)) {
		t.Errorf("inputs should serialize as [] not null\n%s", data)
	}
}
