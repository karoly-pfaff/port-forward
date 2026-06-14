package commands

// White-box tests for `portier workflow run --record-history` and
// `portier workflow history` (v1.12 Slice 1). These are white-box (package
// commands) because they override the unexported history path/suffix seams
// (historyStorePath / newHistorySuffix) to point at a temp file with deterministic
// ids — the only reason they are not black-box. They exercise the exported command
// handlers end to end.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"portier/cli/sources/workflow"
)

// histFailWriter is an io.Writer that always errors, to exercise JSON-encode
// failure branches.
type histFailWriter struct{}

func (histFailWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// useFailingHistoryPath points the path seam at a resolver that errors.
func useFailingHistoryPath(t *testing.T) {
	t.Helper()
	orig := historyStorePath
	historyStorePath = func() (string, error) { return "", errors.New("no config dir") }
	t.Cleanup(func() { historyStorePath = orig })
}

const histStrictPolicy = `{"schemaVersion":1,"rules":{"requireGroup":true,"allowLanExposure":false,"allowPrivilegedPorts":false,"allowAutostart":false,"forbidDuplicateBindings":true}}`
const histCleanConfig = `[{"name":"Admin","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48080,"targetHost":"h","targetPort":8080,"enabled":false,"group":"admin"}]`
const histDirtyConfig = `[{"name":"X","protocol":"tcp","listenHost":"0.0.0.0","listenPort":80,"targetHost":"h","targetPort":8080,"enabled":true}]`

// useHistoryPath points the history seam at path with deterministic suffixes and
// restores the originals on cleanup.
func useHistoryPath(t *testing.T, path string) {
	t.Helper()
	origPath := historyStorePath
	origSuffix := newHistorySuffix
	n := 0
	historyStorePath = func() (string, error) { return path, nil }
	newHistorySuffix = func() string { n++; return fmt.Sprintf("s%04d", n) }
	t.Cleanup(func() {
		historyStorePath = origPath
		newHistorySuffix = origSuffix
	})
}

// histTempFile writes content to a uniquely named file in the test temp dir.
func histTempFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return p
}

// histWorkflowFile builds a workflow JSON file with the given steps (json.Marshal
// keeps Windows paths escaped) and returns its path.
func histWorkflowFile(t *testing.T, name string, steps []map[string]any) string {
	t.Helper()
	wf := map[string]any{"schemaVersion": 1, "name": name, "steps": steps}
	data, err := json.Marshal(wf)
	if err != nil {
		t.Fatalf("marshal workflow: %v", err)
	}
	return histTempFile(t, "wf.json", string(data))
}

// loadHistory reads the history file at the seam path.
func loadHistory(t *testing.T, path string) workflow.History {
	t.Helper()
	h, err := workflow.NewHistoryStore(path).Load()
	if err != nil {
		t.Fatalf("loading history: %v", err)
	}
	return h
}

// --- record-history on workflow run ---

func TestWorkflowRunRecordHistory_RecordsCompactEntry(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)

	cfg := histTempFile(t, "cfg.json", histCleanConfig)
	pol := histTempFile(t, "pol.json", histStrictPolicy)
	wf := histWorkflowFile(t, "rec-pass", []map[string]any{
		{"id": "check", "type": "policy.check", "config": cfg, "policy": pol},
	})

	var out, errBuf strings.Builder
	code := RunWorkflowRun(false, ConnFlags{}, []string{"--file", wf, "--record-history"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s\n%s", code, out.String(), errBuf.String())
	}

	h := loadHistory(t, histPath)
	if len(h.Runs) != 1 {
		t.Fatalf("recorded runs = %d, want 1", len(h.Runs))
	}
	r := h.Runs[0]
	if r.Workflow != "rec-pass" || r.Result != "passed" {
		t.Errorf("entry = %+v", r)
	}
	if r.ID != "" && !strings.Contains(r.ID, "rec-pass") {
		t.Errorf("id = %q, want it to contain the workflow name", r.ID)
	}
	if r.Summary.Total != 1 || r.Summary.Passed != 1 {
		t.Errorf("summary = %+v", r.Summary)
	}
	if len(r.Steps) != 1 || r.Steps[0].ID != "check" || r.Steps[0].Type != "policy.check" || r.Steps[0].Status != "passed" {
		t.Errorf("steps = %+v", r.Steps)
	}

	// The raw stored file must never carry report payloads, messages, host data,
	// the runtime URL, env, or process data.
	raw, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatalf("read history file: %v", err)
	}
	for _, banned := range []string{"\"report\"", "\"message\"", "listenHost", "targetHost", "127.0.0.1", "48080", "http://", "PATH=", "USER="} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("history file contains banned content %q:\n%s", banned, raw)
		}
	}
}

func TestWorkflowRunNoRecordWithoutFlag(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)

	cfg := histTempFile(t, "cfg.json", histCleanConfig)
	pol := histTempFile(t, "pol.json", histStrictPolicy)
	wf := histWorkflowFile(t, "no-rec", []map[string]any{
		{"id": "check", "type": "policy.check", "config": cfg, "policy": pol},
	})

	var out, errBuf strings.Builder
	code := RunWorkflowRun(false, ConnFlags{}, []string{"--file", wf}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if _, err := os.Stat(histPath); !os.IsNotExist(err) {
		t.Errorf("history file should not exist without --record-history (stat err = %v)", err)
	}
}

func TestWorkflowRunRecordHistory_FailedRunRecordsFailedResult(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)

	cfg := histTempFile(t, "cfg.json", histDirtyConfig)
	pol := histTempFile(t, "pol.json", histStrictPolicy)
	wf := histWorkflowFile(t, "rec-fail", []map[string]any{
		{"id": "check", "type": "policy.check", "config": cfg, "policy": pol},
	})

	var out, errBuf strings.Builder
	code := RunWorkflowRun(false, ConnFlags{}, []string{"--file", wf, "--record-history"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (run failed)", code)
	}
	h := loadHistory(t, histPath)
	if len(h.Runs) != 1 || h.Runs[0].Result != "failed" {
		t.Fatalf("recorded run = %+v, want one failed run", h.Runs)
	}
	if len(h.Runs[0].Codes) == 0 {
		t.Errorf("expected emitted policy codes recorded for a failed run, got none")
	}
}

func TestWorkflowRunRecordHistory_InvalidPlanDoesNotRecord(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)

	// Missing required policy field → invalid plan.
	wf := histWorkflowFile(t, "bad", []map[string]any{
		{"id": "check", "type": "policy.check", "config": "x.json"},
	})

	var out, errBuf strings.Builder
	code := RunWorkflowRun(false, ConnFlags{}, []string{"--file", wf, "--record-history"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (invalid plan)", code)
	}
	if _, err := os.Stat(histPath); !os.IsNotExist(err) {
		t.Errorf("an invalid plan must not record history (stat err = %v)", err)
	}
}

func TestWorkflowRunRecordHistory_WriteFailurePassingRunExit1(t *testing.T) {
	// A regular file used as the parent directory makes the history write fail.
	blocker := histTempFile(t, "blocker", "x")
	useHistoryPath(t, filepath.Join(blocker, "wh.json"))

	cfg := histTempFile(t, "cfg.json", histCleanConfig)
	pol := histTempFile(t, "pol.json", histStrictPolicy)
	wf := histWorkflowFile(t, "rec", []map[string]any{
		{"id": "check", "type": "policy.check", "config": cfg, "policy": pol},
	})

	var out, errBuf strings.Builder
	code := RunWorkflowRun(false, ConnFlags{}, []string{"--file", wf, "--record-history"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (passing run + history write failure)", code)
	}
	if !strings.Contains(errBuf.String(), "failed to record workflow history") {
		t.Errorf("expected a history-failure warning, got: %s", errBuf.String())
	}
	if !strings.Contains(out.String(), "Result: passed") {
		t.Errorf("the run result must still be reported: %s", out.String())
	}
}

func TestWorkflowRunRecordHistory_WriteFailureKeepsRuntimeExit3(t *testing.T) {
	// A runtime-unreachable run exits 3; a history write failure must NOT downgrade
	// that to 1 — the workflow exit code is kept and a warning is added.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	srv.Close()

	blocker := histTempFile(t, "blocker", "x")
	useHistoryPath(t, filepath.Join(blocker, "wh.json"))

	pol := histTempFile(t, "pol.json", histStrictPolicy)
	wf := histWorkflowFile(t, "rt", []map[string]any{
		{"id": "check", "type": "policy.check", "runtime": true, "policy": pol},
	})

	var out, errBuf strings.Builder
	code := RunWorkflowRun(false, ConnFlags{URL: deadURL}, []string{"--file", wf, "--record-history"}, &out, &errBuf)
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (runtime unreachable kept despite history write failure)\n%s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "failed to record workflow history") {
		t.Errorf("expected a history-failure warning, got: %s", errBuf.String())
	}
}

// --- history list ---

func TestWorkflowHistoryList_Empty(t *testing.T) {
	useHistoryPath(t, filepath.Join(t.TempDir(), "wh.json"))
	var out, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "No workflow runs recorded.") {
		t.Errorf("expected empty message, got: %s", out.String())
	}
}

func TestWorkflowHistoryList_EmptyJSON(t *testing.T) {
	useHistoryPath(t, filepath.Join(t.TempDir(), "wh.json"))
	var out, errBuf strings.Builder
	code := RunWorkflowHistory(true, []string{"list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	view := decodeListView(t, out.String())
	if view.SchemaVersion != workflow.HistorySchemaVersion || view.Shown != 0 || view.TotalStored != 0 || len(view.Runs) != 0 {
		t.Errorf("view = %+v", view)
	}
	if view.Filters != nil {
		t.Errorf("filters should be omitted when none active, got %+v", view.Filters)
	}
	// runs must serialize as [] (not null) for a stable shape.
	if !strings.Contains(out.String(), `"runs": []`) {
		t.Errorf("empty list should emit runs: []; got %s", out.String())
	}
}

func TestWorkflowHistoryList_WithEntries(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	store := workflow.NewHistoryStore(histPath)
	if err := store.Append(workflow.HistoryRun{ID: "id-1", Workflow: "demo", Result: "failed", Summary: workflow.WorkflowRunSummary{Total: 2, Passed: 1, Failed: 1}, CreatedAt: "2026-06-14T10:15:30Z", Codes: []string{"policy.lan_exposure_forbidden"}}); err != nil {
		t.Fatal(err)
	}

	var out, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	s := out.String()
	for _, want := range []string{"id-1", "Workflow: demo", "Result: failed", "2 total, 1 passed, 1 failed", "policy.lan_exposure_forbidden"} {
		if !strings.Contains(s, want) {
			t.Errorf("list output missing %q:\n%s", want, s)
		}
	}
}

func TestWorkflowHistoryList_ReadFailure(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	if err := os.WriteFile(histPath, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	useHistoryPath(t, histPath)
	var out, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"list"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (malformed history)", code)
	}
}

// --- history list filtering (Slice 3) ---

func decodeListView(t *testing.T, s string) workflow.HistoryListView {
	t.Helper()
	var v workflow.HistoryListView
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("decode list view: %v\n%s", err, s)
	}
	return v
}

// seedFilterHistory writes three runs; appended a,b,c so newest-first is c,b,a.
func seedFilterHistory(t *testing.T) string {
	t.Helper()
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	store := workflow.NewHistoryStore(histPath)
	runs := []workflow.HistoryRun{
		{ID: "a", Workflow: "alpha", Result: "passed", Summary: workflow.WorkflowRunSummary{Total: 1, Passed: 1}},
		{ID: "b", Workflow: "beta", Result: "failed", Summary: workflow.WorkflowRunSummary{Total: 1, Failed: 1}, Codes: []string{"policy.lan_exposure_forbidden", "workflow.run.input_failed"}},
		{ID: "c", Workflow: "beta", Result: "failed", Summary: workflow.WorkflowRunSummary{Total: 1, Failed: 1}, Codes: []string{"policy.autostart_forbidden"}},
	}
	for _, r := range runs {
		if err := store.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	return histPath
}

func listIDs(view workflow.HistoryListView) []string {
	ids := make([]string, len(view.Runs))
	for i, r := range view.Runs {
		ids[i] = r.ID
	}
	return ids
}

func runFilterJSON(t *testing.T, args ...string) (workflow.HistoryListView, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := RunWorkflowHistory(true, append([]string{"list"}, args...), &out, &errBuf)
	if code != 0 {
		return workflow.HistoryListView{}, code
	}
	return decodeListView(t, out.String()), code
}

func TestWorkflowHistoryListFilter_Result(t *testing.T) {
	seedFilterHistory(t)
	view, code := runFilterJSON(t, "--result", "failed")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if got := listIDs(view); !reflect.DeepEqual(got, []string{"c", "b"}) {
		t.Errorf("ids = %v, want [c b] (newest-first failed)", got)
	}
	if view.Shown != 2 || view.TotalStored != 3 {
		t.Errorf("shown/total = %d/%d, want 2/3", view.Shown, view.TotalStored)
	}
}

func TestWorkflowHistoryListFilter_InvalidResult(t *testing.T) {
	seedFilterHistory(t)
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"list", "--result", "bogus"}, &out, &errBuf); code != 2 {
		t.Fatalf("exit = %d, want 2 (invalid result)", code)
	}
	// An explicit empty value is also rejected.
	out.Reset()
	errBuf.Reset()
	if code := RunWorkflowHistory(false, []string{"list", "--result", ""}, &out, &errBuf); code != 2 {
		t.Fatalf("empty result exit = %d, want 2", code)
	}
}

func TestWorkflowHistoryListFilter_Workflow(t *testing.T) {
	seedFilterHistory(t)
	view, code := runFilterJSON(t, "--workflow", "beta")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if got := listIDs(view); !reflect.DeepEqual(got, []string{"c", "b"}) {
		t.Errorf("ids = %v, want [c b]", got)
	}
	// Exact, case-sensitive: "Beta" matches nothing.
	view2, _ := runFilterJSON(t, "--workflow", "Beta")
	if view2.Shown != 0 {
		t.Errorf("case-sensitive workflow match should be empty, got %v", listIDs(view2))
	}
}

func TestWorkflowHistoryListFilter_WorkflowEmpty(t *testing.T) {
	seedFilterHistory(t)
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"list", "--workflow", ""}, &out, &errBuf); code != 2 {
		t.Fatalf("exit = %d, want 2 (empty workflow)", code)
	}
}

func TestWorkflowHistoryListFilter_Code(t *testing.T) {
	seedFilterHistory(t)
	view, code := runFilterJSON(t, "--code", "policy.lan_exposure_forbidden")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if got := listIDs(view); !reflect.DeepEqual(got, []string{"b"}) {
		t.Errorf("ids = %v, want [b]", got)
	}
}

func TestWorkflowHistoryListFilter_CodeEmpty(t *testing.T) {
	seedFilterHistory(t)
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"list", "--code", ""}, &out, &errBuf); code != 2 {
		t.Fatalf("exit = %d, want 2 (empty code)", code)
	}
}

func TestWorkflowHistoryListFilter_CodeUnmatchedIsEmptySuccess(t *testing.T) {
	seedFilterHistory(t)
	view, code := runFilterJSON(t, "--code", "no.such.code")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 for unmatched code", code)
	}
	if view.Shown != 0 || len(view.Runs) != 0 || view.TotalStored != 3 {
		t.Errorf("view = %+v, want empty match with totalStored 3", view)
	}
}

func TestWorkflowHistoryListFilter_Limit(t *testing.T) {
	seedFilterHistory(t)
	view, code := runFilterJSON(t, "--limit", "2")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if got := listIDs(view); !reflect.DeepEqual(got, []string{"c", "b"}) {
		t.Errorf("ids = %v, want newest two [c b]", got)
	}
	if view.Shown != 2 || view.TotalStored != 3 {
		t.Errorf("shown/total = %d/%d, want 2/3", view.Shown, view.TotalStored)
	}
}

func TestWorkflowHistoryListFilter_InvalidLimits(t *testing.T) {
	seedFilterHistory(t)
	for _, val := range []string{"0", "-1", "abc"} {
		var out, errBuf strings.Builder
		if code := RunWorkflowHistory(false, []string{"list", "--limit", val}, &out, &errBuf); code != 2 {
			t.Errorf("--limit %q exit = %d, want 2", val, code)
		}
	}
}

func TestWorkflowHistoryListFilter_ANDComposition(t *testing.T) {
	seedFilterHistory(t)
	view, code := runFilterJSON(t, "--result", "failed", "--workflow", "beta", "--code", "policy.autostart_forbidden", "--limit", "10")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if got := listIDs(view); !reflect.DeepEqual(got, []string{"c"}) {
		t.Errorf("ids = %v, want [c] (AND of all filters)", got)
	}
}

func TestWorkflowHistoryListFilter_JSONShape(t *testing.T) {
	seedFilterHistory(t)
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(true, []string{"list", "--result", "failed", "--limit", "5"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	view := decodeListView(t, out.String())
	if view.Filters == nil || view.Filters.Result != "failed" || view.Filters.Limit != 5 {
		t.Errorf("filters = %+v", view.Filters)
	}
	if view.Filters.Workflow != "" || view.Filters.Code != "" {
		t.Errorf("inactive filter fields should be empty/omitted: %+v", view.Filters)
	}
	if view.Shown != 2 || view.TotalStored != 3 {
		t.Errorf("shown/total = %d/%d", view.Shown, view.TotalStored)
	}
	// Inactive filter fields must be omitted from the `filters` object itself
	// (run entries legitimately carry their own "workflow" key, so inspect the
	// filters sub-object precisely).
	var raw struct {
		Filters map[string]any `json:"filters"`
	}
	if err := json.Unmarshal([]byte(out.String()), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	if _, ok := raw.Filters["workflow"]; ok {
		t.Errorf("inactive filter key 'workflow' should be omitted: %v", raw.Filters)
	}
	if _, ok := raw.Filters["code"]; ok {
		t.Errorf("inactive filter key 'code' should be omitted: %v", raw.Filters)
	}
	if _, ok := raw.Filters["result"]; !ok {
		t.Errorf("active filter key 'result' missing: %v", raw.Filters)
	}
}

func TestWorkflowHistoryListFilter_HumanShowsFiltersAndSummary(t *testing.T) {
	seedFilterHistory(t)
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"list", "--result", "failed", "--workflow", "beta", "--code", "policy.autostart_forbidden", "--limit", "5"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	s := out.String()
	for _, want := range []string{"Filters:", "- Result: failed", "- Workflow: beta", "- Code: policy.autostart_forbidden", "- Limit: 5", "Summary:", "- 1 shown", "- 3 total stored"} {
		if !strings.Contains(s, want) {
			t.Errorf("human output missing %q:\n%s", want, s)
		}
	}
}

func TestWorkflowHistoryListFilter_HumanNoMatch(t *testing.T) {
	seedFilterHistory(t)
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"list", "--code", "no.such.code"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	s := out.String()
	for _, want := range []string{"No workflow history entries matched the filters.", "- 0 shown", "- 3 total stored"} {
		if !strings.Contains(s, want) {
			t.Errorf("no-match output missing %q:\n%s", want, s)
		}
	}
}

func TestWorkflowHistoryListFilter_EmptyHistoryWithFilters(t *testing.T) {
	useHistoryPath(t, filepath.Join(t.TempDir(), "wh.json"))
	view, code := runFilterJSON(t, "--result", "failed")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if view.Shown != 0 || view.TotalStored != 0 || len(view.Runs) != 0 {
		t.Errorf("view = %+v, want empty", view)
	}
}

func TestWorkflowHistoryListFilter_ReadFailure(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	if err := os.WriteFile(histPath, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	useHistoryPath(t, histPath)
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"list", "--result", "failed"}, &out, &errBuf); code != 1 {
		t.Fatalf("exit = %d, want 1 (read failure with filters)", code)
	}
}

func TestWorkflowHistoryListFilter_DoesNotMutateStore(t *testing.T) {
	histPath := seedFilterHistory(t)
	before, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, code := runFilterJSON(t, "--result", "failed", "--limit", "1"); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	after, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("filtering mutated the history store")
	}
}

// --- history show ---

func TestWorkflowHistoryShow_Existing(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	store := workflow.NewHistoryStore(histPath)
	if err := store.Append(workflow.HistoryRun{ID: "the-id", Workflow: "demo", Result: "failed", Summary: workflow.WorkflowRunSummary{Total: 1, Failed: 1}, CreatedAt: "2026-06-14T10:15:30Z", Codes: []string{"policy.autostart_forbidden"}}); err != nil {
		t.Fatal(err)
	}

	var out, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"show", "the-id"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"the-id", "Workflow: demo", "Codes: policy.autostart_forbidden"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("show output missing %q:\n%s", want, out.String())
		}
	}
}

func TestWorkflowHistoryShow_JSON(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	store := workflow.NewHistoryStore(histPath)
	if err := store.Append(workflow.HistoryRun{ID: "the-id", Workflow: "demo", Result: "passed", Summary: workflow.WorkflowRunSummary{Total: 1, Passed: 1}}); err != nil {
		t.Fatal(err)
	}

	var out, errBuf strings.Builder
	code := RunWorkflowHistory(true, []string{"show", "the-id"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var r workflow.HistoryRun
	if err := json.Unmarshal([]byte(out.String()), &r); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if r.ID != "the-id" || r.Result != "passed" {
		t.Errorf("decoded = %+v", r)
	}
}

func TestWorkflowHistoryShow_UnknownIDExit1(t *testing.T) {
	useHistoryPath(t, filepath.Join(t.TempDir(), "wh.json"))
	var out, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"show", "nope"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (unknown id)", code)
	}
	if !strings.Contains(errBuf.String(), "no recorded run with id") {
		t.Errorf("expected unknown-id error, got: %s", errBuf.String())
	}
}

func TestWorkflowHistoryShow_MissingIDExit2(t *testing.T) {
	useHistoryPath(t, filepath.Join(t.TempDir(), "wh.json"))
	var out, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"show"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (missing id)", code)
	}
}

func TestWorkflowHistoryShow_ReadFailure(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	if err := os.WriteFile(histPath, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	useHistoryPath(t, histPath)
	var out, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"show", "any"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (malformed history)", code)
	}
}

// --- history clear ---

func TestWorkflowHistoryClear_WithYes(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	if err := workflow.NewHistoryStore(histPath).Append(workflow.HistoryRun{ID: "x"}); err != nil {
		t.Fatal(err)
	}

	var out, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"clear", "--yes"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if _, err := os.Stat(histPath); !os.IsNotExist(err) {
		t.Errorf("history file should be removed after clear (stat err = %v)", err)
	}
	if !strings.Contains(out.String(), "Workflow history cleared.") {
		t.Errorf("expected confirmation, got: %s", out.String())
	}
}

func TestWorkflowHistoryClear_WithoutYesExit2(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	if err := workflow.NewHistoryStore(histPath).Append(workflow.HistoryRun{ID: "x"}); err != nil {
		t.Fatal(err)
	}

	var out, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"clear"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (clear without --yes)", code)
	}
	if _, err := os.Stat(histPath); err != nil {
		t.Errorf("history file must NOT be removed without --yes: %v", err)
	}
}

func TestWorkflowHistoryClear_AlreadyEmptyExit0(t *testing.T) {
	useHistoryPath(t, filepath.Join(t.TempDir(), "wh.json"))
	var out, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"clear", "--yes"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (already empty)", code)
	}
}

// --- history export ---

// exportSnapshot is a minimal decode of the export file for assertions.
type exportSnapshot struct {
	SchemaVersion int    `json:"schemaVersion"`
	CreatedAt     string `json:"createdAt"`
	Source        string `json:"source"`
	RunCount      int    `json:"runCount"`
	Runs          []struct {
		ID    string   `json:"id"`
		Codes []string `json:"codes"`
	} `json:"runs"`
	Safety map[string]bool `json:"safety"`
}

func decodeExportFile(t *testing.T, path string) exportSnapshot {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}
	var snap exportSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("decode export file: %v\n%s", err, data)
	}
	return snap
}

func TestWorkflowHistoryExport_EmptyHistory(t *testing.T) {
	useHistoryPath(t, filepath.Join(t.TempDir(), "wh.json"))
	out := filepath.Join(t.TempDir(), "export.json")

	var stdout, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"export", "--out", out}, &stdout, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, errBuf.String())
	}
	snap := decodeExportFile(t, out)
	if snap.SchemaVersion != 1 || snap.Source != "workflow-history" || snap.RunCount != 0 || snap.Runs == nil {
		t.Errorf("empty export snapshot = %+v", snap)
	}
	for k, v := range snap.Safety {
		if v {
			t.Errorf("safety flag %q must be false", k)
		}
	}
	if len(snap.Safety) != 8 {
		t.Errorf("expected 8 safety flags, got %d", len(snap.Safety))
	}
	for _, want := range []string{"Workflow history export written.", "Runs: 0", "Compact metadata only"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("human output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkflowHistoryExport_Populated(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	store := workflow.NewHistoryStore(histPath)
	// Appended oldest-first; the store prepends, so "second" ends up newest.
	if err := store.Append(workflow.HistoryRun{ID: "first", Workflow: "a", Result: "passed", Summary: workflow.WorkflowRunSummary{Total: 1, Passed: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(workflow.HistoryRun{ID: "second", Workflow: "b", Result: "failed", Summary: workflow.WorkflowRunSummary{Total: 1, Failed: 1}, Codes: []string{"policy.lan_exposure_forbidden", "policy.autostart_forbidden"}}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "export.json")

	var stdout, errBuf strings.Builder
	code := RunWorkflowHistory(false, []string{"export", "--out", out}, &stdout, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, errBuf.String())
	}
	snap := decodeExportFile(t, out)
	if snap.RunCount != 2 {
		t.Fatalf("RunCount = %d, want 2", snap.RunCount)
	}
	if snap.Runs[0].ID != "second" || snap.Runs[1].ID != "first" {
		t.Errorf("export not newest-first: %+v", snap.Runs)
	}
	// Codes are kept exactly as stored — export does not re-order or re-dedupe
	// (deduping/sorting happens at record time via ProjectRun).
	if !reflect.DeepEqual(snap.Runs[0].Codes, []string{"policy.lan_exposure_forbidden", "policy.autostart_forbidden"}) {
		t.Errorf("codes = %v", snap.Runs[0].Codes)
	}
	if !strings.Contains(stdout.String(), "Runs: 2") {
		t.Errorf("human output missing run count:\n%s", stdout.String())
	}
}

func TestWorkflowHistoryExport_NoSensitiveContent(t *testing.T) {
	// Even if a run somehow carried a message/report-shaped value, the compact
	// export model has no such fields, so the file must be free of them.
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	if err := workflow.NewHistoryStore(histPath).Append(workflow.HistoryRun{
		ID:       "id-x",
		Workflow: "demo",
		Result:   "failed",
		Summary:  workflow.WorkflowRunSummary{Total: 1, Failed: 1},
		Steps:    []workflow.HistoryStep{{ID: "s", Type: "policy.check", Status: "failed", ExitCode: 1}},
		Codes:    []string{"policy.lan_exposure_forbidden"},
	}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "export.json")
	var stdout, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"export", "--out", out}, &stdout, &errBuf); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"\"report\"", "\"message\"", "listenHost", "targetHost", "http://", "PATH=", "USER=", "token"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("export file contains banned content %q:\n%s", banned, raw)
		}
	}
	// Safety object must explicitly state the exclusions.
	if !strings.Contains(string(raw), "\"safety\"") || !strings.Contains(string(raw), "containsTokens") {
		t.Errorf("export file missing safety statement:\n%s", raw)
	}
}

func TestWorkflowHistoryExport_JSONByteParity(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	if err := workflow.NewHistoryStore(histPath).Append(workflow.HistoryRun{ID: "id-x", Workflow: "demo", Result: "passed", Summary: workflow.WorkflowRunSummary{Total: 1, Passed: 1}}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "export.json")

	var stdout, errBuf strings.Builder
	code := RunWorkflowHistory(true, []string{"export", "--out", out}, &stdout, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, errBuf.String())
	}
	fileBytes, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != string(fileBytes) {
		t.Errorf("--json stdout and --out file are not byte-identical\nstdout:\n%s\nfile:\n%s", stdout.String(), fileBytes)
	}
}

func TestWorkflowHistoryExport_MissingOut(t *testing.T) {
	useHistoryPath(t, filepath.Join(t.TempDir(), "wh.json"))
	var stdout, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"export"}, &stdout, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2 (missing --out)", code)
	}
}

func TestWorkflowHistoryExport_ReadFailure(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	if err := os.WriteFile(histPath, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	useHistoryPath(t, histPath)
	out := filepath.Join(t.TempDir(), "export.json")
	var stdout, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"export", "--out", out}, &stdout, &errBuf); code != 1 {
		t.Fatalf("exit code = %d, want 1 (malformed history)", code)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("no export file should be written on a read failure (stat err = %v)", err)
	}
}

func TestWorkflowHistoryExport_WriteFailure(t *testing.T) {
	useHistoryPath(t, filepath.Join(t.TempDir(), "wh.json"))
	// A regular file used as the parent directory makes the output write fail
	// (output.WritePrettyJSON does not create parent directories).
	blocker := histTempFile(t, "blocker", "x")
	out := filepath.Join(blocker, "export.json")
	var stdout, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"export", "--out", out}, &stdout, &errBuf); code != 1 {
		t.Fatalf("exit code = %d, want 1 (write failure)", code)
	}
	if !strings.Contains(errBuf.String(), "Error writing") {
		t.Errorf("expected a write error, got: %s", errBuf.String())
	}
}

func TestWorkflowHistoryExport_DoesNotMutateStore(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	if err := workflow.NewHistoryStore(histPath).Append(workflow.HistoryRun{ID: "id-x", Workflow: "demo", Result: "passed", Summary: workflow.WorkflowRunSummary{Total: 1, Passed: 1}}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "export.json")
	var stdout, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"export", "--out", out}, &stdout, &errBuf); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	after, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("export mutated the history store\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestWorkflowHistoryExport_JSONEncodeFailure(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	if err := workflow.NewHistoryStore(histPath).Append(workflow.HistoryRun{ID: "id-x"}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "export.json")
	var errBuf strings.Builder
	if code := runWorkflowHistoryExport(true, []string{"--out", out}, histFailWriter{}, &errBuf); code != 1 {
		t.Errorf("export JSON encode failure exit = %d, want 1", code)
	}
}

// --- dispatch ---

func TestWorkflowHistory_NoSubcommandExit2(t *testing.T) {
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(false, nil, &out, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestWorkflowHistory_UnknownSubcommandExit2(t *testing.T) {
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"bogus"}, &out, &errBuf); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestWorkflowHistory_HelpExit0(t *testing.T) {
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"help"}, &out, &errBuf); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "workflow history") {
		t.Errorf("expected help text, got: %s", out.String())
	}
}

// --- defaults / seams ---

func TestHistoryDefaults(t *testing.T) {
	// The real suffix generator returns 8 hex characters.
	s := newHistorySuffix()
	if len(s) != 8 {
		t.Errorf("newHistorySuffix length = %d (%q), want 8", len(s), s)
	}
	// The real path resolver produces a portier-scoped workflow-history path.
	p, err := historyStorePath()
	if err != nil {
		t.Skipf("user config dir unavailable: %v", err)
	}
	if !strings.Contains(p, "portier") || !strings.HasSuffix(p, workflow.HistoryFileName) {
		t.Errorf("historyStorePath = %q, want a portier-scoped %s path", p, workflow.HistoryFileName)
	}
}

// --- path-resolution failure (exit 1) ---

func TestWorkflowHistory_PathResolutionFailures(t *testing.T) {
	useFailingHistoryPath(t)
	for _, args := range [][]string{{"list"}, {"show", "any"}, {"export", "--out", "x.json"}, {"clear", "--yes"}} {
		var out, errBuf strings.Builder
		code := RunWorkflowHistory(false, args, &out, &errBuf)
		if code != 1 {
			t.Errorf("%v: exit code = %d, want 1 (path resolution failure)", args, code)
		}
		if !strings.Contains(errBuf.String(), "could not resolve history path") {
			t.Errorf("%v: expected path-resolution error, got: %s", args, errBuf.String())
		}
	}
}

// --- flag parsing edge cases ---

func TestWorkflowHistory_FlagParseEdges(t *testing.T) {
	useHistoryPath(t, filepath.Join(t.TempDir(), "wh.json"))
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"list help", []string{"list", "--help"}, 0},
		{"list bad flag", []string{"list", "--bogus"}, 2},
		{"show help", []string{"show", "--help"}, 0},
		{"show bad flag", []string{"show", "--bogus"}, 2},
		{"show too many", []string{"show", "a", "b"}, 2},
		{"clear help", []string{"clear", "--help"}, 0},
		{"clear bad flag", []string{"clear", "--bogus"}, 2},
		{"export help", []string{"export", "--help"}, 0},
		{"export bad flag", []string{"export", "--bogus"}, 2},
		{"export missing out value", []string{"export", "--out"}, 2},
	}
	for _, tc := range cases {
		var out, errBuf strings.Builder
		if code := RunWorkflowHistory(false, tc.args, &out, &errBuf); code != tc.want {
			t.Errorf("%s: exit code = %d, want %d\n%s", tc.name, code, tc.want, errBuf.String())
		}
	}
}

// --- unnamed workflow rendering ---

func TestWorkflowHistory_UnnamedWorkflowRendered(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	if err := workflow.NewHistoryStore(histPath).Append(workflow.HistoryRun{ID: "id-x", Workflow: "", Result: "passed", Summary: workflow.WorkflowRunSummary{Total: 1, Passed: 1}}); err != nil {
		t.Fatal(err)
	}
	var out, errBuf strings.Builder
	if code := RunWorkflowHistory(false, []string{"list"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out.String(), "Workflow: (unnamed)") {
		t.Errorf("expected (unnamed) placeholder, got: %s", out.String())
	}
}

// --- JSON encode failures (exit 1) ---

func TestWorkflowHistory_JSONEncodeFailures(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "wh.json")
	useHistoryPath(t, histPath)
	if err := workflow.NewHistoryStore(histPath).Append(workflow.HistoryRun{ID: "id-x"}); err != nil {
		t.Fatal(err)
	}
	var errBuf strings.Builder
	if code := runWorkflowHistoryList(true, nil, histFailWriter{}, &errBuf); code != 1 {
		t.Errorf("list JSON encode failure exit = %d, want 1", code)
	}
	errBuf.Reset()
	if code := runWorkflowHistoryShow(true, []string{"id-x"}, histFailWriter{}, &errBuf); code != 1 {
		t.Errorf("show JSON encode failure exit = %d, want 1", code)
	}
}
