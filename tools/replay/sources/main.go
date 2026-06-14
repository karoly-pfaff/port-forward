// Command portier-replay is a standalone, offline analysis utility for existing
// Portier workflow artifacts. It reads workflow run/plan JSON reports, workflow
// history exports, and workflow support-report bundles, and reports what offline
// replay/analysis each saved artifact can support.
//
// It is strictly offline and read-only. It NEVER executes workflows, contacts the
// Portier runtime, reads the config/policy/baseline/report files an artifact refers
// to, mutates runtime/config/history/report files, applies/imports configs,
// enforces policy, schedules jobs, runs shell commands, uploads anything, or
// collects logs/environment/process data.
//
// It is a separate tool beside the Portier CLI — not a `portier` subcommand.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"portier/replay/sources/replay"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point. Exit codes: 0 success; 1 an output-write or
// JSON-encode failure; 2 a usage error or an unreadable/malformed/unsupported
// input artifact.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("portier-replay", flag.ContinueOnError)
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
	default:
		fmt.Fprintf(stderr, "Unknown command %q\n\n", remaining[0])
		printHelp(stderr)
		return 2
	}
}

// runPlan handles `portier-replay plan --from <file-or-dir> [--out <file>]`.
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

	in, err := replay.DetectInput(*from)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}

	plan := replay.BuildPlan(in)

	data, err := replay.MarshalPlan(plan)
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
		replay.RenderHuman(stdout, plan)
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

// printHelp writes the tool's usage text.
func printHelp(w io.Writer) {
	fmt.Fprint(w, `portier-replay - offline analysis for saved Portier workflow artifacts

Usage:
  portier-replay [--json] plan --from <file-or-dir> [--out <file>]

Commands:
  plan      Report what offline replay/analysis a saved artifact can support
  help      Show this help

Flags:
  --json           Output as machine-readable JSON
  --from <path>    A workflow run/plan report, history export, or report bundle dir
  --out <file>     Write the replay plan JSON to a file

This tool is offline and read-only. It never executes workflows, contacts the
runtime, reads referenced config/policy/baseline/report files, mutates inputs, or
uploads anything.
`)
}
