package core

// Rendering for the replay explanation report: deterministic human-readable text and
// a single JSON marshaller (used for both stdout under --json and the --out file so
// the two are byte-identical).

import (
	"encoding/json"
	"fmt"
	"io"
)

// MarshalExplain renders the report as deterministic, indented JSON with a trailing
// newline — the single source of JSON bytes so `--json` stdout and the `--out` file
// are byte-identical.
func MarshalExplain(e Explain) ([]byte, error) {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}
	return append(data, '\n'), nil
}

// RenderExplainHuman writes the human-readable explanation report: a header with the
// code counts, then one block per code (or a no-codes notice).
func RenderExplainHuman(w io.Writer, e Explain) {
	fmt.Fprintln(w, "Portier Replay Explanations")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Source: %s\n", e.Source)
	fmt.Fprintf(w, "Codes: %d\n", e.CodeCount)
	if e.CodeCount > 0 {
		fmt.Fprintf(w, "Known: %d\n", e.Known)
		fmt.Fprintf(w, "Unknown: %d\n", e.Unknown)
	}
	fmt.Fprintln(w)

	if e.CodeCount == 0 {
		fmt.Fprintln(w, "No emitted codes were found in this artifact.")
		return
	}

	for _, ex := range e.Explanations {
		fmt.Fprintf(w, "%s [%s]\n", ex.Code, ex.Severity)
		fmt.Fprintf(w, "%s\n", ex.Title)
		fmt.Fprintf(w, "%s\n", ex.Meaning)
		fmt.Fprintf(w, "Suggestion: %s\n", ex.Suggestion)
		fmt.Fprintln(w)
	}
}
