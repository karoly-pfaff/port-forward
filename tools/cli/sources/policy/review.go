package policy

// Policy-aware config review: compare a current config with a candidate config
// (compact, deterministic change summary) and evaluate ONLY the candidate against
// a policy, reusing the policy check finding/report semantics. Fully offline and
// dry-run — it reads files, never mutates anything, and never contacts the
// runtime. It is NOT a full config diff engine; the change summary is scalar
// counts derived from the existing config.Summary.

import (
	"fmt"
	"io"

	"portier/cli/sources/config"
	"portier/cli/sources/explain"
	"portier/cli/sources/output"
)

// ReviewDelta is the compact scalar difference (candidate − current) between the
// two config summaries. It mirrors the scalar count fields of config.Summary;
// per-protocol and per-group deltas are intentionally omitted (keep it compact).
type ReviewDelta struct {
	RuleCount          int `json:"ruleCount"`
	EnabledRuleCount   int `json:"enabledRuleCount"`
	DisabledRuleCount  int `json:"disabledRuleCount"`
	GroupCount         int `json:"groupCount"`
	UngroupedRuleCount int `json:"ungroupedRuleCount"`
}

// ReviewSummary is the change summary of a review: the current and candidate
// config summaries (reusing config.BuildSummary) plus their scalar delta.
type ReviewSummary struct {
	Current   config.Summary `json:"current"`
	Candidate config.Summary `json:"candidate"`
	Delta     ReviewDelta    `json:"delta"`
}

// Review is the full result of a policy review: the change summary under
// "review", plus the candidate's policy evaluation (findings/summary/result)
// at the top level via the embedded Report. The embedded Report reuses the exact
// policy check semantics — there is no second finding/report schema.
type Review struct {
	Change ReviewSummary `json:"review"`
	Report
}

// BuildReview computes a deterministic review: summaries for both configs, their
// scalar delta, and the candidate's policy evaluation. Pure and offline — it uses
// only the parsed configs and the policy.
func BuildReview(current, candidate []config.Rule, pol Policy) Review {
	cur := config.BuildSummary(current)
	cand := config.BuildSummary(candidate)
	return Review{
		Change: ReviewSummary{
			Current:   cur,
			Candidate: cand,
			Delta: ReviewDelta{
				RuleCount:          cand.RuleCount - cur.RuleCount,
				EnabledRuleCount:   cand.EnabledRuleCount - cur.EnabledRuleCount,
				DisabledRuleCount:  cand.DisabledRuleCount - cur.DisabledRuleCount,
				GroupCount:         cand.GroupCount - cur.GroupCount,
				UngroupedRuleCount: cand.UngroupedRuleCount - cur.UngroupedRuleCount,
			},
		},
		Report: Evaluate(candidate, pol),
	}
}

// PrintReviewHuman renders a review in deterministic human-readable form: a
// compact config-changes block, then the candidate's policy findings and summary
// (reusing the policy check finding rendering). With withExplain each finding is
// followed by its inline explanation.
func PrintReviewHuman(rv Review, withExplain bool, w io.Writer) {
	fmt.Fprintln(w, "Portier Policy Review")
	fmt.Fprintln(w)

	c, n := rv.Change.Current, rv.Change.Candidate
	fmt.Fprintln(w, "Config changes:")
	fmt.Fprintf(w, "- Rules: %d → %d (%+d)\n", c.RuleCount, n.RuleCount, n.RuleCount-c.RuleCount)
	fmt.Fprintf(w, "- Enabled rules: %d → %d (%+d)\n", c.EnabledRuleCount, n.EnabledRuleCount, n.EnabledRuleCount-c.EnabledRuleCount)
	fmt.Fprintf(w, "- Disabled rules: %d → %d (%+d)\n", c.DisabledRuleCount, n.DisabledRuleCount, n.DisabledRuleCount-c.DisabledRuleCount)
	fmt.Fprintf(w, "- Groups: %d → %d (%+d)\n", c.GroupCount, n.GroupCount, n.GroupCount-c.GroupCount)
	fmt.Fprintf(w, "- Ungrouped rules: %d → %d (%+d)\n", c.UngroupedRuleCount, n.UngroupedRuleCount, n.UngroupedRuleCount-c.UngroupedRuleCount)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Policy findings:")
	printFindings(rv.Report, withExplain, w)
	printSummaryAndResult(rv.Report, w)
}

// reviewJSON is the JSON encoding of a review plus an optional additive
// explanations map. Review is embedded so review/findings/summary/result stay at
// the top level; Explanations is populated ONLY with --explain (and only for the
// codes present in the candidate evaluation), so output without --explain is
// byte-identical to the bare Review.
type reviewJSON struct {
	Review
	Explanations map[string]explain.Explanation `json:"explanations,omitempty"`
}

// EmitReview prints a review (JSON when opts.JSON is set, otherwise human) and,
// when opts.OutPath is non-empty, also writes the exact same JSON review to that
// file. With opts.Explain it adds inline explanations (human blocks; an additive
// `explanations` map in JSON, deduplicated by code) for the candidate's finding
// codes — without changing the review, findings, summary, result, or exit code.
// It returns the candidate's policy exit code, except that a JSON-encode or
// file-write failure overrides it with 1 (an operation failure). Never mutates
// configs/policy and never contacts the runtime.
func EmitReview(rv Review, opts EmitOptions, stdout, stderr io.Writer) int {
	payload := reviewJSON{Review: rv}
	if opts.Explain {
		payload.Explanations = explain.ForReport(explanations, codesOf(rv.Report))
	}

	if opts.JSON {
		if err := output.PrintJSON(stdout, payload); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		PrintReviewHuman(rv, opts.Explain, stdout)
	}

	if opts.OutPath != "" {
		if err := output.WritePrettyJSON(opts.OutPath, payload); err != nil {
			fmt.Fprintf(stderr, "Error writing %s: %v\n", opts.OutPath, err)
			return 1
		}
		// In human mode, confirm the export on stdout (doctor/policy --out do the
		// same). In JSON mode stdout must stay valid JSON, so stay silent.
		if !opts.JSON {
			fmt.Fprintf(stdout, "\nReview written to %s\n", opts.OutPath)
		}
	}

	return ExitCode(rv.Report)
}
