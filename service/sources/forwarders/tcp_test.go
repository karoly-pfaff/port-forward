package forwarders

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"portier/service/sources/activity"
	"portier/service/sources/connections"
	"portier/service/sources/domain"
)

func TestTCPForwarderForwardsBothDirectionsAndTracksStatus(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "direct")
	defer stopTarget()

	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarder(rule, nil, nil)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)
	if _, err := conn.Write([]byte("hello\n")); err != nil {
		t.Fatalf("write through forwarder: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read through forwarder: %v", err)
	}
	if line != "direct:hello\n" {
		t.Fatalf("line = %q", line)
	}

	waitForTestCondition(t, func() bool {
		s := forwarder.Status()
		return s.BytesIn > 0 && s.BytesOut > 0
	})
	status := forwarder.Status()
	if !status.Running || status.ActiveConnections == nil || *status.ActiveConnections != 1 {
		t.Fatalf("status while connected = %#v", status)
	}
	if status.BytesIn == 0 || status.BytesOut == 0 {
		t.Fatalf("bytes were not tracked: %#v", status)
	}

	_ = conn.Close()
	waitForTestCondition(t, func() bool {
		active := forwarder.Status().ActiveConnections
		return active != nil && *active == 0
	})
}

func TestTCPForwarderStopClosesListener(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "stop")
	defer stopTarget()

	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarder(rule, nil, nil)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	forwarder.Stop()

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", rule.ListenPort)), 100*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected stopped listener to reject new connections")
	}
}

func TestTCPForwarderRecordsTargetConnectError(t *testing.T) {
	rule := testTCPRule(freeTestTCPPort(t), freeTestTCPPort(t))
	forwarder := NewTCPForwarder(rule, nil, nil)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)
	_, _ = conn.Write([]byte("hello\n"))
	_ = conn.Close()

	waitForTestCondition(t, func() bool {
		return forwarder.Status().LastError != ""
	})
}

func TestTCPForwarderEmitsConnectionOpenedEvent(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "evt")
	defer stopTarget()

	var mu sync.Mutex
	var events []activity.ActivityEventInput
	onEvent := func(e activity.ActivityEventInput) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarder(rule, nil, onEvent)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)
	defer conn.Close()

	waitForTestCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range events {
			if e.Type == activity.EventTCPConnectionOpened {
				return true
			}
		}
		return false
	})

	mu.Lock()
	defer mu.Unlock()
	var opened *activity.ActivityEventInput
	for i := range events {
		if events[i].Type == activity.EventTCPConnectionOpened {
			opened = &events[i]
			break
		}
	}
	if opened == nil {
		t.Fatal("tcp.connection.opened event not emitted")
	}
	if opened.RuleID == nil || *opened.RuleID != rule.ID {
		t.Fatalf("ruleId = %v", opened.RuleID)
	}
	if opened.Protocol == nil || *opened.Protocol != "tcp" {
		t.Fatalf("protocol = %v", opened.Protocol)
	}
	if opened.Details == nil || opened.Details["remoteAddress"] == nil {
		t.Fatalf("details missing remoteAddress: %v", opened.Details)
	}
}

func TestTCPForwarderEmitsConnectionClosedEvent(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "cls")
	defer stopTarget()

	var mu sync.Mutex
	var events []activity.ActivityEventInput
	onEvent := func(e activity.ActivityEventInput) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarder(rule, nil, onEvent)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)
	_, _ = conn.Write([]byte("hello\n"))
	_, _ = bufio.NewReader(conn).ReadString('\n')
	_ = conn.Close()

	waitForTestCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range events {
			if e.Type == activity.EventTCPConnectionClosed {
				return true
			}
		}
		return false
	})

	mu.Lock()
	defer mu.Unlock()
	var closed *activity.ActivityEventInput
	for i := range events {
		if events[i].Type == activity.EventTCPConnectionClosed {
			closed = &events[i]
			break
		}
	}
	if closed == nil {
		t.Fatal("tcp.connection.closed event not emitted")
	}
	if closed.Severity != activity.SeverityInfo {
		t.Fatalf("severity = %q, want info", closed.Severity)
	}
}

func TestTCPForwarderEmitsConnectionErrorNotClosedOnTargetFail(t *testing.T) {
	// Target port is closed — dial will fail, should emit error but NOT closed.
	rule := testTCPRule(freeTestTCPPort(t), freeTestTCPPort(t))

	var mu sync.Mutex
	var events []activity.ActivityEventInput
	onEvent := func(e activity.ActivityEventInput) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	forwarder := NewTCPForwarder(rule, nil, onEvent)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)
	_ = conn.Close()

	waitForTestCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range events {
			if e.Type == activity.EventTCPConnectionError {
				return true
			}
		}
		return false
	})

	mu.Lock()
	defer mu.Unlock()
	for _, e := range events {
		if e.Type == activity.EventTCPConnectionClosed {
			t.Fatal("tcp.connection.closed must not be emitted when an error was already emitted")
		}
	}
}

func testTCPRule(listenPort int, targetPort int) domain.ForwardRule {
	return domain.ForwardRule{
		ID:         "tcp-test",
		Name:       "TCP test",
		Protocol:   domain.ProtocolTCP,
		ListenHost: "127.0.0.1",
		ListenPort: listenPort,
		TargetHost: "127.0.0.1",
		TargetPort: targetPort,
		Enabled:    true,
	}
}

func freeTestTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func startTestEchoServer(t *testing.T, prefix string) (int, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start echo server: %v", err)
	}
	stop := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							return
						}
						return
					}
					_, _ = conn.Write([]byte(prefix + ":" + line))
				}
			}(conn)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, func() {
		close(stop)
		_ = listener.Close()
	}
}

func dialTestTCP(t *testing.T, port int) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port)), time.Second)
	if err != nil {
		t.Fatalf("dial tcp forwarder: %v", err)
	}
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	return conn
}

func waitForTestCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

// --- Registry integration tests ---

func TestTCPForwarderWithRegistryTracksConnection(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "reg1")
	defer stopTarget()

	reg := connections.NewTcpConnectionRegistry()
	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarderWithRegistry(rule, nil, nil, reg)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)
	defer conn.Close()

	waitForTestCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) == 1
	})

	snap := reg.Snapshot(time.Now())
	if snap[0].RuleID != rule.ID {
		t.Errorf("RuleID = %q; want %q", snap[0].RuleID, rule.ID)
	}
}

func TestTCPForwarderWithRegistryTracksBytesIn(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "bin")
	defer stopTarget()

	reg := connections.NewTcpConnectionRegistry()
	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarderWithRegistry(rule, nil, nil, reg)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)
	defer conn.Close()

	waitForTestCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) == 1
	})

	_, _ = conn.Write([]byte("hello\n"))

	waitForTestCondition(t, func() bool {
		snap := reg.Snapshot(time.Now())
		return len(snap) == 1 && snap[0].BytesIn > 0
	})

	snap := reg.Snapshot(time.Now())
	if snap[0].BytesIn == 0 {
		t.Fatal("BytesIn was not tracked after client write")
	}
}

func TestTCPForwarderWithRegistryTracksBytesOut(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "bout")
	defer stopTarget()

	reg := connections.NewTcpConnectionRegistry()
	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarderWithRegistry(rule, nil, nil, reg)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)
	defer conn.Close()

	waitForTestCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) == 1
	})

	_, _ = conn.Write([]byte("hello\n"))
	_, _ = bufio.NewReader(conn).ReadString('\n') // read echo to ensure reply has been sent

	waitForTestCondition(t, func() bool {
		snap := reg.Snapshot(time.Now())
		return len(snap) == 1 && snap[0].BytesOut > 0
	})

	snap := reg.Snapshot(time.Now())
	if snap[0].BytesOut == 0 {
		t.Fatal("BytesOut was not tracked after target echo")
	}
}

func TestTCPForwarderWithRegistryClosesOnDisconnect(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "cls")
	defer stopTarget()

	reg := connections.NewTcpConnectionRegistry()
	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarderWithRegistry(rule, nil, nil, reg)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)

	waitForTestCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) == 1
	})

	_ = conn.Close()

	waitForTestCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) == 0
	})
}

func TestTCPForwarderWithRegistryNoLeakOnTargetFail(t *testing.T) {
	// Target port is not listening — dial will fail.
	rule := testTCPRule(freeTestTCPPort(t), freeTestTCPPort(t))

	reg := connections.NewTcpConnectionRegistry()
	forwarder := NewTCPForwarderWithRegistry(rule, nil, nil, reg)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)
	defer conn.Close()

	// Entry may appear briefly; must be removed after target dial failure.
	waitForTestCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) == 0
	})
}

func TestTCPForwarderWithRegistryClearsOnStop(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "stop")
	defer stopTarget()

	reg := connections.NewTcpConnectionRegistry()
	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarderWithRegistry(rule, nil, nil, reg)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	conn := dialTestTCP(t, rule.ListenPort)
	defer conn.Close()

	waitForTestCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) == 1
	})

	forwarder.Stop()

	if len(reg.Snapshot(time.Now())) != 0 {
		t.Fatal("registry should be empty after Stop()")
	}
}

func TestTCPForwarderWithRegistryTracksMultipleConnections(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "multi")
	defer stopTarget()

	reg := connections.NewTcpConnectionRegistry()
	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarderWithRegistry(rule, nil, nil, reg)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer forwarder.Stop()

	conn1 := dialTestTCP(t, rule.ListenPort)
	defer conn1.Close()
	conn2 := dialTestTCP(t, rule.ListenPort)
	defer conn2.Close()

	waitForTestCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) == 2
	})

	snap := reg.Snapshot(time.Now())
	if snap[0].ID == snap[1].ID {
		t.Fatal("multiple connections should have distinct registry IDs")
	}
}

func TestTCPForwarderWithRegistrySnapshotHasNoPayloadFields(t *testing.T) {
	targetPort, stopTarget := startTestEchoServer(t, "shape")
	defer stopTarget()

	reg := connections.NewTcpConnectionRegistry()
	rule := testTCPRule(freeTestTCPPort(t), targetPort)
	forwarder := NewTCPForwarderWithRegistry(rule, nil, nil, reg)
	if err := forwarder.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer forwarder.Stop()

	conn := dialTestTCP(t, rule.ListenPort)
	defer conn.Close()

	waitForTestCondition(t, func() bool {
		return len(reg.Snapshot(time.Now())) == 1
	})

	snap := reg.Snapshot(time.Now())
	data, err := json.Marshal(snap[0])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	expected := []string{
		"id", "ruleId", "ruleName", "protocol",
		"clientAddress", "clientPort", "targetAddress", "targetPort",
		"startedAt", "durationMs", "bytesIn", "bytesOut", "status",
	}
	if len(m) != len(expected) {
		t.Errorf("JSON has %d fields; want %d (no payload fields)", len(m), len(expected))
	}
	for _, key := range expected {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON missing field %q", key)
		}
	}
}
