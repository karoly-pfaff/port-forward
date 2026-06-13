package doctor

// White-box test for the config-doctor LAN/privileged advisory aggregation
// and single-vs-multi-rule titles.

import (
	"testing"

	"portier/cli/sources/config"
)

func TestConfigDoctorAdvisories_AggregatesAndTitles(t *testing.T) {
	// Two exposed + two privileged rules → one check each (aggregated).
	rules := []config.Rule{
		{Name: "X", Protocol: "tcp", ListenHost: "0.0.0.0", ListenPort: 80},
		{Name: "Y", Protocol: "tcp", ListenHost: "0.0.0.0", ListenPort: 443},
	}
	adv := configAdvisories(rules)
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
	one := configAdvisories([]config.Rule{{Name: "SSH", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 22}})
	if len(one) != 1 || one[0].Title != `Rule "SSH" uses privileged port 22` {
		t.Errorf("single privileged title wrong: %+v", one)
	}
}
