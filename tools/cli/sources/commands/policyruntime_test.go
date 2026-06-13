package commands_test

// Black-box tests for `portier policy check --runtime` (v1.10 Slice 6): evaluating
// the live runtime config (read via the existing read-only config-export path)
// against a policy, reusing the offline policy check evaluator and report shape.
// Runtime mode is read-only — the test server only ever serves GET
// /api/config/export; any mutation would trip the handler's default case.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portier/cli/sources/commands"
)

// makeExportServer serves GET /api/config/export with the given rules and fails
// the test on any other request (so a mutation attempt is caught).
func makeExportServer(t *testing.T, rules []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/config/export" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version": "1", "exportedAt": "2026-01-01T12:00:00Z", "rules": rules,
			})
			return
		}
		t.Errorf("unexpected request %s %s (runtime policy check must be read-only)", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
}

func runPolicyCheckRuntime(t *testing.T, jsonOutput bool, serverURL, policyPath string, extra ...string) (string, string, int) {
	t.Helper()
	args := append([]string{"--runtime", "--policy", policyPath}, extra...)
	var out, errBuf strings.Builder
	code := commands.RunPolicyCheck(jsonOutput, commands.ConnFlags{URL: serverURL}, args, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// compliant runtime rule: grouped, loopback, high port, autostart off.
var compliantRuntimeRules = []map[string]any{
	{"id": "r1", "name": "Admin UI", "protocol": "tcp", "listenHost": "127.0.0.1",
		"listenPort": 48080, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": false, "group": "admin"},
}

// exposed runtime rule: listens on 0.0.0.0.
var exposedRuntimeRules = []map[string]any{
	{"id": "r1", "name": "Admin UI", "protocol": "tcp", "listenHost": "0.0.0.0",
		"listenPort": 80, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": false, "group": "admin"},
}

// --- pass / fail ---

func TestPolicyCheckRuntime_CompliantPassesExit0(t *testing.T) {
	srv := makeExportServer(t, compliantRuntimeRules)
	defer srv.Close()
	pol := writeTempFile(t, "pol.json", strictPolicy)

	out, _, code := runPolicyCheckRuntime(t, false, srv.URL, pol)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "Source: runtime") {
		t.Errorf("human output should show the runtime source:\n%s", out)
	}
	if !strings.Contains(out, "Result: passed") {
		t.Errorf("expected passed result:\n%s", out)
	}
}

func TestPolicyCheckRuntime_ViolationExit1(t *testing.T) {
	srv := makeExportServer(t, exposedRuntimeRules)
	defer srv.Close()
	pol := writeTempFile(t, "pol.json", `{"schemaVersion":1,"rules":{"allowLanExposure":false}}`)

	out, _, code := runPolicyCheckRuntime(t, true, srv.URL, pol)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, out)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out)
	}
	if raw["result"] != "failed" {
		t.Errorf("result = %v, want failed", raw["result"])
	}
	findings, _ := raw["findings"].([]any)
	if len(findings) == 0 {
		t.Fatalf("expected a finding for the exposed runtime rule")
	}
	// No new policy codes — the runtime evaluator reuses the offline codes.
	first, _ := findings[0].(map[string]any)
	if code, _ := first["code"].(string); !strings.HasPrefix(code, "policy.") {
		t.Errorf("unexpected finding code %q", code)
	}
}

// --- JSON / source field ---

func TestPolicyCheckRuntime_JSONShapeHasSource(t *testing.T) {
	srv := makeExportServer(t, compliantRuntimeRules)
	defer srv.Close()
	pol := writeTempFile(t, "pol.json", strictPolicy)

	out, _, code := runPolicyCheckRuntime(t, true, srv.URL, pol)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out)
	}
	if raw["source"] != "runtime" {
		t.Errorf("source = %v, want runtime", raw["source"])
	}
	for _, key := range []string{"findings", "summary", "result"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("runtime JSON missing %q: %s", key, out)
		}
	}
}

func TestPolicyCheck_OfflineJSONHasConfigFileSource(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", compliantConfig)
	pol := writeTempFile(t, "pol.json", strictPolicy)
	out, _, code := runPolicyCheckArgs(t, true, "--config", cfg, "--policy", pol)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("decoding JSON: %v\n%s", err, out)
	}
	if raw["source"] != "config file" {
		t.Errorf("offline source = %v, want \"config file\"", raw["source"])
	}
}

// --- --out / --explain ---

func TestPolicyCheckRuntime_OutJSONByteParity(t *testing.T) {
	srv := makeExportServer(t, exposedRuntimeRules)
	defer srv.Close()
	pol := writeTempFile(t, "pol.json", `{"schemaVersion":1,"rules":{"allowLanExposure":false}}`)
	outPath := filepath.Join(t.TempDir(), "report.json")

	out, _, code := runPolicyCheckRuntime(t, true, srv.URL, pol, "--out", outPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	fileData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading report file: %v", err)
	}
	if out != string(fileData) {
		t.Errorf("stdout and file differ\nstdout:\n%q\nfile:\n%q", out, string(fileData))
	}
	// The file records the runtime source too.
	if !strings.Contains(string(fileData), `"source": "runtime"`) {
		t.Errorf("exported runtime report should record the source:\n%s", string(fileData))
	}
}

func TestPolicyCheckRuntime_ExplainAddsExplanations(t *testing.T) {
	srv := makeExportServer(t, exposedRuntimeRules)
	defer srv.Close()
	pol := writeTempFile(t, "pol.json", `{"schemaVersion":1,"rules":{"allowLanExposure":false}}`)

	// Human inline explanation.
	humanOut, _, _ := runPolicyCheckRuntime(t, false, srv.URL, pol, "--explain")
	if !strings.Contains(humanOut, "Code: policy.lan_exposure_forbidden") {
		t.Errorf("human --explain missing inline explanation:\n%s", humanOut)
	}
	// JSON additive explanations map.
	jsonOut, _, _ := runPolicyCheckRuntime(t, true, srv.URL, pol, "--explain")
	var raw map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &raw); err != nil {
		t.Fatalf("decoding JSON: %v", err)
	}
	if _, ok := raw["explanations"].(map[string]any); !ok {
		t.Errorf("--explain should add an explanations map:\n%s", jsonOut)
	}
}

// --- runtime failures ---

func TestPolicyCheckRuntime_UnreachableExit3(t *testing.T) {
	srv := makeExportServer(t, compliantRuntimeRules)
	url := srv.URL
	srv.Close() // now refuses connections
	pol := writeTempFile(t, "pol.json", strictPolicy)

	_, errBuf, code := runPolicyCheckRuntime(t, false, url, pol)
	if code != 3 {
		t.Errorf("exit code = %d, want 3 (runtime unreachable)", code)
	}
	if !strings.Contains(errBuf, "Error:") {
		t.Errorf("stderr should report the connection error: %q", errBuf)
	}
}

func TestPolicyCheckRuntime_ExportFailureExit1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errors":["boom"]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()
	pol := writeTempFile(t, "pol.json", strictPolicy)

	_, _, code := runPolicyCheckRuntime(t, false, srv.URL, pol)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (config-export/API failure)", code)
	}
}

// --- source selection / input errors ---

func TestPolicyCheck_ConfigAndRuntimeMutuallyExclusiveExit2(t *testing.T) {
	cfg := writeTempFile(t, "cfg.json", compliantConfig)
	pol := writeTempFile(t, "pol.json", strictPolicy)
	_, errBuf, code := runPolicyCheckArgs(t, false, "--config", cfg, "--runtime", "--policy", pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf, "mutually exclusive") {
		t.Errorf("stderr should explain the mutual exclusion: %q", errBuf)
	}
}

func TestPolicyCheck_NeitherSourceExit2(t *testing.T) {
	pol := writeTempFile(t, "pol.json", strictPolicy)
	_, errBuf, code := runPolicyCheckArgs(t, false, "--policy", pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf, "one of --config or --runtime is required") {
		t.Errorf("stderr should require a config source: %q", errBuf)
	}
}

func TestPolicyCheckRuntime_MissingPolicyExit2(t *testing.T) {
	srv := makeExportServer(t, compliantRuntimeRules)
	defer srv.Close()
	var out, errBuf strings.Builder
	code := commands.RunPolicyCheck(false, commands.ConnFlags{URL: srv.URL}, []string{"--runtime"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestPolicyCheckRuntime_InvalidPolicyCheckedBeforeFetchExit2(t *testing.T) {
	// Point at a closed server; the policy is validated BEFORE any runtime I/O,
	// so an unsupported schema must exit 2 (input error), not 3 (unreachable).
	srv := makeExportServer(t, compliantRuntimeRules)
	url := srv.URL
	srv.Close()
	pol := writeTempFile(t, "pol.json", `{"schemaVersion":99}`)
	_, _, code := runPolicyCheckRuntime(t, false, url, pol)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (invalid policy precedes runtime fetch)", code)
	}
}

func TestPolicyCheckRuntime_InvalidURLExit2(t *testing.T) {
	pol := writeTempFile(t, "pol.json", strictPolicy)
	// Runtime mode with a malformed --url → ResolveURL rejects it → exit 2.
	var out, errBuf strings.Builder
	code := commands.RunPolicyCheck(false, commands.ConnFlags{URL: "ftp://bad"},
		[]string{"--runtime", "--policy", pol}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (invalid runtime URL)", code)
	}
}

// --- offline mode stays offline ---

func TestPolicyCheck_OfflineModeDoesNotContactOrResolveRuntime(t *testing.T) {
	// A server that fails the test if it is ever contacted.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("offline --config mode must not contact the runtime (got %s %s)", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	cfg := writeTempFile(t, "cfg.json", compliantConfig)
	pol := writeTempFile(t, "pol.json", strictPolicy)

	// A deliberately invalid --url; offline mode must NOT resolve it (ResolveURL
	// would reject the scheme), so the run still succeeds with exit 0.
	var out, errBuf strings.Builder
	code := commands.RunPolicyCheck(false, commands.ConnFlags{URL: "ftp://invalid"},
		[]string{"--config", cfg, "--policy", pol}, &out, &errBuf)
	if code != 0 {
		t.Errorf("offline mode should not resolve/use the runtime URL (exit %d, want 0)\n%s", code, errBuf.String())
	}
}
