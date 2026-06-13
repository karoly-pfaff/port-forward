package commands_test

// Black-box tests for policy report export (`policy check --out`, v1.10 Slice 3):
// `portier policy check` writes the same JSON report shape as --json (findings +
// summary + result, plus the additive explanations map under --explain) to a
// file. --json --out stdout/file are byte-identical. A write failure is an
// operation failure (exit 1), not a policy finding; a missing --out value is a
// usage error (exit 2); and a malformed config/policy never writes a file.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

// runPolicyCheckArgs runs `policy check` with arbitrary extra args (so tests can
// add --out/--explain) and returns stdout, stderr, and the exit code.
func runPolicyCheckArgs(t *testing.T, jsonOutput bool, extra ...string) (string, string, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := commands.RunPolicyCheck(jsonOutput, commands.ConnFlags{}, extra, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// readPolicyReportFile reads and JSON-decodes a written policy report into a map
// so findings/summary/result/explanations can be asserted.
func readPolicyReportFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading policy report file %s: %v", path, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decoding policy report file %s: %v\n%s", path, err, data)
	}
	return raw
}

// --- write + result ---

func TestPolicyExport_PassingWritesPassedFile(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	outPath := filepath.Join(t.TempDir(), "policy-report.json")

	out, _, code := runPolicyCheckArgs(t, false, "--config", cfg, "--policy", pol, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	raw := readPolicyReportFile(t, outPath)
	for _, key := range []string{"findings", "summary", "result"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("report file missing %q: %v", key, raw)
		}
	}
	if raw["result"] != "passed" {
		t.Errorf("result = %v, want passed", raw["result"])
	}
	// Human output still printed to stdout, with an export confirmation.
	if !strings.Contains(out, "Report written to") {
		t.Errorf("human stdout should confirm the export:\n%s", out)
	}
	if !strings.Contains(out, "Portier Policy Check") {
		t.Errorf("human report should still print to stdout:\n%s", out)
	}
}

func TestPolicyExport_ViolatingWritesFailedFile(t *testing.T) {
	cfg := writeTempFile(t, "config.json", exposedConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false}}`)
	outPath := filepath.Join(t.TempDir(), "policy-report.json")

	_, _, code := runPolicyCheckArgs(t, false, "--config", cfg, "--policy", pol, "--out", outPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	raw := readPolicyReportFile(t, outPath)
	if raw["result"] != "failed" {
		t.Errorf("result = %v, want failed", raw["result"])
	}
}

// --- stdout/file byte parity ---

func TestPolicyExport_JSONStdoutAndFileByteIdentical(t *testing.T) {
	cfg := writeTempFile(t, "config.json", exposedConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false}}`)
	outPath := filepath.Join(t.TempDir(), "policy-report.json")

	out, _, code := runPolicyCheckArgs(t, true, "--config", cfg, "--policy", pol, "--out", outPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	fileData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading report file: %v", err)
	}
	if out != string(fileData) {
		t.Errorf("stdout and file contents differ\nstdout:\n%q\nfile:\n%q", out, string(fileData))
	}
}

func TestPolicyExport_JSONExplainStdoutAndFileByteIdentical(t *testing.T) {
	cfg := writeTempFile(t, "config.json", exposedConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false, "allowPrivilegedPorts": false}}`)
	outPath := filepath.Join(t.TempDir(), "policy-report.json")

	out, _, code := runPolicyCheckArgs(t, true, "--config", cfg, "--policy", pol, "--explain", "--out", outPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	fileData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading report file: %v", err)
	}
	if out != string(fileData) {
		t.Errorf("stdout and file contents differ\nstdout:\n%q\nfile:\n%q", out, string(fileData))
	}
	// The exported file carries the additive explanations map.
	raw := readPolicyReportFile(t, outPath)
	exps, ok := raw["explanations"].(map[string]any)
	if !ok {
		t.Fatalf("exported file missing explanations map with --explain: %s", string(fileData))
	}
	if _, ok := exps["policy.lan_exposure_forbidden"]; !ok {
		t.Errorf("exported explanations missing policy.lan_exposure_forbidden: %v", exps)
	}
}

// --- explain in human-mode export ---

func TestPolicyExport_ExplainHumanStdoutFileHasExplanations(t *testing.T) {
	cfg := writeTempFile(t, "config.json", exposedConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false}}`)
	outPath := filepath.Join(t.TempDir(), "policy-report.json")

	// Human stdout (with inline explanations), file gets JSON with explanations map.
	out, _, code := runPolicyCheckArgs(t, false, "--config", cfg, "--policy", pol, "--explain", "--out", outPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out, "Code: policy.lan_exposure_forbidden") {
		t.Errorf("human stdout should carry inline explanations:\n%s", out)
	}
	raw := readPolicyReportFile(t, outPath)
	if _, ok := raw["explanations"].(map[string]any); !ok {
		t.Errorf("exported file should carry explanations map: %v", raw)
	}
}

func TestPolicyExport_NoExplanationsWithoutFlag(t *testing.T) {
	cfg := writeTempFile(t, "config.json", exposedConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false}}`)
	outPath := filepath.Join(t.TempDir(), "policy-report.json")

	runPolicyCheckArgs(t, true, "--config", cfg, "--policy", pol, "--out", outPath)
	raw := readPolicyReportFile(t, outPath)
	if _, ok := raw["explanations"]; ok {
		t.Errorf("explanations must be omitted in the exported file without --explain: %v", raw)
	}
}

// --- error behavior ---

func TestPolicyExport_WriteFailure_Exit1(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	// Parent directory does not exist → os.WriteFile fails.
	badPath := filepath.Join(t.TempDir(), "no-such-dir", "policy-report.json")

	_, errBuf, code := runPolicyCheckArgs(t, false, "--config", cfg, "--policy", pol, "--out", badPath)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (write failure is an operation failure)", code)
	}
	if !strings.Contains(errBuf, "Error writing") {
		t.Errorf("stderr should report the write failure: %q", errBuf)
	}
	if _, err := os.Stat(badPath); !os.IsNotExist(err) {
		t.Errorf("no file should have been created at %s", badPath)
	}
}

func TestPolicyExport_MissingOutValue_Exit2(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	// `--out` with no following value → flag parse error → exit 2.
	_, _, code := runPolicyCheckArgs(t, false, "--config", cfg, "--policy", pol, "--out")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestPolicyExport_MalformedConfigWritesNoFile(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `not valid json`)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	outPath := filepath.Join(t.TempDir(), "policy-report.json")

	_, _, code := runPolicyCheckArgs(t, false, "--config", cfg, "--policy", pol, "--out", outPath)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (malformed config)", code)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("no report file should be written for a malformed config: %s", outPath)
	}
}

func TestPolicyExport_MalformedPolicyWritesNoFile(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 99}`)
	outPath := filepath.Join(t.TempDir(), "policy-report.json")

	_, _, code := runPolicyCheckArgs(t, false, "--config", cfg, "--policy", pol, "--out", outPath)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (unsupported schemaVersion)", code)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("no report file should be written for a malformed policy: %s", outPath)
	}
}

// --- non-mutation ---

func TestPolicyExport_DoesNotMutateInputs(t *testing.T) {
	const cfgContent = `[
		{"name": "Admin UI", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 80, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true, "group": "admin"}
	]`
	const polContent = `{"schemaVersion": 1, "rules": {"allowLanExposure": false, "allowPrivilegedPorts": false}}`
	cfg := writeTempFile(t, "config.json", cfgContent)
	pol := writeTempFile(t, "policy.json", polContent)
	outPath := filepath.Join(t.TempDir(), "policy-report.json")

	runPolicyCheckArgs(t, false, "--config", cfg, "--policy", pol, "--explain", "--out", outPath)

	gotCfg, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("reading config back: %v", err)
	}
	if string(gotCfg) != cfgContent {
		t.Errorf("config file was mutated:\n%s", string(gotCfg))
	}
	gotPol, err := os.ReadFile(pol)
	if err != nil {
		t.Fatalf("reading policy back: %v", err)
	}
	if string(gotPol) != polContent {
		t.Errorf("policy file was mutated:\n%s", string(gotPol))
	}
}
