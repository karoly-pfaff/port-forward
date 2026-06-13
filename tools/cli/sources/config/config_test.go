package config_test

// Black-box tests for the exported config domain helpers: duplicate-binding
// grouping/ordering, rule-detail mapping, config summary counts,
// validation-error partitioning, and rule labels.

import (
	"reflect"
	"testing"

	"portier/cli/sources/config"
)

func TestFindDuplicateBindings_ThreeOnOneBinding(t *testing.T) {
	rules := []config.Rule{
		{Name: "A", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000, Enabled: true},
		{Name: "B", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{Name: "C", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
	}
	got := config.FindDuplicateBindings(rules)
	if len(got) != 1 {
		t.Fatalf("conflicts = %d, want 1", len(got))
	}
	if len(got[0].Rules) != 3 {
		t.Fatalf("rules in binding = %d, want 3", len(got[0].Rules))
	}
	// File order preserved.
	if got[0].Rules[0].Name != "A" || got[0].Rules[1].Name != "B" || got[0].Rules[2].Name != "C" {
		t.Errorf("rules not in file order: %+v", got[0].Rules)
	}
}

func TestFindDuplicateBindings_InvalidRulesSkipped(t *testing.T) {
	// A duplicate among invalid rules (bad protocol / empty host / bad port) is
	// not a binding conflict — mirrors Validate's gating.
	rules := []config.Rule{
		{Name: "BadProto", Protocol: "sctp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{Name: "BadProto2", Protocol: "sctp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{Name: "NoHost", Protocol: "tcp", ListenHost: "", ListenPort: 48000},
		{Name: "BadPort", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 0},
	}
	if got := config.FindDuplicateBindings(rules); len(got) != 0 {
		t.Errorf("expected no conflicts among invalid rules, got %+v", got)
	}
}

func TestFindDuplicateBindings_NoDuplicates(t *testing.T) {
	rules := []config.Rule{
		{Name: "A", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{Name: "B", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48001},
		{Name: "C", Protocol: "udp", ListenHost: "127.0.0.1", ListenPort: 48000}, // different protocol
	}
	if got := config.FindDuplicateBindings(rules); len(got) != 0 {
		t.Errorf("expected no conflicts, got %+v", got)
	}
}

func TestFindDuplicateBindings_SortTieBreakers(t *testing.T) {
	// Same protocol+host, differ by port: sorted ascending by port.
	byPort := config.FindDuplicateBindings([]config.Rule{
		{Name: "P2a", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48001},
		{Name: "P2b", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48001},
		{Name: "P1a", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{Name: "P1b", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
	})
	if len(byPort) != 2 || byPort[0].ListenPort != 48000 || byPort[1].ListenPort != 48001 {
		t.Errorf("not sorted by port: %+v", byPort)
	}

	// Same protocol+port, differ by host: sorted ascending by host string.
	byHost := config.FindDuplicateBindings([]config.Rule{
		{Name: "Hb1", Protocol: "tcp", ListenHost: "10.0.0.2", ListenPort: 48000},
		{Name: "Hb2", Protocol: "tcp", ListenHost: "10.0.0.2", ListenPort: 48000},
		{Name: "Ha1", Protocol: "tcp", ListenHost: "10.0.0.1", ListenPort: 48000},
		{Name: "Ha2", Protocol: "tcp", ListenHost: "10.0.0.1", ListenPort: 48000},
	})
	if len(byHost) != 2 || byHost[0].ListenHost != "10.0.0.1" || byHost[1].ListenHost != "10.0.0.2" {
		t.Errorf("not sorted by host: %+v", byHost)
	}
}

func TestToRuleDetail_TrimsGroupAndKeepsFields(t *testing.T) {
	d := config.ToRuleDetail(config.Rule{
		ID: "r1", Name: "Web", Protocol: "tcp", ListenHost: "0.0.0.0",
		ListenPort: 48080, Enabled: true, Group: "  backend  ",
	})
	if d.Group != "backend" {
		t.Errorf("group = %q, want trimmed \"backend\"", d.Group)
	}
	if d.ID != "r1" || d.Name != "Web" || d.Protocol != "tcp" || d.ListenHost != "0.0.0.0" || d.ListenPort != 48080 || !d.Enabled {
		t.Errorf("rule detail fields wrong: %+v", d)
	}
}

func TestBuildConfigSummary_Counts(t *testing.T) {
	s := config.BuildSummary([]config.Rule{
		{Name: "A", Protocol: "tcp", Enabled: true, Group: "backend"},
		{Name: "B", Protocol: "tcp", Enabled: false, Group: " backend "}, // trimmed → same group
		{Name: "C", Protocol: "udp", Enabled: true, Group: "admin"},
		{Name: "D", Protocol: "tcp", Enabled: true, Group: ""},   // ungrouped
		{Name: "E", Protocol: "tcp", Enabled: true, Group: "  "}, // whitespace → ungrouped
	})
	if s.RuleCount != 5 || s.EnabledRuleCount != 4 || s.DisabledRuleCount != 1 {
		t.Errorf("counts wrong: %+v", s)
	}
	if s.Protocols.TCP != 4 || s.Protocols.UDP != 1 {
		t.Errorf("protocols wrong: %+v", s.Protocols)
	}
	if s.GroupCount != 2 || s.UngroupedRuleCount != 2 {
		t.Errorf("group/ungrouped wrong: groupCount=%d ungrouped=%d", s.GroupCount, s.UngroupedRuleCount)
	}
	// Sorted by name: admin, backend.
	if len(s.Groups) != 2 || s.Groups[0].Name != "admin" || s.Groups[1].Name != "backend" {
		t.Fatalf("groups not sorted: %+v", s.Groups)
	}
	if s.Groups[1].RuleCount != 2 || s.Groups[1].EnabledRuleCount != 1 || s.Groups[1].DisabledRuleCount != 1 {
		t.Errorf("backend rollup wrong: %+v", s.Groups[1])
	}
}

func TestBuildConfigSummary_Empty(t *testing.T) {
	s := config.BuildSummary(nil)
	if s.RuleCount != 0 || s.GroupCount != 0 || s.UngroupedRuleCount != 0 {
		t.Errorf("empty summary nonzero: %+v", s)
	}
	if s.Groups == nil || len(s.Groups) != 0 {
		t.Errorf("groups should be a non-nil empty slice, got %v", s.Groups)
	}
}

func TestBuildConfigSummary_UnknownProtocolNotCounted(t *testing.T) {
	s := config.BuildSummary([]config.Rule{
		{Name: "X", Protocol: "sctp", Enabled: true},
	})
	if s.Protocols.TCP != 0 || s.Protocols.UDP != 0 {
		t.Errorf("unknown protocol should not be counted: %+v", s.Protocols)
	}
	if s.RuleCount != 1 {
		t.Errorf("ruleCount should still count the rule: %d", s.RuleCount)
	}
}
func TestPartitionValidationErrors(t *testing.T) {
	dup, field := config.PartitionValidationErrors([]string{
		"rule 1 \"A\": name is required",
		config.DuplicateBindingErrPrefix + " tcp 127.0.0.1:48000 (rules 1 and 2)",
		"rule 2 \"B\": targetHost is required",
	})
	wantDup := []string{config.DuplicateBindingErrPrefix + " tcp 127.0.0.1:48000 (rules 1 and 2)"}
	wantField := []string{"rule 1 \"A\": name is required", "rule 2 \"B\": targetHost is required"}
	if !reflect.DeepEqual(dup, wantDup) {
		t.Errorf("dup = %v, want %v", dup, wantDup)
	}
	if !reflect.DeepEqual(field, wantField) {
		t.Errorf("field = %v, want %v", field, wantField)
	}
}

func TestRuleLabel(t *testing.T) {
	if got := config.RuleLabel(config.Rule{Name: "Web"}); got != `Rule "Web"` {
		t.Errorf("named RuleLabel = %q, want `Rule \"Web\"`", got)
	}
	if got := config.RuleLabel(config.Rule{}); got != "An unnamed rule" {
		t.Errorf("unnamed config.RuleLabel = %q, want \"An unnamed rule\"", got)
	}
}
