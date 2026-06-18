package manager

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"portier/service/sources/config"
	"portier/service/sources/domain"
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

// ── Autostart recovery (Slice 3) ──────────────────────────────────────────────

// TestStartEnabledFailedRuleDoesNotBlockOthers: one enabled rule on an occupied
// port fails to bind, but an unrelated enabled rule still autostarts, and the
// whole pass is non-fatal.
func TestStartEnabledFailedRuleDoesNotBlockOthers(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer occupied.Close()
	occupiedPort := occupied.Addr().(*net.TCPAddr).Port

	bad := tcpRule()
	bad.ID = "bad"
	bad.Name = "Bad"
	bad.ListenPort = occupiedPort
	bad.Enabled = true

	good := tcpRule()
	good.ID = "good"
	good.Name = "Good"
	good.ListenPort = freeTCPPort(t)
	good.Enabled = true

	m := testManager(t, []domain.ForwardRule{bad, good})
	defer m.StopAll()

	result := m.StartEnabled()
	if result.Attempted != 2 || result.Started != 1 {
		t.Fatalf("result = %+v, want attempted 2 / started 1", result)
	}
	if len(result.Failed) != 1 || result.Failed[0].RuleID != "bad" {
		t.Fatalf("failed = %+v, want one entry for bad", result.Failed)
	}

	badStatus := statusByRuleID(m.ListStatus(), "bad")
	if badStatus.Running || badStatus.LastError == "" || badStatus.Health != domain.HealthError {
		t.Fatalf("bad rule status = %#v, want stopped+lastError+error health", badStatus)
	}
	goodStatus := statusByRuleID(m.ListStatus(), "good")
	if !goodStatus.Running {
		t.Fatalf("good rule should be running: %#v", goodStatus)
	}
}

// TestStartEnabledSkipsDuplicateBoundRules: two enabled rules share a listen
// binding (persisted duplicate). Neither autostarts; both are marked
// stopped/error; an unrelated enabled rule still starts; nothing is rewritten.
func TestStartEnabledSkipsDuplicateBoundRules(t *testing.T) {
	dupPort := freeTCPPort(t)

	ruleA := tcpRule()
	ruleA.ID = "dup-a"
	ruleA.Name = "A"
	ruleA.ListenPort = dupPort
	ruleA.Enabled = true

	ruleC := tcpRule()
	ruleC.ID = "dup-c"
	ruleC.Name = "C"
	ruleC.ListenPort = dupPort
	ruleC.Enabled = true

	other := tcpRule()
	other.ID = "other-b"
	other.Name = "B"
	other.ListenPort = freeTCPPort(t)
	other.Enabled = true

	configPath := filepath.Join(t.TempDir(), "rules.json")
	store := config.NewStore(configPath)
	// Rule order A, B, C so we can assert conflict reporting is in rule order.
	m, err := NewWithStore(&store, []domain.ForwardRule{ruleA, other, ruleC})
	if err != nil {
		t.Fatalf("NewWithStore: %v", err)
	}
	defer m.StopAll()

	result := m.StartEnabled()
	if result.Attempted != 3 || result.Started != 1 {
		t.Fatalf("result = %+v, want attempted 3 / started 1", result)
	}
	if len(result.Skipped) != 2 {
		t.Fatalf("skipped = %+v, want 2 conflicting rules", result.Skipped)
	}
	// Deterministic: skipped reported in rule order (A before C).
	if result.Skipped[0].RuleID != "dup-a" || result.Skipped[1].RuleID != "dup-c" {
		t.Fatalf("skipped order = %+v, want dup-a then dup-c", result.Skipped)
	}

	for _, id := range []string{"dup-a", "dup-c"} {
		s := statusByRuleID(m.ListStatus(), id)
		if s.Running {
			t.Fatalf("conflicting rule %s must not run", id)
		}
		if s.LastError == "" || s.Health != domain.HealthError {
			t.Fatalf("conflicting rule %s status = %#v, want lastError+error health", id, s)
		}
	}
	if s := statusByRuleID(m.ListStatus(), "other-b"); !s.Running {
		t.Fatalf("non-conflicting rule should run: %#v", s)
	}

	// StartEnabled must not rewrite (or create) the config file.
	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Fatalf("config file must not be written by autostart; stat err = %v", statErr)
	}
}

// TestStartEnabledEnabledDisabledShareBindingStillStarts: a binding shared by an
// enabled and a DISABLED rule is not a runtime conflict — the enabled rule
// autostarts (the disabled one would never bind).
func TestStartEnabledEnabledDisabledShareBindingStillStarts(t *testing.T) {
	port := freeTCPPort(t)

	enabled := tcpRule()
	enabled.ID = "en"
	enabled.Name = "Enabled"
	enabled.ListenPort = port
	enabled.Enabled = true

	disabled := tcpRule()
	disabled.ID = "dis"
	disabled.Name = "Disabled"
	disabled.ListenPort = port
	disabled.Enabled = false

	m := testManager(t, []domain.ForwardRule{enabled, disabled})
	defer m.StopAll()

	result := m.StartEnabled()
	if result.Started != 1 || len(result.Skipped) != 0 || len(result.Failed) != 0 {
		t.Fatalf("result = %+v, want started 1 / no skip / no fail", result)
	}
	if s := statusByRuleID(m.ListStatus(), "en"); !s.Running {
		t.Fatalf("enabled rule should run despite a disabled duplicate: %#v", s)
	}
}
