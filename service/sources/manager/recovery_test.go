package manager

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"portier/service/sources/recovery"
)

// TestNewFromConfigMissingIsNormal: a missing config is the normal first-run
// state — no recovery, writes allowed, rules persist.
func TestNewFromConfigMissingIsNormal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")

	manager, err := NewFromConfig(path)
	if err != nil {
		t.Fatalf("NewFromConfig returned error: %v", err)
	}
	if manager.RecoveryState().Active() {
		t.Fatalf("recovery state = %+v, want inactive", manager.RecoveryState())
	}

	input := tcpRule()
	input.Enabled = false
	if _, err := manager.CreateRule(inputForRule(input, "")); err != nil {
		t.Fatalf("CreateRule should succeed when not in recovery: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config should have been written: %v", err)
	}
}

// TestNewFromConfigMalformedRecoversNonFatal: a malformed config no longer aborts
// construction — the manager builds with no active rules, recovery is active, and
// the bad file is quarantined.
func TestNewFromConfigMalformedRecoversNonFatal(t *testing.T) {
	path := writeRawConfig(t, "this is not json")

	manager, err := NewFromConfig(path)
	if err != nil {
		t.Fatalf("NewFromConfig must not fail for malformed config: %v", err)
	}
	state := manager.RecoveryState()
	if !state.Active() {
		t.Fatal("expected active recovery state")
	}
	if state.Reason != recovery.ReasonMalformed {
		t.Fatalf("reason = %q, want %q", state.Reason, recovery.ReasonMalformed)
	}
	if len(manager.ListRules()) != 0 {
		t.Fatalf("rules = %#v, want empty in recovery mode", manager.ListRules())
	}
	if state.QuarantinePath == "" {
		t.Fatal("expected the bad config to be quarantined")
	}
}

// TestRecoveryBlocksWritesAndPreventsOverwrite: while recovery is active, a
// create is refused with RecoveryError and no fresh config is written over the
// quarantined bad one.
func TestRecoveryBlocksWritesAndPreventsOverwrite(t *testing.T) {
	path := writeRawConfig(t, "{ not valid")

	manager, err := NewFromConfig(path)
	if err != nil {
		t.Fatalf("NewFromConfig returned error: %v", err)
	}
	if !manager.RecoveryState().Active() {
		t.Fatal("expected active recovery state")
	}

	input := tcpRule()
	input.Enabled = false
	_, createErr := manager.CreateRule(inputForRule(input, ""))
	if createErr == nil {
		t.Fatal("expected CreateRule to be refused in recovery mode")
	}
	var recErr RecoveryError
	if !errors.As(createErr, &recErr) {
		t.Fatalf("error = %T (%v), want RecoveryError", createErr, createErr)
	}
	if len(manager.ListRules()) != 0 {
		t.Fatalf("rules = %#v, want empty after refused create", manager.ListRules())
	}
	// No fresh rules.json was written over the (quarantined) original path.
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("a config file was written at the original path despite recovery; stat err = %v", statErr)
	}
}

// TestSchemaInvalidRecoversNoSalvage: an otherwise-valid config with one bad rule
// is rejected whole — no rules are loaded, recovery is active, file quarantined.
func TestSchemaInvalidRecoversNoSalvage(t *testing.T) {
	path := writeRawConfig(t, `[
  {"id":"good","name":"Good","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false},
  {"name":"","protocol":"invalid"}
]`)

	manager, err := NewFromConfig(path)
	if err != nil {
		t.Fatalf("NewFromConfig must not fail for schema-invalid config: %v", err)
	}
	state := manager.RecoveryState()
	if state.Reason != recovery.ReasonSchemaInvalid {
		t.Fatalf("reason = %q, want %q", state.Reason, recovery.ReasonSchemaInvalid)
	}
	if len(manager.ListRules()) != 0 {
		t.Fatalf("rules = %#v, want empty (no partial salvage)", manager.ListRules())
	}
}

func writeRawConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
