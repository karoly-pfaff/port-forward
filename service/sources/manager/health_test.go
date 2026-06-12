package manager

import (
	"net"
	"testing"

	"portier/service/sources/domain"
)

func healthOf(t *testing.T, m *Manager, ruleID string) domain.RuleHealth {
	t.Helper()
	for _, s := range m.ListStatus() {
		if s.RuleID == ruleID {
			return s.Health
		}
	}
	t.Fatalf("no status for rule %s", ruleID)
	return ""
}

func TestStatusHealthHealthyWhenDisabledStopped(t *testing.T) {
	rule := tcpRule()
	rule.Enabled = false
	m := testManager(t, []domain.ForwardRule{rule})
	defer m.StopAll()
	if got := healthOf(t, m, rule.ID); got != domain.HealthHealthy {
		t.Fatalf("health = %q, want healthy", got)
	}
}

func TestStatusHealthWarningWhenEnabledStopped(t *testing.T) {
	rule := tcpRule()
	rule.Enabled = true // enabled but never started → expected-to-run drift
	m := testManager(t, []domain.ForwardRule{rule})
	defer m.StopAll()
	if got := healthOf(t, m, rule.ID); got != domain.HealthWarning {
		t.Fatalf("health = %q, want warning", got)
	}
}

func TestStatusHealthHealthyWhenRunning(t *testing.T) {
	rule := tcpRule()
	rule.Enabled = true
	m := testManager(t, []domain.ForwardRule{rule})
	defer m.StopAll()
	startRuleStable(t, m, &rule)
	if got := healthOf(t, m, rule.ID); got != domain.HealthHealthy {
		t.Fatalf("health = %q, want healthy", got)
	}
}

func TestStatusHealthErrorWhenStartFails(t *testing.T) {
	// Occupy a port so the rule's start fails and records a lastError.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	occupied := ln.Addr().(*net.TCPAddr).Port

	rule := tcpRule()
	rule.Enabled = false
	rule.ListenPort = occupied
	m := testManager(t, []domain.ForwardRule{rule})
	defer m.StopAll()

	if _, err := m.StartRule(rule.ID); err == nil {
		t.Fatal("expected StartRule to fail on an occupied port")
	}
	if got := healthOf(t, m, rule.ID); got != domain.HealthError {
		t.Fatalf("health = %q, want error", got)
	}
}
