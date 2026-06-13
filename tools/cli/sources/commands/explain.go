package commands

import (
	"flag"
	"fmt"
	"io"

	"portier/cli/sources/doctor"
	"portier/cli/sources/output"
)

const explainHelp = `Usage: portier explain <code>
       portier explain --list

Explain a stable Portier doctor/check code: what it means and what to do next.
Fully offline — does not contact the runtime and changes nothing.

Options:
  --list   List all known doctor/check codes (with their titles).

Output:
  Human explanation by default; --json emits the explanation, or with --list the
  full list of explanations, as JSON.

Exit codes:
  0  Explanation printed (or list printed)
  2  Unknown code, missing code argument, or usage error

Examples:
  portier explain config.duplicate_binding
  portier explain rules.health_error
  portier --json explain runtime.unreachable
  portier explain --list
`

// RunExplain runs the `portier explain` command. It is fully offline: it looks
// up a static explanation for a stable doctor/check code (or lists all known
// codes with --list). Exit codes: 0 success, 2 unknown/missing code or usage.
func RunExplain(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagList := fs.Bool("list", false, "list all known doctor/check codes")

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
	exp, ok := doctor.ExplanationFor(code)
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
	codes := doctor.SortedExplanationCodes()

	if jsonOutput {
		if err := output.PrintJSON(stdout, doctor.SortedExplanations()); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintln(stdout, "Known doctor/check codes:")
	for _, c := range codes {
		exp, _ := doctor.ExplanationFor(c)
		fmt.Fprintf(stdout, "  %-26s %s\n", c, exp.Title)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Run 'portier explain <code>' for details on one code.")
	return 0
}

// printExplanationHuman renders one explanation in the CLI's plain style.
func printExplanationHuman(exp doctor.Explanation, w io.Writer) {
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
