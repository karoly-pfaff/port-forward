package commands

// Internal (white-box) tests for the config doctor detail builders: duplicate
// binding grouping/ordering and rule-detail mapping. These cover the grouping
// edge cases (3-on-one, invalid rules skipped, no duplicates) that are awkward
// to assert through the full command JSON.

import (
	"testing"
)

func TestFindDuplicateBindings_ThreeOnOneBinding(t *testing.T) {
	rules := []rawConfigRule{
		{Name: "A", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000, Enabled: true},
		{Name: "B", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{Name: "C", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
	}
	got := findDuplicateBindings(rules)
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
	// not a binding conflict — mirrors validateLocalConfig's gating.
	rules := []rawConfigRule{
		{Name: "BadProto", Protocol: "sctp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{Name: "BadProto2", Protocol: "sctp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{Name: "NoHost", Protocol: "tcp", ListenHost: "", ListenPort: 48000},
		{Name: "BadPort", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 0},
	}
	if got := findDuplicateBindings(rules); len(got) != 0 {
		t.Errorf("expected no conflicts among invalid rules, got %+v", got)
	}
}

func TestFindDuplicateBindings_NoDuplicates(t *testing.T) {
	rules := []rawConfigRule{
		{Name: "A", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{Name: "B", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48001},
		{Name: "C", Protocol: "udp", ListenHost: "127.0.0.1", ListenPort: 48000}, // different protocol
	}
	if got := findDuplicateBindings(rules); len(got) != 0 {
		t.Errorf("expected no conflicts, got %+v", got)
	}
}

func TestFindDuplicateBindings_SortTieBreakers(t *testing.T) {
	// Same protocol+host, differ by port: sorted ascending by port.
	byPort := findDuplicateBindings([]rawConfigRule{
		{Name: "P2a", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48001},
		{Name: "P2b", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48001},
		{Name: "P1a", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{Name: "P1b", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
	})
	if len(byPort) != 2 || byPort[0].ListenPort != 48000 || byPort[1].ListenPort != 48001 {
		t.Errorf("not sorted by port: %+v", byPort)
	}

	// Same protocol+port, differ by host: sorted ascending by host string.
	byHost := findDuplicateBindings([]rawConfigRule{
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
	d := toRuleDetail(rawConfigRule{
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
	s := buildConfigSummary([]rawConfigRule{
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
	s := buildConfigSummary(nil)
	if s.RuleCount != 0 || s.GroupCount != 0 || s.UngroupedRuleCount != 0 {
		t.Errorf("empty summary nonzero: %+v", s)
	}
	if s.Groups == nil || len(s.Groups) != 0 {
		t.Errorf("groups should be a non-nil empty slice, got %v", s.Groups)
	}
}

func TestBuildConfigSummary_UnknownProtocolNotCounted(t *testing.T) {
	s := buildConfigSummary([]rawConfigRule{
		{Name: "X", Protocol: "sctp", Enabled: true},
	})
	if s.Protocols.TCP != 0 || s.Protocols.UDP != 0 {
		t.Errorf("unknown protocol should not be counted: %+v", s.Protocols)
	}
	if s.RuleCount != 1 {
		t.Errorf("ruleCount should still count the rule: %d", s.RuleCount)
	}
}

func TestConfigDoctorAdvisories_AggregatesAndTitles(t *testing.T) {
	// Two exposed + two privileged rules → one check each (aggregated).
	rules := []rawConfigRule{
		{Name: "X", Protocol: "tcp", ListenHost: "0.0.0.0", ListenPort: 80},
		{Name: "Y", Protocol: "tcp", ListenHost: "0.0.0.0", ListenPort: 443},
	}
	adv := configDoctorAdvisories(rules)
	if len(adv) != 2 {
		t.Fatalf("advisories = %d, want 2 (1 lan + 1 privileged)", len(adv))
	}
	if adv[0].Code != checkConfigLanExposure || adv[1].Code != checkConfigPrivilegedPort {
		t.Errorf("advisory order/codes wrong: %s, %s", adv[0].Code, adv[1].Code)
	}
	if adv[0].Title != "2 rules listen on 0.0.0.0" {
		t.Errorf("lan title = %q", adv[0].Title)
	}
	if adv[1].Title != "2 rules use privileged ports" {
		t.Errorf("privileged title = %q", adv[1].Title)
	}

	// Single-rule wording preserved.
	one := configDoctorAdvisories([]rawConfigRule{{Name: "SSH", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 22}})
	if len(one) != 1 || one[0].Title != `Rule "SSH" uses privileged port 22` {
		t.Errorf("single privileged title wrong: %+v", one)
	}
}
