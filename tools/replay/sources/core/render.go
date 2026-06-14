package core

// Rendering for the replay plan: deterministic human-readable text and a single
// JSON marshaller used for both stdout (--json) and the --out file so the two are
// byte-identical.

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// MarshalPlan renders the plan as deterministic, indented JSON with a trailing
// newline. It is the single source of JSON bytes so `--json` stdout and the
// `--out` file are byte-identical.
func MarshalPlan(p Plan) ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}
	return append(data, '\n'), nil
}

// RenderHuman writes the human-readable replay plan. Available and unavailable
// analyses are listed separately (by their messages, in plan order), followed by
// the available/unavailable counts. Workflow/Result lines are omitted when absent.
func RenderHuman(w io.Writer, p Plan) {
	fmt.Fprintln(w, "Portier Replay Plan")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Source: %s\n", p.Source)
	if strings.TrimSpace(p.Workflow) != "" {
		fmt.Fprintf(w, "Workflow: %s\n", p.Workflow)
	}
	if strings.TrimSpace(p.Result) != "" {
		fmt.Fprintf(w, "Result: %s\n", p.Result)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Available:")
	writeItems(w, p, ItemAvailable)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Unavailable:")
	writeItems(w, p, ItemUnavailable)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "- %d available\n", p.Summary.Available)
	fmt.Fprintf(w, "- %d unavailable\n", p.Summary.Unavailable)
}

// writeItems writes the messages of every item with the given status, or "- none"
// when there are none.
func writeItems(w io.Writer, p Plan, status string) {
	any := false
	for _, it := range p.Items {
		if it.Status == status {
			fmt.Fprintf(w, "- %s\n", it.Message)
			any = true
		}
	}
	if !any {
		fmt.Fprintln(w, "- none")
	}
}
