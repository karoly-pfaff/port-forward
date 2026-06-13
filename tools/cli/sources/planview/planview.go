// Package planview renders Portier config plan/diff/apply API responses for the
// CLI: deterministic human-readable plan, diff, and apply output, plus the
// exit-code interpretation of a plan/apply response. It is presentation only —
// it operates on the client DTOs (ConfigPlanResponse / ConfigApplyResponse) and
// holds no plan/apply logic (that lives server-side). The CLI command handlers
// in package commands compose it with the API client.
package planview

import (
	"fmt"
	"io"
	"strings"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
)

// PlanExitCode determines the exit code for the plan/diff commands.
// Priority: plan errors (1) > drift with --fail-on-drift (4) > success (0).
func PlanExitCode(plan *client.ConfigPlanResponse, failOnDrift bool) int {
	if plan.Summary.HasErrors {
		return 1
	}
	if failOnDrift && plan.Summary.HasDrift {
		return 4
	}
	return 0
}

// PrintPlan renders the plan response in structured human-readable format.
func PrintPlan(plan *client.ConfigPlanResponse, w io.Writer) {
	s := plan.Summary

	if !s.HasDrift && !s.HasErrors {
		fmt.Fprintln(w, "No drift detected.")
		if s.Unchanged > 0 {
			fmt.Fprintf(w, "  %d %s unchanged.\n", s.Unchanged, output.PluralRule(s.Unchanged))
		}
		if len(plan.Warnings) > 0 {
			fmt.Fprintln(w)
			printPlanWarnings(plan.Warnings, w)
		}
		return
	}

	fmt.Fprintln(w, "Config Plan")
	parts := []string{
		fmt.Sprintf("Add: %d", s.Add),
		fmt.Sprintf("Update: %d", s.Update),
		fmt.Sprintf("Remove: %d", s.Remove),
		fmt.Sprintf("Unchanged: %d", s.Unchanged),
	}
	if s.Destructive > 0 {
		parts = append(parts, fmt.Sprintf("Destructive: %d", s.Destructive))
	}
	fmt.Fprintf(w, "  %s\n", strings.Join(parts, "  "))

	if len(plan.Operations) > 0 {
		fmt.Fprintln(w)
		for _, op := range plan.Operations {
			dest := ""
			if op.Destructive {
				dest = "  [destructive]"
			}
			fmt.Fprintf(w, "  %-10s %-24s %-5s %s%s\n",
				op.Type, op.RuleName, op.Protocol, opEndpoint(op), dest)
			for _, ch := range op.Changes {
				fmt.Fprintf(w, "    %s: %s → %s\n",
					ch.Field, formatChangeValue(ch.Before), formatChangeValue(ch.After))
			}
		}
	}

	if len(plan.Warnings) > 0 {
		fmt.Fprintln(w)
		printPlanWarnings(plan.Warnings, w)
	}
	if len(plan.Errors) > 0 {
		fmt.Fprintln(w)
		printPlanErrors(plan.Errors, w)
	}
}

// PrintDiff renders the plan response as a visual diff.
func PrintDiff(plan *client.ConfigPlanResponse, showUnchanged bool, w io.Writer) {
	s := plan.Summary

	if !s.HasDrift && !s.HasErrors {
		fmt.Fprintln(w, "No drift detected.")
		return
	}

	for _, op := range plan.Operations {
		if op.Type == "unchanged" && !showUnchanged {
			continue
		}
		dest := ""
		if op.Destructive {
			dest = "  [destructive]"
		}
		fmt.Fprintf(w, "%s %-9s %-24s %-5s %s%s\n",
			opDiffPrefix(op.Type), opDiffLabel(op.Type),
			op.RuleName, op.Protocol, opEndpoint(op), dest)
		for _, ch := range op.Changes {
			fmt.Fprintf(w, "  %s: %s → %s\n",
				ch.Field, formatChangeValue(ch.Before), formatChangeValue(ch.After))
		}
	}

	if len(plan.Warnings) > 0 {
		fmt.Fprintln(w)
		printPlanWarnings(plan.Warnings, w)
	}
	if len(plan.Errors) > 0 {
		fmt.Fprintln(w)
		printPlanErrors(plan.Errors, w)
	}
}

func opEndpoint(op client.ConfigPlanOperation) string {
	snap := op.Current
	if snap == nil {
		snap = op.Desired
	}
	if snap == nil {
		return "—"
	}
	return fmt.Sprintf("%s:%d → %s:%d", snap.ListenHost, snap.ListenPort, snap.TargetHost, snap.TargetPort)
}

func opDiffPrefix(opType string) string {
	switch opType {
	case "add":
		return "+"
	case "update":
		return "~"
	case "remove":
		return "-"
	default:
		return "="
	}
}

func opDiffLabel(opType string) string {
	switch opType {
	case "add":
		return "Add:"
	case "update":
		return "Update:"
	case "remove":
		return "Remove:"
	default:
		return "Unchanged:"
	}
}

func formatChangeValue(v any) string {
	if v == nil {
		return "(none)"
	}
	if f, ok := v.(float64); ok {
		if f == float64(int64(f)) {
			return fmt.Sprintf("%d", int64(f))
		}
		return fmt.Sprintf("%g", f)
	}
	return fmt.Sprintf("%v", v)
}

func printPlanWarnings(warnings []client.ConfigPlanWarning, w io.Writer) {
	fmt.Fprintln(w, "Warnings:")
	for _, warn := range warnings {
		fmt.Fprintf(w, "  [%s] %s\n", warn.Code, warn.Message)
	}
}

func printPlanErrors(errs []client.ConfigPlanError, w io.Writer) {
	fmt.Fprintln(w, "Errors:")
	for _, e := range errs {
		fmt.Fprintf(w, "  [%s] %s\n", e.Code, e.Message)
	}
}

// ApplyExitCode maps an apply response to an exit code.
func ApplyExitCode(resp *client.ConfigApplyResponse) int {
	if !resp.Ok {
		return 1
	}
	return 0
}

// PrintApply renders the apply response in human-readable format.
func PrintApply(resp *client.ConfigApplyResponse, stdout, stderr io.Writer) {
	plan := resp.Plan

	if !resp.Ok {
		fmt.Fprintln(stderr, "Apply failed — plan has errors.")
		printPlanErrors(plan.Errors, stderr)
		return
	}

	if resp.DryRun {
		fmt.Fprintln(stdout, "Dry run complete — no changes applied.")
		if !plan.Summary.HasDrift {
			fmt.Fprintln(stdout, "  No drift detected.")
			return
		}
		fmt.Fprintf(stdout, "  Would apply: +%d update:%d -%d (unchanged:%d)\n",
			resp.Applied.Add, resp.Applied.Update, resp.Applied.Remove, resp.Applied.Unchanged)
		return
	}

	if !plan.Summary.HasDrift {
		fmt.Fprintln(stdout, "No drift detected — nothing to apply.")
		return
	}

	fmt.Fprintln(stdout, "Config applied.")
	fmt.Fprintf(stdout, "  +%d added  ~%d updated  -%d removed  =%d unchanged\n",
		resp.Applied.Add, resp.Applied.Update, resp.Applied.Remove, resp.Applied.Unchanged)

	if len(plan.Warnings) > 0 {
		fmt.Fprintln(stdout)
		printPlanWarnings(plan.Warnings, stdout)
	}
}
