package domain

import "strings"

// RuleHealth is the operator-facing health classification for a rule. It is
// derived deterministically from existing runtime state — it does NOT probe
// targets or run any background check. Mirrors the TypeScript RuleHealth union;
// parity-tested via validate:contract.
type RuleHealth string

const (
	HealthHealthy RuleHealth = "healthy"
	HealthWarning RuleHealth = "warning"
	HealthError   RuleHealth = "error"
)

// DeriveRuleHealth classifies a rule's health from existing runtime state. Pure
// and deterministic; performs no I/O. Must stay identical to the TypeScript
// deriveRuleHealth. Priority: a present lastError is always "error"; otherwise
// an enabled-but-stopped rule is "warning"; everything else is "healthy".
func DeriveRuleHealth(enabled, running bool, lastError string) RuleHealth {
	if strings.TrimSpace(lastError) != "" {
		return HealthError
	}
	if enabled && !running {
		return HealthWarning
	}
	return HealthHealthy
}
