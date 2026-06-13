package policy

// White-box tests for the policy parser/evaluator/report helpers. These lock the
// internal behavior (default application, report derivation, severity tags) that
// the black-box command tests rely on.

import (
	"testing"

	"portier/cli/sources/config"
)

func TestParsePolicy_AppliesPermissiveDefaults(t *testing.T) {
	// An empty rules object must default to the permissive baseline.
	pol, err := Parse([]byte(`{"schemaVersion": 1, "rules": {}}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if pol.requireGroup {
		t.Errorf("requireGroup default = true, want false")
	}
	if !pol.allowLanExposure {
		t.Errorf("allowLanExposure default = false, want true")
	}
	if !pol.allowPrivilegedPorts {
		t.Errorf("allowPrivilegedPorts default = false, want true")
	}
	if !pol.allowAutostart {
		t.Errorf("allowAutostart default = false, want true")
	}
	if pol.forbidDuplicateBindings {
		t.Errorf("forbidDuplicateBindings default = true, want false")
	}
}

func TestParsePolicy_ExplicitValuesOverrideDefaults(t *testing.T) {
	pol, err := Parse([]byte(`{"schemaVersion": 1, "rules": {
		"requireGroup": true,
		"allowLanExposure": false,
		"allowPrivilegedPorts": false,
		"allowAutostart": false,
		"forbidDuplicateBindings": true
	}}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !pol.requireGroup || pol.allowLanExposure || pol.allowPrivilegedPorts || pol.allowAutostart || !pol.forbidDuplicateBindings {
		t.Errorf("explicit policy not applied: %+v", pol)
	}
}

func TestParsePolicy_OmittedRulesObjectIsPermissive(t *testing.T) {
	pol, err := Parse([]byte(`{"schemaVersion": 1}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if pol.requireGroup || !pol.allowLanExposure {
		t.Errorf("omitted rules object should be permissive: %+v", pol)
	}
}

func TestParsePolicy_Errors(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"empty", `   `},
		{"missing schemaVersion", `{"rules": {}}`},
		{"unsupported schemaVersion", `{"schemaVersion": 99, "rules": {}}`},
		{"unknown rule field", `{"schemaVersion": 1, "rules": {"allowUdp": true}}`},
		{"unknown top-level field", `{"schemaVersion": 1, "rules": {}, "x": 1}`},
		{"not an object", `[1, 2, 3]`},
		{"wrong type", `{"schemaVersion": "one", "rules": {}}`},
		{"trailing data", `{"schemaVersion": 1, "rules": {}} {"extra": 1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.data)); err == nil {
				t.Errorf("Parse(%q) = nil error, want error", tc.data)
			}
		})
	}
}

func TestBoolOr(t *testing.T) {
	tru, fls := true, false
	if boolOr(nil, true) != true {
		t.Errorf("boolOr(nil, true) = false")
	}
	if boolOr(nil, false) != false {
		t.Errorf("boolOr(nil, false) = true")
	}
	if boolOr(&tru, false) != true {
		t.Errorf("boolOr(&true, false) = false")
	}
	if boolOr(&fls, true) != false {
		t.Errorf("boolOr(&false, true) = true")
	}
}

func TestNewPolicyReport_SummaryAndResult(t *testing.T) {
	r := NewReport([]Finding{
		{Code: "a", Severity: Info},
		{Code: "b", Severity: Error},
		{Code: "c", Severity: Warning},
		{Code: "d", Severity: Error},
	})
	if r.Summary.Info != 1 || r.Summary.Warning != 1 || r.Summary.Error != 2 {
		t.Errorf("summary = %+v, want info=1 warning=1 error=2", r.Summary)
	}
	if r.Result != "failed" {
		t.Errorf("result = %q, want failed", r.Result)
	}
}

func TestNewPolicyReport_NilFindingsPasses(t *testing.T) {
	r := NewReport(nil)
	if r.Findings == nil {
		t.Errorf("findings should be a non-nil empty slice")
	}
	if r.Result != "passed" {
		t.Errorf("result = %q, want passed", r.Result)
	}
}

func TestPolicyExitCode(t *testing.T) {
	pass := NewReport([]Finding{{Code: codeValid, Severity: Info}})
	if ExitCode(pass) != 0 {
		t.Errorf("exit code for info-only = %d, want 0", ExitCode(pass))
	}
	fail := NewReport([]Finding{{Code: codeAutostartForbidden, Severity: Error}})
	if ExitCode(fail) != 1 {
		t.Errorf("exit code for error = %d, want 1", ExitCode(fail))
	}
}

func TestPolicySeverityTag(t *testing.T) {
	cases := map[Severity]string{
		Info:    "[INFO]",
		Warning: "[WARN]",
		Error:   "[ERROR]",
		"weird": "[?]",
	}
	for sev, want := range cases {
		if got := severityTag(sev); got != want {
			t.Errorf("severityTag(%q) = %q, want %q", sev, got, want)
		}
	}
}

func TestEvaluatePolicy_ValidWhenPermissive(t *testing.T) {
	rules := []config.Rule{
		{Name: "A", Protocol: "tcp", ListenHost: "0.0.0.0", ListenPort: 22, TargetHost: "x", TargetPort: 1, Enabled: true},
	}
	// Permissive policy: nothing enabled.
	report := Evaluate(rules, Policy{
		allowLanExposure: true, allowPrivilegedPorts: true, allowAutostart: true,
	})
	if len(report.Findings) != 1 || report.Findings[0].Code != codeValid {
		t.Errorf("permissive policy should yield policy.valid, got %+v", report.Findings)
	}
}

func TestEvaluatePolicy_DuplicateBindingLastAndSingle(t *testing.T) {
	rules := []config.Rule{
		{Name: "A", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48080, TargetHost: "x", TargetPort: 1, Group: "g"},
		{Name: "B", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48080, TargetHost: "y", TargetPort: 2, Group: "g"},
	}
	report := Evaluate(rules, Policy{
		allowLanExposure: true, allowPrivilegedPorts: true, allowAutostart: true,
		forbidDuplicateBindings: true,
	})
	if len(report.Findings) != 1 {
		t.Fatalf("expected a single duplicate-binding finding, got %+v", report.Findings)
	}
	if report.Findings[0].Code != codeDuplicateBindingForbidden {
		t.Errorf("finding code = %q, want %q", report.Findings[0].Code, codeDuplicateBindingForbidden)
	}
}
