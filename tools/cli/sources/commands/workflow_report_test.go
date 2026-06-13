package commands_test

// Black-box tests for `portier workflow report` (v1.11 Slice 7): packaging an
// EXISTING workflow plan/run JSON report into a local diagnostic bundle. The
// command is fully offline and read-only — it parses only the provided report,
// never executes a workflow, contacts the runtime, reads the files a step refers
// to, or mutates the input.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

// failedRunReport is a workflow run report with one failed policy.check (a policy
// finding) and one skipped baseline compare. Referenced files are intentionally
// absent — the report command must never open them.
const failedRunReport = `{
  "workflow": "policy-baseline-check",
  "steps": [
    {"id":"check-config","type":"policy.check","status":"failed","exitCode":1,"message":"Policy check found 1 violation.",
     "report":{"findings":[{"code":"policy.lan_exposure_forbidden","severity":"error","title":"t","message":"m"}],"summary":{"info":0,"warning":0,"error":1},"result":"failed"}},
    {"id":"compare-baseline","type":"policy.baseline.compare","status":"skipped","exitCode":1,"message":"Skipped: depends on step \"check-config\", which produced no policy report."}
  ],
  "summary": {"total":2,"passed":0,"failed":1,"skipped":1},
  "result": "failed"
}`

const passedRunReport = `{
  "workflow": "ok",
  "steps": [
    {"id":"c","type":"policy.check","status":"passed","exitCode":0,"message":"Policy check passed (no violations).",
     "report":{"findings":[{"code":"policy.valid","severity":"info","title":"t","message":"m"}],"summary":{"info":1,"warning":0,"error":0},"result":"passed"}}
  ],
  "summary": {"total":1,"passed":1,"failed":0,"skipped":0},
  "result": "passed"
}`

const invalidPlanReport = `{
  "schemaVersion": 1,
  "name": "bad",
  "steps": [
    {"id":"r","type":"policy.review","code":"workflow.step.missing_candidate","status":"invalid","message":"...","inputs":{},"dependsOn":[]}
  ],
  "summary": {"total":1,"valid":0,"invalid":1},
  "result": "invalid"
}`

const explainRunReport = `{
  "workflow": "rt",
  "steps": [
    {"id":"c","type":"policy.check","status":"failed","exitCode":1,"message":"Runtime config could not be read: runtime unreachable"}
  ],
  "summary": {"total":1,"passed":0,"failed":1,"skipped":0},
  "result": "failed",
  "explanations": {"workflow.run.runtime_unreachable":{"code":"workflow.run.runtime_unreachable","title":"t","meaning":"m","action":"a","severity":"error"}}
}`

func runWorkflowReport(t *testing.T, jsonOutput bool, from, out string) (string, string, int) {
	t.Helper()
	var stdout, stderr strings.Builder
	code := commands.RunWorkflowReport(jsonOutput, []string{"--from", from, "--out", out}, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func readBundleJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	return m
}

func TestWorkflowReport_FailedRun(t *testing.T) {
	from := writeTempFile(t, "run.json", failedRunReport)
	out := filepath.Join(t.TempDir(), "bundle")
	stdout, _, code := runWorkflowReport(t, false, from, out)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Workflow report written to "+out) {
		t.Errorf("missing confirmation:\n%s", stdout)
	}
	for _, f := range []string{"manifest.json", "summary.txt", "report.json", "explanations.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing bundle file %s: %v", f, err)
		}
	}
	// manifest shape.
	man := readBundleJSON(t, filepath.Join(out, "manifest.json"))
	if man["type"] != "workflow-report" || man["source"] != "workflow-run" || man["result"] != "failed" {
		t.Errorf("manifest = %+v", man)
	}
	// explanations.json includes the embedded policy code + the skip run code.
	exp := readBundleJSON(t, filepath.Join(out, "explanations.json"))
	if _, ok := exp["policy.lan_exposure_forbidden"]; !ok {
		t.Errorf("explanations missing policy code: %+v", exp)
	}
	if _, ok := exp["workflow.run.dependency_failed"]; !ok {
		t.Errorf("explanations missing workflow.run.dependency_failed: %+v", exp)
	}
	// summary.txt mentions safety guarantees.
	summary, _ := os.ReadFile(filepath.Join(out, "summary.txt"))
	if !strings.Contains(string(summary), "No runtime contact.") {
		t.Errorf("summary missing safety block:\n%s", summary)
	}
}

func TestWorkflowReport_PassedRunEmptyExplanations(t *testing.T) {
	from := writeTempFile(t, "run.json", passedRunReport)
	out := filepath.Join(t.TempDir(), "bundle")
	if _, _, code := runWorkflowReport(t, false, from, out); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	exp := readBundleJSON(t, filepath.Join(out, "explanations.json"))
	if len(exp) != 0 {
		t.Errorf("passed run should yield empty explanations object, got %+v", exp)
	}
}

func TestWorkflowReport_InvalidPlanStepCodes(t *testing.T) {
	from := writeTempFile(t, "plan.json", invalidPlanReport)
	out := filepath.Join(t.TempDir(), "bundle")
	if _, _, code := runWorkflowReport(t, true, from, out); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	man := readBundleJSON(t, filepath.Join(out, "manifest.json"))
	if man["source"] != "workflow-plan" || man["result"] != "invalid" {
		t.Errorf("manifest = %+v", man)
	}
	exp := readBundleJSON(t, filepath.Join(out, "explanations.json"))
	if _, ok := exp["workflow.step.missing_candidate"]; !ok {
		t.Errorf("explanations missing workflow.step code: %+v", exp)
	}
}

func TestWorkflowReport_ExplainInputCarriesRunCode(t *testing.T) {
	from := writeTempFile(t, "run.json", explainRunReport)
	out := filepath.Join(t.TempDir(), "bundle")
	if _, _, code := runWorkflowReport(t, false, from, out); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	exp := readBundleJSON(t, filepath.Join(out, "explanations.json"))
	if _, ok := exp["workflow.run.runtime_unreachable"]; !ok {
		t.Errorf("explanations should carry the input report's run code: %+v", exp)
	}
}

func TestWorkflowReport_JSONEmitsManifest(t *testing.T) {
	from := writeTempFile(t, "run.json", failedRunReport)
	out := filepath.Join(t.TempDir(), "bundle")
	stdout, _, code := runWorkflowReport(t, true, from, out)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var man map[string]any
	if err := json.Unmarshal([]byte(stdout), &man); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout)
	}
	if man["type"] != "workflow-report" {
		t.Errorf("json manifest = %+v", man)
	}
}

func TestWorkflowReport_DoesNotReadReferencedFilesOrMutateInput(t *testing.T) {
	// The report references no real files; the command must succeed without
	// touching anything but --from and --out, and must not modify --from.
	from := writeTempFile(t, "run.json", failedRunReport)
	before, _ := os.ReadFile(from)
	out := filepath.Join(t.TempDir(), "bundle")
	if _, _, code := runWorkflowReport(t, false, from, out); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	after, _ := os.ReadFile(from)
	if string(before) != string(after) {
		t.Errorf("input report was mutated")
	}
}

func TestWorkflowReport_CreatesMissingDir(t *testing.T) {
	from := writeTempFile(t, "run.json", failedRunReport)
	out := filepath.Join(t.TempDir(), "nested", "bundle")
	if _, _, code := runWorkflowReport(t, false, from, out); code != 0 {
		t.Fatalf("exit code = %d, want 0 (should create missing dir)", code)
	}
	if _, err := os.Stat(filepath.Join(out, "manifest.json")); err != nil {
		t.Errorf("bundle not created in nested dir: %v", err)
	}
}

func TestWorkflowReport_ExistingEmptyDirOK(t *testing.T) {
	from := writeTempFile(t, "run.json", failedRunReport)
	out := t.TempDir() // exists and is empty
	if _, _, code := runWorkflowReport(t, false, from, out); code != 0 {
		t.Fatalf("exit code = %d, want 0 (empty dir allowed)", code)
	}
}

func TestWorkflowReport_NonEmptyDirRefusedExit1(t *testing.T) {
	from := writeTempFile(t, "run.json", failedRunReport)
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runWorkflowReport(t, false, from, out)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (non-empty dir refused)", code)
	}
	if !strings.Contains(stderr, "not empty") {
		t.Errorf("stderr = %q", stderr)
	}
	// The pre-existing file must be untouched.
	if _, err := os.Stat(filepath.Join(out, "existing.txt")); err != nil {
		t.Errorf("unrelated file removed: %v", err)
	}
}

func TestWorkflowReport_MalformedInputExit2(t *testing.T) {
	from := writeTempFile(t, "run.json", `{not json`)
	out := filepath.Join(t.TempDir(), "bundle")
	if _, _, code := runWorkflowReport(t, false, from, out); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowReport_UnsupportedKindExit2(t *testing.T) {
	// A bare policy report (no steps) is not a workflow report.
	from := writeTempFile(t, "policy.json", `{"findings":[],"summary":{"info":1,"warning":0,"error":0},"result":"passed"}`)
	out := filepath.Join(t.TempDir(), "bundle")
	if _, _, code := runWorkflowReport(t, false, from, out); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowReport_MissingArgsExit2(t *testing.T) {
	from := writeTempFile(t, "run.json", failedRunReport)
	var out, errBuf strings.Builder
	// Missing --out.
	if code := commands.RunWorkflowReport(false, []string{"--from", from}, &out, &errBuf); code != 2 {
		t.Errorf("missing --out exit = %d, want 2", code)
	}
	// Missing --from.
	out.Reset()
	errBuf.Reset()
	if code := commands.RunWorkflowReport(false, []string{"--out", filepath.Join(t.TempDir(), "b")}, &out, &errBuf); code != 2 {
		t.Errorf("missing --from exit = %d, want 2", code)
	}
	// Missing --out value.
	out.Reset()
	errBuf.Reset()
	if code := commands.RunWorkflowReport(false, []string{"--from", from, "--out"}, &out, &errBuf); code != 2 {
		t.Errorf("missing --out value exit = %d, want 2", code)
	}
}

func TestWorkflowReport_UnreadableInputExit2(t *testing.T) {
	out := filepath.Join(t.TempDir(), "bundle")
	if _, _, code := runWorkflowReport(t, false, filepath.Join(t.TempDir(), "missing.json"), out); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowReport_HelpExit0(t *testing.T) {
	var out, errBuf strings.Builder
	if code := commands.RunWorkflowReport(false, []string{"--help"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier workflow report") {
		t.Errorf("help missing:\n%s", out.String())
	}
}

func TestWorkflowReport_JSONEncodeFailureExit1(t *testing.T) {
	// The bundle is written to disk, then the --json manifest is encoded to stdout;
	// a stdout encode failure is an operation failure → exit 1.
	from := writeTempFile(t, "run.json", failedRunReport)
	out := filepath.Join(t.TempDir(), "bundle")
	var errBuf strings.Builder
	code := commands.RunWorkflowReport(true, []string{"--from", from, "--out", out}, failingWriter{}, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Error encoding JSON") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestWorkflowReport_Dispatch(t *testing.T) {
	from := writeTempFile(t, "run.json", failedRunReport)
	out := filepath.Join(t.TempDir(), "bundle")
	var stdout, stderr strings.Builder
	code := commands.RunWorkflow(false, commands.ConnFlags{}, []string{"report", "--from", from, "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dispatch exit = %d, want 0\n%s", code, stderr.String())
	}
}
