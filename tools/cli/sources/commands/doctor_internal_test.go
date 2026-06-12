package commands

// Internal (white-box) tests for the doctor result model helpers — the summary
// derivation, exit-code mapping, severity tag, error partitioning, and the
// default/empty branches that are awkward to reach through a command path.

import (
	"reflect"
	"strings"
	"testing"
)

func TestNewDoctorReport_Summary(t *testing.T) {
	report := newDoctorReport([]DoctorCheckResult{
		{Code: "a", Severity: DoctorInfo},
		{Code: "b", Severity: DoctorWarning},
		{Code: "c", Severity: DoctorWarning},
		{Code: "d", Severity: DoctorError},
	})
	want := DoctorSummary{Info: 1, Warning: 2, Error: 1}
	if report.Summary != want {
		t.Errorf("summary = %+v, want %+v", report.Summary, want)
	}
}

func TestNewDoctorReport_NilNormalizedToEmpty(t *testing.T) {
	report := newDoctorReport(nil)
	if report.Checks == nil {
		t.Error("Checks should be a non-nil empty slice, got nil")
	}
	if len(report.Checks) != 0 {
		t.Errorf("len(Checks) = %d, want 0", len(report.Checks))
	}
	if report.Summary != (DoctorSummary{}) {
		t.Errorf("summary = %+v, want zero", report.Summary)
	}
}

func TestDoctorExitCode_Normal(t *testing.T) {
	info := newDoctorReport([]DoctorCheckResult{{Severity: DoctorInfo}})
	warn := newDoctorReport([]DoctorCheckResult{{Severity: DoctorWarning}})
	err := newDoctorReport([]DoctorCheckResult{{Severity: DoctorError}})

	if c := doctorExitCode(info, false); c != 0 {
		t.Errorf("info-only normal exit = %d, want 0", c)
	}
	if c := doctorExitCode(warn, false); c != 0 {
		t.Errorf("warning-only normal exit = %d, want 0", c)
	}
	if c := doctorExitCode(err, false); c != 1 {
		t.Errorf("error normal exit = %d, want 1", c)
	}
}

func TestDoctorExitCode_Strict(t *testing.T) {
	info := newDoctorReport([]DoctorCheckResult{{Severity: DoctorInfo}})
	warn := newDoctorReport([]DoctorCheckResult{{Severity: DoctorWarning}})
	errWarn := newDoctorReport([]DoctorCheckResult{{Severity: DoctorError}, {Severity: DoctorWarning}})

	if c := doctorExitCode(info, true); c != 0 {
		t.Errorf("info-only strict exit = %d, want 0", c)
	}
	if c := doctorExitCode(warn, true); c != 1 {
		t.Errorf("warning-only strict exit = %d, want 1", c)
	}
	if c := doctorExitCode(errWarn, true); c != 1 {
		t.Errorf("error+warning strict exit = %d, want 1", c)
	}
}

func TestDoctorResultLabel(t *testing.T) {
	warn := newDoctorReport([]DoctorCheckResult{{Severity: DoctorWarning}})
	if got := doctorResultLabel(warn, false); got != "passed" {
		t.Errorf("warning-only normal result = %q, want passed", got)
	}
	if got := doctorResultLabel(warn, true); got != "failed" {
		t.Errorf("warning-only strict result = %q, want failed", got)
	}
	errd := newDoctorReport([]DoctorCheckResult{{Severity: DoctorError}})
	if got := doctorResultLabel(errd, false); got != "failed" {
		t.Errorf("error result = %q, want failed", got)
	}
}

func TestDoctorSeverityTag(t *testing.T) {
	cases := map[DoctorSeverity]string{
		DoctorInfo:          "[INFO]",
		DoctorWarning:       "[WARN]",
		DoctorError:         "[ERROR]",
		DoctorSeverity("?"): "[?]",
	}
	for sev, want := range cases {
		if got := doctorSeverityTag(sev); got != want {
			t.Errorf("doctorSeverityTag(%q) = %q, want %q", sev, got, want)
		}
	}
}

func TestPartitionValidationErrors(t *testing.T) {
	dup, field := partitionValidationErrors([]string{
		"rule 1 \"A\": name is required",
		duplicateBindingErrPrefix + " tcp 127.0.0.1:48000 (rules 1 and 2)",
		"rule 2 \"B\": targetHost is required",
	})
	wantDup := []string{duplicateBindingErrPrefix + " tcp 127.0.0.1:48000 (rules 1 and 2)"}
	wantField := []string{"rule 1 \"A\": name is required", "rule 2 \"B\": targetHost is required"}
	if !reflect.DeepEqual(dup, wantDup) {
		t.Errorf("dup = %v, want %v", dup, wantDup)
	}
	if !reflect.DeepEqual(field, wantField) {
		t.Errorf("field = %v, want %v", field, wantField)
	}
}

func TestRuleLabel(t *testing.T) {
	if got := ruleLabel(rawConfigRule{Name: "Web"}); got != `Rule "Web"` {
		t.Errorf("named ruleLabel = %q, want `Rule \"Web\"`", got)
	}
	if got := ruleLabel(rawConfigRule{}); got != "An unnamed rule" {
		t.Errorf("unnamed ruleLabel = %q, want \"An unnamed rule\"", got)
	}
}

func TestPrintDoctorHuman_NoChecks(t *testing.T) {
	var b strings.Builder
	printDoctorHuman("Title", newDoctorReport(nil), false, &b)
	out := b.String()
	if !strings.Contains(out, "No checks were run.") {
		t.Errorf("empty report output missing fallback line:\n%s", out)
	}
	if !strings.Contains(out, "Result: passed") {
		t.Errorf("empty report should report Result: passed:\n%s", out)
	}
}

func TestPrintDoctorHuman_StrictWarningNote(t *testing.T) {
	warn := newDoctorReport([]DoctorCheckResult{{Severity: DoctorWarning, Title: "w"}})

	var normal strings.Builder
	printDoctorHuman("Title", warn, false, &normal)
	if strings.Contains(normal.String(), "Strict mode:") {
		t.Errorf("non-strict output must not show the strict note:\n%s", normal.String())
	}
	if !strings.Contains(normal.String(), "Result: passed") {
		t.Errorf("warning-only normal result should be passed:\n%s", normal.String())
	}

	var strict strings.Builder
	printDoctorHuman("Title", warn, true, &strict)
	if !strings.Contains(strict.String(), "Strict mode: warnings are treated as failures.") {
		t.Errorf("strict warning-only output missing strict note:\n%s", strict.String())
	}
	if !strings.Contains(strict.String(), "Result: failed") {
		t.Errorf("warning-only strict result should be failed:\n%s", strict.String())
	}
}

func TestPluralWord(t *testing.T) {
	if got := pluralWord(1, "warning", "warnings"); got != "warning" {
		t.Errorf("pluralWord(1) = %q, want warning", got)
	}
	if got := pluralWord(0, "warning", "warnings"); got != "warnings" {
		t.Errorf("pluralWord(0) = %q, want warnings", got)
	}
	if got := pluralWord(2, "warning", "warnings"); got != "warnings" {
		t.Errorf("pluralWord(2) = %q, want warnings", got)
	}
}
