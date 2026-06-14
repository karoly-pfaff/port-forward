package core

// Rendering for the replay analysis: deterministic human-readable text and a single
// JSON marshaller (used for both stdout under --json and the --out file so the two
// are byte-identical).

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// MarshalAnalysis renders the analysis as deterministic, indented JSON with a
// trailing newline — the single source of JSON bytes so `--json` stdout and the
// `--out` file are byte-identical.
func MarshalAnalysis(a Analysis) ([]byte, error) {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}
	return append(data, '\n'), nil
}

// RenderAnalysisHuman writes the human-readable analysis. History exports show a
// runs summary (plus a workflow distribution); other artifacts show a step summary
// (valid/invalid for a plan, passed/failed/skipped otherwise). Codes, findings, and
// insights follow.
func RenderAnalysisHuman(w io.Writer, a Analysis) {
	fmt.Fprintln(w, "Portier Replay Analysis")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Source: %s\n", a.Source)
	if strings.TrimSpace(a.Workflow) != "" {
		fmt.Fprintf(w, "Workflow: %s\n", a.Workflow)
	}
	if strings.TrimSpace(a.Result) != "" {
		fmt.Fprintf(w, "Result: %s\n", a.Result)
	}
	fmt.Fprintln(w)

	if a.Source == SourceWorkflowHistoryExport {
		fmt.Fprintln(w, "Runs:")
		fmt.Fprintf(w, "- %d total\n", a.Summary.Runs.Total)
		fmt.Fprintf(w, "- %d passed\n", a.Summary.Runs.Passed)
		fmt.Fprintf(w, "- %d failed\n", a.Summary.Runs.Failed)
		fmt.Fprintln(w)
		if len(a.Workflows) > 0 {
			fmt.Fprintln(w, "Workflows:")
			for _, wc := range a.Workflows {
				fmt.Fprintf(w, "- %s: %d\n", wc.Name, wc.Count)
			}
			fmt.Fprintln(w)
		}
	} else {
		fmt.Fprintln(w, "Step summary:")
		fmt.Fprintf(w, "- %d total\n", a.Summary.Steps.Total)
		if isPlanStyle(a) {
			fmt.Fprintf(w, "- %d valid\n", a.Summary.Steps.Valid)
			fmt.Fprintf(w, "- %d invalid\n", a.Summary.Steps.Invalid)
		} else {
			fmt.Fprintf(w, "- %d passed\n", a.Summary.Steps.Passed)
			fmt.Fprintf(w, "- %d failed\n", a.Summary.Steps.Failed)
			fmt.Fprintf(w, "- %d skipped\n", a.Summary.Steps.Skipped)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Codes:")
	if len(a.Codes) == 0 {
		fmt.Fprintln(w, "- none")
	} else {
		for _, c := range a.Codes {
			fmt.Fprintf(w, "- %s: %d\n", c.Code, c.Count)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Findings:")
	if len(a.Findings) == 0 {
		fmt.Fprintln(w, "- none")
	} else {
		for _, f := range a.Findings {
			fmt.Fprintf(w, "- %s\n", f.Message)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Insights:")
	if len(a.Insights) == 0 {
		fmt.Fprintln(w, "- none")
	} else {
		for _, s := range a.Insights {
			fmt.Fprintf(w, "- %s\n", s)
		}
	}
}

// isPlanStyle reports whether the analysis should present steps as valid/invalid
// (a plan report, or a bundle wrapping one) rather than passed/failed/skipped.
func isPlanStyle(a Analysis) bool {
	return a.Source == SourceWorkflowPlanReport || a.Summary.Steps.Valid+a.Summary.Steps.Invalid > 0
}
