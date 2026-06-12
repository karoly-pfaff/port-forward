package commands

import (
	"flag"
	"fmt"
	"io"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
	"portier/cli/sources/version"
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

// --- live runtime doctor (`portier doctor`) ---

// Live doctor check codes. These are stable, operator-facing identifiers —
// do not rename them casually (they are a CLI/tool contract). They share the
// Slice 1 doctor model and naming style with the offline config doctor.
const (
	checkRuntimeReachable    = "runtime.reachable"
	checkRuntimeUnreachable  = "runtime.unreachable"
	checkRuntimeVersion      = "runtime.version"
	checkRuntimeStatusRead   = "runtime.status_read"
	checkRuntimeStatusFailed = "runtime.status_failed"
	checkRulesNone           = "rules.none"
	checkRulesPresent        = "rules.present"
	checkRulesHealthOK       = "rules.health_ok"
	checkRulesHealthWarning  = "rules.health_warning"
	checkRulesHealthError    = "rules.health_error"
	checkConfigExportRead    = "config.export_read"
	checkConfigExportFailed  = "config.export_failed"
)

const doctorRuntimeHelp = `Usage: portier doctor

Run deterministic diagnostic checks against the live Portier runtime.
Read-only: never mutates the runtime or its config, and never probes targets.

Checks (each produces a stable check code):
  runtime.reachable / runtime.unreachable   Can the runtime be reached?
  runtime.version                           Runtime version (warns on CLI/runtime mismatch).
  runtime.status_read / runtime.status_failed   Could rule status be read?
  rules.none / rules.present                Are any forwarding rules configured?
  rules.health_ok / rules.health_warning / rules.health_error
                                            Aggregated rule health (from the API's health field).
  config.export_read / config.export_failed   Can the current config be read (read-only)?

Output:
  Human report by default; --json emits the full doctor report (checks + summary).

Exit codes:
  0  Doctor completed; no error-severity checks (warnings alone still exit 0)
  1  Doctor completed; one or more error-severity checks (incl. unreachable runtime)
  2  Usage error (unexpected argument)

Note: an unreachable runtime is reported as a doctor check and exits 1 (the
doctor ran and found a problem), NOT the usual connection exit code 3.

Examples:
  portier doctor
  portier --json doctor
  portier --host 127.0.0.1 --port 47831 doctor
`

// RunDoctor runs deterministic diagnostic checks against the live Portier
// runtime and prints a doctor report. It is read-only: it never mutates the
// runtime or config and never probes forwarding targets. Exit codes follow the
// doctor policy: 0 = no error-severity checks, 1 = one or more error-severity
// checks (including an unreachable runtime), 2 = usage error.
func RunDoctor(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, doctorRuntimeHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, doctorRuntimeHelp)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "Error: doctor takes no arguments (got %q)\n\n", fs.Arg(0))
		fmt.Fprint(stderr, doctorRuntimeHelp)
		return 2
	}

	report := runLiveDoctorChecks(c)
	return emitDoctorReport("Portier Doctor", report, jsonOutput, stdout, stderr)
}

// runLiveDoctorChecks performs the live runtime analysis and returns a
// deterministic doctor report. An unreachable runtime short-circuits (later
// checks need runtime data); otherwise version, status/health, and a read-only
// config-export check run in a fixed order. The runtime API is the source of
// truth for rule health — the CLI never re-derives it.
func runLiveDoctorChecks(c *client.Client) DoctorReport {
	checks := []DoctorCheckResult{}
	baseURL := c.BaseURL()

	info, err := c.GetRuntime()
	if err != nil {
		checks = append(checks, runtimeUnreachableCheck(baseURL, err))
		return newDoctorReport(checks)
	}
	checks = append(checks, runtimeReachableCheck(baseURL))
	checks = append(checks, runtimeVersionCheck(info))

	if statuses, statusErr := c.GetStatus(); statusErr != nil {
		checks = append(checks, statusFailedCheck(statusErr))
	} else {
		checks = append(checks, statusReadCheck(len(statuses)))
		checks = append(checks, rulesHealthChecks(statuses)...)
	}

	if cfg, cfgErr := c.ExportConfig(); cfgErr != nil {
		checks = append(checks, configExportFailedCheck(cfgErr))
	} else {
		checks = append(checks, configExportReadCheck(len(cfg.Rules)))
	}

	return newDoctorReport(checks)
}

func runtimeUnreachableCheck(baseURL string, err error) DoctorCheckResult {
	return DoctorCheckResult{
		Code:     checkRuntimeUnreachable,
		Severity: DoctorError,
		Title:    "Runtime is unreachable",
		Message:  fmt.Sprintf("Could not reach the Portier runtime: %v. Is the Portier service running?", err),
		Details:  map[string]any{"url": baseURL},
	}
}

func runtimeReachableCheck(baseURL string) DoctorCheckResult {
	return DoctorCheckResult{
		Code:     checkRuntimeReachable,
		Severity: DoctorInfo,
		Title:    "Runtime is reachable",
		Message:  "The Portier runtime responded successfully.",
		Details:  map[string]any{"url": baseURL},
	}
}

// runtimeVersionCheck reports the runtime version, warning (never failing) when
// it differs from the CLI version.
func runtimeVersionCheck(info *client.RuntimeInfo) DoctorCheckResult {
	cliVersion := version.Version
	runtimeVersion := info.Version
	details := map[string]any{
		"runtimeVersion": runtimeVersion,
		"cliVersion":     cliVersion,
		"runtime":        info.Runtime,
	}
	if runtimeVersion != "" && cliVersion != "" && runtimeVersion != cliVersion {
		return DoctorCheckResult{
			Code:     checkRuntimeVersion,
			Severity: DoctorWarning,
			Title:    fmt.Sprintf("Runtime version %s differs from CLI version %s", runtimeVersion, cliVersion),
			Message:  "The runtime and CLI are different versions. This is usually fine, but matching versions are recommended.",
			Details:  details,
		}
	}
	return DoctorCheckResult{
		Code:     checkRuntimeVersion,
		Severity: DoctorInfo,
		Title:    fmt.Sprintf("Runtime version: %s", runtimeVersion),
		Message:  "The Portier runtime reported its version.",
		Details:  details,
	}
}

func statusFailedCheck(err error) DoctorCheckResult {
	return DoctorCheckResult{
		Code:     checkRuntimeStatusFailed,
		Severity: DoctorError,
		Title:    "Could not read rule status",
		Message:  fmt.Sprintf("Reading rule status from the runtime failed: %v", err),
	}
}

func statusReadCheck(ruleCount int) DoctorCheckResult {
	return DoctorCheckResult{
		Code:     checkRuntimeStatusRead,
		Severity: DoctorInfo,
		Title:    "Rule status read",
		Message:  "Rule status was read from the runtime.",
		Details:  map[string]any{"ruleCount": ruleCount},
	}
}

// rulesHealthChecks summarizes rule presence and health from the status list.
// Health values come straight from the API's `health` field — the CLI never
// re-derives health. Order is deterministic: presence, then error, then
// warning, then the all-clear, so a mixed fleet reports both health_error and
// health_warning (matching the runtime's own classification).
func rulesHealthChecks(statuses []client.ForwardStatus) []DoctorCheckResult {
	total := len(statuses)
	if total == 0 {
		return []DoctorCheckResult{{
			Code:     checkRulesNone,
			Severity: DoctorWarning,
			Title:    "No forwarding rules configured",
			Message:  "The runtime has no forwarding rules. Add a rule to start forwarding traffic.",
		}}
	}

	var healthy, warning, errored int
	for _, s := range statuses {
		switch s.Health {
		case "error":
			errored++
		case "warning":
			warning++
		case "healthy":
			healthy++
		}
	}

	checks := []DoctorCheckResult{{
		Code:     checkRulesPresent,
		Severity: DoctorInfo,
		Title:    fmt.Sprintf("%d forwarding %s configured", total, pluralRule(total)),
		Message:  "The runtime reported status for the configured forwarding rules.",
		Details: map[string]any{
			"total":   total,
			"healthy": healthy,
			"warning": warning,
			"error":   errored,
		},
	}}

	if errored > 0 {
		checks = append(checks, DoctorCheckResult{
			Code:     checkRulesHealthError,
			Severity: DoctorError,
			Title:    fmt.Sprintf("%d %s in error health", errored, pluralRule(errored)),
			Message:  "One or more rules report error health (a failed start or socket error). Run 'portier status' or 'portier diagnose <id|name>' for detail.",
			Details:  map[string]any{"count": errored},
		})
	}
	if warning > 0 {
		checks = append(checks, DoctorCheckResult{
			Code:     checkRulesHealthWarning,
			Severity: DoctorWarning,
			Title:    fmt.Sprintf("%d %s in warning health", warning, pluralRule(warning)),
			Message:  "One or more rules report warning health (enabled for autostart but not running).",
			Details:  map[string]any{"count": warning},
		})
	}
	if errored == 0 && warning == 0 {
		checks = append(checks, DoctorCheckResult{
			Code:     checkRulesHealthOK,
			Severity: DoctorInfo,
			Title:    "All rules report healthy",
			Message:  "No rule reports warning or error health.",
			Details:  map[string]any{"total": total},
		})
	}

	return checks
}

func configExportReadCheck(ruleCount int) DoctorCheckResult {
	return DoctorCheckResult{
		Code:     checkConfigExportRead,
		Severity: DoctorInfo,
		Title:    "Config is readable",
		Message:  "The current configuration was read successfully (read-only; nothing was written).",
		Details:  map[string]any{"ruleCount": ruleCount},
	}
}

func configExportFailedCheck(err error) DoctorCheckResult {
	return DoctorCheckResult{
		Code:     checkConfigExportFailed,
		Severity: DoctorError,
		Title:    "Config could not be read",
		Message:  fmt.Sprintf("Reading the current configuration from the runtime failed: %v", err),
	}
}
