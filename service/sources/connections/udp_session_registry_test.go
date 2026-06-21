package connections_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"portier/service/sources/connections"
)

// --- helpers ---

func basicUDPInput(ruleID string) connections.UdpSessionInput {
	return connections.UdpSessionInput{
		RuleID:        ruleID,
		RuleName:      "Test Rule " + ruleID,
		Mode:          "one-way",
		ClientAddress: "127.0.0.1",
		ClientPort:    50000,
		TargetAddress: "127.0.0.1",
		TargetPort:    48000,
	}
}

func udpInputWithClient(ruleID, mode, clientAddress string, clientPort int) connections.UdpSessionInput {
	return connections.UdpSessionInput{
		RuleID:        ruleID,
		RuleName:      "Test Rule",
		Mode:          mode,
		ClientAddress: clientAddress,
		ClientPort:    clientPort,
		TargetAddress: "127.0.0.1",
		TargetPort:    48000,
	}
}

// --- empty state ---

func TestUdpSessionRegistryEmptySnapshotNotNil(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	snap := reg.Snapshot(time.Now())
	if snap == nil {
		t.Fatal("Snapshot() returned nil; want empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("Snapshot() len = %d; want 0", len(snap))
	}
}

func TestUdpSessionRegistryEmptySnapshotForRuleNotNil(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	snap := reg.SnapshotForRule("no-such-rule", time.Now())
	if snap == nil {
		t.Fatal("SnapshotForRule() returned nil; want empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("SnapshotForRule() len = %d; want 0", len(snap))
	}
}

// --- OpenOrTouchSession ---

func TestUdpSessionRegistryOpenCreatesSession(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id := reg.OpenOrTouchSession(basicUDPInput("r1"))
	if id == "" {
		t.Fatal("OpenOrTouchSession returned empty ID")
	}
	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("expected 1 session, got %d", len(snap))
	}
}

func TestUdpSessionRegistryOpenPreservesFields(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	input := connections.UdpSessionInput{
		RuleID:        "r1",
		RuleName:      "My Rule",
		Mode:          "bidirectional-last-client",
		ClientAddress: "10.0.0.1",
		ClientPort:    54321,
		TargetAddress: "192.168.1.1",
		TargetPort:    8080,
	}
	reg.OpenOrTouchSession(input)

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("expected 1 session, got %d", len(snap))
	}
	s := snap[0]
	if s.RuleID != "r1" {
		t.Errorf("RuleID = %q, want r1", s.RuleID)
	}
	if s.RuleName != "My Rule" {
		t.Errorf("RuleName = %q, want My Rule", s.RuleName)
	}
	if s.Mode != "bidirectional-last-client" {
		t.Errorf("Mode = %q, want bidirectional-last-client", s.Mode)
	}
	if s.ClientAddress != "10.0.0.1" {
		t.Errorf("ClientAddress = %q, want 10.0.0.1", s.ClientAddress)
	}
	if s.ClientPort != 54321 {
		t.Errorf("ClientPort = %d, want 54321", s.ClientPort)
	}
	if s.TargetAddress != "192.168.1.1" {
		t.Errorf("TargetAddress = %q, want 192.168.1.1", s.TargetAddress)
	}
	if s.TargetPort != 8080 {
		t.Errorf("TargetPort = %d, want 8080", s.TargetPort)
	}
	if s.Protocol != "udp" {
		t.Errorf("Protocol = %q, want udp", s.Protocol)
	}
}

func TestUdpSessionRegistryOpenReturnsSameIDForSameKey(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	input := basicUDPInput("r1")
	id1 := reg.OpenOrTouchSession(input)
	id2 := reg.OpenOrTouchSession(input)
	if id1 != id2 {
		t.Fatalf("expected same ID for same session key: %q vs %q", id1, id2)
	}
	// Only one session should exist
	if len(reg.Snapshot(time.Now())) != 1 {
		t.Fatal("duplicate session created for same key")
	}
}

func TestUdpSessionRegistryOpenTouchUpdatesLastSeenAt(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	input := basicUDPInput("r1")
	reg.OpenOrTouchSession(input)

	// Snapshot at idle+1s in future: should be idle (lastSeenAt was set at creation)
	idleFuture := time.Now().Add(connections.UDPSessionIdleDuration + time.Second)
	snap := reg.Snapshot(idleFuture)
	if len(snap) != 1 || snap[0].Status != "idle" {
		t.Fatalf("expected idle status, got %v", snap)
	}

	// Touch the session — updates lastSeenAt to ~now
	reg.OpenOrTouchSession(input)

	// Snapshot at idle+1s should now be idle again (since touch refreshed lastSeenAt to now)
	// and just-past-idle should show active
	justAfterTouch := time.Now().Add(time.Millisecond)
	snap2 := reg.Snapshot(justAfterTouch)
	if len(snap2) != 1 {
		t.Fatal("expected 1 session after touch")
	}
	if snap2[0].Status != "active" {
		t.Fatalf("after touch, status = %q, want active", snap2[0].Status)
	}
}

func TestUdpSessionRegistryOpenPreservesStartedAt(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	input := basicUDPInput("r1")
	reg.OpenOrTouchSession(input)

	snap1 := reg.Snapshot(time.Now())
	startedAt := snap1[0].StartedAt

	// Touch the session
	time.Sleep(5 * time.Millisecond)
	reg.OpenOrTouchSession(input)

	snap2 := reg.Snapshot(time.Now())
	if snap2[0].StartedAt != startedAt {
		t.Fatalf("startedAt changed on touch: %q vs %q", snap2[0].StartedAt, startedAt)
	}
}

// --- session key uniqueness ---

func TestUdpSessionRegistryDifferentClientsGetDifferentSessions(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id1 := reg.OpenOrTouchSession(udpInputWithClient("r1", "one-way", "127.0.0.1", 50000))
	id2 := reg.OpenOrTouchSession(udpInputWithClient("r1", "one-way", "127.0.0.1", 50001))
	if id1 == id2 {
		t.Fatal("different client ports should produce different sessions")
	}
	if len(reg.Snapshot(time.Now())) != 2 {
		t.Fatal("expected 2 sessions for 2 different clients")
	}
}

func TestUdpSessionRegistryDifferentRulesGetDifferentSessions(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id1 := reg.OpenOrTouchSession(udpInputWithClient("rule-a", "one-way", "127.0.0.1", 50000))
	id2 := reg.OpenOrTouchSession(udpInputWithClient("rule-b", "one-way", "127.0.0.1", 50000))
	if id1 == id2 {
		t.Fatal("different rules should produce different sessions even with same client")
	}
}

func TestUdpSessionRegistryDifferentModesGetDifferentSessions(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id1 := reg.OpenOrTouchSession(udpInputWithClient("r1", "one-way", "127.0.0.1", 50000))
	id2 := reg.OpenOrTouchSession(udpInputWithClient("r1", "bidirectional-last-client", "127.0.0.1", 50000))
	if id1 == id2 {
		t.Fatal("different modes should produce different sessions")
	}
}

// --- RecordInbound ---

func TestUdpSessionRegistryRecordInbound(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id := reg.OpenOrTouchSession(basicUDPInput("r1"))

	reg.RecordInbound(id, 100)
	reg.RecordInbound(id, 200)

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatal("expected 1 session")
	}
	if snap[0].PacketsIn != 2 {
		t.Fatalf("packetsIn = %d, want 2", snap[0].PacketsIn)
	}
	if snap[0].BytesIn != 300 {
		t.Fatalf("bytesIn = %d, want 300", snap[0].BytesIn)
	}
}

func TestUdpSessionRegistryRecordInboundUnknownIDIsNoop(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.RecordInbound("nonexistent", 100) // must not panic
	if len(reg.Snapshot(time.Now())) != 0 {
		t.Fatal("no-op RecordInbound must not create a session")
	}
}

// --- RecordOutbound ---

func TestUdpSessionRegistryRecordOutbound(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id := reg.OpenOrTouchSession(basicUDPInput("r1"))

	reg.RecordOutbound(id, 50)
	reg.RecordOutbound(id, 75)

	snap := reg.Snapshot(time.Now())
	if snap[0].PacketsOut != 2 {
		t.Fatalf("packetsOut = %d, want 2", snap[0].PacketsOut)
	}
	if snap[0].BytesOut != 125 {
		t.Fatalf("bytesOut = %d, want 125", snap[0].BytesOut)
	}
}

func TestUdpSessionRegistryRecordOutboundDoesNotUpdateLastSeenAt(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id := reg.OpenOrTouchSession(basicUDPInput("r1"))

	// Record inbound (sets lastSeenAt to ~now)
	reg.RecordInbound(id, 10)

	// Snapshot at just-past-idle: session was last seen at RecordInbound time (~now)
	// so it should still be active at idle+1ms in the future
	// RecordOutbound must NOT refresh lastSeenAt
	reg.RecordOutbound(id, 20)

	// Use a future time past the idle threshold
	futureNow := time.Now().Add(connections.UDPSessionIdleDuration + time.Second)
	snap := reg.Snapshot(futureNow)
	if len(snap) != 1 {
		t.Fatal("expected session to still exist (not expired)")
	}
	if snap[0].Status != "idle" {
		t.Fatalf("status = %q, want idle (RecordOutbound must not update lastSeenAt)", snap[0].Status)
	}
	if snap[0].PacketsOut != 1 || snap[0].BytesOut != 20 {
		t.Fatalf("outbound counters wrong: packetsOut=%d bytesOut=%d", snap[0].PacketsOut, snap[0].BytesOut)
	}
}

func TestUdpSessionRegistryRecordOutboundUnknownIDIsNoop(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.RecordOutbound("nonexistent", 100) // must not panic
}

// --- idle / status ---

func TestUdpSessionRegistryActiveStatusBeforeIdle(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))

	// 1 second in the future — well below the 30s idle threshold
	snap := reg.Snapshot(time.Now().Add(time.Second))
	if len(snap) != 1 || snap[0].Status != "active" {
		t.Fatalf("expected active status, got %v", snap)
	}
}

func TestUdpSessionRegistryIdleStatusAfterThreshold(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))

	// Just past the idle threshold
	snap := reg.Snapshot(time.Now().Add(connections.UDPSessionIdleDuration + time.Millisecond))
	if len(snap) != 1 || snap[0].Status != "idle" {
		t.Fatalf("expected idle status, got %v", snap)
	}
}

func TestUdpSessionRegistryIdleMsCalculation(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))

	future := time.Now().Add(10 * time.Second)
	snap := reg.Snapshot(future)
	if len(snap) != 1 {
		t.Fatal("expected 1 session")
	}
	// idleMs should be approximately 10000ms (between 9000 and 11000 for test robustness)
	if snap[0].IdleMs < 9000 || snap[0].IdleMs > 11000 {
		t.Fatalf("idleMs = %d, want ~10000", snap[0].IdleMs)
	}
}

// --- PruneExpired ---

func TestUdpSessionRegistryPruneExpiredRemovesOld(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))

	futureNow := time.Now().Add(connections.UDPSessionExpireDuration + time.Millisecond)
	reg.PruneExpired(futureNow)

	snap := reg.Snapshot(futureNow)
	if len(snap) != 0 {
		t.Fatalf("expected empty snapshot after pruning expired session, got %d", len(snap))
	}
}

func TestUdpSessionRegistryPruneExpiredKeepsRecent(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))

	// Prune at just past idle threshold — well before expire threshold
	partialFuture := time.Now().Add(connections.UDPSessionIdleDuration + time.Millisecond)
	reg.PruneExpired(partialFuture)

	snap := reg.Snapshot(partialFuture)
	if len(snap) != 1 {
		t.Fatalf("expected session to survive partial prune, got %d", len(snap))
	}
	if snap[0].Status != "idle" {
		t.Fatalf("status = %q, want idle after prune threshold passed", snap[0].Status)
	}
}

// PruneExpired must actually free memory, not merely hide sessions the way
// Snapshot does. Len() reflects retained entries, so it distinguishes reclaim
// from hiding.
func TestUdpSessionRegistryPruneExpiredReclaimsMemory(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))
	if reg.Len() != 1 {
		t.Fatalf("expected 1 retained entry, got %d", reg.Len())
	}

	// A recent session survives a prune well before the expire threshold.
	reg.PruneExpired(time.Now().Add(connections.UDPSessionIdleDuration + time.Millisecond))
	if reg.Len() != 1 {
		t.Fatalf("recent session must survive prune, retained = %d", reg.Len())
	}

	// Past the expire threshold the entry is freed (Snapshot alone would hide it).
	reg.PruneExpired(time.Now().Add(connections.UDPSessionExpireDuration + time.Millisecond))
	if reg.Len() != 0 {
		t.Fatalf("expired session must be reclaimed, retained = %d", reg.Len())
	}
}

// --- Snapshot hides expired ---

func TestUdpSessionRegistrySnapshotHidesExpiredWithoutPrune(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))

	futureNow := time.Now().Add(connections.UDPSessionExpireDuration + time.Millisecond)

	// Without calling PruneExpired, Snapshot should still hide the expired session
	snap := reg.Snapshot(futureNow)
	if len(snap) != 0 {
		t.Fatalf("Snapshot must hide expired sessions without explicit prune, got %d", len(snap))
	}
}

// --- SnapshotForRule ---

func TestUdpSessionRegistrySnapshotForRule(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("rule-a"))
	reg.OpenOrTouchSession(udpInputWithClient("rule-b", "one-way", "127.0.0.1", 50001))

	now := time.Now()
	snapA := reg.SnapshotForRule("rule-a", now)
	snapB := reg.SnapshotForRule("rule-b", now)
	snapC := reg.SnapshotForRule("rule-c", now)

	if len(snapA) != 1 || snapA[0].RuleID != "rule-a" {
		t.Fatalf("rule-a: expected 1 matching session, got %v", snapA)
	}
	if len(snapB) != 1 || snapB[0].RuleID != "rule-b" {
		t.Fatalf("rule-b: expected 1 matching session, got %v", snapB)
	}
	if snapC == nil || len(snapC) != 0 {
		t.Fatalf("rule-c: expected empty non-nil slice, got %v", snapC)
	}
}

func TestUdpSessionRegistrySnapshotForRuleFiltersExpired(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))

	futureNow := time.Now().Add(connections.UDPSessionExpireDuration + time.Millisecond)
	snap := reg.SnapshotForRule("r1", futureNow)
	if len(snap) != 0 {
		t.Fatalf("SnapshotForRule must hide expired sessions, got %d", len(snap))
	}
}

// --- CloseSession ---

func TestUdpSessionRegistryCloseSession(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id := reg.OpenOrTouchSession(basicUDPInput("r1"))

	reg.CloseSession(id)

	snap := reg.Snapshot(time.Now())
	if len(snap) != 0 {
		t.Fatalf("expected empty snapshot after CloseSession, got %d", len(snap))
	}
}

func TestUdpSessionRegistryCloseUnknownSessionIsNoop(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.CloseSession("nonexistent") // must not panic
}

func TestUdpSessionRegistryCloseSessionRemovesKeyForReuse(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	input := basicUDPInput("r1")
	id1 := reg.OpenOrTouchSession(input)

	reg.CloseSession(id1)

	// Opening the same logical session again should produce a new ID
	id2 := reg.OpenOrTouchSession(input)
	if id2 == id1 {
		t.Fatal("expected new session ID after close and reopen")
	}
}

// --- CloseSessionsForRule ---

func TestUdpSessionRegistryCloseSessionsForRule(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(udpInputWithClient("rule-a", "one-way", "127.0.0.1", 50000))
	reg.OpenOrTouchSession(udpInputWithClient("rule-a", "one-way", "127.0.0.1", 50001))
	reg.OpenOrTouchSession(udpInputWithClient("rule-b", "one-way", "127.0.0.1", 50000))

	reg.CloseSessionsForRule("rule-a")

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("expected 1 session (rule-b) after closing rule-a, got %d", len(snap))
	}
	if snap[0].RuleID != "rule-b" {
		t.Fatalf("remaining session should be rule-b, got %q", snap[0].RuleID)
	}
}

// --- multiple clients / multi-client mode ---

func TestUdpSessionRegistryMultipleClientsTrackedSeparately(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id1 := reg.OpenOrTouchSession(udpInputWithClient("r1", "bidirectional-multi-client", "127.0.0.1", 50000))
	id2 := reg.OpenOrTouchSession(udpInputWithClient("r1", "bidirectional-multi-client", "127.0.0.1", 50001))
	id3 := reg.OpenOrTouchSession(udpInputWithClient("r1", "bidirectional-multi-client", "127.0.0.1", 50002))

	if id1 == id2 || id2 == id3 || id1 == id3 {
		t.Fatal("each client should get a unique session ID")
	}

	reg.RecordInbound(id1, 100)
	reg.RecordInbound(id2, 200)
	reg.RecordOutbound(id3, 300)

	snap := reg.Snapshot(time.Now())
	if len(snap) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(snap))
	}
}

// --- snapshot is a copy ---

func TestUdpSessionRegistrySnapshotIsCopy(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id := reg.OpenOrTouchSession(basicUDPInput("r1"))

	snap1 := reg.Snapshot(time.Now())
	if len(snap1) != 1 {
		t.Fatal("expected 1 session")
	}

	// Mutate the snapshot
	snap1[0].PacketsIn = 99999

	// Original registry entry must be unaffected
	reg.RecordInbound(id, 10)
	snap2 := reg.Snapshot(time.Now())
	if snap2[0].PacketsIn == 99999 {
		t.Fatal("mutating snapshot slice must not affect registry internals")
	}
	if snap2[0].PacketsIn != 1 {
		t.Fatalf("expected packetsIn = 1, got %d", snap2[0].PacketsIn)
	}
}

// --- JSON shape ---

func TestUdpSessionRegistryJSONShape(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatal("expected 1 session")
	}

	data, err := json.Marshal(snap[0])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	required := []string{
		"id", "ruleId", "ruleName", "protocol", "mode",
		"clientAddress", "clientPort", "targetAddress", "targetPort",
		"startedAt", "lastSeenAt", "idleMs",
		"packetsIn", "packetsOut", "bytesIn", "bytesOut",
		"status",
	}
	for _, field := range required {
		if _, ok := m[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}
	if len(m) != len(required) {
		t.Errorf("unexpected field count: got %d, want %d; fields: %v", len(m), len(required), m)
	}
}

func TestUdpSessionRegistryProtocolIsAlwaysUDP(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))
	snap := reg.Snapshot(time.Now())
	if snap[0].Protocol != "udp" {
		t.Fatalf("protocol = %q, want udp", snap[0].Protocol)
	}
}

// --- ID uniqueness ---

func TestUdpSessionRegistryIDUniqueness(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		id := reg.OpenOrTouchSession(udpInputWithClient("r1", "one-way", "127.0.0.1", 50000+i))
		if seen[id] {
			t.Fatalf("duplicate session ID generated: %q", id)
		}
		seen[id] = true
	}
}

// --- no payload fields ---

func TestUdpSessionRegistryNoPayloadFields(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	reg.OpenOrTouchSession(basicUDPInput("r1"))
	reg.RecordInbound(reg.Snapshot(time.Now())[0].ID, 1024)

	snap := reg.Snapshot(time.Now())
	data, _ := json.Marshal(snap[0])
	raw := string(data)

	forbidden := []string{"payload", "data", "packet", "content", "body"}
	for _, f := range forbidden {
		for _, b := range []byte(raw) {
			_ = b
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		if _, ok := m[f]; ok {
			t.Errorf("snapshot must not contain field %q", f)
		}
	}
	// Verify bytesIn reflects count, not content
	if snap[0].BytesIn != 1024 {
		t.Fatalf("bytesIn = %d, want 1024", snap[0].BytesIn)
	}
}

// --- empty ruleName ---

func TestUdpSessionRegistryEmptyRuleName(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	input := basicUDPInput("r1")
	input.RuleName = ""
	reg.OpenOrTouchSession(input)

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatal("expected 1 session")
	}
	if snap[0].RuleName != "" {
		t.Fatalf("expected empty ruleName, got %q", snap[0].RuleName)
	}
}

// --- concurrency ---

func TestUdpSessionRegistryConcurrentRecordInboundOutbound(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id := reg.OpenOrTouchSession(basicUDPInput("r1"))

	const goroutines = 50
	const ops = 100
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				reg.RecordInbound(id, 1)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				reg.RecordOutbound(id, 1)
			}
		}()
	}
	wg.Wait()

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatal("expected 1 session after concurrent ops")
	}
	if snap[0].PacketsIn != goroutines*ops {
		t.Fatalf("packetsIn = %d, want %d", snap[0].PacketsIn, goroutines*ops)
	}
	if snap[0].PacketsOut != goroutines*ops {
		t.Fatalf("packetsOut = %d, want %d", snap[0].PacketsOut, goroutines*ops)
	}
}

func TestUdpSessionRegistryCloseWhileRecordingNoPanic(t *testing.T) {
	reg := connections.NewUdpSessionRegistry()
	id := reg.OpenOrTouchSession(basicUDPInput("r1"))

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			reg.RecordInbound(id, 1)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			reg.CloseSession(id)
			id = reg.OpenOrTouchSession(basicUDPInput("r1"))
		}
	}()
	wg.Wait()
	// No panic = pass
}
