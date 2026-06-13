package commands

import (
	"flag"
	"fmt"
	"io"
	"os"

	"portier/cli/sources/workflow"
)

const workflowHelp = `Usage: portier workflow <subcommand> [options]

Plan and validate a local workflow — an ordered sequence of existing safe Portier
operations described in a small JSON file. Fully offline and dry-run: it reads the
workflow file, validates its schema and step references, and prints a deterministic
plan. It does NOT execute any step, never contacts the runtime, never applies or
imports configs, never enforces a policy, and never mutates any file (except the
requested --out file).

Subcommands:
  plan --file <workflow.json>   Validate a workflow file and print its plan
  help                          Show this help message

Run 'portier workflow plan --help' for options.
`

const workflowPlanHelp = `Usage: portier workflow plan --file <workflow.json> [--out <file>]

Read a local workflow file, validate its schema and step references, and print a
deterministic plan describing each step. Fully offline and dry-run: it does NOT
execute any step, never contacts the runtime, never reads the files a step refers
to, never enforces a policy, and never mutates any file except the requested
--out file.

Options:
  --file <file>   Path to the workflow JSON file (required).
  --out <file>    Also write the JSON plan to <file> (same shape as --json). With
                  --json the JSON also prints to stdout and is byte-identical to
                  the file.

Workflow file format (schemaVersion 1):
  {
    "schemaVersion": 1,
    "name": "local-policy-check",
    "steps": [
      { "id": "check-current", "type": "policy.check",
        "config": "portier.json", "policy": "local-safe.policy.json" },
      { "id": "compare-baseline", "type": "policy.baseline.compare",
        "baseline": "policy-baseline.json", "reportFrom": "check-current" }
    ]
  }

Supported step types and their required fields:
  policy.check              Exactly one of "config" (a file) or "runtime" (true),
                            plus "policy" (a file).
  policy.review             "current", "candidate", and "policy" (all files).
  policy.baseline.compare   "baseline" (a file), plus exactly one of "report" (a
                            file) or "reportFrom" (an earlier step id).

Unknown fields in the workflow file are rejected. Step ids must be present and
unique. A "reportFrom" must reference a step that appears earlier in the file.

Exit codes:
  0  The workflow plan is valid
  1  The workflow parsed but the plan is invalid (one or more invalid steps), or
     an --out write failure
  2  Missing/invalid arguments (including a missing --out value), or an
     unreadable/malformed workflow file (including a missing or unsupported
     schemaVersion, or no steps)

Examples:
  portier workflow plan --file workflow.json
  portier --json workflow plan --file workflow.json
  portier workflow plan --file workflow.json --out plan.json
`

// RunWorkflow dispatches the `portier workflow <subcommand>` commands. Every
// subcommand is fully offline (workflow planning never contacts the runtime), so
// no connection flags are threaded in.
func RunWorkflow(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, workflowHelp)
		return 2
	}
	switch args[0] {
	case "plan":
		return RunWorkflowPlan(jsonOutput, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, workflowHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown workflow subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, workflowHelp)
		return 2
	}
}

// RunWorkflowPlan reads a local workflow file, validates its schema and step
// references, and prints a deterministic plan. It is fully offline and dry-run:
// it does not execute any step, never contacts the runtime, never reads the files
// a step refers to, and never mutates any file except the requested --out file.
// Exit codes: 0 = plan valid, 1 = plan invalid (or a JSON-encode / --out write
// failure), 2 = usage error (including a missing --out value) or an
// unreadable/malformed workflow file (including a missing/unsupported
// schemaVersion or no steps).
func RunWorkflowPlan(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagFile := fs.String("file", "", "path to the workflow JSON file")
	flagOut := fs.String("out", "", "also write the JSON plan to this file")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, workflowPlanHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, workflowPlanHelp)
		return 2
	}
	if *flagFile == "" {
		fmt.Fprintln(stderr, "Error: --file is required")
		fmt.Fprint(stderr, workflowPlanHelp)
		return 2
	}

	data, err := os.ReadFile(*flagFile)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading workflow %s: %v\n", *flagFile, err)
		return 2
	}
	file, parseErr := workflow.Parse(data)
	if parseErr != nil {
		fmt.Fprintf(stderr, "Error: invalid workflow %s: %v\n", *flagFile, parseErr)
		return 2
	}

	plan := workflow.BuildPlan(file)
	return workflow.Emit(plan, workflow.EmitOptions{JSON: jsonOutput, OutPath: *flagOut}, stdout, stderr)
}
