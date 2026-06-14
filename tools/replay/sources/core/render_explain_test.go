package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderExplainHuman_Shape(t *testing.T) {
	e := BuildExplain("workflow-run-report", []string{"policy.lan_exposure_forbidden", "custom.future.code"})
	var buf bytes.Buffer
	RenderExplainHuman(&buf, e)
	out := buf.String()
	for _, want := range []string{
		"Portier Replay Explanations",
		"Source: workflow-run-report",
		"Codes: 2",
		"Known: 1",
		"Unknown: 1",
		"policy.lan_exposure_forbidden [warning]",
		"LAN exposure is forbidden",
		"Suggestion: Review the rule's listen host or use a policy that explicitly allows LAN exposure.",
		"custom.future.code [unknown]",
		"Unknown code",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderExplainHuman_NoCodes(t *testing.T) {
	e := BuildExplain("workflow-history-export", nil)
	var buf bytes.Buffer
	RenderExplainHuman(&buf, e)
	out := buf.String()
	if !strings.Contains(out, "Codes: 0") {
		t.Errorf("expected 'Codes: 0'\n%s", out)
	}
	if !strings.Contains(out, "No emitted codes were found in this artifact.") {
		t.Errorf("expected no-codes notice\n%s", out)
	}
	if strings.Contains(out, "Known:") {
		t.Errorf("no-codes output should not print Known/Unknown lines\n%s", out)
	}
}

func TestMarshalExplain_ValidAndArrays(t *testing.T) {
	e := BuildExplain("workflow-history-export", nil)
	data, err := MarshalExplain(e)
	if err != nil {
		t.Fatalf("MarshalExplain: %v", err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Error("MarshalExplain output should end with a newline")
	}
	if bytes.Contains(data, []byte(`"explanations": null`)) {
		t.Errorf("explanations must serialize as [] not null\n%s", data)
	}
	var decoded Explain
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if decoded.SchemaVersion != ExplainSchemaVersion {
		t.Errorf("decoded schemaVersion = %d", decoded.SchemaVersion)
	}
}
