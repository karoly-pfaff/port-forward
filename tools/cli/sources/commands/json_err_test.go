package commands_test

// Coverage Slice C: every command that emits machine-readable output funnels
// through output.PrintJSON and maps a write/encode failure to "Error encoding
// JSON" on stderr + exit code 1. These tests drive that failure path with a
// stdout writer that always fails, asserting the user-visible behavior (exit 1
// and a useful message) rather than the implementation. No real service is
// required — a local httptest server returns just enough valid JSON for each
// command to reach its PrintJSON call.

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

// failingWriter fails every Write, simulating a broken stdout (e.g. a closed
// pipe) so that json.Encoder.Encode returns an error.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, os.ErrClosed
}

// okAPIServer returns 200 + minimally-valid JSON for every endpoint the CLI
// calls, so a command in --json mode reaches its PrintJSON step.
func okAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case path == "/api/runtime":
			json.NewEncoder(w).Encode(client.RuntimeInfo{Name: "Portier", ManagementHost: "127.0.0.1", ManagementPort: 47831})
		case path == "/api/forwards" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode([]client.ForwardRuleResponse{{
				ID: "rule-1", Name: "R1", Protocol: "tcp",
				ListenHost: "127.0.0.1", ListenPort: 48080,
				TargetHost: "127.0.0.1", TargetPort: 9000, Enabled: true,
			}})
		case path == "/api/status":
			json.NewEncoder(w).Encode([]client.ForwardStatus{{RuleID: "rule-1", Running: true}})
		case path == "/api/activity":
			json.NewEncoder(w).Encode(map[string]any{"events": []client.ActivityEvent{{
				Type: "rule.started", Severity: "success", Message: "ok",
				Timestamp: "2026-01-01T00:00:00Z",
			}}})
		case strings.HasSuffix(path, "/diagnose"):
			json.NewEncoder(w).Encode(client.RuleDiagnosticsResult{
				RuleID: "rule-1", RuleName: "R1", Protocol: "tcp",
				Summary: client.DiagnosticSummary{Status: "pass", Message: "ok"},
			})
		case strings.HasSuffix(path, "/start"), strings.HasSuffix(path, "/stop"):
			json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case path == "/api/config/export":
			json.NewEncoder(w).Encode(client.ConfigExportResponse{Version: "1", Rules: []client.ConfigRule{}})
		case path == "/api/config/plan":
			json.NewEncoder(w).Encode(client.ConfigPlanResponse{Summary: client.ConfigPlanSummary{}})
		case path == "/api/config/apply":
			json.NewEncoder(w).Encode(client.ConfigApplyResponse{Ok: true})
		case path == "/api/config/import":
			json.NewEncoder(w).Encode(client.ConfigImportResponse{Result: client.ImportResult{Imported: 1}})
		default:
			http.NotFound(w, r)
		}
	}))
}

// assertEncodeError asserts the standard "encode failed" behavior: exit 1 and a
// stderr message mentioning JSON encoding.
func assertEncodeError(t *testing.T, code int, stderr string) {
	t.Helper()
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (JSON encode failure)", code)
	}
	if !strings.Contains(stderr, "Error encoding JSON") {
		t.Errorf("stderr should mention JSON encode failure, got: %q", stderr)
	}
}

// writeRuleConfigFile writes a single-rule config file and returns its path.
func writeRuleConfigFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	body := `[{"id":"rule-1","name":"R1","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48080,"targetHost":"127.0.0.1","targetPort":9000,"enabled":true}]`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestRunRuntime_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	var stderr strings.Builder
	code := commands.RunRuntime(client.New(srv.URL), true, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunList_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	var stderr strings.Builder
	code := commands.RunList(client.New(srv.URL), true, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunStatus_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	var stderr strings.Builder
	code := commands.RunStatus(client.New(srv.URL), true, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunStart_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	var stderr strings.Builder
	code := commands.RunStart(client.New(srv.URL), true, []string{"rule-1"}, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunStop_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	var stderr strings.Builder
	code := commands.RunStop(client.New(srv.URL), true, []string{"rule-1"}, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunDiagnose_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	var stderr strings.Builder
	code := commands.RunDiagnose(client.New(srv.URL), true, []string{"rule-1"}, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunActivity_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	var stderr strings.Builder
	code := commands.RunActivity(client.New(srv.URL), true, nil, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunConfigExport_StdoutJSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	var stderr strings.Builder
	// No --out, --json: the exported config prints to stdout via PrintJSON.
	code := commands.RunConfigExport(client.New(srv.URL), true, nil, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunConfigValidate_JSONEncodeError_Exit1(t *testing.T) {
	cfg := writeRuleConfigFile(t)
	var stderr strings.Builder
	code := commands.RunConfigValidate(true, []string{cfg}, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunConfigPlan_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	cfg := writeRuleConfigFile(t)
	var stderr strings.Builder
	code := commands.RunConfigPlan(client.New(srv.URL), true, []string{cfg}, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunConfigDiff_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	cfg := writeRuleConfigFile(t)
	var stderr strings.Builder
	code := commands.RunConfigDiff(client.New(srv.URL), true, []string{cfg}, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunConfigApply_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	cfg := writeRuleConfigFile(t)
	var stderr strings.Builder
	code := commands.RunConfigApply(client.New(srv.URL), true, []string{cfg}, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunConfigImport_JSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	cfg := writeRuleConfigFile(t)
	var stderr strings.Builder
	code := commands.RunConfigImport(client.New(srv.URL), true, []string{"--mode", "merge", cfg}, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}

func TestRunDiagnosticsExport_StdoutJSONEncodeError_Exit1(t *testing.T) {
	srv := okAPIServer(t)
	defer srv.Close()
	var stderr strings.Builder
	// No --out, --json: the full bundle prints to stdout via PrintJSON.
	code := commands.RunDiagnosticsExport(client.New(srv.URL), true, nil, failingWriter{}, &stderr)
	assertEncodeError(t, code, stderr.String())
}
