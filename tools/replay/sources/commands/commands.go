// Package commands implements the replay tool's CLI dispatch, argument parsing,
// and command runners. It depends on the core domain package for detection and
// the replay plan/analysis models, builders, and renderers — domain logic never
// depends on this package, and neither package depends on tools/cli.
package commands

import (
	"flag"
	"fmt"
	"io"
	"os"

	"portier/replay/sources/core"
)

// Run is the testable entry point. Exit codes: 0 success; 1 an output-write or
// JSON-encode failure; 2 a usage error or an unreadable/malformed/unsupported
// input artifact.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // suppress default usage output; we print our own

	flagJSON := fs.Bool("json", false, "output as machine-readable JSON")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printHelp(stdout)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		printHelp(stderr)
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		printHelp(stdout)
		return 0
	}

	switch remaining[0] {
	case "help":
		printHelp(stdout)
		return 0
	case "plan":
		return runPlan(*flagJSON, remaining[1:], stdout, stderr)
	case "analyze":
		return runAnalyze(*flagJSON, remaining[1:], stdout, stderr)
	case "timeline":
		return runTimeline(*flagJSON, remaining[1:], stdout, stderr)
	case "compare":
		return runCompare(*flagJSON, remaining[1:], stdout, stderr)
	case "explain":
		return runExplain(*flagJSON, remaining[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown command %q\n\n", remaining[0])
		printHelp(stderr)
		return 2
	}
}

// runPlan handles `replay plan --from <file-or-dir> [--out <file>]`.
func runPlan(jsonOut bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	from := fs.String("from", "", "path to a workflow artifact (run/plan report, history export, or report bundle directory)")
	out := fs.String("out", "", "optional file to write the replay plan JSON to")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	if *from == "" {
		fmt.Fprintln(stderr, "Error: --from is required")
		return 2
	}

	in, err := core.DetectInput(*from)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}

	plan := core.BuildPlan(in)

	data, err := core.MarshalPlan(plan)
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
		core.RenderHuman(stdout, plan)
	}

	if *out != "" {
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "Error: writing %s: %v\n", *out, err)
			return 1
		}
		if !jsonOut {
			fmt.Fprintf(stdout, "Replay plan written to %s\n", *out)
		}
	}

	return 0
}

// runAnalyze handles `replay analyze --from <file-or-dir> [--out <file>]`.
func runAnalyze(jsonOut bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	from := fs.String("from", "", "path to a workflow artifact (run/plan report, history export, or report bundle directory)")
	out := fs.String("out", "", "optional file to write the analysis JSON to")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	if *from == "" {
		fmt.Fprintln(stderr, "Error: --from is required")
		return 2
	}

	analysis, err := core.AnalyzeInput(*from)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}

	data, err := core.MarshalAnalysis(analysis)
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
		core.RenderAnalysisHuman(stdout, analysis)
	}

	if *out != "" {
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "Error: writing %s: %v\n", *out, err)
			return 1
		}
		if !jsonOut {
			fmt.Fprintf(stdout, "Replay analysis written to %s\n", *out)
		}
	}

	return 0
}

// printHelp writes the tool's usage text.
func printHelp(w io.Writer) {
	fmt.Fprint(w, `replay - offline analysis for saved Portier workflow artifacts

Usage:
  replay [--json] plan     --from <file-or-dir> [--out <file>]
  replay [--json] analyze  --from <file-or-dir> [--out <file>]
  replay [--json] timeline --from <file-or-dir> [--out <file>]
  replay [--json] compare  --left <file-or-dir> --right <file-or-dir> [--out <file>]
  replay [--json] explain  --from <file-or-dir> [--out <file>]
  replay [--json] explain  --code <code>
  replay [--json] explain  --list

Commands:
  plan      Report what offline replay/analysis a saved artifact can support
  analyze   Produce deterministic offline analysis from a saved artifact
  timeline  Reconstruct a deterministic ordered timeline from a saved artifact
  compare   Compare two saved artifacts offline and report what changed
  explain   Explain the emitted codes found in a saved artifact (or a code/the registry)
  help      Show this help

Flags:
  --json            Output as machine-readable JSON
  --from <path>     A workflow run/plan report, history export, or report bundle dir
  --left <path>     The left artifact for compare
  --right <path>    The right artifact for compare
  --code <code>     Explain a single code (explain)
  --list            List every known code (explain)
  --out <file>      Write the JSON output to a file

Exit codes:
  0  Output produced (including no codes / no differences)
  1  Output-write or JSON-encode failure
  2  Usage error, or unreadable / malformed / unsupported input
     (there is no exit 3 — replay never contacts a runtime)

This tool is offline and read-only. It never executes workflows, contacts the
runtime, reads referenced config/policy/baseline/report files, mutates inputs, or
uploads anything.
`)
}
