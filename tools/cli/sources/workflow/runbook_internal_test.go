package workflow

// White-box tests for the workflow runbook builder and renderer (v1.11 Slice 4):
// the per-step command mapping, the placeholder convention, the display quoting,
// and the deterministic human renderer. Black-box command behavior is covered in
// package commands_test.

import (
	"strings"
	"testing"
)

func planFrom(t *testing.T, src string) File {
	t.Helper()
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if BuildPlan(f).Result != statusValid {
		t.Fatalf("fixture workflow is not valid: %s", src)
	}
	return f
}

func stepByID(rb Runbook, id string) (RunbookStep, bool) {
	for _, s := range rb.Steps {
		if s.ID == id {
			return s, true
		}
	}
	return RunbookStep{}, false
}

func TestBuildRunbook_PolicyCheckConfig(t *testing.T) {
	rb := BuildRunbook(planFrom(t, `{"schemaVersion":1,"name":"w","steps":[{"id":"c","type":"policy.check","config":"portier.json","policy":"p.json"}]}`))
	if rb.Result != runbookResultReady || rb.Summary.Total != 1 {
		t.Fatalf("unexpected runbook: %+v", rb)
	}
	s := rb.Steps[0]
	want := []string{"portier", "policy", "check", "--config", "portier.json", "--policy", "p.json"}
	if strings.Join(s.Command, " ") != strings.Join(want, " ") {
		t.Errorf("command = %v, want %v", s.Command, want)
	}
	if s.Display != "portier policy check --config portier.json --policy p.json" {
		t.Errorf("display = %q", s.Display)
	}
	if len(s.Notes) != 0 {
		t.Errorf("notes = %v, want empty", s.Notes)
	}
}

func TestBuildRunbook_PolicyCheckRuntime(t *testing.T) {
	rb := BuildRunbook(planFrom(t, `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","runtime":true,"policy":"p.json"}]}`))
	s := rb.Steps[0]
	want := "portier policy check --runtime --policy p.json"
	if s.Display != want {
		t.Errorf("display = %q, want %q", s.Display, want)
	}
	for _, tok := range s.Command {
		if tok == "--config" {
			t.Errorf("runtime command must not contain --config: %v", s.Command)
		}
	}
}

func TestBuildRunbook_PolicyReview(t *testing.T) {
	rb := BuildRunbook(planFrom(t, `{"schemaVersion":1,"steps":[{"id":"r","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"p.json"}]}`))
	want := "portier policy review --current cur.json --candidate cand.json --policy p.json"
	if rb.Steps[0].Display != want {
		t.Errorf("display = %q, want %q", rb.Steps[0].Display, want)
	}
}

func TestBuildRunbook_BaselineCompareReportFile(t *testing.T) {
	rb := BuildRunbook(planFrom(t, `{"schemaVersion":1,"steps":[{"id":"b","type":"policy.baseline.compare","baseline":"b.json","report":"r.json"}]}`))
	s := rb.Steps[0]
	want := "portier policy baseline compare --baseline b.json --report r.json"
	if s.Display != want {
		t.Errorf("display = %q, want %q", s.Display, want)
	}
	if len(s.Notes) != 0 {
		t.Errorf("report-file step should have no notes: %v", s.Notes)
	}
}

func TestBuildRunbook_BaselineCompareReportFromPlaceholder(t *testing.T) {
	rb := BuildRunbook(planFrom(t, `{"schemaVersion":1,"steps":[
		{"id":"chk","type":"policy.check","runtime":true,"policy":"p.json"},
		{"id":"cmp","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"chk"}
	]}`))
	cmp, ok := stepByID(rb, "cmp")
	if !ok {
		t.Fatal("missing cmp step")
	}
	// The placeholder, not a real path.
	if cmp.Command[len(cmp.Command)-1] != "<report-from:chk>" {
		t.Errorf("expected placeholder report token, got %v", cmp.Command)
	}
	if !strings.Contains(cmp.Display, "<report-from:chk>") {
		t.Errorf("display missing placeholder: %q", cmp.Display)
	}
	if len(cmp.Notes) != 1 || !strings.Contains(cmp.Notes[0], "chk") {
		t.Errorf("expected a note referencing the producing step: %v", cmp.Notes)
	}
}

func TestBuildRunbook_OrderingAndSummary(t *testing.T) {
	rb := BuildRunbook(planFrom(t, `{"schemaVersion":1,"name":"multi","steps":[
		{"id":"one","type":"policy.check","config":"c.json","policy":"p.json"},
		{"id":"two","type":"policy.review","current":"cur.json","candidate":"cand.json","policy":"p.json"},
		{"id":"three","type":"policy.baseline.compare","baseline":"b.json","reportFrom":"one"}
	]}`))
	if rb.Summary.Total != 3 || len(rb.Steps) != 3 {
		t.Fatalf("summary/steps wrong: %+v", rb)
	}
	gotOrder := []string{rb.Steps[0].ID, rb.Steps[1].ID, rb.Steps[2].ID}
	wantOrder := []string{"one", "two", "three"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("step order = %v, want %v", gotOrder, wantOrder)
		}
	}
	if rb.Workflow != "multi" || rb.Result != runbookResultReady {
		t.Errorf("workflow/result wrong: %+v", rb)
	}
}

func TestRunbookCommand_UnknownTypeFallback(t *testing.T) {
	// runbookCommand is only called for validated steps; the defensive fallback
	// for an unsupported type returns nil/nil (documented unreachable in practice).
	cmd, notes := runbookCommand(Step{ID: "x", Type: "policy.apply"})
	if cmd != nil || notes != nil {
		t.Errorf("unknown-type fallback should return nil, nil; got %v, %v", cmd, notes)
	}
}

func TestShellQuoteToken(t *testing.T) {
	cases := map[string]string{
		"plain.json":      "plain.json",
		"<report-from:a>": "<report-from:a>", // angle brackets are not quoted (no whitespace)
		"has space.json":  "'has space.json'",
		"":                "''",
		"tab\tinside":     "'tab\tinside'",
		"quote'inside":    `'quote'\''inside'`,
	}
	for in, want := range cases {
		if got := shellQuoteToken(in); got != want {
			t.Errorf("shellQuoteToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrintRunbookHuman_Shape(t *testing.T) {
	rb := BuildRunbook(planFrom(t, `{"schemaVersion":1,"name":"policy-baseline-check","steps":[
		{"id":"check-config","type":"policy.check","config":"portier.json","policy":"local-safe.policy.json"},
		{"id":"compare-baseline","type":"policy.baseline.compare","baseline":"policy-baseline.json","reportFrom":"check-config"}
	]}`))
	var sb strings.Builder
	PrintRunbookHuman(rb, &sb)
	out := sb.String()
	for _, want := range []string{
		"Portier Workflow Runbook",
		"Workflow: policy-baseline-check",
		"1. check-config",
		"portier policy check --config portier.json --policy local-safe.policy.json",
		"2. compare-baseline",
		"--report <report-from:check-config>",
		"Note:",
		"2 steps",
		"Result: ready",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human runbook missing %q:\n%s", want, out)
		}
	}
}

func TestPrintRunbookHuman_UnnamedAndSingular(t *testing.T) {
	rb := BuildRunbook(planFrom(t, `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`))
	var sb strings.Builder
	PrintRunbookHuman(rb, &sb)
	out := sb.String()
	if !strings.Contains(out, "Workflow: (unnamed)") {
		t.Errorf("missing unnamed placeholder:\n%s", out)
	}
	if !strings.Contains(out, "1 step\n") {
		t.Errorf("singular 'step' expected:\n%s", out)
	}
}

// TestBuildRunbook_AllTemplatesProduceRunbooks ensures every built-in template
// renders to a ready runbook (templates are valid workflows).
func TestBuildRunbook_AllTemplatesProduceRunbooks(t *testing.T) {
	for _, tm := range Templates() {
		t.Run(tm.Name, func(t *testing.T) {
			f := tm.file
			rb := BuildRunbook(File{SchemaVersion: f.SchemaVersion, Name: f.Name, Steps: templateFileToSteps(f)})
			if rb.Result != runbookResultReady {
				t.Errorf("template %q runbook result = %s", tm.Name, rb.Result)
			}
			if rb.Summary.Total != len(f.Steps) {
				t.Errorf("template %q step count mismatch", tm.Name)
			}
			for _, s := range rb.Steps {
				if len(s.Command) == 0 || s.Command[0] != "portier" {
					t.Errorf("template %q step %q has no portier command: %v", tm.Name, s.ID, s.Command)
				}
			}
		})
	}
}

// templateFileToSteps converts a template's internal file steps into workflow
// Steps (the public shape BuildRunbook consumes).
func templateFileToSteps(f templateWorkflowFile) []Step {
	steps := make([]Step, len(f.Steps))
	for i, s := range f.Steps {
		steps[i] = Step{
			ID: s.ID, Type: s.Type, Config: s.Config, Runtime: s.Runtime, Policy: s.Policy,
			Current: s.Current, Candidate: s.Candidate, Baseline: s.Baseline, Report: s.Report, ReportFrom: s.ReportFrom,
		}
	}
	return steps
}
