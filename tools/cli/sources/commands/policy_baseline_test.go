package commands_test

// Black-box tests for `portier policy baseline create|compare` (v1.10 Slice 7):
// saving an accepted policy report as a baseline and comparing a fresh report
// against it (new/resolved/unchanged). Fully offline — it operates on policy
// report JSON files (from policy check / policy review) and mutates nothing.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portier/cli/sources/commands"
)

// makeCheckReport runs `policy check --json --out` to produce a real report file.
func makeCheckReport(t *testing.T, configContent, policyContent string) string {
	t.Helper()
	cfg := writeTempFile(t, "cfg.json", configContent)
	pol := writeTempFile(t, "pol.json", policyContent)
	reportPath := filepath.Join(t.TempDir(), "report.json")
	var out, errBuf strings.Builder
	commands.RunPolicyCheck(true, commands.ConnFlags{},
		[]string{"--config", cfg, "--policy", pol, "--out", reportPath}, &out, &errBuf)
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("report not written: %v (stderr %s)", err, errBuf.String())
	}
	return reportPath
}

// makeBaseline runs `policy baseline create` and returns the baseline path.
func makeBaseline(t *testing.T, reportPath string) string {
	t.Helper()
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	var out, errBuf strings.Builder
	code := commands.RunPolicyBaselineCreate(false,
		[]string{"--from-report", reportPath, "--out", baselinePath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("baseline create failed (%d): %s", code, errBuf.String())
	}
	return baselinePath
}

func runBaselineCompare(t *testing.T, jsonOutput bool, baseline, report string) (string, string, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := commands.RunPolicyBaselineCompare(jsonOutput,
		[]string{"--baseline", baseline, "--report", report}, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// exposedEnabledConfig: Admin UI on 0.0.0.0:80, autostart on.
const exposedEnabledConfig = `[
	{"name": "Admin UI", "protocol": "tcp", "listenHost": "0.0.0.0", "listenPort": 80, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true, "group": "admin"}
]`

const lanPolicy = `{"schemaVersion":1,"rules":{"allowLanExposure":false}}`
const lanPrivPolicy = `{"schemaVersion":1,"rules":{"allowLanExposure":false,"allowPrivilegedPorts":false}}`
const lanAutostartPolicy = `{"schemaVersion":1,"rules":{"allowLanExposure":false,"allowAutostart":false}}`

// --- create ---

func TestPolicyBaselineCreate_FromCheckReport(t *testing.T) {
	report := makeCheckReport(t, exposedConfig, lanPolicy)
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	var out, errBuf strings.Builder
	code := commands.RunPolicyBaselineCreate(false,
		[]string{"--from-report", report, "--out", baselinePath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Baseline written to") {
		t.Errorf("missing confirmation:\n%s", out.String())
	}

	raw := readPolicyReportFile(t, baselinePath)
	if raw["schemaVersion"] != float64(1) {
		t.Errorf("schemaVersion = %v, want 1", raw["schemaVersion"])
	}
	if raw["result"] != "failed" {
		t.Errorf("result = %v, want failed", raw["result"])
	}
	if raw["source"] != "config file" {
		t.Errorf("source = %v, want \"config file\"", raw["source"])
	}
	// createdAt is a deterministic RFC3339 timestamp.
	createdAt, _ := raw["createdAt"].(string)
	if _, err := time.Parse(time.RFC3339, createdAt); err != nil {
		t.Errorf("createdAt %q is not RFC3339: %v", createdAt, err)
	}
	findings, _ := raw["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected 1 baseline finding, got %d", len(findings))
	}
	first, _ := findings[0].(map[string]any)
	if fp, _ := first["fingerprint"].(string); fp == "" {
		t.Errorf("baseline finding missing fingerprint: %v", first)
	}
	// Baseline must NOT embed raw config / details / explanations.
	for _, banned := range []string{"details", "rule", "binding", "explanations", "listenHost"} {
		if _, ok := first[banned]; ok {
			t.Errorf("baseline finding must not carry %q: %v", banned, first)
		}
	}
}

func TestPolicyBaselineCreate_FromReviewReport(t *testing.T) {
	cur := writeTempFile(t, "cur.json", compliantConfig)
	cand := writeTempFile(t, "cand.json", exposedConfig)
	pol := writeTempFile(t, "pol.json", lanPolicy)
	reportPath := filepath.Join(t.TempDir(), "review.json")
	var o, e strings.Builder
	commands.RunPolicyReview(true,
		[]string{"--current", cur, "--candidate", cand, "--policy", pol, "--out", reportPath}, &o, &e)

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	var out, errBuf strings.Builder
	code := commands.RunPolicyBaselineCreate(false,
		[]string{"--from-report", reportPath, "--out", baselinePath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("create from review report exit = %d: %s", code, errBuf.String())
	}
	raw := readPolicyReportFile(t, baselinePath)
	findings, _ := raw["findings"].([]any)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding from the review's candidate, got %d", len(findings))
	}
}

func TestPolicyBaselineCreate_JSONSummary(t *testing.T) {
	report := makeCheckReport(t, exposedConfig, lanPolicy)
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	var out, errBuf strings.Builder
	code := commands.RunPolicyBaselineCreate(true,
		[]string{"--from-report", report, "--out", baselinePath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decoding JSON summary: %v\n%s", err, out.String())
	}
	if raw["ok"] != true || raw["findingCount"] != float64(1) {
		t.Errorf("summary = %v", raw)
	}
}

func TestPolicyBaselineCreate_Errors(t *testing.T) {
	report := makeCheckReport(t, exposedConfig, lanPolicy)
	good := filepath.Join(t.TempDir(), "b.json")
	malformed := writeTempFile(t, "bad.json", `not json`)
	missing := filepath.Join(t.TempDir(), "nope.json")

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"missing --from-report", []string{"--out", good}, 2},
		{"missing --out", []string{"--from-report", report}, 2},
		{"unreadable report", []string{"--from-report", missing, "--out", good}, 2},
		{"malformed report", []string{"--from-report", malformed, "--out", good}, 2},
		{"write failure", []string{"--from-report", report, "--out", filepath.Join(t.TempDir(), "no-dir", "b.json")}, 1},
	}
	for _, c := range cases {
		var out, errBuf strings.Builder
		if code := commands.RunPolicyBaselineCreate(false, c.args, &out, &errBuf); code != c.want {
			t.Errorf("%s: exit = %d, want %d", c.name, code, c.want)
		}
	}
}

// --- compare ---

func TestPolicyBaselineCompare_NoChangesExit0(t *testing.T) {
	report := makeCheckReport(t, exposedConfig, lanPolicy)
	baseline := makeBaseline(t, report)
	out, _, code := runBaselineCompare(t, false, baseline, report)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "Result: passed") {
		t.Errorf("expected passed:\n%s", out)
	}
}

func TestPolicyBaselineCompare_NewFindingExit1(t *testing.T) {
	baseline := makeBaseline(t, makeCheckReport(t, exposedConfig, lanPolicy))
	// Fresh report has the lan finding PLUS a new privileged-port finding.
	fresh := makeCheckReport(t, exposedConfig, lanPrivPolicy)
	shape, code := runBaselineCompareJSON(t, baseline, fresh)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if shape.Summary.New != 1 || shape.Summary.Unchanged != 1 || shape.Summary.Resolved != 0 {
		t.Errorf("summary = %+v, want new1 unchanged1 resolved0", shape.Summary)
	}
	if shape.Result != "failed" {
		t.Errorf("result = %q, want failed", shape.Result)
	}
}

func TestPolicyBaselineCompare_ResolvedOnlyExit0(t *testing.T) {
	baseline := makeBaseline(t, makeCheckReport(t, exposedConfig, lanPolicy))
	// Fresh report is compliant → the lan finding is resolved, none new.
	fresh := makeCheckReport(t, compliantConfig, lanPolicy)
	shape, code := runBaselineCompareJSON(t, baseline, fresh)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (resolved-only must not fail)", code)
	}
	if shape.Summary.New != 0 || shape.Summary.Resolved != 1 {
		t.Errorf("summary = %+v, want new0 resolved1", shape.Summary)
	}
	if shape.Result != "passed" {
		t.Errorf("result = %q, want passed", shape.Result)
	}
}

func TestPolicyBaselineCompare_Mixed(t *testing.T) {
	// Baseline: lan + privileged (on Admin UI 0.0.0.0:80).
	baseline := makeBaseline(t, makeCheckReport(t, exposedConfig, lanPrivPolicy))
	// Fresh: lan (unchanged) + autostart (new); privileged resolved.
	fresh := makeCheckReport(t, exposedEnabledConfig, lanAutostartPolicy)
	shape, code := runBaselineCompareJSON(t, baseline, fresh)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if shape.Summary.New != 1 || shape.Summary.Resolved != 1 || shape.Summary.Unchanged != 1 {
		t.Errorf("summary = %+v, want new1 resolved1 unchanged1", shape.Summary)
	}
}

func TestPolicyBaselineCompare_HumanShape(t *testing.T) {
	baseline := makeBaseline(t, makeCheckReport(t, exposedConfig, lanPolicy))
	fresh := makeCheckReport(t, exposedConfig, lanPrivPolicy)
	out, _, code := runBaselineCompare(t, false, baseline, fresh)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	for _, want := range []string{
		"Portier Policy Baseline Compare",
		"New findings:",
		"policy.privileged_port_forbidden",
		"Resolved findings:",
		"Unchanged findings:",
		"policy.lan_exposure_forbidden",
		"Result: failed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q:\n%s", want, out)
		}
	}
}

func TestPolicyBaselineCompare_Errors(t *testing.T) {
	report := makeCheckReport(t, exposedConfig, lanPolicy)
	baseline := makeBaseline(t, report)
	malformed := writeTempFile(t, "bad.json", `not json`)
	badBaseline := writeTempFile(t, "badbase.json", `{"schemaVersion":99}`)
	missing := filepath.Join(t.TempDir(), "nope.json")

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"missing --baseline", []string{"--report", report}, 2},
		{"missing --report", []string{"--baseline", baseline}, 2},
		{"unreadable baseline", []string{"--baseline", missing, "--report", report}, 2},
		{"malformed baseline", []string{"--baseline", malformed, "--report", report}, 2},
		{"bad-schema baseline", []string{"--baseline", badBaseline, "--report", report}, 2},
		{"unreadable report", []string{"--baseline", baseline, "--report", missing}, 2},
		{"malformed report", []string{"--baseline", baseline, "--report", malformed}, 2},
	}
	for _, c := range cases {
		var out, errBuf strings.Builder
		if code := commands.RunPolicyBaselineCompare(false, c.args, &out, &errBuf); code != c.want {
			t.Errorf("%s: exit = %d, want %d", c.name, code, c.want)
		}
	}
}

// --- safety: offline, no mutation ---

func TestPolicyBaseline_DoesNotMutateInputs(t *testing.T) {
	report := makeCheckReport(t, exposedConfig, lanPolicy)
	baseline := makeBaseline(t, report)

	reportBefore, _ := os.ReadFile(report)
	baselineBefore, _ := os.ReadFile(baseline)
	runBaselineCompare(t, true, baseline, report)
	reportAfter, _ := os.ReadFile(report)
	baselineAfter, _ := os.ReadFile(baseline)

	if string(reportBefore) != string(reportAfter) {
		t.Error("compare mutated the report file")
	}
	if string(baselineBefore) != string(baselineAfter) {
		t.Error("compare mutated the baseline file")
	}
}

// --- dispatch / help ---

func TestPolicyBaseline_Dispatch(t *testing.T) {
	report := makeCheckReport(t, exposedConfig, lanPolicy)
	baselinePath := filepath.Join(t.TempDir(), "b.json")
	var out, errBuf strings.Builder
	if code := commands.RunPolicy(false, commands.ConnFlags{},
		[]string{"baseline", "create", "--from-report", report, "--out", baselinePath}, &out, &errBuf); code != 0 {
		t.Fatalf("RunPolicy baseline create exit = %d: %s", code, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	if code := commands.RunPolicyBaseline(false, nil, &out, &errBuf); code != 2 {
		t.Errorf("no-subcommand exit = %d, want 2", code)
	}
	out.Reset()
	errBuf.Reset()
	if code := commands.RunPolicyBaseline(false, []string{"help"}, &out, &errBuf); code != 0 {
		t.Errorf("help exit = %d, want 0", code)
	}
	out.Reset()
	errBuf.Reset()
	if code := commands.RunPolicyBaseline(false, []string{"bogus"}, &out, &errBuf); code != 2 {
		t.Errorf("unknown subcommand exit = %d, want 2", code)
	}
}

func TestPolicyBaseline_HelpAndBadFlags(t *testing.T) {
	cases := []struct {
		name string
		run  func(args []string, o, e *strings.Builder) int
		args []string
		want int
	}{
		{"create --help", func(a []string, o, e *strings.Builder) int { return commands.RunPolicyBaselineCreate(false, a, o, e) }, []string{"--help"}, 0},
		{"create bad flag", func(a []string, o, e *strings.Builder) int { return commands.RunPolicyBaselineCreate(false, a, o, e) }, []string{"--nope"}, 2},
		{"compare --help", func(a []string, o, e *strings.Builder) int { return commands.RunPolicyBaselineCompare(false, a, o, e) }, []string{"--help"}, 0},
		{"compare bad flag", func(a []string, o, e *strings.Builder) int { return commands.RunPolicyBaselineCompare(false, a, o, e) }, []string{"--nope"}, 2},
	}
	for _, c := range cases {
		var out, errBuf strings.Builder
		if code := c.run(c.args, &out, &errBuf); code != c.want {
			t.Errorf("%s: exit = %d, want %d", c.name, code, c.want)
		}
	}
}

func runBaselineCompareJSON(t *testing.T, baseline, report string) (baselineCompareShape, int) {
	t.Helper()
	out, errBuf, code := runBaselineCompare(t, true, baseline, report)
	var shape baselineCompareShape
	if err := json.Unmarshal([]byte(out), &shape); err != nil {
		t.Fatalf("decoding compare JSON: %v\nstdout:\n%s\nstderr:\n%s", err, out, errBuf)
	}
	return shape, code
}

type baselineCompareShape struct {
	Summary struct {
		New       int `json:"new"`
		Resolved  int `json:"resolved"`
		Unchanged int `json:"unchanged"`
	} `json:"summary"`
	Result    string           `json:"result"`
	New       []map[string]any `json:"new"`
	Resolved  []map[string]any `json:"resolved"`
	Unchanged []map[string]any `json:"unchanged"`
}
