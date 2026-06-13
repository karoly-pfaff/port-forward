// Package policy holds the Portier CLI's offline policy model and its
// deterministic dry-run evaluator: parsing a small JSON policy file into a set
// of guardrails, evaluating parsed config rules against it, and the policy
// result types (Report/Finding/Summary). It builds on the config domain and
// reuses the doctor/config interpretations (LAN exposure, privileged port,
// duplicate bindings) so the two cannot drift. It contains no command/dispatch
// code and never contacts the runtime.
package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"portier/cli/sources/config"
	"portier/cli/sources/explain"
	"portier/cli/sources/output"
)

// Severity classifies the importance of a single policy finding. It is a small
// fixed vocabulary, intentionally mirroring the doctor severity style but kept
// separate from the doctor model so the two cannot blur. In this first slice
// every policy violation is an error; info is used only for "policy.valid".
type Severity string

const (
	// Info is an informational finding (no violation).
	Info Severity = "info"
	// Warning flags something to review (unused in this slice; reserved).
	Warning Severity = "warning"
	// Error flags a policy violation; its presence fails the policy check.
	Error Severity = "error"
)

// Policy finding codes. These are stable, operator-facing identifiers — do not
// rename them casually (they are a CLI/tool contract). They are deliberately
// separate from the doctor/check codes (config doctor uses "config.*").
const (
	codeValid                     = "policy.valid"
	codeGroupRequired             = "policy.group_required"
	codeLanExposureForbidden      = "policy.lan_exposure_forbidden"
	codePrivilegedPortForbidden   = "policy.privileged_port_forbidden"
	codeAutostartForbidden        = "policy.autostart_forbidden"
	codeDuplicateBindingForbidden = "policy.duplicate_binding_forbidden"
)

// schemaVersion is the only supported policy file schema version.
const schemaVersion = 1

// Finding is one deterministic outcome of a policy evaluation:
//   - Code:     stable machine-readable identifier (e.g. "policy.lan_exposure_forbidden").
//   - Severity: info / warning / error.
//   - Title:    short human heading.
//   - Message:  actionable human explanation.
//   - Details:  optional deterministic extra data (omitted when empty).
//
// Finding codes are operator-facing identifiers — do not rename them casually.
type Finding struct {
	Code     string         `json:"code"`
	Severity Severity       `json:"severity"`
	Title    string         `json:"title"`
	Message  string         `json:"message"`
	Details  map[string]any `json:"details,omitempty"`
}

// Summary counts the findings in a report by severity.
type Summary struct {
	Info    int `json:"info"`
	Warning int `json:"warning"`
	Error   int `json:"error"`
}

// Report is the full deterministic result of a policy evaluation: the ordered
// list of findings, a severity summary, and a pass/fail result. It is the JSON
// contract emitted by `policy check` under --json. It is intentionally kept
// separate from the doctor Report (different semantics: a policy is operator-
// defined guardrails, not runtime/config diagnostics).
type Report struct {
	Findings []Finding `json:"findings"`
	Summary  Summary   `json:"summary"`
	Result   string    `json:"result"`
}

// NewReport builds a report from an ordered list of findings, deriving the
// summary counts and the pass/fail result. A nil findings slice is normalized to
// an empty slice so the JSON contract always carries a "findings" array. The
// result is "failed" when any error-severity finding is present, otherwise
// "passed" (warnings alone do not fail in this slice).
func NewReport(findings []Finding) Report {
	if findings == nil {
		findings = []Finding{}
	}
	summary := Summary{}
	for _, f := range findings {
		switch f.Severity {
		case Info:
			summary.Info++
		case Warning:
			summary.Warning++
		case Error:
			summary.Error++
		}
	}
	result := "passed"
	if summary.Error > 0 {
		result = "failed"
	}
	return Report{Findings: findings, Summary: summary, Result: result}
}

// ExitCode maps a report to a CLI exit code: 1 when one or more error-severity
// (violation) findings are present, otherwise 0.
func ExitCode(r Report) int {
	if r.Summary.Error > 0 {
		return 1
	}
	return 0
}

// Policy is a policy file's rules after defaults are applied. Booleans are plain
// (not pointers) so evaluation stays simple; the defaults are applied in Parse.
// The baseline (an empty rules object) is fully permissive — an operator opts
// INTO each guardrail. Fields are unexported; construct one via Parse.
type Policy struct {
	requireGroup            bool
	allowLanExposure        bool
	allowPrivilegedPorts    bool
	allowAutostart          bool
	forbidDuplicateBindings bool
}

// fileRaw is the on-disk policy JSON. Pointers distinguish an omitted field
// (apply the default) from an explicit value. schemaVersion is required.
type fileRaw struct {
	SchemaVersion *int     `json:"schemaVersion"`
	Rules         rulesRaw `json:"rules"`
}

// rulesRaw is the rules object of a policy file. Pointers let an omitted field
// fall back to its default in Parse.
type rulesRaw struct {
	RequireGroup            *bool `json:"requireGroup"`
	AllowLanExposure        *bool `json:"allowLanExposure"`
	AllowPrivilegedPorts    *bool `json:"allowPrivilegedPorts"`
	AllowAutostart          *bool `json:"allowAutostart"`
	ForbidDuplicateBindings *bool `json:"forbidDuplicateBindings"`
}

// Parse decodes and validates a policy file, applying defaults for omitted
// fields. Unknown fields are REJECTED (DisallowUnknownFields) so a typo'd rule
// name fails loudly rather than silently relaxing a guardrail. The supported
// schemaVersion is required and must equal schemaVersion. Defaults form a
// permissive baseline: requireGroup=false, allow*=true, forbidDuplicateBindings
// =false (so an empty policy permits everything; operators opt into restrictions).
func Parse(data []byte) (Policy, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Policy{}, fmt.Errorf("policy file is empty")
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var raw fileRaw
	if err := dec.Decode(&raw); err != nil {
		return Policy{}, fmt.Errorf("not a valid policy file: %w", err)
	}
	if dec.More() {
		return Policy{}, fmt.Errorf("policy file has trailing data after the JSON object")
	}

	if raw.SchemaVersion == nil {
		return Policy{}, fmt.Errorf("policy file is missing the required \"schemaVersion\" field")
	}
	if *raw.SchemaVersion != schemaVersion {
		return Policy{}, fmt.Errorf("unsupported policy schemaVersion %d (expected %d)", *raw.SchemaVersion, schemaVersion)
	}

	return Policy{
		requireGroup:            boolOr(raw.Rules.RequireGroup, false),
		allowLanExposure:        boolOr(raw.Rules.AllowLanExposure, true),
		allowPrivilegedPorts:    boolOr(raw.Rules.AllowPrivilegedPorts, true),
		allowAutostart:          boolOr(raw.Rules.AllowAutostart, true),
		forbidDuplicateBindings: boolOr(raw.Rules.ForbidDuplicateBindings, false),
	}, nil
}

// boolOr returns *p when p is non-nil, otherwise def.
func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// Evaluate evaluates parsed config rules against a policy and returns a
// deterministic report. Ordering is fixed: per-rule checks run in config file
// order, and for a single rule its findings appear in policy order (group → LAN
// exposure → privileged port → autostart). Duplicate-binding findings (which are
// inherently multi-rule) are emitted last, one per conflicting binding, sorted by
// protocol → listenHost → listenPort (reusing config.FindDuplicateBindings so the
// two interpretations cannot drift). When nothing is violated, a single info
// "policy.valid" finding is returned. Pure and offline: it uses only the config
// and policy contents.
func Evaluate(rules []config.Rule, pol Policy) Report {
	findings := []Finding{}

	for _, r := range rules {
		if pol.requireGroup && strings.TrimSpace(r.Group) == "" {
			findings = append(findings, groupRequiredFinding(r))
		}
		if !pol.allowLanExposure && r.ListenHost == "0.0.0.0" {
			findings = append(findings, lanExposureFinding(r))
		}
		if !pol.allowPrivilegedPorts && r.ListenPort >= 1 && r.ListenPort < 1024 {
			findings = append(findings, privilegedPortFinding(r))
		}
		if !pol.allowAutostart && r.Enabled {
			findings = append(findings, autostartFinding(r))
		}
	}

	if pol.forbidDuplicateBindings {
		for _, b := range config.FindDuplicateBindings(rules) {
			findings = append(findings, duplicateBindingFinding(b))
		}
	}

	if len(findings) == 0 {
		findings = append(findings, validFinding(len(rules)))
	}
	return NewReport(findings)
}

func groupRequiredFinding(r config.Rule) Finding {
	return Finding{
		Code:     codeGroupRequired,
		Severity: Error,
		Title:    fmt.Sprintf("%s is missing a required group", config.RuleLabelName(r.Name)),
		Message:  "The policy requires every rule to belong to a group, but this rule has no group.",
		Details:  map[string]any{"rule": config.ToRuleDetail(r)},
	}
}

func lanExposureFinding(r config.Rule) Finding {
	return Finding{
		Code:     codeLanExposureForbidden,
		Severity: Error,
		Title:    fmt.Sprintf("%s listens on 0.0.0.0", config.RuleLabelName(r.Name)),
		Message:  "This rule listens on 0.0.0.0, but the policy forbids LAN exposure.",
		Details:  map[string]any{"rule": config.ToRuleDetail(r)},
	}
}

func privilegedPortFinding(r config.Rule) Finding {
	return Finding{
		Code:     codePrivilegedPortForbidden,
		Severity: Error,
		Title:    fmt.Sprintf("%s uses privileged port %d", config.RuleLabelName(r.Name), r.ListenPort),
		Message:  "This rule listens on a privileged port (below 1024), but the policy forbids privileged ports.",
		Details:  map[string]any{"rule": config.ToRuleDetail(r)},
	}
}

func autostartFinding(r config.Rule) Finding {
	return Finding{
		Code:     codeAutostartForbidden,
		Severity: Error,
		Title:    fmt.Sprintf("%s has autostart enabled", config.RuleLabelName(r.Name)),
		Message:  "This rule is enabled for autostart, but the policy forbids autostart.",
		Details:  map[string]any{"rule": config.ToRuleDetail(r)},
	}
}

func duplicateBindingFinding(b config.BindingConflict) Finding {
	return Finding{
		Code:     codeDuplicateBindingForbidden,
		Severity: Error,
		Title:    fmt.Sprintf("Duplicate listen binding %s %s:%d", b.Protocol, b.ListenHost, b.ListenPort),
		Message:  fmt.Sprintf("%d rules share the listen binding %s %s:%d, but the policy forbids duplicate bindings.", len(b.Rules), b.Protocol, b.ListenHost, b.ListenPort),
		Details:  map[string]any{"binding": b},
	}
}

func validFinding(ruleCount int) Finding {
	return Finding{
		Code:     codeValid,
		Severity: Info,
		Title:    "Config complies with the policy",
		Message:  "The config satisfies all enabled policy rules.",
		Details:  map[string]any{"ruleCount": ruleCount},
	}
}

// severityTag returns the fixed-width ASCII tag for a policy severity, matching
// the CLI's symbol-free human output style.
func severityTag(s Severity) string {
	switch s {
	case Info:
		return "[INFO]"
	case Warning:
		return "[WARN]"
	case Error:
		return "[ERROR]"
	default:
		return "[?]"
	}
}

// PrintHuman renders a policy report in deterministic human-readable form. When
// withExplain is set, each finding is followed by an inline explanation block
// (code, meaning, next action, related codes) for its finding code.
func PrintHuman(r Report, withExplain bool, w io.Writer) {
	fmt.Fprintln(w, "Portier Policy Check")
	fmt.Fprintln(w)

	for _, f := range r.Findings {
		fmt.Fprintf(w, "%-7s %s\n", severityTag(f.Severity), f.Title)
		if f.Message != "" {
			fmt.Fprintf(w, "        %s\n", f.Message)
		}
		if withExplain {
			explain.PrintInline(explanations, f.Code, w)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  %d info\n", r.Summary.Info)
	fmt.Fprintf(w, "  %d %s\n", r.Summary.Warning, output.PluralWord(r.Summary.Warning, "warning", "warnings"))
	fmt.Fprintf(w, "  %d %s\n", r.Summary.Error, output.PluralWord(r.Summary.Error, "error", "errors"))
	fmt.Fprintf(w, "\nResult: %s\n", r.Result)
}

// reportJSON is the JSON encoding of a policy report plus an optional additive
// explanations map. Report is embedded so findings/summary/result stay at the
// top level; Explanations is populated ONLY with --explain (and only for the
// codes present in the report), so output without --explain is byte-identical to
// the bare Report.
type reportJSON struct {
	Report
	Explanations map[string]explain.Explanation `json:"explanations,omitempty"`
}

// EmitOptions groups the presentation flags for a policy report. They affect
// ONLY how the report is rendered/exported — never the findings, summary, result,
// or exit code. OutPath, when non-empty, also writes the JSON report to that file.
type EmitOptions struct {
	Explain bool
	JSON    bool
	OutPath string
}

// codesOf returns the finding codes of a report, in order.
func codesOf(r Report) []string {
	codes := make([]string, len(r.Findings))
	for i, f := range r.Findings {
		codes[i] = f.Code
	}
	return codes
}

// Emit prints a policy report (JSON when opts.JSON is set, otherwise human) and,
// when opts.OutPath is non-empty, also writes the exact same JSON report to that
// file (the same `{findings, summary, result}` shape as --json, plus the additive
// `explanations` map under --explain). With opts.Explain it adds inline
// explanations (human blocks; an additive `explanations` map in JSON, deduplicated
// by code) for the finding codes present in the report — without changing
// findings, summary, result, or the exit code. It returns the report's exit code,
// except that a JSON-encode or file-write failure overrides it with 1 (an
// operation failure, not a policy finding). Never mutates config/policy files and
// never contacts the runtime.
func Emit(r Report, opts EmitOptions, stdout, stderr io.Writer) int {
	payload := reportJSON{Report: r}
	if opts.Explain {
		payload.Explanations = explain.ForReport(explanations, codesOf(r))
	}

	if opts.JSON {
		if err := output.PrintJSON(stdout, payload); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		PrintHuman(r, opts.Explain, stdout)
	}

	if opts.OutPath != "" {
		if err := output.WritePrettyJSON(opts.OutPath, payload); err != nil {
			fmt.Fprintf(stderr, "Error writing %s: %v\n", opts.OutPath, err)
			return 1
		}
		// In human mode, confirm the export on stdout (doctor --out does the same).
		// In JSON mode stdout must stay valid JSON, so stay silent.
		if !opts.JSON {
			fmt.Fprintf(stdout, "\nReport written to %s\n", opts.OutPath)
		}
	}

	return ExitCode(r)
}
