package core

// Rendering for the replay timeline: deterministic human-readable text and a single
// JSON marshaller (used for both stdout under --json and the --out file so the two
// are byte-identical).

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// MarshalTimeline renders the timeline as deterministic, indented JSON with a
// trailing newline — the single source of JSON bytes so `--json` stdout and the
// `--out` file are byte-identical.
func MarshalTimeline(t Timeline) ([]byte, error) {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}
	return append(data, '\n'), nil
}

// RenderTimelineHuman writes the human-readable timeline: an ordered list of events
// (synthetic lifecycle markers show their message; saved events show type, exit
// code, history fields, and codes) followed by an event summary.
func RenderTimelineHuman(w io.Writer, t Timeline) {
	fmt.Fprintln(w, "Portier Replay Timeline")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Source: %s\n", t.Source)
	if strings.TrimSpace(t.Workflow) != "" {
		fmt.Fprintf(w, "Workflow: %s\n", t.Workflow)
	}
	if strings.TrimSpace(t.Result) != "" {
		fmt.Fprintf(w, "Result: %s\n", t.Result)
	}
	fmt.Fprintln(w)

	for _, ev := range t.Events {
		header := fmt.Sprintf("%d. %s [%s]", ev.Order, ev.ID, ev.Status)
		if !ev.Synthetic && ev.Type != "" {
			header += " " + ev.Type
		}
		fmt.Fprintln(w, header)

		if ev.Synthetic {
			if ev.Message != "" {
				fmt.Fprintf(w, "   %s\n", ev.Message)
			}
		} else {
			if ev.Workflow != "" {
				fmt.Fprintf(w, "   Workflow: %s\n", ev.Workflow)
			}
			if ev.CreatedAt != "" {
				fmt.Fprintf(w, "   Created: %s\n", ev.CreatedAt)
			}
			if ev.ExitCode != nil {
				fmt.Fprintf(w, "   Exit code: %d\n", *ev.ExitCode)
			}
			if len(ev.Codes) > 0 {
				fmt.Fprintf(w, "   Codes: %s\n", strings.Join(ev.Codes, ", "))
			}
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "- %d events\n", t.Summary.Total)
	fmt.Fprintf(w, "- %d passed\n", t.Summary.Passed)
	fmt.Fprintf(w, "- %d failed\n", t.Summary.Failed)
	fmt.Fprintf(w, "- %d skipped\n", t.Summary.Skipped)
	if t.Summary.Valid+t.Summary.Invalid > 0 {
		fmt.Fprintf(w, "- %d valid\n", t.Summary.Valid)
		fmt.Fprintf(w, "- %d invalid\n", t.Summary.Invalid)
	}
	fmt.Fprintf(w, "- %d synthetic\n", t.Summary.Synthetic)
}
