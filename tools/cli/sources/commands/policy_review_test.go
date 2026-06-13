package commands_test

// Black-box tests for `portier policy review` (v1.10 Slice 5): a fully offline,
// dry-run command that compares a current config with a candidate config and
// evaluates ONLY the candidate against a policy, reusing the policy check finding
// semantics. It never mutates the configs/policy and never contacts the runtime.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

// reviewCurrent: 2 rules, 1 enabled, 1 disabled, 1 group (web), 1 ungrouped.
const reviewCurrent = `[
	{"name":"A","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"h","targetPort":1,"enabled":true,"group":"web"},
	{"name":"B","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"h","targetPort":2,"enabled":false}
]`

// reviewCandidate: 3 rules, 3 enabled, 0 disabled, 2 groups (web,api), 0 ungrouped.
// delta vs current: rule +1, enabled +2, disabled -1, group +1, ungrouped -1.
const reviewCandidate = `[
	{"name":"A","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"h","targetPort":1,"enabled":true,"group":"web"},
	{"name":"B","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"h","targetPort":2,"enabled":true,"group":"api"},
	{"name":"C","protocol":"udp","listenHost":"127.0.0.1","listenPort":48002,"targetHost":"h","targetPort":3,"enabled":true,"group":"api"}
]`

type reviewCounts struct {
	RuleCount          int `json:"ruleCount"`
	EnabledRuleCount   int `json:"enabledRuleCount"`
	DisabledRuleCount  int `json:"disabledRuleCount"`
	GroupCount         int `json:"groupCount"`
	UngroupedRuleCount int `json:"ungroupedRuleCount"`
}

type reviewJSONShape struct {
	Review struct {
		Current   reviewCounts `json:"current"`
		Candidate reviewCounts `json:"candidate"`
		Delta     reviewCounts `json:"delta"`
	} `json:"review"`
	Findings []map[string]any `json:"findings"`
	Summary  map[string]any   `json:"summary"`
	Result   string           `json:"result"`
}

func runPolicyReview(t *testing.T, jsonOutput bool, current, candidate, policy string, extra ...string) (string, string, int) {
	t.Helper()
	args := append([]string{"--current", current, "--candidate", candidate, "--policy", policy}, extra...)
	var out, errBuf strings.Builder
	code := commands.RunPolicyReview(jsonOutput, args, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// --- pass / fail ---

func TestPolicyReview_CandidatePassesExit0(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", compliantConfig)
	pol := writeTempFile(t, "pol.json", strictPolicy)
	out, _, code := runPolicyReview(t, false, cur, cand, pol)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "Result: passed") {
		t.Errorf("expected passed result:\n%s", out)
	}
}

func TestPolicyReview_CandidateViolatesExit1(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", exposedConfig) // listens on 0.0.0.0:80
	pol := writeTempFile(t, "pol.json", `{"schemaVersion":1,"rules":{"allowLanExposure":false}}`)
	report, code := runPolicyReviewJSON(t, cur, cand, pol)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if report.Result != "failed" {
		t.Errorf("result = %q, want failed", report.Result)
	}
	if len(report.Findings) == 0 {
		t.Errorf("expected at least one finding for a violating candidate")
	}
}

// --- change summary / delta ---

func runPolicyReviewJSON(t *testing.T, current, candidate, policy string, extra ...string) (reviewJSONShape, int) {
	t.Helper()
	out, errBuf, code := runPolicyReview(t, true, current, candidate, policy, extra...)
	var shape reviewJSONShape
	if err := json.Unmarshal([]byte(out), &shape); err != nil {
		t.Fatalf("decoding review JSON: %v\nstdout:\n%s\nstderr:\n%s", err, out, errBuf)
	}
	return shape, code
}

func TestPolicyReview_SummaryAndDelta(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", reviewCandidate)
	pol := writeTempFile(t, "pol.json", permissivePolicy) // candidate passes regardless
	shape, code := runPolicyReviewJSON(t, cur, cand, pol)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	wantCurrent := reviewCounts{RuleCount: 2, EnabledRuleCount: 1, DisabledRuleCount: 1, GroupCount: 1, UngroupedRuleCount: 1}
	wantCandidate := reviewCounts{RuleCount: 3, EnabledRuleCount: 3, DisabledRuleCount: 0, GroupCount: 2, UngroupedRuleCount: 0}
	wantDelta := reviewCounts{RuleCount: 1, EnabledRuleCount: 2, DisabledRuleCount: -1, GroupCount: 1, UngroupedRuleCount: -1}
	if shape.Review.Current != wantCurrent {
		t.Errorf("current = %+v, want %+v", shape.Review.Current, wantCurrent)
	}
	if shape.Review.Candidate != wantCandidate {
		t.Errorf("candidate = %+v, want %+v", shape.Review.Candidate, wantCandidate)
	}
	if shape.Review.Delta != wantDelta {
		t.Errorf("delta = %+v, want %+v", shape.Review.Delta, wantDelta)
	}
}

func TestPolicyReview_JSONShape(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", reviewCandidate)
	pol := writeTempFile(t, "pol.json", permissivePolicy)
	out, _, code := runPolicyReview(t, true, cur, cand, pol)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out)
	}
	for _, key := range []string{"review", "findings", "summary", "result"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("review JSON missing %q: %s", key, out)
		}
	}
	if _, ok := raw["explanations"]; ok {
		t.Errorf("explanations must be omitted without --explain: %s", out)
	}
}

func TestPolicyReview_HumanShape(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", reviewCandidate)
	pol := writeTempFile(t, "pol.json", permissivePolicy)
	out, _, code := runPolicyReview(t, false, cur, cand, pol)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"Portier Policy Review",
		"Config changes:",
		"- Rules: 2 → 3 (+1)",
		"- Enabled rules: 1 → 3 (+2)",
		"- Ungrouped rules: 1 → 0 (-1)",
		"Policy findings:",
		"Result: passed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q:\n%s", want, out)
		}
	}
}

// --- input errors ---

func TestPolicyReview_MissingFlagsExit2(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", reviewCandidate)
	pol := writeTempFile(t, "pol.json", permissivePolicy)
	cases := [][]string{
		{"--candidate", cand, "--policy", pol},  // no --current
		{"--current", cur, "--policy", pol},     // no --candidate
		{"--current", cur, "--candidate", cand}, // no --policy
	}
	for _, args := range cases {
		var out, errBuf strings.Builder
		if code := commands.RunPolicyReview(false, args, &out, &errBuf); code != 2 {
			t.Errorf("args %v: exit = %d, want 2", args, code)
		}
	}
}

func TestPolicyReview_BadInputsExit2(t *testing.T) {
	good := writeTempFile(t, "good.json", reviewCurrent)
	pol := writeTempFile(t, "pol.json", permissivePolicy)
	malformed := writeTempFile(t, "bad.json", `not json`)
	badPolicy := writeTempFile(t, "badpol.json", `{"schemaVersion":99}`)
	missing := filepath.Join(t.TempDir(), "nope.json")

	cases := []struct {
		name                       string
		current, candidate, policy string
	}{
		{"missing current", missing, good, pol},
		{"malformed current", malformed, good, pol},
		{"missing candidate", good, missing, pol},
		{"malformed candidate", good, malformed, pol},
		{"missing policy", good, good, missing},
		{"unsupported schema", good, good, badPolicy},
	}
	for _, c := range cases {
		_, _, code := runPolicyReview(t, false, c.current, c.candidate, c.policy)
		if code != 2 {
			t.Errorf("%s: exit = %d, want 2", c.name, code)
		}
	}
}

// --- --out / --explain ---

func TestPolicyReview_OutJSONByteParity(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", reviewCandidate)
	pol := writeTempFile(t, "pol.json", permissivePolicy)
	outPath := filepath.Join(t.TempDir(), "review.json")

	out, _, code := runPolicyReview(t, true, cur, cand, pol, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	fileData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading review file: %v", err)
	}
	if out != string(fileData) {
		t.Errorf("stdout and file differ\nstdout:\n%q\nfile:\n%q", out, string(fileData))
	}
}

func TestPolicyReview_OutHumanWritesFileAndConfirms(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", reviewCandidate)
	pol := writeTempFile(t, "pol.json", permissivePolicy)
	outPath := filepath.Join(t.TempDir(), "review.json")

	out, _, code := runPolicyReview(t, false, cur, cand, pol, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Review written to") {
		t.Errorf("human stdout should confirm the export:\n%s", out)
	}
	raw := readPolicyReportFile(t, outPath)
	if _, ok := raw["review"]; !ok {
		t.Errorf("written file missing review block: %v", raw)
	}
}

func TestPolicyReview_OutWriteFailureExit1(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", reviewCandidate)
	pol := writeTempFile(t, "pol.json", permissivePolicy)
	badPath := filepath.Join(t.TempDir(), "no-such-dir", "review.json")

	_, errBuf, code := runPolicyReview(t, false, cur, cand, pol, "--out", badPath)
	if code != 1 {
		t.Errorf("exit = %d, want 1 (write failure)", code)
	}
	if !strings.Contains(errBuf, "Error writing") {
		t.Errorf("stderr should report the write failure: %q", errBuf)
	}
}

func TestPolicyReview_ExplainAddsExplanationsAndDoesNotChangeReport(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", exposedConfig)
	pol := writeTempFile(t, "pol.json", `{"schemaVersion":1,"rules":{"allowLanExposure":false}}`)

	plainOut, _, plainCode := runPolicyReview(t, true, cur, cand, pol)
	explainOut, _, explainCode := runPolicyReview(t, true, cur, cand, pol, "--explain")
	if plainCode != explainCode {
		t.Errorf("--explain changed exit code: %d vs %d", plainCode, explainCode)
	}

	var plain, withExplain map[string]any
	if err := json.Unmarshal([]byte(plainOut), &plain); err != nil {
		t.Fatalf("decode plain: %v", err)
	}
	if err := json.Unmarshal([]byte(explainOut), &withExplain); err != nil {
		t.Fatalf("decode explain: %v", err)
	}
	if _, ok := withExplain["explanations"]; !ok {
		t.Errorf("--explain should add an explanations map:\n%s", explainOut)
	}
	for _, k := range []string{"review", "findings", "summary", "result"} {
		pj, _ := json.Marshal(plain[k])
		ej, _ := json.Marshal(withExplain[k])
		if string(pj) != string(ej) {
			t.Errorf("--explain changed %q:\n%s\nvs\n%s", k, pj, ej)
		}
	}

	// Human --explain shows an inline explanation block for the finding.
	humanOut, _, _ := runPolicyReview(t, false, cur, cand, pol, "--explain")
	if !strings.Contains(humanOut, "Code: policy.lan_exposure_forbidden") {
		t.Errorf("human --explain missing inline explanation:\n%s", humanOut)
	}
}

// --- safety ---

func TestPolicyReview_DoesNotMutateInputs(t *testing.T) {
	curContent := reviewCurrent
	candContent := reviewCandidate
	polContent := permissivePolicy
	cur := writeTempFile(t, "cur.json", curContent)
	cand := writeTempFile(t, "cand.json", candContent)
	pol := writeTempFile(t, "pol.json", polContent)
	outPath := filepath.Join(t.TempDir(), "review.json")

	runPolicyReview(t, false, cur, cand, pol, "--explain", "--out", outPath)

	for path, want := range map[string]string{cur: curContent, cand: candContent, pol: polContent} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s back: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("input file %s was mutated:\n%s", path, string(got))
		}
	}
}

// --- dispatch / help ---

func TestPolicyReview_ViaRunPolicy(t *testing.T) {
	cur := writeTempFile(t, "cur.json", reviewCurrent)
	cand := writeTempFile(t, "cand.json", reviewCandidate)
	pol := writeTempFile(t, "pol.json", permissivePolicy)
	var out, errBuf strings.Builder
	code := commands.RunPolicy(false, commands.ConnFlags{}, []string{"review", "--current", cur, "--candidate", cand, "--policy", pol}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("RunPolicy review exit = %d, want 0\nstderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Portier Policy Review") {
		t.Errorf("RunPolicy did not route to review:\n%s", out.String())
	}
}

func TestPolicyReview_BadFlagExit2(t *testing.T) {
	var out, errBuf strings.Builder
	// An unknown flag → flag parse error (not ErrHelp) → exit 2 with help on stderr.
	code := commands.RunPolicyReview(false, []string{"--bogus"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "policy review") {
		t.Errorf("stderr should print usage: %q", errBuf.String())
	}
}

func TestPolicyReview_Help(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunPolicyReview(false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("--help exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "policy review") {
		t.Errorf("help should describe the command:\n%s", out.String())
	}
}
