package doctor

import (
	"fmt"
	"os"

	"portier/cli/sources/config"
	"portier/cli/sources/output"
)

// Config doctor check codes. These are stable, operator-facing identifiers — do
// not rename them casually (they are a CLI/tool contract).
const (
	checkConfigReadFailed       = "config.read_failed"
	checkConfigParseFailed      = "config.parse_failed"
	checkConfigEmpty            = "config.empty"
	checkConfigValidationFailed = "config.validation_failed"
	checkConfigDuplicateBinding = "config.duplicate_binding"
	checkConfigLanExposure      = "config.lan_exposure"
	checkConfigPrivilegedPort   = "config.privileged_port"
	checkConfigValid            = "config.valid"
)

// RunConfigChecks performs the offline analysis of a config file and returns a
// deterministic doctor report plus a compact config summary. Read and parse
// failures short-circuit (no later checks are meaningful, and the summary is nil
// because no rules could be parsed); otherwise the summary is derived from the
// parsed rules and validation, duplicate-binding, emptiness, validity, and the
// LAN-exposure / privileged-port advisories run.
func RunConfigChecks(filePath string) (Report, *config.Summary) {
	checks := []CheckResult{}

	data, err := os.ReadFile(filePath)
	if err != nil {
		checks = append(checks, CheckResult{
			Code:     checkConfigReadFailed,
			Severity: Error,
			Title:    "Config file could not be read",
			Message:  fmt.Sprintf("Reading %s failed: %v", filePath, err),
			Details:  map[string]any{"path": filePath},
		})
		return NewReport(checks), nil
	}

	rules, parseErr := config.ParseLocal(data)
	if parseErr != nil {
		checks = append(checks, CheckResult{
			Code:     checkConfigParseFailed,
			Severity: Error,
			Title:    "Config file could not be parsed",
			Message:  parseErr.Error(),
			Details:  map[string]any{"path": filePath},
		})
		return NewReport(checks), nil
	}

	// The summary is available whenever parsing succeeded (including an empty
	// config or a config with validation errors).
	summary := config.BuildSummary(rules)

	if len(rules) == 0 {
		checks = append(checks, CheckResult{
			Code:     checkConfigEmpty,
			Severity: Warning,
			Title:    "Config contains no rules",
			Message:  "The config file parsed successfully but defines no forwarding rules.",
			Details:  map[string]any{"ruleCount": 0},
		})
		return NewReport(checks), &summary
	}

	// Field validation and duplicate bindings are reported separately so an
	// operator can distinguish a binding conflict from a malformed rule.
	vr := config.Validate(rules)
	dupErrs, fieldErrs := config.PartitionValidationErrors(vr.Errors)
	if len(fieldErrs) > 0 {
		checks = append(checks, CheckResult{
			Code:     checkConfigValidationFailed,
			Severity: Error,
			Title:    fmt.Sprintf("Config has %d validation %s", len(fieldErrs), output.PluralWord(len(fieldErrs), "error", "errors")),
			Message:  "One or more rules have invalid fields. Run 'portier config validate' for the full list.",
			Details:  map[string]any{"errors": fieldErrs, "ruleCount": len(rules)},
		})
	}
	if len(dupErrs) > 0 {
		checks = append(checks, CheckResult{
			Code:     checkConfigDuplicateBinding,
			Severity: Error,
			Title:    fmt.Sprintf("Config has %d duplicate listen %s", len(dupErrs), output.PluralWord(len(dupErrs), "binding", "bindings")),
			Message:  "Two or more rules share a listen binding (protocol + host + port). Each binding must be unique.",
			Details:  map[string]any{"errors": dupErrs, "bindings": config.FindDuplicateBindings(rules)},
		})
	}

	if vr.Valid {
		checks = append(checks, CheckResult{
			Code:     checkConfigValid,
			Severity: Info,
			Title:    "Config is valid",
			Message: fmt.Sprintf("The config file can be read, parsed, and validated (%d %s: %d TCP, %d UDP).",
				vr.RuleCount, output.PluralRule(vr.RuleCount), vr.TCPCount, vr.UDPCount),
			Details: map[string]any{"ruleCount": vr.RuleCount, "tcpCount": vr.TCPCount, "udpCount": vr.UDPCount},
		})
	}

	// Deterministic advisories run on every parsed rule (in file order),
	// independent of field validity, since exposure and port concerns are
	// meaningful even when other fields are wrong.
	checks = append(checks, configAdvisories(rules)...)

	return NewReport(checks), &summary
}

// configAdvisories returns the LAN-exposure and privileged-port advisory checks
// for the given rules. Each advisory aggregates ALL affected rules (in file
// order) into one check carrying a deterministic, structured `rules` list in its
// details — so the warning count stays one-per-advisory-kind and exit behavior is
// unchanged. LAN exposure stays the existing `0.0.0.0` check (kept consistent
// with the shared advisory; `::` is intentionally not added).
func configAdvisories(rules []config.Rule) []CheckResult {
	advisories := []CheckResult{}

	var exposed, privileged []config.RuleDetail
	for _, r := range rules {
		if r.ListenHost == "0.0.0.0" {
			exposed = append(exposed, config.ToRuleDetail(r))
		}
		if r.ListenPort >= 1 && r.ListenPort < 1024 {
			privileged = append(privileged, config.ToRuleDetail(r))
		}
	}

	if len(exposed) > 0 {
		advisories = append(advisories, CheckResult{
			Code:     checkConfigLanExposure,
			Severity: Warning,
			Title:    lanExposureTitle(exposed),
			Message:  "Listening on 0.0.0.0 exposes the forwarded port on all interfaces. Other LAN devices may be able to connect if firewall settings allow it.",
			Details:  map[string]any{"rules": exposed},
		})
	}
	if len(privileged) > 0 {
		advisories = append(advisories, CheckResult{
			Code:     checkConfigPrivilegedPort,
			Severity: Warning,
			Title:    privilegedPortTitle(privileged),
			Message:  "One or more rules listen on a privileged port (below 1024), which may require elevated permissions to bind.",
			Details:  map[string]any{"rules": privileged},
		})
	}
	return advisories
}

// lanExposureTitle renders the LAN-exposure check title, preserving the
// single-rule wording and summarizing when multiple rules are affected.
func lanExposureTitle(rules []config.RuleDetail) string {
	if len(rules) == 1 {
		return fmt.Sprintf("%s listens on 0.0.0.0", config.RuleLabelName(rules[0].Name))
	}
	return fmt.Sprintf("%d rules listen on 0.0.0.0", len(rules))
}

// privilegedPortTitle renders the privileged-port check title, preserving the
// single-rule wording and summarizing when multiple rules are affected.
func privilegedPortTitle(rules []config.RuleDetail) string {
	if len(rules) == 1 {
		return fmt.Sprintf("%s uses privileged port %d", config.RuleLabelName(rules[0].Name), rules[0].ListenPort)
	}
	return fmt.Sprintf("%d rules use privileged ports", len(rules))
}
