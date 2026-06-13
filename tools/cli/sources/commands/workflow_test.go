package commands_test

// Black-box tests for `portier workflow plan` (v1.11 Slice 1): a fully offline,
// dry-run command that reads a local workflow file, validates its schema and step
// references, and prints a deterministic plan. It never executes a step, never
// contacts the runtime, and never mutates any file except the requested --out
// file.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
	"portier/cli/sources/workflow"
)

// --- shared fixtures ---

const validWorkflow = `{
	"schemaVersion": 1,
	"name": "local-policy-check",
	"steps": [
		{"id": "check-current", "type": "policy.check", "config": "portier.json", "policy": "local-safe.policy.json"},
		{"id": "compare-baseline", "type": "policy.baseline.compare", "baseline": "policy-baseline.json", "reportFrom": "check-current"}
	]
}`

// runWorkflowPlan runs `workflow plan --file <file>` (plus any extra args) and
// returns stdout, stderr, and the exit code.
func runWorkflowPlan(t *testing.T, jsonOutput bool, file string, extra ...string) (string, string, int) {
	t.Helper()
	args := append([]string{"--file", file}, extra...)
	var out, errBuf strings.Builder
	code := commands.RunWorkflowPlan(jsonOutput, args, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// runWorkflowPlanJSON runs `workflow plan --json` and decodes the plan.
func runWorkflowPlanJSON(t *testing.T, file string, extra ...string) (workflow.Plan, int) {
	t.Helper()
	out, errBuf, code := runWorkflowPlan(t, true, file, extra...)
	var plan workflow.Plan
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("decoding workflow JSON: %v\nstdout:\n%s\nstderr:\n%s", err, out, errBuf)
	}
	return plan, code
}

// --- valid workflows ---

func TestWorkflowPlan_ValidExit0(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", validWorkflow)
	out, _, code := runWorkflowPlan(t, false, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "Result: valid") {
		t.Errorf("expected valid result:\n%s", out)
	}
}

func TestWorkflowPlan_ValidPolicyCheckRuntimeMarker(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`)
	plan, code := runWorkflowPlanJSON(t, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if plan.Steps[0].Inputs["runtime"] != "true" {
		t.Errorf("runtime marker missing: %v", plan.Steps[0].Inputs)
	}
}

func TestWorkflowPlan_ValidPolicyReview(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"p.json"}]}`)
	_, code := runWorkflowPlanJSON(t, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestWorkflowPlan_ValidBaselineCompareReportFile(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","baseline":"b.json","report":"r.json"}]}`)
	plan, code := runWorkflowPlanJSON(t, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(plan.Steps[0].DependsOn) != 0 {
		t.Errorf("report-file compare should have no dependency: %v", plan.Steps[0].DependsOn)
	}
}

func TestWorkflowPlan_ValidBaselineCompareReportFrom(t *testing.T) {
	plan, code := runWorkflowPlanJSON(t, writeTempFile(t, "workflow.json", validWorkflow))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	cmp := plan.Steps[1]
	if len(cmp.DependsOn) != 1 || cmp.DependsOn[0] != "check-current" {
		t.Errorf("dependsOn = %v, want [check-current]", cmp.DependsOn)
	}
}

// --- invalid plans (exit 1) ---

func TestWorkflowPlan_DuplicateIDExit1(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[
		{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"},
		{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}
	]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (invalid plan)", code)
	}
}

func TestWorkflowPlan_UnknownTypeExit1(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.apply"}]}`)
	plan, code := runWorkflowPlanJSON(t, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if plan.Result != "invalid" {
		t.Errorf("result = %s, want invalid", plan.Result)
	}
}

func TestWorkflowPlan_MissingRequiredFieldExit1(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","config":"c.json"}]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestWorkflowPlan_ReportFromUnknownExit1(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"nope"}]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestWorkflowPlan_ReportFromLaterStepExit1(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[
		{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"chk"},
		{"id":"chk","type":"policy.check","runtime":true,"policy":"p.json"}
	]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestWorkflowPlan_ReportAndReportFromExit1(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[
		{"id":"chk","type":"policy.check","runtime":true,"policy":"p.json"},
		{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","report":"r.json","reportFrom":"chk"}
	]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestWorkflowPlan_ConfigAndRuntimeExit1(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","config":"c.json","runtime":true,"policy":"p.json"}]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestWorkflowPlan_NeitherConfigNorRuntimeExit1(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","policy":"p.json"}]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

// --- usage / malformed input (exit 2) ---

func TestWorkflowPlan_MissingFileFlagExit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflowPlan(false, nil, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "--file is required") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestWorkflowPlan_UnreadableFileExit2(t *testing.T) {
	_, _, code := runWorkflowPlan(t, false, filepath.Join(t.TempDir(), "missing.json"))
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowPlan_MalformedExit2(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{not json`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowPlan_UnknownFieldExit2(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json","bogus":1}]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowPlan_UnsupportedSchemaVersionExit2(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":2,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowPlan_EmptyStepsExit2(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowPlan_HelpFlagExit0(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflowPlan(false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier workflow plan") {
		t.Errorf("help missing usage:\n%s", out.String())
	}
}

func TestWorkflowPlan_BadFlagExit2(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", validWorkflow)
	_, _, code := runWorkflowPlan(t, false, wf, "--nope")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// --- JSON output shape ---

func TestWorkflowPlan_JSONShape(t *testing.T) {
	plan, code := runWorkflowPlanJSON(t, writeTempFile(t, "workflow.json", validWorkflow))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if plan.SchemaVersion != 1 || plan.Name != "local-policy-check" {
		t.Errorf("unexpected plan header: %+v", plan)
	}
	if plan.Summary.Total != 2 || plan.Summary.Valid != 2 || plan.Summary.Invalid != 0 {
		t.Errorf("summary = %+v", plan.Summary)
	}
	if plan.Result != "valid" {
		t.Errorf("result = %s, want valid", plan.Result)
	}
	if plan.Steps[0].Status != "valid" || plan.Steps[0].Inputs == nil {
		t.Errorf("step 0 = %+v", plan.Steps[0])
	}
}

// --- --out behavior ---

func TestWorkflowPlan_OutWritesFile(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", validWorkflow)
	outPath := filepath.Join(t.TempDir(), "plan.json")
	stdout, _, code := runWorkflowPlan(t, false, wf, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Report written to "+outPath) {
		t.Errorf("missing write confirmation:\n%s", stdout)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading out file: %v", err)
	}
	var plan workflow.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatalf("out file is not valid JSON: %v\n%s", err, data)
	}
	if plan.Result != "valid" {
		t.Errorf("out file result = %s, want valid", plan.Result)
	}
}

func TestWorkflowPlan_JSONOutByteIdentical(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", validWorkflow)
	outPath := filepath.Join(t.TempDir(), "plan.json")
	stdout, _, code := runWorkflowPlan(t, true, wf, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	fileBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading out file: %v", err)
	}
	if stdout != string(fileBytes) {
		t.Errorf("--json --out stdout and file differ:\nstdout:\n%q\nfile:\n%q", stdout, string(fileBytes))
	}
}

func TestWorkflowPlan_OutWriteFailureExit1(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", validWorkflow)
	// A path whose parent directory does not exist → write failure (not created).
	badOut := filepath.Join(t.TempDir(), "nope", "plan.json")
	_, errOut, code := runWorkflowPlan(t, false, wf, "--out", badOut)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (write failure)", code)
	}
	if !strings.Contains(errOut, "Error writing") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestWorkflowPlan_MissingOutValueExit2(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", validWorkflow)
	// "--out" with no value: flag parsing fails → exit 2.
	_, _, code := runWorkflowPlan(t, false, wf, "--out")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// --- safety: no file mutation, no runtime access ---

func TestWorkflowPlan_DoesNotMutateInputs(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", validWorkflow)
	before, err := os.ReadFile(wf)
	if err != nil {
		t.Fatalf("reading workflow: %v", err)
	}
	if _, _, code := runWorkflowPlan(t, false, wf); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	after, err := os.ReadFile(wf)
	if err != nil {
		t.Fatalf("re-reading workflow: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("workflow file was mutated")
	}
}

func TestWorkflowPlan_DoesNotReadReferencedFiles(t *testing.T) {
	// The referenced config/policy/baseline files do NOT exist. Planning must
	// still succeed (it validates references, it does not open the files).
	wf := writeTempFile(t, "workflow.json", `{"schemaVersion":1,"steps":[
		{"id":"a","type":"policy.check","config":"does-not-exist.json","policy":"also-missing.json"}
	]}`)
	_, _, code := runWorkflowPlan(t, false, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (planning must not open referenced files)", code)
	}
}

// --- dispatch ---

func TestRunWorkflow_NoSubcommandExit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflow(false, nil, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRunWorkflow_HelpExit0(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflow(false, []string{"help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier workflow") {
		t.Errorf("help missing:\n%s", out.String())
	}
}

func TestRunWorkflow_UnknownSubcommandExit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflow(false, []string{"run"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "Unknown workflow subcommand") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestRunWorkflow_PlanDispatch(t *testing.T) {
	wf := writeTempFile(t, "workflow.json", validWorkflow)
	var out, errBuf strings.Builder
	code := commands.RunWorkflow(false, []string{"plan", "--file", wf}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, out.String())
	}
}
