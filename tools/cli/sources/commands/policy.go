package commands

import (
	"flag"
	"fmt"
	"io"
	"os"

	"portier/cli/sources/config"
	"portier/cli/sources/policy"
)

const policyHelp = `Usage: portier policy <subcommand> [options]

Evaluate Portier config files against a small, offline policy file.

Subcommands:
  check --config <file> --policy <file>   Evaluate a config against a policy (read-only, offline)
  help                                    Show this help message

Run 'portier policy check --help' for check options.
`

const policyCheckHelp = `Usage: portier policy check --config <config-file> --policy <policy-file>

Evaluate a Portier config file against a JSON policy file. Fully offline:
never contacts the runtime, never probes targets, and never modifies any file.

Options:
  --config <file>   Path to the Portier config file to evaluate (required).
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
  1  Policy evaluation completed with one or more violations, or the --out file
     could not be written
  2  Missing/invalid arguments (including a missing --out value), or an
     unreadable/malformed config or policy file (including an unsupported
     schemaVersion)

Examples:
  portier policy check --config portier.json --policy policy.json
  portier --json policy check --config portier.json --policy policy.json
`

// RunPolicy dispatches the `portier policy <subcommand>` commands. Policy
// evaluation is fully offline, so no management client is needed.
func RunPolicy(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, policyHelp)
		return 2
	}
	switch args[0] {
	case "check":
		return RunPolicyCheck(jsonOutput, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, policyHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown policy subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, policyHelp)
		return 2
	}
}

// RunPolicyCheck evaluates a local config file against a local policy file and
// prints a deterministic policy report. With --out it also writes the JSON report
// to a file. It is fully offline: it never contacts the runtime, never probes
// targets, and never modifies the config or policy file. Exit codes: 0 = no
// violations, 1 = one or more violations (or a JSON-encode / --out write failure),
// 2 = usage error (including a missing --out value) or an unreadable/malformed
// config or policy file.
func RunPolicyCheck(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("policy check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagConfig := fs.String("config", "", "path to the Portier config file")
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

	if *flagConfig == "" {
		fmt.Fprintln(stderr, "Error: --config is required")
		fmt.Fprint(stderr, policyCheckHelp)
		return 2
	}
	if *flagPolicy == "" {
		fmt.Fprintln(stderr, "Error: --policy is required")
		fmt.Fprint(stderr, policyCheckHelp)
		return 2
	}

	configData, err := os.ReadFile(*flagConfig)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading config %s: %v\n", *flagConfig, err)
		return 2
	}
	rules, parseErr := config.ParseLocal(configData)
	if parseErr != nil {
		fmt.Fprintf(stderr, "Error: invalid config %s: %v\n", *flagConfig, parseErr)
		return 2
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

	report := policy.Evaluate(rules, pol)
	return policy.Emit(report, policy.EmitOptions{Explain: *flagExplain, JSON: jsonOutput, OutPath: *flagOut}, stdout, stderr)
}
