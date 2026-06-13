package commands

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"portier/cli/sources/output"
	"portier/cli/sources/workflow"
)

const workflowHelp = `Usage: portier workflow <subcommand> [options]

Plan and validate a local workflow — an ordered sequence of existing safe Portier
operations described in a small JSON file — or generate a starter workflow from a
built-in template. Fully offline and dry-run: it reads/writes local files only,
validates schema and step references, and never executes a step, contacts the
runtime, applies or imports configs, enforces a policy, or mutates any file
(except the requested --out file).

Subcommands:
  plan --file <workflow.json>   Validate a workflow file and print its plan
  template <name> | --list      Print a built-in workflow template, or list them
  help                          Show this help message

Run 'portier workflow plan --help' or 'portier workflow template --help' for options.
`

const workflowTemplateHelp = `Usage: portier workflow template <name> [--out <file>]
       portier workflow template --list

Print a built-in workflow template, or list the available templates. Fully
offline: never contacts the runtime, never executes a step, and never modifies
any file except the requested --out file. A rendered template is a complete
workflow file (schemaVersion 1) that can be passed straight to
'portier workflow plan --file <file>'.

Options:
  --list            List the available templates (name, title, description).
  --out <file>      Write the template's workflow JSON to <file> (bare workflow
                    only, directly usable by 'workflow plan'). Without --out, the
                    workflow JSON is printed to stdout.

Output:
  workflow template <name>           Prints the template's workflow JSON to stdout.
  workflow template <name> --json    Prints a metadata wrapper {name,title,description,workflow}.
  workflow template <name> --out f   Writes the workflow JSON to f (human mode confirms on stdout).
  workflow template --list           Prints a compact human list of the templates.
  workflow template --list --json    Prints {"templates":[{name,title,description}]}.

Built-in templates: policy-baseline-check, policy-check-local,
policy-check-runtime, policy-review.

Exit codes:
  0  Success
  1  Output file write failure (an operation failure)
  2  Missing/invalid arguments (including a missing --out value) or an unknown template

Examples:
  portier workflow template --list
  portier workflow template policy-check-local
  portier workflow template policy-check-local --out workflow.json
  portier workflow plan --file workflow.json
`

const workflowPlanHelp = `Usage: portier workflow plan --file <workflow.json> [--out <file>]

Read a local workflow file, validate its schema and step references, and print a
deterministic plan describing each step. Fully offline and dry-run: it does NOT
execute any step, never contacts the runtime, never reads the files a step refers
to, never enforces a policy, and never mutates any file except the requested
--out file.

Options:
  --file <file>   Path to the workflow JSON file (required).
  --explain       Show an explanation (meaning + next action) for each INVALID
                  step's validation code (inline in human output; an additive
                  explanations map in --json). Does not change the steps, summary,
                  result, or exit code. Use 'portier explain <code>' for any code.
  --out <file>    Also write the JSON plan to <file> (same shape as --json,
                  including the additive explanations map under --explain). With
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
  portier workflow plan --file workflow.json --explain
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
	case "template":
		return RunWorkflowTemplate(jsonOutput, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, workflowHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown workflow subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, workflowHelp)
		return 2
	}
}

// RunWorkflowTemplate prints a built-in workflow template (or lists them). It is
// fully offline: it never contacts the runtime, never executes a step, and never
// modifies any file except the requested --out file. A rendered template is a
// complete workflow file in the schema `workflow plan` accepts. Exit codes: 0 =
// success, 1 = output-file write failure, 2 = usage error (including a missing
// --out value) or an unknown template.
func RunWorkflowTemplate(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow template", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagList := fs.Bool("list", false, "list the available templates")
	flagOut := fs.String("out", "", "write the template workflow JSON to this file")

	// Parse flags that may appear before AND after the positional template name
	// (e.g. `template policy-check-local --out f`); Go's flag package otherwise
	// stops at the first non-flag argument.
	var positional []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			if err == flag.ErrHelp {
				fmt.Fprint(stdout, workflowTemplateHelp)
				return 0
			}
			fmt.Fprintf(stderr, "Error: %v\n\n", err)
			fmt.Fprint(stderr, workflowTemplateHelp)
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
			fmt.Fprint(stderr, workflowTemplateHelp)
			return 2
		}
		if *flagOut != "" {
			fmt.Fprintln(stderr, "Error: --out cannot be combined with --list")
			fmt.Fprint(stderr, workflowTemplateHelp)
			return 2
		}
		if err := workflow.PrintTemplateList(jsonOutput, stdout); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if len(positional) == 0 {
		fmt.Fprintln(stderr, "Error: a template name is required (or use --list)")
		fmt.Fprint(stderr, workflowTemplateHelp)
		return 2
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "Error: too many arguments (expected one template name, got %d)\n\n", len(positional))
		fmt.Fprint(stderr, workflowTemplateHelp)
		return 2
	}

	name := positional[0]
	tmpl, ok := workflow.FindTemplate(name)
	if !ok {
		fmt.Fprintf(stderr, "Error: unknown template %q. Available: %s\n", name, strings.Join(workflow.TemplateNames(), ", "))
		return 2
	}

	// stdout carries the metadata+workflow wrapper under --json, the bare workflow
	// JSON in human mode, or (human mode + --out) just the write confirmation —
	// the --out file always gets the bare workflow JSON so it is directly usable
	// by `workflow plan --file`.
	if jsonOutput {
		if err := output.PrintJSON(stdout, tmpl.DetailValue()); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else if *flagOut == "" {
		if err := output.PrintJSON(stdout, tmpl.WorkflowValue()); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	}

	if *flagOut != "" {
		if err := output.WritePrettyJSON(*flagOut, tmpl.WorkflowValue()); err != nil {
			fmt.Fprintf(stderr, "Error writing %s: %v\n", *flagOut, err)
			return 1
		}
		if !jsonOutput {
			fmt.Fprintf(stdout, "Workflow written to %s\n", *flagOut)
		}
	}

	return 0
}

// RunWorkflowPlan reads a local workflow file, validates its schema and step
// references, and prints a deterministic plan. It is fully offline and dry-run:
// it does not execute any step, never contacts the runtime, never reads the files
// a step refers to, and never mutates any file except the requested --out file.
// With --explain it adds inline explanations (human) / an additive explanations
// map (JSON) for the plan's invalid step codes, without changing the steps,
// summary, result, or exit code. Exit codes: 0 = plan valid, 1 = plan invalid (or
// a JSON-encode / --out write failure), 2 = usage error (including a missing
// --out value) or an unreadable/malformed workflow file (including a
// missing/unsupported schemaVersion or no steps).
func RunWorkflowPlan(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagFile := fs.String("file", "", "path to the workflow JSON file")
	flagExplain := fs.Bool("explain", false, "explain each invalid step's validation code")
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
	return workflow.Emit(plan, workflow.EmitOptions{JSON: jsonOutput, OutPath: *flagOut, Explain: *flagExplain}, stdout, stderr)
}
