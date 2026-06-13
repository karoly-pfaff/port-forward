package commands_test

// Black-box tests for the config doctor machine-readable config summary (v1.9
// Slice 9): config doctor JSON gains a deterministic top-level `config` summary
// (rule/enabled/disabled/protocol/group counts) derived only from the offline
// config. It flows through --json/--out, is unaffected by --explain/--strict,
// and is absent from the live doctor and from read/parse failures.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/client"
	"portier/cli/sources/commands"
)

// mixedConfig is a valid config with TCP/UDP, enabled/disabled, grouped and
// ungrouped rules (distinct loopback bindings ≥1024 so no advisory fires).
const mixedConfig = `[
	{"name": "A", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true, "group": "backend"},
	{"name": "B", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48001, "targetHost": "10.0.0.2", "targetPort": 8081, "enabled": false, "group": "backend"},
	{"name": "C", "protocol": "udp", "listenHost": "127.0.0.1", "listenPort": 48002, "targetHost": "10.0.0.3", "targetPort": 8082, "enabled": true, "group": "admin"},
	{"name": "D", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48003, "targetHost": "10.0.0.4", "targetPort": 8083, "enabled": true}
]`

// configSummaryFromJSON decodes a config doctor JSON report's `config` summary.
func configSummaryFromJSON(t *testing.T, raw map[string]any) map[string]any {
	t.Helper()
	cfg, ok := raw["config"].(map[string]any)
	if !ok {
		t.Fatalf("report has no `config` summary: %v", raw)
	}
	return cfg
}

func TestConfigDoctorSummary_MixedConfig(t *testing.T) {
	cfg := writeTempConfig(t, mixedConfig)
	report, code := configDoctorJSON(t, []string{cfg})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	s := configSummaryFromJSON(t, report)

	if s["ruleCount"].(float64) != 4 {
		t.Errorf("ruleCount = %v, want 4", s["ruleCount"])
	}
	if s["enabledRuleCount"].(float64) != 3 || s["disabledRuleCount"].(float64) != 1 {
		t.Errorf("enabled/disabled = %v/%v, want 3/1", s["enabledRuleCount"], s["disabledRuleCount"])
	}
	protocols, _ := s["protocols"].(map[string]any)
	if protocols["tcp"].(float64) != 3 || protocols["udp"].(float64) != 1 {
		t.Errorf("protocols = %v, want tcp:3 udp:1", protocols)
	}
	if s["groupCount"].(float64) != 2 || s["ungroupedRuleCount"].(float64) != 1 {
		t.Errorf("groupCount/ungrouped = %v/%v, want 2/1", s["groupCount"], s["ungroupedRuleCount"])
	}

	groups, _ := s["groups"].([]any)
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	g0, _ := groups[0].(map[string]any)
	g1, _ := groups[1].(map[string]any)
	// Sorted by name: admin before backend.
	if g0["name"] != "admin" || g1["name"] != "backend" {
		t.Errorf("groups not sorted by name: %v, %v", g0["name"], g1["name"])
	}
	if g1["ruleCount"].(float64) != 2 || g1["enabledRuleCount"].(float64) != 1 || g1["disabledRuleCount"].(float64) != 1 {
		t.Errorf("backend rollup wrong: %v", g1)
	}
}

func TestConfigDoctorSummary_WhitespaceGroupIsUngrouped(t *testing.T) {
	cfg := writeTempConfig(t, `[
		{"name": "A", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true, "group": "   "}
	]`)
	report, _ := configDoctorJSON(t, []string{cfg})
	s := configSummaryFromJSON(t, report)
	if s["groupCount"].(float64) != 0 || s["ungroupedRuleCount"].(float64) != 1 {
		t.Errorf("whitespace group should be ungrouped: groupCount=%v ungrouped=%v", s["groupCount"], s["ungroupedRuleCount"])
	}
}

func TestConfigDoctorSummary_EmptyConfig(t *testing.T) {
	cfg := writeTempConfig(t, `[]`)
	report, code := configDoctorJSON(t, []string{cfg})
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	s := configSummaryFromJSON(t, report)
	if s["ruleCount"].(float64) != 0 || s["groupCount"].(float64) != 0 || s["ungroupedRuleCount"].(float64) != 0 {
		t.Errorf("empty summary wrong: %v", s)
	}
	// groups is an empty array, not null.
	groups, ok := s["groups"].([]any)
	if !ok || len(groups) != 0 {
		t.Errorf("groups should be an empty array: %v", s["groups"])
	}
}

func TestConfigDoctorSummary_ValidationFailureStillIncludesSummary(t *testing.T) {
	// targetHost missing → validation failure, but parsing succeeded → summary present.
	cfg := writeTempConfig(t, `[
		{"name": "Broken", "protocol": "tcp", "listenHost": "127.0.0.1", "listenPort": 48000, "targetHost": "", "targetPort": 8080, "enabled": true}
	]`)
	report, code := configDoctorJSON(t, []string{cfg})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	s := configSummaryFromJSON(t, report)
	if s["ruleCount"].(float64) != 1 {
		t.Errorf("ruleCount = %v, want 1", s["ruleCount"])
	}
}

func TestConfigDoctorSummary_ParseFailureHasNoSummary(t *testing.T) {
	cfg := writeTempConfig(t, `{ not valid json`)
	report, code := configDoctorJSON(t, []string{cfg})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if _, ok := report["config"]; ok {
		t.Errorf("parse failure must not include a config summary: %v", report["config"])
	}
}

func TestConfigDoctorSummary_OutWritesSummary(t *testing.T) {
	cfg := writeTempConfig(t, mixedConfig)
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
	if _, ok := raw["config"]; !ok {
		t.Errorf("exported JSON should include the config summary: %v", raw)
	}
}

func TestConfigDoctorSummary_JSONOutParity(t *testing.T) {
	cfg := writeTempConfig(t, mixedConfig)
	outPath := filepath.Join(t.TempDir(), "report.json")
	var out, errBuf strings.Builder
	commands.RunConfigDoctor(true, []string{"--out", outPath, cfg}, &out, &errBuf)

	fileData, _ := os.ReadFile(outPath)
	// stdout JSON and file are byte-identical (both encode the same payload;
	// writePrettyJSON appends a trailing newline).
	if strings.TrimRight(out.String(), "\n") != strings.TrimRight(string(fileData), "\n") {
		t.Errorf("stdout and file differ:\nstdout:\n%s\nfile:\n%s", out.String(), fileData)
	}
}

func TestConfigDoctorSummary_ExplainDoesNotChangeSummary(t *testing.T) {
	cfg := writeTempConfig(t, mixedConfig)
	plain, _ := configDoctorJSON(t, []string{cfg})
	withExplain, _ := configDoctorJSON(t, []string{"--explain", cfg})

	a, _ := json.Marshal(plain["config"])
	b, _ := json.Marshal(withExplain["config"])
	if string(a) != string(b) {
		t.Errorf("config summary changed with --explain:\n%s\n%s", a, b)
	}
}

func TestConfigDoctorSummary_StrictDoesNotChangeSummary(t *testing.T) {
	cfg := writeTempConfig(t, warningOnlyConfig)
	plain, _ := configDoctorJSON(t, []string{cfg})
	strict, _ := configDoctorJSON(t, []string{"--strict", cfg})

	a, _ := json.Marshal(plain["config"])
	b, _ := json.Marshal(strict["config"])
	if string(a) != string(b) {
		t.Errorf("config summary changed with --strict:\n%s\n%s", a, b)
	}
}

func TestConfigDoctorSummary_LiveDoctorHasNoConfigSummary(t *testing.T) {
	srv := makeDoctorServer(t, doctorServerConfig{statuses: []map[string]any{statusEntry("r1", "healthy")}})
	defer srv.Close()
	var out, errBuf strings.Builder
	commands.RunDoctor(client.New(srv.URL), true, nil, &out, &errBuf)

	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON: %v", err)
	}
	if _, ok := raw["config"]; ok {
		t.Errorf("live doctor JSON must not include a config summary: %v", raw["config"])
	}
}
