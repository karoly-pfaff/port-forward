package core

import (
	"path/filepath"
	"testing"
)

func explByCode(t *testing.T, e Explain, code string) Explanation {
	t.Helper()
	for _, ex := range e.Explanations {
		if ex.Code == code {
			return ex
		}
	}
	t.Fatalf("no explanation for code %q", code)
	return Explanation{}
}

func TestExplainRegistry_Integrity(t *testing.T) {
	if len(replayExplanations) != 24 {
		t.Errorf("registry has %d entries, want 24", len(replayExplanations))
	}
	valid := map[string]bool{explainInfo: true, explainWarning: true, explainError: true}
	for code, ex := range replayExplanations {
		if ex.Code != code {
			t.Errorf("entry %q has Code %q (must match key)", code, ex.Code)
		}
		if !ex.Known {
			t.Errorf("entry %q is not marked Known", code)
		}
		if ex.Title == "" || ex.Meaning == "" || ex.Suggestion == "" {
			t.Errorf("entry %q has empty fields: %+v", code, ex)
		}
		if !valid[ex.Severity] {
			t.Errorf("entry %q has invalid severity %q", code, ex.Severity)
		}
	}
}

func TestKnownCodes_Sorted(t *testing.T) {
	codes := knownCodes()
	if len(codes) != 24 {
		t.Fatalf("knownCodes len = %d, want 24", len(codes))
	}
	for i := 1; i < len(codes); i++ {
		if codes[i-1] >= codes[i] {
			t.Errorf("knownCodes not strictly sorted at %d: %q >= %q", i, codes[i-1], codes[i])
		}
	}
}

func TestBuildExplain_KnownAndUnknown(t *testing.T) {
	e := BuildExplain("test", []string{"policy.lan_exposure_forbidden", "custom.future.code"})
	if e.CodeCount != 2 || e.Known != 1 || e.Unknown != 1 {
		t.Errorf("counts = %+v", e)
	}
	known := explByCode(t, e, "policy.lan_exposure_forbidden")
	if !known.Known || known.Severity != explainWarning || known.Title == "" {
		t.Errorf("known explanation = %+v", known)
	}
	unknown := explByCode(t, e, "custom.future.code")
	if unknown.Known || unknown.Severity != explainUnknown || unknown.Title != "Unknown code" {
		t.Errorf("unknown explanation = %+v", unknown)
	}
}

func TestBuildExplain_NoCodes(t *testing.T) {
	e := BuildExplain("workflow-history-export", nil)
	if e.CodeCount != 0 || len(e.Explanations) != 0 {
		t.Errorf("expected empty, got %+v", e)
	}
	if e.Explanations == nil {
		t.Error("Explanations must be non-nil (serialize as [])")
	}
}

func TestExplainJSON_RunReport(t *testing.T) {
	data := []byte(`{"workflow":"w","result":"failed","steps":[
		{"id":"compare","type":"policy.baseline.compare","status":"failed","exitCode":1,"report":{"findings":[{"code":"policy.lan_exposure_forbidden","severity":"error"}]}},
		{"id":"dep","type":"policy.check","status":"skipped"}]}`)
	e, err := ExplainJSON(data)
	if err != nil {
		t.Fatalf("ExplainJSON: %v", err)
	}
	if e.Source != SourceWorkflowRunReport {
		t.Errorf("source = %q", e.Source)
	}
	// codes are sorted ascending: policy.lan... before workflow.run...
	if len(e.Explanations) != 2 || e.Explanations[0].Code != "policy.lan_exposure_forbidden" {
		t.Errorf("explanations order = %+v", e.Explanations)
	}
	if e.Explanations[1].Code != "workflow.run.dependency_failed" {
		t.Errorf("expected dependency_failed second, got %q", e.Explanations[1].Code)
	}
}

func TestExplainJSON_PlanReport(t *testing.T) {
	data := []byte(`{"schemaVersion":1,"name":"p","result":"invalid","steps":[
		{"id":"a","type":"policy.review","status":"invalid","code":"workflow.step.missing_policy"}]}`)
	e, err := ExplainJSON(data)
	if err != nil {
		t.Fatalf("ExplainJSON: %v", err)
	}
	ex := explByCode(t, e, "workflow.step.missing_policy")
	if !ex.Known || ex.Severity != explainError {
		t.Errorf("plan code explanation = %+v", ex)
	}
}

func TestExplainJSON_HistoryExport(t *testing.T) {
	data := []byte(`{"schemaVersion":1,"source":"workflow-history","runCount":2,"runs":[
		{"id":"r2","workflow":"w","result":"failed","codes":["workflow.run.input_failed"]},
		{"id":"r1","workflow":"w","result":"failed","codes":["workflow.run.input_failed","policy.lan_exposure_forbidden"]}],"safety":{}}`)
	e, err := ExplainJSON(data)
	if err != nil {
		t.Fatalf("ExplainJSON: %v", err)
	}
	// codes deduped across runs (input_failed appears once) and sorted
	if e.CodeCount != 2 {
		t.Errorf("codeCount = %d, want 2 (deduped)", e.CodeCount)
	}
	if e.Explanations[0].Code != "policy.lan_exposure_forbidden" || e.Explanations[1].Code != "workflow.run.input_failed" {
		t.Errorf("order = %+v", e.Explanations)
	}
}

func TestExplainJSON_NoCodesHistory(t *testing.T) {
	e, err := ExplainJSON([]byte(`{"schemaVersion":1,"source":"workflow-history","runCount":0,"runs":[],"safety":{}}`))
	if err != nil {
		t.Fatalf("ExplainJSON: %v", err)
	}
	if e.CodeCount != 0 {
		t.Errorf("expected 0 codes, got %d", e.CodeCount)
	}
}

func TestExplainJSON_Malformed(t *testing.T) {
	if _, err := ExplainJSON([]byte("{ not json")); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestExplainJSON_Unsupported(t *testing.T) {
	if _, err := ExplainJSON([]byte(`{"hello":"world"}`)); err == nil {
		t.Fatal("expected error for unsupported shape")
	}
}

func TestReplayOutputCodes_Analysis(t *testing.T) {
	// A replay analysis JSON: source + summary + codes[{code,count}].
	data := []byte(`{"schemaVersion":1,"source":"workflow-run-report","result":"failed",
		"summary":{"steps":{"total":1}},"codes":[{"code":"policy.lan_exposure_forbidden","count":1}],
		"findings":[],"insights":[]}`)
	src, codes, ok := replayOutputCodes(data)
	if !ok || src != SourceReplayAnalysis {
		t.Fatalf("analysis detect: ok=%v src=%q", ok, src)
	}
	if len(codes) != 1 || codes[0] != "policy.lan_exposure_forbidden" {
		t.Errorf("analysis codes = %v", codes)
	}
}

func TestReplayOutputCodes_Timeline(t *testing.T) {
	data := []byte(`{"schemaVersion":1,"source":"workflow-run-report","result":"failed","events":[
		{"order":1,"id":"workflow-start","synthetic":true},
		{"order":2,"id":"compare","synthetic":false,"codes":["policy.lan_exposure_forbidden"]},
		{"order":3,"id":"workflow-result","synthetic":true}],"summary":{"total":3}}`)
	src, codes, ok := replayOutputCodes(data)
	if !ok || src != SourceReplayTimeline {
		t.Fatalf("timeline detect: ok=%v src=%q", ok, src)
	}
	if len(codes) != 1 || codes[0] != "policy.lan_exposure_forbidden" {
		t.Errorf("timeline codes = %v", codes)
	}
}

func TestReplayOutputCodes_Compare(t *testing.T) {
	data := []byte(`{"schemaVersion":1,"left":{"source":"workflow-run-report"},"right":{"source":"workflow-run-report"},
		"summary":{"sameKind":true,"changed":true,"changeCount":1},"changes":[],
		"sets":{"codes":{"added":["policy.lan_exposure_forbidden"],"removed":[],"unchanged":["workflow.run.input_failed"]}},"insights":[]}`)
	src, codes, ok := replayOutputCodes(data)
	if !ok || src != SourceReplayCompare {
		t.Fatalf("compare detect: ok=%v src=%q", ok, src)
	}
	// added + unchanged, sorted
	if len(codes) != 2 || codes[0] != "policy.lan_exposure_forbidden" || codes[1] != "workflow.run.input_failed" {
		t.Errorf("compare codes = %v", codes)
	}
}

func TestReplayOutputCodes_NotReplayOutput(t *testing.T) {
	if _, _, ok := replayOutputCodes([]byte(`{"hello":"world"}`)); ok {
		t.Error("plain JSON should not detect as a replay output")
	}
	if _, _, ok := replayOutputCodes([]byte("{ bad")); ok {
		t.Error("malformed JSON should not detect as a replay output")
	}
}

func TestExplainInput_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.json")
	writeFile(t, path, `{"workflow":"w","result":"failed","steps":[{"id":"c","type":"policy.check","status":"failed","exitCode":1,"report":{"findings":[{"code":"policy.autostart_forbidden","severity":"error"}]}}]}`)
	e, err := ExplainInput(path)
	if err != nil {
		t.Fatalf("ExplainInput: %v", err)
	}
	if e.CodeCount != 1 || e.Explanations[0].Code != "policy.autostart_forbidden" {
		t.Errorf("explanations = %+v", e.Explanations)
	}
}

func TestExplainInput_BundleDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"),
		`{"schemaVersion":1,"type":"workflow-report","source":"workflow-run","workflow":"w","result":"failed"}`)
	writeFile(t, filepath.Join(dir, "report.json"),
		`{"kind":"workflow-run","steps":[{"id":"c","type":"policy.baseline.compare","status":"failed","codes":["policy.lan_exposure_forbidden"]}]}`)
	e, err := ExplainInput(dir)
	if err != nil {
		t.Fatalf("ExplainInput: %v", err)
	}
	if e.Source != SourceWorkflowReportBundle {
		t.Errorf("source = %q", e.Source)
	}
	if e.CodeCount != 1 || e.Explanations[0].Code != "policy.lan_exposure_forbidden" {
		t.Errorf("explanations = %+v", e.Explanations)
	}
}

func TestExplainInput_BundleNoManifest(t *testing.T) {
	if _, err := ExplainInput(t.TempDir()); err == nil {
		t.Fatal("expected error for bundle dir without manifest.json")
	}
}

func TestExplainInput_MissingPath(t *testing.T) {
	if _, err := ExplainInput(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestExplainCode_KnownUnknown(t *testing.T) {
	known := ExplainCode("policy.valid")
	if known.Source != sourceCode || known.Known != 1 || known.Unknown != 0 {
		t.Errorf("known code report = %+v", known)
	}
	unknown := ExplainCode("nope.code")
	if unknown.Known != 0 || unknown.Unknown != 1 {
		t.Errorf("unknown code report = %+v", unknown)
	}
}

func TestExplainRegistry_AllKnown(t *testing.T) {
	e := ExplainRegistry()
	if e.Source != sourceRegistry || e.CodeCount != 24 || e.Known != 24 || e.Unknown != 0 {
		t.Errorf("registry report counts = codeCount:%d known:%d unknown:%d source:%q", e.CodeCount, e.Known, e.Unknown, e.Source)
	}
}

func TestSortedDedupeStrings(t *testing.T) {
	got := sortedDedupeStrings([]string{"b", "", "a", "b", ""})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("sortedDedupeStrings = %v, want [a b] (deduped, empties dropped, sorted)", got)
	}
	if sortedDedupeStrings(nil) == nil {
		t.Error("expected non-nil empty slice")
	}
}

func TestExplain_Deterministic(t *testing.T) {
	data := []byte(`{"workflow":"w","result":"failed","steps":[{"id":"c","type":"policy.baseline.compare","status":"failed","exitCode":1,"report":{"new":[{"code":"policy.b"},{"code":"policy.a"}]}}]}`)
	a, err := ExplainJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ExplainJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	da, _ := MarshalExplain(a)
	db, _ := MarshalExplain(b)
	if string(da) != string(db) {
		t.Error("explain is not deterministic across calls")
	}
}
