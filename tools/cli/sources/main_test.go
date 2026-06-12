package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
