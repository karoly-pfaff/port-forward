package core

// Rendering for the replay comparison: deterministic human-readable text and a
// single JSON marshaller (used for both stdout under --json and the --out file so
// the two are byte-identical).

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// MarshalCompare renders the comparison as deterministic, indented JSON with a
// trailing newline — the single source of JSON bytes so `--json` stdout and the
// `--out` file are byte-identical.
func MarshalCompare(c Compare) ([]byte, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}
	return append(data, '\n'), nil
}

// RenderCompareHuman writes the human-readable comparison: the two sides, the
// detected changes (with added/removed sub-bullets), and the insights.
func RenderCompareHuman(w io.Writer, c Compare) {
	fmt.Fprintln(w, "Portier Replay Compare")
	fmt.Fprintln(w)

	writeCompareSide(w, "Left", c.Left)
	fmt.Fprintln(w)
	writeCompareSide(w, "Right", c.Right)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Changes:")
	if len(c.Changes) == 0 {
		fmt.Fprintln(w, "- None.")
	} else {
		for _, ch := range c.Changes {
			fmt.Fprintf(w, "- %s\n", ch.Message)
			for _, a := range ch.Added {
				fmt.Fprintf(w, "  - %s\n", a)
			}
			for _, rm := range ch.Removed {
				fmt.Fprintf(w, "  - %s\n", rm)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Insights:")
	if len(c.Insights) == 0 {
		fmt.Fprintln(w, "- none")
	} else {
		for _, s := range c.Insights {
			fmt.Fprintf(w, "- %s\n", s)
		}
	}
}

func writeCompareSide(w io.Writer, label string, side CompareSide) {
	fmt.Fprintf(w, "%s:\n", label)
	fmt.Fprintf(w, "- Source: %s\n", side.Source)
	if strings.TrimSpace(side.Workflow) != "" {
		fmt.Fprintf(w, "- Workflow: %s\n", side.Workflow)
	}
	if strings.TrimSpace(side.Result) != "" {
		fmt.Fprintf(w, "- Result: %s\n", side.Result)
	}
}
