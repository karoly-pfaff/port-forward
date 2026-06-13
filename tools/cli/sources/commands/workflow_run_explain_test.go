package commands_test

// Black-box tests for `portier workflow run --explain` (v1.11 Slice 6): additive
// explanations for failed/skipped steps. --explain must NOT change which steps run,
// their status, the run result, or the exit code — it only adds inline human
// blocks and an additive JSON `explanations` map. Passed steps are not explained.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

// runExplainResult decodes a run report's result/summary/steps plus the additive
// explanations map (present only under --explain).
type runExplainResult struct {
	Steps []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"steps"`
	Summary struct {
		Total, Passed, Failed, Skipped int
	} `json:"summary"`
	Result       string `json:"result"`
	Explanations map[string]struct {
		Code     string `json:"code"`
		Title    string `json:"title"`
		Meaning  string `json:"meaning"`
		Action   string `json:"action"`
		Severity string `json:"severity"`
	} `json:"explanations"`
}

func decodeRunExplain(t *testing.T, out string) runExplainResult {
	t.Helper()
	var r runExplainResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("decoding run JSON: %v\n%s", err, out)
	}
	return r
}

// --- failing steps surface the right codes ---

func TestWorkflowRunExplain_PolicyCheckFindingCodes(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfDirtyConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})

	// Human: the inline block names the policy code + meaning.
	out, _, code := runWorkflowRun(t, false, wf, "--explain")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "Code: policy.lan_exposure_forbidden") || !strings.Contains(out, "Meaning:") {
		t.Errorf("human --explain missing policy finding explanation:\n%s", out)
	}

	// JSON: the additive explanations map carries the policy code.
	jout, _, _ := runWorkflowRun(t, true, wf, "--explain")
	r := decodeRunExplain(t, jout)
	if _, ok := r.Explanations["policy.lan_exposure_forbidden"]; !ok {
		t.Errorf("JSON explanations missing policy.lan_exposure_forbidden: %+v", r.Explanations)
	}
}

func TestWorkflowRunExplain_PolicyReviewFindingCodes(t *testing.T) {
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	cur := writeTempFile(t, "cur.json", wfCleanConfig)
	dirtyCand := writeTempFile(t, "dcand.json", wfDirtyConfig)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "r", "type": "policy.review", "current": cur, "candidate": dirtyCand, "policy": pol}})
	jout, _, code := runWorkflowRun(t, true, wf, "--explain")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, jout)
	}
	r := decodeRunExplain(t, jout)
	if _, ok := r.Explanations["policy.lan_exposure_forbidden"]; !ok {
		t.Errorf("review --explain missing policy finding code: %+v", r.Explanations)
	}
}

func TestWorkflowRunExplain_BaselineNewFindingCodes(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfDirtyConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	baseline := writeTempFile(t, "b.json", wfEmptyBaseline)
	wf := writeWorkflowFile(t, "policy-baseline-check", []map[string]any{
		{"id": "check-config", "type": "policy.check", "config": cfg, "policy": pol},
		{"id": "compare-baseline", "type": "policy.baseline.compare", "baseline": baseline, "reportFrom": "check-config"},
	})
	jout, _, code := runWorkflowRun(t, true, wf, "--explain")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, jout)
	}
	r := decodeRunExplain(t, jout)
	// The failing check-config emits the policy code; the failing baseline compare
	// surfaces the SAME underlying finding code (deduped in the map).
	if _, ok := r.Explanations["policy.lan_exposure_forbidden"]; !ok {
		t.Errorf("baseline --explain missing underlying finding code: %+v", r.Explanations)
	}
}

// --- workflow.run.* codes ---

func TestWorkflowRunExplain_DependencySkipCode(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfCleanConfig)
	baseline := writeTempFile(t, "b.json", wfEmptyBaseline)
	wf := writeWorkflowFile(t, "w", []map[string]any{
		{"id": "check-config", "type": "policy.check", "config": cfg, "policy": filepath.Join(t.TempDir(), "missing.json")},
		{"id": "compare-baseline", "type": "policy.baseline.compare", "baseline": baseline, "reportFrom": "check-config"},
	})
	jout, _, code := runWorkflowRun(t, true, wf, "--explain")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, jout)
	}
	r := decodeRunExplain(t, jout)
	// check-config failed on a missing policy file → input_failed; the dependent
	// compare was skipped → dependency_failed.
	if _, ok := r.Explanations["workflow.run.input_failed"]; !ok {
		t.Errorf("missing workflow.run.input_failed: %+v", r.Explanations)
	}
	if _, ok := r.Explanations["workflow.run.dependency_failed"]; !ok {
		t.Errorf("missing workflow.run.dependency_failed: %+v", r.Explanations)
	}

	// Human: the skipped step gets the dependency explanation block.
	hout, _, _ := runWorkflowRun(t, false, wf, "--explain")
	if !strings.Contains(hout, "Code: workflow.run.dependency_failed") {
		t.Errorf("human --explain missing dependency code:\n%s", hout)
	}
}

func TestWorkflowRunExplain_InputFailedCode(t *testing.T) {
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{
		{"id": "c", "type": "policy.check", "config": filepath.Join(t.TempDir(), "missing-config.json"), "policy": pol},
	})
	jout, _, code := runWorkflowRun(t, true, wf, "--explain")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, jout)
	}
	r := decodeRunExplain(t, jout)
	if _, ok := r.Explanations["workflow.run.input_failed"]; !ok {
		t.Errorf("missing workflow.run.input_failed: %+v", r.Explanations)
	}
}

func TestWorkflowRunExplain_RuntimeUnreachableCode(t *testing.T) {
	srv := makeExportServer(t, compliantRuntimeRules)
	deadURL := srv.URL
	srv.Close() // connections refused
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "runtime": true, "policy": pol}})
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRun(true, commands.ConnFlags{URL: deadURL}, []string{"--file", wf, "--explain"}, &out, &errBuf)
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (runtime unreachable)\n%s", code, out.String())
	}
	r := decodeRunExplain(t, out.String())
	if _, ok := r.Explanations["workflow.run.runtime_unreachable"]; !ok {
		t.Errorf("missing workflow.run.runtime_unreachable: %+v", r.Explanations)
	}
}

// --- passed steps are not explained ---

func TestWorkflowRunExplain_PassedStepNotNoisy(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfCleanConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})

	// JSON: a passed run emits NO explanations map (omitempty), even with --explain.
	jout, _, code := runWorkflowRun(t, true, wf, "--explain")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(jout, "\"explanations\"") {
		t.Errorf("passed run with --explain must not emit an explanations map:\n%s", jout)
	}

	// Human: no inline explanation blocks under a passed step.
	hout, _, _ := runWorkflowRun(t, false, wf, "--explain")
	if strings.Contains(hout, "Meaning:") || strings.Contains(hout, "Code: ") {
		t.Errorf("passed run with --explain must not print explanation blocks:\n%s", hout)
	}
}

// --- --explain does not change semantics ---

func TestWorkflowRunExplain_DoesNotChangeRun(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfDirtyConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})

	plain, _, codePlain := runWorkflowRun(t, true, wf)
	explained, _, codeExplain := runWorkflowRun(t, true, wf, "--explain")
	if codePlain != codeExplain {
		t.Fatalf("exit codes differ: plain=%d explain=%d", codePlain, codeExplain)
	}
	// Compare everything except the additive explanations map.
	a := decodeRun(t, plain)
	b := decodeRun(t, explained)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("--explain changed the run report:\nplain=%+v\nexplain=%+v", a, b)
	}
}

// --- --json --out --explain byte parity ---

func TestWorkflowRunExplain_JSONOutByteParity(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfDirtyConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})
	outPath := filepath.Join(t.TempDir(), "report.json")
	stdout, _, code := runWorkflowRun(t, true, wf, "--explain", "--out", outPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	fileBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading out file: %v", err)
	}
	if stdout != string(fileBytes) {
		t.Errorf("--json --out --explain stdout/file differ:\nstdout=%q\nfile=%q", stdout, string(fileBytes))
	}
	// And the file carries the explanations map.
	if !strings.Contains(string(fileBytes), "\"explanations\"") {
		t.Errorf("--out file missing explanations map:\n%s", fileBytes)
	}
}

// --- explain command integration ---

func TestWorkflowRunExplain_ExplainCommandKnowsRunCodes(t *testing.T) {
	for _, code := range []string{"workflow.run.dependency_failed", "workflow.run.runtime_unreachable", "workflow.run.input_failed"} {
		var out, errBuf strings.Builder
		if rc := commands.RunExplain(false, []string{code}, &out, &errBuf); rc != 0 {
			t.Errorf("explain %s exit = %d, want 0 (stderr=%q)", code, rc, errBuf.String())
		}
		if !strings.Contains(out.String(), code) {
			t.Errorf("explain %s output missing the code:\n%s", code, out.String())
		}
	}
}

func TestWorkflowRunExplain_ListIncludesRunCodes(t *testing.T) {
	var out, errBuf strings.Builder
	if rc := commands.RunExplain(false, []string{"--list"}, &out, &errBuf); rc != 0 {
		t.Fatalf("explain --list exit = %d, want 0", rc)
	}
	for _, code := range []string{"workflow.run.dependency_failed", "workflow.run.runtime_unreachable", "workflow.run.input_failed"} {
		if !strings.Contains(out.String(), code) {
			t.Errorf("explain --list missing %s", code)
		}
	}
}
