package commands_test

// Black-box tests for `portier policy check --explain` (v1.10 Slice 2): inline
// human explanations and an additive JSON explanations map for emitted policy
// findings. --explain must not change findings, summary, result, or exit code.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

// exposedConfig violates LAN-exposure and privileged-port guardrails.
const exposedConfig = `[
	{"name": "Admin UI", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 80, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": false, "group": "admin"}
]`

func runPolicyExplain(t *testing.T, jsonOutput bool, cfg, pol string) (string, string, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := commands.RunPolicyCheck(jsonOutput, []string{"--config", cfg, "--policy", pol, "--explain"}, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

func TestPolicyCheck_ExplainHuman(t *testing.T) {
	cfg := writeTempFile(t, "config.json", exposedConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false, "allowPrivilegedPorts": false}}`)
	out, _, code := runPolicyExplain(t, false, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	for _, want := range []string{
		"policy.lan_exposure_forbidden",
		"        Code: policy.lan_exposure_forbidden",
		"        Meaning:",
		"        What to do:",
		"policy.privileged_port_forbidden",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain human output missing %q\n%s", want, out)
		}
	}
}

func TestPolicyCheck_ExplainJSON(t *testing.T) {
	cfg := writeTempFile(t, "config.json", exposedConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false}}`)
	out, _, code := runPolicyExplain(t, true, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out)
	}
	// findings/summary/result still present.
	for _, k := range []string{"findings", "summary", "result"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("JSON missing %q with --explain: %s", k, out)
		}
	}
	exps, ok := raw["explanations"].(map[string]any)
	if !ok {
		t.Fatalf("JSON missing explanations map with --explain: %s", out)
	}
	e, ok := exps["policy.lan_exposure_forbidden"].(map[string]any)
	if !ok {
		t.Fatalf("explanations missing policy.lan_exposure_forbidden: %v", exps)
	}
	if e["code"] != "policy.lan_exposure_forbidden" || e["severity"] != "error" {
		t.Errorf("lan explanation = %v", e)
	}
	if e["meaning"] == "" || e["action"] == "" {
		t.Errorf("explanation missing meaning/action: %v", e)
	}
}

func TestPolicyCheck_ExplainOmittedWithoutFlag(t *testing.T) {
	cfg := writeTempFile(t, "config.json", exposedConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false}}`)
	var out, errBuf strings.Builder
	commands.RunPolicyCheck(true, []string{"--config", cfg, "--policy", pol}, &out, &errBuf)
	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v", err)
	}
	if _, ok := raw["explanations"]; ok {
		t.Errorf("explanations must be omitted without --explain:\n%s", out.String())
	}
}

func TestPolicyCheck_ExplainDoesNotChangeReport(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Bad", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 80, "targetHost": "h", "targetPort": 1, "enabled": true}
	]`)
	pol := writeTempFile(t, "policy.json", strictPolicy)

	plainOut, _, plainCode := runPolicyCheck(t, true, cfg, pol)
	explainOut, _, explainCode := runPolicyExplain(t, true, cfg, pol)

	if plainCode != explainCode {
		t.Errorf("exit code changed with --explain: %d vs %d", plainCode, explainCode)
	}
	var a, b map[string]any
	if err := json.Unmarshal([]byte(plainOut), &a); err != nil {
		t.Fatalf("decode plain: %v", err)
	}
	if err := json.Unmarshal([]byte(explainOut), &b); err != nil {
		t.Fatalf("decode explain: %v", err)
	}
	for _, k := range []string{"findings", "summary", "result"} {
		if !reflect.DeepEqual(a[k], b[k]) {
			t.Errorf("--explain changed %q:\n%v\nvs\n%v", k, a[k], b[k])
		}
	}
}

func TestPolicyCheck_ExplainValidFinding(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	out, _, code := runPolicyExplain(t, false, cfg, pol)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"Code: policy.valid", "Meaning:", "What to do:"} {
		if !strings.Contains(out, want) {
			t.Errorf("compliant --explain output missing %q\n%s", want, out)
		}
	}
}
