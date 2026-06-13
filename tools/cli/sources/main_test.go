package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jsonPath makes a filesystem path safe to embed in a JSON string literal by
// using forward slashes (accepted by os.ReadFile on Windows too), avoiding
// backslash-escaping issues on Windows.
func jsonPath(p string) string { return strings.ReplaceAll(p, "\\", "/") }

// makeDispatchServer creates a test server that handles all command API endpoints.
func makeDispatchServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/runtime":
			json.NewEncoder(w).Encode(map[string]any{
				"name": "Portier", "version": "1.3.0-dev",
				"runtime": "go", "platform": "linux", "arch": "amd64",
				"uptimeSeconds": 10, "startedAt": "2026-01-01T00:00:00Z",
				"managementHost": "127.0.0.1", "managementPort": 47831,
				"configPath": "/tmp/rules.json", "staticDir": "web",
				"serviceMode": false, "pid": 1234,
			})
		case r.URL.Path == "/api/forwards":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": "r1", "name": "Dev", "protocol": "tcp",
					"listenHost": "127.0.0.1", "listenPort": 48000,
					"targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true,
					"advisories": []any{},
				},
			})
		case r.URL.Path == "/api/status":
			json.NewEncoder(w).Encode([]map[string]any{
				{"ruleId": "r1", "running": true, "bytesIn": 0, "bytesOut": 0},
			})
		case r.URL.Path == "/api/activity":
			json.NewEncoder(w).Encode(map[string]any{"events": []any{}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/start"):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/stop"):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/diagnose"):
			json.NewEncoder(w).Encode(map[string]any{
				"ruleId": "r1", "ruleName": "Dev", "protocol": "tcp",
				"summary": map[string]any{"status": "pass", "message": "All checks passed."},
				"checks":  []any{}, "diagnosedAt": "2026-01-01T00:00:00Z",
			})
		case r.URL.Path == "/api/config/export":
			json.NewEncoder(w).Encode(map[string]any{
				"version": "1", "exportedAt": "2026-01-01T00:00:00Z",
				"rules": []any{},
			})
		default:
			t.Logf("dispatch server: unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

// --- flag / help / version tests ---

func TestRun_Version(t *testing.T) {
	code := run([]string{"--version"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_HelpFlag(t *testing.T) {
	code := run([]string{"--help"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_ShortHelpFlag(t *testing.T) {
	code := run([]string{"-h"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_NoArgs(t *testing.T) {
	code := run([]string{})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_InvalidFlag(t *testing.T) {
	// --port requires an int; passing a non-int causes a parse error → exit 2
	code := run([]string{"--port", "abc"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

// --- built-in subcommand tests ---

func TestRun_HelpCommand(t *testing.T) {
	code := run([]string{"help"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_VersionCommand(t *testing.T) {
	code := run([]string{"version"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	code := run([]string{"unknown-cmd"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRun_InvalidURL(t *testing.T) {
	// ftp:// is rejected by validateURL → exit 2
	code := run([]string{"--url", "ftp://bad-scheme", "runtime"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

// --- command dispatch tests ---

func TestRun_RuntimeDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	code := run([]string{"--url", srv.URL, "runtime"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_ListDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	code := run([]string{"--url", srv.URL, "list"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_StatusDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	code := run([]string{"--url", srv.URL, "status"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_ActivityDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	code := run([]string{"--url", srv.URL, "activity"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_StartDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	code := run([]string{"--url", srv.URL, "start", "r1"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_StopDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	code := run([]string{"--url", srv.URL, "stop", "r1"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_DiagnoseDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	code := run([]string{"--url", srv.URL, "diagnose", "r1"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_DoctorDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	code := run([]string{"--url", srv.URL, "doctor"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_DoctorStrictDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	// The dispatch server reports a version-mismatch (runtime 1.3.0-dev vs CLI)
	// → a warning. Without --strict this would exit 0; with --strict the warning
	// fails the run, which proves --strict is wired through dispatch.
	code := run([]string{"--url", srv.URL, "doctor", "--strict"})
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (warning under --strict)", code)
	}
	if normal := run([]string{"--url", srv.URL, "doctor"}); normal != 0 {
		t.Errorf("non-strict exit = %d, want 0 (warning alone does not fail)", normal)
	}
}

func TestRun_ConfigDoctorStrictDispatch(t *testing.T) {
	// A valid config with a LAN-exposure warning exits 1 under --strict.
	dir := t.TempDir()
	file := filepath.Join(dir, "warn.json")
	content := `[{"name":"Web","protocol":"tcp","listenHost":"0.0.0.0","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true}]`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	code := run([]string{"config", "doctor", "--strict", file})
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (warning under --strict)", code)
	}
}

func TestRun_DoctorOutDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "doctor-report.json")
	// Dispatch server has a version-mismatch warning → exit 0 in normal mode,
	// and the report file is written.
	code := run([]string{"--url", srv.URL, "doctor", "--out", out})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("expected report file at %s: %v", out, err)
	}
}

func TestRun_ConfigDoctorOutDispatch(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "rules.json")
	content := `[{"name":"API","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true}]`
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	out := filepath.Join(dir, "config-doctor-report.json")
	code := run([]string{"config", "doctor", "--out", out, cfg})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("expected report file at %s: %v", out, err)
	}
}

func TestRun_SupportBundleDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "bundle")
	// Dispatch server has a version-mismatch warning → exit 0 in normal mode.
	code := run([]string{"--url", srv.URL, "support-bundle", "--out", out})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(out, "manifest.json")); err != nil {
		t.Errorf("expected manifest.json in bundle: %v", err)
	}
}

func TestRun_ExplainDispatch(t *testing.T) {
	// explain is fully offline — no server needed, and an invalid --url is ignored.
	code := run([]string{"--url", "ftp://bad-scheme", "explain", "config.valid"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_ExplainListDispatch(t *testing.T) {
	code := run([]string{"explain", "--list"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_WorkflowPlanDispatch(t *testing.T) {
	// workflow plan is fully offline — no server needed, and an invalid --url is
	// ignored. A valid workflow file exits 0.
	dir := t.TempDir()
	wf := filepath.Join(dir, "workflow.json")
	wfContent := `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`
	if err := os.WriteFile(wf, []byte(wfContent), 0o644); err != nil {
		t.Fatalf("writing temp workflow: %v", err)
	}
	code := run([]string{"--url", "ftp://bad-scheme", "workflow", "plan", "--file", wf})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_WorkflowRunDispatch(t *testing.T) {
	// A config-only workflow run executes offline (no runtime step), so an invalid
	// --url is never resolved. A passing run exits 0.
	dir := t.TempDir()
	cfg := filepath.Join(dir, "cfg.json")
	pol := filepath.Join(dir, "pol.json")
	wf := filepath.Join(dir, "workflow.json")
	if err := os.WriteFile(cfg, []byte(`[{"name":"A","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48080,"targetHost":"h","targetPort":1,"enabled":false,"group":"g"}]`), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	if err := os.WriteFile(pol, []byte(`{"schemaVersion":1,"rules":{}}`), 0o644); err != nil {
		t.Fatalf("writing policy: %v", err)
	}
	wfContent := `{"schemaVersion":1,"steps":[{"id":"c","type":"policy.check","config":"` + jsonPath(cfg) + `","policy":"` + jsonPath(pol) + `"}]}`
	if err := os.WriteFile(wf, []byte(wfContent), 0o644); err != nil {
		t.Fatalf("writing workflow: %v", err)
	}
	code := run([]string{"--url", "ftp://bad-scheme", "workflow", "run", "--file", wf})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_WorkflowRunbookDispatch(t *testing.T) {
	// workflow runbook is fully offline — no server needed, an invalid --url is
	// ignored. A valid workflow file produces a runbook and exits 0.
	dir := t.TempDir()
	wf := filepath.Join(dir, "workflow.json")
	wfContent := `{"schemaVersion":1,"steps":[{"id":"a","type":"policy.check","runtime":true,"policy":"p.json"}]}`
	if err := os.WriteFile(wf, []byte(wfContent), 0o644); err != nil {
		t.Fatalf("writing temp workflow: %v", err)
	}
	code := run([]string{"--url", "ftp://bad-scheme", "workflow", "runbook", "--file", wf})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_WorkflowTemplateDispatch(t *testing.T) {
	// workflow template is fully offline — no server needed, an invalid --url is
	// ignored. Rendering a built-in template exits 0.
	code := run([]string{"--url", "ftp://bad-scheme", "workflow", "template", "policy-check-local"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_WorkflowDispatch_NoSubcommand(t *testing.T) {
	if code := run([]string{"workflow"}); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRun_PolicyCheckDispatch(t *testing.T) {
	// policy check is fully offline — no server needed, and an invalid --url is
	// ignored. A compliant config against a strict policy exits 0.
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")
	cfgContent := `[{"name":"Admin","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48080,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false,"group":"admin"}]`
	if err := os.WriteFile(cfg, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	pol := filepath.Join(dir, "policy.json")
	polContent := `{"schemaVersion":1,"rules":{"requireGroup":true,"allowLanExposure":false,"allowPrivilegedPorts":false,"allowAutostart":false,"forbidDuplicateBindings":true}}`
	if err := os.WriteFile(pol, []byte(polContent), 0o644); err != nil {
		t.Fatalf("writing temp policy: %v", err)
	}
	code := run([]string{"--url", "ftp://bad-scheme", "policy", "check", "--config", cfg, "--policy", pol})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_PolicyDispatch_NoSubcommand(t *testing.T) {
	// policy with no subcommand → 2
	code := run([]string{"policy"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRun_PolicyCheckViolationDispatch(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")
	cfgContent := `[{"name":"Exposed","protocol":"tcp","listenHost":"0.0.0.0","listenPort":48080,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false,"group":"g"}]`
	if err := os.WriteFile(cfg, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	pol := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(pol, []byte(`{"schemaVersion":1,"rules":{"allowLanExposure":false}}`), 0o644); err != nil {
		t.Fatalf("writing temp policy: %v", err)
	}
	code := run([]string{"policy", "check", "--config", cfg, "--policy", pol})
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (violation)", code)
	}
}

func TestRun_ConfigDispatch_NoSubcommand(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	// config with no subcommand → 2
	code := run([]string{"--url", srv.URL, "config"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRun_ConfigExportDispatch(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	// --json + no --out → prints config JSON to stdout
	code := run([]string{"--json", "--url", srv.URL, "config", "export"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRun_DiagnosticsDispatch_NoSubcommand(t *testing.T) {
	srv := makeDispatchServer(t)
	defer srv.Close()
	// diagnostics with no subcommand → 2
	code := run([]string{"--url", srv.URL, "diagnostics"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

// TestRun_InvalidURL_AllDispatches covers the repeated ResolveURL-error arm in
// run() for every subcommand: an invalid --url must fail fast with exit 2
// before any client is constructed. (runtime is already covered by
// TestRun_InvalidURL; this covers the remaining dispatch arms.)
func TestRun_InvalidURL_AllDispatches(t *testing.T) {
	subcommands := [][]string{
		{"list"},
		{"status"},
		{"activity"},
		{"start", "rule-1"},
		{"stop", "rule-1"},
		{"diagnose", "rule-1"},
		{"doctor"},
		{"support-bundle", "--out", "x"},
		{"config", "export"},
		{"group", "list"},
		{"diagnostics", "export"},
	}
	for _, sub := range subcommands {
		t.Run(sub[0], func(t *testing.T) {
			args := append([]string{"--url", "ftp://bad-scheme"}, sub...)
			if code := run(args); code != 2 {
				t.Errorf("run(%v) exit = %d, want 2", args, code)
			}
		})
	}
}
