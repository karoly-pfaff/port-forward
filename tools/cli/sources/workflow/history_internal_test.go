package workflow

// White-box tests for the local workflow run history (v1.12 Slice 1): the compact
// projection from a WorkflowRun, code dedupe/sort, run-id shape, and the store's
// load/append/retention/clear behavior including missing and malformed files.
// Black-box command behavior is in package commands_test.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixedTime() time.Time {
	return time.Date(2026, 6, 14, 10, 15, 30, 0, time.UTC)
}

// makeRun builds a WorkflowRun with the given steps and recomputes its summary +
// result the same way Run does, so projection tests use realistic data.
func makeRun(name string, steps []WorkflowRunStep) WorkflowRun {
	run := WorkflowRun{Workflow: name, Steps: steps}
	run.Summary.Total = len(steps)
	for _, s := range steps {
		switch s.Status {
		case runStatusPassed:
			run.Summary.Passed++
		case runStatusSkipped:
			run.Summary.Skipped++
		default:
			run.Summary.Failed++
		}
	}
	run.Result = runResultPassed
	if run.Summary.Failed > 0 || run.Summary.Skipped > 0 {
		run.Result = runResultFailed
	}
	return run
}

func TestNewRunID_Shape(t *testing.T) {
	id := NewRunID(fixedTime(), "Policy Baseline Check!", "a1b2c3d4")
	want := "20260614T101530Z-policy-baseline-check-a1b2c3d4"
	if id != want {
		t.Errorf("NewRunID = %q, want %q", id, want)
	}
}

func TestNewRunID_EmptyWorkflowName(t *testing.T) {
	id := NewRunID(fixedTime(), "   ", "deadbeef")
	want := "20260614T101530Z-workflow-deadbeef"
	if id != want {
		t.Errorf("NewRunID = %q, want %q", id, want)
	}
}

func TestNewRunID_NoSuffix(t *testing.T) {
	id := NewRunID(fixedTime(), "wf", "")
	want := "20260614T101530Z-wf"
	if id != want {
		t.Errorf("NewRunID = %q, want %q", id, want)
	}
}

func TestProjectRun_CompactFields(t *testing.T) {
	run := makeRun("demo", []WorkflowRunStep{
		{ID: "check", Type: "policy.check", Status: runStatusPassed, ExitCode: 0, Message: "Policy check passed.", Report: map[string]any{"secret": "should not be projected"}},
		{ID: "compare", Type: "policy.baseline.compare", Status: runStatusFailed, ExitCode: 1, Message: "1 new finding.", explainCodes: []string{"policy.lan_exposure_forbidden"}},
	})

	got := ProjectRun(run, "id-1", fixedTime())

	if got.ID != "id-1" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.CreatedAt != "2026-06-14T10:15:30Z" {
		t.Errorf("CreatedAt = %q", got.CreatedAt)
	}
	if got.Workflow != "demo" || got.Result != runResultFailed {
		t.Errorf("Workflow/Result = %q/%q", got.Workflow, got.Result)
	}
	wantSummary := WorkflowRunSummary{Total: 2, Passed: 1, Failed: 1, Skipped: 0}
	if got.Summary != wantSummary {
		t.Errorf("Summary = %+v, want %+v", got.Summary, wantSummary)
	}
	wantSteps := []HistoryStep{
		{ID: "check", Type: "policy.check", Status: runStatusPassed, ExitCode: 0},
		{ID: "compare", Type: "policy.baseline.compare", Status: runStatusFailed, ExitCode: 1},
	}
	if !reflect.DeepEqual(got.Steps, wantSteps) {
		t.Errorf("Steps = %+v, want %+v", got.Steps, wantSteps)
	}
	if !reflect.DeepEqual(got.Codes, []string{"policy.lan_exposure_forbidden"}) {
		t.Errorf("Codes = %v", got.Codes)
	}
}

func TestProjectRun_NoSensitiveDataInJSON(t *testing.T) {
	// A step's embedded report and message must never appear in the projected JSON.
	run := makeRun("secrets-wf", []WorkflowRunStep{
		{
			ID:       "check",
			Type:     "policy.check",
			Status:   runStatusFailed,
			ExitCode: 1,
			Message:  "0.0.0.0 listen host SUPERSECRETHOST:65000 target",
			Report:   map[string]any{"rule": "TOPSECRET", "listenHost": "0.0.0.0", "targetHost": "internal.example.com"},
		},
	})
	got := ProjectRun(run, "id", fixedTime())
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, banned := range []string{"TOPSECRET", "SUPERSECRETHOST", "internal.example.com", "report", "message", "listenHost", "targetHost"} {
		if strings.Contains(string(data), banned) {
			t.Errorf("projected history JSON contains %q (must be excluded): %s", banned, data)
		}
	}
}

func TestProjectRun_CodesDedupedAndSorted(t *testing.T) {
	run := makeRun("wf", []WorkflowRunStep{
		{ID: "a", Type: "policy.check", Status: runStatusFailed, ExitCode: 1, explainCodes: []string{"policy.privileged_port_forbidden", "policy.lan_exposure_forbidden"}},
		{ID: "b", Type: "policy.check", Status: runStatusFailed, ExitCode: 1, explainCodes: []string{"policy.lan_exposure_forbidden"}},
		{ID: "c", Type: "policy.check", Status: runStatusPassed, ExitCode: 0, explainCodes: []string{"ignored.passed.code"}},
	})
	got := ProjectRun(run, "id", fixedTime())
	want := []string{"policy.lan_exposure_forbidden", "policy.privileged_port_forbidden"}
	if !reflect.DeepEqual(got.Codes, want) {
		t.Errorf("Codes = %v, want %v (deduped, sorted, passed steps excluded)", got.Codes, want)
	}
}

func TestProjectRun_NoCodesOmitted(t *testing.T) {
	run := makeRun("wf", []WorkflowRunStep{{ID: "a", Type: "policy.check", Status: runStatusPassed}})
	got := ProjectRun(run, "id", fixedTime())
	if got.Codes != nil {
		t.Errorf("Codes = %v, want nil", got.Codes)
	}
	data, _ := json.Marshal(got)
	if strings.Contains(string(data), "codes") {
		t.Errorf("expected codes omitted from JSON: %s", data)
	}
}

func TestHistoryStore_LoadMissingFile(t *testing.T) {
	store := NewHistoryStore(filepath.Join(t.TempDir(), "nope.json"))
	h, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for missing file", err)
	}
	if h.SchemaVersion != HistorySchemaVersion || len(h.Runs) != 0 {
		t.Errorf("Load() = %+v, want empty history with schema version", h)
	}
}

func TestHistoryStore_LoadEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.json")
	if err := os.WriteFile(path, []byte("   \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := NewHistoryStore(path).Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for empty file", err)
	}
	if h.SchemaVersion != HistorySchemaVersion || len(h.Runs) != 0 {
		t.Errorf("Load() = %+v", h)
	}
}

func TestHistoryStore_LoadMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewHistoryStore(path).Load(); err == nil {
		t.Fatal("Load() error = nil, want error for malformed file")
	}
}

func TestHistoryStore_AppendCreatesAndOrdersNewestFirst(t *testing.T) {
	// The parent directory does not exist yet — Append must create it.
	path := filepath.Join(t.TempDir(), "sub", "dir", "h.json")
	store := NewHistoryStore(path)

	if err := store.Append(HistoryRun{ID: "first"}); err != nil {
		t.Fatalf("Append first: %v", err)
	}
	if err := store.Append(HistoryRun{ID: "second"}); err != nil {
		t.Fatalf("Append second: %v", err)
	}

	h, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(h.Runs) != 2 || h.Runs[0].ID != "second" || h.Runs[1].ID != "first" {
		t.Errorf("runs = %+v, want newest-first [second, first]", h.Runs)
	}
	if h.SchemaVersion != HistorySchemaVersion {
		t.Errorf("schemaVersion = %d", h.SchemaVersion)
	}
}

func TestHistoryStore_RetentionKeepsLatest100(t *testing.T) {
	store := NewHistoryStore(filepath.Join(t.TempDir(), "h.json"))
	for i := 0; i < historyMaxRuns+25; i++ {
		if err := store.Append(HistoryRun{ID: runIDForIndex(i)}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	h, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(h.Runs) != historyMaxRuns {
		t.Fatalf("len(runs) = %d, want %d", len(h.Runs), historyMaxRuns)
	}
	// Newest first: the most recently appended id is index historyMaxRuns+24.
	if h.Runs[0].ID != runIDForIndex(historyMaxRuns+24) {
		t.Errorf("newest = %q, want %q", h.Runs[0].ID, runIDForIndex(historyMaxRuns+24))
	}
	// The oldest 25 must have been dropped; the oldest kept is index 25.
	if h.Runs[historyMaxRuns-1].ID != runIDForIndex(25) {
		t.Errorf("oldest kept = %q, want %q", h.Runs[historyMaxRuns-1].ID, runIDForIndex(25))
	}
}

func runIDForIndex(i int) string { return "run-" + itoa(i) }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func filterRuns() []HistoryRun {
	// Newest-first, as the store holds them.
	return []HistoryRun{
		{ID: "c", Workflow: "beta", Result: "failed", Codes: []string{"policy.autostart_forbidden"}},
		{ID: "b", Workflow: "beta", Result: "failed", Codes: []string{"policy.lan_exposure_forbidden", "workflow.run.input_failed"}},
		{ID: "a", Workflow: "alpha", Result: "passed"},
	}
}

func idsOf(runs []HistoryRun) []string {
	ids := make([]string, len(runs))
	for i, r := range runs {
		ids[i] = r.ID
	}
	return ids
}

func TestApplyHistoryFilters_NoFilters(t *testing.T) {
	got := ApplyHistoryFilters(filterRuns(), HistoryFilters{})
	if !reflect.DeepEqual(idsOf(got), []string{"c", "b", "a"}) {
		t.Errorf("ids = %v, want all newest-first", idsOf(got))
	}
}

func TestApplyHistoryFilters_AND(t *testing.T) {
	cases := []struct {
		name   string
		filter HistoryFilters
		want   []string
	}{
		{"result", HistoryFilters{Result: "failed"}, []string{"c", "b"}},
		{"workflow", HistoryFilters{Workflow: "beta"}, []string{"c", "b"}},
		{"code", HistoryFilters{Code: "policy.lan_exposure_forbidden"}, []string{"b"}},
		{"limit", HistoryFilters{Limit: 2}, []string{"c", "b"}},
		{"all AND", HistoryFilters{Result: "failed", Workflow: "beta", Code: "policy.autostart_forbidden"}, []string{"c"}},
		{"unmatched code", HistoryFilters{Code: "no.such.code"}, []string{}},
		{"limit after filter", HistoryFilters{Result: "failed", Limit: 1}, []string{"c"}},
	}
	for _, tc := range cases {
		got := idsOf(ApplyHistoryFilters(filterRuns(), tc.filter))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: ids = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestApplyHistoryFilters_DoesNotMutateInput(t *testing.T) {
	runs := filterRuns()
	before := append([]HistoryRun{}, runs...)
	_ = ApplyHistoryFilters(runs, HistoryFilters{Result: "failed", Limit: 1})
	if !reflect.DeepEqual(runs, before) {
		t.Errorf("ApplyHistoryFilters mutated the input slice")
	}
}

func TestValidHistoryResult(t *testing.T) {
	for _, ok := range []string{"passed", "failed"} {
		if !ValidHistoryResult(ok) {
			t.Errorf("ValidHistoryResult(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "invalid", "PASSED", "skipped", "ok"} {
		if ValidHistoryResult(bad) {
			t.Errorf("ValidHistoryResult(%q) = true, want false", bad)
		}
	}
}

func TestBuildHistoryListView_NoFiltersOmitsFilters(t *testing.T) {
	view := BuildHistoryListView(History{Runs: filterRuns()}, HistoryFilters{})
	if view.Filters != nil {
		t.Errorf("filters should be nil when none active, got %+v", view.Filters)
	}
	if view.Shown != 3 || view.TotalStored != 3 {
		t.Errorf("shown/total = %d/%d, want 3/3", view.Shown, view.TotalStored)
	}
	// Runs must be non-nil (stable [] shape) even on an empty history.
	empty := BuildHistoryListView(History{}, HistoryFilters{})
	if empty.Runs == nil {
		t.Errorf("Runs should be non-nil empty slice")
	}
}

func TestBuildHistoryListView_WithFilters(t *testing.T) {
	view := BuildHistoryListView(History{Runs: filterRuns()}, HistoryFilters{Result: "failed", Limit: 1})
	if view.Filters == nil || view.Filters.Result != "failed" || view.Filters.Limit != 1 {
		t.Fatalf("filters = %+v", view.Filters)
	}
	if view.Shown != 1 || view.TotalStored != 3 {
		t.Errorf("shown/total = %d/%d, want 1/3", view.Shown, view.TotalStored)
	}
	if idsOf(view.Runs)[0] != "c" {
		t.Errorf("expected newest failed run c, got %v", idsOf(view.Runs))
	}
}

// statsRuns returns a richer fixture for stats: 4 runs (newest-first c,d,b,a) so
// counts/sorting are non-trivial. (Order here is newest-first as the store holds.)
func statsRuns() []HistoryRun {
	return []HistoryRun{
		{ID: "d", Workflow: "beta", Result: "failed", Steps: []HistoryStep{{Type: "policy.check", Status: "passed"}, {Type: "policy.baseline.compare", Status: "failed"}}, Codes: []string{"policy.lan_exposure_forbidden"}},
		{ID: "c", Workflow: "beta", Result: "failed", Steps: []HistoryStep{{Type: "policy.check", Status: "failed"}}, Codes: []string{"policy.lan_exposure_forbidden", "policy.autostart_forbidden"}},
		{ID: "b", Workflow: "alpha", Result: "passed", Steps: []HistoryStep{{Type: "policy.review", Status: "passed"}, {Type: "policy.check", Status: "skipped"}}},
		{ID: "a", Workflow: "alpha", Result: "passed", Steps: []HistoryStep{{Type: "policy.check", Status: "passed"}}},
	}
}

func TestBuildHistoryStats_Empty(t *testing.T) {
	stats := BuildHistoryStats(History{}, HistoryFilters{})
	if stats.SchemaVersion != HistorySchemaVersion || stats.TotalStored != 0 || stats.Shown != 0 {
		t.Errorf("stats = %+v", stats)
	}
	if stats.Filters != nil {
		t.Errorf("filters should be nil when none active")
	}
	// Grouped slices must be non-nil for a stable shape.
	if stats.Workflows == nil || stats.StepTypes == nil || stats.Codes == nil {
		t.Errorf("grouped slices must be non-nil: %+v", stats)
	}
}

func TestBuildHistoryStats_Counts(t *testing.T) {
	stats := BuildHistoryStats(History{Runs: statsRuns()}, HistoryFilters{})
	if stats.TotalStored != 4 || stats.Shown != 4 {
		t.Fatalf("total/shown = %d/%d, want 4/4", stats.TotalStored, stats.Shown)
	}
	if stats.Results.Passed != 2 || stats.Results.Failed != 2 {
		t.Errorf("results = %+v, want 2/2", stats.Results)
	}
	// Steps: passed = a(1)+b.review(1)+d.check(1) = 3; failed = c.check(1)+d.compare(1) = 2; skipped = b(1) = 1.
	if stats.Steps.Passed != 3 || stats.Steps.Failed != 2 || stats.Steps.Skipped != 1 {
		t.Errorf("steps = %+v, want 3/2/1", stats.Steps)
	}
}

func TestBuildHistoryStats_GroupedSortingAndCounts(t *testing.T) {
	stats := BuildHistoryStats(History{Runs: statsRuns()}, HistoryFilters{})

	// Workflows: alpha=2, beta=2 → tie broken by name ascending → alpha, beta.
	if !reflect.DeepEqual(stats.Workflows, []WorkflowCount{{Name: "alpha", Count: 2}, {Name: "beta", Count: 2}}) {
		t.Errorf("workflows = %+v", stats.Workflows)
	}

	// Step types: policy.check appears in a,b,c,d steps = 4; policy.baseline.compare = 1; policy.review = 1.
	// Count desc then key asc → policy.check(4), policy.baseline.compare(1), policy.review(1).
	wantTypes := []StepTypeCount{{Type: "policy.check", Count: 4}, {Type: "policy.baseline.compare", Count: 1}, {Type: "policy.review", Count: 1}}
	if !reflect.DeepEqual(stats.StepTypes, wantTypes) {
		t.Errorf("stepTypes = %+v, want %+v", stats.StepTypes, wantTypes)
	}

	// Codes (runs containing the code): policy.lan_exposure_forbidden in c,d = 2;
	// policy.autostart_forbidden in c = 1. Count desc then code asc.
	wantCodes := []CodeCount{{Code: "policy.lan_exposure_forbidden", Count: 2}, {Code: "policy.autostart_forbidden", Count: 1}}
	if !reflect.DeepEqual(stats.Codes, wantCodes) {
		t.Errorf("codes = %+v, want %+v", stats.Codes, wantCodes)
	}
}

func TestBuildHistoryStats_CodeCountsRunsNotOccurrences(t *testing.T) {
	// A run with a duplicated code in its slice would still count once per run; the
	// store dedupes, but assert the run-level semantics explicitly.
	runs := []HistoryRun{
		{ID: "x", Workflow: "w", Result: "failed", Codes: []string{"c1"}},
		{ID: "y", Workflow: "w", Result: "failed", Codes: []string{"c1"}},
	}
	stats := BuildHistoryStats(History{Runs: runs}, HistoryFilters{})
	if !reflect.DeepEqual(stats.Codes, []CodeCount{{Code: "c1", Count: 2}}) {
		t.Errorf("codes = %+v, want c1=2 (one per run)", stats.Codes)
	}
}

func TestBuildHistoryStats_WithFilters(t *testing.T) {
	stats := BuildHistoryStats(History{Runs: statsRuns()}, HistoryFilters{Result: "failed"})
	if stats.Filters == nil || stats.Filters.Result != "failed" {
		t.Fatalf("filters = %+v", stats.Filters)
	}
	if stats.TotalStored != 4 || stats.Shown != 2 {
		t.Errorf("total/shown = %d/%d, want 4/2", stats.TotalStored, stats.Shown)
	}
	if stats.Results.Passed != 0 || stats.Results.Failed != 2 {
		t.Errorf("results = %+v, want 0/2 after result filter", stats.Results)
	}
	// Only beta runs (c,d) are failed.
	if !reflect.DeepEqual(stats.Workflows, []WorkflowCount{{Name: "beta", Count: 2}}) {
		t.Errorf("workflows = %+v", stats.Workflows)
	}
}

func TestBuildHistoryStats_EmptyMatchZeroStats(t *testing.T) {
	stats := BuildHistoryStats(History{Runs: statsRuns()}, HistoryFilters{Code: "no.such.code"})
	if stats.TotalStored != 4 || stats.Shown != 0 {
		t.Errorf("total/shown = %d/%d, want 4/0", stats.TotalStored, stats.Shown)
	}
	if stats.Results.Passed != 0 || stats.Results.Failed != 0 {
		t.Errorf("results should be zero on no match: %+v", stats.Results)
	}
	if len(stats.Workflows) != 0 || len(stats.StepTypes) != 0 || len(stats.Codes) != 0 {
		t.Errorf("grouped tables should be empty on no match: %+v", stats)
	}
}

func TestBuildHistoryStats_DoesNotMutateInput(t *testing.T) {
	runs := statsRuns()
	before := append([]HistoryRun{}, runs...)
	_ = BuildHistoryStats(History{Runs: runs}, HistoryFilters{Result: "failed", Limit: 1})
	if !reflect.DeepEqual(runs, before) {
		t.Errorf("BuildHistoryStats mutated the input slice")
	}
}

// --- prune (Slice 5) ---

func intPtr(n int) *int { return &n }

func TestPruneHistory_KeepNewestN(t *testing.T) {
	h := History{Runs: filterRuns()} // newest-first: c, b, a
	kept, result := PruneHistory(h, HistoryPruneOptions{Keep: intPtr(2)})
	if got := idsOf(kept.Runs); !reflect.DeepEqual(got, []string{"c", "b"}) {
		t.Errorf("kept = %v, want newest two [c b]", got)
	}
	if result.Removed != 1 || result.Kept != 2 || result.TotalBefore != 3 || result.TotalAfter != 2 {
		t.Errorf("result = %+v", result)
	}
	if kept.SchemaVersion != HistorySchemaVersion {
		t.Errorf("schemaVersion = %d", kept.SchemaVersion)
	}
}

func TestPruneHistory_KeepZeroRemovesAll(t *testing.T) {
	kept, result := PruneHistory(History{Runs: filterRuns()}, HistoryPruneOptions{Keep: intPtr(0)})
	if len(kept.Runs) != 0 || result.Removed != 3 || result.Kept != 0 || result.TotalAfter != 0 {
		t.Errorf("kept=%v result=%+v", kept.Runs, result)
	}
}

func TestPruneHistory_NegativeKeepClampsToZero(t *testing.T) {
	// Defensive: the command rejects a negative --keep, but PruneHistory clamps a
	// negative Keep to 0 (remove all) rather than panicking on a negative slice.
	kept, result := PruneHistory(History{Runs: filterRuns()}, HistoryPruneOptions{Keep: intPtr(-1)})
	if len(kept.Runs) != 0 || result.Removed != 3 || result.Kept != 0 {
		t.Errorf("negative keep: kept=%v result=%+v, want all removed", kept.Runs, result)
	}
}

func TestPruneHistory_KeepMoreThanStoredRemovesNothing(t *testing.T) {
	kept, result := PruneHistory(History{Runs: filterRuns()}, HistoryPruneOptions{Keep: intPtr(10)})
	if result.Removed != 0 || result.Kept != 3 {
		t.Errorf("result = %+v, want removed 0 kept 3", result)
	}
	if !reflect.DeepEqual(idsOf(kept.Runs), []string{"c", "b", "a"}) {
		t.Errorf("kept = %v, want all newest-first", idsOf(kept.Runs))
	}
}

func TestPruneHistory_FilterRemovesMatching(t *testing.T) {
	// filterRuns: c(beta,failed), b(beta,failed), a(alpha,passed).
	// Remove result=failed → keeps only a.
	kept, result := PruneHistory(History{Runs: filterRuns()}, HistoryPruneOptions{Filters: HistoryFilters{Result: "failed"}})
	if got := idsOf(kept.Runs); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("kept = %v, want [a]", got)
	}
	if result.Removed != 2 || result.Kept != 1 {
		t.Errorf("result = %+v", result)
	}
}

func TestPruneHistory_FilterByWorkflowAndCode(t *testing.T) {
	keptW, rW := PruneHistory(History{Runs: filterRuns()}, HistoryPruneOptions{Filters: HistoryFilters{Workflow: "beta"}})
	if got := idsOf(keptW.Runs); !reflect.DeepEqual(got, []string{"a"}) || rW.Removed != 2 {
		t.Errorf("workflow prune kept=%v removed=%d", got, rW.Removed)
	}
	// Remove runs containing policy.lan_exposure_forbidden → only b has it.
	keptC, rC := PruneHistory(History{Runs: filterRuns()}, HistoryPruneOptions{Filters: HistoryFilters{Code: "policy.lan_exposure_forbidden"}})
	if got := idsOf(keptC.Runs); !reflect.DeepEqual(got, []string{"c", "a"}) || rC.Removed != 1 {
		t.Errorf("code prune kept=%v removed=%d", got, rC.Removed)
	}
}

func TestPruneHistory_FilterANDComposition(t *testing.T) {
	// Remove result=failed AND workflow=beta → c and b both match → keeps a.
	kept, result := PruneHistory(History{Runs: filterRuns()}, HistoryPruneOptions{Filters: HistoryFilters{Result: "failed", Workflow: "beta"}})
	if got := idsOf(kept.Runs); !reflect.DeepEqual(got, []string{"a"}) || result.Removed != 2 {
		t.Errorf("AND prune kept=%v removed=%d", got, result.Removed)
	}
}

func TestPruneHistory_EmptyMatchRemovesNothing(t *testing.T) {
	kept, result := PruneHistory(History{Runs: filterRuns()}, HistoryPruneOptions{Filters: HistoryFilters{Code: "no.such.code"}})
	if result.Removed != 0 || result.Kept != 3 {
		t.Errorf("result = %+v, want removed 0 kept 3", result)
	}
	if !reflect.DeepEqual(idsOf(kept.Runs), []string{"c", "b", "a"}) {
		t.Errorf("kept = %v", idsOf(kept.Runs))
	}
}

func TestPruneHistory_EmptyHistory(t *testing.T) {
	keptK, rK := PruneHistory(History{}, HistoryPruneOptions{Keep: intPtr(5)})
	if rK.Removed != 0 || rK.Kept != 0 || rK.TotalBefore != 0 || rK.TotalAfter != 0 || len(keptK.Runs) != 0 {
		t.Errorf("keep on empty: kept=%v result=%+v", keptK.Runs, rK)
	}
	keptF, rF := PruneHistory(History{}, HistoryPruneOptions{Filters: HistoryFilters{Result: "failed"}})
	if rF.Removed != 0 || rF.Kept != 0 {
		t.Errorf("filter on empty: result=%+v", rF)
	}
	_ = keptF
}

func TestPruneHistory_DoesNotMutateInput(t *testing.T) {
	runs := filterRuns()
	before := append([]HistoryRun{}, runs...)
	_, _ = PruneHistory(History{Runs: runs}, HistoryPruneOptions{Keep: intPtr(1)})
	if !reflect.DeepEqual(runs, before) {
		t.Errorf("PruneHistory mutated the input runs")
	}
	_, _ = PruneHistory(History{Runs: runs}, HistoryPruneOptions{Filters: HistoryFilters{Result: "failed"}})
	if !reflect.DeepEqual(runs, before) {
		t.Errorf("PruneHistory (filter) mutated the input runs")
	}
}

func TestHistoryStore_SaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wh.json")
	store := NewHistoryStore(path)
	if err := store.Save(History{Runs: []HistoryRun{{ID: "a"}, {ID: "b"}}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SchemaVersion != HistorySchemaVersion || len(got.Runs) != 2 || got.Runs[0].ID != "a" {
		t.Errorf("round-trip = %+v", got)
	}
}

func TestBuildHistoryExport_Empty(t *testing.T) {
	export := BuildHistoryExport(History{SchemaVersion: HistorySchemaVersion}, fixedTime())
	if export.SchemaVersion != HistoryExportSchemaVersion {
		t.Errorf("SchemaVersion = %d", export.SchemaVersion)
	}
	if export.Source != "workflow-history" {
		t.Errorf("Source = %q", export.Source)
	}
	if export.CreatedAt != "2026-06-14T10:15:30Z" {
		t.Errorf("CreatedAt = %q", export.CreatedAt)
	}
	if export.RunCount != 0 {
		t.Errorf("RunCount = %d, want 0", export.RunCount)
	}
	// An empty export must serialize runs as [] (not null) for a stable shape.
	data, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"runs":[]`) {
		t.Errorf("empty export should emit runs:[]; got %s", data)
	}
}

func TestBuildHistoryExport_PopulatedPreservesOrderAndCompactRuns(t *testing.T) {
	h := History{SchemaVersion: HistorySchemaVersion, Runs: []HistoryRun{
		{ID: "newest", Workflow: "b", Result: "failed", Summary: WorkflowRunSummary{Total: 1, Failed: 1}, Steps: []HistoryStep{{ID: "s", Type: "policy.check", Status: "failed", ExitCode: 1}}, Codes: []string{"policy.lan_exposure_forbidden"}},
		{ID: "oldest", Workflow: "a", Result: "passed", Summary: WorkflowRunSummary{Total: 1, Passed: 1}},
	}}
	export := BuildHistoryExport(h, fixedTime())
	if export.RunCount != 2 {
		t.Fatalf("RunCount = %d, want 2", export.RunCount)
	}
	if export.Runs[0].ID != "newest" || export.Runs[1].ID != "oldest" {
		t.Errorf("order not preserved newest-first: %+v", export.Runs)
	}
	if !reflect.DeepEqual(export.Runs[0].Codes, []string{"policy.lan_exposure_forbidden"}) {
		t.Errorf("codes not preserved: %v", export.Runs[0].Codes)
	}
}

func TestBuildHistoryExport_SafetyAllFalse(t *testing.T) {
	export := BuildHistoryExport(History{}, fixedTime())
	s := export.Safety
	if s.ContainsRawConfigs || s.ContainsRawPolicies || s.ContainsFullReports || s.ContainsLogs ||
		s.ContainsEnvironment || s.ContainsProcessData || s.ContainsRuntimeURLs || s.ContainsTokens {
		t.Errorf("safety flags must all be false, got %+v", s)
	}
	// All eight safety keys must be present in the JSON.
	data, _ := json.Marshal(export)
	for _, key := range []string{"containsRawConfigs", "containsRawPolicies", "containsFullReports", "containsLogs", "containsEnvironment", "containsProcessData", "containsRuntimeUrls", "containsTokens"} {
		if !strings.Contains(string(data), key) {
			t.Errorf("safety key %q missing from export JSON: %s", key, data)
		}
	}
}

func TestBuildHistoryExport_DoesNotMutateInput(t *testing.T) {
	h := History{SchemaVersion: HistorySchemaVersion, Runs: []HistoryRun{{ID: "a"}, {ID: "b"}}}
	before := append([]HistoryRun{}, h.Runs...)
	_ = BuildHistoryExport(h, fixedTime())
	if !reflect.DeepEqual(h.Runs, before) {
		t.Errorf("BuildHistoryExport mutated the input history: %+v", h.Runs)
	}
}

func TestHistoryStore_LoadDefaultsSchemaVersion(t *testing.T) {
	// A valid history JSON without a schemaVersion gets the current version.
	path := filepath.Join(t.TempDir(), "h.json")
	if err := os.WriteFile(path, []byte(`{"runs":[{"id":"a"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := NewHistoryStore(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if h.SchemaVersion != HistorySchemaVersion || len(h.Runs) != 1 {
		t.Errorf("Load() = %+v", h)
	}
}

func TestHistoryStore_ClearError(t *testing.T) {
	// Removing a path that is a non-empty directory fails with a non-not-exist
	// error, which Clear must surface.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NewHistoryStore(dir).Clear(); err == nil {
		t.Fatal("Clear() error = nil, want error removing a non-empty directory")
	}
}

func TestHistoryStore_AppendOnMalformedFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NewHistoryStore(path).Append(HistoryRun{ID: "x"}); err == nil {
		t.Fatal("Append() error = nil, want error when existing file is malformed")
	}
}

func TestHistoryStore_Clear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.json")
	store := NewHistoryStore(path)
	if err := store.Append(HistoryRun{ID: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still exists after Clear: stat err = %v", err)
	}
	// Clearing again (already missing) is a success.
	if err := store.Clear(); err != nil {
		t.Errorf("Clear on missing file = %v, want nil", err)
	}
}
