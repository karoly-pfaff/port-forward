package manager

import (
	"errors"
	"strings"
	"syscall"
	"testing"

	"portier/service/sources/domain"
	"portier/service/sources/validation"
)

// Test-A stabilization (audits/v1.6-testing-audit-1.md): the free*Port helpers
// allocate an ephemeral port, close it, and return the number — a TOCTOU window
// in which another test or process can grab the port before the manager binds it
// in StartRule. Under heavy parallel load this surfaces as a transient EADDRINUSE
// failure. startRuleStable closes that gap by retrying the bind on a fresh port
// (and only on EADDRINUSE), exactly as the audit prescribes (Pattern C). It never
// retries whole tests and never retries on any other error.

const bindRetryAttempts = 10

// isAddrInUse reports whether err is an "address already in use" bind failure,
// cross-platform (POSIX EADDRINUSE and the Windows WSAEADDRINUSE message).
func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "only one usage of each socket address")
}

// freePortForProtocol allocates an ephemeral port of the protocol matching the
// rule (UDP vs TCP) using the existing per-protocol helpers.
func freePortForProtocol(t *testing.T, proto domain.ForwardProtocol) int {
	t.Helper()
	if proto == domain.ProtocolUDP {
		return freeUDPPort(t)
	}
	return freeTCPPort(t)
}

// startRuleStable starts rule.ID on m and returns the resulting status, retrying
// on a fresh listen port if the bind raced with another process (EADDRINUSE). On
// a rebind it updates both the manager (via UpdateRule) and rule.ListenPort, so
// callers that later dial rule.ListenPort remain correct. The replacement port
// protocol is derived from rule.Protocol. Use in place of a bare StartRule when
// the rule binds an ephemeral listen port from free*Port. Callers that don't
// need the status can ignore the return value.
func startRuleStable(t *testing.T, m *Manager, rule *domain.ForwardRule) domain.ForwardStatus {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < bindRetryAttempts; attempt++ {
		status, err := m.StartRule(rule.ID)
		if err == nil {
			return status
		}
		if !isAddrInUse(err) {
			t.Fatalf("StartRule %s: %v", rule.ID, err)
		}
		lastErr = err
		newPort := freePortForProtocol(t, rule.Protocol)
		if _, err := m.UpdateRule(rule.ID, validation.ForwardRulePatch{ListenPort: &newPort}); err != nil {
			t.Fatalf("rebind %s to free port %d: %v", rule.ID, newPort, err)
		}
		rule.ListenPort = newPort
	}
	t.Fatalf("could not start %s on a free listen port after %d attempts: %v", rule.ID, bindRetryAttempts, lastErr)
	return domain.ForwardStatus{}
}
