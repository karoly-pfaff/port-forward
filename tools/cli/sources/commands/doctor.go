package commands

import (
	"fmt"
	"io"

	"portier/cli/sources/output"
)

// DoctorSeverity classifies the importance of a single doctor check result.
// It is a small, fixed vocabulary shared by every doctor command (config
// doctor today; a live runtime doctor later). Do not add new severities
// casually — they are part of the operator-facing doctor contract.
type DoctorSeverity string

const (
	// DoctorInfo is an informational result (no action required).
	DoctorInfo DoctorSeverity = "info"
	// DoctorWarning flags something an operator should review (non-fatal).
	DoctorWarning DoctorSeverity = "warning"
	// DoctorError flags a definite problem; its presence fails the doctor run.
	DoctorError DoctorSeverity = "error"
)

// DoctorCheckResult is one deterministic finding from a doctor run. The fields
// are a stable, machine-readable contract:
//   - Code:     stable machine-readable identifier (e.g. "config.valid").
//   - Severity: info / warning / error.
//   - Title:    short human heading.
//   - Message:  actionable human explanation.
//   - Details:  optional deterministic extra data (omitted when empty).
//
// Check codes are operator-facing identifiers — do not rename them casually.
type DoctorCheckResult struct {
	Code     string         `json:"code"`
	Severity DoctorSeverity `json:"severity"`
	Title    string         `json:"title"`
	Message  string         `json:"message"`
	Details  map[string]any `json:"details,omitempty"`
}

// DoctorSummary counts the checks in a report by severity.
type DoctorSummary struct {
	Info    int `json:"info"`
	Warning int `json:"warning"`
	Error   int `json:"error"`
}

// DoctorReport is the full deterministic result of a doctor run: the ordered
// list of checks plus a severity summary. It is the JSON contract emitted by
// every doctor command under --json.
type DoctorReport struct {
	Checks  []DoctorCheckResult `json:"checks"`
	Summary DoctorSummary       `json:"summary"`
}

// newDoctorReport builds a report from an ordered list of checks, deriving the
// summary counts. A nil checks slice is normalized to an empty slice so the
// JSON contract always carries a "checks" array.
func newDoctorReport(checks []DoctorCheckResult) DoctorReport {
	if checks == nil {
		checks = []DoctorCheckResult{}
	}
	summary := DoctorSummary{}
	for _, c := range checks {
		switch c.Severity {
		case DoctorInfo:
			summary.Info++
		case DoctorWarning:
			summary.Warning++
		case DoctorError:
			summary.Error++
		}
	}
	return DoctorReport{Checks: checks, Summary: summary}
}

// exitCode maps a report to a CLI exit code following the v1.7 exit-code policy:
// 0 when the doctor completed with no error-severity checks, 1 when one or more
// error-severity checks are present. Warnings alone do not fail the run.
func (r DoctorReport) exitCode() int {
	if r.Summary.Error > 0 {
		return 1
	}
	return 0
}

// doctorSeverityTag returns the fixed-width ASCII tag for a severity, matching
// the CLI's symbol-free human output style.
func doctorSeverityTag(s DoctorSeverity) string {
	switch s {
	case DoctorInfo:
		return "[INFO]"
	case DoctorWarning:
		return "[WARN]"
	case DoctorError:
		return "[ERROR]"
	default:
		return "[?]"
	}
}

// printDoctorHuman renders a report in deterministic human-readable form under
// the given title heading.
func printDoctorHuman(title string, r DoctorReport, w io.Writer) {
	fmt.Fprintln(w, title)
	fmt.Fprintln(w)

	if len(r.Checks) == 0 {
		fmt.Fprintln(w, "No checks were run.")
		fmt.Fprintln(w)
	}
	for _, c := range r.Checks {
		fmt.Fprintf(w, "%-7s %s\n", doctorSeverityTag(c.Severity), c.Title)
		if c.Message != "" {
			fmt.Fprintf(w, "        %s\n", c.Message)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  %d info\n", r.Summary.Info)
	fmt.Fprintf(w, "  %d %s\n", r.Summary.Warning, pluralWord(r.Summary.Warning, "warning", "warnings"))
	fmt.Fprintf(w, "  %d %s\n", r.Summary.Error, pluralWord(r.Summary.Error, "error", "errors"))
}

// emitDoctorReport writes the report as JSON (when jsonOutput is set) or human
// output and returns the report's exit code. A JSON encoding failure is the one
// case that overrides the report exit code (exit 1, matching other commands).
func emitDoctorReport(title string, r DoctorReport, jsonOutput bool, stdout, stderr io.Writer) int {
	if jsonOutput {
		if err := output.PrintJSON(stdout, r); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return r.exitCode()
	}
	printDoctorHuman(title, r, stdout)
	return r.exitCode()
}

// pluralWord returns singular when n == 1, otherwise plural.
func pluralWord(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
