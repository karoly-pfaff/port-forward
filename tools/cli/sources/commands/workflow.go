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
  plan --file <workflow.json>     Validate a workflow file and print its plan
  runbook --file <workflow.json>  Preview the CLI commands a valid workflow maps to
  template <name> | --list        Print a built-in workflow template, or list them
  help                            Show this help message

Run 'portier workflow plan --help', 'portier workflow runbook --help', or
'portier workflow template --help' for options.
`

const workflowRunbookHelp = `Usage: portier workflow runbook --file <workflow.json> [--out <file>]

Preview the ordered list of Portier CLI commands a VALID workflow maps to — the
manual commands you would run to carry the workflow out. Fully offline and a
preview only: it parses and validates the workflow, then maps each step to a CLI
command. It does NOT execute any command, never contacts the runtime, never reads
the files a step refers to (or inspects their contents), and never mutates any
file except the requested --out file.

If the workflow is invalid, the plan (with the validation errors) is printed and
the command exits 1 — no runbook and no --out file are produced. Fix the workflow
(see 'portier workflow plan --explain') and re-run.

Options:
  --file <file>   Path to the workflow JSON file (required).
  --out <file>    Also write the JSON runbook to <file> (same shape as --json).
                  With --json the JSON also prints to stdout and is byte-identical
                  to the file.

Each step maps to a command preview:
  policy.check (config)     portier policy check --config <f> --policy <f>
  policy.check (runtime)    portier policy check --runtime --policy <f>
  policy.review             portier policy review --current <f> --candidate <f> --policy <f>
  policy.baseline.compare   portier policy baseline compare --baseline <f> --report <f>
    (with reportFrom)       ... --report <report-from:STEP-ID>  (a placeholder to replace)

The JSON runbook gives each step a canonical argv "command" array plus a
copy/paste "display" string; the argv form is canonical (Portier never
shell-executes the display string). A reportFrom step uses an explicit
<report-from:step-id> placeholder and a note — it is NOT a real file path.

Exit codes:
  0  The workflow is valid and a runbook was produced
  1  The workflow parsed but the plan is invalid, or an --out write failure
  2  Missing/invalid arguments (including a missing --file or --out value), or an
     unreadable/malformed workflow file (including a missing/unsupported
     schemaVersion, or no steps)

Workflow runbook generation never contacts the runtime, so there is no
connection-failure (3) exit code.

Examples:
  portier workflow runbook --file workflow.json
  portier --json workflow runbook --file workflow.json
  portier workflow runbook --file workflow.json --out runbook.json
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
	case "runbook":
		return RunWorkflowRunbook(jsonOutput, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, workflowHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown workflow subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, workflowHelp)
		return 2
	}
}

// RunWorkflowRunbook reads a local workflow file, validates it, and previews the
// ordered Portier CLI commands the workflow maps to. It is fully offline and a
// PREVIEW only: it never executes a command, contacts the runtime, reads the
// files a step refers to, or mutates any file except the requested --out file.
// If the workflow is invalid it prints the plan (the validation errors) and
// exits 1 — no runbook and no --out file are produced, exactly like
// `workflow plan` on an invalid workflow. Exit codes: 0 = runbook produced, 1 =
// plan invalid (or a JSON-encode / --out write failure), 2 = usage error
// (including a missing --file/--out value) or an unreadable/malformed workflow
// file (including a missing/unsupported schemaVersion or no steps).
func RunWorkflowRunbook(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow runbook", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagFile := fs.String("file", "", "path to the workflow JSON file")
	flagOut := fs.String("out", "", "also write the JSON runbook to this file")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, workflowRunbookHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, workflowRunbookHelp)
		return 2
	}
	if *flagFile == "" {
		fmt.Fprintln(stderr, "Error: --file is required")
		fmt.Fprint(stderr, workflowRunbookHelp)
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

	// A runbook is produced only from a VALID plan. If the plan is invalid, print
	// the plan (the validation errors) and exit 1 — no runbook, and do NOT write
	// --out (there is no runbook to write).
	plan := workflow.BuildPlan(file)
	if workflow.PlanExitCode(plan) != 0 {
		return workflow.Emit(plan, workflow.EmitOptions{JSON: jsonOutput}, stdout, stderr)
	}

	runbook := workflow.BuildRunbook(file)
	return workflow.EmitRunbook(runbook, jsonOutput, *flagOut, stdout, stderr)
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
