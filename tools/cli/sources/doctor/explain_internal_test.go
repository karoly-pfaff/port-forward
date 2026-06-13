package doctor

// Internal (white-box) tests guarding the explanation registry against the
// actual doctor code constants — every stable code the doctors emit must be
// explainable, and the registry must stay internally consistent. This is the
// guard behind the durable rule "stable doctor/check codes must have
// explanations": adding a code constant without an explanation fails here.

import "testing"

// allDoctorCodes is the explicit list of every stable doctor/check code
// constant from Slices 1 (config doctor) and 2 (live doctor). It is maintained
// independently of the explanations map so a new code with no explanation is
// caught. Keep this in sync with the code constants in config.go and doctor.go.
var allDoctorCodes = []string{
	// config doctor (config.go)
	checkConfigReadFailed,
	checkConfigParseFailed,
	checkConfigEmpty,
	checkConfigValidationFailed,
	checkConfigDuplicateBinding,
	checkConfigLanExposure,
	checkConfigPrivilegedPort,
	checkConfigValid,
	// live doctor (doctor.go)
	checkRuntimeReachable,
	checkRuntimeUnreachable,
	checkRuntimeVersion,
	checkRuntimeStatusRead,
	checkRuntimeStatusFailed,
	checkRulesNone,
	checkRulesPresent,
	checkRulesHealthOK,
	checkRulesHealthWarning,
	checkRulesHealthError,
	checkConfigExportRead,
	checkConfigExportFailed,
}

func TestExplanations_EveryDoctorCodeIsExplainable(t *testing.T) {
	for _, code := range allDoctorCodes {
		if _, ok := explanations[code]; !ok {
			t.Errorf("doctor code %q has no explanation (every stable code must be explainable)", code)
		}
	}
}

func TestExplanations_NoExtraOrDuplicateEntries(t *testing.T) {
	known := make(map[string]bool, len(allDoctorCodes))
	for _, c := range allDoctorCodes {
		if known[c] {
			t.Errorf("duplicate code in allDoctorCodes: %q", c)
		}
		known[c] = true
	}
	for code := range explanations {
		if !known[code] {
			t.Errorf("explanations has an entry for unknown/removed code %q", code)
		}
	}
	if len(explanations) != len(allDoctorCodes) {
		t.Errorf("explanations has %d entries, want %d", len(explanations), len(allDoctorCodes))
	}
}

func TestExplanations_CodeFieldMatchesKey(t *testing.T) {
	for key, exp := range explanations {
		if exp.Code != key {
			t.Errorf("explanation under key %q has Code %q", key, exp.Code)
		}
	}
}

func TestExplanations_RequiredFieldsPresent(t *testing.T) {
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

func TestExplanations_NoDanglingRelated(t *testing.T) {
	for code, exp := range explanations {
		for _, rel := range exp.Related {
			if _, ok := explanations[rel]; !ok {
				t.Errorf("%s: related code %q is not a known code", code, rel)
			}
			if rel == code {
				t.Errorf("%s: lists itself as related", code)
			}
		}
	}
}
