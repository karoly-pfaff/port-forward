package manager

import (
	"net"
	"testing"

	"portier/service/sources/domain"
)

func groupPtr(s string) *string { return &s }

func groupedRule(t *testing.T, id, name, group string) domain.ForwardRule {
	t.Helper()
	r := domain.ForwardRule{
		ID:         id,
		Name:       name,
		Protocol:   domain.ProtocolTCP,
		ListenHost: "127.0.0.1",
		ListenPort: freeTCPPort(t),
		TargetHost: "127.0.0.1",
		TargetPort: 49999,
		Enabled:    false,
	}
	if group != "" {
		r.Group = groupPtr(group)
	}
	return r
}

func runningByID(t *testing.T, m *Manager, id string) bool {
	t.Helper()
	for _, s := range m.ListStatus() {
		if s.RuleID == id {
			return s.Running
		}
	}
	t.Fatalf("status for rule %s not found", id)
	return false
}

func TestStartGroupStartsMatchingRulesInOrder(t *testing.T) {
	m := testManager(t, []domain.ForwardRule{
		groupedRule(t, "w1", "Web One", "web"),
		groupedRule(t, "a1", "Api One", "api"),
		groupedRule(t, "w2", "Web Two", "web"),
		groupedRule(t, "u1", "Loose", ""),
	})
	defer m.StopAll()

	results := m.StartGroup("web")
	if len(results) != 2 || results[0].RuleID != "w1" || results[1].RuleID != "w2" {
		t.Fatalf("expected [w1 w2] in order, got %#v", results)
	}
	for _, r := range results {
		if r.Status != "started" {
			t.Fatalf("expected started, got %#v", r)
		}
	}
	if runningByID(t, m, "a1") || runningByID(t, m, "u1") {
		t.Fatal("non-web rules must not be started")
	}
}

func TestStartGroupSkipsAlreadyRunning(t *testing.T) {
	rule := groupedRule(t, "w1", "Web One", "web")
	m := testManager(t, []domain.ForwardRule{rule})
	defer m.StopAll()
	startRuleStable(t, m, &rule)

	results := m.StartGroup("web")
	if len(results) != 1 || results[0].Status != "skipped" || results[0].Reason != "already_running" {
		t.Fatalf("expected skipped already_running, got %#v", results)
	}
}

func TestStopGroupStopsRunningSkipsStopped(t *testing.T) {
	w1 := groupedRule(t, "w1", "Web One", "web")
	w2 := groupedRule(t, "w2", "Web Two", "web")
	m := testManager(t, []domain.ForwardRule{w1, w2})
	defer m.StopAll()
	startRuleStable(t, m, &w1) // only w1 running

	results := m.StopGroup("web")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %#v", results)
	}
	if results[0].RuleID != "w1" || results[0].Status != "stopped" {
		t.Fatalf("expected w1 stopped, got %#v", results[0])
	}
	if results[1].RuleID != "w2" || results[1].Status != "skipped" || results[1].Reason != "not_running" {
		t.Fatalf("expected w2 skipped not_running, got %#v", results[1])
	}
	if runningByID(t, m, "w1") {
		t.Fatal("w1 should be stopped")
	}
}

func TestGroupNoMatchReturnsEmpty(t *testing.T) {
	m := testManager(t, []domain.ForwardRule{groupedRule(t, "w1", "Web One", "web")})
	defer m.StopAll()
	if got := m.StartGroup("ghost"); len(got) != 0 {
		t.Fatalf("StartGroup(ghost) = %#v, want empty", got)
	}
	if got := m.StopGroup("ghost"); len(got) != 0 {
		t.Fatalf("StopGroup(ghost) = %#v, want empty", got)
	}
}

func TestStartGroupPartialFailure(t *testing.T) {
	// Occupy a port so a rule bound to it fails to start.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	occupied := ln.Addr().(*net.TCPAddr).Port

	bad := domain.ForwardRule{
		ID: "bad", Name: "Bad", Protocol: domain.ProtocolTCP, ListenHost: "127.0.0.1",
		ListenPort: occupied, TargetHost: "127.0.0.1", TargetPort: 49999, Enabled: false, Group: groupPtr("web"),
	}
	good := groupedRule(t, "good", "Good", "web")
	m := testManager(t, []domain.ForwardRule{bad, good})
	defer m.StopAll()

	results := m.StartGroup("web")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %#v", results)
	}
	if results[0].RuleID != "bad" || results[0].Status != "failed" || results[0].Reason == "" {
		t.Fatalf("expected bad failed with reason, got %#v", results[0])
	}
	if results[1].RuleID != "good" || results[1].Status != "started" {
		t.Fatalf("expected good started, got %#v", results[1])
	}
}

func TestGroupActionDoesNotMutateRules(t *testing.T) {
	m := testManager(t, []domain.ForwardRule{
		groupedRule(t, "w1", "Web One", "web"),
		groupedRule(t, "a1", "Api One", "api"),
	})
	defer m.StopAll()

	before := m.ListRules()
	m.StartGroup("web")
	m.StopGroup("web")
	after := m.ListRules()

	if len(before) != len(after) {
		t.Fatalf("rule count changed: %d -> %d", len(before), len(after))
	}
	for i := range before {
		if before[i].ID != after[i].ID || before[i].Name != after[i].Name ||
			before[i].ListenPort != after[i].ListenPort || before[i].Enabled != after[i].Enabled ||
			groupValue(before[i].Group) != groupValue(after[i].Group) {
			t.Fatalf("rule %d mutated: %#v -> %#v", i, before[i], after[i])
		}
	}
}

func groupValue(g *string) string {
	if g == nil {
		return ""
	}
	return *g
}
