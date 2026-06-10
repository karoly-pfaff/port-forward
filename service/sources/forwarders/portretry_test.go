package forwarders

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

// Test-A stabilization (audits/v1.6-testing-audit-1.md): the free*Port helpers
// allocate an ephemeral port, close it, and return the number — a TOCTOU window
// in which another test or process can grab the port before the forwarder binds
// it. Under heavy parallel load this surfaces as a transient EADDRINUSE failure.
//
// These helpers close that gap by retrying the *bind itself* on a fresh port
// when (and only when) the failure is EADDRINUSE. They never retry on any other
// error, and they never retry whole tests — only the allocate→bind step, exactly
// as the audit prescribes (Pattern C). A partially-built forwarder is stopped
// before the next attempt so no listener leaks between retries.

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

// startTCPForwarderOnFreePort builds a TCP forwarder via mk bound to a fresh
// ephemeral listen port and starts it, retrying with a new port if the chosen
// port was taken between allocation and bind. It returns the started forwarder
// and the listen port it successfully bound.
func startTCPForwarderOnFreePort(t *testing.T, mk func(listenPort int) *TCPForwarder) (*TCPForwarder, int) {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < bindRetryAttempts; attempt++ {
		port := freeTestTCPPort(t)
		f := mk(port)
		err := f.Start()
		if err == nil {
			return f, port
		}
		f.Stop()
		if !isAddrInUse(err) {
			t.Fatalf("start TCP forwarder on port %d: %v", port, err)
		}
		lastErr = err
	}
	t.Fatalf("could not bind a free TCP listen port after %d attempts: %v", bindRetryAttempts, lastErr)
	return nil, 0
}

// startUDPForwarderOnFreePort builds a UDP forwarder via mk bound to a fresh
// ephemeral listen port and starts it, retrying with a new port on EADDRINUSE.
// It returns the started forwarder and the listen port it successfully bound.
func startUDPForwarderOnFreePort(t *testing.T, mk func(listenPort int) *UDPForwarder) (*UDPForwarder, int) {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < bindRetryAttempts; attempt++ {
		port := freeTestUDPPort(t)
		f := mk(port)
		err := f.Start()
		if err == nil {
			return f, port
		}
		f.Stop()
		if !isAddrInUse(err) {
			t.Fatalf("start UDP forwarder on port %d: %v", port, err)
		}
		lastErr = err
	}
	t.Fatalf("could not bind a free UDP listen port after %d attempts: %v", bindRetryAttempts, lastErr)
	return nil, 0
}
