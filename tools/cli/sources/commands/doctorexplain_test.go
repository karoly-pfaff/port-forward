package commands_test

// Black-box tests for inline doctor explanations (`--explain`, v1.9 Slice 7):
// both doctor commands add per-check explanations (human blocks; an additive
// JSON `explanations` map) for the codes present in the report, without
// changing checks/summary/result or exit codes.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/client"
	"portier/cli/sources/commands"
)

// --- config doctor --explain ---

func TestConfigDoctorExplain_HumanIncludesExplanation(t *testing.T) {
	cfg := writeTempConfig(t, `[
		{"name": "API", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--explain", cfg}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	s := out.String()
	for _, want := range []string{"Code: config.valid", "Meaning:", "What to do:"} {
		if !strings.Contains(s, want) {
			t.Errorf("explain output missing %q\n%s", want, s)
		}
	}
}

func TestConfigDoctorExplain_WarningNormal_Exit0(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--explain", cfg}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit = %d, want 0 (warning, normal mode, --explain unchanged)", code)
	}
	if !strings.Contains(out.String(), "Code: config.lan_exposure") {
		t.Errorf("explain should include the warning code explanation\n%s", out.String())
	}
}

func TestConfigDoctorExplain_WarningStrict_Exit1(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--strict", "--explain", cfg}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit = %d, want 1 (warning under --strict, --explain does not change exit)", code)
	}
}

func TestConfigDoctorExplain_JSONIncludesExplanationsMap(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	var out, errBuf strings.Builder
	commands.RunConfigDoctor(true, []string{"--explain", cfg}, &out, &errBuf)

	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out.String())
	}
	exps, ok := raw["explanations"].(map[string]any)
	if !ok {
		t.Fatalf("explanations map missing/wrong type: %v", raw["explanations"])
	}
	// Only emitted codes appear: config.valid (info) + config.lan_exposure (warning).
	for _, code := range []string{"config.valid", "config.lan_exposure"} {
		if _, ok := exps[code]; !ok {
			t.Errorf("explanations missing emitted code %q: %v", code, exps)
		}
	}
	// A non-emitted code must NOT appear.
	if _, ok := exps["config.duplicate_binding"]; ok {
		t.Errorf("explanations must not include non-emitted codes: %v", exps)
	}
	entry, _ := exps["config.lan_exposure"].(map[string]any)
	for _, k := range []string{"meaning", "action"} {
		if _, ok := entry[k]; !ok {
			t.Errorf("explanation entry missing %q: %v", k, entry)
		}
	}
}

func TestConfigDoctorExplain_JSONWithoutExplain_NoMap(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	var out, errBuf strings.Builder
	commands.RunConfigDoctor(true, []string{cfg}, &out, &errBuf)
	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v", err)
	}
	if _, ok := raw["explanations"]; ok {
		t.Errorf("explanations must be omitted without --explain: %v", raw)
	}
}

func TestConfigDoctorExplain_OutWritesExplanations(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	outPath := filepath.Join(t.TempDir(), "report.json")
	var out, errBuf strings.Builder
	commands.RunConfigDoctor(false, []string{"--explain", "--out", outPath, cfg}, &out, &errBuf)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading export: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decoding export: %v\n%s", err, data)
	}
	if _, ok := raw["explanations"]; !ok {
		t.Errorf("exported JSON should include explanations with --explain: %v", raw)
	}
}

// --explain must not change checks/summary/result vs a non-explain run.
func TestConfigDoctorExplain_DoesNotChangeReport(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)

	var plain, withExplain strings.Builder
	commands.RunConfigDoctor(true, []string{cfg}, &plain, &strings.Builder{})
	commands.RunConfigDoctor(true, []string{"--explain", cfg}, &withExplain, &strings.Builder{})

	var a, b map[string]any
	_ = json.Unmarshal([]byte(plain.String()), &a)
	_ = json.Unmarshal([]byte(withExplain.String()), &b)

	for _, key := range []string{"checks", "summary", "result", "strict"} {
		ja, _ := json.Marshal(a[key])
		jb, _ := json.Marshal(b[key])
		if string(ja) != string(jb) {
			t.Errorf("%s changed with --explain:\n plain: %s\n explain: %s", key, ja, jb)
		}
	}
}

// --- live doctor --explain ---

func TestDoctorExplain_HumanIncludesExplanation(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, []string{"--explain"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	s := out.String()
	if !strings.Contains(s, "Code: runtime.reachable") || !strings.Contains(s, "What to do:") {
		t.Errorf("explain output missing explanation block\n%s", s)
	}
}

func TestDoctorExplain_Unreachable_IncludesExplanation_Exit1(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{})
	url := srv.URL
	srv.Close()

	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(url), false, []string{"--explain"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "Code: runtime.unreachable") {
		t.Errorf("unreachable explain output should include runtime.unreachable explanation\n%s", out.String())
	}
}

func TestDoctorExplain_JSONIncludesExplanationsMap(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	var out, errBuf strings.Builder
	commands.RunDoctor(client.New(srv.URL), true, []string{"--explain"}, &out, &errBuf)

	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out.String())
	}
	exps, ok := raw["explanations"].(map[string]any)
	if !ok {
		t.Fatalf("explanations map missing: %v", raw["explanations"])
	}
	if _, ok := exps["runtime.reachable"]; !ok {
		t.Errorf("explanations missing runtime.reachable: %v", exps)
	}
}

func TestDoctorExplain_StrictPreservesExit(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "warning")}})
	defer srv.Close()
	var out, errBuf strings.Builder
	code := commands.RunDoctor(client.New(srv.URL), false, []string{"--strict", "--explain"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit = %d, want 1 (strict warning; --explain does not change exit)", code)
	}
}
