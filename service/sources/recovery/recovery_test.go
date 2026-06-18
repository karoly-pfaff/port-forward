package recovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const validConfig = `[
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
]`

var fixedNow = time.Date(2026, 6, 18, 14, 25, 30, 0, time.UTC)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestMissingConfigIsNormal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	rules, state := loadConfigAt(path, fixedNow)

	if state.Active() {
		t.Fatalf("state = %+v, want nil/inactive for missing config", state)
	}
	if len(rules) != 0 {
		t.Fatalf("rules = %#v, want empty", rules)
	}
}

func TestValidConfigIsNormal(t *testing.T) {
	path := writeConfig(t, validConfig)

	rules, state := loadConfigAt(path, fixedNow)

	if state.Active() {
		t.Fatalf("state = %+v, want nil/inactive for valid config", state)
	}
	if len(rules) != 1 || rules[0].ID != "rule-1" {
		t.Fatalf("rules = %#v, want one rule-1", rules)
	}
}

func TestMalformedJSONQuarantinesAndRecovers(t *testing.T) {
	path := writeConfig(t, "this is not json")

	rules, state := loadConfigAt(path, fixedNow)

	if !state.Active() {
		t.Fatal("expected active recovery state")
	}
	if state.Reason != ReasonMalformed {
		t.Fatalf("reason = %q, want %q", state.Reason, ReasonMalformed)
	}
	if !state.WritesBlocked {
		t.Fatal("expected writes blocked")
	}
	if len(rules) != 0 {
		t.Fatalf("rules = %#v, want empty (no salvage)", rules)
	}

	// Original path was quarantined away; the bad bytes are preserved there.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("original config should have been moved; stat err = %v", err)
	}
	if state.QuarantinePath == "" {
		t.Fatal("expected a quarantine path")
	}
	quarantined, err := os.ReadFile(state.QuarantinePath)
	if err != nil {
		t.Fatalf("read quarantined file: %v", err)
	}
	if string(quarantined) != "this is not json" {
		t.Fatalf("quarantined contents = %q, want original bytes", string(quarantined))
	}
	if filepath.Dir(state.QuarantinePath) != filepath.Dir(path) {
		t.Fatalf("quarantine should be same-directory: %q vs %q", state.QuarantinePath, path)
	}
}

func TestWrongTopLevelTypeIsMalformed(t *testing.T) {
	path := writeConfig(t, `{"version":"1"}`)

	rules, state := loadConfigAt(path, fixedNow)

	if state.Reason != ReasonMalformed {
		t.Fatalf("reason = %q, want %q", state.Reason, ReasonMalformed)
	}
	if len(rules) != 0 {
		t.Fatalf("rules = %#v, want empty", rules)
	}
	if state.QuarantinePath == "" {
		t.Fatal("expected quarantine for malformed container")
	}
}

func TestSchemaInvalidQuarantinesNoSalvage(t *testing.T) {
	// One valid rule, one invalid rule: the whole file must be rejected with no
	// partial salvage (the valid rule must not survive).
	path := writeConfig(t, `[
  {"id":"good","name":"Good","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"127.0.0.1","targetPort":3000,"enabled":true},
  {"name":"","protocol":"invalid"}
]`)

	rules, state := loadConfigAt(path, fixedNow)

	if state.Reason != ReasonSchemaInvalid {
		t.Fatalf("reason = %q, want %q", state.Reason, ReasonSchemaInvalid)
	}
	if len(rules) != 0 {
		t.Fatalf("rules = %#v, want empty (no partial salvage)", rules)
	}
	if state.QuarantinePath == "" {
		t.Fatal("expected quarantine for schema-invalid config")
	}
	if !state.WritesBlocked {
		t.Fatal("expected writes blocked")
	}
}

func TestUnreadableConfigPreservedNotQuarantined(t *testing.T) {
	// A directory at the config path makes os.ReadFile fail with a non-ErrNotExist
	// error on all platforms, simulating an unreadable file portably.
	dir := t.TempDir()
	path := filepath.Join(dir, "rules-as-dir.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir config-as-dir: %v", err)
	}

	rules, state := loadConfigAt(path, fixedNow)

	if state.Reason != ReasonUnreadable {
		t.Fatalf("reason = %q, want %q", state.Reason, ReasonUnreadable)
	}
	if state.QuarantinePath != "" {
		t.Fatalf("unreadable config must not be quarantined; got %q", state.QuarantinePath)
	}
	if !state.WritesBlocked {
		t.Fatal("expected writes blocked")
	}
	if len(rules) != 0 {
		t.Fatalf("rules = %#v, want empty", rules)
	}
	// Original is left in place.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("original path should be untouched: %v", err)
	}
}

func TestQuarantineNameIsUniqueAndDoesNotOverwrite(t *testing.T) {
	path := writeConfig(t, "not json")
	// Pre-create the name the first quarantine would choose, so the loader must
	// pick a distinct name rather than overwrite it.
	taken := path + ".corrupt-" + fixedNow.Format("2006-01-02T150405Z")
	if err := os.WriteFile(taken, []byte("PRIOR"), 0o600); err != nil {
		t.Fatalf("seed prior quarantine: %v", err)
	}

	_, state := loadConfigAt(path, fixedNow)

	if state.QuarantinePath == taken {
		t.Fatal("quarantine overwrote an existing quarantine file")
	}
	if state.QuarantinePath == "" {
		t.Fatal("expected a quarantine path")
	}
	prior, err := os.ReadFile(taken)
	if err != nil || string(prior) != "PRIOR" {
		t.Fatalf("prior quarantine should be intact; contents=%q err=%v", string(prior), err)
	}
}

func TestQuarantineFailureKeepsRecoveryAndPreservesOriginal(t *testing.T) {
	// Make the parent directory read-only so the rename (quarantine) fails, but
	// the file was already read into memory before the failure. On Windows,
	// directory permissions do not block rename the same way, so skip there.
	if isWindows() {
		t.Skip("directory permission semantics differ on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	rules, state := loadConfigAt(path, fixedNow)

	if !state.Active() {
		t.Fatal("expected active recovery state even when quarantine fails")
	}
	if state.QuarantinePath != "" {
		t.Fatalf("quarantine should have failed; got path %q", state.QuarantinePath)
	}
	if !strings.Contains(state.Message, "left in place") {
		t.Fatalf("message = %q, want note that original was left in place", state.Message)
	}
	if len(rules) != 0 {
		t.Fatalf("rules = %#v, want empty", rules)
	}
	// Original must still be present (never deleted on quarantine failure).
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("original config must survive a failed quarantine: %v", err)
	}
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}
