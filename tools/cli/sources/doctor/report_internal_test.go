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
