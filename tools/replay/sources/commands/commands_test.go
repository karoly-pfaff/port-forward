package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/replay/sources/version"
)

// runCLI invokes run() with captured stdout/stderr.
func runCLI(args ...string) (code int, stdout, stderr string) {
	var out, errBuf bytes.Buffer
	code = Run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

const runReportJSON = `{
  "workflow": "policy-baseline-check",
  "result": "failed",
  "steps": [
    {"id": "check", "type": "policy.check", "status": "passed", "exitCode": 0},
    {"id": "compare", "type": "policy.baseline.compare", "status": "failed", "exitCode": 1}
  ],
  "summary": {"total": 2, "passed": 1, "failed": 1, "skipped": 0}
}`

const historyExportJSON = `{
  "schemaVersion": 1,
  "createdAt": "2026-06-14T10:15:30Z",
  "source": "workflow-history",
  "runCount": 2,
  "runs": [
    {"id": "a", "workflow": "w1", "result": "passed"},
    {"id": "b", "workflow": "w2", "result": "failed"}
  ],
  "safety": {"containsRawConfigs": false}
}`

func TestRun_NoArgs_PrintsHelp(t *testing.T) {
	code, out, _ := runCLI()
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "replay - offline analysis") {
		t.Errorf("help not printed: %q", out)
	}
	if strings.Contains(out, "portier-replay") {
		t.Errorf("help should use the 'replay' binary name, not 'portier-replay': %q", out)
	}
	// Help must list every command and document the exit codes.
	for _, want := range []string{"plan", "analyze", "timeline", "compare", "explain", "version", "Exit codes:", "no exit 3"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
}

func TestRun_VersionFlag(t *testing.T) {
	code, out, _ := runCLI("--version")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	want := "Portier replay " + version.Version
	if !strings.Contains(out, want) {
		t.Errorf("--version output = %q, want it to contain %q", out, want)
	}
}

func TestRun_VersionCommand(t *testing.T) {
	code, out, _ := runCLI("version")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Portier replay "+version.Version) {
		t.Errorf("version output = %q", out)
	}
}

func TestVersion_IsCurrentRelease(t *testing.T) {
	// The replay tool version tracks the overall Portier release version.
	if version.Version != "1.15.0" {
		t.Errorf("version.Version = %q, want 1.15.0", version.Version)
	}
}

func TestRun_Help(t *testing.T) {
	code, out, _ := runCLI("help")
	if code != 0 || !strings.Contains(out, "Usage:") {
		t.Errorf("help exit=%d out=%q", code, out)
	}
}

func TestRun_BadGlobalFlag(t *testing.T) {
	code, _, errOut := runCLI("--bogus", "plan")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "Error:") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestRun_HelpFlag(t *testing.T) {
	code, out, _ := runCLI("-h")
	if code != 0 || !strings.Contains(out, "replay - offline analysis") {
		t.Errorf("-h exit=%d out=%q", code, out)
	}
}

func TestPlan_BadFlag(t *testing.T) {
	code, _, _ := runCLI("plan", "--nope")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	code, _, errOut := runCLI("bogus")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "Unknown command") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestPlan_RunReport_Human(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	code, out, _ := runCLI("plan", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Source: workflow-run-report") {
		t.Errorf("missing source line: %q", out)
	}
	if !strings.Contains(out, "Workflow: policy-baseline-check") {
		t.Errorf("missing workflow line: %q", out)
	}
}

func TestPlan_RunReport_JSON(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	code, out, _ := runCLI("--json", "plan", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, `"source": "workflow-run-report"`) {
		t.Errorf("missing source in JSON: %q", out)
	}
	if !strings.Contains(out, `"schemaVersion": 1`) {
		t.Errorf("missing schemaVersion in JSON: %q", out)
	}
}

func TestPlan_HistoryExport(t *testing.T) {
	path := writeTemp(t, "export.json", historyExportJSON)
	code, out, _ := runCLI("plan", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Source: workflow-history-export") {
		t.Errorf("missing source line: %q", out)
	}
}

func TestPlan_ReportBundleDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"),
		[]byte(`{"schemaVersion":1,"type":"workflow-report","workflow":"w","result":"failed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI("plan", "--from", dir)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Source: workflow-report-bundle") {
		t.Errorf("missing source line: %q", out)
	}
}

func TestPlan_MissingFrom(t *testing.T) {
	code, _, errOut := runCLI("plan")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "--from is required") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestPlan_UnreadableInput(t *testing.T) {
	code, _, _ := runCLI("plan", "--from", filepath.Join(t.TempDir(), "nope.json"))
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestPlan_MalformedInput(t *testing.T) {
	path := writeTemp(t, "bad.json", "{ not json")
	code, _, _ := runCLI("plan", "--from", path)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestPlan_UnsupportedShape(t *testing.T) {
	path := writeTemp(t, "weird.json", `{"hello": "world"}`)
	code, _, _ := runCLI("plan", "--from", path)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestPlan_Out(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.json")
	if err := os.WriteFile(path, []byte(runReportJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "plan.json")
	code, _, _ := runCLI("plan", "--from", path, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out file: %v", err)
	}
	if !bytes.Contains(data, []byte(`"source": "workflow-run-report"`)) {
		t.Errorf("out file missing source: %s", data)
	}
}

func TestPlan_JSONOutByteParity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.json")
	if err := os.WriteFile(path, []byte(runReportJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "plan.json")
	code, stdout, _ := runCLI("--json", "plan", "--from", path, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	fileData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != string(fileData) {
		t.Errorf("--json stdout and --out file differ:\nstdout=%q\nfile=%q", stdout, fileData)
	}
}

func TestPlan_OutWriteFailure(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	// A path inside a nonexistent directory cannot be written.
	outPath := filepath.Join(t.TempDir(), "missing-dir", "plan.json")
	code, _, errOut := runCLI("plan", "--from", path, "--out", outPath)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "writing") {
		t.Errorf("stderr = %q", errOut)
	}
}

// TestPlan_DoesNotMutateInput proves the input artifact is never modified.
func TestPlan_DoesNotMutateInput(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCLI("--json", "plan", "--from", path); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("input artifact was mutated")
	}
}

// TestPlan_DoesNotReadReferencedFiles proves the tool never opens the files an
// artifact refers to: a report referencing absent config/policy paths still
// succeeds, and those referenced files are never created or required.
func TestPlan_DoesNotReadReferencedFiles(t *testing.T) {
	dir := t.TempDir()
	referenced := filepath.Join(dir, "policy.json")
	// Reference the file by a forward-slash relative name to keep the embedded JSON
	// valid on all platforms; the point is that no referenced file is ever required.
	report := `{
		"workflow": "ref",
		"result": "failed",
		"steps": [{"id": "check", "type": "policy.check", "status": "failed", "exitCode": 1,
			"report": {"policy": "policy.json", "config": "rules.json"}}]
	}`
	path := writeTemp(t, "run.json", report)
	code, _, _ := runCLI("plan", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (referenced files must not be required)", code)
	}
	if _, err := os.Stat(referenced); !os.IsNotExist(err) {
		t.Error("referenced file should never be created or touched")
	}
}
