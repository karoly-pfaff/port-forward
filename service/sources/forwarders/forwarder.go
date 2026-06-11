package forwarders

import (
	"portier/service/sources/activity"
	"portier/service/sources/connections"
	"portier/service/sources/domain"
)

// Forwarder is the common lifecycle surface shared by the TCP and UDP
// forwarders. It lets the manager start, stop, and inspect a rule's runtime
// without carrying protocol-specific construction or pointer-juggling detail.
// Both *TCPForwarder and *UDPForwarder already satisfy it; the interface adds
// no new behavior, only a shared shape.
type Forwarder interface {
	// Start binds the listener and begins forwarding, or returns the bind error.
	Start() error
	// Stop shuts the forwarder down. It is idempotent (guarded by sync.Once in
	// both implementations) and bounded.
	Stop()
	// Status reports the current runtime status (running, bytes, lastError, etc.).
	Status() domain.ForwardStatus
}

// NewForwarder builds the protocol-appropriate Forwarder for rule, wiring the
// matching live-connection registry (TCP connections / UDP sessions). It returns
// nil for an unrecognized protocol, leaving that case to the caller — this keeps
// the protocol switch in one place instead of duplicated across the manager.
func NewForwarder(
	rule domain.ForwardRule,
	log LogFunc,
	onEvent activity.EventFunc,
	tcpReg *connections.TcpConnectionRegistry,
	udpReg *connections.UdpSessionRegistry,
) Forwarder {
	switch rule.Protocol {
	case domain.ProtocolTCP:
		return NewTCPForwarderWithRegistry(rule, log, onEvent, tcpReg)
	case domain.ProtocolUDP:
		return NewUDPForwarderWithRegistry(rule, log, onEvent, udpReg)
	default:
		return nil
	}
}
