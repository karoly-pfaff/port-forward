package commands

import (
	"flag"
	"fmt"
	"io"
	"os"

	"portier/replay/sources/core"
)

// runExplain handles `replay explain` in one of three mutually-exclusive modes:
// `--from <file-or-dir>` (explain an artifact's codes), `--code <code>` (explain one
// code), or `--list` (list every known code). All are offline.
func runExplain(jsonOut bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	from := fs.String("from", "", "path to a workflow artifact or replay output to explain")
	code := fs.String("code", "", "explain a single code")
	list := fs.Bool("list", false, "list every known code")
	out := fs.String("out", "", "optional file to write the explanations JSON to")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}

	modes := 0
	if *from != "" {
		modes++
	}
	if *code != "" {
		modes++
	}
	if *list {
		modes++
	}
	switch {
	case modes == 0:
		fmt.Fprintln(stderr, "Error: one of --from, --code, or --list is required")
		return 2
	case modes > 1:
		fmt.Fprintln(stderr, "Error: specify only one of --from, --code, or --list")
		return 2
	}

	var report core.Explain
	switch {
	case *list:
		report = core.ExplainRegistry()
	case *code != "":
		report = core.ExplainCode(*code)
	default:
		r, err := core.ExplainInput(*from)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 2
		}
		report = r
	}

	data, err := core.MarshalExplain(report)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	if jsonOut {
		if _, err := stdout.Write(data); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
	} else {
		core.RenderExplainHuman(stdout, report)
	}

	if *out != "" {
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "Error: writing %s: %v\n", *out, err)
			return 1
		}
		if !jsonOut {
			fmt.Fprintf(stdout, "Replay explanations written to %s\n", *out)
		}
	}

	return 0
}
