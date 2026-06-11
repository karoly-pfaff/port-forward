package forwarders

import (
	"testing"

	"portier/service/sources/connections"
	"portier/service/sources/domain"
)

// The factory is the one new behavior boundary introduced by the Forwarder
// abstraction: it must dispatch to the protocol-appropriate forwarder and return
// nil for an unrecognized protocol. These checks observe the dispatch through the
// returned forwarder's status shape (TCP exposes ActiveConnections, UDP exposes
// PacketsIn/PacketsOut) rather than asserting concrete types, and bind no sockets
// (Status works before Start).

func TestNewForwarderTCPDispatch(t *testing.T) {
	tcpReg := connections.NewTcpConnectionRegistry()
	udpReg := connections.NewUdpSessionRegistry()
	rule := testTCPRule(48010, 48011)

	f := NewForwarder(rule, nil, nil, tcpReg, udpReg)
	if f == nil {
		t.Fatal("NewForwarder returned nil for a TCP rule")
	}

	status := f.Status()
	if status.RuleID != rule.ID {
		t.Errorf("status RuleID = %q, want %q", status.RuleID, rule.ID)
	}
	if status.ActiveConnections == nil {
		t.Error("TCP forwarder status should expose ActiveConnections (TCP shape)")
	}
	if status.PacketsIn != nil {
		t.Error("TCP forwarder status should not expose PacketsIn")
	}
}

func TestNewForwarderUDPDispatch(t *testing.T) {
	tcpReg := connections.NewTcpConnectionRegistry()
	udpReg := connections.NewUdpSessionRegistry()
	rule := testUDPRule(48012, 48013, domain.UdpModeOneWay)

	f := NewForwarder(rule, nil, nil, tcpReg, udpReg)
	if f == nil {
		t.Fatal("NewForwarder returned nil for a UDP rule")
	}

	status := f.Status()
	if status.RuleID != rule.ID {
		t.Errorf("status RuleID = %q, want %q", status.RuleID, rule.ID)
	}
	if status.PacketsIn == nil || status.PacketsOut == nil {
		t.Error("UDP forwarder status should expose PacketsIn/PacketsOut (UDP shape)")
	}
	if status.ActiveConnections != nil {
		t.Error("UDP forwarder status should not expose ActiveConnections")
	}
}

func TestNewForwarderUnknownProtocolReturnsNil(t *testing.T) {
	rule := testTCPRule(48014, 48015)
	rule.Protocol = domain.ForwardProtocol("sctp")

	f := NewForwarder(rule, nil, nil, nil, nil)
	if f != nil {
		t.Errorf("NewForwarder should return nil for an unrecognized protocol, got %T", f)
	}
}
