package commands_test

// Black-box tests for `portier workflow plan --explain` (v1.11 Slice 2): inline
// explanations for invalid step validation codes, an additive JSON explanations
// map, and integration with `portier explain`. --explain must never change the
// steps, summary, result, or exit code.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"portier/cli/sources/commands"
	"portier/cli/sources/explain"
)

// invalidWorkflow has one valid step and one invalid step (unknown reportFrom).
const invalidWorkflow = `{
	"schemaVersion": 1,
	"name": "broken",
	"steps": [
		{"id": "chk", "type": "policy.check", "runtime": true, "policy": "p.json"},
		{"id": "cmp", "type": "policy.baseline.compare", "baseline": "b.json", "reportFrom": "nope"}
	]
}`

// planWithExplanations mirrors the workflow plan JSON plus the additive
// explanations map emitted under --explain.
type planWithExplanations struct {
	Steps []struct {
		ID     string `json:"id"`
		Code   string `json:"code"`
		Status string `json:"status"`
	} `json:"steps"`
	Summary struct {
		Total   int `json:"total"`
		Valid   int `json:"valid"`
		Invalid int `json:"invalid"`
	} `json:"summary"`
	Result       string                         `json:"result"`
	Explanations map[string]explain.Explanation `json:"explanations"`
}

func decodePlanWithExplanations(t *testing.T, out string) planWithExplanations {
	t.Helper()
	var p planWithExplanations
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("decoding plan JSON: %v\n%s", err, out)
	}
	return p
}

// --- stable codes in the plan ---

func TestWorkflowPlan_InvalidStepCarriesStableCode(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", invalidWorkflow)
	out, _, code := runWorkflowPlan(t, true, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	p := decodePlanWithExplanations(t, out)
	if p.Steps[0].Code != "workflow.step.valid" {
		t.Errorf("valid step code = %q, want workflow.step.valid", p.Steps[0].Code)
	}
	if p.Steps[1].Code != "workflow.step.unknown_report_from" {
		t.Errorf("invalid step code = %q, want workflow.step.unknown_report_from", p.Steps[1].Code)
	}
}

// --- human --explain ---

func TestWorkflowPlan_ExplainHumanInvalidStep(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", invalidWorkflow)
	out, _, code := runWorkflowPlan(t, false, wf, "--explain")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	for _, want := range []string{
		"Code: workflow.step.unknown_report_from",
		"Meaning:",
		"What to do:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human --explain missing %q:\n%s", want, out)
		}
	}
	// The valid step must not be explained.
	if strings.Contains(out, "Code: workflow.step.valid") {
		t.Errorf("valid step should not be explained:\n%s", out)
	}
}

func TestWorkflowPlan_NoExplainNoBlocks(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", invalidWorkflow)
	out, _, _ := runWorkflowPlan(t, false, wf)
	if strings.Contains(out, "Code:") || strings.Contains(out, "Meaning:") {
		t.Errorf("non-explain human output should have no explanation block:\n%s", out)
	}
}

// --- JSON --explain ---

func TestWorkflowPlan_JSONExplainAddsMap(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", invalidWorkflow)
	out, _, code := runWorkflowPlan(t, true, wf, "--explain")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	p := decodePlanWithExplanations(t, out)
	exp, ok := p.Explanations["workflow.step.unknown_report_from"]
	if !ok {
		t.Fatalf("explanations map missing the invalid code:\n%s", out)
	}
	if exp.Code != "workflow.step.unknown_report_from" || exp.Meaning == "" || exp.Action == "" {
		t.Errorf("explanation incomplete: %+v", exp)
	}
	// The valid step's code must NOT appear (only invalid steps are explained).
	if _, ok := p.Explanations["workflow.step.valid"]; ok {
		t.Errorf("explanations should not include workflow.step.valid:\n%s", out)
	}
}

func TestWorkflowPlan_JSONNoExplainOmitsMap(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", invalidWorkflow)
	out, _, _ := runWorkflowPlan(t, true, wf)
	if strings.Contains(out, "\"explanations\"") {
		t.Errorf("non-explain JSON must omit the explanations key:\n%s", out)
	}
}

func TestWorkflowPlan_JSONExplainValidPlanOmitsMap(t *testing.T) {
	// A fully valid plan has no invalid steps → nothing to explain → omitted.
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`)
	out, _, code := runWorkflowPlan(t, true, wf, "--explain")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(out, "\"explanations\"") {
		t.Errorf("valid-plan --explain must omit the explanations key:\n%s", out)
	}
}

// --- --explain does not change plan semantics ---

func TestWorkflowPlan_ExplainDoesNotChangePlan(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", invalidWorkflow)
	// Decode both into workflow.Plan (which ignores the additive explanations
	// map) and deep-compare: steps, summary, and result must be identical.
	plain, codePlain := runWorkflowPlanJSON(t, wf)
	explained, codeExplained := runWorkflowPlanJSON(t, wf, "--explain")
	if codePlain != codeExplained {
		t.Fatalf("exit code changed with --explain: %d vs %d", codePlain, codeExplained)
	}
	if !reflect.DeepEqual(plain, explained) {
		t.Errorf("--explain changed the plan body:\nplain:    %+v\nexplained:%+v", plain, explained)
	}
}

// --- --out --explain ---

func TestWorkflowPlan_OutExplainWritesExplanations(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", invalidWorkflow)
	outPath := filepath.Join(t.TempDir(), "plan.json")
	_, _, code := runWorkflowPlan(t, false, wf, "--explain", "--out", outPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading out file: %v", err)
	}
	p := decodePlanWithExplanations(t, string(data))
	if _, ok := p.Explanations["workflow.step.unknown_report_from"]; !ok {
		t.Errorf("out file missing explanations map:\n%s", data)
	}
}

func TestWorkflowPlan_JSONOutExplainByteIdentical(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", invalidWorkflow)
	outPath := filepath.Join(t.TempDir(), "plan.json")
	stdout, _, code := runWorkflowPlan(t, true, wf, "--explain", "--out", outPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	fileBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading out file: %v", err)
	}
	if stdout != string(fileBytes) {
		t.Errorf("--json --out --explain stdout and file differ:\nstdout:\n%q\nfile:\n%q", stdout, string(fileBytes))
	}
}

// --- explain command integration ---

func TestExplain_WorkflowCode(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"workflow.step.unknown_report_from"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "workflow.step.unknown_report_from") {
		t.Errorf("explain output missing the code:\n%s", out.String())
	}
}

func TestExplain_WorkflowCodeJSON(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(true, []string{"workflow.step.future_report_from"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var exp explain.Explanation
	if err := json.Unmarshal([]byte(out.String()), &exp); err != nil {
		t.Fatalf("decoding explanation JSON: %v\n%s", err, out.String())
	}
	if exp.Code != "workflow.step.future_report_from" || exp.Severity != "error" {
		t.Errorf("explanation = %+v", exp)
	}
}

func TestExplain_ListIncludesWorkflowCodes(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"--list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"workflow.step.valid",
		"workflow.step.duplicate_id",
		"workflow.step.unknown_report_from",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("--list missing workflow code %q:\n%s", want, out.String())
		}
	}
	// Existing domains still listed (no regression).
	for _, want := range []string{"config.valid", "policy.lan_exposure_forbidden"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("--list missing existing code %q (regression)", want)
		}
	}
}

func TestExplain_UnknownWorkflowCodeExit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunExplain(false, []string{"workflow.step.does_not_exist"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
