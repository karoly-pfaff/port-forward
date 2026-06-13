package commands_test

// Black-box tests for `portier policy template` (v1.10 Slice 4): listing and
// rendering built-in policy templates, deterministic/sorted output, --json
// wrappers, --out producing a file usable by `policy check`, and the usage/exit
// codes (unknown/missing template → 2, missing --out value → 2, write failure → 1).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

func runPolicyTemplate(t *testing.T, jsonOutput bool, args ...string) (string, string, int) {
	t.Helper()
	var out, errBuf strings.Builder
	code := commands.RunPolicyTemplate(jsonOutput, args, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// --- list ---

func TestPolicyTemplate_ListHuman(t *testing.T) {
	out, _, code := runPolicyTemplate(t, false, "--list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Policy templates") {
		t.Errorf("list output missing header:\n%s", out)
	}
	for _, name := range []string{"local-safe", "managed", "permissive"} {
		if !strings.Contains(out, name) {
			t.Errorf("list output missing template %q:\n%s", name, out)
		}
	}
	// Deterministic alphabetical order: local-safe < managed < permissive.
	li := strings.Index(out, "local-safe")
	mi := strings.Index(out, "managed")
	pi := strings.Index(out, "permissive")
	if !(li < mi && mi < pi) {
		t.Errorf("templates not listed in sorted order (local-safe=%d managed=%d permissive=%d):\n%s", li, mi, pi, out)
	}
}

func TestPolicyTemplate_ListJSON(t *testing.T) {
	out, _, code := runPolicyTemplate(t, true, "--list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var raw struct {
		Templates []struct {
			Name        string `json:"name"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Policy      any    `json:"policy"`
		} `json:"templates"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("decoding list JSON: %v\n%s", err, out)
	}
	if len(raw.Templates) < 3 {
		t.Fatalf("expected >= 3 templates, got %d", len(raw.Templates))
	}
	var names []string
	for _, e := range raw.Templates {
		if e.Name == "" || e.Title == "" || e.Description == "" {
			t.Errorf("list entry missing metadata: %+v", e)
		}
		if e.Policy != nil {
			t.Errorf("list entry should not embed policy: %+v", e)
		}
		names = append(names, e.Name)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Errorf("list JSON not sorted by name: %v", names)
		}
	}
}

// --- single render ---

// decodePolicyFile asserts a string is a bare policy file and returns its rules.
func decodePolicyFile(t *testing.T, s string) map[string]any {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		t.Fatalf("decoding policy JSON: %v\n%s", err, s)
	}
	if raw["schemaVersion"] != float64(1) {
		t.Errorf("schemaVersion = %v, want 1: %s", raw["schemaVersion"], s)
	}
	rules, ok := raw["rules"].(map[string]any)
	if !ok {
		t.Fatalf("policy JSON missing rules object: %s", s)
	}
	// Must not carry metadata (bare policy only) or any protocol/udp rule.
	if _, bad := raw["name"]; bad {
		t.Errorf("bare policy must not include metadata: %s", s)
	}
	for k := range rules {
		if strings.Contains(strings.ToLower(k), "udp") || strings.Contains(strings.ToLower(k), "protocol") {
			t.Errorf("policy rules must not contain a protocol/udp rule (%q): %s", k, s)
		}
	}
	return rules
}

func TestPolicyTemplate_RenderEach(t *testing.T) {
	for _, name := range []string{"permissive", "local-safe", "managed"} {
		out, _, code := runPolicyTemplate(t, false, name)
		if code != 0 {
			t.Fatalf("%s: exit code = %d, want 0", name, code)
		}
		rules := decodePolicyFile(t, out)
		// All five standard guardrails are present and boolean.
		for _, key := range []string{"requireGroup", "allowLanExposure", "allowPrivilegedPorts", "allowAutostart", "forbidDuplicateBindings"} {
			if _, ok := rules[key].(bool); !ok {
				t.Errorf("%s: rules missing boolean %q: %v", name, key, rules)
			}
		}
	}
}

func TestPolicyTemplate_SingleJSONWrapper(t *testing.T) {
	out, _, code := runPolicyTemplate(t, true, "local-safe")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var raw struct {
		Name        string         `json:"name"`
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Policy      map[string]any `json:"policy"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("decoding wrapper JSON: %v\n%s", err, out)
	}
	if raw.Name != "local-safe" || raw.Title == "" || raw.Description == "" {
		t.Errorf("wrapper metadata wrong: %+v", raw)
	}
	if raw.Policy["schemaVersion"] != float64(1) {
		t.Errorf("wrapper policy schemaVersion = %v, want 1", raw.Policy["schemaVersion"])
	}
	if _, ok := raw.Policy["rules"]; !ok {
		t.Errorf("wrapper policy missing rules: %+v", raw.Policy)
	}
}

// --- --out producing a usable policy file ---

func TestPolicyTemplate_OutWritesUsableFile(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "policy.json")
	out, _, code := runPolicyTemplate(t, false, "managed", "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Policy written to") {
		t.Errorf("human stdout should confirm the write:\n%s", out)
	}
	// The written file is a bare policy file (no metadata wrapper)...
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading written policy: %v", err)
	}
	decodePolicyFile(t, string(data))

	// ...and it is directly usable by `policy check`. A compliant config passes.
	cfg := writeTempFile(t, "config.json", compliantConfig)
	checkCode := commands.RunPolicyCheck(false, []string{"--config", cfg, "--policy", outPath}, &strings.Builder{}, &strings.Builder{})
	if checkCode != 0 {
		t.Errorf("generated managed policy should validate a compliant config (exit %d, want 0)", checkCode)
	}
}

func TestPolicyTemplate_JSONOutStdoutWrapperFileBare(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "policy.json")
	out, _, code := runPolicyTemplate(t, true, "managed", "--out", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// stdout is the metadata wrapper (has "name"); the file is the bare policy.
	if !strings.Contains(out, `"name"`) {
		t.Errorf("--json stdout should be the metadata wrapper:\n%s", out)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading written policy: %v", err)
	}
	if strings.Contains(string(data), `"name"`) {
		t.Errorf("written file should be the bare policy, not the wrapper:\n%s", data)
	}
	decodePolicyFile(t, string(data))
}

// --- error behavior ---

func TestPolicyTemplate_UnknownExits2(t *testing.T) {
	_, errBuf, code := runPolicyTemplate(t, false, "nope")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf, "unknown template") || !strings.Contains(errBuf, "local-safe") {
		t.Errorf("stderr should name the unknown template and list available: %q", errBuf)
	}
}

func TestPolicyTemplate_MissingNameExits2(t *testing.T) {
	_, _, code := runPolicyTemplate(t, false)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestPolicyTemplate_TooManyArgsExits2(t *testing.T) {
	_, _, code := runPolicyTemplate(t, false, "managed", "permissive")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestPolicyTemplate_MissingOutValueExits2(t *testing.T) {
	// `--out` with no following value → flag parse error → exit 2.
	_, _, code := runPolicyTemplate(t, false, "managed", "--out")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestPolicyTemplate_WriteFailureExits1(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "no-such-dir", "policy.json")
	_, errBuf, code := runPolicyTemplate(t, false, "managed", "--out", badPath)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (write failure)", code)
	}
	if !strings.Contains(errBuf, "Error writing") {
		t.Errorf("stderr should report the write failure: %q", errBuf)
	}
	if _, err := os.Stat(badPath); !os.IsNotExist(err) {
		t.Errorf("no file should have been created at %s", badPath)
	}
}

func TestPolicyTemplate_ListWithNameExits2(t *testing.T) {
	_, _, code := runPolicyTemplate(t, false, "--list", "managed")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestPolicyTemplate_ListWithOutExits2(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "policy.json")
	_, _, code := runPolicyTemplate(t, false, "--list", "--out", outPath)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("no file should be written for --list --out: %s", outPath)
	}
}

// --- dispatch ---

func TestPolicyTemplate_ViaRunPolicy(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunPolicy(false, []string{"template", "--list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("RunPolicy template --list exit = %d, want 0\nstderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Policy templates") {
		t.Errorf("RunPolicy did not route to template list:\n%s", out.String())
	}
}

func TestPolicyTemplate_Help(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunPolicyTemplate(false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("--help exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "policy template") {
		t.Errorf("help should describe the command:\n%s", out.String())
	}
}
