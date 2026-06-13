package policy

import "portier/cli/sources/explain"

// explanations is the static registry of policy finding code explanations, keyed
// by the policy code constants so the strings cannot drift. Every policy finding
// code MUST have an entry here (guarded by policy_internal_test.go). The
// Explanation type and rendering helpers live in the shared explain package;
// this package owns only the policy explanation data. Explanations describe what
// a guardrail means and what an operator can do — they never claim that Portier
// enforces a policy or fixes a violation automatically, and they frame each
// finding as a policy choice rather than necessarily a product defect.
var explanations = map[string]explain.Explanation{
	codeValid: {
		Code:     codeValid,
		Title:    "Config complies with the policy",
		Meaning:  "The config satisfies every enabled policy guardrail — no violations were found. This is an informational result, not a forwarding diagnostic.",
		Action:   "No action needed.",
		Severity: "info",
	},
	codeGroupRequired: {
		Code:     codeGroupRequired,
		Title:    "Rule is missing a required group",
		Meaning:  "The policy's requireGroup guardrail expects every rule to belong to a non-empty group, but this rule has no group set. This is a policy choice, not a Portier defect.",
		Action:   "Add a group to the rule, or relax requireGroup in the policy file if grouping is not required.",
		Severity: "error",
	},
	codeLanExposureForbidden: {
		Code:     codeLanExposureForbidden,
		Title:    "LAN exposure is forbidden by the policy",
		Meaning:  "The policy's allowLanExposure guardrail is disabled and this rule listens on 0.0.0.0, which can expose the forwarded port beyond localhost to other devices on the network.",
		Action:   "Change the rule to listen on 127.0.0.1, or enable allowLanExposure in the policy if LAN exposure is intentional. Confirm firewall settings before exposing a port on the LAN.",
		Severity: "error",
		Related:  []string{codePrivilegedPortForbidden},
	},
	codePrivilegedPortForbidden: {
		Code:     codePrivilegedPortForbidden,
		Title:    "Privileged port is forbidden by the policy",
		Meaning:  "The policy's allowPrivilegedPorts guardrail is disabled and this rule listens on a privileged port (below 1024), which usually requires elevated permissions to bind.",
		Action:   "Use a listen port at or above 1024 (Portier recommends 48000-48999), or enable allowPrivilegedPorts in the policy if the low port is required.",
		Severity: "error",
		Related:  []string{codeLanExposureForbidden},
	},
	codeAutostartForbidden: {
		Code:     codeAutostartForbidden,
		Title:    "Autostart is forbidden by the policy",
		Meaning:  "The policy's allowAutostart guardrail is disabled and this rule is enabled for autostart, so forwarding would begin automatically without an explicit start.",
		Action:   "Disable autostart (set enabled to false) for the rule, or enable allowAutostart in the policy if autostart is intended.",
		Severity: "error",
	},
	codeDuplicateBindingForbidden: {
		Code:     codeDuplicateBindingForbidden,
		Title:    "Duplicate listen binding is forbidden by the policy",
		Meaning:  "The policy's forbidDuplicateBindings guardrail is enabled and two or more rules share the same listen binding (protocol + listen host + listen port).",
		Action:   "Change one of the conflicting rules so each listen binding is unique, or disable forbidDuplicateBindings in the policy if duplicates are acceptable.",
		Severity: "error",
	},
}

// Explanations returns the policy explanation registry. The `explain` command
// merges this with other domains' registries (e.g. doctor) for unified lookup
// and listing.
func Explanations() map[string]explain.Explanation {
	return explanations
}
