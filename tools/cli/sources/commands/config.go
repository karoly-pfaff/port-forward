package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

// groupMaxLength mirrors the server-side rule group label limit (characters).
// The CLI does a light local pre-check; the server is authoritative.
const groupMaxLength = 64

// duplicateBindingErrPrefix is the leading text of the duplicate listen-binding
// validation error. It is shared between validateLocalConfig (the producer) and
// the config doctor (which partitions errors by it) so the two cannot drift.
const duplicateBindingErrPrefix = "duplicate listen binding:"

const configHelp = `Usage: portier config <subcommand> [options]

Manage Portier forwarding rule configuration.

Subcommands:
  validate <file>                     Validate a local config file without importing it
  doctor <file>                       Run offline diagnostic checks on a local config file (read-only)
  export --out <file>                 Export current rules to a file (omit --out to print to stdout)
  import <file> --mode merge|replace  Import rules from a file (--yes required for replace mode)
  plan <file>                         Compare a desired config file against the running config (read-only)
  diff <file>                         Human-friendly diff view of changes (read-only)
  apply <file> --yes                  Apply a desired config file to the running service

Run 'portier config <subcommand> --help' for subcommand options.
`

const doctorHelp = `Usage: portier config doctor [--strict] [--out <file>] <file>

Run deterministic, offline diagnostic checks on a local Portier config file.
Does NOT contact or require a running Portier service, and never modifies the file.

Checks (each produces a stable check code):
  config.read_failed        The file could not be read.
  config.parse_failed       The file is not valid Portier config JSON.
  config.empty              The config contains no rules.
  config.validation_failed  One or more rules have invalid fields.
  config.duplicate_binding  Two rules share a listen binding (protocol+host+port).
  config.lan_exposure       A rule listens on 0.0.0.0 (exposed on the LAN).
  config.privileged_port    A rule listens on a privileged port (< 1024).
  config.valid              The config is readable, parseable, and valid.

Options:
  --strict       Treat warnings as failures (a warning-only report exits 1).
  --out <file>   Also write the JSON report to <file> (same shape as --json).
                 Place flags before the config file (e.g. config doctor --strict file).

Accepted file shapes:
  Raw JSON array:  [{ "name": "...", ... }, ...]
  Wrapper object:  { "rules": [{ "name": "...", ... }, ...] }
  Exported config: { "version": "1", "exportedAt": "...", "rules": [...] }

Exit codes:
  0  Doctor completed; no error-severity checks (warnings alone exit 0 unless --strict)
  1  Doctor completed; one or more error-severity checks, any warning when --strict,
     or the --out file could not be written
  2  Missing argument or usage error

Examples:
  portier config doctor desired.json
  portier config doctor --strict desired.json
  portier config doctor --out config-doctor-report.json desired.json
  portier --json config doctor desired.json
`

const applyHelp = `Usage: portier config apply <file> --yes [options]

Apply a desired config file to the running Portier service.
Calls POST /api/config/apply. Plans the changes first; no mutation occurs on errors.

Options:
  --yes               Required when the plan contains destructive operations (update, remove).
  --dry-run           Preview what would change without mutating the running config.
  --backup-out <file> Export current config to <file> before applying. Backup failure prevents apply.

Accepted file shapes:
  Raw JSON array:  [{ "name": "...", ... }, ...]
  Wrapper object:  { "rules": [{ "name": "...", ... }, ...] }
  Exported config: { "version": "1", "exportedAt": "...", "rules": [...] }

Exit codes:
  0  Apply succeeded (or dry-run completed with no errors)
  1  Plan errors (hasErrors: true), API error, or backup failure
  2  Invalid file, missing argument, local validation failure, or missing --yes for destructive ops
  3  Connection failure — Portier service unreachable

Examples:
  portier config apply desired.json --yes
  portier config apply desired.json --dry-run
  portier config apply desired.json --yes --backup-out backup.json
  portier --json config apply desired.json --yes
`

const planHelp = `Usage: portier config plan <file> [options]

Compare a desired config file against the running Portier configuration.
Calls POST /api/config/plan. Read-only: does not mutate any running config.

Options:
  --fail-on-drift   Exit with code 4 when any add, update, or remove is present.

Accepted file shapes:
  Raw JSON array:  [{ "name": "...", ... }, ...]
  Wrapper object:  { "rules": [{ "name": "...", ... }, ...] }
  Exported config: { "version": "1", "exportedAt": "...", "rules": [...] }

Exit codes:
  0  No drift or --fail-on-drift not set
  1  Plan errors (hasErrors: true) or API error
  2  Invalid file, missing argument, or local validation failure
  3  Connection failure — Portier service unreachable
  4  Drift detected with --fail-on-drift (only when no plan errors)

Examples:
  portier config plan desired.json
  portier config plan desired.json --fail-on-drift
  portier --json config plan desired.json
`

const diffHelp = `Usage: portier config diff <file> [options]

Show a human-friendly diff between a desired config file and the running Portier configuration.
Calls POST /api/config/plan. Read-only: does not mutate any running config.

Options:
  --fail-on-drift    Exit with code 4 when any add, update, or remove is present.
  --show-unchanged   Include unchanged rules in diff output (hidden by default).

Exit codes: same as portier config plan.

Examples:
  portier config diff desired.json
  portier config diff desired.json --show-unchanged
  portier config diff desired.json --fail-on-drift
  portier --json config diff desired.json
`

const exportHelp = `Usage: portier config export [--out <file>]

Export the current forwarding rules from the running Portier service to a JSON file.

Options:
  --out string   Output file path. If omitted, prints JSON to stdout.

Examples:
  portier config export --out rules-backup.json
  portier --json config export --out rules-backup.json
  portier --json config export            # print raw config JSON to stdout
`

const importHelp = `Usage: portier config import <file> --mode merge|replace [--yes]

Import forwarding rules from a local JSON config file.

The file is validated locally before any API call is made. Invalid files are rejected
without contacting the running service.

Options:
  --mode string   Required. Import mode: merge or replace.
  --yes           Required when --mode replace. Confirms destructive overwrite.

Modes:
  merge     Add rules from the file. Existing rules are preserved.
            ID conflicts generate new IDs. Listen-binding conflicts abort the import.
  replace   Remove all existing rules and replace with the imported set.
            Requires --yes to confirm.

Exit codes:
  0  Import succeeded
  1  API error — the server rejected the import or returned an error
  2  Invalid file, missing argument, invalid local config, or missing --yes for replace
  3  Connection failure — Portier service unreachable

Examples:
  portier config import rules-backup.json --mode merge
  portier config import rules-backup.json --mode replace --yes
`

const validateHelp = `Usage: portier config validate <file>

Validate a local Portier config file without importing it or contacting the service.

Accepted file shapes:
  Raw JSON array:  [{ "name": "...", ... }, ...]
  Wrapper object:  { "rules": [{ "name": "...", ... }, ...] }
  Exported config: { "version": "1", "exportedAt": "...", "rules": [...] }

Exit codes:
  0  Config is valid
  1  Config is invalid, unreadable, or the file does not exist

Examples:
  portier config validate rules-backup.json
  portier --json config validate rules-backup.json
`

// RunConfig dispatches portier config subcommands.
func RunConfig(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, configHelp)
		return 2
	}
	switch args[0] {
	case "export":
		return RunConfigExport(c, jsonOutput, args[1:], stdout, stderr)
	case "import":
		return RunConfigImport(c, jsonOutput, args[1:], stdout, stderr)
	case "validate":
		return RunConfigValidate(jsonOutput, args[1:], stdout, stderr)
	case "doctor":
		return RunConfigDoctor(jsonOutput, args[1:], stdout, stderr)
	case "plan":
		return RunConfigPlan(c, jsonOutput, args[1:], stdout, stderr)
	case "diff":
		return RunConfigDiff(c, jsonOutput, args[1:], stdout, stderr)
	case "apply":
		return RunConfigApply(c, jsonOutput, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, configHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown config subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, configHelp)
		return 2
	}
}

// RunConfigExport exports the current service config to a file or stdout.
func RunConfigExport(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagOut := fs.String("out", "", "output file path")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, exportHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, exportHelp)
		return 2
	}

	cfg, err := c.ExportConfig()
	if err != nil {
		return exitWithError(err, stderr)
	}

	if *flagOut == "" {
		// No file specified — print JSON to stdout regardless of --json flag.
		// (The exported config IS the JSON; there is no alternative human representation.)
		if jsonOutput {
			if err := output.PrintJSON(stdout, cfg); err != nil {
				fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
				return 1
			}
			return 0
		}
		fmt.Fprintln(stderr, "Error: --out is required in human mode")
		fmt.Fprintln(stderr, "Use --out <file> to write to a file, or add --json to print to stdout.")
		return 2
	}

	if err := writePrettyJSON(*flagOut, cfg); err != nil {
		fmt.Fprintf(stderr, "Error writing %s: %v\n", *flagOut, err)
		return 1
	}

	ruleCount := len(cfg.Rules)
	if jsonOutput {
		if err := output.PrintJSON(stdout, map[string]any{
			"ok":        true,
			"path":      *flagOut,
			"ruleCount": ruleCount,
		}); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Exported %d %s to %s\n",
		ruleCount, pluralRule(ruleCount), *flagOut)
	return 0
}

// RunConfigImport imports rules from a local config file.
func RunConfigImport(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagMode := fs.String("mode", "", "import mode: merge or replace")
	flagYes := fs.Bool("yes", false, "confirm replace mode")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, importHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, importHelp)
		return 2
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "Error: import requires a config file path")
		fmt.Fprint(stderr, importHelp)
		return 2
	}

	if *flagMode == "" {
		fmt.Fprintln(stderr, "Error: --mode is required (merge or replace)")
		fmt.Fprint(stderr, importHelp)
		return 2
	}
	if *flagMode != "merge" && *flagMode != "replace" {
		fmt.Fprintf(stderr, "Error: invalid --mode %q: must be \"merge\" or \"replace\"\n\n", *flagMode)
		fmt.Fprint(stderr, importHelp)
		return 2
	}
	if *flagMode == "replace" && !*flagYes {
		fmt.Fprintln(stderr, "Error: replace mode overwrites all existing rules.")
		fmt.Fprintln(stderr, "Re-run with --yes to confirm.")
		return 2
	}

	rules, code, ok := loadConfigForCommand(fs.Arg(0), "import", 2, stderr)
	if !ok {
		return code
	}

	resp, err := c.ImportConfig(buildImportRequest(rules, *flagMode))
	if err != nil {
		return exitWithError(err, stderr)
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, map[string]any{
			"ok":   true,
			"mode": *flagMode,
		}); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Imported config using %s mode.\n", *flagMode)
	if resp.Result.Imported > 0 {
		fmt.Fprintf(stdout, "  %d %s imported.\n",
			resp.Result.Imported, pluralRule(resp.Result.Imported))
	}
	return 0
}

// RunConfigValidate validates a local config file without importing it.
func RunConfigValidate(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, validateHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, validateHelp)
		return 2
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "Error: validate requires a config file path")
		fmt.Fprint(stderr, validateHelp)
		return 2
	}

	filePath := fs.Arg(0)
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading %s: %v\n", filePath, err)
		return 1
	}

	rules, parseErr := parseLocalConfig(data)
	if parseErr != nil {
		if jsonOutput {
			if err := output.PrintJSON(stdout, configValidationResult{
				Valid:  false,
				Errors: []string{parseErr.Error()},
			}); err != nil {
				fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			}
			return 1
		}
		fmt.Fprintln(stdout, "Config is invalid.")
		fmt.Fprintf(stdout, "  %s\n", parseErr)
		return 1
	}

	vr := validateLocalConfig(rules)

	if jsonOutput {
		if err := output.PrintJSON(stdout, vr); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		if !vr.Valid {
			return 1
		}
		return 0
	}

	if vr.Valid {
		fmt.Fprintln(stdout, "Config is valid.")
		fmt.Fprintf(stdout, "  %d %s: %d TCP, %d UDP\n",
			vr.RuleCount, pluralRule(vr.RuleCount), vr.TCPCount, vr.UDPCount)
		return 0
	}

	fmt.Fprintln(stdout, "Config is invalid.")
	for _, e := range vr.Errors {
		fmt.Fprintf(stdout, "  %s\n", e)
	}
	return 1
}

// Config doctor check codes. These are stable, operator-facing identifiers —
// do not rename them casually (they are a CLI/tool contract).
const (
	checkConfigReadFailed       = "config.read_failed"
	checkConfigParseFailed      = "config.parse_failed"
	checkConfigEmpty            = "config.empty"
	checkConfigValidationFailed = "config.validation_failed"
	checkConfigDuplicateBinding = "config.duplicate_binding"
	checkConfigLanExposure      = "config.lan_exposure"
	checkConfigPrivilegedPort   = "config.privileged_port"
	checkConfigValid            = "config.valid"
)

// RunConfigDoctor runs deterministic, offline diagnostic checks on a local
// config file and prints a doctor report. It never contacts the running
// service and never modifies the file. Exit codes follow the doctor policy:
// 0 = no error-severity checks, 1 = one or more error-severity checks,
// 2 = missing argument / usage error.
func RunConfigDoctor(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagStrict := fs.Bool("strict", false, "treat warnings as failures (exit 1)")
	flagOut := fs.String("out", "", "also write the JSON report to this file")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, doctorHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, doctorHelp)
		return 2
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "Error: doctor requires a config file path")
		fmt.Fprint(stderr, doctorHelp)
		return 2
	}

	report := runConfigDoctorChecks(fs.Arg(0))
	return emitDoctorReport("Portier Config Doctor", report, *flagStrict, jsonOutput, *flagOut, stdout, stderr)
}

// runConfigDoctorChecks performs the offline analysis of a config file and
// returns a deterministic doctor report. Read and parse failures short-circuit
// (no later checks are meaningful); otherwise validation, duplicate-binding,
// emptiness, validity, and the LAN-exposure / privileged-port advisories run.
func runConfigDoctorChecks(filePath string) DoctorReport {
	checks := []DoctorCheckResult{}

	data, err := os.ReadFile(filePath)
	if err != nil {
		checks = append(checks, DoctorCheckResult{
			Code:     checkConfigReadFailed,
			Severity: DoctorError,
			Title:    "Config file could not be read",
			Message:  fmt.Sprintf("Reading %s failed: %v", filePath, err),
			Details:  map[string]any{"path": filePath},
		})
		return newDoctorReport(checks)
	}

	rules, parseErr := parseLocalConfig(data)
	if parseErr != nil {
		checks = append(checks, DoctorCheckResult{
			Code:     checkConfigParseFailed,
			Severity: DoctorError,
			Title:    "Config file could not be parsed",
			Message:  parseErr.Error(),
			Details:  map[string]any{"path": filePath},
		})
		return newDoctorReport(checks)
	}

	if len(rules) == 0 {
		checks = append(checks, DoctorCheckResult{
			Code:     checkConfigEmpty,
			Severity: DoctorWarning,
			Title:    "Config contains no rules",
			Message:  "The config file parsed successfully but defines no forwarding rules.",
		})
		return newDoctorReport(checks)
	}

	// Field validation and duplicate bindings are reported separately so an
	// operator can distinguish a binding conflict from a malformed rule.
	vr := validateLocalConfig(rules)
	dupErrs, fieldErrs := partitionValidationErrors(vr.Errors)
	if len(fieldErrs) > 0 {
		checks = append(checks, DoctorCheckResult{
			Code:     checkConfigValidationFailed,
			Severity: DoctorError,
			Title:    fmt.Sprintf("Config has %d validation %s", len(fieldErrs), pluralWord(len(fieldErrs), "error", "errors")),
			Message:  "One or more rules have invalid fields. Run 'portier config validate' for the full list.",
			Details:  map[string]any{"errors": fieldErrs},
		})
	}
	if len(dupErrs) > 0 {
		checks = append(checks, DoctorCheckResult{
			Code:     checkConfigDuplicateBinding,
			Severity: DoctorError,
			Title:    fmt.Sprintf("Config has %d duplicate listen %s", len(dupErrs), pluralWord(len(dupErrs), "binding", "bindings")),
			Message:  "Two or more rules share a listen binding (protocol + host + port). Each binding must be unique.",
			Details:  map[string]any{"errors": dupErrs},
		})
	}

	if vr.Valid {
		checks = append(checks, DoctorCheckResult{
			Code:     checkConfigValid,
			Severity: DoctorInfo,
			Title:    "Config is valid",
			Message: fmt.Sprintf("The config file can be read, parsed, and validated (%d %s: %d TCP, %d UDP).",
				vr.RuleCount, pluralRule(vr.RuleCount), vr.TCPCount, vr.UDPCount),
		})
	}

	// Deterministic advisories run on every parsed rule (in file order),
	// independent of field validity, since exposure and port concerns are
	// meaningful even when other fields are wrong.
	checks = append(checks, configDoctorAdvisories(rules)...)

	return newDoctorReport(checks)
}

// configDoctorAdvisories returns the LAN-exposure and privileged-port advisory
// checks for the given rules, in file order, exposure before port per rule.
func configDoctorAdvisories(rules []rawConfigRule) []DoctorCheckResult {
	advisories := []DoctorCheckResult{}
	for _, r := range rules {
		label := ruleLabel(r)
		if r.ListenHost == "0.0.0.0" {
			advisories = append(advisories, DoctorCheckResult{
				Code:     checkConfigLanExposure,
				Severity: DoctorWarning,
				Title:    fmt.Sprintf("%s listens on 0.0.0.0", label),
				Message:  "Listening on 0.0.0.0 exposes this forwarded port on all interfaces. Other LAN devices may be able to connect if firewall settings allow it.",
				Details:  map[string]any{"name": r.Name, "listenHost": r.ListenHost, "listenPort": r.ListenPort},
			})
		}
		if r.ListenPort >= 1 && r.ListenPort < 1024 {
			advisories = append(advisories, DoctorCheckResult{
				Code:     checkConfigPrivilegedPort,
				Severity: DoctorWarning,
				Title:    fmt.Sprintf("%s uses privileged port %d", label, r.ListenPort),
				Message:  fmt.Sprintf("Port %d is privileged and may require elevated permissions to bind.", r.ListenPort),
				Details:  map[string]any{"name": r.Name, "listenPort": r.ListenPort},
			})
		}
	}
	return advisories
}

// ruleLabel returns a stable human label for a rule in doctor output:
// the quoted name when present, otherwise a positional-free generic label.
func ruleLabel(r rawConfigRule) string {
	if r.Name != "" {
		return fmt.Sprintf("Rule %q", r.Name)
	}
	return "An unnamed rule"
}

// partitionValidationErrors splits validation error messages into duplicate
// listen-binding errors and all other (field-level) errors, preserving order.
func partitionValidationErrors(errs []string) (dup, field []string) {
	for _, e := range errs {
		if strings.HasPrefix(e, duplicateBindingErrPrefix) {
			dup = append(dup, e)
		} else {
			field = append(field, e)
		}
	}
	return dup, field
}

// --- local config parsing and validation ---

// configValidationResult holds the outcome of local config validation.
type configValidationResult struct {
	Valid     bool     `json:"valid"`
	RuleCount int      `json:"ruleCount"`
	TCPCount  int      `json:"tcpCount"`
	UDPCount  int      `json:"udpCount"`
	Errors    []string `json:"errors"`
}

// rawConfigRule is a forwarding rule parsed from a local config file.
type rawConfigRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
	Enabled    bool   `json:"enabled"`
	UDPMode    string `json:"udpMode"`
	Group      string `json:"group"`
}

// parseLocalConfig extracts forwarding rules from a config file in any supported shape:
//   - raw JSON array: [...]
//   - wrapper object: { "rules": [...] }
//   - exported config: { "version": "1", "exportedAt": "...", "rules": [...] }
func parseLocalConfig(data []byte) ([]rawConfigRule, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("config file is empty")
	}

	if trimmed[0] == '[' {
		var rules []rawConfigRule
		if err := json.Unmarshal(trimmed, &rules); err != nil {
			return nil, fmt.Errorf("not a valid JSON array: %w", err)
		}
		if rules == nil {
			rules = []rawConfigRule{}
		}
		return rules, nil
	}

	if trimmed[0] == '{' {
		var obj struct {
			Rules json.RawMessage `json:"rules"`
		}
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return nil, fmt.Errorf("not a valid JSON object: %w", err)
		}
		if obj.Rules == nil {
			return nil, fmt.Errorf("config object is missing the required \"rules\" field")
		}
		var rules []rawConfigRule
		if err := json.Unmarshal(obj.Rules, &rules); err != nil {
			return nil, fmt.Errorf("\"rules\" field is not a valid JSON array: %w", err)
		}
		if rules == nil {
			rules = []rawConfigRule{}
		}
		return rules, nil
	}

	return nil, fmt.Errorf("not a valid Portier config: expected a JSON array or an object with a \"rules\" field")
}

// validateLocalConfig checks rules for field validity and duplicate listen bindings.
func validateLocalConfig(rules []rawConfigRule) configValidationResult {
	errs := make([]string, 0)
	tcpCount := 0
	udpCount := 0

	validProtocols := map[string]bool{"tcp": true, "udp": true}
	validUDPModes := map[string]bool{
		"one-way":                    true,
		"bidirectional-last-client":  true,
		"bidirectional-multi-client": true,
	}

	type bindingKey struct {
		proto string
		host  string
		port  int
	}
	seen := map[bindingKey]int{} // key → 1-based rule index

	for i, r := range rules {
		ruleNum := i + 1
		prefix := fmt.Sprintf("rule %d", ruleNum)
		if r.Name != "" {
			prefix = fmt.Sprintf("rule %d %q", ruleNum, r.Name)
		}

		if r.Name == "" {
			errs = append(errs, fmt.Sprintf("%s: name is required", prefix))
		}
		if !validProtocols[r.Protocol] {
			errs = append(errs, fmt.Sprintf("%s: invalid protocol %q (must be \"tcp\" or \"udp\")", prefix, r.Protocol))
		}
		if r.ListenHost == "" {
			errs = append(errs, fmt.Sprintf("%s: listenHost is required", prefix))
		}
		if r.ListenPort < 1 || r.ListenPort > 65535 {
			errs = append(errs, fmt.Sprintf("%s: listenPort %d is out of range (must be 1–65535)", prefix, r.ListenPort))
		}
		if r.TargetHost == "" {
			errs = append(errs, fmt.Sprintf("%s: targetHost is required", prefix))
		}
		if r.TargetPort < 1 || r.TargetPort > 65535 {
			errs = append(errs, fmt.Sprintf("%s: targetPort %d is out of range (must be 1–65535)", prefix, r.TargetPort))
		}
		if r.Protocol == "udp" && r.UDPMode != "" && !validUDPModes[r.UDPMode] {
			errs = append(errs, fmt.Sprintf("%s: invalid udpMode %q", prefix, r.UDPMode))
		}
		if g := strings.TrimSpace(r.Group); g != "" {
			if utf8.RuneCountInString(g) > groupMaxLength {
				errs = append(errs, fmt.Sprintf("%s: group must be %d characters or fewer", prefix, groupMaxLength))
			} else if hasControlChar(g) {
				errs = append(errs, fmt.Sprintf("%s: group must not contain control characters", prefix))
			}
		}

		if r.Protocol == "tcp" {
			tcpCount++
		} else if r.Protocol == "udp" {
			udpCount++
		}

		// Duplicate binding check (only when basic fields are valid)
		if validProtocols[r.Protocol] && r.ListenHost != "" && r.ListenPort >= 1 && r.ListenPort <= 65535 {
			key := bindingKey{r.Protocol, r.ListenHost, r.ListenPort}
			if prev, ok := seen[key]; ok {
				errs = append(errs, fmt.Sprintf("%s %s %s:%d (rules %d and %d)",
					duplicateBindingErrPrefix, r.Protocol, r.ListenHost, r.ListenPort, prev, ruleNum))
			} else {
				seen[key] = ruleNum
			}
		}
	}

	return configValidationResult{
		Valid:     len(errs) == 0,
		RuleCount: len(rules),
		TCPCount:  tcpCount,
		UDPCount:  udpCount,
		Errors:    errs,
	}
}

// hasControlChar reports whether s contains a C0 control character (U+0000-
// U+001F) or DEL (U+007F). Used for the local group-label pre-check.
func hasControlChar(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// loadConfigForCommand reads, parses, and validates a local config file for the
// import/plan/diff/apply commands, writing the standard error output to stderr.
// On any failure (unreadable file, parse error, or invalid rules) it returns
// ok=false and the exit code the caller should return (errExit); verb names the
// command for the "Config is invalid — <verb> aborted." message. RunConfigValidate
// keeps its own JSON-aware flow and does not use this helper.
func loadConfigForCommand(filePath, verb string, errExit int, stderr io.Writer) ([]rawConfigRule, int, bool) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading %s: %v\n", filePath, err)
		return nil, errExit, false
	}

	rules, parseErr := parseLocalConfig(data)
	if parseErr != nil {
		fmt.Fprintf(stderr, "Error: %v\n", parseErr)
		return nil, errExit, false
	}

	vr := validateLocalConfig(rules)
	if !vr.Valid {
		fmt.Fprintf(stderr, "Config is invalid — %s aborted.\n", verb)
		for _, e := range vr.Errors {
			fmt.Fprintf(stderr, "  %s\n", e)
		}
		return nil, errExit, false
	}

	return rules, 0, true
}

// writePrettyJSON marshals v as indented JSON and writes it to path.
// The file is only written after a successful marshal, so no partial writes occur.
func writePrettyJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func pluralRule(n int) string {
	if n == 1 {
		return "rule"
	}
	return "rules"
}

// RunConfigPlan compares a desired config file against the running config.
func RunConfigPlan(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagFailOnDrift := fs.Bool("fail-on-drift", false, "exit code 4 when drift is detected")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, planHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, planHelp)
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "Error: plan requires a config file path")
		fmt.Fprint(stderr, planHelp)
		return 2
	}

	rules, code, ok := loadConfigForCommand(fs.Arg(0), "plan", 2, stderr)
	if !ok {
		return code
	}

	plan, err := c.PlanConfig(buildPlanRequest(rules))
	if err != nil {
		return exitWithError(err, stderr)
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, plan); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return planExitCode(plan, *flagFailOnDrift)
	}

	printPlanHuman(plan, stdout)
	return planExitCode(plan, *flagFailOnDrift)
}

// RunConfigDiff shows a human-friendly diff between a desired config and the running config.
func RunConfigDiff(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config diff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagFailOnDrift := fs.Bool("fail-on-drift", false, "exit code 4 when drift is detected")
	flagShowUnchanged := fs.Bool("show-unchanged", false, "include unchanged rules in diff output")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, diffHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, diffHelp)
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "Error: diff requires a config file path")
		fmt.Fprint(stderr, diffHelp)
		return 2
	}

	rules, code, ok := loadConfigForCommand(fs.Arg(0), "diff", 2, stderr)
	if !ok {
		return code
	}

	plan, err := c.PlanConfig(buildPlanRequest(rules))
	if err != nil {
		return exitWithError(err, stderr)
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, plan); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return planExitCode(plan, *flagFailOnDrift)
	}

	printDiffHuman(plan, *flagShowUnchanged, stdout)
	return planExitCode(plan, *flagFailOnDrift)
}

// buildPlanRequest converts local config rules into a ConfigPlanRequest.
func buildPlanRequest(rules []rawConfigRule) client.ConfigPlanRequest {
	return client.ConfigPlanRequest{
		Desired: client.ConfigPlanDesired{Rules: toConfigRules(rules)},
	}
}

// buildApplyRequest converts local config rules and apply flags into a ConfigApplyRequest.
func buildApplyRequest(rules []rawConfigRule, yes, dryRun bool) client.ConfigApplyRequest {
	return client.ConfigApplyRequest{
		Desired: client.ConfigPlanDesired{Rules: toConfigRules(rules)},
		Yes:     yes,
		DryRun:  dryRun,
	}
}

// buildImportRequest converts local config rules and a mode into a ConfigImportRequest.
func buildImportRequest(rules []rawConfigRule, mode string) client.ConfigImportRequest {
	return client.ConfigImportRequest{
		Mode: mode,
		Config: client.ConfigExportResponse{
			Version: "1",
			Rules:   toConfigRules(rules),
		},
	}
}

// toConfigRules maps locally-parsed config rules to the API ConfigRule DTO.
// Shared by the import, apply, and plan/diff request builders so the field
// mapping cannot drift between them.
func toConfigRules(rules []rawConfigRule) []client.ConfigRule {
	configRules := make([]client.ConfigRule, len(rules))
	for i, r := range rules {
		configRules[i] = client.ConfigRule{
			ID:         r.ID,
			Name:       r.Name,
			Protocol:   r.Protocol,
			ListenHost: r.ListenHost,
			ListenPort: r.ListenPort,
			TargetHost: r.TargetHost,
			TargetPort: r.TargetPort,
			Enabled:    r.Enabled,
			UDPMode:    r.UDPMode,
			Group:      r.Group,
		}
	}
	return configRules
}

// planExitCode determines the exit code for plan/diff commands.
// Priority: plan errors (1) > drift with --fail-on-drift (4) > success (0).
func planExitCode(plan *client.ConfigPlanResponse, failOnDrift bool) int {
	if plan.Summary.HasErrors {
		return 1
	}
	if failOnDrift && plan.Summary.HasDrift {
		return 4
	}
	return 0
}

// printPlanHuman renders the plan response in structured human-readable format.
func printPlanHuman(plan *client.ConfigPlanResponse, w io.Writer) {
	s := plan.Summary

	if !s.HasDrift && !s.HasErrors {
		fmt.Fprintln(w, "No drift detected.")
		if s.Unchanged > 0 {
			fmt.Fprintf(w, "  %d %s unchanged.\n", s.Unchanged, pluralRule(s.Unchanged))
		}
		if len(plan.Warnings) > 0 {
			fmt.Fprintln(w)
			printPlanWarnings(plan.Warnings, w)
		}
		return
	}

	fmt.Fprintln(w, "Config Plan")
	parts := []string{
		fmt.Sprintf("Add: %d", s.Add),
		fmt.Sprintf("Update: %d", s.Update),
		fmt.Sprintf("Remove: %d", s.Remove),
		fmt.Sprintf("Unchanged: %d", s.Unchanged),
	}
	if s.Destructive > 0 {
		parts = append(parts, fmt.Sprintf("Destructive: %d", s.Destructive))
	}
	fmt.Fprintf(w, "  %s\n", strings.Join(parts, "  "))

	if len(plan.Operations) > 0 {
		fmt.Fprintln(w)
		for _, op := range plan.Operations {
			dest := ""
			if op.Destructive {
				dest = "  [destructive]"
			}
			fmt.Fprintf(w, "  %-10s %-24s %-5s %s%s\n",
				op.Type, op.RuleName, op.Protocol, opEndpoint(op), dest)
			for _, ch := range op.Changes {
				fmt.Fprintf(w, "    %s: %s → %s\n",
					ch.Field, formatChangeValue(ch.Before), formatChangeValue(ch.After))
			}
		}
	}

	if len(plan.Warnings) > 0 {
		fmt.Fprintln(w)
		printPlanWarnings(plan.Warnings, w)
	}
	if len(plan.Errors) > 0 {
		fmt.Fprintln(w)
		printPlanErrors(plan.Errors, w)
	}
}

// printDiffHuman renders the plan response as a visual diff.
func printDiffHuman(plan *client.ConfigPlanResponse, showUnchanged bool, w io.Writer) {
	s := plan.Summary

	if !s.HasDrift && !s.HasErrors {
		fmt.Fprintln(w, "No drift detected.")
		return
	}

	for _, op := range plan.Operations {
		if op.Type == "unchanged" && !showUnchanged {
			continue
		}
		dest := ""
		if op.Destructive {
			dest = "  [destructive]"
		}
		fmt.Fprintf(w, "%s %-9s %-24s %-5s %s%s\n",
			opDiffPrefix(op.Type), opDiffLabel(op.Type),
			op.RuleName, op.Protocol, opEndpoint(op), dest)
		for _, ch := range op.Changes {
			fmt.Fprintf(w, "  %s: %s → %s\n",
				ch.Field, formatChangeValue(ch.Before), formatChangeValue(ch.After))
		}
	}

	if len(plan.Warnings) > 0 {
		fmt.Fprintln(w)
		printPlanWarnings(plan.Warnings, w)
	}
	if len(plan.Errors) > 0 {
		fmt.Fprintln(w)
		printPlanErrors(plan.Errors, w)
	}
}

func opEndpoint(op client.ConfigPlanOperation) string {
	snap := op.Current
	if snap == nil {
		snap = op.Desired
	}
	if snap == nil {
		return "—"
	}
	return fmt.Sprintf("%s:%d → %s:%d", snap.ListenHost, snap.ListenPort, snap.TargetHost, snap.TargetPort)
}

func opDiffPrefix(opType string) string {
	switch opType {
	case "add":
		return "+"
	case "update":
		return "~"
	case "remove":
		return "-"
	default:
		return "="
	}
}

func opDiffLabel(opType string) string {
	switch opType {
	case "add":
		return "Add:"
	case "update":
		return "Update:"
	case "remove":
		return "Remove:"
	default:
		return "Unchanged:"
	}
}

func formatChangeValue(v any) string {
	if v == nil {
		return "(none)"
	}
	if f, ok := v.(float64); ok {
		if f == float64(int64(f)) {
			return fmt.Sprintf("%d", int64(f))
		}
		return fmt.Sprintf("%g", f)
	}
	return fmt.Sprintf("%v", v)
}

func printPlanWarnings(warnings []client.ConfigPlanWarning, w io.Writer) {
	fmt.Fprintln(w, "Warnings:")
	for _, warn := range warnings {
		fmt.Fprintf(w, "  [%s] %s\n", warn.Code, warn.Message)
	}
}

func printPlanErrors(errs []client.ConfigPlanError, w io.Writer) {
	fmt.Fprintln(w, "Errors:")
	for _, e := range errs {
		fmt.Fprintf(w, "  [%s] %s\n", e.Code, e.Message)
	}
}

// RunConfigApply applies a desired config file to the running Portier service.
func RunConfigApply(c *client.Client, jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config apply", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagYes := fs.Bool("yes", false, "confirm destructive operations")
	flagDryRun := fs.Bool("dry-run", false, "preview changes without mutating the running config")
	flagBackupOut := fs.String("backup-out", "", "export current config to this file before applying")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, applyHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, applyHelp)
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "Error: apply requires a config file path")
		fmt.Fprint(stderr, applyHelp)
		return 2
	}

	rules, code, ok := loadConfigForCommand(fs.Arg(0), "apply", 2, stderr)
	if !ok {
		return code
	}

	// Export backup before applying if --backup-out was specified.
	if *flagBackupOut != "" && !*flagDryRun {
		cfg, backupErr := c.ExportConfig()
		if backupErr != nil {
			fmt.Fprintf(stderr, "Error exporting backup: %v\n", backupErr)
			fmt.Fprintln(stderr, "Apply aborted — backup failed.")
			return 1
		}
		if writeErr := writePrettyJSON(*flagBackupOut, cfg); writeErr != nil {
			fmt.Fprintf(stderr, "Error writing backup to %s: %v\n", *flagBackupOut, writeErr)
			fmt.Fprintln(stderr, "Apply aborted — backup failed.")
			return 1
		}
		if !jsonOutput {
			fmt.Fprintf(stdout, "Backup written to %s\n", *flagBackupOut)
		}
	}

	resp, err := c.ApplyConfig(buildApplyRequest(rules, *flagYes, *flagDryRun))
	if err != nil {
		return exitWithError(err, stderr)
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, resp); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return applyExitCode(resp)
	}

	printApplyHuman(resp, stdout, stderr)
	return applyExitCode(resp)
}

// applyExitCode maps an apply response to an exit code.
func applyExitCode(resp *client.ConfigApplyResponse) int {
	if !resp.Ok {
		return 1
	}
	return 0
}

// printApplyHuman renders the apply response in human-readable format.
func printApplyHuman(resp *client.ConfigApplyResponse, stdout, stderr io.Writer) {
	plan := resp.Plan

	if !resp.Ok {
		fmt.Fprintln(stderr, "Apply failed — plan has errors.")
		printPlanErrors(plan.Errors, stderr)
		return
	}

	if resp.DryRun {
		fmt.Fprintln(stdout, "Dry run complete — no changes applied.")
		if !plan.Summary.HasDrift {
			fmt.Fprintln(stdout, "  No drift detected.")
			return
		}
		fmt.Fprintf(stdout, "  Would apply: +%d update:%d -%d (unchanged:%d)\n",
			resp.Applied.Add, resp.Applied.Update, resp.Applied.Remove, resp.Applied.Unchanged)
		return
	}

	if !plan.Summary.HasDrift {
		fmt.Fprintln(stdout, "No drift detected — nothing to apply.")
		return
	}

	fmt.Fprintln(stdout, "Config applied.")
	fmt.Fprintf(stdout, "  +%d added  ~%d updated  -%d removed  =%d unchanged\n",
		resp.Applied.Add, resp.Applied.Update, resp.Applied.Remove, resp.Applied.Unchanged)

	if len(plan.Warnings) > 0 {
		fmt.Fprintln(stdout)
		printPlanWarnings(plan.Warnings, stdout)
	}
}
