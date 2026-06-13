package commands_test

// Black-box tests for config doctor structured advisory details (v1.9 Slice 8):
// duplicate-binding / LAN-exposure / privileged-port / empty / validation /
// valid findings carry deterministic, JSON-serializable details derived only
// from the offline config. Details flow through --json and --out, and --explain
// stays presentation-only. Exit semantics are unchanged.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

// configDoctorJSON runs config doctor with the given args and decodes the JSON
// report into a generic map so details can be inspected.
func configDoctorJSON(t *testing.T, args []string) (map[string]any, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(true, args, &out, &errBuf)
	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out.String())
	}
	return raw, code
}

// checkByCode finds the first check with the given code in a decoded report.
func checkByCode(t *testing.T, report map[string]any, code string) map[string]any {
	t.Helper()
	checks, _ := report["checks"].([]any)
	for _, c := range checks {
		cm, _ := c.(map[string]any)
		if cm["code"] == code {
			return cm
		}
	}
	t.Fatalf("check %q not found in report: %v", code, report["checks"])
	return nil
}

func TestConfigDoctorDetails_DuplicateBinding(t *testing.T) {
	cfg := writeTempConfig(t, `[
		{"id": "r1", "name": "Web A", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true, "group": "backend"},
		{"id": "r2", "name": "Web B", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.2", "targetPort": 9090, "enabled": false, "group": "backend"}
	]`)
	report, code := configDoctorJSON(t, []string{cfg})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}

	details, _ := checkByCode(t, report, "config.duplicate_binding")["details"].(map[string]any)
	bindings, _ := details["bindings"].([]any)
	if len(bindings) != 1 {
		t.Fatalf("bindings = %d, want 1\n%v", len(bindings), details)
	}
	b0, _ := bindings[0].(map[string]any)
	if b0["protocol"] != "tcp" || b0["listenHost"] != "127.0.0.1" || b0["listenPort"].(float64) != 48000 {
		t.Errorf("binding identity wrong: %v", b0)
	}
	rules, _ := b0["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("binding rules = %d, want 2", len(rules))
	}
	// File order: Web A before Web B.
	r0, _ := rules[0].(map[string]any)
	r1, _ := rules[1].(map[string]any)
	if r0["name"] != "Web A" || r1["name"] != "Web B" {
		t.Errorf("rules not in file order: %v, %v", r0["name"], r1["name"])
	}
	if r0["id"] != "r1" || r0["enabled"] != true || r0["group"] != "backend" {
		t.Errorf("rule detail fields wrong: %v", r0)
	}
	if r1["enabled"] != false {
		t.Errorf("Web B enabled = %v, want false", r1["enabled"])
	}
}

func TestConfigDoctorDetails_DuplicateBindingDeterministicOrder(t *testing.T) {
	// Two distinct conflicting bindings, declared out of sorted order: udp:48001
	// then tcp:48000. The bindings array must be sorted (tcp before udp).
	cfg := writeTempConfig(t, `[
		{"name": "U1", "protocol": "udp", "listenHost": "127.0.0.1", "listenPort": 48001, "targetHost": "10.0.0.1", "targetPort": 1, "enabled": true},
		{"name": "U2", "protocol": "udp", "listenHost": "127.0.0.1", "listenPort": 48001, "targetHost": "10.0.0.2", "targetPort": 2, "enabled": true},
		{"name": "T1", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.3", "targetPort": 3, "enabled": true},
		{"name": "T2", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.4", "targetPort": 4, "enabled": true}
	]`)
	report, _ := configDoctorJSON(t, []string{cfg})
	details, _ := checkByCode(t, report, "config.duplicate_binding")["details"].(map[string]any)
	bindings, _ := details["bindings"].([]any)
	if len(bindings) != 2 {
		t.Fatalf("bindings = %d, want 2", len(bindings))
	}
	first, _ := bindings[0].(map[string]any)
	second, _ := bindings[1].(map[string]any)
	if first["protocol"] != "tcp" || second["protocol"] != "udp" {
		t.Errorf("bindings not sorted by protocol: %v, %v", first["protocol"], second["protocol"])
	}
}

func TestConfigDoctorDetails_LanExposureRules(t *testing.T) {
	// Two LAN-exposed rules → ONE config.lan_exposure warning with both rules.
	cfg := writeTempConfig(t, `[
		{"name": "Dash", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true, "group": "admin"},
		{"name": "Api", "protocol": "udp", "listenHost": "0.0.0.0", "listenPort": 48081, "targetHost": "10.0.0.2", "targetPort": 9090, "enabled": false}
	]`)
	report, code := configDoctorJSON(t, []string{cfg})
	if code != 0 {
		t.Errorf("exit = %d, want 0 (LAN exposure is a warning)", code)
	}
	// Exactly one lan_exposure check (aggregated).
	checks, _ := report["checks"].([]any)
	lanCount := 0
	for _, c := range checks {
		if cm, _ := c.(map[string]any); cm["code"] == "config.lan_exposure" {
			lanCount++
		}
	}
	if lanCount != 1 {
		t.Errorf("lan_exposure checks = %d, want 1 (aggregated)", lanCount)
	}
	details, _ := checkByCode(t, report, "config.lan_exposure")["details"].(map[string]any)
	rules, _ := details["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("lan rules = %d, want 2", len(rules))
	}
	r0, _ := rules[0].(map[string]any)
	if r0["name"] != "Dash" || r0["protocol"] != "tcp" || r0["listenHost"] != "0.0.0.0" ||
		r0["listenPort"].(float64) != 48080 || r0["enabled"] != true || r0["group"] != "admin" {
		t.Errorf("lan rule detail wrong: %v", r0)
	}
}

func TestConfigDoctorDetails_PrivilegedPortRules(t *testing.T) {
	cfg := writeTempConfig(t, `[
		{"name": "SSH", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 22, "targetHost": "10.0.0.1", "targetPort": 22, "enabled": true, "group": "ops"}
	]`)
	report, code := configDoctorJSON(t, []string{cfg})
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	details, _ := checkByCode(t, report, "config.privileged_port")["details"].(map[string]any)
	rules, _ := details["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("privileged rules = %d, want 1", len(rules))
	}
	r0, _ := rules[0].(map[string]any)
	if r0["name"] != "SSH" || r0["listenPort"].(float64) != 22 || r0["group"] != "ops" {
		t.Errorf("privileged rule detail wrong: %v", r0)
	}
}

func TestConfigDoctorDetails_Empty(t *testing.T) {
	cfg := writeTempConfig(t, `[]`)
	report, code := configDoctorJSON(t, []string{cfg})
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	details, _ := checkByCode(t, report, "config.empty")["details"].(map[string]any)
	if details["ruleCount"].(float64) != 0 {
		t.Errorf("ruleCount = %v, want 0", details["ruleCount"])
	}
}

func TestConfigDoctorDetails_Valid(t *testing.T) {
	cfg := writeTempConfig(t, `[
		{"name": "API", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true}
	]`)
	report, _ := configDoctorJSON(t, []string{cfg})
	details, _ := checkByCode(t, report, "config.valid")["details"].(map[string]any)
	if details["ruleCount"].(float64) != 1 || details["tcpCount"].(float64) != 1 {
		t.Errorf("valid details wrong: %v", details)
	}
}

func TestConfigDoctorDetails_ValidationFailed(t *testing.T) {
	cfg := writeTempConfig(t, `[
		{"name": "Broken", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "", "targetPort": 8080, "enabled": true}
	]`)
	report, code := configDoctorJSON(t, []string{cfg})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	details, _ := checkByCode(t, report, "config.validation_failed")["details"].(map[string]any)
	errs, _ := details["errors"].([]any)
	if len(errs) == 0 {
		t.Errorf("validation errors empty: %v", details)
	}
	if details["ruleCount"].(float64) != 1 {
		t.Errorf("ruleCount = %v, want 1", details["ruleCount"])
	}
}

func TestConfigDoctorDetails_OutPreservesDetails(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	outPath := filepath.Join(t.TempDir(), "report.json")
	var out, errBuf strings.Builder
	commands.RunConfigDoctor(false, []string{"--out", outPath, cfg}, &out, &errBuf)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading export: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decoding export: %v", err)
	}
	details, _ := checkByCode(t, raw, "config.lan_exposure")["details"].(map[string]any)
	if _, ok := details["rules"]; !ok {
		t.Errorf("exported lan_exposure should carry rules details: %v", details)
	}
}

func TestConfigDoctorDetails_ExplainCompatible(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	var out, errBuf strings.Builder
	code := commands.RunConfigDoctor(false, []string{"--explain", cfg}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	// --explain stays presentation-only; the explanation block still renders.
	if !strings.Contains(out.String(), "Code: config.lan_exposure") {
		t.Errorf("explain output missing explanation block\n%s", out.String())
	}
}

// Details must not change exit semantics: strict warning-only still exits 1,
// normal warning-only still exits 0.
func TestConfigDoctorDetails_ExitSemanticsUnchanged(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	var o1, e1 strings.Builder
	if code := commands.RunConfigDoctor(false, []string{cfg}, &o1, &e1); code != 0 {
		t.Errorf("normal exit = %d, want 0", code)
	}
	var o2, e2 strings.Builder
	if code := commands.RunConfigDoctor(false, []string{"--strict", cfg}, &o2, &e2); code != 1 {
		t.Errorf("strict exit = %d, want 1", code)
	}
}
