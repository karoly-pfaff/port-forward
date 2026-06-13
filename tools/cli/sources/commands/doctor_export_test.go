package commands_test

// Black-box tests for doctor report export (`--out`, v1.9 Slice 5): both
// `portier doctor` and `portier config doctor` write the same JSON report shape
// (checks + summary + strict + result) to a file, and a write failure is an
// operation failure (exit 1) rather than a diagnostic finding.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/client"
	"portier/cli/sources/commands"
)

// readReportFile reads and JSON-decodes a written doctor report into a map so
// the strict/result fields (not on DoctorReport) can be asserted alongside the
// checks/summary shape. It fails the test if the file is missing or malformed.
func readReportFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading report file %s: %v", path, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decoding report file %s: %v\n%s", path, err, data)
	}
	return raw
}

// assertReportShape checks the exported JSON carries the full doctor report
// shape with the expected strict/result values.
func assertReportShape(t *testing.T, raw map[string]any, wantStrict bool, wantResult string) {
	t.Helper()
	for _, key := range []string{"checks", "summary", "strict", "result"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("report file missing %q: %v", key, raw)
		}
	}
	if raw["strict"] != wantStrict {
		t.Errorf("strict = %v, want %v", raw["strict"], wantStrict)
	}
	if raw["result"] != wantResult {
		t.Errorf("result = %v, want %v", raw["result"], wantResult)
	}
}

// --- config doctor export ---

func TestConfigDoctorExport_ValidWritesFile(t *testing.T) {
	cfg := writeTempConfig(t, `[
		{"name": "API", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	outPath := filepath.Join(t.TempDir(), "config-doctor-report.json")

	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--out", outPath, cfg}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertReportShape(t, readReportFile(t, outPath), false, "passed")
	// Human output still printed to stdout, with an export confirmation.
	if !strings.Contains(out.String(), "Report written to") {
		t.Errorf("human stdout should confirm the export:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Portier Config Doctor") {
		t.Errorf("human report should still print to stdout:\n%s", out.String())
	}
}

func TestConfigDoctorExport_WarningNormal_Exit0(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	outPath := filepath.Join(t.TempDir(), "report.json")

	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--out", outPath, cfg}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (warning, normal mode)", code)
	}
	assertReportShape(t, readReportFile(t, outPath), false, "passed")
}

func TestConfigDoctorExport_WarningStrict_Exit1(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	outPath := filepath.Join(t.TempDir(), "report.json")

	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--strict", "--out", outPath, cfg}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (warning, strict)", code)
	}
	// The report is still written even though the run "failed" on the warning.
	assertReportShape(t, readReportFile(t, outPath), true, "failed")
}

func TestConfigDoctorExport_JSONStdoutAndFile(t *testing.T) {
	cfg := writeTempConfig(t, `[
		{"name": "API", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	outPath := filepath.Join(t.TempDir(), "report.json")

	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(true, []string{"--out", outPath, cfg}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// stdout is valid JSON (no confirmation line corrupting it) and matches the file.
	var stdoutReport map[string]any
	if err := json.Unmarshal([]byte(out.String()), &stdoutReport); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out.String())
	}
	fileReport := readReportFile(t, outPath)
	if stdoutReport["result"] != fileReport["result"] || stdoutReport["strict"] != fileReport["strict"] {
		t.Errorf("stdout and file reports differ: stdout=%v file=%v", stdoutReport, fileReport)
	}
}

func TestConfigDoctorExport_WriteFailure_Exit1(t *testing.T) {
	cfg := writeTempConfig(t, `[
		{"name": "API", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	// Parent directory does not exist → os.WriteFile fails.
	badPath := filepath.Join(t.TempDir(), "no-such-dir", "report.json")

	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--out", badPath, cfg}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (write failure is an operation failure)", code)
	}
	if !strings.Contains(errBuf.String(), "Error writing") {
		t.Errorf("stderr should report the write failure: %q", errBuf.String())
	}
	if _, err := os.Stat(badPath); !os.IsNotExist(err) {
		t.Errorf("no file should have been created at %s", badPath)
	}
}

func TestConfigDoctorExport_MissingOutValue_Exit2(t *testing.T) {
	var out, errBuf strings.Builder
	// `--out` with no following value → flag parse error → exit 2.
	code := commands.RunConfigDoctor(false, []string{"--out"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

// --- live doctor export ---

func TestDoctorExport_WritesFile(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses:    []map[string]any{statusEntry("r1", "healthy")},
		exportRules: []map[string]any{{"id": "r1"}},
	})
	defer srv.Close()
	outPath := filepath.Join(t.TempDir(), "doctor-report.json")

	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, []string{"--out", outPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertReportShape(t, readReportFile(t, outPath), false, "passed")
}

func TestDoctorExport_StrictWarning_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "warning")},
	})
	defer srv.Close()
	outPath := filepath.Join(t.TempDir(), "doctor-report.json")

	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, []string{"--strict", "--out", outPath}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	assertReportShape(t, readReportFile(t, outPath), true, "failed")
}

func TestDoctorExport_JSONStdoutAndFile(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "healthy")},
	})
	defer srv.Close()
	outPath := filepath.Join(t.TempDir(), "doctor-report.json")

	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), true, []string{"--out", outPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var stdoutReport map[string]any
	if err := json.Unmarshal([]byte(out.String()), &stdoutReport); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out.String())
	}
	fileReport := readReportFile(t, outPath)
	if stdoutReport["result"] != fileReport["result"] {
		t.Errorf("stdout/file result mismatch: %v vs %v", stdoutReport["result"], fileReport["result"])
	}
}

func TestDoctorExport_WriteFailure_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{
		statuses: []map[string]any{statusEntry("r1", "healthy")},
	})
	defer srv.Close()
	badPath := filepath.Join(t.TempDir(), "no-such-dir", "doctor-report.json")

	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, []string{"--out", badPath}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (write failure)", code)
	}
	if !strings.Contains(errBuf.String(), "Error writing") {
		t.Errorf("stderr should report the write failure: %q", errBuf.String())
	}
}
