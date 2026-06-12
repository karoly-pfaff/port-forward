package domain

import "testing"

func TestDeriveRuleHealth(t *testing.T) {
	cases := []struct {
		name      string
		enabled   bool
		running   bool
		lastError string
		want      RuleHealth
	}{
		{"error wins when lastError present and stopped", false, false, "boom", HealthError},
		{"error wins even when running/enabled", true, true, "boom", HealthError},
		{"whitespace lastError is not an error", false, true, "   ", HealthHealthy},
		{"enabled but not running is warning", true, false, "", HealthWarning},
		{"running cleanly is healthy", true, true, "", HealthHealthy},
		{"running cleanly (disabled) is healthy", false, true, "", HealthHealthy},
		{"intentionally stopped is healthy", false, false, "", HealthHealthy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DeriveRuleHealth(c.enabled, c.running, c.lastError); got != c.want {
				t.Fatalf("DeriveRuleHealth(%v,%v,%q) = %q, want %q", c.enabled, c.running, c.lastError, got, c.want)
			}
		})
	}
}
