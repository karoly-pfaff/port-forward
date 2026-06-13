package policy

// Internal (white-box) guard for the policy explanation registry: every policy
// finding code MUST be explainable, the registry must stay internally
// consistent, and there must be no orphan/duplicate entries. This is the guard
// behind the durable rule "policy finding codes are operator-facing and must
// have explanations": adding a policy code without an explanation fails here.

import "testing"

// allPolicyCodes is the explicit list of every stable policy finding code this
// package can emit. Maintained independently of the explanations map so a new
// code with no explanation is caught. Keep in sync with the code constants and
// the finding constructors in policy.go.
var allPolicyCodes = []string{
	codeValid,
	codeGroupRequired,
	codeLanExposureForbidden,
	codePrivilegedPortForbidden,
	codeAutostartForbidden,
	codeDuplicateBindingForbidden,
}

func TestPolicyExplanations_EveryCodeIsExplainable(t *testing.T) {
	for _, code := range allPolicyCodes {
		if _, ok := explanations[code]; !ok {
			t.Errorf("policy code %q has no explanation (every stable code must be explainable)", code)
		}
	}
}

func TestPolicyExplanations_NoExtraOrDuplicateEntries(t *testing.T) {
	known := make(map[string]bool, len(allPolicyCodes))
	for _, c := range allPolicyCodes {
		if known[c] {
			t.Errorf("duplicate code in allPolicyCodes: %q", c)
		}
		known[c] = true
	}
	for code := range explanations {
		if !known[code] {
			t.Errorf("explanations has an entry for unknown/removed policy code %q", code)
		}
	}
	if len(explanations) != len(allPolicyCodes) {
		t.Errorf("policy explanations has %d entries, want %d", len(explanations), len(allPolicyCodes))
	}
}

func TestPolicyExplanations_CodeFieldMatchesKey(t *testing.T) {
	for key, exp := range explanations {
		if exp.Code != key {
			t.Errorf("policy explanation under key %q has Code %q", key, exp.Code)
		}
	}
}

func TestPolicyExplanations_RequiredFieldsPresent(t *testing.T) {
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

func TestPolicyExplanations_NoDanglingRelated(t *testing.T) {
	for code, exp := range explanations {
		for _, rel := range exp.Related {
			if _, ok := explanations[rel]; !ok {
				t.Errorf("%s: related code %q is not a known policy code", code, rel)
			}
			if rel == code {
				t.Errorf("%s: lists itself as related", code)
			}
		}
	}
}

// TestPolicyExplanations_SeverityMatchesFinding ensures the explanation severity
// agrees with the finding severity each constructor emits (all violations are
// errors; policy.valid is info).
func TestPolicyExplanations_SeverityMatchesFinding(t *testing.T) {
	if explanations[codeValid].Severity != "info" {
		t.Errorf("policy.valid explanation severity = %q, want info", explanations[codeValid].Severity)
	}
	for _, code := range []string{codeGroupRequired, codeLanExposureForbidden, codePrivilegedPortForbidden, codeAutostartForbidden, codeDuplicateBindingForbidden} {
		if explanations[code].Severity != "error" {
			t.Errorf("%s explanation severity = %q, want error", code, explanations[code].Severity)
		}
	}
}
