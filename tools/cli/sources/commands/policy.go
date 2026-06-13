package commands

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"portier/cli/sources/client"
	"portier/cli/sources/config"
	"portier/cli/sources/output"
	"portier/cli/sources/policy"
)

// ConnFlags carries the global connection flags so an otherwise-offline command
// can lazily resolve a runtime URL ONLY when a runtime mode is requested. The
// offline policy subcommands never touch it; `policy check --runtime` resolves
// the URL and builds a client on demand inside the handler.
type ConnFlags struct {
	URL  string
	Host string
	Port int
}

const policyHelp = `Usage: portier policy <subcommand> [options]

Evaluate Portier config files against a small, offline policy file, and inspect
built-in policy templates.

Subcommands:
  check --config <file> --policy <file>   Evaluate a config against a policy (read-only, offline)
  review --current <file> --candidate <file> --policy <file>
                                          Compare current vs candidate config and evaluate the
                                          candidate against a policy (read-only, offline)
  baseline create|compare ...             Save an accepted policy report as a baseline, or compare
                                          a fresh report against it (offline)
  template <name> | --list                Print a built-in policy template, or list them
  help                                    Show this help message

Run 'portier policy check --help', 'portier policy review --help',
'portier policy baseline --help', or 'portier policy template --help' for options.
`

const policyBaselineHelp = `Usage: portier policy baseline <subcommand> [options]

Save an accepted set of policy findings as a baseline, then compare later policy
reports against it — a dry-run acceptance workflow. Fully offline: works on the
policy report JSON files produced by 'policy check' / 'policy review' (--json or
--out); never contacts the runtime and never mutates inputs.

Subcommands:
  create --from-report <report.json> --out <baseline.json>
        Save a policy report's findings as an accepted baseline.
  compare --baseline <baseline.json> --report <report.json>
        Compare a fresh policy report against a baseline (new/resolved/unchanged).
  help  Show this help message.
`

const policyBaselineCreateHelp = `Usage: portier policy baseline create --from-report <report.json> --out <baseline.json>

Save an accepted policy report's findings as a baseline file. Fully offline: it
reads the report, writes a compact baseline JSON (schemaVersion 1: createdAt,
source, result, and fingerprinted findings — no raw config, no secrets, no
runtime host data), and never contacts the runtime or mutates the report.

The input report may come from 'policy check' or 'policy review' (--json/--out).

Options:
  --from-report <file>  Path to the policy report JSON to baseline (required).
  --out <file>          Path to write the baseline JSON (required).

Exit codes:
  0  Baseline written
  1  Output file write failure
  2  Missing/invalid arguments, or an unreadable/malformed report
`

const policyBaselineCompareHelp = `Usage: portier policy baseline compare --baseline <baseline.json> --report <report.json>

Compare a fresh policy report against an accepted baseline and report which
findings are new, resolved, or unchanged. Fully offline; never contacts the
runtime and never mutates inputs. New findings fail the comparison; resolved-only
changes do not.

Options:
  --baseline <file>  Path to the baseline JSON (required).
  --report <file>    Path to the fresh policy report JSON to compare (required).

Output: a human report (or the full comparison under --json) listing new,
resolved, and unchanged findings, and a Result: passed/failed line.

Exit codes:
  0  No new findings (passed)
  1  One or more new findings (failed)
  2  Missing/invalid arguments, or an unreadable/malformed baseline or report
`

const policyReviewHelp = `Usage: portier policy review --current <file> --candidate <file> --policy <file>

Compare a current config with a candidate config and evaluate ONLY the candidate
against a policy: "if I moved from current to candidate, would the candidate pass
this policy, and what changed?". Fully offline and dry-run — it reads the three
files, never contacts the runtime, never probes targets, and never modifies any
file except the requested --out file. It does NOT apply or import anything.

Options:
  --current <file>    Path to the current Portier config file (required).
  --candidate <file>  Path to the candidate Portier config file (required).
  --policy <file>     Path to the JSON policy file (required).
  --explain           Show an explanation for each emitted policy finding (inline
                      in human output; an additive explanations map in --json).
  --out <file>        Also write the JSON review report to <file> (same shape as
                      --json). With --json the JSON also prints to stdout and is
                      byte-identical to the file.

The review reports a compact change summary (current vs candidate rule/group
counts and their delta) plus the candidate's policy findings, summary, and result
(reusing 'policy check' semantics). --explain does not change the findings,
summary, result, review, or exit code.

Exit codes:
  0  Candidate passes the policy
  1  Candidate violates the policy, or an --out write failure
  2  Missing/invalid arguments, or an unreadable/malformed config or policy file
     (including an unsupported schemaVersion)

Examples:
  portier policy review --current current.json --candidate candidate.json --policy policy.json
  portier --json policy review --current current.json --candidate candidate.json --policy policy.json --explain
`

const policyTemplateHelp = `Usage: portier policy template <name> [--out <file>]
       portier policy template --list

Print a built-in policy template, or list the available templates. Fully offline:
never contacts the runtime and never modifies any file except the requested --out
file. A rendered template is a complete policy file (schemaVersion 1) that can be
passed straight to 'portier policy check --policy <file>'.

Options:
  --list            List the available templates (name, title, description).
  --out <file>      Write the template's policy JSON to <file> (bare policy only,
                    directly usable by 'policy check'). Without --out, the policy
                    JSON is printed to stdout.

Output:
  policy template <name>           Prints the template's policy JSON to stdout.
  policy template <name> --json    Prints a metadata wrapper {name,title,description,policy}.
  policy template <name> --out f   Writes the policy JSON to f (human mode confirms on stdout).
  policy template --list           Prints a compact human list of the templates.
  policy template --list --json    Prints {"templates":[{name,title,description}]}.

Built-in templates: local-safe, managed, permissive.

Exit codes:
  0  Success
  1  Output file write failure (an operation failure)
  2  Missing/invalid arguments (including a missing --out value) or an unknown template

Examples:
  portier policy template --list
  portier policy template local-safe
  portier policy template managed --out policy.json
  portier policy check --config portier.json --policy policy.json
`

const policyCheckHelp = `Usage: portier policy check --config <config-file> --policy <policy-file>
       portier policy check --runtime --policy <policy-file>

Evaluate a Portier config against a JSON policy file. Choose exactly one config
source: a local file (--config, fully offline) or the live runtime config
(--runtime, read-only). Either way it is read-only and dry-run: it never probes
targets, never enforces the policy, and never modifies the config, the policy
file, or the runtime.

Config source (exactly one required):
  --config <file>   Evaluate a local Portier config file (fully offline — does not
                    contact or resolve the runtime URL).
  --runtime         Evaluate the live runtime config, read via the existing
                    read-only config-export path (uses the global --url/--host/
                    --port like other live commands). No new endpoint, no probing.

Options:
  --policy <file>   Path to the JSON policy file (required).
  --explain         Show an explanation (meaning + next action) for each emitted
                    finding (inline in human output; an additive explanations map
                    in --json). Does not change findings, result, or exit code.
  --out <file>      Also write the JSON report to <file> (same shape as --json,
                    including the additive explanations map under --explain). The
                    file is written regardless of --json; with --json the JSON
                    also prints to stdout (stdout and file are byte-identical).

Policy file format (schemaVersion 1):
  {
    "schemaVersion": 1,
    "rules": {
      "requireGroup": false,
      "allowLanExposure": true,
      "allowPrivilegedPorts": true,
      "allowAutostart": true,
      "forbidDuplicateBindings": true
    }
  }

Policy rules (each omitted field falls back to the permissive default shown above):
  requireGroup             When true, every rule must have a non-empty group.
  allowLanExposure         When false, listening on 0.0.0.0 is a violation.
  allowPrivilegedPorts     When false, listen ports below 1024 are violations.
  allowAutostart           When false, autostart-enabled rules are violations.
  forbidDuplicateBindings  When true, duplicate protocol+host+port bindings are violations.

Unknown fields in the policy file are rejected.

Exit codes:
  0  Policy evaluation completed with no violations
  1  Policy evaluation completed with one or more violations, the --out file
     could not be written, or (runtime mode) the runtime config could not be read
  2  Missing/invalid arguments (including a missing --out value, or both/neither
     of --config and --runtime), or an unreadable/malformed config or policy file
     (including an unsupported schemaVersion)
  3  (runtime mode) The Portier runtime is unreachable

Examples:
  portier policy check --config portier.json --policy policy.json
  portier policy check --runtime --policy policy.json
  portier --json policy check --runtime --policy policy.json --explain
`

// RunPolicy dispatches the `portier policy <subcommand>` commands. All
// subcommands are offline except `policy check --runtime`, which lazily resolves
// the runtime URL from conn inside the handler; the offline subcommands never
// touch conn.
func RunPolicy(jsonOutput bool, conn ConnFlags, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, policyHelp)
		return 2
	}
	switch args[0] {
	case "check":
		return RunPolicyCheck(jsonOutput, conn, args[1:], stdout, stderr)
	case "review":
		return RunPolicyReview(jsonOutput, args[1:], stdout, stderr)
	case "baseline":
		return RunPolicyBaseline(jsonOutput, args[1:], stdout, stderr)
	case "template":
		return RunPolicyTemplate(jsonOutput, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, policyHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown policy subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, policyHelp)
		return 2
	}
}

// exportedRules converts the read-only runtime config-export response into the
// config domain rules the evaluator consumes. The fields map 1:1, so a runtime
// config evaluates identically to the same config in a local file.
func exportedRules(cfg *client.ConfigExportResponse) []config.Rule {
	rules := make([]config.Rule, len(cfg.Rules))
	for i, r := range cfg.Rules {
		rules[i] = config.Rule{
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
	return rules
}

// loadCheckRules selects the config source for `policy check` and returns the
// rules to evaluate, the source label, and an exit code (0 = ok). With --runtime
// it reads the live runtime config read-only via the existing config-export path
// (lazily resolving the URL from conn — offline --config mode never does this):
// an unreachable runtime exits 3 and a config-export/API failure exits 1 (via
// exitWithError), NOT a fabricated policy report. With --config it parses the
// local file fully offline. Exactly-one-source is enforced by the caller.
func loadCheckRules(useRuntime bool, configPath string, conn ConnFlags, stderr io.Writer) ([]config.Rule, string, int) {
	if useRuntime {
		managementURL, err := ResolveURL(conn.URL, conn.Host, conn.Port)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return nil, "", 2
		}
		cfg, expErr := client.New(managementURL).ExportConfig()
		if expErr != nil {
			return nil, "", exitWithError(expErr, stderr)
		}
		return exportedRules(cfg), policy.SourceRuntime, 0
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading config %s: %v\n", configPath, err)
		return nil, "", 2
	}
	rules, parseErr := config.ParseLocal(configData)
	if parseErr != nil {
		fmt.Fprintf(stderr, "Error: invalid config %s: %v\n", configPath, parseErr)
		return nil, "", 2
	}
	return rules, policy.SourceConfigFile, 0
}

// RunPolicyCheck evaluates a Portier config against a policy file and prints a
// deterministic policy report. The config source is exactly one of --config (a
// local file, fully offline) or --runtime (the live runtime config, read-only via
// the existing config-export path). It never probes targets, never enforces the
// policy, and never modifies the config, the policy file, or the runtime. Exit
// codes: 0 = no violations, 1 = one or more violations / a JSON-encode or --out
// write failure / (runtime) a config-export failure, 2 = usage error (including a
// missing --out value or both/neither config source) or an unreadable/malformed
// config or policy file, 3 = (runtime) the runtime is unreachable.
func RunPolicyCheck(jsonOutput bool, conn ConnFlags, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("policy check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagConfig := fs.String("config", "", "path to the Portier config file (offline)")
	flagRuntime := fs.Bool("runtime", false, "evaluate the live runtime config (read-only)")
	flagPolicy := fs.String("policy", "", "path to the policy file")
	flagExplain := fs.Bool("explain", false, "show an explanation for each emitted finding")
	flagOut := fs.String("out", "", "also write the JSON report to this file")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, policyCheckHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, policyCheckHelp)
		return 2
	}

	// Exactly one config source.
	if *flagConfig != "" && *flagRuntime {
		fmt.Fprintln(stderr, "Error: --config and --runtime are mutually exclusive")
		fmt.Fprint(stderr, policyCheckHelp)
		return 2
	}
	if *flagConfig == "" && !*flagRuntime {
		fmt.Fprintln(stderr, "Error: one of --config or --runtime is required")
		fmt.Fprint(stderr, policyCheckHelp)
		return 2
	}
	if *flagPolicy == "" {
		fmt.Fprintln(stderr, "Error: --policy is required")
		fmt.Fprint(stderr, policyCheckHelp)
		return 2
	}

	// Validate the policy file (a usage/input error) before any runtime I/O.
	policyData, err := os.ReadFile(*flagPolicy)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading policy %s: %v\n", *flagPolicy, err)
		return 2
	}
	pol, polErr := policy.Parse(policyData)
	if polErr != nil {
		fmt.Fprintf(stderr, "Error: invalid policy %s: %v\n", *flagPolicy, polErr)
		return 2
	}

	rules, source, code := loadCheckRules(*flagRuntime, *flagConfig, conn, stderr)
	if code != 0 {
		return code
	}

	report := policy.Evaluate(rules, pol)
	return policy.Emit(report, policy.EmitOptions{Source: source, Explain: *flagExplain, JSON: jsonOutput, OutPath: *flagOut}, stdout, stderr)
}

// loadReviewConfig reads and parses a local config file for `policy review`,
// labelling errors with the role (current/candidate). On failure it writes a
// clear message to stderr and returns exit code 2; on success it returns the
// parsed rules and 0.
func loadReviewConfig(path, label string, stderr io.Writer) ([]config.Rule, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading %s config %s: %v\n", label, path, err)
		return nil, 2
	}
	rules, parseErr := config.ParseLocal(data)
	if parseErr != nil {
		fmt.Fprintf(stderr, "Error: invalid %s config %s: %v\n", label, path, parseErr)
		return nil, 2
	}
	return rules, 0
}

// RunPolicyReview compares a current config with a candidate config and evaluates
// ONLY the candidate against a policy, printing a deterministic review (change
// summary + candidate policy findings). It is fully offline and dry-run: it never
// contacts the runtime, never probes targets, and never modifies any file except
// the requested --out file. Exit codes: 0 = candidate passes, 1 = candidate
// violates the policy (or a JSON-encode / --out write failure), 2 = usage error
// or an unreadable/malformed config or policy file.
func RunPolicyReview(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("policy review", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagCurrent := fs.String("current", "", "path to the current config file")
	flagCandidate := fs.String("candidate", "", "path to the candidate config file")
	flagPolicy := fs.String("policy", "", "path to the policy file")
	flagExplain := fs.Bool("explain", false, "show an explanation for each emitted finding")
	flagOut := fs.String("out", "", "also write the JSON review report to this file")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, policyReviewHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, policyReviewHelp)
		return 2
	}

	if *flagCurrent == "" {
		fmt.Fprintln(stderr, "Error: --current is required")
		fmt.Fprint(stderr, policyReviewHelp)
		return 2
	}
	if *flagCandidate == "" {
		fmt.Fprintln(stderr, "Error: --candidate is required")
		fmt.Fprint(stderr, policyReviewHelp)
		return 2
	}
	if *flagPolicy == "" {
		fmt.Fprintln(stderr, "Error: --policy is required")
		fmt.Fprint(stderr, policyReviewHelp)
		return 2
	}

	current, code := loadReviewConfig(*flagCurrent, "current", stderr)
	if code != 0 {
		return code
	}
	candidate, code := loadReviewConfig(*flagCandidate, "candidate", stderr)
	if code != 0 {
		return code
	}

	policyData, err := os.ReadFile(*flagPolicy)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading policy %s: %v\n", *flagPolicy, err)
		return 2
	}
	pol, polErr := policy.Parse(policyData)
	if polErr != nil {
		fmt.Fprintf(stderr, "Error: invalid policy %s: %v\n", *flagPolicy, polErr)
		return 2
	}

	review := policy.BuildReview(current, candidate, pol)
	return policy.EmitReview(review, policy.EmitOptions{Explain: *flagExplain, JSON: jsonOutput, OutPath: *flagOut}, stdout, stderr)
}

// RunPolicyBaseline dispatches the `policy baseline <subcommand>` commands. All
// are fully offline (they operate on local report/baseline JSON files).
func RunPolicyBaseline(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, policyBaselineHelp)
		return 2
	}
	switch args[0] {
	case "create":
		return RunPolicyBaselineCreate(jsonOutput, args[1:], stdout, stderr)
	case "compare":
		return RunPolicyBaselineCompare(jsonOutput, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, policyBaselineHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown baseline subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, policyBaselineHelp)
		return 2
	}
}

// RunPolicyBaselineCreate reads a policy report JSON file and writes a compact
// accepted baseline. Fully offline: it never contacts the runtime and never
// mutates the report. Exit codes: 0 = baseline written, 1 = output-file write
// failure (or a JSON-encode failure of the summary), 2 = usage error or an
// unreadable/malformed report.
func RunPolicyBaselineCreate(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("policy baseline create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagFrom := fs.String("from-report", "", "path to the policy report JSON")
	flagOut := fs.String("out", "", "path to write the baseline JSON")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, policyBaselineCreateHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, policyBaselineCreateHelp)
		return 2
	}
	if *flagFrom == "" {
		fmt.Fprintln(stderr, "Error: --from-report is required")
		fmt.Fprint(stderr, policyBaselineCreateHelp)
		return 2
	}
	if *flagOut == "" {
		fmt.Fprintln(stderr, "Error: --out is required")
		fmt.Fprint(stderr, policyBaselineCreateHelp)
		return 2
	}

	reportData, err := os.ReadFile(*flagFrom)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading report %s: %v\n", *flagFrom, err)
		return 2
	}
	snap, snapErr := policy.ParseReportSnapshot(reportData)
	if snapErr != nil {
		fmt.Fprintf(stderr, "Error: invalid report %s: %v\n", *flagFrom, snapErr)
		return 2
	}

	baseline := policy.BuildBaseline(snap, time.Now())
	if err := output.WritePrettyJSON(*flagOut, baseline); err != nil {
		fmt.Fprintf(stderr, "Error writing %s: %v\n", *flagOut, err)
		return 1
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, map[string]any{
			"ok": true, "path": *flagOut, "findingCount": len(baseline.Findings),
		}); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintf(stdout, "Baseline written to %s (%d %s)\n", *flagOut, len(baseline.Findings), output.PluralWord(len(baseline.Findings), "finding", "findings"))
	}
	return 0
}

// RunPolicyBaselineCompare compares a fresh policy report against a baseline and
// prints which findings are new, resolved, or unchanged. Fully offline: it never
// contacts the runtime and never mutates the inputs. Exit codes: 0 = no new
// findings, 1 = one or more new findings (or a JSON-encode failure), 2 = usage
// error or an unreadable/malformed baseline or report.
func RunPolicyBaselineCompare(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("policy baseline compare", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagBaseline := fs.String("baseline", "", "path to the baseline JSON")
	flagReport := fs.String("report", "", "path to the fresh policy report JSON")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, policyBaselineCompareHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, policyBaselineCompareHelp)
		return 2
	}
	if *flagBaseline == "" {
		fmt.Fprintln(stderr, "Error: --baseline is required")
		fmt.Fprint(stderr, policyBaselineCompareHelp)
		return 2
	}
	if *flagReport == "" {
		fmt.Fprintln(stderr, "Error: --report is required")
		fmt.Fprint(stderr, policyBaselineCompareHelp)
		return 2
	}

	baselineData, err := os.ReadFile(*flagBaseline)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading baseline %s: %v\n", *flagBaseline, err)
		return 2
	}
	baseline, baseErr := policy.ParseBaseline(baselineData)
	if baseErr != nil {
		fmt.Fprintf(stderr, "Error: invalid baseline %s: %v\n", *flagBaseline, baseErr)
		return 2
	}

	reportData, err := os.ReadFile(*flagReport)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading report %s: %v\n", *flagReport, err)
		return 2
	}
	snap, snapErr := policy.ParseReportSnapshot(reportData)
	if snapErr != nil {
		fmt.Fprintf(stderr, "Error: invalid report %s: %v\n", *flagReport, snapErr)
		return 2
	}

	report := policy.Compare(baseline, snap)
	if jsonOutput {
		if err := output.PrintJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		policy.PrintCompareHuman(report, stdout)
	}
	return policy.CompareExitCode(report)
}

// RunPolicyTemplate prints a built-in policy template (or lists them). It is
// fully offline: it never contacts the runtime and never modifies any file
// except the requested --out file. A rendered template is a complete policy file
// in the schema `policy check` accepts. Exit codes: 0 = success, 1 = output-file
// write failure, 2 = usage error (including a missing --out value) or an unknown
// template.
func RunPolicyTemplate(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("policy template", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagList := fs.Bool("list", false, "list the available templates")
	flagOut := fs.String("out", "", "write the template policy JSON to this file")

	// Parse flags that may appear before AND after the positional template name
	// (e.g. `template managed --out f`); Go's flag package otherwise stops at the
	// first non-flag argument.
	var positional []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			if err == flag.ErrHelp {
				fmt.Fprint(stdout, policyTemplateHelp)
				return 0
			}
			fmt.Fprintf(stderr, "Error: %v\n\n", err)
			fmt.Fprint(stderr, policyTemplateHelp)
			return 2
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}

	if *flagList {
		if len(positional) > 0 {
			fmt.Fprintf(stderr, "Error: --list takes no template name (got %q)\n\n", positional[0])
			fmt.Fprint(stderr, policyTemplateHelp)
			return 2
		}
		if *flagOut != "" {
			fmt.Fprintln(stderr, "Error: --out cannot be combined with --list")
			fmt.Fprint(stderr, policyTemplateHelp)
			return 2
		}
		if err := policy.PrintTemplateList(jsonOutput, stdout); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if len(positional) == 0 {
		fmt.Fprintln(stderr, "Error: a template name is required (or use --list)")
		fmt.Fprint(stderr, policyTemplateHelp)
		return 2
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "Error: too many arguments (expected one template name, got %d)\n\n", len(positional))
		fmt.Fprint(stderr, policyTemplateHelp)
		return 2
	}

	name := positional[0]
	tmpl, ok := policy.FindTemplate(name)
	if !ok {
		fmt.Fprintf(stderr, "Error: unknown template %q. Available: %s\n", name, strings.Join(policy.TemplateNames(), ", "))
		return 2
	}

	// stdout carries the metadata+policy wrapper under --json, the bare policy
	// JSON in human mode, or (human mode + --out) just the write confirmation —
	// the --out file always gets the bare policy JSON so it is directly usable by
	// `policy check`.
	if jsonOutput {
		if err := output.PrintJSON(stdout, tmpl.DetailValue()); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else if *flagOut == "" {
		if err := output.PrintJSON(stdout, tmpl.PolicyValue()); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	}

	if *flagOut != "" {
		if err := output.WritePrettyJSON(*flagOut, tmpl.PolicyValue()); err != nil {
			fmt.Fprintf(stderr, "Error writing %s: %v\n", *flagOut, err)
			return 1
		}
		if !jsonOutput {
			fmt.Fprintf(stdout, "Policy written to %s\n", *flagOut)
		}
	}

	return 0
}
