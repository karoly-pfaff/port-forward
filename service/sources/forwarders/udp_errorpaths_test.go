package forwarders

// Coverage Slice D — UDP forwarder constructor and mode-default paths. These
// cover the public NewUDPForwarder constructor (used by the manager but not
// previously exercised directly) and the udpMode nil-default branch (a rule with
// no UdpMode set must behave as one-way). Socket binds go through the Test-A
// startUDPForwarderOnFreePort retry helper; no fixed sleeps.

import (
	"testing"

	"portier/service/sources/domain"
)

// TestNewUDPForwarder_StartsOneWayByDefault exercises the public NewUDPForwarder
// constructor (default session timeout, no registry) and the udpMode() nil
// branch: a rule with UdpMode == nil must start as one-way.
func TestNewUDPForwarder_StartsWithNilModeAsOneWay(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		rule := testUDPRule(p, targetPort, domain.UdpModeOneWay)
		rule.UdpMode = nil // force the udpMode() nil-default branch
		return NewUDPForwarder(rule, nil, nil)
	})
	defer forwarder.Stop()

	if mode := forwarder.udpMode(); mode != domain.UdpModeOneWay {
		t.Fatalf("udpMode() = %q, want one-way for nil UdpMode", mode)
	}

	client := sendUDPPacket(t, listenPort, []byte("hello"))
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		return forwarder.Status().PacketsIn != nil && *forwarder.Status().PacketsIn >= 1
	})
}

// TestNewUDPForwarder_NilModeStatusShape confirms a nil-mode forwarder reports a
// one-way status shape (no activeUdpSessions) when stopped.
func TestNewUDPForwarder_NilModeStatusShape(t *testing.T) {
	rule := testUDPRule(0, 0, domain.UdpModeOneWay)
	rule.UdpMode = nil
	forwarder := NewUDPForwarder(rule, nil, nil)

	status := forwarder.Status()
	if status.ActiveUdpSessions != nil {
		t.Fatalf("one-way (nil-mode) status must not carry activeUdpSessions, got %v", *status.ActiveUdpSessions)
	}
}
