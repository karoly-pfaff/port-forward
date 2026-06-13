// Package doctor holds the Portier CLI's diagnostic doctor model and its
// deterministic check logic: the result types (Report/CheckResult/Summary), the
// offline config-doctor and live-runtime check builders, the stable check-code
// explanation registry, and report emission (human + JSON, with --strict /
// --explain / --out). It contains no command/dispatch code — the CLI command
// handlers in package commands compose it with the API client.
package doctor

import (
	"fmt"
	"io"
	"strings"

	"portier/cli/sources/config"
	"portier/cli/sources/output"
)

// Severity classifies the importance of a single doctor check result. It is a
// small, fixed vocabulary shared by every doctor command (config doctor and the
// live runtime doctor). Do not add new severities casually — they are part of
// the operator-facing doctor contract.
type Severity string

const (
	// Info is an informational result (no action required).
	Info Severity = "info"
	// Warning flags something an operator should review (non-fatal).
	Warning Severity = "warning"
	// Error flags a definite problem; its presence fails the doctor run.
	Error Severity = "error"
)

// CheckResult is one deterministic finding from a doctor run. The fields are a
// stable, machine-readable contract:
//   - Code:     stable machine-readable identifier (e.g. "config.valid").
//   - Severity: info / warning / error.
//   - Title:    short human heading.
//   - Message:  actionable human explanation.
//   - Details:  optional deterministic extra data (omitted when empty).
//
// Check codes are operator-facing identifiers — do not rename them casually.
type CheckResult struct {
	Code     string         `json:"code"`
	Severity Severity       `json:"severity"`
	Title    string         `json:"title"`
	Message  string         `json:"message"`
	Details  map[string]any `json:"details,omitempty"`
}

// Summary counts the checks in a report by severity.
type Summary struct {
	Info    int `json:"info"`
	Warning int `json:"warning"`
	Error   int `json:"error"`
}

// Report is the full deterministic result of a doctor run: the ordered list of
// checks plus a severity summary. It is the JSON contract emitted by every
// doctor command under --json.
type Report struct {
	Checks  []CheckResult `json:"checks"`
	Summary Summary       `json:"summary"`
}

// NewReport builds a report from an ordered list of checks, deriving the summary
// counts. A nil checks slice is normalized to an empty slice so the JSON
// contract always carries a "checks" array.
func NewReport(checks []CheckResult) Report {
	if checks == nil {
		checks = []CheckResult{}
	}
	summary := Summary{}
	for _, c := range checks {
		switch c.Severity {
		case Info:
			summary.Info++
		case Warning:
			summary.Warning++
		case Error:
			summary.Error++
		}
	}
	return Report{Checks: checks, Summary: summary}
}

// ExitCode maps a report to a CLI exit code. Normal mode: 1 when one or more
// error-severity checks are present; warnings alone exit 0. Strict mode
// additionally treats warnings as failures (a warning-only report exits 1).
// Info-only is always 0. Strict mode changes only this interpretation — it never
// changes which checks ran.
func ExitCode(r Report, strict bool) int {
	if r.Summary.Error > 0 {
		return 1
	}
	if strict && r.Summary.Warning > 0 {
		return 1
	}
	return 0
}

// ResultLabel returns the stable pass/fail label for a report under the given
// strictness, mirroring its exit code ("passed" → 0, "failed" → 1).
func ResultLabel(r Report, strict bool) string {
	if ExitCode(r, strict) != 0 {
		return "failed"
	}
	return "passed"
}

// reportJSON is the JSON encoding of a doctor report plus the strict-mode
// outcome. Report is embedded so its `checks`/`summary` stay at the top level;
// `strict`/`result`/`explanations` are additive and CLI-local (not an API DTO).
// Explanations is populated only with --explain and only for the codes present in
// checks; otherwise it is omitted (nil → omitempty), so output without --explain
// is byte-identical to before.
type reportJSON struct {
	Report
	Strict       bool                   `json:"strict"`
	Result       string                 `json:"result"`
	Explanations map[string]Explanation `json:"explanations,omitempty"`
	// Config is a compact, deterministic config summary set ONLY by config doctor
	// (when the file parses); the live doctor leaves it nil so its JSON is
	// byte-identical to before (omitempty).
	Config *config.Summary `json:"config,omitempty"`
}

// EmitOptions groups the presentation flags for a doctor report. They affect
// ONLY how the report is rendered/exported and its exit-code interpretation —
// never which checks run. Config is the optional config summary (config doctor
// only; nil for the live doctor). Fields are exported so command handlers in
// other packages can construct EmitOptions directly.
type EmitOptions struct {
	Strict  bool
	Explain bool
	JSON    bool
	OutPath string
	Config  *config.Summary
}

// ReportPayload returns the JSON-marshalable doctor report payload — the same
// `{checks, summary, strict, result}` shape Emit writes under --json — for the
// support bundle's doctor.json artifact. It carries no explanations or config
// summary (those are bundle-specific concerns handled elsewhere).
func ReportPayload(r Report, strict bool) any {
	return reportJSON{Report: r, Strict: strict, Result: ResultLabel(r, strict)}
}

// explanationsForReport returns the canonical explanation for each code present
// in the report's checks (deduplicated via the map; codes with no registry entry
// are safely omitted). Reuses the explanation registry — it does not duplicate
// explanation strings.
func explanationsForReport(r Report) map[string]Explanation {
	m := make(map[string]Explanation)
	for _, c := range r.Checks {
		if exp, ok := explanations[c.Code]; ok {
			m[c.Code] = exp
		}
	}
	return m
}

// severityTag returns the fixed-width ASCII tag for a severity, matching the
// CLI's symbol-free human output style.
func severityTag(s Severity) string {
	switch s {
	case Info:
		return "[INFO]"
	case Warning:
		return "[WARN]"
	case Error:
		return "[ERROR]"
	default:
		return "[?]"
	}
}

// PrintHuman renders a report in deterministic human-readable form under the
// given title heading. In strict mode, when warnings (and no errors) are the
// reason for failure, it notes that warnings are treated as failures. When
// explain is set, each check is followed by its code, meaning, and next action.
func PrintHuman(title string, r Report, strict, explain bool, w io.Writer) {
	fmt.Fprintln(w, title)
	fmt.Fprintln(w)

	if len(r.Checks) == 0 {
		fmt.Fprintln(w, "No checks were run.")
		fmt.Fprintln(w)
	}
	for _, c := range r.Checks {
		fmt.Fprintf(w, "%-7s %s\n", severityTag(c.Severity), c.Title)
		if c.Message != "" {
			fmt.Fprintf(w, "        %s\n", c.Message)
		}
		if explain {
			printCheckExplanation(c, w)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  %d info\n", r.Summary.Info)
	fmt.Fprintf(w, "  %d %s\n", r.Summary.Warning, output.PluralWord(r.Summary.Warning, "warning", "warnings"))
	fmt.Fprintf(w, "  %d %s\n", r.Summary.Error, output.PluralWord(r.Summary.Error, "error", "errors"))

	if strict && r.Summary.Error == 0 && r.Summary.Warning > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Strict mode: warnings are treated as failures.")
	}
	fmt.Fprintf(w, "\nResult: %s\n", ResultLabel(r, strict))
}

// printCheckExplanation renders the inline explanation block for one check
// (--explain), indented to align with the check message. Codes with no registry
// entry degrade safely to a clear "(no explanation available)" note.
func printCheckExplanation(c CheckResult, w io.Writer) {
	fmt.Fprintf(w, "        Code: %s\n", c.Code)
	exp, ok := explanations[c.Code]
	if !ok {
		fmt.Fprintln(w, "        (no explanation available)")
		return
	}
	fmt.Fprintf(w, "        Meaning: %s\n", exp.Meaning)
	fmt.Fprintf(w, "        What to do: %s\n", exp.Action)
	if len(exp.Related) > 0 {
		fmt.Fprintf(w, "        Related: %s\n", strings.Join(exp.Related, ", "))
	}
}

// Emit prints the report (JSON when opts.JSON is set, otherwise human) and, when
// opts.OutPath is non-empty, also writes the exact same JSON report to that file.
// With opts.Explain it adds inline explanations (human blocks; an additive
// `explanations` map in JSON) for the codes present in the report — without
// changing checks/summary/result. It returns the report's exit code under
// opts.Strict — except that a JSON-encode or file-write failure overrides it with
// exit 1 (an operation failure). Never mutates runtime/config.
func Emit(title string, r Report, opts EmitOptions, stdout, stderr io.Writer) int {
	payload := reportJSON{Report: r, Strict: opts.Strict, Result: ResultLabel(r, opts.Strict)}
	if opts.Explain {
		payload.Explanations = explanationsForReport(r)
	}
	payload.Config = opts.Config

	if opts.JSON {
		if err := output.PrintJSON(stdout, payload); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		PrintHuman(title, r, opts.Strict, opts.Explain, stdout)
	}

	if opts.OutPath != "" {
		if err := output.WritePrettyJSON(opts.OutPath, payload); err != nil {
			fmt.Fprintf(stderr, "Error writing %s: %v\n", opts.OutPath, err)
			return 1
		}
		// In human mode, confirm the export on stdout (config export does the
		// same). In JSON mode stdout must stay valid JSON, so stay silent.
		if !opts.JSON {
			fmt.Fprintf(stdout, "\nReport written to %s\n", opts.OutPath)
		}
	}

	return ExitCode(r, opts.Strict)
}
