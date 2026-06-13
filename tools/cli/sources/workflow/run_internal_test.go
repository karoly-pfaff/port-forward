package workflow

// White-box tests for workflow execution internals (v1.11 Slice 5): step
// dispatch, the per-step executors over injected readers, reportFrom in-memory
// consumption + fingerprint round-trip, dependency-skip, runtime-unreachable
// mapping, exit codes, and the human renderer. Black-box command behavior is in
// package commands_test.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"portier/cli/sources/config"
	"portier/cli/sources/policy"
)

// mustTime returns a fixed timestamp for deterministic baseline construction.
func mustTime() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

// mustJSON marshals v to a JSON string, failing the test on error.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

// fakeFS builds a RunDeps whose ReadFile serves an in-memory map (missing keys
// return an error) and whose RuntimeRules returns the given rules/err.
func fakeDeps(files map[string]string, runtimeRules []config.Rule, runtimeErr error) RunDeps {
	return RunDeps{
		ReadFile: func(path string) ([]byte, error) {
			if content, ok := files[path]; ok {
				return []byte(content), nil
			}
			return nil, fmt.Errorf("no such file %q", path)
		},
		RuntimeRules: func() ([]config.Rule, error) { return runtimeRules, runtimeErr },
	}
}

const strictPol = `{"schemaVersion":1,"rules":{"requireGroup":true,"allowLanExposure":false,"allowPrivilegedPorts":false,"allowAutostart":false,"forbidDuplicateBindings":true}}`
const cleanCfg = `[{"name":"Admin","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48080,"targetHost":"h","targetPort":8080,"enabled":false,"group":"admin"}]`
const dirtyCfg = `[{"name":"X","protocol":"tcp","listenHost":"0.0.0.0","listenPort":80,"targetHost":"h","targetPort":8080,"enabled":true}]`

func runFrom(t *testing.T, src string, deps RunDeps) WorkflowRun {
	t.Helper()
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if BuildPlan(f).Result != statusValid {
		t.Fatalf("fixture workflow is not valid: %s", src)
	}
	return Run(f, deps)
}

func TestRun_PolicyCheckConfigPasses(t *testing.T) {
	deps := fakeDeps(map[string]string{"cfg.json": cleanCfg, "pol.json": strictPol}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"name":"w","steps":[{"id":"c","type":"policy.check","config":"cfg.json","policy":"pol.json"}]}`, deps)
	if run.Result != runResultPassed || RunExitCode(run) != 0 {
		t.Fatalf("expected passed/0, got %s/%d: %+v", run.Result, RunExitCode(run), run)
	}
	if run.Steps[0].Status != runStatusPassed || run.Steps[0].ExitCode != 0 {
		t.Errorf("step = %+v", run.Steps[0])
	}
	if _, ok := run.Steps[0].Report.(policy.Report); !ok {
		t.Errorf("expected a policy.Report on the step, got %T", run.Steps[0].Report)
	}
}

func TestRun_PolicyCheckConfigFails(t *testing.T) {
	deps := fakeDeps(map[string]string{"cfg.json": dirtyCfg, "pol.json": strictPol}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","config":"cfg.json","policy":"pol.json"}]}`, deps)
	if run.Result != runResultFailed || RunExitCode(run) != 1 {
		t.Fatalf("expected failed/1, got %s/%d", run.Result, RunExitCode(run))
	}
	if run.Steps[0].Status != runStatusFailed {
		t.Errorf("step status = %s, want failed", run.Steps[0].Status)
	}
}

func TestRun_PolicyCheckRuntimePasses(t *testing.T) {
	rules, _ := config.ParseLocal([]byte(cleanCfg))
	deps := fakeDeps(map[string]string{"pol.json": strictPol}, rules, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","runtime":true,"policy":"pol.json"}]}`, deps)
	if run.Result != runResultPassed || RunExitCode(run) != 0 {
		t.Fatalf("expected passed/0, got %s/%d", run.Result, RunExitCode(run))
	}
}

func TestRun_PolicyCheckRuntimeUnreachableExit3(t *testing.T) {
	deps := fakeDeps(map[string]string{"pol.json": strictPol}, nil, fmt.Errorf("dial: %w", ErrRuntimeUnreachable))
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","runtime":true,"policy":"pol.json"}]}`, deps)
	if RunExitCode(run) != 3 {
		t.Fatalf("runtime unreachable should exit 3, got %d", RunExitCode(run))
	}
	if run.Steps[0].Status != runStatusFailed {
		t.Errorf("unreachable step status = %s, want failed", run.Steps[0].Status)
	}
	if run.Result != runResultFailed {
		t.Errorf("result = %s, want failed", run.Result)
	}
}

func TestRun_PolicyCheckRuntimeGenericErrorExit1(t *testing.T) {
	// A non-connection runtime error is a step failure (exit 1), not exit 3.
	deps := fakeDeps(map[string]string{"pol.json": strictPol}, nil, errors.New("boom"))
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","runtime":true,"policy":"pol.json"}]}`, deps)
	if RunExitCode(run) != 1 {
		t.Fatalf("generic runtime error should exit 1, got %d", RunExitCode(run))
	}
}

func TestRun_PolicyReviewPassesAndFails(t *testing.T) {
	pass := fakeDeps(map[string]string{"cur.json": cleanCfg, "cand.json": cleanCfg, "pol.json": strictPol}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"r","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"pol.json"}]}`, pass)
	if run.Result != runResultPassed {
		t.Errorf("clean candidate should pass, got %s", run.Result)
	}
	fail := fakeDeps(map[string]string{"cur.json": cleanCfg, "cand.json": dirtyCfg, "pol.json": strictPol}, nil, nil)
	run = runFrom(t, `{"schemaVersion":1,"steps":[{"id":"r","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"pol.json"}]}`, fail)
	if run.Result != runResultFailed || run.Steps[0].Status != runStatusFailed {
		t.Errorf("dirty candidate should fail, got %s/%s", run.Result, run.Steps[0].Status)
	}
}

func TestRun_BaselineCompareReportFilePasses(t *testing.T) {
	// A report file with no findings vs an empty baseline → no new findings.
	report := `{"result":"passed","findings":[]}`
	baseline := `{"schemaVersion":1,"createdAt":"2026-01-01T00:00:00Z","result":"passed","findings":[]}`
	deps := fakeDeps(map[string]string{"r.json": report, "b.json": baseline}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","report":"r.json"}]}`, deps)
	if run.Result != runResultPassed {
		t.Errorf("expected passed, got %s: %+v", run.Result, run.Steps[0])
	}
}

func TestRun_BaselineCompareReportFromNewFindings(t *testing.T) {
	// check-config fails (violations) → its in-memory report feeds compare; vs a
	// clean baseline that has NO findings, compare must report NEW findings.
	baseline := `{"schemaVersion":1,"createdAt":"2026-01-01T00:00:00Z","result":"passed","findings":[]}`
	deps := fakeDeps(map[string]string{"cfg.json": dirtyCfg, "pol.json": strictPol, "b.json": baseline}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[
		{"id":"check-config","type":"policy.check","config":"cfg.json","policy":"pol.json"},
		{"id":"compare-baseline","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"check-config"}
	]}`, deps)
	if run.Steps[1].Status != runStatusFailed {
		t.Fatalf("reportFrom compare should fail with new findings, got %s: %s", run.Steps[1].Status, run.Steps[1].Message)
	}
	cmp, ok := run.Steps[1].Report.(policy.CompareReport)
	if !ok {
		t.Fatalf("expected CompareReport, got %T", run.Steps[1].Report)
	}
	if cmp.Summary.New == 0 {
		t.Errorf("expected new findings from the in-memory report, got %+v", cmp.Summary)
	}
}

func TestRun_BaselineCompareReportFromMatchesBaseline(t *testing.T) {
	// A baseline built from the SAME dirty check report → reportFrom compare shows
	// no NEW findings (proves the in-memory fingerprint round-trip matches the
	// file-derived baseline fingerprints).
	cfgRules, _ := config.ParseLocal([]byte(dirtyCfg))
	pol, _ := policy.Parse([]byte(strictPol))
	rep := policy.Evaluate(cfgRules, pol)
	snap, err := snapshotFromReport(rep)
	if err != nil {
		t.Fatalf("snapshotFromReport: %v", err)
	}
	baseline := policy.BuildBaseline(snap, mustTime())
	baselineJSON := mustJSON(t, baseline)

	deps := fakeDeps(map[string]string{"cfg.json": dirtyCfg, "pol.json": strictPol, "b.json": baselineJSON}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[
		{"id":"check-config","type":"policy.check","config":"cfg.json","policy":"pol.json"},
		{"id":"compare-baseline","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"check-config"}
	]}`, deps)
	if run.Steps[1].Status != runStatusPassed {
		t.Fatalf("compare against a matching baseline should pass (no new findings), got %s: %s", run.Steps[1].Status, run.Steps[1].Message)
	}
}

func TestRun_DependencyFailedDependentSkipped(t *testing.T) {
	// check-config fails on an unreadable policy → produces no report → the
	// reportFrom compare is skipped, and the run fails.
	deps := fakeDeps(map[string]string{"cfg.json": cleanCfg, "b.json": `{"schemaVersion":1,"createdAt":"2026-01-01T00:00:00Z","result":"passed","findings":[]}`}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[
		{"id":"check-config","type":"policy.check","config":"cfg.json","policy":"missing-policy.json"},
		{"id":"compare-baseline","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"check-config"}
	]}`, deps)
	if run.Steps[0].Status != runStatusFailed {
		t.Errorf("check-config should fail (missing policy): %+v", run.Steps[0])
	}
	if run.Steps[1].Status != runStatusSkipped {
		t.Errorf("compare-baseline should be skipped: %+v", run.Steps[1])
	}
	if !strings.Contains(run.Steps[1].Message, "produced no policy report") {
		t.Errorf("skip reason = %q", run.Steps[1].Message)
	}
	if run.Result != runResultFailed || RunExitCode(run) != 1 {
		t.Errorf("a skip should fail the run with exit 1, got %s/%d", run.Result, RunExitCode(run))
	}
	if run.Summary.Skipped != 1 || run.Summary.Failed != 1 {
		t.Errorf("summary = %+v", run.Summary)
	}
}

func TestRun_UnreadableReferencedFileFailsStep(t *testing.T) {
	// A baseline-compare whose report FILE is unreadable → step failed exit 1.
	deps := fakeDeps(map[string]string{"b.json": `{"schemaVersion":1,"createdAt":"t","result":"passed","findings":[]}`}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","report":"missing.json"}]}`, deps)
	if run.Steps[0].Status != runStatusFailed || run.Steps[0].ExitCode != 1 {
		t.Errorf("unreadable report should fail the step with exit 1: %+v", run.Steps[0])
	}
}

// --- input-error branches (referenced files unreadable/malformed during run) ---

func TestRun_PolicyCheckInvalidConfigContentFails(t *testing.T) {
	deps := fakeDeps(map[string]string{"cfg.json": "{not a config", "pol.json": strictPol}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","config":"cfg.json","policy":"pol.json"}]}`, deps)
	if run.Steps[0].Status != runStatusFailed || !strings.Contains(run.Steps[0].Message, "Invalid config") {
		t.Errorf("invalid config content should fail the step: %+v", run.Steps[0])
	}
}

func TestRun_PolicyCheckInvalidPolicyContentFails(t *testing.T) {
	deps := fakeDeps(map[string]string{"cfg.json": cleanCfg, "pol.json": "{not a policy"}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","config":"cfg.json","policy":"pol.json"}]}`, deps)
	if run.Steps[0].Status != runStatusFailed || !strings.Contains(run.Steps[0].Message, "Invalid policy") {
		t.Errorf("invalid policy content should fail the step: %+v", run.Steps[0])
	}
}

func TestRun_PolicyReviewUnreadableCurrentFails(t *testing.T) {
	deps := fakeDeps(map[string]string{"cand.json": cleanCfg, "pol.json": strictPol}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"r","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"pol.json"}]}`, deps)
	if run.Steps[0].Status != runStatusFailed || !strings.Contains(run.Steps[0].Message, "current config") {
		t.Errorf("unreadable current should fail the step: %+v", run.Steps[0])
	}
}

func TestRun_PolicyReviewUnreadableCandidateFails(t *testing.T) {
	deps := fakeDeps(map[string]string{"cur.json": cleanCfg, "pol.json": strictPol}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"r","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"pol.json"}]}`, deps)
	if run.Steps[0].Status != runStatusFailed || !strings.Contains(run.Steps[0].Message, "candidate config") {
		t.Errorf("unreadable candidate should fail the step: %+v", run.Steps[0])
	}
}

func TestRun_PolicyReviewInvalidPolicyFails(t *testing.T) {
	deps := fakeDeps(map[string]string{"cur.json": cleanCfg, "cand.json": cleanCfg, "pol.json": "{bad"}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"r","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"pol.json"}]}`, deps)
	if run.Steps[0].Status != runStatusFailed {
		t.Errorf("invalid policy should fail the review step: %+v", run.Steps[0])
	}
}

func TestRun_BaselineCompareMalformedReportFileFails(t *testing.T) {
	deps := fakeDeps(map[string]string{"r.json": "{not a report", "b.json": `{"schemaVersion":1,"createdAt":"t","result":"passed","findings":[]}`}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","report":"r.json"}]}`, deps)
	if run.Steps[0].Status != runStatusFailed || !strings.Contains(run.Steps[0].Message, "Invalid report") {
		t.Errorf("malformed report should fail the step: %+v", run.Steps[0])
	}
}

func TestRun_BaselineCompareMalformedBaselineFails(t *testing.T) {
	deps := fakeDeps(map[string]string{"r.json": `{"result":"passed","findings":[]}`, "b.json": "{not a baseline"}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","report":"r.json"}]}`, deps)
	if run.Steps[0].Status != runStatusFailed || !strings.Contains(run.Steps[0].Message, "Invalid baseline") {
		t.Errorf("malformed baseline should fail the step: %+v", run.Steps[0])
	}
}

func TestRun_BaselineCompareUnreadableBaselineFails(t *testing.T) {
	deps := fakeDeps(map[string]string{"r.json": `{"result":"passed","findings":[]}`}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","report":"r.json"}]}`, deps)
	if run.Steps[0].Status != runStatusFailed || !strings.Contains(run.Steps[0].Message, "Error reading baseline") {
		t.Errorf("unreadable baseline should fail the step: %+v", run.Steps[0])
	}
}

func TestExecuteStep_UnknownTypeDefensiveFail(t *testing.T) {
	out := executeStep(Step{ID: "x", Type: "policy.apply"}, RunDeps{}, nil)
	if out.status != runStatusFailed {
		t.Errorf("unknown type should produce a failed outcome, got %s", out.status)
	}
}

func TestRunStatusTag(t *testing.T) {
	if runStatusTag(runStatusPassed) != "[PASSED]" || runStatusTag(runStatusFailed) != "[FAILED]" || runStatusTag(runStatusSkipped) != "[SKIPPED]" {
		t.Errorf("unexpected tags")
	}
}

func TestPrintRunHuman_Shape(t *testing.T) {
	deps := fakeDeps(map[string]string{"cfg.json": cleanCfg, "pol.json": strictPol}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"name":"demo","steps":[{"id":"c","type":"policy.check","config":"cfg.json","policy":"pol.json"}]}`, deps)
	var sb strings.Builder
	PrintRunHuman(run, false, &sb)
	out := sb.String()
	for _, want := range []string{"Portier Workflow Run", "Workflow: demo", "[PASSED]", "c  (policy.check)", "1 total", "1 passed", "Result: passed"} {
		if !strings.Contains(out, want) {
			t.Errorf("human run output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintRunHuman_Unnamed(t *testing.T) {
	deps := fakeDeps(map[string]string{"cfg.json": cleanCfg, "pol.json": strictPol}, nil, nil)
	run := runFrom(t, `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","config":"cfg.json","policy":"pol.json"}]}`, deps)
	var sb strings.Builder
	PrintRunHuman(run, false, &sb)
	if !strings.Contains(sb.String(), "Workflow: (unnamed)") {
		t.Errorf("missing unnamed placeholder:\n%s", sb.String())
	}
}
