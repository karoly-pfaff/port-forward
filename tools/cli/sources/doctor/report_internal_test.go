package doctor

// White-box tests for the doctor report model: summary derivation, exit codes,
// result labels, severity tags, inline explanations, and human output.

import (
	"strings"
	"testing"
)

func TestNewDoctorReport_Summary(t *testing.T) {
	report := NewReport([]CheckResult{
		{Code: "a", Severity: Info},
		{Code: "b", Severity: Warning},
		{Code: "c", Severity: Warning},
		{Code: "d", Severity: Error},
	})
	want := Summary{Info: 1, Warning: 2, Error: 1}
	if report.Summary != want {
		t.Errorf("summary = %+v, want %+v", report.Summary, want)
	}
}

func TestNewDoctorReport_NilNormalizedToEmpty(t *testing.T) {
	report := NewReport(nil)
	if report.Checks == nil {
		t.Error("Checks should be a non-nil empty slice, got nil")
	}
	if len(report.Checks) != 0 {
		t.Errorf("len(Checks) = %d, want 0", len(report.Checks))
	}
	if report.Summary != (Summary{}) {
		t.Errorf("summary = %+v, want zero", report.Summary)
	}
}

func TestDoctorExitCode_Normal(t *testing.T) {
	info := NewReport([]CheckResult{{Severity: Info}})
	warn := NewReport([]CheckResult{{Severity: Warning}})
	err := NewReport([]CheckResult{{Severity: Error}})

	if c := ExitCode(info, false); c != 0 {
		t.Errorf("info-only normal exit = %d, want 0", c)
	}
	if c := ExitCode(warn, false); c != 0 {
		t.Errorf("warning-only normal exit = %d, want 0", c)
	}
	if c := ExitCode(err, false); c != 1 {
		t.Errorf("error normal exit = %d, want 1", c)
	}
}

func TestDoctorExitCode_Strict(t *testing.T) {
	info := NewReport([]CheckResult{{Severity: Info}})
	warn := NewReport([]CheckResult{{Severity: Warning}})
	errWarn := NewReport([]CheckResult{{Severity: Error}, {Severity: Warning}})

	if c := ExitCode(info, true); c != 0 {
		t.Errorf("info-only strict exit = %d, want 0", c)
	}
	if c := ExitCode(warn, true); c != 1 {
		t.Errorf("warning-only strict exit = %d, want 1", c)
	}
	if c := ExitCode(errWarn, true); c != 1 {
		t.Errorf("error+warning strict exit = %d, want 1", c)
	}
}

func TestDoctorResultLabel(t *testing.T) {
	warn := NewReport([]CheckResult{{Severity: Warning}})
	if got := ResultLabel(warn, false); got != "passed" {
		t.Errorf("warning-only normal result = %q, want passed", got)
	}
	if got := ResultLabel(warn, true); got != "failed" {
		t.Errorf("warning-only strict result = %q, want failed", got)
	}
	errd := NewReport([]CheckResult{{Severity: Error}})
	if got := ResultLabel(errd, false); got != "failed" {
		t.Errorf("error result = %q, want failed", got)
	}
}

func TestDoctorSeverityTag(t *testing.T) {
	cases := map[Severity]string{
		Info:          "[INFO]",
		Warning:       "[WARN]",
		Error:         "[ERROR]",
		Severity("?"): "[?]",
	}
	for sev, want := range cases {
		if got := severityTag(sev); got != want {
			t.Errorf("severityTag(%q) = %q, want %q", sev, got, want)
		}
	}
}

func TestExplanationsForReport_OnlyEmittedCodesDeduped(t *testing.T) {
	r := NewReport([]CheckResult{
		{Code: checkRuntimeReachable, Severity: Info},
		{Code: checkRuntimeReachable, Severity: Info}, // duplicate code
		{Code: checkRulesHealthError, Severity: Error},
	})
	m := explanationsForReport(r)

	if len(m) != 2 {
		t.Errorf("len = %d, want 2 (duplicate code collapsed)", len(m))
	}
	if _, ok := m[checkRuntimeReachable]; !ok {
		t.Errorf("missing %s", checkRuntimeReachable)
	}
	if _, ok := m[checkRulesHealthError]; !ok {
		t.Errorf("missing %s", checkRulesHealthError)
	}
	// A code NOT present in the report must not appear.
	if _, ok := m[checkConfigValid]; ok {
		t.Errorf("%s should not appear (not in report)", checkConfigValid)
	}
	// Each entry is the canonical registry explanation.
	if m[checkRuntimeReachable].Code != checkRuntimeReachable {
		t.Errorf("explanation Code mismatch: %q", m[checkRuntimeReachable].Code)
	}
}

func TestExplanationsForReport_UnknownCodeSafelyOmitted(t *testing.T) {
	r := NewReport([]CheckResult{{Code: "not.a.real.code", Severity: Info}})
	if m := explanationsForReport(r); len(m) != 0 {
		t.Errorf("unknown code should be omitted, got %v", m)
	}
}

func TestExplanationsForReport_AllDoctorCodesExplainable(t *testing.T) {
	// Every code a doctor can emit must resolve to a registry explanation, so
	// inline --explain never hits the "(no explanation available)" fallback.
	checks := make([]CheckResult, len(allDoctorCodes))
	for i, code := range allDoctorCodes {
		checks[i] = CheckResult{Code: code, Severity: Info}
	}
	m := explanationsForReport(NewReport(checks))
	if len(m) != len(allDoctorCodes) {
		t.Errorf("explained %d of %d doctor codes", len(m), len(allDoctorCodes))
	}
}

func TestPrintCheckExplanation(t *testing.T) {
	// Known code: prints Code/Meaning/What to do (+ Related when present).
	var known strings.Builder
	printCheckExplanation(CheckResult{Code: checkConfigLanExposure}, &known)
	ks := known.String()
	for _, want := range []string{"Code: config.lan_exposure", "Meaning:", "What to do:", "Related:"} {
		if !strings.Contains(ks, want) {
			t.Errorf("known-code explanation missing %q:\n%s", want, ks)
		}
	}

	// Unknown code: safe-degrade to a clear fallback (no crash).
	var unknown strings.Builder
	printCheckExplanation(CheckResult{Code: "not.a.real.code"}, &unknown)
	us := unknown.String()
	if !strings.Contains(us, "Code: not.a.real.code") || !strings.Contains(us, "(no explanation available)") {
		t.Errorf("unknown-code fallback wrong:\n%s", us)
	}
}

func TestPrintDoctorHuman_NoChecks(t *testing.T) {
	var b strings.Builder
	PrintHuman("Title", NewReport(nil), false, false, &b)
	out := b.String()
	if !strings.Contains(out, "No checks were run.") {
		t.Errorf("empty report output missing fallback line:\n%s", out)
	}
	if !strings.Contains(out, "Result: passed") {
		t.Errorf("empty report should report Result: passed:\n%s", out)
	}
}

func TestPrintDoctorHuman_StrictWarningNote(t *testing.T) {
	warn := NewReport([]CheckResult{{Severity: Warning, Title: "w"}})

	var normal strings.Builder
	PrintHuman("Title", warn, false, false, &normal)
	if strings.Contains(normal.String(), "Strict mode:") {
		t.Errorf("non-strict output must not show the strict note:\n%s", normal.String())
	}
	if !strings.Contains(normal.String(), "Result: passed") {
		t.Errorf("warning-only normal result should be passed:\n%s", normal.String())
	}

	var strict strings.Builder
	PrintHuman("Title", warn, true, false, &strict)
	if !strings.Contains(strict.String(), "Strict mode: warnings are treated as failures.") {
		t.Errorf("strict warning-only output missing strict note:\n%s", strict.String())
	}
	if !strings.Contains(strict.String(), "Result: failed") {
		t.Errorf("warning-only strict result should be failed:\n%s", strict.String())
	}
}
