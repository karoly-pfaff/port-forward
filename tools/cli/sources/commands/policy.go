package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"portier/cli/sources/output"
)

// PolicySeverity classifies the importance of a single policy finding. It is a
// small fixed vocabulary, intentionally mirroring the doctor severity style but
// kept separate from the doctor model so the two cannot blur. In this first
// slice every policy violation is an error; info is used only for "policy.valid".
type PolicySeverity string

const (
	// PolicyInfo is an informational finding (no violation).
	PolicyInfo PolicySeverity = "info"
	// PolicyWarning flags something to review (unused in this slice; reserved).
	PolicyWarning PolicySeverity = "warning"
	// PolicyError flags a policy violation; its presence fails the policy check.
	PolicyError PolicySeverity = "error"
)

// Policy finding codes. These are stable, operator-facing identifiers — do not
// rename them casually (they are a CLI/tool contract). They are deliberately
// separate from the doctor/check codes (config doctor uses "config.*").
const (
	policyValid                     = "policy.valid"
	policyGroupRequired             = "policy.group_required"
	policyLanExposureForbidden      = "policy.lan_exposure_forbidden"
	policyPrivilegedPortForbidden   = "policy.privileged_port_forbidden"
	policyAutostartForbidden        = "policy.autostart_forbidden"
	policyDuplicateBindingForbidden = "policy.duplicate_binding_forbidden"
)

// policySchemaVersion is the only supported policy file schema version.
const policySchemaVersion = 1

// PolicyFinding is one deterministic outcome of a policy evaluation:
//   - Code:     stable machine-readable identifier (e.g. "policy.lan_exposure_forbidden").
//   - Severity: info / warning / error.
//   - Title:    short human heading.
//   - Message:  actionable human explanation.
//   - Details:  optional deterministic extra data (omitted when empty).
//
// Finding codes are operator-facing identifiers — do not rename them casually.
type PolicyFinding struct {
	Code     string         `json:"code"`
	Severity PolicySeverity `json:"severity"`
	Title    string         `json:"title"`
	Message  string         `json:"message"`
	Details  map[string]any `json:"details,omitempty"`
}

// PolicySummary counts the findings in a report by severity.
type PolicySummary struct {
	Info    int `json:"info"`
	Warning int `json:"warning"`
	Error   int `json:"error"`
}

// PolicyReport is the full deterministic result of a policy evaluation: the
// ordered list of findings, a severity summary, and a pass/fail result. It is
// the JSON contract emitted by `policy check` under --json. It is intentionally
// kept separate from DoctorReport (different semantics: a policy is operator-
// defined guardrails, not runtime/config diagnostics).
type PolicyReport struct {
	Findings []PolicyFinding `json:"findings"`
	Summary  PolicySummary   `json:"summary"`
	Result   string          `json:"result"`
}

// newPolicyReport builds a report from an ordered list of findings, deriving the
// summary counts and the pass/fail result. A nil findings slice is normalized to
// an empty slice so the JSON contract always carries a "findings" array. The
// result is "failed" when any error-severity finding is present, otherwise
// "passed" (warnings alone do not fail in this slice).
func newPolicyReport(findings []PolicyFinding) PolicyReport {
	if findings == nil {
		findings = []PolicyFinding{}
	}
	summary := PolicySummary{}
	for _, f := range findings {
		switch f.Severity {
		case PolicyInfo:
			summary.Info++
		case PolicyWarning:
			summary.Warning++
		case PolicyError:
			summary.Error++
		}
	}
	result := "passed"
	if summary.Error > 0 {
		result = "failed"
	}
	return PolicyReport{Findings: findings, Summary: summary, Result: result}
}

// policyExitCode maps a report to a CLI exit code: 1 when one or more
// error-severity (violation) findings are present, otherwise 0.
func policyExitCode(r PolicyReport) int {
	if r.Summary.Error > 0 {
		return 1
	}
	return 0
}

// resolvedPolicy is the policy file's rules after defaults are applied. Booleans
// are plain (not pointers) so evaluation stays simple; the defaults are applied
// in parsePolicy. The baseline (an empty rules object) is fully permissive — an
// operator opts INTO each guardrail.
type resolvedPolicy struct {
	requireGroup            bool
	allowLanExposure        bool
	allowPrivilegedPorts    bool
	allowAutostart          bool
	forbidDuplicateBindings bool
}

// policyFileRaw is the on-disk policy JSON. Pointers distinguish an omitted
// field (apply the default) from an explicit value. schemaVersion is required.
type policyFileRaw struct {
	SchemaVersion *int           `json:"schemaVersion"`
	Rules         policyRulesRaw `json:"rules"`
}

// policyRulesRaw is the rules object of a policy file. Pointers let an omitted
// field fall back to its default in parsePolicy.
type policyRulesRaw struct {
	RequireGroup            *bool `json:"requireGroup"`
	AllowLanExposure        *bool `json:"allowLanExposure"`
	AllowPrivilegedPorts    *bool `json:"allowPrivilegedPorts"`
	AllowAutostart          *bool `json:"allowAutostart"`
	ForbidDuplicateBindings *bool `json:"forbidDuplicateBindings"`
}

// parsePolicy decodes and validates a policy file, applying defaults for omitted
// fields. Unknown fields are REJECTED (DisallowUnknownFields) so a typo'd rule
// name fails loudly rather than silently relaxing a guardrail. The supported
// schemaVersion is required and must equal policySchemaVersion. Defaults form a
// permissive baseline: requireGroup=false, allow*=true, forbidDuplicateBindings
// =false (so an empty policy permits everything; operators opt into restrictions).
func parsePolicy(data []byte) (resolvedPolicy, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return resolvedPolicy{}, fmt.Errorf("policy file is empty")
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var raw policyFileRaw
	if err := dec.Decode(&raw); err != nil {
		return resolvedPolicy{}, fmt.Errorf("not a valid policy file: %w", err)
	}
	if dec.More() {
		return resolvedPolicy{}, fmt.Errorf("policy file has trailing data after the JSON object")
	}

	if raw.SchemaVersion == nil {
		return resolvedPolicy{}, fmt.Errorf("policy file is missing the required \"schemaVersion\" field")
	}
	if *raw.SchemaVersion != policySchemaVersion {
		return resolvedPolicy{}, fmt.Errorf("unsupported policy schemaVersion %d (expected %d)", *raw.SchemaVersion, policySchemaVersion)
	}

	return resolvedPolicy{
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

// evaluatePolicy evaluates parsed config rules against a resolved policy and
// returns a deterministic report. Ordering is fixed: per-rule checks run in
// config file order, and for a single rule its findings appear in policy order
// (group → LAN exposure → privileged port → autostart). Duplicate-binding
// findings (which are inherently multi-rule) are emitted last, one per
// conflicting binding, sorted by protocol → listenHost → listenPort (reusing the
// config doctor's findDuplicateBindings so the two interpretations cannot drift).
// When nothing is violated, a single info "policy.valid" finding is returned.
// Pure and offline: it uses only the config and policy contents.
func evaluatePolicy(rules []rawConfigRule, pol resolvedPolicy) PolicyReport {
	findings := []PolicyFinding{}

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
		for _, b := range findDuplicateBindings(rules) {
			findings = append(findings, duplicateBindingFinding(b))
		}
	}

	if len(findings) == 0 {
		findings = append(findings, policyValidFinding(len(rules)))
	}
	return newPolicyReport(findings)
}

func groupRequiredFinding(r rawConfigRule) PolicyFinding {
	return PolicyFinding{
		Code:     policyGroupRequired,
		Severity: PolicyError,
		Title:    fmt.Sprintf("%s is missing a required group", ruleLabelName(r.Name)),
		Message:  "The policy requires every rule to belong to a group, but this rule has no group.",
		Details:  map[string]any{"rule": toRuleDetail(r)},
	}
}

func lanExposureFinding(r rawConfigRule) PolicyFinding {
	return PolicyFinding{
		Code:     policyLanExposureForbidden,
		Severity: PolicyError,
		Title:    fmt.Sprintf("%s listens on 0.0.0.0", ruleLabelName(r.Name)),
		Message:  "This rule listens on 0.0.0.0, but the policy forbids LAN exposure.",
		Details:  map[string]any{"rule": toRuleDetail(r)},
	}
}

func privilegedPortFinding(r rawConfigRule) PolicyFinding {
	return PolicyFinding{
		Code:     policyPrivilegedPortForbidden,
		Severity: PolicyError,
		Title:    fmt.Sprintf("%s uses privileged port %d", ruleLabelName(r.Name), r.ListenPort),
		Message:  "This rule listens on a privileged port (below 1024), but the policy forbids privileged ports.",
		Details:  map[string]any{"rule": toRuleDetail(r)},
	}
}

func autostartFinding(r rawConfigRule) PolicyFinding {
	return PolicyFinding{
		Code:     policyAutostartForbidden,
		Severity: PolicyError,
		Title:    fmt.Sprintf("%s has autostart enabled", ruleLabelName(r.Name)),
		Message:  "This rule is enabled for autostart, but the policy forbids autostart.",
		Details:  map[string]any{"rule": toRuleDetail(r)},
	}
}

func duplicateBindingFinding(b bindingConflict) PolicyFinding {
	return PolicyFinding{
		Code:     policyDuplicateBindingForbidden,
		Severity: PolicyError,
		Title:    fmt.Sprintf("Duplicate listen binding %s %s:%d", b.Protocol, b.ListenHost, b.ListenPort),
		Message:  fmt.Sprintf("%d rules share the listen binding %s %s:%d, but the policy forbids duplicate bindings.", len(b.Rules), b.Protocol, b.ListenHost, b.ListenPort),
		Details:  map[string]any{"binding": b},
	}
}

func policyValidFinding(ruleCount int) PolicyFinding {
	return PolicyFinding{
		Code:     policyValid,
		Severity: PolicyInfo,
		Title:    "Config complies with the policy",
		Message:  "The config satisfies all enabled policy rules.",
		Details:  map[string]any{"ruleCount": ruleCount},
	}
}

// policySeverityTag returns the fixed-width ASCII tag for a policy severity,
// matching the CLI's symbol-free human output style.
func policySeverityTag(s PolicySeverity) string {
	switch s {
	case PolicyInfo:
		return "[INFO]"
	case PolicyWarning:
		return "[WARN]"
	case PolicyError:
		return "[ERROR]"
	default:
		return "[?]"
	}
}

// printPolicyHuman renders a policy report in deterministic human-readable form.
func printPolicyHuman(r PolicyReport, w io.Writer) {
	fmt.Fprintln(w, "Portier Policy Check")
	fmt.Fprintln(w)

	for _, f := range r.Findings {
		fmt.Fprintf(w, "%-7s %s\n", policySeverityTag(f.Severity), f.Title)
		if f.Message != "" {
			fmt.Fprintf(w, "        %s\n", f.Message)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  %d info\n", r.Summary.Info)
	fmt.Fprintf(w, "  %d %s\n", r.Summary.Warning, pluralWord(r.Summary.Warning, "warning", "warnings"))
	fmt.Fprintf(w, "  %d %s\n", r.Summary.Error, pluralWord(r.Summary.Error, "error", "errors"))
	fmt.Fprintf(w, "\nResult: %s\n", r.Result)
}

const policyHelp = `Usage: portier policy <subcommand> [options]

Evaluate Portier config files against a small, offline policy file.

Subcommands:
  check --config <file> --policy <file>   Evaluate a config against a policy (read-only, offline)
  help                                    Show this help message

Run 'portier policy check --help' for check options.
`

const policyCheckHelp = `Usage: portier policy check --config <config-file> --policy <policy-file>

Evaluate a Portier config file against a JSON policy file. Fully offline:
never contacts the runtime, never probes targets, and never modifies any file.

Options:
  --config <file>   Path to the Portier config file to evaluate (required).
  --policy <file>   Path to the JSON policy file (required).

Policy file format (schemaVersion 1):
  {
    "schemaVersion": 1,
    "rules": {
      "requireGroup": false,
      "allowLanExposure": true,
      "allowPrivilegedPorts": true,
      "allowAutostart": true,
      "forbidDuplicateBindings": true
    }
  }

Policy rules (each omitted field falls back to the permissive default shown above):
  requireGroup             When true, every rule must have a non-empty group.
  allowLanExposure         When false, listening on 0.0.0.0 is a violation.
  allowPrivilegedPorts     When false, listen ports below 1024 are violations.
  allowAutostart           When false, autostart-enabled rules are violations.
  forbidDuplicateBindings  When true, duplicate protocol+host+port bindings are violations.

Unknown fields in the policy file are rejected.

Exit codes:
  0  Policy evaluation completed with no violations
  1  Policy evaluation completed with one or more violations
  2  Missing/invalid arguments, or an unreadable/malformed config or policy file
     (including an unsupported schemaVersion)

Examples:
  portier policy check --config portier.json --policy policy.json
  portier --json policy check --config portier.json --policy policy.json
`

// RunPolicy dispatches the `portier policy <subcommand>` commands. Policy
// evaluation is fully offline, so no management client is needed.
func RunPolicy(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, policyHelp)
		return 2
	}
	switch args[0] {
	case "check":
		return RunPolicyCheck(jsonOutput, args[1:], stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, policyHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown policy subcommand %q\n\n", args[0])
		fmt.Fprint(stderr, policyHelp)
		return 2
	}
}

// RunPolicyCheck evaluates a local config file against a local policy file and
// prints a deterministic policy report. It is fully offline: it never contacts
// the runtime, never probes targets, and never modifies any file. Exit codes:
// 0 = no violations, 1 = one or more violations (or a JSON-encode failure),
// 2 = usage error or an unreadable/malformed config or policy file.
func RunPolicyCheck(jsonOutput bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("policy check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flagConfig := fs.String("config", "", "path to the Portier config file")
	flagPolicy := fs.String("policy", "", "path to the policy file")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, policyCheckHelp)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fmt.Fprint(stderr, policyCheckHelp)
		return 2
	}

	if *flagConfig == "" {
		fmt.Fprintln(stderr, "Error: --config is required")
		fmt.Fprint(stderr, policyCheckHelp)
		return 2
	}
	if *flagPolicy == "" {
		fmt.Fprintln(stderr, "Error: --policy is required")
		fmt.Fprint(stderr, policyCheckHelp)
		return 2
	}

	configData, err := os.ReadFile(*flagConfig)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading config %s: %v\n", *flagConfig, err)
		return 2
	}
	rules, parseErr := parseLocalConfig(configData)
	if parseErr != nil {
		fmt.Fprintf(stderr, "Error: invalid config %s: %v\n", *flagConfig, parseErr)
		return 2
	}

	policyData, err := os.ReadFile(*flagPolicy)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading policy %s: %v\n", *flagPolicy, err)
		return 2
	}
	pol, polErr := parsePolicy(policyData)
	if polErr != nil {
		fmt.Fprintf(stderr, "Error: invalid policy %s: %v\n", *flagPolicy, polErr)
		return 2
	}

	report := evaluatePolicy(rules, pol)

	if jsonOutput {
		if err := output.PrintJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return policyExitCode(report)
	}

	printPolicyHuman(report, stdout)
	return policyExitCode(report)
}
