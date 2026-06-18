package doctor

import (
	"fmt"

	"portier/cli/sources/client"
	"portier/cli/sources/output"
	"portier/cli/sources/version"
)

// Live doctor check codes. These are stable, operator-facing identifiers — do
// not rename them casually (they are a CLI/tool contract). They share the doctor
// model and naming style with the offline config doctor.
const (
	checkRuntimeReachable     = "runtime.reachable"
	checkRuntimeUnreachable   = "runtime.unreachable"
	checkRuntimeVersion       = "runtime.version"
	checkConfigRecoveryActive = "config.recovery_active"
	checkRuntimeStatusRead    = "runtime.status_read"
	checkRuntimeStatusFailed  = "runtime.status_failed"
	checkRulesNone            = "rules.none"
	checkRulesPresent         = "rules.present"
	checkRulesHealthOK        = "rules.health_ok"
	checkRulesHealthWarning   = "rules.health_warning"
	checkRulesHealthError     = "rules.health_error"
	checkConfigExportRead     = "config.export_read"
	checkConfigExportFailed   = "config.export_failed"
)

// RunLiveChecks performs the live runtime analysis and returns a deterministic
// doctor report. An unreachable runtime short-circuits (later checks need
// runtime data); otherwise version, status/health, and a read-only config-export
// check run in a fixed order. The runtime API is the source of truth for rule
// health — the CLI never re-derives it.
func RunLiveChecks(c *client.Client) Report {
	checks := []CheckResult{}
	baseURL := c.BaseURL()

	info, err := c.GetRuntime()
	if err != nil {
		checks = append(checks, runtimeUnreachableCheck(baseURL, err))
		return NewReport(checks)
	}
	checks = append(checks, runtimeReachableCheck(baseURL))
	checks = append(checks, runtimeVersionCheck(info))
	if check, ok := recoveryActiveCheck(info); ok {
		checks = append(checks, check)
	}

	if statuses, statusErr := c.GetStatus(); statusErr != nil {
		checks = append(checks, statusFailedCheck(statusErr))
	} else {
		checks = append(checks, statusReadCheck(len(statuses)))
		checks = append(checks, rulesHealthChecks(statuses)...)
	}

	if cfg, cfgErr := c.ExportConfig(); cfgErr != nil {
		checks = append(checks, configExportFailedCheck(cfgErr))
	} else {
		checks = append(checks, configExportReadCheck(len(cfg.Rules)))
	}

	return NewReport(checks)
}

func runtimeUnreachableCheck(baseURL string, err error) CheckResult {
	return CheckResult{
		Code:     checkRuntimeUnreachable,
		Severity: Error,
		Title:    "Runtime is unreachable",
		Message:  fmt.Sprintf("Could not reach the Portier runtime: %v. Is the Portier service running?", err),
		Details:  map[string]any{"url": baseURL},
	}
}

func runtimeReachableCheck(baseURL string) CheckResult {
	return CheckResult{
		Code:     checkRuntimeReachable,
		Severity: Info,
		Title:    "Runtime is reachable",
		Message:  "The Portier runtime responded successfully.",
		Details:  map[string]any{"url": baseURL},
	}
}

// runtimeVersionCheck reports the runtime version, warning (never failing) when
// it differs from the CLI version.
func runtimeVersionCheck(info *client.RuntimeInfo) CheckResult {
	cliVersion := version.Version
	runtimeVersion := info.Version
	details := map[string]any{
		"runtimeVersion": runtimeVersion,
		"cliVersion":     cliVersion,
		"runtime":        info.Runtime,
	}
	if runtimeVersion != "" && cliVersion != "" && runtimeVersion != cliVersion {
		return CheckResult{
			Code:     checkRuntimeVersion,
			Severity: Warning,
			Title:    fmt.Sprintf("Runtime version %s differs from CLI version %s", runtimeVersion, cliVersion),
			Message:  "The runtime and CLI are different versions. This is usually fine, but matching versions are recommended.",
			Details:  details,
		}
	}
	return CheckResult{
		Code:     checkRuntimeVersion,
		Severity: Info,
		Title:    fmt.Sprintf("Runtime version: %s", runtimeVersion),
		Message:  "The Portier runtime reported its version.",
		Details:  details,
	}
}

// recoveryActiveCheck reports an active config-recovery state (v1.17). It emits a
// finding ONLY when recovery is active; a normal runtime adds no check (so the
// doctor's healthy output is unchanged). Severity is Warning: the management API
// is up and serving, but the persisted config did not load and writes may be
// blocked until it is repaired. Details carry the safe reason and quarantine path
// (already part of local diagnostics); no file contents are exposed.
func recoveryActiveCheck(info *client.RuntimeInfo) (CheckResult, bool) {
	if info.Recovery == nil || !info.Recovery.Active {
		return CheckResult{}, false
	}
	r := info.Recovery
	details := map[string]any{"reason": r.Reason, "writesBlocked": r.WritesBlocked}
	if r.ConfigPath != "" {
		details["configPath"] = r.ConfigPath
	}
	if r.QuarantinePath != "" {
		details["quarantinePath"] = r.QuarantinePath
	}
	message := "Configuration recovery mode is active: the management API is available but the persisted configuration could not be loaded"
	if r.WritesBlocked {
		message += ", and rule changes are blocked until it is repaired"
	}
	message += ". Review the quarantined config and import/save a valid configuration (see docs/recovery.md)."
	return CheckResult{
		Code:     checkConfigRecoveryActive,
		Severity: Warning,
		Title:    "Configuration recovery mode is active",
		Message:  message,
		Details:  details,
	}, true
}

func statusFailedCheck(err error) CheckResult {
	return CheckResult{
		Code:     checkRuntimeStatusFailed,
		Severity: Error,
		Title:    "Could not read rule status",
		Message:  fmt.Sprintf("Reading rule status from the runtime failed: %v", err),
	}
}

func statusReadCheck(ruleCount int) CheckResult {
	return CheckResult{
		Code:     checkRuntimeStatusRead,
		Severity: Info,
		Title:    "Rule status read",
		Message:  "Rule status was read from the runtime.",
		Details:  map[string]any{"ruleCount": ruleCount},
	}
}

// rulesHealthChecks summarizes rule presence and health from the status list.
// Health values come straight from the API's `health` field — the CLI never
// re-derives health. Order is deterministic: presence, then error, then warning,
// then the all-clear, so a mixed fleet reports both health_error and
// health_warning (matching the runtime's own classification).
func rulesHealthChecks(statuses []client.ForwardStatus) []CheckResult {
	total := len(statuses)
	if total == 0 {
		return []CheckResult{{
			Code:     checkRulesNone,
			Severity: Warning,
			Title:    "No forwarding rules configured",
			Message:  "The runtime has no forwarding rules. Add a rule to start forwarding traffic.",
		}}
	}

	var healthy, warning, errored int
	for _, s := range statuses {
		switch s.Health {
		case "error":
			errored++
		case "warning":
			warning++
		case "healthy":
			healthy++
		}
	}

	checks := []CheckResult{{
		Code:     checkRulesPresent,
		Severity: Info,
		Title:    fmt.Sprintf("%d forwarding %s configured", total, output.PluralRule(total)),
		Message:  "The runtime reported status for the configured forwarding rules.",
		Details: map[string]any{
			"total":   total,
			"healthy": healthy,
			"warning": warning,
			"error":   errored,
		},
	}}

	if errored > 0 {
		checks = append(checks, CheckResult{
			Code:     checkRulesHealthError,
			Severity: Error,
			Title:    fmt.Sprintf("%d %s in error health", errored, output.PluralRule(errored)),
			Message:  "One or more rules report error health (a failed start or socket error). Run 'portier status' or 'portier diagnose <id|name>' for detail.",
			Details:  map[string]any{"count": errored},
		})
	}
	if warning > 0 {
		checks = append(checks, CheckResult{
			Code:     checkRulesHealthWarning,
			Severity: Warning,
			Title:    fmt.Sprintf("%d %s in warning health", warning, output.PluralRule(warning)),
			Message:  "One or more rules report warning health (enabled for autostart but not running).",
			Details:  map[string]any{"count": warning},
		})
	}
	if errored == 0 && warning == 0 {
		checks = append(checks, CheckResult{
			Code:     checkRulesHealthOK,
			Severity: Info,
			Title:    "All rules report healthy",
			Message:  "No rule reports warning or error health.",
			Details:  map[string]any{"total": total},
		})
	}

	return checks
}

func configExportReadCheck(ruleCount int) CheckResult {
	return CheckResult{
		Code:     checkConfigExportRead,
		Severity: Info,
		Title:    "Config is readable",
		Message:  "The current configuration was read successfully (read-only; nothing was written).",
		Details:  map[string]any{"ruleCount": ruleCount},
	}
}

func configExportFailedCheck(err error) CheckResult {
	return CheckResult{
		Code:     checkConfigExportFailed,
		Severity: Error,
		Title:    "Config could not be read",
		Message:  fmt.Sprintf("Reading the current configuration from the runtime failed: %v", err),
	}
}
