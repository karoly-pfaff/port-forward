package commands_test

// Black-box tests for `portier workflow runbook` (v1.11 Slice 4): a deterministic,
// fully offline command-preview built from a VALID workflow plan. It never
// executes a command, contacts the runtime, reads the files a step refers to, or
// mutates any file except the requested --out file.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

// runbookJSON mirrors the runbook JSON shape for decoding in tests.
type runbookJSON struct {
	Workflow string `json:"workflow"`
	Steps    []struct {
		ID      string   `json:"id"`
		Type    string   `json:"type"`
		Command []string `json:"command"`
		Display string   `json:"display"`
		Notes   []string `json:"notes"`
	} `json:"steps"`
	Summary struct {
		Total int `json:"total"`
	} `json:"summary"`
	Result string `json:"result"`
}

// runWorkflowRunbook runs `workflow runbook --file <file>` (plus extra args) and
// returns stdout, stderr, and the exit code.
func runWorkflowRunbook(t *testing.T, jsonOutput bool, file string, extra ...string) (string, string, int) {
	t.Helper()
	args := append([]string{"--file", file}, extra...)
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRunbook(jsonOutput, args, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

func decodeRunbook(t *testing.T, out string) runbookJSON {
	t.Helper()
	var rb runbookJSON
	if err := json.Unmarshal([]byte(out), &rb); err != nil {
		t.Fatalf("decoding runbook JSON: %v\n%s", err, out)
	}
	return rb
}

// --- step-type mappings ---

func TestWorkflowRunbook_PolicyCheckConfig(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"name":"w","steps":[{"id":"c","type":"policy.check","config":"portier.json","policy":"p.json"}]}`)
	rb, code := func() (runbookJSON, int) {
		out, _, c := runWorkflowRunbook(t, true, wf)
		return decodeRunbook(t, out), c
	}()
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if rb.Result != "ready" || rb.Summary.Total != 1 {
		t.Fatalf("unexpected runbook: %+v", rb)
	}
	if rb.Steps[0].Display != "portier policy check --config portier.json --policy p.json" {
		t.Errorf("display = %q", rb.Steps[0].Display)
	}
}

func TestWorkflowRunbook_PolicyCheckRuntime(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","runtime":true,"policy":"p.json"}]}`)
	out, _, code := runWorkflowRunbook(t, true, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rb := decodeRunbook(t, out)
	if rb.Steps[0].Display != "portier policy check --runtime --policy p.json" {
		t.Errorf("display = %q", rb.Steps[0].Display)
	}
}

func TestWorkflowRunbook_PolicyReview(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[{"id":"r","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"p.json"}]}`)
	out, _, code := runWorkflowRunbook(t, true, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rb := decodeRunbook(t, out)
	if rb.Steps[0].Display != "portier policy review --current cur.json --candidate cand.json --policy p.json" {
		t.Errorf("display = %q", rb.Steps[0].Display)
	}
}

func TestWorkflowRunbook_BaselineCompareReportFile(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[{"id":"b","type":"policy.baseline.compare","baseline":"b.json","report":"r.json"}]}`)
	out, _, code := runWorkflowRunbook(t, true, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rb := decodeRunbook(t, out)
	if rb.Steps[0].Display != "portier policy baseline compare --baseline b.json --report r.json" {
		t.Errorf("display = %q", rb.Steps[0].Display)
	}
	if len(rb.Steps[0].Notes) != 0 {
		t.Errorf("report-file step should have no notes: %v", rb.Steps[0].Notes)
	}
}

func TestWorkflowRunbook_BaselineCompareReportFromPlaceholder(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[
		{"id":"chk","type":"policy.check","runtime":true,"policy":"p.json"},
		{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"chk"}
	]}`)
	out, _, code := runWorkflowRunbook(t, true, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rb := decodeRunbook(t, out)
	cmp := rb.Steps[1]
	last := cmp.Command[len(cmp.Command)-1]
	if last != "<report-from:chk>" {
		t.Errorf("expected placeholder report token, got %q (%v)", last, cmp.Command)
	}
	if len(cmp.Notes) != 1 || !strings.Contains(cmp.Notes[0], "chk") {
		t.Errorf("expected a note referencing the producing step: %v", cmp.Notes)
	}
}

func TestWorkflowRunbook_MultiStepOrdering(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[
		{"id":"one","type":"policy.check","config":"c.json","policy":"p.json"},
		{"id":"two","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"p.json"},
		{"id":"three","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"one"}
	]}`)
	out, _, code := runWorkflowRunbook(t, true, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rb := decodeRunbook(t, out)
	if rb.Summary.Total != 3 {
		t.Fatalf("total = %d, want 3", rb.Summary.Total)
	}
	if rb.Steps[0].ID != "one" || rb.Steps[1].ID != "two" || rb.Steps[2].ID != "three" {
		t.Errorf("ordering wrong: %s/%s/%s", rb.Steps[0].ID, rb.Steps[1].ID, rb.Steps[2].ID)
	}
}

// --- human output ---

func TestWorkflowRunbook_HumanShape(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"name":"demo","steps":[{"id":"c","type":"policy.check","config":"portier.json","policy":"p.json"}]}`)
	out, _, code := runWorkflowRunbook(t, false, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"Portier Workflow Runbook",
		"Workflow: demo",
		"1. c",
		"portier policy check --config portier.json --policy p.json",
		"1 step",
		"Result: ready",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human runbook missing %q:\n%s", want, out)
		}
	}
}

// --- invalid / error paths ---

func TestWorkflowRunbook_InvalidPlanExit1NoRunbook(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"x"}]}`)
	out, _, code := runWorkflowRunbook(t, false, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	// The plan (validation errors) is shown — NOT a ready runbook.
	if strings.Contains(out, "Result: ready") || strings.Contains(out, "Portier Workflow Runbook") {
		t.Errorf("invalid workflow must not produce a ready runbook:\n%s", out)
	}
	if !strings.Contains(out, "Result: invalid") {
		t.Errorf("expected the plan's invalid result:\n%s", out)
	}
}

func TestWorkflowRunbook_InvalidPlanDoesNotWriteOut(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"x"}]}`)
	outPath := filepath.Join(t.TempDir(), "runbook.json")
	_, _, code := runWorkflowRunbook(t, false, wf, "--out", outPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("invalid workflow must not write the --out runbook file (stat err=%v)", err)
	}
}

func TestWorkflowRunbook_MalformedExit2(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{not json`)
	_, _, code := runWorkflowRunbook(t, false, wf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowRunbook_UnsupportedSchemaExit2(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":2,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`)
	_, _, code := runWorkflowRunbook(t, false, wf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowRunbook_MissingFileFlagExit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRunbook(false, nil, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "--file is required") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestWorkflowRunbook_UnreadableFileExit2(t *testing.T) {
	_, _, code := runWorkflowRunbook(t, false, filepath.Join(t.TempDir(), "missing.json"))
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowRunbook_MissingOutValueExit2(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","runtime":true,"policy":"p.json"}]}`)
	_, _, code := runWorkflowRunbook(t, false, wf, "--out")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowRunbook_HelpExit0(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRunbook(false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier workflow runbook") {
		t.Errorf("help missing:\n%s", out.String())
	}
}

// --- --out ---

func TestWorkflowRunbook_OutWritesFile(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","config":"portier.json","policy":"p.json"}]}`)
	outPath := filepath.Join(t.TempDir(), "runbook.json")
	stdout, _, code := runWorkflowRunbook(t, false, wf, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Runbook written to "+outPath) {
		t.Errorf("missing write confirmation:\n%s", stdout)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading out file: %v", err)
	}
	rb := decodeRunbook(t, string(data))
	if rb.Result != "ready" {
		t.Errorf("out file result = %s, want ready", rb.Result)
	}
}

func TestWorkflowRunbook_JSONOutByteIdentical(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"name":"demo","steps":[
		{"id":"chk","type":"policy.check","runtime":true,"policy":"p.json"},
		{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"chk"}
	]}`)
	outPath := filepath.Join(t.TempDir(), "runbook.json")
	stdout, _, code := runWorkflowRunbook(t, true, wf, "--out", outPath)
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

func TestWorkflowRunbook_OutWriteFailureExit1(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","runtime":true,"policy":"p.json"}]}`)
	badOut := filepath.Join(t.TempDir(), "nope", "runbook.json")
	_, errOut, code := runWorkflowRunbook(t, false, wf, "--out", badOut)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (write failure)", code)
	}
	if !strings.Contains(errOut, "Error writing") {
		t.Errorf("stderr = %q", errOut)
	}
}

// --- safety: no file mutation, does not read referenced files ---

func TestWorkflowRunbook_DoesNotMutateWorkflowOrReadReferences(t *testing.T) {
	// The referenced config/policy/baseline files do NOT exist. The runbook must
	// still be produced (it never opens them), and the workflow file is unchanged.
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[
		{"id":"c","type":"policy.check","config":"does-not-exist.json","policy":"missing.json"}
	]}`)
	before, _ := os.ReadFile(wf)
	out, _, code := runWorkflowRunbook(t, true, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (runbook must not open referenced files)", code)
	}
	rb := decodeRunbook(t, out)
	if rb.Result != "ready" {
		t.Errorf("result = %s, want ready", rb.Result)
	}
	after, _ := os.ReadFile(wf)
	if string(before) != string(after) {
		t.Errorf("workflow file was mutated")
	}
}

// --- templates produce runbooks ---

func TestWorkflowRunbook_FromTemplates(t *testing.T) {
	for _, name := range []string{"policy-check-local", "policy-check-runtime", "policy-review", "policy-baseline-check"} {
		t.Run(name, func(t *testing.T) {
			// Generate the template to a file, then runbook it end-to-end.
			tmplPath := filepath.Join(t.TempDir(), "wf.json")
			var to, te strings.Builder
			if c := commands.RunWorkflowTemplate(false, []string{name, "--out", tmplPath}, &to, &te); c != 0 {
				t.Fatalf("template %s exit %d: %s", name, c, te.String())
			}
			out, _, code := runWorkflowRunbook(t, true, tmplPath)
			if code != 0 {
				t.Fatalf("runbook from template %s exit %d", name, code)
			}
			rb := decodeRunbook(t, out)
			if rb.Result != "ready" || rb.Summary.Total < 1 {
				t.Errorf("template %s runbook wrong: %+v", name, rb)
			}
		})
	}
}

// --- dispatch ---

func TestRunWorkflow_RunbookDispatch(t *testing.T) {
	wf := writeTempFile(t, "w.json", `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","runtime":true,"policy":"p.json"}]}`)
	var out, errBuf strings.Builder
	code := commands.RunWorkflow(false, commands.ConnFlags{}, []string{"runbook", "--file", wf}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Portier Workflow Runbook") {
		t.Errorf("runbook dispatch output wrong:\n%s", out.String())
	}
}
