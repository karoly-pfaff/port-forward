package manager

// Coverage Slice D — manager branches not exercised by the existing lifecycle
// and rollback suites. Both tests are deterministic and bind no sockets.

import (
	"testing"

	"portier/service/sources/domain"
)

// TestStartEnabledSkipsDisabledRules covers the `continue` branch in
// StartEnabled: disabled rules must be skipped and not counted. Using only
// disabled rules keeps the test free of real socket binds.
func TestStartEnabledSkipsDisabledRules(t *testing.T) {
	ruleA := tcpRule()
	ruleA.Enabled = false
	ruleB := udpRule() // already disabled
	ruleB.ListenPort = 48050

	m := testManager(t, []domain.ForwardRule{ruleA, ruleB})
	started, err := m.StartEnabled()
	if err != nil {
		t.Fatalf("StartEnabled: %v", err)
	}
	if started != 0 {
		t.Fatalf("started = %d, want 0 (all rules disabled)", started)
	}
	for _, s := range m.ListStatus() {
		if s.Running {
			t.Fatalf("disabled rule %s should not be running", s.RuleID)
		}
	}
}

// TestImportMergeRegeneratesCollidingID covers manager.hasIDIn's match branch:
// an imported rule whose ID collides with an existing rule but binds a different
// port must be kept with a freshly generated ID (no data loss, no conflict).
func TestImportMergeRegeneratesCollidingID(t *testing.T) {
	existing := tcpRule() // id "tcp-1", port 48001
	m := testManager(t, []domain.ForwardRule{existing})

	incoming := tcpRule()
	incoming.ID = "tcp-1"      // same ID as existing → must be regenerated
	incoming.Name = "Incoming"
	incoming.ListenPort = 48060 // different binding → no conflict
	incoming.Enabled = false

	result, err := m.ImportConfig(domain.ExportedConfig{Version: "1", Rules: []domain.ForwardRule{incoming}}, "merge")
	if err != nil {
		t.Fatalf("ImportConfig: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("imported = %d, want 1", result.Imported)
	}

	rules := m.ListRules()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules after merge, got %d", len(rules))
	}
	// Exactly one rule keeps "tcp-1"; the imported one got a fresh ID.
	ids := map[string]int{}
	for _, r := range rules {
		ids[r.ID]++
	}
	if ids["tcp-1"] != 1 {
		t.Fatalf("expected exactly one rule with id tcp-1, got %d", ids["tcp-1"])
	}
	// The imported rule (Name "Incoming") must not have kept the colliding ID.
	for _, r := range rules {
		if r.Name == "Incoming" && r.ID == "tcp-1" {
			t.Fatal("imported rule kept the colliding ID instead of regenerating")
		}
	}
}
