package config_test

import (
	"errors"
	"strings"
	"testing"

	"portier/cli/sources/config"
)

func TestClassify_BareArray(t *testing.T) {
	cls, rules, err := config.Classify([]byte(`[{"name":"a","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"127.0.0.1","targetPort":3000,"enabled":true}]`))
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if cls.Format != config.FormatArray || cls.SourceVersion != "" {
		t.Fatalf("classification = %+v, want bare-array/no-version", cls)
	}
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(rules))
	}
}

func TestClassify_WrapperObject(t *testing.T) {
	cls, rules, err := config.Classify([]byte(`{"rules":[]}`))
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if cls.Format != config.FormatWrapper || cls.SourceVersion != "" {
		t.Fatalf("classification = %+v, want wrapper-object/no-version", cls)
	}
	if len(rules) != 0 {
		t.Fatalf("rules = %d, want 0", len(rules))
	}
}

func TestClassify_ExportedEnvelope(t *testing.T) {
	cls, _, err := config.Classify([]byte(`{"version":"1","exportedAt":"2026-01-01T00:00:00Z","rules":[]}`))
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if cls.Format != config.FormatExported || cls.SourceVersion != "1" {
		t.Fatalf("classification = %+v, want exported/version 1", cls)
	}
}

func TestClassify_UnsupportedVersion(t *testing.T) {
	_, _, err := config.Classify([]byte(`{"version":"2","rules":[]}`))
	if !errors.Is(err, config.ErrUnsupportedVersion) {
		t.Fatalf("err = %v, want ErrUnsupportedVersion", err)
	}
	if !strings.Contains(err.Error(), `"2"`) {
		t.Fatalf("error should name the version: %v", err)
	}
}

func TestClassify_Empty(t *testing.T) {
	if _, _, err := config.Classify([]byte("   ")); !errors.Is(err, config.ErrEmptyConfig) {
		t.Fatalf("err = %v, want ErrEmptyConfig", err)
	}
}

func TestClassify_Malformed(t *testing.T) {
	cases := map[string]string{
		"not json":        "this is not json",
		"object no rules": `{"version":"1"}`,
		"bad array":       `[{,}]`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := config.Classify([]byte(body)); !errors.Is(err, config.ErrMalformedConfig) {
				t.Fatalf("err = %v, want ErrMalformedConfig", err)
			}
		})
	}
}

func TestCanonicalRules_OmitsEmptyOptionalFieldsKeepsEnabled(t *testing.T) {
	rules := []config.Rule{
		{Name: "tcp", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48001, TargetHost: "127.0.0.1", TargetPort: 3000, Enabled: false},
		{ID: "u1", Name: "udp", Protocol: "udp", ListenHost: "127.0.0.1", ListenPort: 48002, TargetHost: "127.0.0.1", TargetPort: 9000, Enabled: true, UDPMode: "one-way", Group: "  web  "},
	}
	out, err := config.CanonicalRules(rules)
	if err != nil {
		t.Fatalf("CanonicalRules error: %v", err)
	}
	s := string(out)
	if !strings.HasSuffix(s, "]\n") {
		t.Fatalf("canonical output should end with a trailing newline: %q", s)
	}
	if !strings.Contains(s, "\"enabled\": false") {
		t.Fatal("enabled:false must be emitted")
	}
	// The TCP rule has no id/udpMode/group → those keys must be absent for it.
	if strings.Contains(s, "\"udpMode\": \"\"") || strings.Contains(s, "\"group\": \"\"") || strings.Contains(s, "\"id\": \"\"") {
		t.Fatalf("empty optional fields must be omitted: %s", s)
	}
	// The UDP rule's group is trimmed.
	if !strings.Contains(s, "\"group\": \"web\"") {
		t.Fatalf("group should be trimmed to \"web\": %s", s)
	}
	if !strings.Contains(s, "\"id\": \"u1\"") || !strings.Contains(s, "\"udpMode\": \"one-way\"") {
		t.Fatalf("present optional fields must be kept: %s", s)
	}

	// Canonical output round-trips cleanly back to the same rule set.
	cls, reparsed, err := config.Classify(out)
	if err != nil || cls.Format != config.FormatArray {
		t.Fatalf("canonical output should re-classify as bare-array: %v / %+v", err, cls)
	}
	if len(reparsed) != 2 {
		t.Fatalf("round-trip rule count = %d, want 2", len(reparsed))
	}
}

func TestCanonicalRules_EmptyIsBareArray(t *testing.T) {
	out, err := config.CanonicalRules(nil)
	if err != nil {
		t.Fatalf("CanonicalRules error: %v", err)
	}
	if string(out) != "[]\n" {
		t.Fatalf("empty canonical = %q, want \"[]\\n\"", string(out))
	}
}
