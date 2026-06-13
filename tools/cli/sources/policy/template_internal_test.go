package policy

// White-box invariants for the built-in policy template registry: every template
// renders to a complete schema-1 policy file that Parse accepts and Evaluate can
// run, the registry is sorted and unique, and no template introduces a protocol
// restriction or an allowUdp rule (UDP is first-class).

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"portier/cli/sources/config"
)

func TestTemplates_SortedAndUnique(t *testing.T) {
	templates := Templates()
	if len(templates) < 3 {
		t.Fatalf("expected at least 3 built-in templates, got %d", len(templates))
	}
	seen := map[string]bool{}
	names := make([]string, len(templates))
	for i, tm := range templates {
		if tm.Name == "" || tm.Title == "" || tm.Description == "" {
			t.Errorf("template %d missing metadata: %+v", i, tm)
		}
		if seen[tm.Name] {
			t.Errorf("duplicate template name %q", tm.Name)
		}
		seen[tm.Name] = true
		names[i] = tm.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("templates not sorted by name: %v", names)
	}
	// The three documented templates must exist.
	for _, want := range []string{"local-safe", "managed", "permissive"} {
		if !seen[want] {
			t.Errorf("missing built-in template %q (have %v)", want, names)
		}
	}
}

func TestTemplates_RenderParsesAndEvaluates(t *testing.T) {
	for _, tm := range Templates() {
		data, err := json.Marshal(tm.PolicyValue())
		if err != nil {
			t.Fatalf("marshaling template %q: %v", tm.Name, err)
		}
		// The rendered policy must use schemaVersion 1 and be accepted by Parse
		// (DisallowUnknownFields), so a template file is directly usable by check.
		if !strings.Contains(string(data), `"schemaVersion":1`) {
			t.Errorf("template %q is not schemaVersion 1: %s", tm.Name, data)
		}
		pol, err := Parse(data)
		if err != nil {
			t.Fatalf("Parse rejected rendered template %q: %v\n%s", tm.Name, err, data)
		}
		// Evaluate must run deterministically against an empty config (no panic).
		report := Evaluate([]config.Rule{}, pol)
		if report.Result != "passed" {
			t.Errorf("empty-config evaluation of %q = %q, want passed", tm.Name, report.Result)
		}
	}
}

func TestTemplates_NoProtocolRestrictionOrAllowUdp(t *testing.T) {
	for _, tm := range Templates() {
		data, err := json.Marshal(tm.DetailValue())
		if err != nil {
			t.Fatalf("marshaling template %q: %v", tm.Name, err)
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"allowudp", "udp", "protocol", "allowlist", "denylist"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("template %q must not mention %q: %s", tm.Name, forbidden, data)
			}
		}
	}
}

func TestFindTemplate(t *testing.T) {
	tm, ok := FindTemplate("managed")
	if !ok {
		t.Fatal("FindTemplate(managed) not found")
	}
	if tm.Name != "managed" || tm.Title == "" {
		t.Errorf("unexpected template: %+v", tm)
	}
	if _, ok := FindTemplate("nope"); ok {
		t.Error("FindTemplate(nope) should not be found")
	}
	if _, ok := FindTemplate(""); ok {
		t.Error("FindTemplate(empty) should not be found")
	}
}

func TestTemplateNames(t *testing.T) {
	got := TemplateNames()
	if !sort.StringsAreSorted(got) {
		t.Errorf("TemplateNames not sorted: %v", got)
	}
	want := map[string]bool{"local-safe": true, "managed": true, "permissive": true}
	for _, n := range got {
		delete(want, n)
	}
	if len(want) != 0 {
		t.Errorf("TemplateNames missing %v", want)
	}
}

// TestTemplates_RulesMatchPurpose locks the guardrail booleans of each built-in
// template so an accidental edit that changes a template's safety posture fails.
func TestTemplates_RulesMatchPurpose(t *testing.T) {
	cases := map[string]templatePolicyRules{
		"permissive": {RequireGroup: false, AllowLanExposure: true, AllowPrivilegedPorts: true, AllowAutostart: true, ForbidDuplicateBindings: true},
		"local-safe": {RequireGroup: false, AllowLanExposure: false, AllowPrivilegedPorts: false, AllowAutostart: true, ForbidDuplicateBindings: true},
		"managed":    {RequireGroup: true, AllowLanExposure: false, AllowPrivilegedPorts: false, AllowAutostart: false, ForbidDuplicateBindings: true},
	}
	for name, want := range cases {
		tm, ok := FindTemplate(name)
		if !ok {
			t.Fatalf("missing template %q", name)
		}
		if tm.file.SchemaVersion != schemaVersion {
			t.Errorf("%q schemaVersion = %d, want %d", name, tm.file.SchemaVersion, schemaVersion)
		}
		if tm.file.Rules != want {
			t.Errorf("%q rules = %+v, want %+v", name, tm.file.Rules, want)
		}
	}
}
