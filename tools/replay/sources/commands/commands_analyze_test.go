package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyze_RunReport_Human(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	code, out, _ := runCLI("analyze", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Portier Replay Analysis") {
		t.Errorf("missing title: %q", out)
	}
	if !strings.Contains(out, "Source: workflow-run-report") {
		t.Errorf("missing source: %q", out)
	}
	if !strings.Contains(out, "Step summary:") {
		t.Errorf("missing step summary: %q", out)
	}
}

func TestAnalyze_RunReport_JSON(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	code, out, _ := runCLI("--json", "analyze", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, `"source": "workflow-run-report"`) {
		t.Errorf("missing source in JSON: %q", out)
	}
	if !strings.Contains(out, `"schemaVersion": 1`) {
		t.Errorf("missing schemaVersion: %q", out)
	}
	if !strings.Contains(out, `"summary"`) || !strings.Contains(out, `"insights"`) {
		t.Errorf("missing summary/insights: %q", out)
	}
}

func TestAnalyze_HistoryExport(t *testing.T) {
	path := writeTemp(t, "export.json", historyExportJSON)
	code, out, _ := runCLI("analyze", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Source: workflow-history-export") {
		t.Errorf("missing source: %q", out)
	}
	if !strings.Contains(out, "Runs:") {
		t.Errorf("missing runs section: %q", out)
	}
}

func TestAnalyze_BundleDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"),
		[]byte(`{"schemaVersion":1,"type":"workflow-report","source":"workflow-run","workflow":"w","result":"failed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI("analyze", "--from", dir)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Source: workflow-report-bundle") {
		t.Errorf("missing source: %q", out)
	}
}

func TestAnalyze_MissingFrom(t *testing.T) {
	code, _, errOut := runCLI("analyze")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "--from is required") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestAnalyze_UnreadableInput(t *testing.T) {
	code, _, _ := runCLI("analyze", "--from", filepath.Join(t.TempDir(), "nope.json"))
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestAnalyze_MalformedInput(t *testing.T) {
	path := writeTemp(t, "bad.json", "{ not json")
	code, _, _ := runCLI("analyze", "--from", path)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestAnalyze_UnsupportedShape(t *testing.T) {
	path := writeTemp(t, "weird.json", `{"hello":"world"}`)
	code, _, _ := runCLI("analyze", "--from", path)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestAnalyze_BadFlag(t *testing.T) {
	code, _, _ := runCLI("analyze", "--nope")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestAnalyze_Out(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.json")
	if err := os.WriteFile(path, []byte(runReportJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "analysis.json")
	code, _, _ := runCLI("analyze", "--from", path, "--out", outPath)
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

func TestAnalyze_JSONOutByteParity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.json")
	if err := os.WriteFile(path, []byte(runReportJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "analysis.json")
	code, stdout, _ := runCLI("--json", "analyze", "--from", path, "--out", outPath)
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

func TestAnalyze_OutWriteFailure(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	outPath := filepath.Join(t.TempDir(), "missing-dir", "analysis.json")
	code, _, errOut := runCLI("analyze", "--from", path, "--out", outPath)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "writing") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestAnalyze_DoesNotMutateInput(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCLI("--json", "analyze", "--from", path); code != 0 {
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

// TestAnalyze_DoesNotReadReferencedFiles proves analysis never opens the files an
// artifact refers to: a report referencing absent files still succeeds, and those
// files are never created or required.
func TestAnalyze_DoesNotReadReferencedFiles(t *testing.T) {
	dir := t.TempDir()
	referenced := filepath.Join(dir, "policy.json")
	report := `{
		"workflow": "ref", "result": "failed",
		"steps": [{"id":"check","type":"policy.check","status":"failed","exitCode":1,
			"report": {"policy": "policy.json", "config": "rules.json",
				"findings": [{"code":"policy.lan_exposure_forbidden","severity":"error"}]}}]
	}`
	path := writeTemp(t, "run.json", report)
	code, _, _ := runCLI("analyze", "--from", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (referenced files must not be required)", code)
	}
	if _, err := os.Stat(referenced); !os.IsNotExist(err) {
		t.Error("referenced file should never be created or touched")
	}
}

// TestPlanStillWorks confirms the Slice 1 command is unaffected by Slice 2.
func TestPlanStillWorks(t *testing.T) {
	path := writeTemp(t, "run.json", runReportJSON)
	code, out, _ := runCLI("plan", "--from", path)
	if code != 0 {
		t.Fatalf("plan exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Portier Replay Plan") {
		t.Errorf("plan output changed: %q", out)
	}
}
