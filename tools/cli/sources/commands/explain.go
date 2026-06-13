package commands

import (
	"flag"
	"fmt"
	"io"

	"portier/cli/sources/doctor"
	"portier/cli/sources/explain"
	"portier/cli/sources/output"
	"portier/cli/sources/policy"
	"portier/cli/sources/workflow"
)

const explainHelp = `Usage: portier explain <code>
       portier explain --list

Explain a stable Portier doctor, policy, or workflow code: what it means and what
to do next. Fully offline — does not contact the runtime and changes nothing.

Options:
  --list   List all known codes (doctor/check, policy, and workflow) with titles.

Output:
  Human explanation by default; --json emits the explanation, or with --list the
  full list of explanations, as JSON.

Exit codes:
  0  Explanation printed (or list printed)
  2  Unknown code, missing code argument, or usage error

Examples:
  portier explain config.duplicate_binding
  portier explain rules.health_error
  portier explain policy.lan_exposure_forbidden
  portier explain workflow.step.unknown_report_from
  portier explain workflow.run.dependency_failed
  portier --json explain runtime.unreachable
  portier explain --list
`

// allExplanations returns the merged explanation registry across all domains
// (doctor/check codes + policy finding codes + workflow validation codes). Each
// domain owns its own registry; the explain command composes them for unified
// lookup and listing.
func allExplanations() map[string]explain.Explanation {
	return explain.Merge(doctor.Explanations(), policy.Explanations(), workflow.Explanations())
}

// RunExplain runs the `portier explain` command. It is fully offline: it looks
// up a static explanation for a stable doctor/check, policy, or workflow code (or
// lists all known codes with --list). Exit codes: 0 success, 2 unknown/missing
// code or usage.
func RunExplain(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagList := fs.Bool("list", false, "list all known codes")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, explainHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, explainHelp)
		return 2
	}

	if *flagList {
		return runExplainList(jsonOutput, stdout, stderr)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "Error: explain requires a code (or --list)")
		fmt.Fprint(stderr, explainHelp)
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "Error: explain takes a single code (got %d arguments)\n\n", fs.NArg())
		fmt.Fprint(stderr, explainHelp)
		return 2
	}

	code := fs.Arg(0)
	exp, ok := explain.For(allExplanations(), code)
	if !ok {
		fmt.Fprintf(stderr, "Error: unknown code %q\n", code)
		fmt.Fprintln(stderr, "Run 'portier explain --list' to see all known codes.")
		return 2
	}

	if jsonOutput {
		if err := output.PrintJSON(stdout, exp); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printExplanationHuman(exp, stdout)
	return 0
}

// runExplainList prints every known code, sorted, in human or JSON form.
func runExplainList(jsonOutput bool, stdout, stderr io.Writer) int {
	reg := allExplanations()

	if jsonOutput {
		if err := output.PrintJSON(stdout, explain.Sorted(reg)); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintln(stdout, "Known codes:")
	for _, c := range explain.SortedCodes(reg) {
		fmt.Fprintf(stdout, "  %-34s %s\n", c, reg[c].Title)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Run 'portier explain <code>' for details on one code.")
	return 0
}

// printExplanationHuman renders one explanation in the CLI's plain style.
func printExplanationHuman(exp explain.Explanation, w io.Writer) {
	fmt.Fprintln(w, exp.Code)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Meaning:")
	fmt.Fprintln(w, exp.Meaning)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "What to do:")
	fmt.Fprintln(w, exp.Action)
	if len(exp.Related) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Related:")
		for _, r := range exp.Related {
			fmt.Fprintf(w, "- %s\n", r)
		}
	}
}
