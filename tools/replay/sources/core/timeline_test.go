package core

import (
	"path/filepath"
	"testing"
)

// eventByID returns the timeline event with the given id, or fails.
func eventByID(t *testing.T, tl Timeline, id string) TimelineEvent {
	t.Helper()
	for _, ev := range tl.Events {
		if ev.ID == id {
			return ev
		}
	}
	t.Fatalf("no timeline event with id %q", id)
	return TimelineEvent{}
}

func TestTimeline_PassedRunReport(t *testing.T) {
	data := []byte(`{
		"workflow": "ok", "result": "passed",
		"steps": [
			{"id":"a","type":"policy.check","status":"passed","exitCode":0,"message":"ok"},
			{"id":"b","type":"policy.review","status":"passed","exitCode":0}
		]
	}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatalf("TimelineJSON: %v", err)
	}
	if tl.SchemaVersion != TimelineSchemaVersion {
		t.Errorf("schemaVersion = %d", tl.SchemaVersion)
	}
	if tl.Source != SourceWorkflowRunReport {
		t.Errorf("source = %q", tl.Source)
	}
	// start + 2 steps + result = 4 events, 2 synthetic, 2 passed.
	if tl.Summary.Total != 4 || tl.Summary.Synthetic != 2 || tl.Summary.Passed != 2 {
		t.Errorf("summary = %+v", tl.Summary)
	}
	start := tl.Events[0]
	if start.ID != "workflow-start" || !start.Synthetic || start.Status != eventStatusStarted {
		t.Errorf("start event = %+v", start)
	}
	end := tl.Events[len(tl.Events)-1]
	if end.ID != "workflow-result" || !end.Synthetic || end.Status != "passed" {
		t.Errorf("end event = %+v", end)
	}
	// Saved step events are not synthetic.
	if eventByID(t, tl, "a").Synthetic {
		t.Error("saved step event must not be marked synthetic")
	}
}

func TestTimeline_FailedRunReportWithCodes(t *testing.T) {
	data := []byte(`{
		"workflow": "w", "result": "failed",
		"steps": [
			{"id":"check","type":"policy.check","status":"passed","exitCode":0},
			{"id":"compare","type":"policy.baseline.compare","status":"failed","exitCode":1,
				"report":{"findings":[{"code":"policy.lan_exposure_forbidden","severity":"error"}]}}
		]
	}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatalf("TimelineJSON: %v", err)
	}
	if tl.Summary.Failed != 1 || tl.Summary.Passed != 1 {
		t.Errorf("summary = %+v", tl.Summary)
	}
	compare := eventByID(t, tl, "compare")
	if compare.Status != "failed" || compare.ExitCode == nil || *compare.ExitCode != 1 {
		t.Errorf("compare event = %+v", compare)
	}
	if len(compare.Codes) != 1 || compare.Codes[0] != "policy.lan_exposure_forbidden" {
		t.Errorf("compare codes = %v", compare.Codes)
	}
	if tl.Events[len(tl.Events)-1].Status != "failed" {
		t.Errorf("result event status = %q", tl.Events[len(tl.Events)-1].Status)
	}
}

func TestTimeline_RunReportSkippedStep(t *testing.T) {
	data := []byte(`{
		"workflow": "w", "result": "failed",
		"steps": [
			{"id":"compare","type":"policy.baseline.compare","status":"failed","exitCode":1},
			{"id":"dependent","type":"policy.check","status":"skipped","exitCode":0}
		]
	}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatalf("TimelineJSON: %v", err)
	}
	if tl.Summary.Skipped != 1 {
		t.Errorf("summary = %+v", tl.Summary)
	}
	dep := eventByID(t, tl, "dependent")
	if dep.Status != "skipped" || len(dep.Codes) != 1 || dep.Codes[0] != codeDependencyFailed {
		t.Errorf("dependent event = %+v", dep)
	}
}

func TestTimeline_ValidPlanReport(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1, "name": "p", "result": "valid",
		"steps": [{"id":"review","type":"policy.review","status":"valid","code":"workflow.step.valid","message":"ok"}]
	}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatalf("TimelineJSON: %v", err)
	}
	if tl.Source != SourceWorkflowPlanReport {
		t.Errorf("source = %q", tl.Source)
	}
	if tl.Events[0].ID != "plan-start" || tl.Events[len(tl.Events)-1].ID != "plan-result" {
		t.Errorf("plan lifecycle events missing: %+v", tl.Events)
	}
	if tl.Summary.Valid != 1 || tl.Summary.Invalid != 0 {
		t.Errorf("summary = %+v", tl.Summary)
	}
	if tl.Events[len(tl.Events)-1].Status != "valid" {
		t.Errorf("plan-result status = %q", tl.Events[len(tl.Events)-1].Status)
	}
}

func TestTimeline_InvalidPlanReport(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1, "name": "p", "result": "invalid",
		"steps": [{"id":"a","type":"policy.review","status":"invalid","code":"workflow.step.missing_policy"}]
	}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatalf("TimelineJSON: %v", err)
	}
	if tl.Summary.Invalid != 1 {
		t.Errorf("summary = %+v", tl.Summary)
	}
	a := eventByID(t, tl, "a")
	if len(a.Codes) != 1 || a.Codes[0] != "workflow.step.missing_policy" {
		t.Errorf("invalid step codes = %v", a.Codes)
	}
}

func TestTimeline_PopulatedHistoryExport(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1, "source": "workflow-history", "runCount": 2, "runs": [
			{"id":"20260614T120000Z-w1-a","createdAt":"2026-06-14T12:00:00Z","workflow":"w1","result":"failed",
				"summary":{"total":2,"passed":1,"failed":1,"skipped":0},"codes":["workflow.run.input_failed"]},
			{"id":"20260614T110000Z-w2-b","createdAt":"2026-06-14T11:00:00Z","workflow":"w2","result":"passed",
				"summary":{"total":1,"passed":1,"failed":0,"skipped":0}}
		], "safety": {}
	}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatalf("TimelineJSON: %v", err)
	}
	if tl.Source != SourceWorkflowHistoryExport {
		t.Errorf("source = %q", tl.Source)
	}
	if tl.Result != "" {
		t.Errorf("history export should have no top-level result, got %q", tl.Result)
	}
	// start + 2 runs + end = 4 events; runs counted by result.
	if tl.Summary.Total != 4 || tl.Summary.Synthetic != 2 || tl.Summary.Failed != 1 || tl.Summary.Passed != 1 {
		t.Errorf("summary = %+v", tl.Summary)
	}
	// Newest-first: the first non-synthetic event is runs[0].
	if tl.Events[1].ID != "20260614T120000Z-w1-a" {
		t.Errorf("history order not newest-first: %q", tl.Events[1].ID)
	}
	first := eventByID(t, tl, "20260614T120000Z-w1-a")
	if first.Type != typeHistoryRun || first.Workflow != "w1" || first.CreatedAt != "2026-06-14T12:00:00Z" {
		t.Errorf("history run event = %+v", first)
	}
	if first.RunSummary == nil || first.RunSummary.Failed != 1 {
		t.Errorf("history run summary = %+v", first.RunSummary)
	}
	if len(first.Codes) != 1 || first.Codes[0] != "workflow.run.input_failed" {
		t.Errorf("history run codes = %v", first.Codes)
	}
	end := tl.Events[len(tl.Events)-1]
	if end.ID != "history-end" || end.Status != eventStatusReconstructed {
		t.Errorf("history-end event = %+v", end)
	}
}

func TestTimeline_EmptyHistoryExport(t *testing.T) {
	data := []byte(`{"schemaVersion":1,"source":"workflow-history","runCount":0,"runs":[],"safety":{}}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatalf("TimelineJSON: %v", err)
	}
	// Only the two synthetic lifecycle events.
	if tl.Summary.Total != 2 || tl.Summary.Synthetic != 2 {
		t.Errorf("summary = %+v", tl.Summary)
	}
	if tl.Events[0].ID != "history-start" || tl.Events[1].ID != "history-end" {
		t.Errorf("empty history events = %+v", tl.Events)
	}
}

func TestTimeline_BundleDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"),
		`{"schemaVersion":1,"type":"workflow-report","source":"workflow-run","workflow":"w","result":"failed"}`)
	writeFile(t, filepath.Join(dir, "report.json"), `{
		"kind":"workflow-run","workflow":"w","result":"failed",
		"steps":[
			{"id":"check","type":"policy.check","status":"passed"},
			{"id":"compare","type":"policy.baseline.compare","status":"failed","exitCode":1,"codes":["policy.lan_exposure_forbidden"]}
		]
	}`)
	tl, err := TimelineInput(dir)
	if err != nil {
		t.Fatalf("TimelineInput: %v", err)
	}
	if tl.Source != SourceWorkflowReportBundle {
		t.Errorf("source = %q", tl.Source)
	}
	if tl.Events[0].ID != "bundle-start" || tl.Events[len(tl.Events)-1].ID != "bundle-result" {
		t.Errorf("bundle lifecycle events missing: %+v", tl.Events)
	}
	if tl.Summary.Total != 4 || tl.Summary.Failed != 1 || tl.Summary.Passed != 1 {
		t.Errorf("summary = %+v", tl.Summary)
	}
	compare := eventByID(t, tl, "compare")
	if len(compare.Codes) != 1 || compare.Codes[0] != "policy.lan_exposure_forbidden" {
		t.Errorf("bundle compare codes = %v", compare.Codes)
	}
}

func TestTimeline_BundleDirManifestOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"),
		`{"schemaVersion":1,"type":"workflow-report","source":"workflow-plan","workflow":"w","result":"valid"}`)
	tl, err := TimelineInput(dir)
	if err != nil {
		t.Fatalf("TimelineInput: %v", err)
	}
	// Only synthetic start/result, no step events.
	if tl.Summary.Total != 2 || tl.Summary.Synthetic != 2 {
		t.Errorf("summary = %+v", tl.Summary)
	}
}

func TestTimeline_BundleDirNoResult(t *testing.T) {
	// A bundle whose manifest/report carry no result → the closing synthetic event
	// is marked "reconstructed", and the timeline has no top-level result.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"),
		`{"schemaVersion":1,"type":"workflow-report","source":"workflow-run","workflow":"w"}`)
	tl, err := TimelineInput(dir)
	if err != nil {
		t.Fatalf("TimelineInput: %v", err)
	}
	if tl.Result != "" {
		t.Errorf("expected no top-level result, got %q", tl.Result)
	}
	end := tl.Events[len(tl.Events)-1]
	if end.ID != "bundle-result" || end.Status != eventStatusReconstructed {
		t.Errorf("bundle-result event = %+v", end)
	}
}

func TestTimeline_BundleDirNoManifest(t *testing.T) {
	if _, err := TimelineInput(t.TempDir()); err == nil {
		t.Fatal("expected error for bundle dir without manifest.json")
	}
}

func TestTimeline_BundleDirWrongManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{"schemaVersion":1,"type":"not-a-report"}`)
	if _, err := TimelineInput(dir); err == nil {
		t.Fatal("expected error for a non-workflow-report manifest")
	}
}

func TestTimeline_BundleMalformedReportTolerated(t *testing.T) {
	// A malformed report.json is tolerated: the manifest alone yields a minimal
	// start/result timeline (no step events), never an error.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"),
		`{"schemaVersion":1,"type":"workflow-report","source":"workflow-run","workflow":"w","result":"failed"}`)
	writeFile(t, filepath.Join(dir, "report.json"), `{ not valid json`)
	tl, err := TimelineInput(dir)
	if err != nil {
		t.Fatalf("TimelineInput: %v", err)
	}
	if tl.Summary.Total != 2 || tl.Summary.Synthetic != 2 {
		t.Errorf("malformed report should yield only synthetic events, got %+v", tl.Summary)
	}
}

func TestTimeline_OrderIsSequential(t *testing.T) {
	data := []byte(`{"workflow":"w","result":"passed","steps":[
		{"id":"a","type":"t","status":"passed"},{"id":"b","type":"t","status":"passed"}]}`)
	tl, err := TimelineJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	for i, ev := range tl.Events {
		if ev.Order != i+1 {
			t.Errorf("event %d has order %d", i, ev.Order)
		}
	}
}

func TestTimeline_Deterministic(t *testing.T) {
	data := []byte(`{"workflow":"w","result":"failed","steps":[
		{"id":"s","type":"policy.check","status":"failed","exitCode":1,
			"report":{"findings":[{"code":"policy.b","severity":"error"},{"code":"policy.a","severity":"error"}]}}]}`)
	a, err := TimelineJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := TimelineJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	da, _ := MarshalTimeline(a)
	db, _ := MarshalTimeline(b)
	if string(da) != string(db) {
		t.Error("timeline is not deterministic across calls")
	}
}

func TestTimelineJSON_Malformed(t *testing.T) {
	if _, err := TimelineJSON([]byte("{ not json")); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestTimelineJSON_Unsupported(t *testing.T) {
	if _, err := TimelineJSON([]byte(`{"hello":"world"}`)); err == nil {
		t.Fatal("expected error for unsupported shape")
	}
}

func TestTimelineInput_MissingPath(t *testing.T) {
	if _, err := TimelineInput(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestTimelineInput_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.json")
	writeFile(t, path, `{"workflow":"w","result":"passed","steps":[{"id":"s","type":"t","status":"passed"}]}`)
	tl, err := TimelineInput(path)
	if err != nil {
		t.Fatalf("TimelineInput: %v", err)
	}
	if tl.Source != SourceWorkflowRunReport {
		t.Errorf("source = %q", tl.Source)
	}
}
