package commands

import (
	"flag"
	"fmt"
	"io"
	"os"

	"portier/replay/sources/core"
)

// runCompare handles `replay compare --left <file-or-dir> --right <file-or-dir> [--out <file>]`.
func runCompare(jsonOut bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	left := fs.String("left", "", "path to the left workflow artifact (run/plan report, history export, or report bundle directory)")
	right := fs.String("right", "", "path to the right workflow artifact (run/plan report, history export, or report bundle directory)")
	out := fs.String("out", "", "optional file to write the comparison JSON to")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	if *left == "" {
		fmt.Fprintln(stderr, "Error: --left is required")
		return 2
	}
	if *right == "" {
		fmt.Fprintln(stderr, "Error: --right is required")
		return 2
	}

	comparison, err := core.CompareInputs(*left, *right)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}

	data, err := core.MarshalCompare(comparison)
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
		core.RenderCompareHuman(stdout, comparison)
	}

	if *out != "" {
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "Error: writing %s: %v\n", *out, err)
			return 1
		}
		if !jsonOut {
			fmt.Fprintf(stdout, "Replay compare written to %s\n", *out)
		}
	}

	return 0
}
