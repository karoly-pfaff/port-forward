package commands_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/client"
	"portier/cli/sources/commands"
)

// --- fixtures ---

func diagRuntimeFixture() map[string]any {
	return map[string]any{
		"name": "Portier", "version": "1.3.0-dev",
		"runtime": "node", "platform": "linux", "arch": "amd64",
		"uptimeSeconds": 100, "startedAt": "2026-01-01T12:00:00Z",
		"managementHost": "127.0.0.1", "managementPort": 47831,
		"configPath": "/etc/portier/rules.json", "staticDir": "web",
		"serviceMode": false, "pid": 1234,
	}
}

func diagRulesFixture() []map[string]any {
	return []map[string]any{
		{
			"id": "r1", "name": "API", "protocol": "tcp",
			"listenHost": "127.0.0.1", "listenPort": 48000,
			"targetHost": "10.0.0.1", "targetPort": 8080,
			"enabled": true, "advisories": []any{},
		},
		{
			"id": "r2", "name": "DB", "protocol": "tcp",
			"listenHost": "127.0.0.1", "listenPort": 48001,
			"targetHost": "10.0.0.1", "targetPort": 5432,
			"enabled": false, "advisories": []any{},
		},
	}
}

func diagStatusesFixture() []map[string]any {
	return []map[string]any{
		{"ruleId": "r1", "running": true, "bytesIn": 0, "bytesOut": 0},
		{"ruleId": "r2", "running": false, "bytesIn": 0, "bytesOut": 0},
	}
}

func diagActivityFixture() map[string]any {
	return map[string]any{
		"events": []map[string]any{
			{
				"id": "e1", "timestamp": "2026-01-01T12:00:00Z",
				"type": "rule_started", "severity": "info", "message": "Rule started",
			},
		},
	}
}

func diagDiagnoseFixture(ruleID string) map[string]any {
	return map[string]any{
		"ruleId": ruleID, "ruleName": "API", "protocol": "tcp",
		"summary":     map[string]any{"status": "pass", "message": "All checks passed"},
		"checks":      []any{},
		"diagnosedAt": "2026-01-01T12:00:00Z",
	}
}

// --- server helper ---

type diagSrvOpts struct {
	failRuntime  bool
	failRules    bool
	failStatuses bool
	failActivity bool
	diagnoseFunc http.HandlerFunc
	onActivity   func(r *http.Request)
}

func makeDiagSrv(t *testing.T, opts diagSrvOpts) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/runtime":
			if opts.failRuntime {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]any{"errors": []string{"runtime error"}})
				return
			}
			json.NewEncoder(w).Encode(diagRuntimeFixture())
		case r.Method == http.MethodGet && r.URL.Path == "/api/forwards":
			if opts.failRules {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]any{"errors": []string{"rules error"}})
				return
			}
			json.NewEncoder(w).Encode(diagRulesFixture())
		case r.Method == http.MethodGet && r.URL.Path == "/api/status":
			if opts.failStatuses {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]any{"errors": []string{"statuses error"}})
				return
			}
			json.NewEncoder(w).Encode(diagStatusesFixture())
		case r.Method == http.MethodGet && r.URL.Path == "/api/activity":
			if opts.onActivity != nil {
				opts.onActivity(r)
			}
			if opts.failActivity {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]any{"errors": []string{"activity error"}})
				return
			}
			json.NewEncoder(w).Encode(diagActivityFixture())
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/diagnose"):
			if opts.diagnoseFunc != nil {
				opts.diagnoseFunc(w, r)
				return
			}
			t.Errorf("unexpected diagnose call %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func readBundleFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading bundle file: %v", err)
	}
	var bundle map[string]any
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("parsing bundle JSON: %v\ncontent: %s", err, data)
	}
	return bundle
}

// --- dispatch tests ---

func TestRunDiagnostics_NoArgs(t *testing.T) {
	c := client.New("http://127.0.0.1:47831")
	errOut := &strings.Builder{}
	code := commands.RunDiagnostics(c, false, []string{}, &strings.Builder{}, errOut)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "diagnostics") {
		t.Errorf("stderr should mention 'diagnostics', got: %s", errOut.String())
	}
}

func TestRunDiagnostics_UnknownSubcommand(t *testing.T) {
	c := client.New("http://127.0.0.1:47831")
	errOut := &strings.Builder{}
	code := commands.RunDiagnostics(c, false, []string{"unknown"}, &strings.Builder{}, errOut)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunDiagnostics_Help(t *testing.T) {
	c := client.New("http://127.0.0.1:47831")
	out := &strings.Builder{}
	code := commands.RunDiagnostics(c, false, []string{"help"}, out, &strings.Builder{})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "export") {
		t.Errorf("help should mention 'export', got: %s", out.String())
	}
}

// --- export usage validation tests ---

func TestRunDiagnosticsExport_MissingOutHumanMode(t *testing.T) {
	c := client.New("http://127.0.0.1:47831")
	errOut := &strings.Builder{}
	code := commands.RunDiagnostics(c, false, []string{"export"}, &strings.Builder{}, errOut)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "--out") {
		t.Errorf("error should mention --out, got: %s", errOut.String())
	}
}

func TestRunDiagnosticsExport_InvalidActivityLimitLow(t *testing.T) {
	c := client.New("http://127.0.0.1:47831")
	errOut := &strings.Builder{}
	code := commands.RunDiagnostics(c, false, []string{"export", "--out", "x.json", "--activity-limit", "0"}, &strings.Builder{}, errOut)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "--activity-limit") {
		t.Errorf("error should mention --activity-limit, got: %s", errOut.String())
	}
}

func TestRunDiagnosticsExport_InvalidActivityLimitHigh(t *testing.T) {
	c := client.New("http://127.0.0.1:47831")
	errOut := &strings.Builder{}
	code := commands.RunDiagnostics(c, false, []string{"export", "--out", "x.json", "--activity-limit", "501"}, &strings.Builder{}, errOut)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

// --- happy path export tests ---

func TestRunDiagnosticsExport_WritesFile(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	out := &strings.Builder{}
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile}, out, errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Exported diagnostics to") {
		t.Errorf("expected success message, got: %s", out.String())
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Errorf("output file not created: %v", err)
	}
}

func TestRunDiagnosticsExport_HumanPartialFailure(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{failActivity: true})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	out := &strings.Builder{}
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile}, out, errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "with warnings") {
		t.Errorf("expected 'with warnings' message, got: %s", out.String())
	}
}

func TestRunDiagnosticsExport_JSONNoOutPrintsBundle(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{})
	defer srv.Close()

	c := client.New(srv.URL)
	out := &strings.Builder{}
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, true, []string{"export"}, out, errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	var bundle map[string]any
	if err := json.Unmarshal([]byte(out.String()), &bundle); err != nil {
		t.Fatalf("parsing bundle JSON: %v\noutput: %s", err, out.String())
	}
	if bundle["schemaVersion"] != "1" {
		t.Errorf("schemaVersion = %v, want \"1\"", bundle["schemaVersion"])
	}
}

func TestRunDiagnosticsExport_JSONWithOutPrintsResult(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	out := &strings.Builder{}
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, true, []string{"export", "--out", outFile}, out, errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("parsing result JSON: %v\noutput: %s", err, out.String())
	}
	if result["ok"] != true {
		t.Errorf("ok = %v, want true", result["ok"])
	}
	if result["path"] != outFile {
		t.Errorf("path = %v, want %q", result["path"], outFile)
	}
	if _, ok := result["ruleCount"]; !ok {
		t.Error("result should include ruleCount")
	}
}

// --- behavior tests ---

func TestRunDiagnosticsExport_RunDiagnosticsCallsDiagnose(t *testing.T) {
	diagnoseCalled := map[string]bool{}
	srv := makeDiagSrv(t, diagSrvOpts{
		diagnoseFunc: func(w http.ResponseWriter, r *http.Request) {
			// extract rule ID from path /api/forwards/:id/diagnose
			parts := strings.Split(r.URL.Path, "/")
			if len(parts) >= 4 {
				diagnoseCalled[parts[3]] = true
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(diagDiagnoseFixture("r1"))
		},
	})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	out := &strings.Builder{}
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile, "--run-diagnostics"}, out, errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	if !diagnoseCalled["r1"] {
		t.Error("diagnose should have been called for rule r1")
	}
	if !diagnoseCalled["r2"] {
		t.Error("diagnose should have been called for rule r2")
	}
}

func TestRunDiagnosticsExport_ActivityLimitPassed(t *testing.T) {
	var capturedLimit string
	srv := makeDiagSrv(t, diagSrvOpts{
		onActivity: func(r *http.Request) {
			capturedLimit = r.URL.Query().Get("limit")
		},
	})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile, "--activity-limit", "50"}, &strings.Builder{}, errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	if capturedLimit != "50" {
		t.Errorf("activity limit = %q, want \"50\"", capturedLimit)
	}
}

func TestRunDiagnosticsExport_RulesFailureSkipsDiagnostics(t *testing.T) {
	diagnoseCalled := false
	srv := makeDiagSrv(t, diagSrvOpts{
		failRules: true,
		diagnoseFunc: func(w http.ResponseWriter, r *http.Request) {
			diagnoseCalled = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(diagDiagnoseFixture("r1"))
		},
	})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile, "--run-diagnostics"}, &strings.Builder{}, errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	if diagnoseCalled {
		t.Error("diagnose should not be called when rules fetch fails")
	}
	bundle := readBundleFile(t, outFile)
	errs, _ := bundle["errors"].([]any)
	found := false
	for _, e := range errs {
		em, _ := e.(map[string]any)
		if em["source"] == "rules" {
			found = true
		}
	}
	if !found {
		t.Error("bundle errors should include source=rules")
	}
}

func TestRunDiagnosticsExport_PartialFailureStillWritesBundle(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{failRuntime: true, failActivity: true})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile}, &strings.Builder{}, errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Error("bundle file should be written even on partial failure")
	}
	bundle := readBundleFile(t, outFile)
	errs, _ := bundle["errors"].([]any)
	if len(errs) == 0 {
		t.Error("errors array should be non-empty on partial failure")
	}
}

func TestRunDiagnosticsExport_WriteFailure(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{})
	defer srv.Close()

	c := client.New(srv.URL)
	badPath := filepath.Join(t.TempDir(), "nonexistent", "bundle.json")
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", badPath}, &strings.Builder{}, errOut)
	if code == 0 {
		t.Error("exit code should be nonzero when write fails")
	}
}

// --- bundle content tests ---

func TestRunDiagnosticsExport_BundleSchemaVersion(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile}, &strings.Builder{}, &strings.Builder{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	bundle := readBundleFile(t, outFile)
	if bundle["schemaVersion"] != "1" {
		t.Errorf("schemaVersion = %v, want \"1\"", bundle["schemaVersion"])
	}
}

func TestRunDiagnosticsExport_MetadataSourceIsCLI(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile}, &strings.Builder{}, &strings.Builder{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	bundle := readBundleFile(t, outFile)
	meta, _ := bundle["metadata"].(map[string]any)
	if meta["source"] != "cli" {
		t.Errorf("metadata.source = %v, want \"cli\"", meta["source"])
	}
}

func TestRunDiagnosticsExport_DiagnosticsNoteWhenNotRun(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile}, &strings.Builder{}, &strings.Builder{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	bundle := readBundleFile(t, outFile)
	note, _ := bundle["diagnosticsNote"].(string)
	if !strings.Contains(note, "--run-diagnostics") {
		t.Errorf("diagnosticsNote should mention --run-diagnostics, got: %s", note)
	}
}

func TestRunDiagnosticsExport_RunDiagnosticsHumanOutput(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{
		diagnoseFunc: func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Split(r.URL.Path, "/")
			ruleID := "r1"
			if len(parts) >= 4 {
				ruleID = parts[3]
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(diagDiagnoseFixture(ruleID))
		},
	})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	out := &strings.Builder{}
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile, "--run-diagnostics"}, out, errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "diagnostics results") {
		t.Errorf("expected 'diagnostics results' in output, got: %s", out.String())
	}
}

func TestRunDiagnosticsExport_JSONWithOutIncludesCounts(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	out := &strings.Builder{}

	code := commands.RunDiagnostics(c, true, []string{"export", "--out", outFile}, out, &strings.Builder{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if result["ruleCount"] != float64(2) {
		t.Errorf("ruleCount = %v, want 2", result["ruleCount"])
	}
	if result["statusCount"] != float64(2) {
		t.Errorf("statusCount = %v, want 2", result["statusCount"])
	}
}

func TestRunDiagnosticsExport_HelpFlag(t *testing.T) {
	c := client.New("http://127.0.0.1:47831")
	out := &strings.Builder{}
	code := commands.RunDiagnostics(c, false, []string{"export", "--help"}, out, &strings.Builder{})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier diagnostics export") {
		t.Errorf("help output missing usage: %s", out.String())
	}
}

func TestRunDiagnosticsExport_EmptyRulesRunDiagnostics(t *testing.T) {
	// Empty rules list with --run-diagnostics → "No rules were available" note
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/runtime":
			json.NewEncoder(w).Encode(diagRuntimeFixture())
		case r.URL.Path == "/api/forwards":
			json.NewEncoder(w).Encode([]any{})
		case r.URL.Path == "/api/status":
			json.NewEncoder(w).Encode([]any{})
		case r.URL.Path == "/api/activity":
			json.NewEncoder(w).Encode(map[string]any{"events": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile, "--run-diagnostics"}, &strings.Builder{}, &strings.Builder{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	bundle := readBundleFile(t, outFile)
	note, _ := bundle["diagnosticsNote"].(string)
	if !strings.Contains(note, "No rules") {
		t.Errorf("diagnosticsNote should mention no rules, got: %s", note)
	}
}

func TestRunDiagnosticsExport_PerRuleDiagnoseError(t *testing.T) {
	// Per-rule diagnose fails → recorded in errors[], bundle still written
	srv := makeDiagSrv(t, diagSrvOpts{
		diagnoseFunc: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"errors": []string{"diagnose failed"}})
		},
	})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	errOut := &strings.Builder{}

	code := commands.RunDiagnostics(c, false, []string{"export", "--out", outFile, "--run-diagnostics"}, &strings.Builder{}, errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	bundle := readBundleFile(t, outFile)
	errs, _ := bundle["errors"].([]any)
	found := false
	for _, e := range errs {
		em, _ := e.(map[string]any)
		src, _ := em["source"].(string)
		if strings.HasPrefix(src, "diagnostics:") {
			found = true
		}
	}
	if !found {
		t.Error("bundle errors should include diagnostics: source entries for per-rule failures")
	}
}

func TestRunDiagnosticsExport_JSONWithWarnings(t *testing.T) {
	// JSON output + --out + partial failure → result includes warningCount
	srv := makeDiagSrv(t, diagSrvOpts{failRuntime: true})
	defer srv.Close()

	c := client.New(srv.URL)
	outFile := filepath.Join(t.TempDir(), "bundle.json")
	out := &strings.Builder{}

	code := commands.RunDiagnostics(c, true, []string{"export", "--out", outFile}, out, &strings.Builder{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if _, ok := result["warningCount"]; !ok {
		t.Error("result should include warningCount when partial failures occur")
	}
}

func TestRunDiagnosticsExport_NoForbiddenFields(t *testing.T) {
	srv := makeDiagSrv(t, diagSrvOpts{})
	defer srv.Close()

	c := client.New(srv.URL)
	out := &strings.Builder{}

	code := commands.RunDiagnostics(c, true, []string{"export"}, out, &strings.Builder{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	content := out.String()
	for _, forbidden := range []string{`"env"`, `"username"`, `"logs"`, `"homedir"`, `"environ"`} {
		if strings.Contains(content, forbidden) {
			t.Errorf("bundle should not contain key %s", forbidden)
		}
	}
}
