package doctor

import "portier/cli/sources/explain"

// explanations is the static registry mapping every stable doctor/check code to
// its explanation, keyed by the doctor code constants so the strings cannot
// drift. Every code the doctors can emit MUST have an entry here (guarded by
// explain_internal_test.go). The Explanation type and rendering helpers live in
// the shared explain package; this package owns only the doctor explanation data.
var explanations = map[string]explain.Explanation{
	// --- config doctor ---
	checkConfigReadFailed: {
		Code:     checkConfigReadFailed,
		Title:    "Config file could not be read",
		Meaning:  "The config file path could not be read — it may not exist, or file permissions prevented reading it.",
		Action:   "Check that the path is correct and readable, then run the doctor again.",
		Severity: "error",
		Related:  []string{checkConfigParseFailed},
	},
	checkConfigParseFailed: {
		Code:     checkConfigParseFailed,
		Title:    "Config file could not be parsed",
		Meaning:  "The file was read but is not valid Portier config JSON (a raw array, a {\"rules\": [...]} object, or an exported config).",
		Action:   "Fix the JSON so it matches a supported config shape, then re-run.",
		Severity: "error",
		Related:  []string{checkConfigReadFailed},
	},
	checkConfigEmpty: {
		Code:     checkConfigEmpty,
		Title:    "Config contains no rules",
		Meaning:  "The config parsed successfully but defines no forwarding rules.",
		Action:   "Add at least one forwarding rule if the config is expected to define forwards.",
		Severity: "warning",
		Related:  []string{checkConfigValid},
	},
	checkConfigValidationFailed: {
		Code:     checkConfigValidationFailed,
		Title:    "Config has invalid rules",
		Meaning:  "One or more rules have invalid fields — for example a missing name or host, an out-of-range port, or an invalid protocol or udpMode.",
		Action:   "Correct the reported fields. Run 'portier config validate <file>' for the full list of errors.",
		Severity: "error",
		Related:  []string{checkConfigDuplicateBinding},
	},
	checkConfigDuplicateBinding: {
		Code:     checkConfigDuplicateBinding,
		Title:    "Duplicate listen binding",
		Meaning:  "Two or more rules use the same protocol, listen host, and listen port.",
		Action:   "Change one of the conflicting rules so each active binding (protocol + listen host + listen port) is unique.",
		Severity: "error",
		Related:  []string{checkConfigValidationFailed},
	},
	checkConfigLanExposure: {
		Code:     checkConfigLanExposure,
		Title:    "Rule exposed on the LAN",
		Meaning:  "A rule listens on 0.0.0.0, which exposes the forwarded port on all network interfaces. Other devices on the LAN may be able to connect if the firewall allows it.",
		Action:   "If LAN access is not intended, change the listen host to 127.0.0.1. Otherwise confirm your firewall rules.",
		Severity: "warning",
		Related:  []string{checkConfigPrivilegedPort},
	},
	checkConfigPrivilegedPort: {
		Code:     checkConfigPrivilegedPort,
		Title:    "Rule uses a privileged port",
		Meaning:  "A rule listens on a privileged port (below 1024), which may require elevated permissions to bind.",
		Action:   "Use a port at or above 1024 (Portier recommends 48000-48999) unless the low port is required and permitted.",
		Severity: "warning",
		Related:  []string{checkConfigLanExposure},
	},
	checkConfigValid: {
		Code:     checkConfigValid,
		Title:    "Config is valid",
		Meaning:  "The config file can be read, parsed, and validated.",
		Action:   "No action needed.",
		Severity: "info",
		Related:  []string{checkConfigEmpty},
	},

	// --- live runtime doctor ---
	checkRuntimeReachable: {
		Code:     checkRuntimeReachable,
		Title:    "Runtime is reachable",
		Meaning:  "The Portier runtime responded to a request on the management API.",
		Action:   "No action needed.",
		Severity: "info",
		Related:  []string{checkRuntimeUnreachable},
	},
	checkRuntimeUnreachable: {
		Code:     checkRuntimeUnreachable,
		Title:    "Runtime is unreachable",
		Meaning:  "The CLI could not reach the Portier runtime at the configured management URL.",
		Action:   "Confirm the Portier service is running and that the host/port (or --url) is correct.",
		Severity: "error",
		Related:  []string{checkRuntimeReachable},
	},
	checkRuntimeVersion: {
		Code:     checkRuntimeVersion,
		Title:    "Runtime version",
		Meaning:  "Reports the runtime's version. When it differs from the CLI version this is a warning, not an error.",
		Action:   "Usually no action is needed. Align the CLI and runtime versions if you want them to match.",
		Severity: "info",
		Related:  []string{checkRuntimeReachable},
	},
	checkConfigRecoveryActive: {
		Code:     checkConfigRecoveryActive,
		Title:    "Configuration recovery mode is active",
		Meaning:  "The runtime started in config recovery mode: the persisted configuration could not be loaded (unreadable, malformed, or schema-invalid), so it is running with no active rules. The management API is reachable, but rule changes may be blocked until the configuration is repaired.",
		Action:   "Review the quarantined config file (its path is in the recovery details), then import or save a valid configuration. See docs/recovery.md.",
		Severity: "warning",
		Related:  []string{checkConfigExportRead},
	},
	checkRuntimeStatusRead: {
		Code:     checkRuntimeStatusRead,
		Title:    "Rule status read",
		Meaning:  "Rule status was read successfully from the runtime.",
		Action:   "No action needed.",
		Severity: "info",
		Related:  []string{checkRuntimeStatusFailed},
	},
	checkRuntimeStatusFailed: {
		Code:     checkRuntimeStatusFailed,
		Title:    "Rule status could not be read",
		Meaning:  "The runtime was reachable but reading rule status from the management API failed.",
		Action:   "Check the runtime logs; the service may be in a bad state. Retry after confirming the service is healthy.",
		Severity: "error",
		Related:  []string{checkRuntimeStatusRead},
	},
	checkRulesNone: {
		Code:     checkRulesNone,
		Title:    "No forwarding rules",
		Meaning:  "The runtime has no forwarding rules configured.",
		Action:   "Add a forwarding rule if traffic forwarding is expected.",
		Severity: "warning",
		Related:  []string{checkRulesPresent},
	},
	checkRulesPresent: {
		Code:     checkRulesPresent,
		Title:    "Forwarding rules configured",
		Meaning:  "The runtime reported one or more configured forwarding rules.",
		Action:   "No action needed.",
		Severity: "info",
		Related:  []string{checkRulesNone, checkRulesHealthOK},
	},
	checkRulesHealthOK: {
		Code:     checkRulesHealthOK,
		Title:    "All rules healthy",
		Meaning:  "No rule reports warning or error health.",
		Action:   "No action needed.",
		Severity: "info",
		Related:  []string{checkRulesHealthWarning, checkRulesHealthError},
	},
	checkRulesHealthWarning: {
		Code:     checkRulesHealthWarning,
		Title:    "Rules in warning health",
		Meaning:  "One or more rules report warning health. A rule reads as warning when it is enabled for autostart but is not currently running.",
		Action:   "Start the affected rules, or disable autostart if they are intentionally stopped. Use 'portier status' to see which rules are affected.",
		Severity: "warning",
		Related:  []string{checkRulesHealthError, checkRulesHealthOK},
	},
	checkRulesHealthError: {
		Code:     checkRulesHealthError,
		Title:    "Rules in error health",
		Meaning:  "One or more rules report error health, which means a failed start or a socket error was recorded.",
		Action:   "Run 'portier diagnose <id|name>' on the affected rules to see the underlying error.",
		Severity: "error",
		Related:  []string{checkRulesHealthWarning},
	},
	checkConfigExportRead: {
		Code:     checkConfigExportRead,
		Title:    "Config is readable",
		Meaning:  "The current runtime configuration was read successfully (read-only; nothing was written).",
		Action:   "No action needed.",
		Severity: "info",
		Related:  []string{checkConfigExportFailed},
	},
	checkConfigExportFailed: {
		Code:     checkConfigExportFailed,
		Title:    "Config could not be read",
		Meaning:  "Reading the current configuration from the runtime failed.",
		Action:   "Check the runtime logs and confirm the service is healthy, then retry.",
		Severity: "error",
		Related:  []string{checkConfigExportRead},
	},
}

// Explanations returns the doctor explanation registry (config-doctor + live
// doctor codes). The `explain` command merges this with other domains'
// registries (e.g. policy) for unified lookup and listing.
func Explanations() map[string]explain.Explanation {
	return explanations
}

// SortedExplanations returns the doctor explanations in deterministic
// code-sorted order. Used by the support bundle's doctor-scoped explanations.json.
func SortedExplanations() []explain.Explanation {
	return explain.Sorted(explanations)
}
