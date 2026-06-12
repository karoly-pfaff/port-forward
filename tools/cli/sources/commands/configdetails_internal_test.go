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
