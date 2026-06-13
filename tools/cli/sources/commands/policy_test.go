package commands_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
	"portier/cli/sources/policy"
)

// writeTempFile writes content to a uniquely named file in t.TempDir and returns
// its path. Used for both config and policy files in policy tests.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

// runPolicyCheck runs `policy check --config <cfg> --policy <pol>` and returns
// stdout, stderr, and the exit code.
func runPolicyCheck(t *testing.T, jsonOutput bool, configPath, policyPath string) (string, string, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := commands.RunPolicyCheck(jsonOutput, commands.ConnFlags{}, []string{"--config", configPath, "--policy", policyPath}, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// runPolicyCheckJSON runs `policy check --json` and decodes the report.
func runPolicyCheckJSON(t *testing.T, configPath, policyPath string) (policy.Report, int) {
	t.Helper()
	out, errBuf, code := runPolicyCheck(t, true, configPath, policyPath)
	var report policy.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("decoding policy JSON: %v\nstdout:\n%s\nstderr:\n%s", err, out, errBuf)
	}
	return report, code
}

// findingCodes returns the ordered list of finding codes in a report.
func findingCodes(report policy.Report) []string {
	codes := make([]string, len(report.Findings))
	for i, f := range report.Findings {
		codes[i] = f.Code
	}
	return codes
}

// hasFinding reports whether the report contains a finding with the given code.
func hasFinding(report policy.Report, code string) bool {
	for _, f := range report.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// --- shared fixtures ---

// permissivePolicy permits everything (the baseline). A compliant config passes.
const permissivePolicy = `{
	"schemaVersion": 1,
	"rules": {
		"requireGroup": false,
		"allowLanExposure": true,
		"allowPrivilegedPorts": true,
		"allowAutostart": true,
		"forbidDuplicateBindings": false
	}
}`

// strictPolicy turns on every guardrail.
const strictPolicy = `{
	"schemaVersion": 1,
	"rules": {
		"requireGroup": true,
		"allowLanExposure": false,
		"allowPrivilegedPorts": false,
		"allowAutostart": false,
		"forbidDuplicateBindings": true
	}
}`

// compliantConfig satisfies every strictPolicy guardrail (grouped, loopback,
// high port, autostart off, no duplicate binding).
const compliantConfig = `[
	{"name": "Admin UI", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": false, "group": "admin"}
]`

// --- happy path ---

func TestPolicyCheck_CompliantPasses(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if report.Result != "passed" {
		t.Errorf("result = %q, want passed", report.Result)
	}
	if !hasFinding(report, "policy.valid") {
		t.Errorf("compliant config should emit policy.valid; got %v", findingCodes(report))
	}
	if report.Summary.Error != 0 {
		t.Errorf("summary error = %d, want 0", report.Summary.Error)
	}
}

func TestPolicyCheck_PermissivePolicyIgnoresEverything(t *testing.T) {
	// A config that would violate every strict guardrail, but the permissive
	// policy enables none of them → passes.
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Exposed", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 22, "targetHost": "10.0.0.1", "targetPort": 22, "enabled": true}
	]`)
	pol := writeTempFile(t, "policy.json", permissivePolicy)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if report.Result != "passed" {
		t.Errorf("result = %q, want passed", report.Result)
	}
}

func TestPolicyCheck_EmptyRulesObjectIsPermissive(t *testing.T) {
	// An empty rules object means every field defaults to permissive.
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Exposed", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 22, "targetHost": "10.0.0.1", "targetPort": 22, "enabled": true}
	]`)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {}}`)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !hasFinding(report, "policy.valid") {
		t.Errorf("empty-rules policy should pass; got %v", findingCodes(report))
	}
}

// --- individual violations ---

func TestPolicyCheck_RequireGroupViolation(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Ungrouped", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": false}
	]`)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"requireGroup": true}}`)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if report.Result != "failed" {
		t.Errorf("result = %q, want failed", report.Result)
	}
	if !hasFinding(report, "policy.group_required") {
		t.Errorf("missing policy.group_required; got %v", findingCodes(report))
	}
	if report.Summary.Error != 1 {
		t.Errorf("summary error = %d, want 1", report.Summary.Error)
	}
}

func TestPolicyCheck_RequireGroupWhitespaceGroupViolates(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Whitespace", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": false, "group": "   "}
	]`)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"requireGroup": true}}`)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !hasFinding(report, "policy.group_required") {
		t.Errorf("a whitespace-only group must count as missing; got %v", findingCodes(report))
	}
}

func TestPolicyCheck_LanExposureViolation(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Admin UI", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": false, "group": "admin"}
	]`)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false}}`)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !hasFinding(report, "policy.lan_exposure_forbidden") {
		t.Errorf("missing policy.lan_exposure_forbidden; got %v", findingCodes(report))
	}
}

func TestPolicyCheck_PrivilegedPortViolation(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `[
		{"name": "SSH", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 22, "targetHost": "10.0.0.1", "targetPort": 22, "enabled": false, "group": "infra"}
	]`)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowPrivilegedPorts": false}}`)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !hasFinding(report, "policy.privileged_port_forbidden") {
		t.Errorf("missing policy.privileged_port_forbidden; got %v", findingCodes(report))
	}
}

func TestPolicyCheck_AutostartViolation(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Auto", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true, "group": "g"}
	]`)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowAutostart": false}}`)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !hasFinding(report, "policy.autostart_forbidden") {
		t.Errorf("missing policy.autostart_forbidden; got %v", findingCodes(report))
	}
}

func TestPolicyCheck_DuplicateBindingViolation(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `[
		{"name": "A", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": false, "group": "g"},
		{"name": "B", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48080, "targetHost": "10.0.0.2", "targetPort": 9090, "enabled": false, "group": "g"}
	]`)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"forbidDuplicateBindings": true}}`)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !hasFinding(report, "policy.duplicate_binding_forbidden") {
		t.Errorf("missing policy.duplicate_binding_forbidden; got %v", findingCodes(report))
	}
	// One finding per duplicated binding (not per rule).
	count := 0
	for _, f := range report.Findings {
		if f.Code == "policy.duplicate_binding_forbidden" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("duplicate-binding findings = %d, want 1 (one per binding group)", count)
	}
}

// --- ordering determinism ---

func TestPolicyCheck_MultipleViolationsDeterministicOrder(t *testing.T) {
	// One rule that violates group, LAN exposure, privileged port, and autostart,
	// plus a duplicate binding with a second rule. Per-rule findings must appear
	// in policy order; the duplicate-binding finding comes last.
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Bad", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 80, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true},
		{"name": "Dup", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 80, "targetHost": "10.0.0.2", "targetPort": 9090, "enabled": false, "group": "g"}
	]`)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	got := findingCodes(report)
	want := []string{
		// Rule "Bad" (file order, policy order within the rule):
		"policy.group_required",
		"policy.lan_exposure_forbidden",
		"policy.privileged_port_forbidden",
		"policy.autostart_forbidden",
		// Rule "Dup": only LAN exposure + privileged port (grouped, autostart off):
		"policy.lan_exposure_forbidden",
		"policy.privileged_port_forbidden",
		// Duplicate binding emitted last:
		"policy.duplicate_binding_forbidden",
	}
	if len(got) != len(want) {
		t.Fatalf("finding codes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// --- JSON output shape ---

func TestPolicyCheck_JSONShape(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Admin UI", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true, "group": "admin"}
	]`)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false}}`)
	var out, errBuf strings.Builder
	code := commands.RunPolicyCheck(true, commands.ConnFlags{}, []string{"--config", cfg, "--policy", pol}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out.String())
	}
	for _, key := range []string{"findings", "summary", "result"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON missing top-level %q\n%s", key, out.String())
		}
	}
	if raw["result"] != "failed" {
		t.Errorf("result = %v, want failed", raw["result"])
	}

	findings, ok := raw["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("findings is not a non-empty array: %v", raw["findings"])
	}
	first, _ := findings[0].(map[string]any)
	for _, key := range []string{"code", "severity", "title", "message", "details"} {
		if _, ok := first[key]; !ok {
			t.Errorf("finding missing %q: %v", key, first)
		}
	}
	details, _ := first["details"].(map[string]any)
	rule, ok := details["rule"].(map[string]any)
	if !ok {
		t.Fatalf("LAN-exposure finding details.rule missing: %v", details)
	}
	if rule["listenHost"] != "0.0.0.0" {
		t.Errorf("details.rule.listenHost = %v, want 0.0.0.0", rule["listenHost"])
	}
}

func TestPolicyCheck_ValidJSONHasNoDetailsLeak(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	report, _ := runPolicyCheckJSON(t, cfg, pol)
	if len(report.Findings) != 1 || report.Findings[0].Code != "policy.valid" {
		t.Fatalf("expected single policy.valid finding, got %v", findingCodes(report))
	}
	if report.Findings[0].Severity != "info" {
		t.Errorf("policy.valid severity = %q, want info", report.Findings[0].Severity)
	}
}

// --- human output shape ---

func TestPolicyCheck_HumanShape(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Admin UI", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true, "group": "admin"}
	]`)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowLanExposure": false}}`)
	out, _, code := runPolicyCheck(t, false, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	for _, want := range []string{
		"Portier Policy Check",
		"[ERROR]",
		"listens on 0.0.0.0",
		"Summary:",
		"1 error",
		"Result: failed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q\n%s", want, out)
		}
	}
}

func TestPolicyCheck_HumanCompliantShowsPassed(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	out, _, code := runPolicyCheck(t, false, cfg, pol)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Result: passed") {
		t.Errorf("compliant human output should report Result: passed\n%s", out)
	}
	if !strings.Contains(out, "[INFO]") {
		t.Errorf("compliant output should show the [INFO] policy.valid finding\n%s", out)
	}
}

// --- input / usage errors (exit 2) ---

func TestPolicyCheck_MissingConfigArg(t *testing.T) {
	pol := writeTempFile(t, "policy.json", permissivePolicy)
	var out, errBuf strings.Builder
	code := commands.RunPolicyCheck(false, commands.ConnFlags{}, []string{"--policy", pol}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "one of --config or --runtime is required") {
		t.Errorf("stderr missing config-source required message\n%s", errBuf.String())
	}
}

func TestPolicyCheck_MissingPolicyArg(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	var out, errBuf strings.Builder
	code := commands.RunPolicyCheck(false, commands.ConnFlags{}, []string{"--config", cfg}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "--policy is required") {
		t.Errorf("stderr missing --policy required message\n%s", errBuf.String())
	}
}

func TestPolicyCheck_UnreadableConfig(t *testing.T) {
	pol := writeTempFile(t, "policy.json", permissivePolicy)
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	_, _, code := runPolicyCheck(t, false, missing, pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestPolicyCheck_UnreadablePolicy(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	_, _, code := runPolicyCheck(t, false, cfg, missing)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestPolicyCheck_MalformedConfig(t *testing.T) {
	cfg := writeTempFile(t, "config.json", `{ not valid json `)
	pol := writeTempFile(t, "policy.json", permissivePolicy)
	_, errBuf, code := runPolicyCheck(t, false, cfg, pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf, "invalid config") {
		t.Errorf("stderr missing invalid config message\n%s", errBuf)
	}
}

func TestPolicyCheck_MalformedPolicy(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", `{ not valid json `)
	_, errBuf, code := runPolicyCheck(t, false, cfg, pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf, "invalid policy") {
		t.Errorf("stderr missing invalid policy message\n%s", errBuf)
	}
}

func TestPolicyCheck_UnsupportedSchemaVersion(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 2, "rules": {}}`)
	_, errBuf, code := runPolicyCheck(t, false, cfg, pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf, "schemaVersion") {
		t.Errorf("stderr missing schemaVersion message\n%s", errBuf)
	}
}

func TestPolicyCheck_MissingSchemaVersion(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", `{"rules": {}}`)
	_, _, code := runPolicyCheck(t, false, cfg, pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (schemaVersion is required)", code)
	}
}

func TestPolicyCheck_UnknownPolicyFieldRejected(t *testing.T) {
	// Documented behavior: unknown policy fields are rejected (exit 2), so a typo
	// cannot silently relax a guardrail.
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {"allowUdp": true}}`)
	_, _, code := runPolicyCheck(t, false, cfg, pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (unknown field rejected)", code)
	}
}

func TestPolicyCheck_UnknownTopLevelFieldRejected(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", `{"schemaVersion": 1, "rules": {}, "extra": true}`)
	_, _, code := runPolicyCheck(t, false, cfg, pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (unknown top-level field rejected)", code)
	}
}

func TestPolicyCheck_EmptyPolicyFileRejected(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", `   `)
	_, _, code := runPolicyCheck(t, false, cfg, pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (empty policy rejected)", code)
	}
}

// --- safety: offline + no mutation ---

func TestPolicyCheck_DoesNotMutateFiles(t *testing.T) {
	cfgContent := compliantConfig
	polContent := strictPolicy
	cfg := writeTempFile(t, "config.json", cfgContent)
	pol := writeTempFile(t, "policy.json", polContent)

	if _, _, code := runPolicyCheck(t, false, cfg, pol); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	gotCfg, _ := os.ReadFile(cfg)
	if string(gotCfg) != cfgContent {
		t.Errorf("config file was mutated:\n%s", gotCfg)
	}
	gotPol, _ := os.ReadFile(pol)
	if string(gotPol) != polContent {
		t.Errorf("policy file was mutated:\n%s", gotPol)
	}
}

func TestPolicyCheck_OfflineNoRuntimeNeeded(t *testing.T) {
	// The whole point: no client, no --url, no server. A violation still reports
	// deterministically with no network access.
	cfg := writeTempFile(t, "config.json", `[
		{"name": "Exposed", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": false, "group": "g"}
	]`)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !hasFinding(report, "policy.lan_exposure_forbidden") {
		t.Errorf("offline evaluation should still flag LAN exposure; got %v", findingCodes(report))
	}
}

// no_udp_policy documents the product stance: there is no UDP-specific policy in
// this slice. A UDP rule is evaluated exactly like a TCP rule (general guardrails
// only) — there is no allowUdp / udpForbidden rule.
func TestPolicyCheck_NoUDPSpecificPolicy(t *testing.T) {
	// allowUdp is not a known policy field, so a policy that tries to use it is
	// rejected (covered by TestPolicyCheck_UnknownPolicyFieldRejected). Here we
	// prove a UDP rule passes a strict-but-compliant policy just like TCP would.
	cfg := writeTempFile(t, "config.json", `[
		{"name": "UDP svc", "protocol": "udp", "listenHost": "127.0.0.1", "listenPort": 48081, "targetHost": "10.0.0.1", "targetPort": 9000, "enabled": false, "group": "media"}
	]`)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	report, code := runPolicyCheckJSON(t, cfg, pol)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (UDP is first-class; no UDP-specific restriction)", code)
	}
	if !hasFinding(report, "policy.valid") {
		t.Errorf("compliant UDP rule should pass; got %v", findingCodes(report))
	}
}

// --- dispatch + help ---

func TestRunPolicy_NoSubcommand(t *testing.T) {
	var out, errBuf strings.Builder
	if code := commands.RunPolicy(false, commands.ConnFlags{}, nil, &out, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunPolicy_Help(t *testing.T) {
	var out, errBuf strings.Builder
	if code := commands.RunPolicy(false, commands.ConnFlags{}, []string{"help"}, &out, &errBuf); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "policy check") {
		t.Errorf("policy help missing 'policy check'\n%s", out.String())
	}
}

func TestRunPolicy_UnknownSubcommand(t *testing.T) {
	var out, errBuf strings.Builder
	if code := commands.RunPolicy(false, commands.ConnFlags{}, []string{"bogus"}, &out, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunPolicy_DispatchesCheck(t *testing.T) {
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	var out, errBuf strings.Builder
	code := commands.RunPolicy(false, commands.ConnFlags{}, []string{"check", "--config", cfg, "--policy", pol}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestPolicyCheck_Help(t *testing.T) {
	var out, errBuf strings.Builder
	if code := commands.RunPolicyCheck(false, commands.ConnFlags{}, []string{"--help"}, &out, &errBuf); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "policy check") {
		t.Errorf("check help missing usage\n%s", out.String())
	}
}

func TestPolicyCheck_BadFlag(t *testing.T) {
	var out, errBuf strings.Builder
	if code := commands.RunPolicyCheck(false, commands.ConnFlags{}, []string{"--nope"}, &out, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestPolicyCheck_JSONEncodeFailure(t *testing.T) {
	// PrintJSON to a failing writer → exit 1 (operation failure), like other
	// JSON-emitting commands.
	cfg := writeTempFile(t, "config.json", compliantConfig)
	pol := writeTempFile(t, "policy.json", strictPolicy)
	var errBuf strings.Builder
	code := commands.RunPolicyCheck(true, commands.ConnFlags{}, []string{"--config", cfg, "--policy", pol}, failingWriter{}, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Error encoding JSON") {
		t.Errorf("stderr missing encode error\n%s", errBuf.String())
	}
}
