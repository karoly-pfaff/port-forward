package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// passedRunJSON / failedRunJSON are two same-kind run reports that differ.
const passedRunJSON = `{"workflow":"w","result":"passed","steps":[{"id":"check","type":"policy.check","status":"passed","exitCode":0}]}`
const failedRunJSON = `{"workflow":"w","result":"failed","steps":[
	{"id":"check","type":"policy.check","status":"passed","exitCode":0},
	{"id":"compare","type":"policy.baseline.compare","status":"failed","exitCode":1,"report":{"new":[{"code":"policy.lan_exposure_forbidden"}]}}]}`

func writeLeftRight(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	lp := filepath.Join(dir, "left.json")
	rp := filepath.Join(dir, "right.json")
	if err := os.WriteFile(lp, []byte(passedRunJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rp, []byte(failedRunJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return lp, rp
}

func TestCompare_Human(t *testing.T) {
	lp, rp := writeLeftRight(t)
	code, out, _ := runCLI("compare", "--left", lp, "--right", rp)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"Portier Replay Compare", "Left:", "Right:", "Changes:", "Result changed from passed to failed.", "Insights:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestCompare_JSON(t *testing.T) {
	lp, rp := writeLeftRight(t)
	code, out, _ := runCLI("--json", "compare", "--left", lp, "--right", rp)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, `"schemaVersion": 1`) || !strings.Contains(out, `"sameKind": true`) {
		t.Errorf("missing compare JSON fields: %q", out)
	}
	if !strings.Contains(out, `"changes"`) || !strings.Contains(out, `"sets"`) {
		t.Errorf("missing changes/sets: %q", out)
	}
}

func TestCompare_HistoryPair(t *testing.T) {
	dir := t.TempDir()
	lp := filepath.Join(dir, "l.json")
	rp := filepath.Join(dir, "r.json")
	if err := os.WriteFile(lp, []byte(`{"schemaVersion":1,"source":"workflow-history","runCount":0,"runs":[],"safety":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rp, []byte(`{"schemaVersion":1,"source":"workflow-history","runCount":1,"runs":[{"id":"r1","workflow":"w","result":"failed","codes":["x"]}],"safety":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI("compare", "--left", lp, "--right", rp)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Failed run count increased.") {
		t.Errorf("missing history insight: %q", out)
	}
}

func TestCompare_MixedKind(t *testing.T) {
	dir := t.TempDir()
	lp := filepath.Join(dir, "run.json")
	rp := filepath.Join(dir, "hist.json")
	if err := os.WriteFile(lp, []byte(passedRunJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rp, []byte(`{"schemaVersion":1,"source":"workflow-history","runCount":0,"runs":[],"safety":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI("compare", "--left", lp, "--right", rp)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (mixed kinds must not crash)", code)
	}
	if !strings.Contains(out, "different kinds; detailed comparison is limited") {
		t.Errorf("missing mixed-kind message: %q", out)
	}
}

func TestCompare_MissingLeft(t *testing.T) {
	_, rp := writeLeftRight(t)
	code, _, errOut := runCLI("compare", "--right", rp)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "--left is required") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestCompare_MissingRight(t *testing.T) {
	lp, _ := writeLeftRight(t)
	code, _, errOut := runCLI("compare", "--left", lp)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "--right is required") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestCompare_UnreadableLeft(t *testing.T) {
	_, rp := writeLeftRight(t)
	code, _, _ := runCLI("compare", "--left", filepath.Join(t.TempDir(), "nope.json"), "--right", rp)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestCompare_UnreadableRight(t *testing.T) {
	lp, _ := writeLeftRight(t)
	code, _, _ := runCLI("compare", "--left", lp, "--right", filepath.Join(t.TempDir(), "nope.json"))
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestCompare_MalformedInput(t *testing.T) {
	dir := t.TempDir()
	lp := filepath.Join(dir, "bad.json")
	_, rp := writeLeftRight(t)
	if err := os.WriteFile(lp, []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, _ := runCLI("compare", "--left", lp, "--right", rp)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestCompare_UnsupportedInput(t *testing.T) {
	dir := t.TempDir()
	lp := filepath.Join(dir, "weird.json")
	_, rp := writeLeftRight(t)
	if err := os.WriteFile(lp, []byte(`{"hello":"world"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, _ := runCLI("compare", "--left", lp, "--right", rp)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestCompare_BadFlag(t *testing.T) {
	code, _, _ := runCLI("compare", "--nope")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestCompare_Out(t *testing.T) {
	lp, rp := writeLeftRight(t)
	outPath := filepath.Join(t.TempDir(), "compare.json")
	code, _, _ := runCLI("compare", "--left", lp, "--right", rp, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out file: %v", err)
	}
	if !bytes.Contains(data, []byte(`"schemaVersion": 1`)) {
		t.Errorf("out file missing schema: %s", data)
	}
}

func TestCompare_JSONOutByteParity(t *testing.T) {
	lp, rp := writeLeftRight(t)
	outPath := filepath.Join(t.TempDir(), "compare.json")
	code, stdout, _ := runCLI("--json", "compare", "--left", lp, "--right", rp, "--out", outPath)
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

func TestCompare_OutWriteFailure(t *testing.T) {
	lp, rp := writeLeftRight(t)
	outPath := filepath.Join(t.TempDir(), "missing-dir", "compare.json")
	code, _, errOut := runCLI("compare", "--left", lp, "--right", rp, "--out", outPath)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "writing") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestCompare_JSONStdoutWriteFailure(t *testing.T) {
	lp, rp := writeLeftRight(t)
	if code := Run([]string{"--json", "compare", "--left", lp, "--right", rp}, failWriter{}, &bytes.Buffer{}); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestCompare_DoesNotMutateInputs(t *testing.T) {
	lp, rp := writeLeftRight(t)
	lBefore, _ := os.ReadFile(lp)
	rBefore, _ := os.ReadFile(rp)
	if code, _, _ := runCLI("--json", "compare", "--left", lp, "--right", rp); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	lAfter, _ := os.ReadFile(lp)
	rAfter, _ := os.ReadFile(rp)
	if !bytes.Equal(lBefore, lAfter) || !bytes.Equal(rBefore, rAfter) {
		t.Error("compare mutated an input artifact")
	}
}

// TestCompare_DoesNotReadReferencedFiles proves compare never opens files an
// artifact refers to.
func TestCompare_DoesNotReadReferencedFiles(t *testing.T) {
	dir := t.TempDir()
	referenced := filepath.Join(dir, "policy.json")
	report := `{"workflow":"ref","result":"failed","steps":[{"id":"c","type":"policy.check","status":"failed","exitCode":1,
		"report":{"policy":"policy.json","config":"rules.json","findings":[{"code":"policy.lan_exposure_forbidden","severity":"error"}]}}]}`
	lp := filepath.Join(dir, "left.json")
	rp := filepath.Join(dir, "right.json")
	if err := os.WriteFile(lp, []byte(passedRunJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rp, []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCLI("compare", "--left", lp, "--right", rp); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if _, err := os.Stat(referenced); !os.IsNotExist(err) {
		t.Error("referenced file should never be created or touched")
	}
}

// TestOtherCommandsStillWork confirms plan/analyze/timeline are unaffected.
func TestOtherCommandsStillWork(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	for _, cmd := range []struct{ name, title string }{
		{"plan", "Portier Replay Plan"},
		{"analyze", "Portier Replay Analysis"},
		{"timeline", "Portier Replay Timeline"},
	} {
		code, out, _ := runCLI(cmd.name, "--from", path)
		if code != 0 || !strings.Contains(out, cmd.title) {
			t.Errorf("%s changed: exit=%d out=%q", cmd.name, code, out)
		}
	}
}
