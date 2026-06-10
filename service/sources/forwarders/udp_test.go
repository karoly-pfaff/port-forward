package forwarders

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"portier/service/sources/activity"
	"portier/service/sources/connections"
	"portier/service/sources/domain"
)

const testSessionTimeout = 100 * time.Millisecond

// --- helpers ---

func freeTestUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve ephemeral UDP port: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return port
}

// startTestUDPEchoServer listens on an ephemeral UDP port and echoes every
// received datagram back to the sender, prepending prefix+":".
func startTestUDPEchoServer(t *testing.T, prefix string) (int, func()) {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve echo addr: %v", err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		t.Fatalf("start udp echo server: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	go func() {
		buf := make([]byte, 65536)
		for {
			n, remote, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			response := append([]byte(prefix+":"), buf[:n]...)
			_, _ = conn.WriteToUDP(response, remote)
		}
	}()
	return port, func() { conn.Close() }
}

// sendUDPPacket sends a single datagram from a new ephemeral local socket and
// returns that socket so the caller can read replies from it.
func sendUDPPacket(t *testing.T, targetPort int, payload []byte) *net.UDPConn {
	t.Helper()
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: targetPort})
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("send udp packet: %v", err)
	}
	return conn
}

// receiveUDPPacket reads one datagram from conn with a short deadline.
// Returns the data and ok=true, or "" and ok=false on timeout.
func receiveUDPPacket(t *testing.T, conn *net.UDPConn, timeout time.Duration) ([]byte, bool) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, false
	}
	return buf[:n], true
}

func testUDPRule(listenPort int, targetPort int, mode domain.UdpMode) domain.ForwardRule {
	return domain.ForwardRule{
		ID:         "udp-test",
		Name:       "UDP test",
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: listenPort,
		TargetHost: "127.0.0.1",
		TargetPort: targetPort,
		Enabled:    true,
		UdpMode:    &mode,
	}
}

// waitForUDPCondition polls condition until it is true or the deadline is
// reached. Using polling avoids goroutine coordination overhead in tests.
func waitForUDPCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

// --- one-way mode ---

func TestUDPOneWayForwardsPacketToTarget(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	forwarder, _ := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeOneWay),
			nil, nil, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	// Send a packet through the forwarder to the echo server.
	// The target will echo back to the forwarder's target socket (shared).
	// For one-way mode we just verify the packet reaches the target.
	// We confirm indirectly via stats: packetsIn must increment.
	client := sendUDPPacket(t, forwarder.rule.ListenPort, []byte("hello"))
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		return *forwarder.Status().PacketsIn >= 1
	})
	s := forwarder.Status()
	if *s.PacketsIn == 0 {
		t.Fatal("packetsIn should be non-zero after forwarded packet")
	}
	if s.BytesIn == 0 {
		t.Fatal("bytesIn should be non-zero")
	}
}

func TestUDPOneWayDoesNotReturnTargetResponse(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeOneWay),
			nil, nil, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	// Dial the forwarder listen port directly so we can check for a response.
	client := sendUDPPacket(t, listenPort, []byte("ping"))
	defer client.Close()

	// Wait briefly; one-way mode must not send anything back to the client.
	_, got := receiveUDPPacket(t, client, 150*time.Millisecond)
	if got {
		t.Fatal("one-way mode must not return target response to client")
	}
}

// --- bidirectional-last-client mode ---

func TestUDPLastClientReturnsTargetResponse(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalLast),
			nil, nil, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, listenPort, []byte("hello"))
	defer client.Close()

	data, ok := receiveUDPPacket(t, client, 500*time.Millisecond)
	if !ok {
		t.Fatal("expected reply from target via forwarder")
	}
	if string(data) != "echo:hello" {
		t.Fatalf("reply = %q, want %q", string(data), "echo:hello")
	}

	s := forwarder.Status()
	if *s.PacketsIn == 0 || *s.PacketsOut == 0 {
		t.Fatalf("packet counters: in=%d out=%d", *s.PacketsIn, *s.PacketsOut)
	}
	if s.BytesIn == 0 || s.BytesOut == 0 {
		t.Fatalf("byte counters: in=%d out=%d", s.BytesIn, s.BytesOut)
	}
}

// TestUDPLastClientTwoClientsDocumentedLimitation verifies the known limitation:
// with two clients, responses from the target go to the most recently seen
// client, not necessarily the one that sent the original packet.
func TestUDPLastClientTwoClientsDocumentedLimitation(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalLast),
			nil, nil, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	// Client A sends first, then client B sends.
	// After B's packet arrives, lastClient is B.
	// The echo response to A's packet might go to B (whichever is last).
	clientA := sendUDPPacket(t, listenPort, []byte("from-a"))
	defer clientA.Close()

	// Wait until A's packet has actually been forwarded (deadline poll, not a
	// fixed sleep) so the ordering this test documents is deterministic.
	waitForUDPCondition(t, func() bool {
		return forwarder.Status().PacketsIn != nil && *forwarder.Status().PacketsIn >= 1
	})

	clientB := sendUDPPacket(t, listenPort, []byte("from-b"))
	defer clientB.Close()

	// Both packets were forwarded; forwarder is running.
	waitForUDPCondition(t, func() bool {
		return *forwarder.Status().PacketsIn >= 2
	})

	// Responses went to whichever client was last — the exact routing is
	// non-deterministic from the test perspective. Verify no panic and
	// that both inbound packets were counted.
	s := forwarder.Status()
	if *s.PacketsIn < 2 {
		t.Fatalf("packetsIn = %d, want >= 2", *s.PacketsIn)
	}
}

// --- bidirectional-multi-client mode ---

func TestUDPMultiClientRouteResponseToCorrectClient(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalMulti),
			nil, nil, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	clientA := sendUDPPacket(t, listenPort, []byte("msgA"))
	defer clientA.Close()
	clientB := sendUDPPacket(t, listenPort, []byte("msgB"))
	defer clientB.Close()

	replyA, okA := receiveUDPPacket(t, clientA, 500*time.Millisecond)
	replyB, okB := receiveUDPPacket(t, clientB, 500*time.Millisecond)

	if !okA || !okB {
		t.Fatalf("both clients should receive replies: A=%v B=%v", okA, okB)
	}
	if string(replyA) != "echo:msgA" {
		t.Fatalf("client A reply = %q, want %q", string(replyA), "echo:msgA")
	}
	if string(replyB) != "echo:msgB" {
		t.Fatalf("client B reply = %q, want %q", string(replyB), "echo:msgB")
	}
}

func TestUDPMultiClientActiveSessionsCountsAndDecrementsOnTimeout(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalMulti),
			nil, nil, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	clientA := sendUDPPacket(t, listenPort, []byte("hello"))
	defer clientA.Close()
	clientB := sendUDPPacket(t, listenPort, []byte("world"))
	defer clientB.Close()

	waitForUDPCondition(t, func() bool {
		return *forwarder.Status().ActiveUdpSessions == 2
	})
	if s := forwarder.Status(); *s.ActiveUdpSessions != 2 {
		t.Fatalf("activeUdpSessions = %d, want 2", *s.ActiveUdpSessions)
	}

	// Wait for sessions to expire (idle timeout = testSessionTimeout).
	waitForUDPCondition(t, func() bool {
		return *forwarder.Status().ActiveUdpSessions == 0
	})
	if s := forwarder.Status(); *s.ActiveUdpSessions != 0 {
		t.Fatalf("after timeout activeUdpSessions = %d, want 0", *s.ActiveUdpSessions)
	}
}

func TestUDPMultiClientStatsUpdate(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalMulti),
			nil, nil, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, listenPort, []byte("stat-test"))
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		s := forwarder.Status()
		return *s.PacketsIn >= 1 && *s.PacketsOut >= 1
	})
	s := forwarder.Status()
	if *s.PacketsIn == 0 {
		t.Fatal("packetsIn should be non-zero")
	}
	if *s.PacketsOut == 0 {
		t.Fatal("packetsOut should be non-zero")
	}
	if s.BytesIn == 0 {
		t.Fatal("bytesIn should be non-zero")
	}
	if s.BytesOut == 0 {
		t.Fatal("bytesOut should be non-zero")
	}
}

// --- stop behavior ---

func TestUDPStopClosesListenerAndPreventsForwarding(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalLast),
			nil, nil, testSessionTimeout,
		)
	})

	// Verify it is working before stop.
	client := sendUDPPacket(t, listenPort, []byte("pre-stop"))
	defer client.Close()
	if _, ok := receiveUDPPacket(t, client, 500*time.Millisecond); !ok {
		t.Fatal("expected reply before stop")
	}

	forwarder.Stop()

	if forwarder.Status().Running {
		t.Fatal("status.Running should be false after Stop")
	}
	if forwarder.Status().StartedAt != "" {
		t.Fatal("status.StartedAt should be empty after Stop")
	}

	// Send a packet after stop; no reply should arrive.
	client2 := sendUDPPacket(t, listenPort, []byte("after-stop"))
	defer client2.Close()
	_, got := receiveUDPPacket(t, client2, 150*time.Millisecond)
	if got {
		t.Fatal("forwarder must not reply after Stop")
	}
}

func TestUDPMultiClientStopClearsSessionCount(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalMulti),
			nil, nil, testSessionTimeout,
		)
	})

	client := sendUDPPacket(t, listenPort, []byte("ping"))
	defer client.Close()
	waitForUDPCondition(t, func() bool {
		return *forwarder.Status().ActiveUdpSessions >= 1
	})

	forwarder.Stop()

	s := forwarder.Status()
	if s.Running {
		t.Fatal("running should be false after Stop")
	}
	if s.ActiveUdpSessions == nil || *s.ActiveUdpSessions != 0 {
		t.Fatalf("activeUdpSessions should be 0 after Stop, got %v", s.ActiveUdpSessions)
	}
}

func TestUDPStopIsIdempotent(t *testing.T) {
	forwarder, _ := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, freeTestUDPPort(t), domain.UdpModeOneWay),
			nil, nil, testSessionTimeout,
		)
	})
	forwarder.Stop()
	forwarder.Stop() // must not panic or deadlock
}

func TestUDPStartBindFailureReturnsError(t *testing.T) {
	// Reserve a UDP port, then try to start a forwarder on the same port.
	addr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	reserved, err := net.ListenUDP("udp4", addr)
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer reserved.Close()
	port := reserved.LocalAddr().(*net.UDPAddr).Port

	forwarder := NewUDPForwarderWithTimeout(
		testUDPRule(port, freeTestUDPPort(t), domain.UdpModeOneWay),
		nil, nil, testSessionTimeout,
	)
	if err := forwarder.Start(); err == nil {
		forwarder.Stop()
		t.Fatal("expected bind failure error")
	}
	if forwarder.Status().Running {
		t.Fatal("running should be false after bind failure")
	}
	if forwarder.Status().LastError == "" {
		t.Fatal("lastError should be set after bind failure")
	}
}

// --- UDP echo server sanity check ---

func TestUDPEchoServerSanity(t *testing.T) {
	port, stop := startTestUDPEchoServer(t, "srv")
	defer stop()

	conn := sendUDPPacket(t, port, []byte("test"))
	defer conn.Close()

	data, ok := receiveUDPPacket(t, conn, 500*time.Millisecond)
	if !ok {
		t.Fatal("no reply from echo server")
	}
	if string(data) != fmt.Sprintf("srv:test") {
		t.Fatalf("reply = %q", string(data))
	}
}

// --- activity event tests ---

func TestUDPForwarderEmitsPacketForwardedEvent(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	var mu sync.Mutex
	var events []activity.ActivityEventInput
	onEvent := func(e activity.ActivityEventInput) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalLast),
			nil, onEvent, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, listenPort, []byte("evt"))
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range events {
			if e.Type == activity.EventUDPPacketForwarded {
				return true
			}
		}
		return false
	})

	mu.Lock()
	defer mu.Unlock()
	var fwdEvent *activity.ActivityEventInput
	for i := range events {
		if events[i].Type == activity.EventUDPPacketForwarded {
			fwdEvent = &events[i]
			break
		}
	}
	if fwdEvent == nil {
		t.Fatal("udp.packet.forwarded event not emitted")
	}
	if fwdEvent.Protocol == nil || *fwdEvent.Protocol != "udp" {
		t.Fatalf("protocol = %v", fwdEvent.Protocol)
	}
}

func TestUDPForwarderEmitsPacketReturnedEvent(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	var mu sync.Mutex
	var events []activity.ActivityEventInput
	onEvent := func(e activity.ActivityEventInput) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalLast),
			nil, onEvent, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, listenPort, []byte("ret"))
	_, _ = receiveUDPPacket(t, client, 500*time.Millisecond)
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range events {
			if e.Type == activity.EventUDPPacketReturned {
				return true
			}
		}
		return false
	})

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, e := range events {
		if e.Type == activity.EventUDPPacketReturned {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("udp.packet.returned event not emitted")
	}
}

func TestUDPForwarderThrottlesPacketEvents(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	var forwardCount int64
	onEvent := func(e activity.ActivityEventInput) {
		if e.Type == activity.EventUDPPacketForwarded {
			atomic.AddInt64(&forwardCount, 1)
		}
	}

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeOneWay),
			nil, onEvent, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	// Send 20 packets rapidly (faster than 1-second throttle interval).
	for i := 0; i < 20; i++ {
		c := sendUDPPacket(t, listenPort, []byte("flood"))
		c.Close()
	}

	waitForUDPCondition(t, func() bool {
		return *forwarder.Status().PacketsIn >= 20
	})

	// All 20 packets were counted by stats.
	if *forwarder.Status().PacketsIn < 20 {
		t.Fatalf("packetsIn = %d, want >= 20", *forwarder.Status().PacketsIn)
	}
	// Activity events were throttled to far fewer than 20.
	got := atomic.LoadInt64(&forwardCount)
	if got >= 20 {
		t.Fatalf("expected throttled events (<20), got %d", got)
	}
}

func TestUDPMultiClientEmitsSessionOpenedAndClosed(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	var mu sync.Mutex
	var events []activity.ActivityEventInput
	onEvent := func(e activity.ActivityEventInput) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return NewUDPForwarderWithTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalMulti),
			nil, onEvent, testSessionTimeout,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, listenPort, []byte("hello"))
	defer client.Close()

	// Wait for session opened event.
	waitForUDPCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range events {
			if e.Type == activity.EventUDPSessionOpened {
				return true
			}
		}
		return false
	})

	// Wait for session to expire and closed event.
	waitForUDPCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range events {
			if e.Type == activity.EventUDPSessionClosed {
				return true
			}
		}
		return false
	})

	mu.Lock()
	defer mu.Unlock()
	hasOpened, hasClosed := false, false
	for _, e := range events {
		if e.Type == activity.EventUDPSessionOpened {
			hasOpened = true
		}
		if e.Type == activity.EventUDPSessionClosed {
			hasClosed = true
		}
	}
	if !hasOpened {
		t.Fatal("udp.session.opened event not emitted")
	}
	if !hasClosed {
		t.Fatal("udp.session.closed event not emitted")
	}
}

// --- registry integration tests ---

func TestUDPForwarderWithRegistryOneWayTracksSession(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, _ := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeOneWay),
			nil, nil, testSessionTimeout, reg,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, forwarder.rule.ListenPort, []byte("hello"))
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) >= 1
	})

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("expected 1 session, got %d", len(snap))
	}
	if snap[0].Protocol != "udp" {
		t.Fatalf("protocol = %q, want udp", snap[0].Protocol)
	}
	if snap[0].Mode != string(domain.UdpModeOneWay) {
		t.Fatalf("mode = %q, want one-way", snap[0].Mode)
	}
	if snap[0].PacketsIn == 0 {
		t.Fatal("packetsIn should be non-zero")
	}
}

func TestUDPForwarderWithRegistryOneWayRecordsBytesIn(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, _ := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeOneWay),
			nil, nil, testSessionTimeout, reg,
		)
	})
	defer forwarder.Stop()

	payload := []byte("test-payload-12345")
	client := sendUDPPacket(t, forwarder.rule.ListenPort, payload)
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		snap := reg.Snapshot(time.Now())
		return len(snap) >= 1 && snap[0].BytesIn >= int64(len(payload))
	})

	snap := reg.Snapshot(time.Now())
	if len(snap) == 0 {
		t.Fatal("expected session")
	}
	if snap[0].BytesIn < int64(len(payload)) {
		t.Fatalf("bytesIn = %d, want >= %d", snap[0].BytesIn, len(payload))
	}
}

func TestUDPForwarderWithRegistryLastClientCreatesSession(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, _ := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalLast),
			nil, nil, testSessionTimeout, reg,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, forwarder.rule.ListenPort, []byte("ping"))
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) >= 1
	})

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("expected 1 session, got %d", len(snap))
	}
	if snap[0].Mode != string(domain.UdpModeBidirectionalLast) {
		t.Fatalf("mode = %q, want bidirectional-last-client", snap[0].Mode)
	}
}

func TestUDPForwarderWithRegistryLastClientRecordsOutbound(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalLast),
			nil, nil, testSessionTimeout, reg,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, listenPort, []byte("hello"))
	defer client.Close()
	_, _ = receiveUDPPacket(t, client, 500*time.Millisecond)

	waitForUDPCondition(t, func() bool {
		snap := reg.Snapshot(time.Now())
		return len(snap) >= 1 && snap[0].PacketsOut >= 1
	})

	snap := reg.Snapshot(time.Now())
	if len(snap) == 0 {
		t.Fatal("expected session")
	}
	if snap[0].PacketsOut == 0 {
		t.Fatal("packetsOut should be non-zero after target response")
	}
	if snap[0].BytesOut == 0 {
		t.Fatal("bytesOut should be non-zero after target response")
	}
}

func TestUDPForwarderWithRegistryLastClientReplacesSession(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalLast),
			nil, nil, testSessionTimeout, reg,
		)
	})
	defer forwarder.Stop()

	clientA := sendUDPPacket(t, listenPort, []byte("from-a"))
	defer clientA.Close()

	waitForUDPCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) >= 1
	})
	firstSnap := reg.Snapshot(time.Now())
	if len(firstSnap) != 1 {
		t.Fatalf("expected 1 session after client A, got %d", len(firstSnap))
	}
	firstID := firstSnap[0].ID

	// Client B sends — should close A's session and open a new one
	clientB := sendUDPPacket(t, listenPort, []byte("from-b"))
	defer clientB.Close()

	waitForUDPCondition(t, func() bool {
		snap := reg.Snapshot(time.Now())
		return len(snap) == 1 && snap[0].ID != firstID
	})

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("expected 1 session after client B, got %d", len(snap))
	}
	if snap[0].ID == firstID {
		t.Fatal("expected new session ID for client B, got same as client A")
	}
}

func TestUDPForwarderWithRegistryMultiClientSeparateSessions(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalMulti),
			nil, nil, testSessionTimeout, reg,
		)
	})
	defer forwarder.Stop()

	clientA := sendUDPPacket(t, listenPort, []byte("msgA"))
	defer clientA.Close()
	clientB := sendUDPPacket(t, listenPort, []byte("msgB"))
	defer clientB.Close()

	waitForUDPCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) >= 2
	})

	snap := reg.Snapshot(time.Now())
	if len(snap) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(snap))
	}
	if snap[0].ID == snap[1].ID {
		t.Fatal("expected different session IDs for different clients")
	}
}

func TestUDPForwarderWithRegistryMultiClientSameClientSameSession(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalMulti),
			nil, nil, testSessionTimeout, reg,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, listenPort, []byte("first"))
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) >= 1
	})
	firstID := reg.Snapshot(time.Now())[0].ID

	if _, err := client.Write([]byte("second")); err != nil {
		t.Fatalf("second write: %v", err)
	}

	waitForUDPCondition(t, func() bool {
		snap := reg.Snapshot(time.Now())
		return len(snap) >= 1 && snap[0].PacketsIn >= 2
	})

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("expected 1 session (same client reuses session), got %d", len(snap))
	}
	if snap[0].ID != firstID {
		t.Fatal("session ID must not change on second packet from same client")
	}
	if snap[0].PacketsIn < 2 {
		t.Fatalf("packetsIn = %d, want >= 2", snap[0].PacketsIn)
	}
}

func TestUDPForwarderWithRegistryMultiClientOutbound(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalMulti),
			nil, nil, testSessionTimeout, reg,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, listenPort, []byte("hi"))
	defer client.Close()
	_, _ = receiveUDPPacket(t, client, 500*time.Millisecond)

	waitForUDPCondition(t, func() bool {
		snap := reg.Snapshot(time.Now())
		return len(snap) >= 1 && snap[0].PacketsOut >= 1
	})

	snap := reg.Snapshot(time.Now())
	if len(snap) == 0 {
		t.Fatal("expected session")
	}
	if snap[0].PacketsOut == 0 {
		t.Fatal("packetsOut should be non-zero after echo response")
	}
	if snap[0].BytesOut == 0 {
		t.Fatal("bytesOut should be non-zero after echo response")
	}
}

func TestUDPForwarderWithRegistryStopClearsSessions(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeOneWay),
			nil, nil, testSessionTimeout, reg,
		)
	})

	client := sendUDPPacket(t, listenPort, []byte("ping"))
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) >= 1
	})

	forwarder.Stop()

	if got := len(reg.Snapshot(time.Now())); got != 0 {
		t.Fatalf("expected 0 sessions after Stop, got %d", got)
	}
}

func TestUDPForwarderWithRegistrySessionTimeoutClosesRegistrySession(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeBidirectionalMulti),
			nil, nil, testSessionTimeout, reg,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, listenPort, []byte("hello"))
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) >= 1
	})

	// Wait for the session to be removed by expireSession (testSessionTimeout = 100ms)
	waitForUDPCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) == 0
	})
}

func TestUDPForwarderWithRegistryNoPayloadData(t *testing.T) {
	targetPort, stopTarget := startTestUDPEchoServer(t, "echo")
	defer stopTarget()

	reg := connections.NewUdpSessionRegistry()
	forwarder, listenPort := startUDPForwarderOnFreePort(t, func(p int) *UDPForwarder {
		return newUDPForwarderWithRegistryAndTimeout(
			testUDPRule(p, targetPort, domain.UdpModeOneWay),
			nil, nil, testSessionTimeout, reg,
		)
	})
	defer forwarder.Stop()

	client := sendUDPPacket(t, listenPort, []byte("secret-payload-data"))
	defer client.Close()

	waitForUDPCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) >= 1
	})

	snap := reg.Snapshot(time.Now())
	if len(snap) == 0 {
		t.Fatal("expected session")
	}

	data, err := json.Marshal(snap[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	if strings.Contains(raw, "secret-payload-data") {
		t.Fatal("session snapshot must not contain payload content")
	}
}
