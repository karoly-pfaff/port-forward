package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/service/sources/domain"
)

func TestMissingConfigReturnsEmptyRules(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "missing.json"))

	rules, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("rules = %#v, want empty", rules)
	}
}

func TestValidConfigLoadsRules(t *testing.T) {
	configPath := writeConfig(t, `[
  {
    "id": "rule-1",
    "name": "Local app",
    "protocol": "tcp",
    "listenHost": "127.0.0.1",
    "listenPort": 48001,
    "targetHost": "127.0.0.1",
    "targetPort": 3000,
    "enabled": true
  },
  {
    "id": "rule-2",
    "name": "Stats",
    "protocol": "udp",
    "listenHost": "127.0.0.1",
    "listenPort": 48002,
    "targetHost": "127.0.0.1",
    "targetPort": 9000,
    "enabled": false
  }
]`)

	rules, err := NewStore(configPath).Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("rule count = %d, want 2", len(rules))
	}
	if rules[1].UdpMode == nil || *rules[1].UdpMode != "one-way" {
		t.Fatalf("udpMode = %v, want one-way", rules[1].UdpMode)
	}
}

func TestObjectWrapperConfigLoadsRules(t *testing.T) {
	configPath := writeConfig(t, `{
  "version": "1",
  "exportedAt": "2026-06-06T00:00:00.000Z",
  "rules": [
    {
      "id": "rule-1",
      "name": "Local app",
      "protocol": "tcp",
      "listenHost": "127.0.0.1",
      "listenPort": 48001,
      "targetHost": "127.0.0.1",
      "targetPort": 3000,
      "enabled": true
    }
  ]
}`)

	rules, err := NewStore(configPath).Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("rule count = %d, want 1", len(rules))
	}
}

func TestInvalidJSONReturnsError(t *testing.T) {
	configPath := writeConfig(t, "this is not json")

	_, err := NewStore(configPath).Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Invalid JSON") {
		t.Fatalf("error = %q, want Invalid JSON", err)
	}
}

func TestInvalidRuleReturnsIndexError(t *testing.T) {
	configPath := writeConfig(t, `[{"name":"","protocol":"invalid"}]`)

	_, err := NewStore(configPath).Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Invalid rule at index 0") {
		t.Fatalf("error = %q, want Invalid rule at index 0", err)
	}
}

func TestSaveWritesValidJSONAndReloads(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "forwards.json")
	store := NewStore(configPath)

	if err := store.Save([]domain.ForwardRule{testRule()}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.HasPrefix(string(raw), "[\n") {
		t.Fatalf("saved shape = %q, want raw array", string(raw))
	}

	rules, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != "rule-1" {
		t.Fatalf("rules = %#v", rules)
	}
}

func TestSaveCreatesParentDirectory(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nested", "forwards.json")
	if err := NewStore(configPath).Save([]domain.ForwardRule{testRule()}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("stat saved config: %v", err)
	}
}

func TestSaveRejectsInvalidRulesWithoutOverwriting(t *testing.T) {
	configPath := writeConfig(t, `[{"id":"existing","name":"Existing","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"127.0.0.1","targetPort":3000,"enabled":true}]`)
	store := NewStore(configPath)
	invalid := testRule()
	invalid.ListenPort = 0

	if err := store.Save([]domain.ForwardRule{invalid}); err == nil {
		t.Fatal("expected invalid rule error")
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(raw), "existing") {
		t.Fatalf("config was overwritten: %s", string(raw))
	}
}

func TestObjectWithoutRulesKeyReturnsError(t *testing.T) {
	configPath := writeConfig(t, `{"version": "1", "exportedAt": "2026-01-01T00:00:00.000Z"}`)
	_, err := NewStore(configPath).Load()
	if err == nil {
		t.Fatal("expected error for object without rules key")
	}
	if !strings.Contains(err.Error(), "Config file must contain an array of forward rules.") {
		t.Fatalf("error = %q, want array error", err)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "forwards.json")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func testRule() domain.ForwardRule {
	return domain.ForwardRule{
		ID:         "rule-1",
		Name:       "Local app",
		Protocol:   domain.ProtocolTCP,
		ListenHost: "127.0.0.1",
		ListenPort: 48001,
		TargetHost: "127.0.0.1",
		TargetPort: 3000,
		Enabled:    true,
	}
}
