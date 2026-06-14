package commands

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"portier/cli/sources/client"
	"portier/cli/sources/config"
	"portier/cli/sources/output"
	"portier/cli/sources/workflow"
)

const workflowHelp = `Usage: portier workflow <subcommand> [options]

Plan, validate, run, preview, report on, template, or inspect the run history of a
local workflow — an ordered sequence of existing safe Portier operations described
in a small JSON file. Read-only and dry-run: it never applies/imports configs,
enforces a policy, runs a shell command, schedules anything, or mutates any file
(except the requested --out file/directory or the opt-in local history file). Only
'workflow run' executes the (read-only) steps; it is the only subcommand that may
contact the runtime, and only for a policy.check runtime step.

Subcommands:
  plan --file <workflow.json>     Validate a workflow file and print its plan
  run --file <workflow.json>      Execute a valid workflow's read-only steps
  runbook --file <workflow.json>  Preview the CLI commands a valid workflow maps to
  report --from <report.json>     Package an existing plan/run report into a bundle
  template <name> | --list        Print a built-in workflow template, or list them
  history <list|show|export|clear>  Inspect/export the opt-in workflow run history
  help                            Show this help message

Run 'portier workflow plan --help', 'portier workflow run --help',
'portier workflow runbook --help', 'portier workflow report --help',
'portier workflow template --help', or 'portier workflow history --help' for options.
`

const workflowReportHelp = `Usage: portier workflow report --from <report.json> --out <directory>

Package an EXISTING workflow report into a small, local diagnostic bundle for
manual review or AI handoff. The input is a JSON report produced by
'portier workflow plan --json --out <file>' or
'portier workflow run --json --out <file>'.

This is a packaging step only — it is fully offline and read-only. It NEVER
executes a workflow, runs a shell command, contacts the runtime, reads the
config/policy/baseline/report files a step refers to, collects logs/environment/
process data, mutates the input report, or uploads anything. It only parses the
provided report and re-derives explanation metadata from the canonical registry.

Options:
  --from <file>   Path to an existing workflow plan/run JSON report (required).
  --out <dir>     Output directory for the bundle (required). Created if missing;
                  an existing directory must be empty (a non-empty directory is
                  refused, matching 'portier support-bundle').

Bundle contents:
  manifest.json        Bundle metadata (schema version, time, source, workflow, result, files).
  summary.txt          Human-readable summary (source, workflow, result, steps, explained codes, safety).
  report.json          The normalized report (step id/type/status/message + the explainable codes per step).
  explanations.json    Canonical explanations for the emitted codes ({} when none).

Explanations are re-derived from the report: workflow.step.* codes for an invalid
plan, workflow.run.* codes for failed/skipped run steps (a skipped step →
workflow.run.dependency_failed), and policy.* codes for the findings embedded in
failed run steps; any codes carried by a --explain report's explanations map are
also included. The bundle never includes raw configs, secrets, or runtime data.

Exit codes:
  0  Bundle written
  1  Output directory create/write failure
  2  Missing/invalid arguments (including a missing --from/--out value), or an
     unreadable/malformed/unsupported input report

Examples:
  portier workflow run --file workflow.json --json --out run.json
  portier workflow report --from run.json --out ./workflow-report
  portier --json workflow report --from run.json --out ./workflow-report
`

const workflowRunHelp = `Usage: portier workflow run --file <workflow.json> [--out <file>]

Execute a VALID workflow's steps in order and print a deterministic run report.
Execution is strictly READ-ONLY: it runs only the existing safe step types by
calling the policy evaluator/review/baseline-compare directly. It NEVER runs a
shell command (or the runbook display text), never applies/imports configs, never
enforces a policy, never schedules anything, and never mutates the
runtime/config/policy/baseline/report files. The only runtime contact is a
read-only runtime-config read for a policy.check runtime step.

If the workflow is invalid, the plan (with the validation errors) is printed and
the command exits 1 — no step runs and no --out file is produced. Referenced
files (config/policy/baseline/report) are read HERE (during the run), unlike
'workflow plan' / 'workflow runbook' which never open them.

Options:
  --file <file>   Path to the workflow JSON file (required).
  --explain       Show an explanation (meaning + next action) for each FAILED or
                  SKIPPED step — its policy finding codes and/or a workflow.run.*
                  failure code (inline in human output; an additive explanations
                  map in --json). Passed steps are not explained. Does not change
                  the steps, summary, result, or exit code. Use
                  'portier explain <code>' for any code.
  --out <file>    Also write the JSON run report to <file> (same shape as --json,
                  including the additive explanations map under --explain). With
                  --json the JSON also prints to stdout and is byte-identical to
                  the file.
  --record-history  After the run completes, record a COMPACT entry in the local,
                  opt-in workflow run history (run id, time, workflow name, result,
                  summary counts, compact step metadata, and emitted codes). It
                  never records raw configs, policies, full reports, secrets, logs,
                  environment, process data, runtime URLs, or tokens. History is
                  bounded to the most recent 100 runs. Inspect it with
                  'portier workflow history list|show|clear'. Without this flag,
                  nothing is recorded.

Step execution (read-only):
  policy.check              Evaluate a local config (offline) or the live runtime
                            config (read-only) against a policy.
  policy.review             Compare current vs candidate and evaluate the candidate
                            against a policy (offline).
  policy.baseline.compare   Compare a baseline against a policy report — a report
                            file, or the IN-MEMORY report produced by an earlier
                            step (reportFrom). A reportFrom step whose dependency
                            produced no report is SKIPPED (which fails the run).

Step status: a policy/review pass → passed; a violation → failed; a baseline
compare with new findings → failed. A failed or skipped step fails the run.

Exit codes:
  0  All executed steps passed (none skipped)
  1  One or more steps failed or were skipped; the workflow plan was invalid; an
     --out write failure; or, with --record-history, a history write failure when
     the run itself passed
  2  Missing/invalid arguments (including a missing --file/--out value), or an
     unreadable/malformed workflow file (including a missing/unsupported
     schemaVersion, or no steps)
  3  A policy.check runtime step could not reach the runtime (matching
     'policy check --runtime')

With --record-history: a history write failure is reported as a warning. If the
run itself passed, the exit code becomes 1; if the run already failed, its exit
code (1 or 3) is kept and the warning is added.

Examples:
  portier workflow run --file workflow.json
  portier --json workflow run --file workflow.json
  portier workflow run --file workflow.json --explain
  portier workflow run --file workflow.json --out report.json
  portier workflow run --file workflow.json --record-history
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

// RunWorkflow dispatches the `portier workflow <subcommand>` commands. All
// subcommands are offline except `workflow run`, which (only for a policy.check
// runtime step) lazily resolves the runtime URL from conn inside the handler; the
// offline subcommands never touch conn.
func RunWorkflow(jsonOutput bool, conn ConnFlags, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, workflowHelp)
		return 2
	}
	switch args[0] {
	case "plan":
		return RunWorkflowPlan(jsonOutput, args[1:], stdout, stderr)
	case "run":
		return RunWorkflowRun(jsonOutput, conn, args[1:], stdout, stderr)
	case "template":
		return RunWorkflowTemplate(jsonOutput, args[1:], stdout, stderr)
	case "runbook":
		return RunWorkflowRunbook(jsonOutput, args[1:], stdout, stderr)
	case "report":
		return RunWorkflowReport(jsonOutput, args[1:], stdout, stderr)
	case "history":
		return RunWorkflowHistory(jsonOutput, args[1:], stdout, stderr)
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

// makeRuntimeRules returns the injected runtime-config fetcher for workflow
// execution. It is the ONLY runtime contact a workflow run makes, and only for a
// policy.check runtime step. The runtime config is read read-only via the
// existing config-export path; the result is memoized so multiple runtime steps
// fetch at most once. A connection failure is wrapped with
// workflow.ErrRuntimeUnreachable so execution can map it to exit 3 (matching
// `policy check --runtime`) without importing the client package.
func makeRuntimeRules(conn ConnFlags) func() ([]config.Rule, error) {
	var (
		cachedRules []config.Rule
		cachedErr   error
		done        bool
	)
	return func() ([]config.Rule, error) {
		if done {
			return cachedRules, cachedErr
		}
		done = true
		managementURL, err := ResolveURL(conn.URL, conn.Host, conn.Port)
		if err != nil {
			cachedErr = err
			return nil, cachedErr
		}
		cfg, expErr := client.New(managementURL).ExportConfig()
		if expErr != nil {
			var connErr *client.ConnectionError
			if errors.As(expErr, &connErr) {
				cachedErr = fmt.Errorf("%w: %v", workflow.ErrRuntimeUnreachable, connErr)
			} else {
				cachedErr = expErr
			}
			return nil, cachedErr
		}
		cachedRules = exportedRules(cfg)
		return cachedRules, nil
	}
}

// RunWorkflowRun reads a local workflow file, validates it, and executes its
// read-only steps in order, printing a deterministic run report. Execution never
// runs a shell command, never mutates any file, never applies/imports configs or
// enforces a policy, and contacts the runtime only (read-only) for a policy.check
// runtime step. With --explain it adds inline explanations (human) / an additive
// explanations map (JSON) for each failed/skipped step's codes, without changing
// the steps, summary, result, or exit code. If the workflow is invalid it prints
// the plan and exits 1 with no step run and no --out file (like
// `workflow plan`/`runbook`). Exit codes: 0 all
// steps passed; 1 a step failed/was skipped, the plan was invalid, or an --out
// write failure; 2 usage error (including a missing --file/--out value) or an
// unreadable/malformed workflow file; 3 a runtime step could not reach the runtime.
func RunWorkflowRun(jsonOutput bool, conn ConnFlags, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagFile := fs.String("file", "", "path to the workflow JSON file")
	flagExplain := fs.Bool("explain", false, "explain each failed/skipped step's codes")
	flagOut := fs.String("out", "", "also write the JSON run report to this file")
	flagRecord := fs.Bool("record-history", false, "record a compact entry in the local workflow run history")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, workflowRunHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, workflowRunHelp)
		return 2
	}
	if *flagFile == "" {
		fmt.Fprintln(stderr, "Error: --file is required")
		fmt.Fprint(stderr, workflowRunHelp)
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

	// A workflow runs only from a VALID plan. If invalid, print the plan (the
	// validation errors) and exit 1 — no step runs, no --out file is written. With
	// --explain the plan emitter explains the invalid step codes (reusing the
	// workflow plan explanations).
	plan := workflow.BuildPlan(file)
	if workflow.PlanExitCode(plan) != 0 {
		return workflow.Emit(plan, workflow.EmitOptions{JSON: jsonOutput, Explain: *flagExplain}, stdout, stderr)
	}

	deps := workflow.RunDeps{
		ReadFile:     os.ReadFile,
		RuntimeRules: makeRuntimeRules(conn),
	}
	run := workflow.Run(file, deps)
	exit := workflow.EmitRun(run, jsonOutput, *flagExplain, *flagOut, stdout, stderr)

	// Opt-in local history recording happens AFTER the run completes and only when
	// requested. A completed run (passed or failed) is recorded; an invalid plan
	// returned earlier, so invalid plans are never recorded. A history write
	// failure is reported as a warning and never hides the workflow result: if the
	// run passed (exit 0) it becomes 1 (the requested recording failed); if the run
	// already failed (1 or 3) that exit code is kept.
	if *flagRecord {
		if err := recordWorkflowHistory(run); err != nil {
			fmt.Fprintf(stderr, "Warning: failed to record workflow history: %v\n", err)
			if exit == 0 {
				exit = 1
			}
		}
	}
	return exit
}

// recordWorkflowHistory projects a completed run into a compact history entry and
// appends it to the local, bounded history store. The run id is built from the
// current time, the workflow name, and an injected short suffix.
func recordWorkflowHistory(run workflow.WorkflowRun) error {
	store, err := historyStore()
	if err != nil {
		return err
	}
	now := time.Now()
	id := workflow.NewRunID(now, run.Workflow, newHistorySuffix())
	return store.Append(workflow.ProjectRun(run, id, now))
}

// RunWorkflowReport packages an EXISTING workflow plan/run JSON report into a
// local diagnostic bundle directory. It is fully offline and read-only: it parses
// ONLY the provided report, and never executes a workflow, runs a shell command,
// contacts the runtime, reads the config/policy/baseline/report files a step
// refers to, collects logs/env/process data, mutates the input, or uploads
// anything. The bundle assembly lives in the `workflow` package (handlers-only
// here). Exit codes: 0 = bundle written, 1 = output directory create/write
// failure, 2 = usage error (including a missing --from/--out value) or an
// unreadable/malformed/unsupported input report.
func RunWorkflowReport(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagFrom := fs.String("from", "", "path to an existing workflow plan/run JSON report")
	flagOut := fs.String("out", "", "output directory for the report bundle")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, workflowReportHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, workflowReportHelp)
		return 2
	}
	if *flagFrom == "" {
		fmt.Fprintln(stderr, "Error: --from is required")
		fmt.Fprint(stderr, workflowReportHelp)
		return 2
	}
	if *flagOut == "" {
		fmt.Fprintln(stderr, "Error: --out is required")
		fmt.Fprint(stderr, workflowReportHelp)
		return 2
	}

	data, err := os.ReadFile(*flagFrom)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading report %s: %v\n", *flagFrom, err)
		return 2
	}
	report, parseErr := workflow.ParseSupportReport(data)
	if parseErr != nil {
		fmt.Fprintf(stderr, "Error: invalid report %s: %v\n", *flagFrom, parseErr)
		return 2
	}

	// Prepare the output directory (create if missing, refuse a non-empty
	// directory) — same convention as `portier support-bundle`.
	if err := prepareBundleDir(*flagOut); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	manifest, err := workflow.WriteSupportReport(*flagOut, report, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "Error writing report bundle: %v\n", err)
		return 1
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, manifest); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	total := len(manifest.Files) + 1 // + manifest.json
	fmt.Fprintf(stdout, "Workflow report written to %s\n", *flagOut)
	fmt.Fprintf(stdout, "  %d %s\n", total, output.PluralWord(total, "file", "files"))
	fmt.Fprintf(stdout, "  Source: %s\n", manifest.Source)
	fmt.Fprintf(stdout, "  Result: %s\n", manifest.Result)
	return 0
}
