package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const explainRunJSON = `{"workflow":"w","result":"failed","steps":[
	{"id":"compare","type":"policy.baseline.compare","status":"failed","exitCode":1,"report":{"new":[{"code":"policy.lan_exposure_forbidden"}]}},
	{"id":"dep","type":"policy.check","status":"skipped"}]}`

func TestExplain_RunReport_Human(t *testing.T) {
	path := writeTemp(t, "run.json", explainRunJSON)
	code, out, _ := runCLI("explain", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{
		"Portier Replay Explanations",
		"Source: workflow-run-report",
		"Codes: 2",
		"policy.lan_exposure_forbidden [warning]",
		"workflow.run.dependency_failed [error]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestExplain_RunReport_JSON(t *testing.T) {
	path := writeTemp(t, "run.json", explainRunJSON)
	code, out, _ := runCLI("--json", "explain", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, `"schemaVersion": 1`) || !strings.Contains(out, `"explanations"`) {
		t.Errorf("missing explain JSON fields: %q", out)
	}
	if !strings.Contains(out, `"known": true`) {
		t.Errorf("expected a known explanation: %q", out)
	}
}

func TestExplain_HistoryNoCodes(t *testing.T) {
	path := writeTemp(t, "hist.json", `{"schemaVersion":1,"source":"workflow-history","runCount":0,"runs":[],"safety":{}}`)
	code, out, _ := runCLI("explain", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (no codes is success)", code)
	}
	if !strings.Contains(out, "No emitted codes were found") {
		t.Errorf("missing no-codes notice: %q", out)
	}
}

func TestExplain_UnknownCodePreserved(t *testing.T) {
	// A failed step whose embedded finding carries an unrecognized code.
	path := writeTemp(t, "run.json", `{"workflow":"w","result":"failed","steps":[
		{"id":"c","type":"policy.check","status":"failed","exitCode":1,"report":{"findings":[{"code":"custom.future.code","severity":"error"}]}}]}`)
	code, out, _ := runCLI("explain", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "custom.future.code [unknown]") {
		t.Errorf("unknown code must be preserved and marked unknown: %q", out)
	}
}

func TestExplain_ReplayAnalysisOutput(t *testing.T) {
	// Feed a replay analysis JSON (produced by `analyze --json`) back into explain.
	dir := t.TempDir()
	runPath := filepath.Join(dir, "run.json")
	if err := os.WriteFile(runPath, []byte(explainRunJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	acode, analysisJSON, _ := runCLI("--json", "analyze", "--from", runPath)
	if acode != 0 {
		t.Fatalf("analyze exit = %d", acode)
	}
	analysisPath := filepath.Join(dir, "analysis.json")
	if err := os.WriteFile(analysisPath, []byte(analysisJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI("explain", "--from", analysisPath)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Source: replay-analysis") {
		t.Errorf("expected replay-analysis source: %q", out)
	}
}

func TestExplain_Code(t *testing.T) {
	code, out, _ := runCLI("explain", "--code", "policy.valid")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "policy.valid [info]") {
		t.Errorf("missing code explanation: %q", out)
	}
}

func TestExplain_CodeUnknown(t *testing.T) {
	code, out, _ := runCLI("explain", "--code", "nope.code")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "nope.code [unknown]") {
		t.Errorf("unknown --code should still be explained: %q", out)
	}
}

func TestExplain_List(t *testing.T) {
	code, out, _ := runCLI("--json", "explain", "--list")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.Count(out, `"code"`) != 24 {
		t.Errorf("expected 24 codes in --list, got %d:\n%s", strings.Count(out, `"code"`), out)
	}
}

func TestExplain_NoMode(t *testing.T) {
	code, _, errOut := runCLI("explain")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "one of --from, --code, or --list is required") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestExplain_MultipleModes(t *testing.T) {
	path := writeTemp(t, "run.json", explainRunJSON)
	code, _, errOut := runCLI("explain", "--from", path, "--list")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "only one of --from, --code, or --list") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestExplain_UnreadableInput(t *testing.T) {
	code, _, _ := runCLI("explain", "--from", filepath.Join(t.TempDir(), "nope.json"))
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestExplain_MalformedInput(t *testing.T) {
	path := writeTemp(t, "bad.json", "{ not json")
	code, _, _ := runCLI("explain", "--from", path)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestExplain_UnsupportedInput(t *testing.T) {
	path := writeTemp(t, "weird.json", `{"hello":"world"}`)
	code, _, _ := runCLI("explain", "--from", path)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestExplain_BadFlag(t *testing.T) {
	code, _, _ := runCLI("explain", "--nope")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestExplain_Out(t *testing.T) {
	path := writeTemp(t, "run.json", explainRunJSON)
	outPath := filepath.Join(t.TempDir(), "explain.json")
	code, _, _ := runCLI("explain", "--from", path, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out file: %v", err)
	}
	if !bytes.Contains(data, []byte(`"explanations"`)) {
		t.Errorf("out file missing explanations: %s", data)
	}
}

func TestExplain_JSONOutByteParity(t *testing.T) {
	path := writeTemp(t, "run.json", explainRunJSON)
	outPath := filepath.Join(t.TempDir(), "explain.json")
	code, stdout, _ := runCLI("--json", "explain", "--from", path, "--out", outPath)
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

func TestExplain_OutWriteFailure(t *testing.T) {
	path := writeTemp(t, "run.json", explainRunJSON)
	outPath := filepath.Join(t.TempDir(), "missing-dir", "explain.json")
	code, _, errOut := runCLI("explain", "--from", path, "--out", outPath)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "writing") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestExplain_JSONStdoutWriteFailure(t *testing.T) {
	path := writeTemp(t, "run.json", explainRunJSON)
	if code := Run([]string{"--json", "explain", "--from", path}, failWriter{}, &bytes.Buffer{}); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestExplain_DoesNotMutateInput(t *testing.T) {
	path := writeTemp(t, "run.json", explainRunJSON)
	before, _ := os.ReadFile(path)
	if code, _, _ := runCLI("--json", "explain", "--from", path); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("explain mutated the input artifact")
	}
}

func TestExplain_DoesNotReadReferencedFiles(t *testing.T) {
	dir := t.TempDir()
	referenced := filepath.Join(dir, "policy.json")
	report := `{"workflow":"ref","result":"failed","steps":[{"id":"c","type":"policy.check","status":"failed","exitCode":1,
		"report":{"policy":"policy.json","config":"rules.json","findings":[{"code":"policy.lan_exposure_forbidden","severity":"error"}]}}]}`
	path := writeTemp(t, "run.json", report)
	if code, _, _ := runCLI("explain", "--from", path); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if _, err := os.Stat(referenced); !os.IsNotExist(err) {
		t.Error("referenced file should never be created or touched")
	}
}

// TestEarlierCommandsStillWork confirms plan/analyze/timeline/compare are unaffected.
func TestEarlierCommandsStillWork(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	for _, c := range []struct{ name, title string }{
		{"plan", "Portier Replay Plan"},
		{"analyze", "Portier Replay Analysis"},
		{"timeline", "Portier Replay Timeline"},
	} {
		code, out, _ := runCLI(c.name, "--from", path)
		if code != 0 || !strings.Contains(out, c.title) {
			t.Errorf("%s changed: exit=%d out=%q", c.name, code, out)
		}
	}
	lp, rp := writeLeftRight(t)
	if code, out, _ := runCLI("compare", "--left", lp, "--right", rp); code != 0 || !strings.Contains(out, "Portier Replay Compare") {
		t.Errorf("compare changed: exit=%d out=%q", code, out)
	}
}
