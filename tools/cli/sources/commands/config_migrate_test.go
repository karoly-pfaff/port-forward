package commands_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

func writeMigrateConfig(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func runMigrate(t *testing.T, jsonOutput bool, args ...string) (string, string, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := commands.RunConfigMigrate(jsonOutput, args, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

const validArrayConfig = `[{"name":"API","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true}]`

func TestConfigMigrate_DryRunValid_NoWrite(t *testing.T) {
	path := writeMigrateConfig(t, "rules.json", validArrayConfig)
	out, _, code := runMigrate(t, false, path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "dry run") {
		t.Fatalf("expected dry-run output, got: %s", out)
	}
	// File is unchanged.
	after, _ := os.ReadFile(path)
	if string(after) != validArrayConfig {
		t.Fatalf("dry run must not modify the file; got: %s", string(after))
	}
}

func TestConfigMigrate_DryRunJSON(t *testing.T) {
	path := writeMigrateConfig(t, "rules.json", validArrayConfig)
	out, _, code := runMigrate(t, true, path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("decode JSON report: %v\n%s", err, out)
	}
	if report["valid"] != true || report["dryRun"] != true || report["written"] != false {
		t.Fatalf("unexpected report: %v", report)
	}
	if report["format"] != "bare-array" {
		t.Fatalf("format = %v, want bare-array", report["format"])
	}
}

func TestConfigMigrate_WriteWrapperInPlace_BackupFirst(t *testing.T) {
	original := `{"rules":[{"name":"API","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true}]}`
	path := writeMigrateConfig(t, "rules.json", original)

	out, _, code := runMigrate(t, false, "--write", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Config migrated") || !strings.Contains(out, "Backup:") {
		t.Fatalf("expected migrated + backup output, got: %s", out)
	}

	// The file is now the canonical bare array.
	after, _ := os.ReadFile(path)
	if !strings.HasPrefix(strings.TrimSpace(string(after)), "[") {
		t.Fatalf("migrated file should be a bare array, got: %s", string(after))
	}

	// A backup exists in the same dir holding the original content.
	entries, _ := os.ReadDir(filepath.Dir(path))
	var backup string
	for _, e := range entries {
		if strings.Contains(e.Name(), "rules.json.bak-") {
			backup = filepath.Join(filepath.Dir(path), e.Name())
		}
	}
	if backup == "" {
		t.Fatal("expected a backup file (rules.json.bak-*)")
	}
	backupContent, _ := os.ReadFile(backup)
	if string(backupContent) != original {
		t.Fatalf("backup must hold the original bytes; got: %s", string(backupContent))
	}
}

func TestConfigMigrate_WriteToOutPath(t *testing.T) {
	path := writeMigrateConfig(t, "exported.json", `{"version":"1","exportedAt":"2026-01-01T00:00:00Z","rules":[{"name":"API","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true}]}`)
	outPath := filepath.Join(filepath.Dir(path), "rules.json")

	_, _, code := runMigrate(t, false, "--write", "--out", outPath, path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected migrated file at --out path: %v", err)
	}
	written, _ := os.ReadFile(outPath)
	if !strings.HasPrefix(strings.TrimSpace(string(written)), "[") {
		t.Fatalf("migrated --out file should be a bare array, got: %s", string(written))
	}
}

func TestConfigMigrate_MalformedNoWrite(t *testing.T) {
	path := writeMigrateConfig(t, "rules.json", "this is not json")
	before, _ := os.ReadFile(path)
	_, _, code := runMigrate(t, false, "--write", path)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("malformed config must not be overwritten")
	}
}

func TestConfigMigrate_SchemaInvalidNoWrite(t *testing.T) {
	path := writeMigrateConfig(t, "rules.json", `[{"name":"","protocol":"nope","listenHost":"","listenPort":0,"targetHost":"","targetPort":0,"enabled":true}]`)
	out, _, code := runMigrate(t, false, "--write", path)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(out, "invalid") {
		t.Fatalf("expected invalid message, got: %s", out)
	}
}

func TestConfigMigrate_UnsupportedVersionNoWrite(t *testing.T) {
	path := writeMigrateConfig(t, "rules.json", `{"version":"2","rules":[]}`)
	before, _ := os.ReadFile(path)
	out, _, code := runMigrate(t, false, "--write", path)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(out, "unsupported config version") {
		t.Fatalf("expected unsupported-version message, got: %s", out)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("unsupported-version config must not be overwritten")
	}
}

func TestConfigMigrate_UnsupportedVersionJSON(t *testing.T) {
	path := writeMigrateConfig(t, "rules.json", `{"version":"99","rules":[]}`)
	out, _, code := runMigrate(t, true, path)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, out)
	}
	if report["format"] != "unsupported" {
		t.Fatalf("format = %v, want unsupported", report["format"])
	}
}

func TestConfigMigrate_MissingArg_Exit2(t *testing.T) {
	_, errBuf, code := runMigrate(t, false)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errBuf, "requires a config file path") {
		t.Fatalf("expected usage error, got: %s", errBuf)
	}
}

func TestConfigMigrate_UnreadableFile_Exit1(t *testing.T) {
	_, errBuf, code := runMigrate(t, false, filepath.Join(t.TempDir(), "does-not-exist.json"))
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errBuf, "Error reading") {
		t.Fatalf("expected read error, got: %s", errBuf)
	}
}

func TestConfigMigrate_Help_Exit0(t *testing.T) {
	out, _, code := runMigrate(t, false, "--help")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "portier config migrate") {
		t.Fatalf("expected help text, got: %s", out)
	}
}
