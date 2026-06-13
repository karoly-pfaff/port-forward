package commands

import (
	"flag"
	"fmt"
	"io"
	"os"

	"portier/cli/sources/client"
	"portier/cli/sources/config"
	"portier/cli/sources/doctor"
	"portier/cli/sources/output"
	"portier/cli/sources/planview"
)

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

const doctorHelp = `Usage: portier config doctor [--strict] [--explain] [--out <file>] <file>

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
  --explain      Show an explanation (meaning + next action) for each emitted check.
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
  portier config doctor --explain desired.json
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

	if err := output.WritePrettyJSON(*flagOut, cfg); err != nil {
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
		ruleCount, output.PluralRule(ruleCount), *flagOut)
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
			resp.Result.Imported, output.PluralRule(resp.Result.Imported))
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

	rules, parseErr := config.ParseLocal(data)
	if parseErr != nil {
		if jsonOutput {
			if err := output.PrintJSON(stdout, config.ValidationResult{
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

	vr := config.Validate(rules)

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
			vr.RuleCount, output.PluralRule(vr.RuleCount), vr.TCPCount, vr.UDPCount)
		return 0
	}

	fmt.Fprintln(stdout, "Config is invalid.")
	for _, e := range vr.Errors {
		fmt.Fprintf(stdout, "  %s\n", e)
	}
	return 1
}

// RunConfigDoctor runs deterministic, offline diagnostic checks on a local
// config file and prints a doctor report. It never contacts the running
// service and never modifies the file. Exit codes follow the doctor policy:
// 0 = no error-severity checks, 1 = one or more error-severity checks,
// 2 = missing argument / usage error.
func RunConfigDoctor(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagStrict := fs.Bool("strict", false, "treat warnings as failures (exit 1)")
	flagExplain := fs.Bool("explain", false, "show an explanation for each emitted check")
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

	report, summary := doctor.RunConfigChecks(fs.Arg(0))
	return doctor.Emit("Portier Config Doctor", report, doctor.EmitOptions{
		Strict: *flagStrict, Explain: *flagExplain, JSON: jsonOutput, OutPath: *flagOut, Config: summary,
	}, stdout, stderr)
}

// --- local config parsing and validation ---

// loadConfigForCommand reads, parses, and validates a local config file for the
// import/plan/diff/apply commands, writing the standard error output to stderr.
// On any failure (unreadable file, parse error, or invalid rules) it returns
// ok=false and the exit code the caller should return (errExit); verb names the
// command for the "Config is invalid — <verb> aborted." message. RunConfigValidate
// keeps its own JSON-aware flow and does not use this helper.
func loadConfigForCommand(filePath, verb string, errExit int, stderr io.Writer) ([]config.Rule, int, bool) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading %s: %v\n", filePath, err)
		return nil, errExit, false
	}

	rules, parseErr := config.ParseLocal(data)
	if parseErr != nil {
		fmt.Fprintf(stderr, "Error: %v\n", parseErr)
		return nil, errExit, false
	}

	vr := config.Validate(rules)
	if !vr.Valid {
		fmt.Fprintf(stderr, "Config is invalid — %s aborted.\n", verb)
		for _, e := range vr.Errors {
			fmt.Fprintf(stderr, "  %s\n", e)
		}
		return nil, errExit, false
	}

	return rules, 0, true
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
		return planview.PlanExitCode(plan, *flagFailOnDrift)
	}

	planview.PrintPlan(plan, stdout)
	return planview.PlanExitCode(plan, *flagFailOnDrift)
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
		return planview.PlanExitCode(plan, *flagFailOnDrift)
	}

	planview.PrintDiff(plan, *flagShowUnchanged, stdout)
	return planview.PlanExitCode(plan, *flagFailOnDrift)
}

// buildPlanRequest converts local config rules into a ConfigPlanRequest.
func buildPlanRequest(rules []config.Rule) client.ConfigPlanRequest {
	return client.ConfigPlanRequest{
		Desired: client.ConfigPlanDesired{Rules: toConfigRules(rules)},
	}
}

// buildApplyRequest converts local config rules and apply flags into a ConfigApplyRequest.
func buildApplyRequest(rules []config.Rule, yes, dryRun bool) client.ConfigApplyRequest {
	return client.ConfigApplyRequest{
		Desired: client.ConfigPlanDesired{Rules: toConfigRules(rules)},
		Yes:     yes,
		DryRun:  dryRun,
	}
}

// buildImportRequest converts local config rules and a mode into a ConfigImportRequest.
func buildImportRequest(rules []config.Rule, mode string) client.ConfigImportRequest {
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
func toConfigRules(rules []config.Rule) []client.ConfigRule {
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
		if writeErr := output.WritePrettyJSON(*flagBackupOut, cfg); writeErr != nil {
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
		return planview.ApplyExitCode(resp)
	}

	planview.PrintApply(resp, stdout, stderr)
	return planview.ApplyExitCode(resp)
}
