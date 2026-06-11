package api

import (
	"testing"

	"portier/service/sources/domain"
)

// These lock the exact diagnose check order/set, which the other diagnose tests
// (HTTP-level, find-by-id) do not guard. isRunning=true skips the listen-bind
// socket; the order is the same regardless of individual check statuses.

func diagOrderingRule(protocol domain.ForwardProtocol) domain.ForwardRule {
	return domain.ForwardRule{
		ID:         "order",
		Name:       "Order",
		Protocol:   protocol,
		ListenHost: "127.0.0.1",
		ListenPort: 48201,
		TargetHost: "127.0.0.1",
		TargetPort: 9,
		Enabled:    true,
	}
}

func diagCheckIDs(result domain.RuleDiagnosticsResult) []string {
	ids := make([]string, len(result.Checks))
	for i, c := range result.Checks {
		ids[i] = c.ID
	}
	return ids
}

func diagOrderEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDiagnoseCheckOrderingTCP(t *testing.T) {
	result := diagnoseRule(diagOrderingRule(domain.ProtocolTCP), true)
	want := []string{"listen-host", "lan-exposure", "privileged-port", "common-port", "listen-bind", "target-host", "target-connect"}
	if got := diagCheckIDs(result); !diagOrderEqual(got, want) {
		t.Errorf("TCP check order = %v, want %v", got, want)
	}
}

func TestDiagnoseCheckOrderingUDP(t *testing.T) {
	result := diagnoseRule(diagOrderingRule(domain.ProtocolUDP), true)
	want := []string{"listen-host", "lan-exposure", "privileged-port", "common-port", "listen-bind", "target-host", "target-connect", "udp-mode"}
	if got := diagCheckIDs(result); !diagOrderEqual(got, want) {
		t.Errorf("UDP check order = %v, want %v", got, want)
	}
}
