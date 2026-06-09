package connections_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"portier/service/sources/connections"
)

func TestTcpConnectionRegistryEmptySnapshot(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	snap := reg.Snapshot(time.Now())
	if snap == nil {
		t.Fatal("Snapshot() returned nil; want empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("Snapshot() len = %d; want 0", len(snap))
	}
}

func TestTcpConnectionRegistryOpenConnectionReturnsID(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	if id == "" {
		t.Fatal("OpenConnection returned empty ID")
	}
}

func TestTcpConnectionRegistryOpenConnectionCreatesEntry(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("Snapshot len = %d; want 1", len(snap))
	}
}

func TestTcpConnectionRegistryOpenConnectionFields(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id := reg.OpenConnection(connections.TcpConnectionInput{
		RuleID:        "rule-abc",
		RuleName:      "My Rule",
		ClientAddress: "127.0.0.1",
		ClientPort:    55000,
		TargetAddress: "10.10.10.10",
		TargetPort:    9090,
	})

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("Snapshot len = %d; want 1", len(snap))
	}
	info := snap[0]

	if info.ID != id {
		t.Errorf("ID = %q; want %q", info.ID, id)
	}
	if info.RuleID != "rule-abc" {
		t.Errorf("RuleID = %q; want rule-abc", info.RuleID)
	}
	if info.RuleName != "My Rule" {
		t.Errorf("RuleName = %q; want My Rule", info.RuleName)
	}
	if info.Protocol != "tcp" {
		t.Errorf("Protocol = %q; want tcp", info.Protocol)
	}
	if info.ClientAddress != "127.0.0.1" {
		t.Errorf("ClientAddress = %q; want 127.0.0.1", info.ClientAddress)
	}
	if info.ClientPort != 55000 {
		t.Errorf("ClientPort = %d; want 55000", info.ClientPort)
	}
	if info.TargetAddress != "10.10.10.10" {
		t.Errorf("TargetAddress = %q; want 10.10.10.10", info.TargetAddress)
	}
	if info.TargetPort != 9090 {
		t.Errorf("TargetPort = %d; want 9090", info.TargetPort)
	}
	if info.Status != "active" {
		t.Errorf("Status = %q; want active", info.Status)
	}
	if info.BytesIn != 0 {
		t.Errorf("initial BytesIn = %d; want 0", info.BytesIn)
	}
	if info.BytesOut != 0 {
		t.Errorf("initial BytesOut = %d; want 0", info.BytesOut)
	}
	if info.StartedAt == "" {
		t.Error("StartedAt is empty")
	}
}

func TestTcpConnectionRegistryDurationMs(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	time.Sleep(10 * time.Millisecond)
	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("Snapshot len = %d; want 1", len(snap))
	}
	if snap[0].DurationMs < 0 {
		t.Errorf("DurationMs = %d; want >= 0", snap[0].DurationMs)
	}
}

func TestTcpConnectionRegistrySnapshotIsCopy(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	snap1 := reg.Snapshot(time.Now())
	snap1[0].BytesIn = 99999 // mutate the local copy
	reg.AddBytesIn(id, 500)
	snap2 := reg.Snapshot(time.Now())
	if snap2[0].BytesIn != 500 {
		t.Errorf("BytesIn = %d after AddBytesIn(500); snap mutation leaked into registry", snap2[0].BytesIn)
	}
}

func TestTcpConnectionRegistryAddBytesIn(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	reg.AddBytesIn(id, 100)
	snap := reg.Snapshot(time.Now())
	if snap[0].BytesIn != 100 {
		t.Errorf("BytesIn = %d; want 100", snap[0].BytesIn)
	}
}

func TestTcpConnectionRegistryAddBytesInAccumulates(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	reg.AddBytesIn(id, 100)
	reg.AddBytesIn(id, 200)
	reg.AddBytesIn(id, 300)
	snap := reg.Snapshot(time.Now())
	if snap[0].BytesIn != 600 {
		t.Errorf("BytesIn = %d; want 600", snap[0].BytesIn)
	}
}

func TestTcpConnectionRegistryAddBytesInUnknownIDIsNoop(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	reg.AddBytesIn("nonexistent-id", 100) // must not panic
	snap := reg.Snapshot(time.Now())
	if len(snap) != 0 {
		t.Fatalf("Snapshot len = %d; want 0", len(snap))
	}
}

func TestTcpConnectionRegistryAddBytesOut(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	reg.AddBytesOut(id, 250)
	snap := reg.Snapshot(time.Now())
	if snap[0].BytesOut != 250 {
		t.Errorf("BytesOut = %d; want 250", snap[0].BytesOut)
	}
}

func TestTcpConnectionRegistryAddBytesOutAccumulates(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	reg.AddBytesOut(id, 50)
	reg.AddBytesOut(id, 75)
	snap := reg.Snapshot(time.Now())
	if snap[0].BytesOut != 125 {
		t.Errorf("BytesOut = %d; want 125", snap[0].BytesOut)
	}
}

func TestTcpConnectionRegistryAddBytesOutUnknownIDIsNoop(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	reg.AddBytesOut("nonexistent-id", 100) // must not panic
	snap := reg.Snapshot(time.Now())
	if len(snap) != 0 {
		t.Fatalf("Snapshot len = %d; want 0", len(snap))
	}
}

func TestTcpConnectionRegistryCloseConnection(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	reg.CloseConnection(id)
	snap := reg.Snapshot(time.Now())
	if len(snap) != 0 {
		t.Fatalf("Snapshot len = %d; want 0 after close", len(snap))
	}
}

func TestTcpConnectionRegistryCloseUnknownIDIsNoop(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	reg.CloseConnection("nonexistent-id") // must not panic or remove existing
	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("Snapshot len = %d; want 1 after close of unknown ID", len(snap))
	}
}

func TestTcpConnectionRegistryMultipleConnectionsTrackedSeparately(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id1 := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1", ClientPort: 10001})
	id2 := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1", ClientPort: 10002})
	id3 := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r2", ClientPort: 10003})

	if id1 == id2 || id1 == id3 || id2 == id3 {
		t.Fatalf("IDs must be unique: %q %q %q", id1, id2, id3)
	}

	snap := reg.Snapshot(time.Now())
	if len(snap) != 3 {
		t.Fatalf("Snapshot len = %d; want 3", len(snap))
	}
}

func TestTcpConnectionRegistryCloseConnectionsForRule(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1", ClientPort: 10001})
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1", ClientPort: 10002})
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r2", ClientPort: 10003})

	reg.CloseConnectionsForRule("r1")
	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("Snapshot len = %d; want 1 after CloseConnectionsForRule(r1)", len(snap))
	}
	if snap[0].RuleID != "r2" {
		t.Errorf("remaining entry RuleID = %q; want r2", snap[0].RuleID)
	}
}

func TestTcpConnectionRegistryCloseConnectionsForRuleUnknownIsNoop(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	reg.CloseConnectionsForRule("nonexistent-rule") // must not panic or remove existing
	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("Snapshot len = %d; want 1", len(snap))
	}
}

func TestTcpConnectionRegistrySnapshotForRule(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1", ClientPort: 10001})
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1", ClientPort: 10002})
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r2", ClientPort: 10003})

	snap := reg.SnapshotForRule("r1", time.Now())
	if len(snap) != 2 {
		t.Fatalf("SnapshotForRule(r1) len = %d; want 2", len(snap))
	}
	for _, info := range snap {
		if info.RuleID != "r1" {
			t.Errorf("SnapshotForRule returned entry with RuleID = %q; want r1", info.RuleID)
		}
	}
}

func TestTcpConnectionRegistrySnapshotForRuleEmpty(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	snap := reg.SnapshotForRule("nonexistent", time.Now())
	if snap == nil {
		t.Fatal("SnapshotForRule returned nil; want empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("SnapshotForRule len = %d; want 0", len(snap))
	}
}

func TestTcpConnectionRegistrySnapshotForRuleFiltersOtherRules(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r2"})

	snap := reg.SnapshotForRule("r2", time.Now())
	if len(snap) != 1 {
		t.Fatalf("SnapshotForRule(r2) len = %d; want 1", len(snap))
	}
	if snap[0].RuleID != "r2" {
		t.Errorf("RuleID = %q; want r2", snap[0].RuleID)
	}
}

func TestTcpConnectionRegistryJSONShape(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	_ = reg.OpenConnection(connections.TcpConnectionInput{
		RuleID:        "r1",
		RuleName:      "Test",
		ClientAddress: "127.0.0.1",
		ClientPort:    10001,
		TargetAddress: "10.0.0.1",
		TargetPort:    8080,
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
	for _, key := range expected {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON missing field %q", key)
		}
	}
	if len(m) != len(expected) {
		t.Errorf("JSON has %d fields; want %d (no extra payload fields)", len(m), len(expected))
	}
}

func TestTcpConnectionRegistryStatusIsAlwaysActive(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	snap := reg.Snapshot(time.Now())
	if snap[0].Status != "active" {
		t.Errorf("Status = %q; want active", snap[0].Status)
	}
}

func TestTcpConnectionRegistryProtocolIsAlwaysTCP(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	_ = reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})
	snap := reg.Snapshot(time.Now())
	if snap[0].Protocol != "tcp" {
		t.Errorf("Protocol = %q; want tcp", snap[0].Protocol)
	}
}

func TestTcpConnectionRegistryIDsAreUnique(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1", ClientPort: i})
		if seen[id] {
			t.Fatalf("duplicate ID generated: %q", id)
		}
		seen[id] = true
	}
}

func TestTcpConnectionRegistryConcurrentByteCounting(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})

	const goroutines = 50
	const addsPerGoroutine = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < addsPerGoroutine; j++ {
				reg.AddBytesIn(id, 1)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < addsPerGoroutine; j++ {
				reg.AddBytesOut(id, 1)
			}
		}()
	}
	wg.Wait()

	snap := reg.Snapshot(time.Now())
	if len(snap) != 1 {
		t.Fatalf("Snapshot len = %d; want 1", len(snap))
	}
	want := int64(goroutines * addsPerGoroutine)
	if snap[0].BytesIn != want {
		t.Errorf("BytesIn = %d; want %d", snap[0].BytesIn, want)
	}
	if snap[0].BytesOut != want {
		t.Errorf("BytesOut = %d; want %d", snap[0].BytesOut, want)
	}
}

func TestTcpConnectionRegistryCloseWhileAddingBytesDoesNotPanic(t *testing.T) {
	reg := connections.NewTcpConnectionRegistry()
	id := reg.OpenConnection(connections.TcpConnectionInput{RuleID: "r1"})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			reg.AddBytesIn(id, 1)
		}
	}()
	go func() {
		defer wg.Done()
		reg.CloseConnection(id)
	}()

	wg.Wait() // must not panic
}
