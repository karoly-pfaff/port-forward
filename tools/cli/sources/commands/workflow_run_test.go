package commands_test

// Black-box tests for `portier workflow run` (v1.11 Slice 5): read-only execution
// of a valid workflow's steps. It executes only the safe step types by calling
// the policy evaluator/review/baseline-compare directly — never a shell command,
// never mutating any file, and contacting the runtime only (read-only) for a
// policy.check runtime step. Referenced files are read during the run.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

// --- fixtures ---

const wfStrictPolicy = `{"schemaVersion":1,"rules":{"requireGroup":true,"allowLanExposure":false,"allowPrivilegedPorts":false,"allowAutostart":false,"forbidDuplicateBindings":true}}`
const wfCleanConfig = `[{"name":"Admin","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48080,"targetHost":"h","targetPort":8080,"enabled":false,"group":"admin"}]`
const wfDirtyConfig = `[{"name":"X","protocol":"tcp","listenHost":"0.0.0.0","listenPort":80,"targetHost":"h","targetPort":8080,"enabled":true}]`
const wfEmptyBaseline = `{"schemaVersion":1,"createdAt":"2026-01-01T00:00:00Z","result":"passed","findings":[]}`

// writeWorkflowFile builds a workflow JSON file with the given steps and returns
// its path. Using json.Marshal keeps absolute (Windows) paths properly escaped.
func writeWorkflowFile(t *testing.T, name string, steps []map[string]any) string {
	t.Helper()
	wf := map[string]any{"schemaVersion": 1, "name": name, "steps": steps}
	data, err := json.Marshal(wf)
	if err != nil {
		t.Fatalf("marshal workflow: %v", err)
	}
	return writeTempFile(t, "wf.json", string(data))
}

// runWorkflowRun runs `workflow run --file <file>` offline (no runtime URL).
func runWorkflowRun(t *testing.T, jsonOutput bool, file string, extra ...string) (string, string, int) {
	t.Helper()
	args := append([]string{"--file", file}, extra...)
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRun(jsonOutput, commands.ConnFlags{}, args, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

type runJSON struct {
	Workflow string `json:"workflow"`
	Steps    []struct {
		ID       string          `json:"id"`
		Type     string          `json:"type"`
		Status   string          `json:"status"`
		ExitCode int             `json:"exitCode"`
		Message  string          `json:"message"`
		Report   json.RawMessage `json:"report"`
	} `json:"steps"`
	Summary struct {
		Total, Passed, Failed, Skipped int
	} `json:"summary"`
	Result string `json:"result"`
}

func decodeRun(t *testing.T, out string) runJSON {
	t.Helper()
	var r runJSON
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("decoding run JSON: %v\n%s", err, out)
	}
	return r
}

// --- policy.check ---

func TestWorkflowRun_PolicyCheckConfigPasses(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfCleanConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})
	out, _, code := runWorkflowRun(t, false, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "Result: passed") {
		t.Errorf("expected passed:\n%s", out)
	}
}

func TestWorkflowRun_PolicyCheckConfigFailsExit1(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfDirtyConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})
	_, _, code := runWorkflowRun(t, false, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestWorkflowRun_PolicyReviewPassesAndFails(t *testing.T) {
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	cur := writeTempFile(t, "cur.json", wfCleanConfig)
	cleanCand := writeTempFile(t, "cand.json", wfCleanConfig)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "r", "type": "policy.review", "current": cur, "candidate": cleanCand, "policy": pol}})
	if _, _, code := runWorkflowRun(t, false, wf); code != 0 {
		t.Fatalf("clean candidate review exit = %d, want 0", code)
	}
	dirtyCand := writeTempFile(t, "dcand.json", wfDirtyConfig)
	wf2 := writeWorkflowFile(t, "w", []map[string]any{{"id": "r", "type": "policy.review", "current": cur, "candidate": dirtyCand, "policy": pol}})
	if _, _, code := runWorkflowRun(t, false, wf2); code != 1 {
		t.Fatalf("dirty candidate review exit = %d, want 1", code)
	}
}

// --- baseline compare ---

func TestWorkflowRun_BaselineCompareReportFile(t *testing.T) {
	report := writeTempFile(t, "r.json", `{"result":"passed","findings":[]}`)
	baseline := writeTempFile(t, "b.json", wfEmptyBaseline)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "cmp", "type": "policy.baseline.compare", "baseline": baseline, "report": report}})
	_, _, code := runWorkflowRun(t, false, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestWorkflowRun_BaselineCompareReportFromNewFindings(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfDirtyConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	baseline := writeTempFile(t, "b.json", wfEmptyBaseline)
	wf := writeWorkflowFile(t, "policy-baseline-check", []map[string]any{
		{"id": "check-config", "type": "policy.check", "config": cfg, "policy": pol},
		{"id": "compare-baseline", "type": "policy.baseline.compare", "baseline": baseline, "reportFrom": "check-config"},
	})
	out, _, code := runWorkflowRun(t, true, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	r := decodeRun(t, out)
	if r.Steps[1].Status != "failed" || !strings.Contains(r.Steps[1].Message, "new") {
		t.Errorf("reportFrom compare should report new findings: %+v", r.Steps[1])
	}
}

func TestWorkflowRun_DependencyFailedDependentSkipped(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfCleanConfig)
	baseline := writeTempFile(t, "b.json", wfEmptyBaseline)
	// check-config references a policy file that does not exist → it fails → the
	// reportFrom compare is skipped → run fails (exit 1).
	wf := writeWorkflowFile(t, "w", []map[string]any{
		{"id": "check-config", "type": "policy.check", "config": cfg, "policy": filepath.Join(t.TempDir(), "missing.json")},
		{"id": "compare-baseline", "type": "policy.baseline.compare", "baseline": baseline, "reportFrom": "check-config"},
	})
	out, _, code := runWorkflowRun(t, true, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, out)
	}
	r := decodeRun(t, out)
	if r.Steps[0].Status != "failed" {
		t.Errorf("check-config should fail: %+v", r.Steps[0])
	}
	if r.Steps[1].Status != "skipped" || !strings.Contains(r.Steps[1].Message, "produced no policy report") {
		t.Errorf("compare-baseline should be skipped: %+v", r.Steps[1])
	}
	if r.Summary.Skipped != 1 || r.Result != "failed" {
		t.Errorf("summary/result = %+v / %s", r.Summary, r.Result)
	}
}

func TestWorkflowRun_MissingReferencedConfigFailsStep(t *testing.T) {
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{
		{"id": "c", "type": "policy.check", "config": filepath.Join(t.TempDir(), "missing-config.json"), "policy": pol},
	})
	out, _, code := runWorkflowRun(t, true, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	r := decodeRun(t, out)
	if r.Steps[0].Status != "failed" || r.Steps[0].ExitCode != 1 {
		t.Errorf("missing config should fail the step with exit 1: %+v", r.Steps[0])
	}
}

// --- runtime step ---

func TestWorkflowRun_RuntimeStepPasses(t *testing.T) {
	srv := makeExportServer(t, compliantRuntimeRules)
	defer srv.Close()
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "runtime": true, "policy": pol}})
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRun(false, commands.ConnFlags{URL: srv.URL}, []string{"--file", wf}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s\n%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), "Result: passed") {
		t.Errorf("expected passed:\n%s", out.String())
	}
}

func TestWorkflowRun_RuntimeUnreachableExit3(t *testing.T) {
	srv := makeExportServer(t, compliantRuntimeRules)
	deadURL := srv.URL
	srv.Close() // now connections are refused
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "runtime": true, "policy": pol}})
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRun(false, commands.ConnFlags{URL: deadURL}, []string{"--file", wf}, &out, &errBuf)
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (runtime unreachable)\n%s", code, out.String())
	}
}

func TestWorkflowRun_RuntimeBadURLExit1(t *testing.T) {
	// A runtime step with an invalid --url fails ResolveURL → step failed (exit 1),
	// NOT the runtime-unreachable exit 3 (no connection was attempted).
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "runtime": true, "policy": pol}})
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRun(false, commands.ConnFlags{URL: "ftp://bad-scheme"}, []string{"--file", wf}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (bad --url is a step failure, not unreachable)", code)
	}
}

func TestWorkflowRun_RuntimeAPIErrorExit1(t *testing.T) {
	// A non-connection runtime error (HTTP 500 from export) is a step failure
	// (exit 1), not the runtime-unreachable exit 3.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "runtime": true, "policy": pol}})
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRun(false, commands.ConnFlags{URL: srv.URL}, []string{"--file", wf}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (API error is a step failure)", code)
	}
}

func TestWorkflowRun_TwoRuntimeStepsFetchOnce(t *testing.T) {
	// Two runtime steps must share a single memoized runtime fetch. The export
	// server counts requests and fails the test if it is hit more than once.
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/config/export" {
			hits++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "1", "exportedAt": "2026-01-01T12:00:00Z", "rules": compliantRuntimeRules})
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer srv.Close()
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{
		{"id": "c1", "type": "policy.check", "runtime": true, "policy": pol},
		{"id": "c2", "type": "policy.check", "runtime": true, "policy": pol},
	})
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRun(false, commands.ConnFlags{URL: srv.URL}, []string{"--file", wf}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, out.String())
	}
	if hits != 1 {
		t.Errorf("runtime config fetched %d times, want exactly 1 (memoized)", hits)
	}
}

// --- invalid / malformed ---

func TestWorkflowRun_InvalidPlanExit1NoRun(t *testing.T) {
	wf := writeTempFile(t, "wf.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"x"}]}`)
	out, _, code := runWorkflowRun(t, false, wf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	// The plan (validation errors) is shown — NOT a run report.
	if strings.Contains(out, "Portier Workflow Run") || strings.Contains(out, "Result: passed") {
		t.Errorf("invalid workflow must not run:\n%s", out)
	}
	if !strings.Contains(out, "Result: invalid") {
		t.Errorf("expected the plan's invalid result:\n%s", out)
	}
}

func TestWorkflowRun_InvalidPlanDoesNotWriteOut(t *testing.T) {
	wf := writeTempFile(t, "wf.json", `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.review","current":"x"}]}`)
	outPath := filepath.Join(t.TempDir(), "report.json")
	if _, _, code := runWorkflowRun(t, false, wf, "--out", outPath); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("invalid workflow must not write the --out report (stat err=%v)", err)
	}
}

func TestWorkflowRun_MalformedExit2(t *testing.T) {
	wf := writeTempFile(t, "wf.json", `{not json`)
	if _, _, code := runWorkflowRun(t, false, wf); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowRun_MissingFileFlagExit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRun(false, commands.ConnFlags{}, nil, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "--file is required") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestWorkflowRun_UnreadableFileExit2(t *testing.T) {
	if _, _, code := runWorkflowRun(t, false, filepath.Join(t.TempDir(), "missing.json")); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowRun_MissingOutValueExit2(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfCleanConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})
	if _, _, code := runWorkflowRun(t, false, wf, "--out"); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWorkflowRun_HelpExit0(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunWorkflowRun(false, commands.ConnFlags{}, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier workflow run") {
		t.Errorf("help missing:\n%s", out.String())
	}
}

// --- JSON shape ---

func TestWorkflowRun_JSONShape(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfCleanConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "demo", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})
	out, _, code := runWorkflowRun(t, true, wf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	r := decodeRun(t, out)
	if r.Workflow != "demo" || r.Result != "passed" || r.Summary.Total != 1 || r.Summary.Passed != 1 {
		t.Errorf("unexpected run JSON: %+v", r)
	}
	if r.Steps[0].Status != "passed" || r.Steps[0].ExitCode != 0 || len(r.Steps[0].Report) == 0 {
		t.Errorf("step JSON wrong: %+v", r.Steps[0])
	}
}

// --- --out ---

func TestWorkflowRun_OutWritesFile(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfCleanConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})
	outPath := filepath.Join(t.TempDir(), "report.json")
	stdout, _, code := runWorkflowRun(t, false, wf, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Report written to "+outPath) {
		t.Errorf("missing write confirmation:\n%s", stdout)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading out file: %v", err)
	}
	if decodeRun(t, string(data)).Result != "passed" {
		t.Errorf("out file result wrong")
	}
}

func TestWorkflowRun_JSONOutByteIdentical(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfDirtyConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	baseline := writeTempFile(t, "b.json", wfEmptyBaseline)
	wf := writeWorkflowFile(t, "demo", []map[string]any{
		{"id": "check-config", "type": "policy.check", "config": cfg, "policy": pol},
		{"id": "compare-baseline", "type": "policy.baseline.compare", "baseline": baseline, "reportFrom": "check-config"},
	})
	outPath := filepath.Join(t.TempDir(), "report.json")
	stdout, _, code := runWorkflowRun(t, true, wf, "--out", outPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	fileBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading out file: %v", err)
	}
	if stdout != string(fileBytes) {
		t.Errorf("--json --out stdout and file differ:\nstdout:\n%q\nfile:\n%q", stdout, string(fileBytes))
	}
}

func TestWorkflowRun_OutWriteFailureExit1(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfCleanConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})
	badOut := filepath.Join(t.TempDir(), "nope", "report.json")
	_, errOut, code := runWorkflowRun(t, false, wf, "--out", badOut)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (write failure)", code)
	}
	if !strings.Contains(errOut, "Error writing") {
		t.Errorf("stderr = %q", errOut)
	}
}

// --- safety: no mutation of input files ---

func TestWorkflowRun_DoesNotMutateInputs(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfDirtyConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	baseline := writeTempFile(t, "b.json", wfEmptyBaseline)
	wf := writeWorkflowFile(t, "w", []map[string]any{
		{"id": "check-config", "type": "policy.check", "config": cfg, "policy": pol},
		{"id": "compare-baseline", "type": "policy.baseline.compare", "baseline": baseline, "reportFrom": "check-config"},
	})
	snapshot := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		return string(b)
	}
	before := []string{snapshot(cfg), snapshot(pol), snapshot(baseline), snapshot(wf)}
	if _, _, code := runWorkflowRun(t, false, wf); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	after := []string{snapshot(cfg), snapshot(pol), snapshot(baseline), snapshot(wf)}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("input file %d was mutated by the run", i)
		}
	}
}

// --- dispatch ---

func TestRunWorkflow_RunDispatch(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", wfCleanConfig)
	pol := writeTempFile(t, "pol.json", wfStrictPolicy)
	wf := writeWorkflowFile(t, "w", []map[string]any{{"id": "c", "type": "policy.check", "config": cfg, "policy": pol}})
	var out, errBuf strings.Builder
	code := commands.RunWorkflow(false, commands.ConnFlags{}, []string{"run", "--file", wf}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Portier Workflow Run") {
		t.Errorf("run dispatch output wrong:\n%s", out.String())
	}
}
