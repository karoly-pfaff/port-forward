package workflow

// Internal (white-box) guard for the workflow-RUN explanation registry: every
// workflow.run.* code MUST be explainable, the registry must stay internally
// consistent, and there must be no orphan/duplicate entries. Mirrors the guard
// for the workflow.step.* registry (explain_internal_test.go) — adding a run code
// without an explanation fails here.

import "testing"

// allWorkflowRunCodes is the explicit, locked list of every stable workflow-run
// code this package can emit (see run.go executors). Maintained independently of
// runExplanations so a new code with no explanation is caught.
var allWorkflowRunCodes = []string{
	runCodeDependencyFailed,
	runCodeRuntimeUnreachable,
	runCodeInputFailed,
}

// TestWorkflowRunCodes_LockedSet locks the known run-code set: there are exactly 3
// stable workflow-run codes, each a "workflow.run.*" identifier, with no
// duplicates. A change here is a deliberate contract change.
func TestWorkflowRunCodes_LockedSet(t *testing.T) {
	if len(allWorkflowRunCodes) != 3 {
		t.Fatalf("allWorkflowRunCodes has %d codes, want 3", len(allWorkflowRunCodes))
	}
	const prefix = "workflow.run."
	seen := map[string]bool{}
	for _, c := range allWorkflowRunCodes {
		if seen[c] {
			t.Errorf("duplicate code in allWorkflowRunCodes: %q", c)
		}
		seen[c] = true
		if len(c) < len(prefix) || c[:len(prefix)] != prefix {
			t.Errorf("code %q is not a workflow.run.* identifier", c)
		}
	}
}

func TestWorkflowRunExplanations_EveryCodeIsExplainable(t *testing.T) {
	for _, code := range allWorkflowRunCodes {
		if _, ok := runExplanations[code]; !ok {
			t.Errorf("workflow-run code %q has no explanation (every stable code must be explainable)", code)
		}
	}
}

func TestWorkflowRunExplanations_NoExtraOrDuplicateEntries(t *testing.T) {
	known := make(map[string]bool, len(allWorkflowRunCodes))
	for _, c := range allWorkflowRunCodes {
		known[c] = true
	}
	for code := range runExplanations {
		if !known[code] {
			t.Errorf("runExplanations has an entry for unknown/removed workflow-run code %q", code)
		}
	}
	if len(runExplanations) != len(allWorkflowRunCodes) {
		t.Errorf("workflow-run explanations has %d entries, want %d", len(runExplanations), len(allWorkflowRunCodes))
	}
}

func TestWorkflowRunExplanations_CodeFieldMatchesKey(t *testing.T) {
	for key, exp := range runExplanations {
		if exp.Code != key {
			t.Errorf("workflow-run explanation under key %q has Code %q", key, exp.Code)
		}
	}
}

func TestWorkflowRunExplanations_RequiredFieldsPresent(t *testing.T) {
	for code, exp := range runExplanations {
		if exp.Title == "" {
			t.Errorf("%s: empty Title", code)
		}
		if exp.Meaning == "" {
			t.Errorf("%s: empty Meaning", code)
		}
		if exp.Action == "" {
			t.Errorf("%s: empty Action", code)
		}
		// Every run code is a failure/skip outcome, so every explanation is an error.
		if exp.Severity != "error" {
			t.Errorf("%s: severity = %q, want error", code, exp.Severity)
		}
	}
}

func TestWorkflowRunExplanations_NoDanglingRelated(t *testing.T) {
	for code, exp := range runExplanations {
		for _, rel := range exp.Related {
			if _, ok := runExplanations[rel]; !ok {
				t.Errorf("%s: related code %q is not a known workflow-run code", code, rel)
			}
			if rel == code {
				t.Errorf("%s: lists itself as related", code)
			}
		}
	}
}

// TestExplanations_MergesStepAndRun verifies the package's public Explanations()
// returns BOTH the step-validation and run code families (so the explain command
// lists/looks up every workflow code).
func TestExplanations_MergesStepAndRun(t *testing.T) {
	reg := Explanations()
	want := len(allWorkflowCodes) + len(allWorkflowRunCodes)
	if len(reg) != want {
		t.Fatalf("Explanations() has %d entries, want %d (step %d + run %d)", len(reg), want, len(allWorkflowCodes), len(allWorkflowRunCodes))
	}
	for _, c := range allWorkflowCodes {
		if _, ok := reg[c]; !ok {
			t.Errorf("Explanations() missing step code %q", c)
		}
	}
	for _, c := range allWorkflowRunCodes {
		if _, ok := reg[c]; !ok {
			t.Errorf("Explanations() missing run code %q", c)
		}
	}
}

// TestRunExplainRegistry_CoversRunAndPolicy verifies the run-render registry can
// resolve both a workflow-run code and a policy finding code (the only code
// families a run report can carry).
func TestRunExplainRegistry_CoversRunAndPolicy(t *testing.T) {
	reg := runExplainRegistry()
	if _, ok := reg[runCodeInputFailed]; !ok {
		t.Errorf("runExplainRegistry missing %q", runCodeInputFailed)
	}
	if _, ok := reg["policy.lan_exposure_forbidden"]; !ok {
		t.Errorf("runExplainRegistry missing policy.lan_exposure_forbidden")
	}
}
