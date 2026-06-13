package workflow

// Workflow runbook: a deterministic, offline preview that turns a VALID workflow
// plan into the ordered list of Portier CLI commands a user would run manually to
// carry the workflow out. It is a PREVIEW ONLY — it never executes a command,
// contacts the runtime, reads the files a step refers to, or inspects their
// contents. It is built only from the parsed workflow file (after the plan is
// confirmed valid by the caller), so the mapping is a pure transformation.

import (
	"fmt"
	"io"
	"strings"

	"portier/cli/sources/output"
)

// runbookResultReady is the only runbook result: a runbook is produced solely
// from a valid plan, so it is always "ready".
const runbookResultReady = "ready"

// Runbook is the deterministic command-preview for a valid workflow. It is the
// JSON contract emitted by `workflow runbook` under --json, kept separate from
// the Plan and policy/doctor report models.
type Runbook struct {
	Workflow string         `json:"workflow"`
	Steps    []RunbookStep  `json:"steps"`
	Summary  RunbookSummary `json:"summary"`
	Result   string         `json:"result"`
}

// RunbookStep is the preview for one workflow step: the argv-style Command (the
// canonical, non-shell-interpreted token list), a best-effort Display string for
// copy/paste, and any Notes (e.g. a reportFrom placeholder explanation).
type RunbookStep struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Command []string `json:"command"`
	Display string   `json:"display"`
	Notes   []string `json:"notes"`
}

// RunbookSummary counts the steps in a runbook.
type RunbookSummary struct {
	Total int `json:"total"`
}

// BuildRunbook maps a parsed workflow file to a deterministic runbook. It MUST be
// called only after BuildPlan has confirmed the workflow is valid (the caller
// enforces this); given a valid plan, every step has a supported type and
// coherent inputs, so each step maps to exactly one command preview. It is pure
// and offline — it reads only the parsed file, never the referenced files, and
// never contacts the runtime. Step order is preserved.
func BuildRunbook(f File) Runbook {
	steps := make([]RunbookStep, len(f.Steps))
	for i, s := range f.Steps {
		cmd, notes := runbookCommand(s)
		if notes == nil {
			notes = []string{}
		}
		steps[i] = RunbookStep{
			ID:      s.ID,
			Type:    s.Type,
			Command: cmd,
			Display: displayCommand(cmd),
			Notes:   notes,
		}
	}
	return Runbook{
		Workflow: f.Name,
		Steps:    steps,
		Summary:  RunbookSummary{Total: len(steps)},
		Result:   runbookResultReady,
	}
}

// reportFromPlaceholder is the deterministic placeholder token used in a
// baseline-compare preview when the report comes from an earlier step
// (reportFrom). It is deliberately NOT a real file path — it is an explicit
// `<report-from:step-id>` marker the user must replace with the report that step
// produces (e.g. by running that step with --out).
func reportFromPlaceholder(stepID string) string {
	return "<report-from:" + stepID + ">"
}

// runbookCommand maps one step to its argv-style command preview and any notes.
// It switches on the (already validated) step type; the trailing return is a
// defensive fallback for an unsupported type, which cannot occur for a valid plan
// (covered directly by a white-box test).
func runbookCommand(s Step) ([]string, []string) {
	switch s.Type {
	case stepPolicyCheck:
		cmd := []string{"portier", "policy", "check"}
		if s.Runtime {
			cmd = append(cmd, "--runtime")
		} else {
			cmd = append(cmd, "--config", s.Config)
		}
		cmd = append(cmd, "--policy", s.Policy)
		return cmd, nil
	case stepPolicyReview:
		return []string{"portier", "policy", "review", "--current", s.Current, "--candidate", s.Candidate, "--policy", s.Policy}, nil
	case stepPolicyBaselineCompare:
		cmd := []string{"portier", "policy", "baseline", "compare", "--baseline", s.Baseline}
		if s.Report != "" {
			return append(cmd, "--report", s.Report), nil
		}
		placeholder := reportFromPlaceholder(s.ReportFrom)
		notes := []string{fmt.Sprintf("Replace %s with the report file produced by step %q (run that step with --out, or supply a report file).", placeholder, s.ReportFrom)}
		return append(cmd, "--report", placeholder), notes
	}
	return nil, nil
}

// displayCommand renders an argv token list as a best-effort, copy/paste-friendly
// single-line command. Tokens containing whitespace/quotes/backslash are
// single-quoted; other tokens (including the `<report-from:...>` placeholder and
// ordinary paths) are left as-is. The Command argv is the canonical form — Portier
// never shell-executes the Display string.
func displayCommand(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuoteToken(a)
	}
	return strings.Join(parts, " ")
}

// shellQuoteToken single-quotes a token only when it contains whitespace, quotes,
// or a backslash; otherwise it returns the token unchanged.
func shellQuoteToken(s string) string {
	if s == "" {
		return "''"
	}
	if strings.ContainsAny(s, " \t\n'\"\\") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

// PrintRunbookHuman renders a runbook as a deterministic, numbered, human-readable
// list of commands. Each step shows its id, the copy/paste command, and any notes.
func PrintRunbookHuman(rb Runbook, w io.Writer) {
	fmt.Fprintln(w, "Portier Workflow Runbook")
	fmt.Fprintln(w)
	name := rb.Workflow
	if strings.TrimSpace(name) == "" {
		name = "(unnamed)"
	}
	fmt.Fprintf(w, "Workflow: %s\n\n", name)

	for i, s := range rb.Steps {
		fmt.Fprintf(w, "%d. %s\n", i+1, s.ID)
		fmt.Fprintf(w, "   %s\n", s.Display)
		for _, note := range s.Notes {
			fmt.Fprintf(w, "   Note: %s\n", note)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  %d %s\n", rb.Summary.Total, output.PluralWord(rb.Summary.Total, "step", "steps"))
	fmt.Fprintf(w, "\nResult: %s\n", rb.Result)
}

// EmitRunbook prints a runbook (JSON when jsonOutput is set, otherwise human) and,
// when outPath is non-empty, also writes the exact same JSON runbook to that file
// (byte-identical to the --json stdout output). It returns 0 on success, or 1 if
// a JSON-encode or file-write failure occurs (an operation failure). It never
// mutates the workflow file and never contacts the runtime.
func EmitRunbook(rb Runbook, jsonOutput bool, outPath string, stdout, stderr io.Writer) int {
	if jsonOutput {
		if err := output.PrintJSON(stdout, rb); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		PrintRunbookHuman(rb, stdout)
	}

	if outPath != "" {
		if err := output.WritePrettyJSON(outPath, rb); err != nil {
			fmt.Fprintf(stderr, "Error writing %s: %v\n", outPath, err)
			return 1
		}
		// In human mode, confirm the export on stdout (plan/policy/doctor --out do
		// the same). In JSON mode stdout must stay valid JSON, so stay silent.
		if !jsonOutput {
			fmt.Fprintf(stdout, "\nRunbook written to %s\n", outPath)
		}
	}

	return 0
}
