package commands_test

import (
	"encoding/json"
	"strings"
	"testing"

	"portier/cli/sources/commands"
	"portier/cli/sources/doctor"
)

// runConfigDoctorJSON runs `config doctor` with --json on the given file and
// decodes the resulting report. It fails the test on a non-decodable report.
func runConfigDoctorJSON(t *testing.T, filePath string) (doctor.Report, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(true, []string{filePath}, &out, &errBuf)
	var report doctor.Report
	if err := json.Unmarshal([]byte(out.String()), &report); err != nil {
		t.Fatalf("decoding doctor JSON: %v\noutput:\n%s", err, out.String())
	}
	return report, code
}

// checkCodes returns the ordered list of check codes in a report.
func checkCodes(report doctor.Report) []string {
	codes := make([]string, len(report.Checks))
	for i, c := range report.Checks {
		codes[i] = c.Code
	}
	return codes
}

// hasCode reports whether the report contains a check with the given code.
func hasCode(report doctor.Report, code string) bool {
	for _, c := range report.Checks {
		if c.Code == code {
			return true
		}
	}
	return false
}

// --- strict mode (v1.9 Slice 4) ---

// warningOnlyConfig is a valid config whose only finding is a LAN-exposure
// warning (config.valid + config.lan_exposure).
const warningOnlyConfig = `[
	{"name": "Web", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
]`

func TestConfigDoctor_WarningOnly_NormalExit0(t *testing.T) {
	file := writeTempConfig(t, warningOnlyConfig)
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("normal exit code = %d, want 0 (warnings do not fail without --strict)", code)
	}
	if !strings.Contains(out.String(), "Result: passed") {
		t.Errorf("normal warning-only output should report Result: passed\n%s", out.String())
	}
}

func TestConfigDoctor_WarningOnly_StrictExit1(t *testing.T) {
	file := writeTempConfig(t, warningOnlyConfig)
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--strict", file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("strict exit code = %d, want 1 (warnings fail under --strict)", code)
	}
	s := out.String()
	if !strings.Contains(s, "Strict mode: warnings are treated as failures.") {
		t.Errorf("strict output missing strict note\n%s", s)
	}
	if !strings.Contains(s, "Result: failed") {
		t.Errorf("strict warning-only output should report Result: failed\n%s", s)
	}
}

func TestConfigDoctor_Error_StrictAndNormalExit1(t *testing.T) {
	// Duplicate binding → config.duplicate_binding (error): exit 1 regardless of strict.
	dup := `[
		{"name": "A", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true},
		{"name": "B", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.2", "targetPort": 9090, "enabled": true}
	]`
	file := writeTempConfig(t, dup)
	var out, errBuf strings.Builder
	if code := commands.RunConfigDoctor(false, []string{file}, &out, &errBuf); code != 1 {
		t.Errorf("normal exit = %d, want 1", code)
	}
	out.Reset()
	errBuf.Reset()
	if code := commands.RunConfigDoctor(false, []string{"--strict", file}, &out, &errBuf); code != 1 {
		t.Errorf("strict exit = %d, want 1", code)
	}
}

func TestConfigDoctor_Valid_StrictExit0(t *testing.T) {
	file := writeTempConfig(t, `[
		{"name": "API", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--strict", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("strict exit = %d, want 0 (no warnings or errors)", code)
	}
}

func TestConfigDoctor_StrictJSON_Fields(t *testing.T) {
	file := writeTempConfig(t, warningOnlyConfig)
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(true, []string{"--strict", file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("strict JSON exit = %d, want 1", code)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out.String())
	}
	if raw["strict"] != true {
		t.Errorf("strict field = %v, want true", raw["strict"])
	}
	if raw["result"] != "failed" {
		t.Errorf("result field = %v, want failed", raw["result"])
	}
	// checks/summary still present (shape preserved).
	if _, ok := raw["checks"]; !ok {
		t.Errorf("JSON missing checks: %v", raw)
	}
	if _, ok := raw["summary"]; !ok {
		t.Errorf("JSON missing summary: %v", raw)
	}
}

func TestConfigDoctor_ValidConfig(t *testing.T) {
	file := writeTempConfig(t, `[
		{"name": "API", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	report, code := runConfigDoctorJSON(t, file)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := checkCodes(report); len(got) != 1 || got[0] != "config.valid" {
		t.Errorf("check codes = %v, want [config.valid]", got)
	}
	if report.Checks[0].Severity != "info" {
		t.Errorf("severity = %q, want info", report.Checks[0].Severity)
	}
	if report.Summary != (doctor.Summary{Info: 1}) {
		t.Errorf("summary = %+v, want {Info:1}", report.Summary)
	}
}

func TestConfigDoctor_MissingFile_ReadFailed(t *testing.T) {
	report, code := runConfigDoctorJSON(t, "does-not-exist-12345.json")

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if got := checkCodes(report); len(got) != 1 || got[0] != "config.read_failed" {
		t.Errorf("check codes = %v, want [config.read_failed]", got)
	}
	if report.Summary.Error != 1 {
		t.Errorf("error count = %d, want 1", report.Summary.Error)
	}
}

func TestConfigDoctor_MalformedJSON_ParseFailed(t *testing.T) {
	file := writeTempConfig(t, `{ this is not json`)
	report, code := runConfigDoctorJSON(t, file)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if got := checkCodes(report); len(got) != 1 || got[0] != "config.parse_failed" {
		t.Errorf("check codes = %v, want [config.parse_failed]", got)
	}
}

func TestConfigDoctor_EmptyConfig(t *testing.T) {
	file := writeTempConfig(t, `[]`)
	report, code := runConfigDoctorJSON(t, file)

	if code != 0 {
		t.Errorf("exit code = %d, want 0 (empty is a warning, not an error)", code)
	}
	if got := checkCodes(report); len(got) != 1 || got[0] != "config.empty" {
		t.Errorf("check codes = %v, want [config.empty]", got)
	}
	if report.Checks[0].Severity != "warning" {
		t.Errorf("severity = %q, want warning", report.Checks[0].Severity)
	}
}

func TestConfigDoctor_ValidationFailed(t *testing.T) {
	// targetHost missing → field validation error. Binding is loopback + a
	// recommended port so no advisory checks are produced.
	file := writeTempConfig(t, `[
		{"name": "Broken", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "", "targetPort": 8080, "enabled": true}
	]`)
	report, code := runConfigDoctorJSON(t, file)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !hasCode(report, "config.validation_failed") {
		t.Errorf("missing config.validation_failed; got %v", checkCodes(report))
	}
	if hasCode(report, "config.valid") {
		t.Errorf("config.valid must not appear for an invalid config")
	}
}

func TestConfigDoctor_DuplicateBinding(t *testing.T) {
	file := writeTempConfig(t, `[
		{"name": "A", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true},
		{"name": "B", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.2", "targetPort": 9090, "enabled": true}
	]`)
	report, code := runConfigDoctorJSON(t, file)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !hasCode(report, "config.duplicate_binding") {
		t.Errorf("missing config.duplicate_binding; got %v", checkCodes(report))
	}
	// A pure binding conflict is not a field error.
	if hasCode(report, "config.validation_failed") {
		t.Errorf("duplicate binding must not surface as config.validation_failed")
	}
	if hasCode(report, "config.valid") {
		t.Errorf("config.valid must not appear when a binding conflict exists")
	}
}

func TestConfigDoctor_LanExposure(t *testing.T) {
	file := writeTempConfig(t, `[
		{"name": "Web", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	report, code := runConfigDoctorJSON(t, file)

	if code != 0 {
		t.Errorf("exit code = %d, want 0 (LAN exposure is a warning)", code)
	}
	if got := checkCodes(report); len(got) != 2 || got[0] != "config.valid" || got[1] != "config.lan_exposure" {
		t.Errorf("check codes = %v, want [config.valid config.lan_exposure]", got)
	}
	if report.Summary.Warning != 1 {
		t.Errorf("warning count = %d, want 1", report.Summary.Warning)
	}
}

func TestConfigDoctor_PrivilegedPort(t *testing.T) {
	file := writeTempConfig(t, `[
		{"name": "SSH", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 22, "targetHost": "10.0.0.1", "targetPort": 22, "enabled": true}
	]`)
	report, code := runConfigDoctorJSON(t, file)

	if code != 0 {
		t.Errorf("exit code = %d, want 0 (privileged port is a warning)", code)
	}
	if !hasCode(report, "config.privileged_port") {
		t.Errorf("missing config.privileged_port; got %v", checkCodes(report))
	}
	if !hasCode(report, "config.valid") {
		t.Errorf("a privileged but otherwise valid rule should still be config.valid")
	}
}

func TestConfigDoctor_LanAndPrivilegedTogether(t *testing.T) {
	// 0.0.0.0 + port 80 → both advisories, exposure before port, both warnings.
	file := writeTempConfig(t, `[
		{"name": "HTTP", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 80, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	report, code := runConfigDoctorJSON(t, file)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := checkCodes(report); len(got) != 3 ||
		got[0] != "config.valid" || got[1] != "config.lan_exposure" || got[2] != "config.privileged_port" {
		t.Errorf("check codes = %v, want [config.valid config.lan_exposure config.privileged_port]", got)
	}
	if report.Summary.Warning != 2 {
		t.Errorf("warning count = %d, want 2", report.Summary.Warning)
	}
}

func TestConfigDoctor_JSONShape(t *testing.T) {
	file := writeTempConfig(t, `[
		{"name": "API", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	var out, errBuf strings.Builder
	commands.RunConfigDoctor(true, []string{file}, &out, &errBuf)

	// Decode into a generic map to assert the exact JSON contract keys.
	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v", err)
	}
	if _, ok := raw["checks"]; !ok {
		t.Errorf("JSON missing \"checks\" key: %v", raw)
	}
	summary, ok := raw["summary"].(map[string]any)
	if !ok {
		t.Fatalf("JSON missing \"summary\" object: %v", raw)
	}
	for _, key := range []string{"info", "warning", "error"} {
		if _, ok := summary[key]; !ok {
			t.Errorf("summary missing %q key: %v", key, summary)
		}
	}
	checks, ok := raw["checks"].([]any)
	if !ok || len(checks) == 0 {
		t.Fatalf("checks is not a non-empty array: %v", raw["checks"])
	}
	first, ok := checks[0].(map[string]any)
	if !ok {
		t.Fatalf("first check is not an object: %v", checks[0])
	}
	for _, key := range []string{"code", "severity", "title", "message"} {
		if _, ok := first[key]; !ok {
			t.Errorf("check missing %q key: %v", key, first)
		}
	}
}

func TestConfigDoctor_HumanOutput(t *testing.T) {
	file := writeTempConfig(t, `[
		{"name": "Web", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 22, "targetHost": "10.0.0.1", "targetPort": 22, "enabled": true}
	]`)
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{file}, &out, &errBuf)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	s := out.String()
	for _, want := range []string{"Portier Config Doctor", "[INFO]", "[WARN]", "Summary:", "info", "warning", "error"} {
		if !strings.Contains(s, want) {
			t.Errorf("human output missing %q\noutput:\n%s", want, s)
		}
	}
}

func TestConfigDoctor_MissingArg_Exit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "requires a config file path") {
		t.Errorf("stderr missing usage message: %q", errBuf.String())
	}
}

func TestConfigDoctor_Help_Exit0(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "config doctor") {
		t.Errorf("help output missing usage: %q", out.String())
	}
}

func TestConfigDoctor_BadFlag_Exit2(t *testing.T) {
	file := writeTempConfig(t, `[]`)
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--nope", file}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestConfigDoctor_JSONEncodeFailure_Exit1(t *testing.T) {
	file := writeTempConfig(t, `[
		{"name": "API", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	var errBuf strings.Builder
	code := commands.RunConfigDoctor(true, []string{file}, failingWriter{}, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Error encoding JSON") {
		t.Errorf("stderr missing encode-error message: %q", errBuf.String())
	}
}

// --- dispatch via RunConfig ---

func TestRunConfig_Doctor_Dispatch(t *testing.T) {
	file := writeTempConfig(t, `[
		{"name": "API", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	// config doctor is offline; a nil client must never be dereferenced.
	var out, errBuf strings.Builder
	code := commands.RunConfig(nil, true, []string{"doctor", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "config.valid") {
		t.Errorf("output missing config.valid: %q", out.String())
	}
}
