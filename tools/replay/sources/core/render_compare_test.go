package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderCompareHuman_Shape(t *testing.T) {
	c := BuildCompare(facts(t, runPassed), facts(t, runFailed))
	var buf bytes.Buffer
	RenderCompareHuman(&buf, c)
	out := buf.String()
	for _, want := range []string{
		"Portier Replay Compare",
		"Left:",
		"- Source: workflow-run-report",
		"Right:",
		"Changes:",
		"- Result changed from passed to failed.",
		"- New emitted codes appeared.",
		"  - policy.lan_exposure_forbidden",
		"Insights:",
		"- Result changed from passed to failed.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderCompareHuman_NoChanges(t *testing.T) {
	c := BuildCompare(facts(t, runPassed), facts(t, runPassed))
	var buf bytes.Buffer
	RenderCompareHuman(&buf, c)
	out := buf.String()
	if !strings.Contains(out, "Changes:\n- None.") {
		t.Errorf("expected '- None.' under Changes\n%s", out)
	}
	if !strings.Contains(out, "- No meaningful differences detected.") {
		t.Errorf("expected no-difference insight\n%s", out)
	}
}

func TestRenderCompareHuman_RemovedItems(t *testing.T) {
	// failed -> passed yields removed codes/failed-steps; the removed items must
	// render as sub-bullets.
	c := BuildCompare(facts(t, runFailed), facts(t, runPassed))
	var buf bytes.Buffer
	RenderCompareHuman(&buf, c)
	out := buf.String()
	if !strings.Contains(out, "Some emitted codes are gone.") {
		t.Errorf("missing removed-codes change\n%s", out)
	}
	if !strings.Contains(out, "  - policy.lan_exposure_forbidden") {
		t.Errorf("removed code should render as a sub-bullet\n%s", out)
	}
	if !strings.Contains(out, "  - compare") {
		t.Errorf("removed failed step should render as a sub-bullet\n%s", out)
	}
}

func TestRenderCompareHuman_MixedKind(t *testing.T) {
	c := BuildCompare(facts(t, runPassed), facts(t, histPopulated))
	var buf bytes.Buffer
	RenderCompareHuman(&buf, c)
	out := buf.String()
	if !strings.Contains(out, "- Artifacts are different kinds; detailed comparison is limited.") {
		t.Errorf("missing mixed-kind change\n%s", out)
	}
	// the right side has no result/workflow lines for a history export
	if strings.Contains(out, "Right:\n- Source: workflow-history-export\n- Result:") {
		t.Errorf("history side should not print a Result line\n%s", out)
	}
}

func TestMarshalCompare_ValidAndArrays(t *testing.T) {
	c := BuildCompare(facts(t, runPassed), facts(t, runFailed))
	data, err := MarshalCompare(c)
	if err != nil {
		t.Fatalf("MarshalCompare: %v", err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Error("MarshalCompare output should end with a newline")
	}
	var decoded Compare
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if decoded.SchemaVersion != CompareSchemaVersion {
		t.Errorf("decoded schemaVersion = %d", decoded.SchemaVersion)
	}
	// changes/insights always arrays; set diff slices never null
	if bytes.Contains(data, []byte(`"changes": null`)) || bytes.Contains(data, []byte(`"insights": null`)) {
		t.Errorf("changes/insights must serialize as [] not null\n%s", data)
	}
	if bytes.Contains(data, []byte(`"added": null`)) || bytes.Contains(data, []byte(`"unchanged": null`)) {
		t.Errorf("set slices must serialize as [] not null\n%s", data)
	}
}
