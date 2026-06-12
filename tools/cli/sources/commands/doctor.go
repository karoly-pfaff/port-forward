package commands

import (
	"flag"
	"fmt"
	"io"
	"strings"

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

// doctorExitCode maps a report to a CLI exit code. Normal mode: 1 when one or
// more error-severity checks are present; warnings alone exit 0. Strict mode
// additionally treats warnings as failures (a warning-only report exits 1).
// Info-only is always 0. Strict mode changes only this interpretation — it
// never changes which checks ran.
func doctorExitCode(r DoctorReport, strict bool) int {
	if r.Summary.Error > 0 {
		return 1
	}
	if strict && r.Summary.Warning > 0 {
		return 1
	}
	return 0
}

// doctorResultLabel returns the stable pass/fail label for a report under the
// given strictness, mirroring its exit code ("passed" → 0, "failed" → 1).
func doctorResultLabel(r DoctorReport, strict bool) string {
	if doctorExitCode(r, strict) != 0 {
		return "failed"
	}
	return "passed"
}

// doctorReportJSON is the JSON encoding of a doctor report plus the strict-mode
// outcome. DoctorReport is embedded so its `checks`/`summary` stay at the top
// level; `strict`/`result`/`explanations` are additive and CLI-local (not an
// API DTO). Explanations is populated only with `--explain` and only for the
// codes present in checks; otherwise it is omitted (nil → omitempty), so output
// without `--explain` is byte-identical to before.
type doctorReportJSON struct {
	DoctorReport
	Strict       bool                   `json:"strict"`
	Result       string                 `json:"result"`
	Explanations map[string]Explanation `json:"explanations,omitempty"`
	// Config is a compact, deterministic config summary set ONLY by config
	// doctor (when the file parses); the live doctor leaves it nil so its JSON
	// is byte-identical to before (omitempty). See config.go configSummary.
	Config *configSummary `json:"config,omitempty"`
}

// doctorEmitOptions groups the presentation flags for a doctor report. They
// affect ONLY how the report is rendered/exported and its exit-code
// interpretation — never which checks run. config is the optional config
// summary (config doctor only; nil for the live doctor).
type doctorEmitOptions struct {
	strict  bool
	explain bool
	json    bool
	outPath string
	config  *configSummary
}

// explanationsForReport returns the canonical explanation for each code present
// in the report's checks (deduplicated via the map; codes with no registry
// entry are safely omitted). Reuses the Slice 3 explanation registry — it does
// not duplicate explanation strings.
func explanationsForReport(r DoctorReport) map[string]Explanation {
	m := make(map[string]Explanation)
	for _, c := range r.Checks {
		if exp, ok := explanations[c.Code]; ok {
			m[c.Code] = exp
		}
	}
	return m
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
// the given title heading. In strict mode, when warnings (and no errors) are
// the reason for failure, it notes that warnings are treated as failures. When
// explain is set, each check is followed by its code, meaning, and next action.
func printDoctorHuman(title string, r DoctorReport, strict, explain bool, w io.Writer) {
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
		if explain {
			printCheckExplanation(c, w)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  %d info\n", r.Summary.Info)
	fmt.Fprintf(w, "  %d %s\n", r.Summary.Warning, pluralWord(r.Summary.Warning, "warning", "warnings"))
	fmt.Fprintf(w, "  %d %s\n", r.Summary.Error, pluralWord(r.Summary.Error, "error", "errors"))

	if strict && r.Summary.Error == 0 && r.Summary.Warning > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Strict mode: warnings are treated as failures.")
	}
	fmt.Fprintf(w, "\nResult: %s\n", doctorResultLabel(r, strict))
}

// printCheckExplanation renders the inline explanation block for one check
// (`--explain`), indented to align with the check message. Codes with no
// registry entry degrade safely to a clear "(no explanation available)" note.
func printCheckExplanation(c DoctorCheckResult, w io.Writer) {
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

// emitDoctorReport prints the report (JSON when opts.json is set, otherwise
// human) and, when opts.outPath is non-empty, also writes the exact same JSON
// report to that file. With opts.explain it adds inline explanations (human
// blocks; an additive `explanations` map in JSON) for the codes present in the
// report — without changing checks/summary/result. It returns the report's exit
// code under opts.strict — except that a JSON-encode or file-write failure
// overrides it with exit 1 (an operation failure). Never mutates runtime/config.
func emitDoctorReport(title string, r DoctorReport, opts doctorEmitOptions, stdout, stderr io.Writer) int {
	payload := doctorReportJSON{DoctorReport: r, Strict: opts.strict, Result: doctorResultLabel(r, opts.strict)}
	if opts.explain {
		payload.Explanations = explanationsForReport(r)
	}
	payload.Config = opts.config

	if opts.json {
		if err := output.PrintJSON(stdout, payload); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		printDoctorHuman(title, r, opts.strict, opts.explain, stdout)
	}

	if opts.outPath != "" {
		if err := writePrettyJSON(opts.outPath, payload); err != nil {
			fmt.Fprintf(stderr, "Error writing %s: %v\n", opts.outPath, err)
			return 1
		}
		// In human mode, confirm the export on stdout (config export does the
		// same). In JSON mode stdout must stay valid JSON, so stay silent.
		if !opts.json {
			fmt.Fprintf(stdout, "\nReport written to %s\n", opts.outPath)
		}
	}

	return doctorExitCode(r, opts.strict)
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

Options:
  --strict       Treat warnings as failures (a warning-only report exits 1).
  --explain      Show an explanation (meaning + next action) for each emitted check.
  --out <file>   Also write the JSON report to <file> (same shape as --json).

Output:
  Human report by default; --json emits the full doctor report
  (checks + summary + strict + result, plus an additive explanations map with
  --explain). --out writes that same JSON to a file regardless of --json; with
  --json the JSON also prints to stdout.

Exit codes:
  0  Doctor completed; no error-severity checks (warnings alone exit 0 unless --strict)
  1  Doctor completed; one or more error-severity checks (incl. unreachable runtime),
     or any warning when --strict is set
  2  Usage error (unexpected argument)

Note: an unreachable runtime is reported as a doctor check and exits 1 (the
doctor ran and found a problem), NOT the usual connection exit code 3.

Examples:
  portier doctor
  portier doctor --strict
  portier doctor --explain
  portier doctor --out doctor-report.json
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
	flagStrict := fs.Bool("strict", false, "treat warnings as failures (exit 1)")
	flagExplain := fs.Bool("explain", false, "show an explanation for each emitted check")
	flagOut := fs.String("out", "", "also write the JSON report to this file")

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
	return emitDoctorReport("Portier Doctor", report, doctorEmitOptions{
		strict: *flagStrict, explain: *flagExplain, json: jsonOutput, outPath: *flagOut,
	}, stdout, stderr)
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
