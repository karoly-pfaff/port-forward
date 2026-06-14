package core

import (
	"path/filepath"
	"testing"
)

// facts parses JSON into compareFacts via the same path the command uses.
func facts(t *testing.T, data string) compareFacts {
	t.Helper()
	f, err := factsFromJSON([]byte(data))
	if err != nil {
		t.Fatalf("factsFromJSON: %v", err)
	}
	return f
}

func changeByID(t *testing.T, c Compare, id string) CompareChange {
	t.Helper()
	for _, ch := range c.Changes {
		if ch.ID == id {
			return ch
		}
	}
	t.Fatalf("no change with id %q (have %v)", id, changeIDs(c))
	return CompareChange{}
}

func hasChange(c Compare, id string) bool {
	for _, ch := range c.Changes {
		if ch.ID == id {
			return true
		}
	}
	return false
}

func changeIDs(c Compare) []string {
	out := make([]string, 0, len(c.Changes))
	for _, ch := range c.Changes {
		out = append(out, ch.ID)
	}
	return out
}

func hasInsight(c Compare, s string) bool {
	for _, in := range c.Insights {
		if in == s {
			return true
		}
	}
	return false
}

const runPassed = `{"workflow":"w","result":"passed","steps":[{"id":"check","type":"policy.check","status":"passed","exitCode":0}]}`
const runFailed = `{"workflow":"w","result":"failed","steps":[
	{"id":"check","type":"policy.check","status":"passed","exitCode":0},
	{"id":"compare","type":"policy.baseline.compare","status":"failed","exitCode":1,"report":{"findings":[{"code":"policy.lan_exposure_forbidden","severity":"error"}]}}]}`

func TestCompare_PassedVsFailedRun(t *testing.T) {
	c := BuildCompare(facts(t, runPassed), facts(t, runFailed))
	if !c.Summary.SameKind || !c.Summary.Changed {
		t.Fatalf("summary = %+v", c.Summary)
	}
	r := changeByID(t, c, "result")
	if r.Left != "passed" || r.Right != "failed" {
		t.Errorf("result change = %+v", r)
	}
	if !hasInsight(c, "Result changed from passed to failed.") {
		t.Errorf("insights = %v", c.Insights)
	}
	if c.Sets["codes"].Added == nil || len(c.Sets["codes"].Added) != 1 || c.Sets["codes"].Added[0] != "policy.lan_exposure_forbidden" {
		t.Errorf("codes set = %+v", c.Sets["codes"])
	}
	if !hasInsight(c, "New emitted codes appeared.") {
		t.Errorf("missing codes insight: %v", c.Insights)
	}
	if !hasChange(c, "failedSteps.added") {
		t.Errorf("expected failedSteps.added change: %v", changeIDs(c))
	}
}

func TestCompare_FailedVsPassedRun(t *testing.T) {
	c := BuildCompare(facts(t, runFailed), facts(t, runPassed))
	if !hasInsight(c, "Result changed from failed to passed.") {
		t.Errorf("insights = %v", c.Insights)
	}
	// codes removed (the failed side had a code, the passed side does not)
	if len(c.Sets["codes"].Removed) != 1 {
		t.Errorf("codes removed = %+v", c.Sets["codes"])
	}
	if !hasChange(c, "codes.removed") || !hasChange(c, "failedSteps.removed") {
		t.Errorf("expected removed changes: %v", changeIDs(c))
	}
}

func TestCompare_EqualRunsNoChanges(t *testing.T) {
	c := BuildCompare(facts(t, runPassed), facts(t, runPassed))
	if c.Summary.Changed || c.Summary.ChangeCount != 0 {
		t.Errorf("summary = %+v", c.Summary)
	}
	if len(c.Changes) != 0 {
		t.Errorf("expected no changes, got %v", changeIDs(c))
	}
	if !hasInsight(c, "No meaningful differences detected.") {
		t.Errorf("insights = %v", c.Insights)
	}
}

func TestCompare_RunCodesUnchanged(t *testing.T) {
	c := BuildCompare(facts(t, runFailed), facts(t, runFailed))
	if len(c.Changes) != 0 {
		t.Errorf("equal failed runs should have no changes: %v", changeIDs(c))
	}
	if len(c.Sets["codes"].Unchanged) != 1 || c.Sets["codes"].Unchanged[0] != "policy.lan_exposure_forbidden" {
		t.Errorf("unchanged codes = %+v", c.Sets["codes"])
	}
}

const runSkipped = `{"workflow":"w","result":"failed","steps":[
	{"id":"compare","type":"policy.baseline.compare","status":"failed","exitCode":1},
	{"id":"dependent","type":"policy.check","status":"skipped","exitCode":0}]}`

func TestCompare_SkippedStepsChange(t *testing.T) {
	c := BuildCompare(facts(t, runPassed), facts(t, runSkipped))
	if !hasChange(c, "skippedSteps.added") {
		t.Errorf("expected skippedSteps.added: %v", changeIDs(c))
	}
	added := c.Sets["skippedSteps"].Added
	if len(added) != 1 || added[0] != "dependent" {
		t.Errorf("skippedSteps added = %v", added)
	}
	// a skipped step contributes the dependency_failed code
	codesAdded := c.Sets["codes"].Added
	found := false
	for _, code := range codesAdded {
		if code == "workflow.run.dependency_failed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dependency_failed in added codes: %v", codesAdded)
	}
}

const planValid = `{"schemaVersion":1,"name":"p","result":"valid","steps":[{"id":"a","type":"policy.review","status":"valid","code":"workflow.step.valid"}]}`
const planInvalid = `{"schemaVersion":1,"name":"p","result":"invalid","steps":[{"id":"a","type":"policy.review","status":"invalid","code":"workflow.step.missing_policy"}]}`

func TestCompare_ValidVsInvalidPlan(t *testing.T) {
	c := BuildCompare(facts(t, planValid), facts(t, planInvalid))
	if !hasInsight(c, "Plan became invalid.") {
		t.Errorf("insights = %v", c.Insights)
	}
	if !hasChange(c, "invalidSteps.added") {
		t.Errorf("expected invalidSteps.added: %v", changeIDs(c))
	}
	if !hasInsight(c, "Invalid step set changed.") {
		t.Errorf("missing invalid-step insight: %v", c.Insights)
	}
	// the missing_policy validation code appeared
	if len(c.Sets["codes"].Added) != 1 || c.Sets["codes"].Added[0] != "workflow.step.missing_policy" {
		t.Errorf("codes added = %+v", c.Sets["codes"])
	}
}

func TestCompare_InvalidVsValidPlan(t *testing.T) {
	c := BuildCompare(facts(t, planInvalid), facts(t, planValid))
	if !hasInsight(c, "Plan became valid.") {
		t.Errorf("insights = %v", c.Insights)
	}
	if !hasChange(c, "invalidSteps.removed") || !hasChange(c, "codes.removed") {
		t.Errorf("expected removed changes: %v", changeIDs(c))
	}
}

const histEmpty = `{"schemaVersion":1,"source":"workflow-history","runCount":0,"runs":[],"safety":{}}`
const histPopulated = `{"schemaVersion":1,"source":"workflow-history","runCount":3,"runs":[
	{"id":"r3","workflow":"w1","result":"failed","codes":["workflow.run.input_failed"]},
	{"id":"r2","workflow":"w2","result":"passed"},
	{"id":"r1","workflow":"w1","result":"failed","codes":["policy.lan_exposure_forbidden"]}],"safety":{}}`

func TestCompare_EmptyVsPopulatedHistory(t *testing.T) {
	c := BuildCompare(facts(t, histEmpty), facts(t, histPopulated))
	if !c.Summary.Changed {
		t.Fatalf("expected changed")
	}
	rc := changeByID(t, c, "runs.total")
	if rc.Left != "0" || rc.Right != "3" {
		t.Errorf("runs.total change = %+v", rc)
	}
	if !hasInsight(c, "Failed run count increased.") {
		t.Errorf("insights = %v", c.Insights)
	}
	if !hasInsight(c, "New codes appeared in history.") {
		t.Errorf("missing codes-in-history insight: %v", c.Insights)
	}
	// workflow distribution deltas appear, sorted by name asc (w1 before w2)
	if !hasChange(c, "workflows.w1") || !hasChange(c, "workflows.w2") {
		t.Errorf("expected workflow deltas: %v", changeIDs(c))
	}
	// new failed runs r1, r3 added
	if len(c.Sets["failedRuns"].Added) != 2 {
		t.Errorf("failedRuns added = %+v", c.Sets["failedRuns"])
	}
}

func TestCompare_HistoryFailedCountDecrease(t *testing.T) {
	c := BuildCompare(facts(t, histPopulated), facts(t, histEmpty))
	if !hasInsight(c, "Failed run count decreased.") {
		t.Errorf("insights = %v", c.Insights)
	}
}

func TestCompare_BothEmptyHistory(t *testing.T) {
	c := BuildCompare(facts(t, histEmpty), facts(t, histEmpty))
	if c.Summary.Changed {
		t.Errorf("two empty histories should not differ: %v", changeIDs(c))
	}
	if !hasInsight(c, "History export contains no comparable runs.") {
		t.Errorf("insights = %v", c.Insights)
	}
}

func TestCompare_HistoryWorkflowDeltaOrdering(t *testing.T) {
	left := `{"schemaVersion":1,"source":"workflow-history","runCount":1,"runs":[{"id":"a","workflow":"alpha","result":"passed"}],"safety":{}}`
	right := `{"schemaVersion":1,"source":"workflow-history","runCount":2,"runs":[{"id":"b","workflow":"beta","result":"passed"},{"id":"a","workflow":"alpha","result":"passed"}],"safety":{}}`
	c := BuildCompare(facts(t, left), facts(t, right))
	// workflow delta changes must appear in name-ascending order: alpha before beta
	var order []string
	for _, ch := range c.Changes {
		if ch.Type == "workflows.delta" {
			order = append(order, ch.ID)
		}
	}
	if len(order) != 1 || order[0] != "workflows.beta" {
		// only beta changed (0 -> 1); alpha stayed 1 -> 1
		t.Errorf("workflow delta changes = %v", order)
	}
}

func TestCompare_MixedKinds(t *testing.T) {
	c := BuildCompare(facts(t, runPassed), facts(t, histPopulated))
	if c.Summary.SameKind {
		t.Fatalf("expected sameKind=false")
	}
	if len(c.Changes) != 1 || c.Changes[0].ID != "kind" {
		t.Errorf("mixed-kind changes = %v", changeIDs(c))
	}
	if c.Sets != nil {
		t.Errorf("mixed-kind should have no sets, got %+v", c.Sets)
	}
	if !hasInsight(c, "Use replay analyze on each artifact for deeper single-artifact analysis.") {
		t.Errorf("insights = %v", c.Insights)
	}
}

func TestCompare_Deterministic(t *testing.T) {
	a := BuildCompare(facts(t, runPassed), facts(t, runFailed))
	b := BuildCompare(facts(t, runPassed), facts(t, runFailed))
	da, _ := MarshalCompare(a)
	db, _ := MarshalCompare(b)
	if string(da) != string(db) {
		t.Error("compare is not deterministic across calls")
	}
}

func TestDiffSets(t *testing.T) {
	s := diffSets([]string{"a", "b"}, []string{"b", "c"})
	if len(s.Added) != 1 || s.Added[0] != "c" {
		t.Errorf("added = %v", s.Added)
	}
	if len(s.Removed) != 1 || s.Removed[0] != "a" {
		t.Errorf("removed = %v", s.Removed)
	}
	if len(s.Unchanged) != 1 || s.Unchanged[0] != "b" {
		t.Errorf("unchanged = %v", s.Unchanged)
	}
	// empty inputs yield non-nil empty slices
	e := diffSets(nil, nil)
	if e.Added == nil || e.Removed == nil || e.Unchanged == nil {
		t.Errorf("empty diff should have non-nil slices: %+v", e)
	}
}

func TestCompareInputs_Files(t *testing.T) {
	dir := t.TempDir()
	lp := filepath.Join(dir, "left.json")
	rp := filepath.Join(dir, "right.json")
	writeFile(t, lp, runPassed)
	writeFile(t, rp, runFailed)
	c, err := CompareInputs(lp, rp)
	if err != nil {
		t.Fatalf("CompareInputs: %v", err)
	}
	if !c.Summary.Changed {
		t.Errorf("expected changes")
	}
}

func TestCompareInputs_BundleDir(t *testing.T) {
	mk := func(result, code string) string {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "manifest.json"),
			`{"schemaVersion":1,"type":"workflow-report","source":"workflow-run","workflow":"w","result":"`+result+`"}`)
		steps := `[{"id":"check","type":"policy.check","status":"passed"}]`
		if code != "" {
			steps = `[{"id":"compare","type":"policy.baseline.compare","status":"failed","codes":["` + code + `"]}]`
		}
		writeFile(t, filepath.Join(dir, "report.json"), `{"kind":"workflow-run","workflow":"w","result":"`+result+`","steps":`+steps+`}`)
		return dir
	}
	c, err := CompareInputs(mk("passed", ""), mk("failed", "policy.lan_exposure_forbidden"))
	if err != nil {
		t.Fatalf("CompareInputs: %v", err)
	}
	if c.Left.Source != SourceWorkflowReportBundle {
		t.Errorf("left source = %q", c.Left.Source)
	}
	if !hasChange(c, "result") || len(c.Sets["codes"].Added) != 1 {
		t.Errorf("bundle compare changes = %v sets=%+v", changeIDs(c), c.Sets)
	}
}

func TestCompareInputs_LeftUnreadable(t *testing.T) {
	dir := t.TempDir()
	rp := filepath.Join(dir, "right.json")
	writeFile(t, rp, runPassed)
	if _, err := CompareInputs(filepath.Join(dir, "nope.json"), rp); err == nil {
		t.Fatal("expected error for unreadable left")
	}
}

func TestCompareInputs_RightUnreadable(t *testing.T) {
	dir := t.TempDir()
	lp := filepath.Join(dir, "left.json")
	writeFile(t, lp, runPassed)
	if _, err := CompareInputs(lp, filepath.Join(dir, "nope.json")); err == nil {
		t.Fatal("expected error for unreadable right")
	}
}

func TestFactsFromJSON_Malformed(t *testing.T) {
	if _, err := factsFromJSON([]byte("{ not json")); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestFactsFromJSON_Unsupported(t *testing.T) {
	if _, err := factsFromJSON([]byte(`{"hello":"world"}`)); err == nil {
		t.Fatal("expected error for unsupported shape")
	}
}

func TestBundleFacts_NoManifest(t *testing.T) {
	if _, err := extractFacts(t.TempDir()); err == nil {
		t.Fatal("expected error for bundle dir without manifest.json")
	}
}

func TestBundleFacts_WrongManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{"schemaVersion":1,"type":"not-a-report"}`)
	if _, err := extractFacts(dir); err == nil {
		t.Fatal("expected error for non-workflow-report manifest")
	}
}
