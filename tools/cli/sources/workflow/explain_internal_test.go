package workflow

// Internal (white-box) guard for the workflow explanation registry: every
// workflow step-validation code MUST be explainable, the registry must stay
// internally consistent, and there must be no orphan/duplicate entries. This is
// the guard behind the durable rule "workflow validation codes are operator-
// facing and must have explanations": adding a workflow code without an
// explanation fails here.

import "testing"

// allWorkflowCodes is the explicit, locked list of every stable workflow step-
// validation code this package can emit. Maintained independently of the
// explanations map so a new code with no explanation is caught. Keep in sync with
// the code constants and the validators in workflow.go.
var allWorkflowCodes = []string{
	codeStepValid,
	codeStepDuplicateID,
	codeStepUnknownType,
	codeStepMissingID,
	codeStepMissingType,
	codeStepMissingConfigSource,
	codeStepConflictingConfigSources,
	codeStepMissingPolicy,
	codeStepMissingCurrent,
	codeStepMissingCandidate,
	codeStepMissingBaseline,
	codeStepMissingReportSource,
	codeStepConflictingReportSources,
	codeStepUnknownReportFrom,
	codeStepFutureReportFrom,
}

// TestWorkflowCodes_LockedSet locks the known code set: there are exactly 15
// stable workflow validation codes, each is a "workflow.step.*" identifier, and
// the list has no duplicates. A change here is a deliberate contract change.
func TestWorkflowCodes_LockedSet(t *testing.T) {
	if len(allWorkflowCodes) != 15 {
		t.Fatalf("allWorkflowCodes has %d codes, want 15", len(allWorkflowCodes))
	}
	seen := map[string]bool{}
	for _, c := range allWorkflowCodes {
		if seen[c] {
			t.Errorf("duplicate code in allWorkflowCodes: %q", c)
		}
		seen[c] = true
		if len(c) < len("workflow.step.") || c[:len("workflow.step.")] != "workflow.step." {
			t.Errorf("code %q is not a workflow.step.* identifier", c)
		}
	}
}

func TestWorkflowExplanations_EveryCodeIsExplainable(t *testing.T) {
	for _, code := range allWorkflowCodes {
		if _, ok := explanations[code]; !ok {
			t.Errorf("workflow code %q has no explanation (every stable code must be explainable)", code)
		}
	}
}

func TestWorkflowExplanations_NoExtraOrDuplicateEntries(t *testing.T) {
	known := make(map[string]bool, len(allWorkflowCodes))
	for _, c := range allWorkflowCodes {
		known[c] = true
	}
	for code := range explanations {
		if !known[code] {
			t.Errorf("explanations has an entry for unknown/removed workflow code %q", code)
		}
	}
	if len(explanations) != len(allWorkflowCodes) {
		t.Errorf("workflow explanations has %d entries, want %d", len(explanations), len(allWorkflowCodes))
	}
}

func TestWorkflowExplanations_CodeFieldMatchesKey(t *testing.T) {
	for key, exp := range explanations {
		if exp.Code != key {
			t.Errorf("workflow explanation under key %q has Code %q", key, exp.Code)
		}
	}
}

func TestWorkflowExplanations_RequiredFieldsPresent(t *testing.T) {
	validSeverity := map[string]bool{"info": true, "warning": true, "error": true}
	for code, exp := range explanations {
		if exp.Title == "" {
			t.Errorf("%s: empty Title", code)
		}
		if exp.Meaning == "" {
			t.Errorf("%s: empty Meaning", code)
		}
		if exp.Action == "" {
			t.Errorf("%s: empty Action", code)
		}
		if !validSeverity[exp.Severity] {
			t.Errorf("%s: invalid Severity %q", code, exp.Severity)
		}
	}
}

func TestWorkflowExplanations_NoDanglingRelated(t *testing.T) {
	for code, exp := range explanations {
		for _, rel := range exp.Related {
			if _, ok := explanations[rel]; !ok {
				t.Errorf("%s: related code %q is not a known workflow code", code, rel)
			}
			if rel == code {
				t.Errorf("%s: lists itself as related", code)
			}
		}
	}
}

// TestWorkflowExplanations_SeverityMatchesOutcome ensures workflow.step.valid is
// info and every validation-failure code is an error (matching the step status
// the validators produce).
func TestWorkflowExplanations_SeverityMatchesOutcome(t *testing.T) {
	if explanations[codeStepValid].Severity != "info" {
		t.Errorf("workflow.step.valid explanation severity = %q, want info", explanations[codeStepValid].Severity)
	}
	for _, code := range allWorkflowCodes {
		if code == codeStepValid {
			continue
		}
		if explanations[code].Severity != "error" {
			t.Errorf("%s explanation severity = %q, want error", code, explanations[code].Severity)
		}
	}
}
