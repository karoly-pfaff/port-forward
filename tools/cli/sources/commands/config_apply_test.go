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

// --- apply fixtures ---

func applyNoDriftFixture() map[string]any {
	return map[string]any{
		"ok": true, "dryRun": false,
		"appliedAt": "2026-01-01T00:00:00.000Z",
		"plan":      planNoDriftFixture(),
		"applied":   map[string]any{"add": 0, "update": 0, "remove": 0, "unchanged": 1},
	}
}

func applyWithDriftFixture() map[string]any {
	return map[string]any{
		"ok": true, "dryRun": false,
		"appliedAt": "2026-01-01T00:00:00.000Z",
		"plan":      planWithDriftFixture(),
		"applied":   map[string]any{"add": 1, "update": 1, "remove": 1, "unchanged": 1},
	}
}

func applyDryRunFixture() map[string]any {
	return map[string]any{
		"ok": true, "dryRun": true,
		"appliedAt": "2026-01-01T00:00:00.000Z",
		"plan":      planWithDriftFixture(),
		"applied":   map[string]any{"add": 1, "update": 1, "remove": 1, "unchanged": 1},
	}
}

func applyDryRunNoDriftFixture() map[string]any {
	return map[string]any{
		"ok": true, "dryRun": true,
		"appliedAt": "2026-01-01T00:00:00.000Z",
		"plan":      planNoDriftFixture(),
		"applied":   map[string]any{"add": 0, "update": 0, "remove": 0, "unchanged": 1},
	}
}

func applyErrorsFixture() map[string]any {
	return map[string]any{
		"ok": false, "dryRun": false,
		"appliedAt": "2026-01-01T00:00:00.000Z",
		"plan":      planWithErrorsFixture(),
		"applied":   map[string]any{"add": 0, "update": 0, "remove": 0, "unchanged": 0},
	}
}

// makeApplyServer creates a test server that handles POST /api/config/apply
// and optionally GET /api/config/export (for backup tests).
func makeApplyServer(t *testing.T, applyBody any, exportBody any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/config/apply":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(applyBody)
		case r.Method == http.MethodGet && r.URL.Path == "/api/config/export":
			if exportBody != nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(exportBody)
			} else {
				t.Errorf("unexpected GET /api/config/export")
				http.NotFound(w, r)
			}
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

// --- RunConfigApply tests ---

func TestRunConfigApply_MissingArg_Exit2(t *testing.T) {
	srv := makeApplyServer(t, applyNoDriftFixture(), nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "Error") {
		t.Errorf("stderr should contain 'Error': %s", errBuf.String())
	}
}

func TestRunConfigApply_Help_Exit0(t *testing.T) {
	srv := makeApplyServer(t, applyNoDriftFixture(), nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "portier config apply") {
		t.Errorf("help output missing usage: %s", out.String())
	}
}

func TestRunConfigApply_InvalidFile_Exit2(t *testing.T) {
	srv := makeApplyServer(t, applyNoDriftFixture(), nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"/nonexistent/desired.json"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "Error reading") {
		t.Errorf("stderr should contain 'Error reading': %s", errBuf.String())
	}
}

func TestRunConfigApply_InvalidJSON_NoAPICall(t *testing.T) {
	apiCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(applyNoDriftFixture())
	}))
	defer srv.Close()

	file := writeTempConfig(t, "not valid json {{{{")
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{file}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if apiCalled {
		t.Error("API should not be called for invalid JSON file")
	}
}

func TestRunConfigApply_LocalValidationFail_NoAPICall(t *testing.T) {
	apiCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(applyNoDriftFixture())
	}))
	defer srv.Close()

	file := writeTempConfig(t, `[{"name":"","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false}]`)
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{file}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if apiCalled {
		t.Error("API should not be called when local validation fails")
	}
}

func TestRunConfigApply_NoDrift_HumanOutput(t *testing.T) {
	srv := makeApplyServer(t, applyNoDriftFixture(), nil)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--yes", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "nothing to apply") {
		t.Errorf("output missing 'nothing to apply': %s", out.String())
	}
}

func TestRunConfigApply_WithDrift_AppliedOutput(t *testing.T) {
	srv := makeApplyServer(t, applyWithDriftFixture(), nil)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--yes", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "Config applied") {
		t.Errorf("output missing 'Config applied': %s", o)
	}
	if !strings.Contains(o, "+1") {
		t.Errorf("output missing '+1' (add count): %s", o)
	}
	if !strings.Contains(o, "-1") {
		t.Errorf("output missing '-1' (remove count): %s", o)
	}
}

func TestRunConfigApply_DryRun_HumanOutput(t *testing.T) {
	srv := makeApplyServer(t, applyDryRunFixture(), nil)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--dry-run", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "Dry run") {
		t.Errorf("output missing 'Dry run': %s", o)
	}
	if !strings.Contains(o, "no changes") {
		t.Errorf("output missing 'no changes': %s", o)
	}
}

func TestRunConfigApply_DryRun_NoDrift(t *testing.T) {
	srv := makeApplyServer(t, applyDryRunNoDriftFixture(), nil)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--dry-run", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "No drift") {
		t.Errorf("output missing 'No drift': %s", out.String())
	}
}

func TestRunConfigApply_PlanErrors_Exit1(t *testing.T) {
	srv := makeApplyServer(t, applyErrorsFixture(), nil)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--yes", file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (plan errors); stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "errors") {
		t.Errorf("stderr should mention errors: %s", errBuf.String())
	}
}

func TestRunConfigApply_PlanErrors_ShowsErrorCodes(t *testing.T) {
	srv := makeApplyServer(t, applyErrorsFixture(), nil)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	commands.RunConfigApply(c, false, []string{"--yes", file}, &out, &errBuf)
	// Error details go to stderr
	if !strings.Contains(errBuf.String(), "INVALID_DESIRED_RULE") {
		t.Errorf("stderr should contain error code 'INVALID_DESIRED_RULE': %s", errBuf.String())
	}
}

func TestRunConfigApply_APIError_Exit1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"internal error"}})
	}))
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--yes", file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (API error)", code)
	}
}

func TestRunConfigApply_DestructiveWithoutYes_APIError_Exit1(t *testing.T) {
	// Simulate the server returning 400 when yes is missing for destructive ops.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"Apply requires yes: true when destructive operations are present."}})
	}))
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (400 from API)", code)
	}
}

func TestRunConfigApply_ConnectionError_Exit3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--yes", file}, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3 (connection failure)", code)
	}
}

func TestRunConfigApply_JSON_Output(t *testing.T) {
	srv := makeApplyServer(t, applyWithDriftFixture(), nil)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, true, []string{"--yes", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["ok"] != true {
		t.Errorf("ok = %v, want true", result["ok"])
	}
	if _, ok := result["plan"]; !ok {
		t.Error("JSON output missing 'plan' field")
	}
	if _, ok := result["applied"]; !ok {
		t.Error("JSON output missing 'applied' field")
	}
}

func TestRunConfigApply_JSON_PlanErrors_Exit1(t *testing.T) {
	srv := makeApplyServer(t, applyErrorsFixture(), nil)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, true, []string{"--yes", file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (ok:false in JSON mode)", code)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if result["ok"] != false {
		t.Errorf("ok = %v, want false", result["ok"])
	}
}

func TestRunConfigApply_BackupOut_WritesFile(t *testing.T) {
	srv := makeApplyServer(t, applyNoDriftFixture(), exportFixture())
	defer srv.Close()

	backupPath := filepath.Join(t.TempDir(), "backup.json")
	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--yes", "--backup-out", backupPath, file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("backup file not created at %s: %v", backupPath, err)
	}
	if !strings.Contains(out.String(), "Backup written") {
		t.Errorf("output missing 'Backup written': %s", out.String())
	}
}

func TestRunConfigApply_BackupOut_InvalidPath_PreventsApply(t *testing.T) {
	applyCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/config/export":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(exportFixture())
		case r.Method == http.MethodPost && r.URL.Path == "/api/config/apply":
			applyCalled = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(applyNoDriftFixture())
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	invalidBackupPath := filepath.Join(t.TempDir(), "nonexistent-dir", "backup.json")
	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--yes", "--backup-out", invalidBackupPath, file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (backup failure)", code)
	}
	if applyCalled {
		t.Error("apply API should not be called when backup fails")
	}
	if !strings.Contains(errBuf.String(), "backup") {
		t.Errorf("stderr should mention backup failure: %s", errBuf.String())
	}
}

func TestRunConfigApply_DryRun_BackupSkipped(t *testing.T) {
	backupRequestMade := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/config/export":
			backupRequestMade = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(exportFixture())
		case r.Method == http.MethodPost && r.URL.Path == "/api/config/apply":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(applyDryRunNoDriftFixture())
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	backupPath := filepath.Join(t.TempDir(), "backup.json")
	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--dry-run", "--backup-out", backupPath, file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if backupRequestMade {
		t.Error("backup export should not be called in --dry-run mode")
	}
	if _, err := os.Stat(backupPath); err == nil {
		t.Error("backup file should not be written in --dry-run mode")
	}
}

func TestRunConfigApply_YesFlagSentInRequest(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/config/apply" {
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				t.Errorf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(applyNoDriftFixture())
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	commands.RunConfigApply(c, false, []string{"--yes", file}, &out, &errBuf)

	if capturedBody["yes"] != true {
		t.Errorf("yes = %v, want true in request body", capturedBody["yes"])
	}
	if capturedBody["dryRun"] == true {
		t.Error("dryRun should not be true when --dry-run is not set")
	}
}

func TestRunConfigApply_DryRunFlagSentInRequest(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/config/apply" {
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				t.Errorf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(applyDryRunNoDriftFixture())
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	commands.RunConfigApply(c, false, []string{"--dry-run", file}, &out, &errBuf)

	if capturedBody["dryRun"] != true {
		t.Errorf("dryRun = %v, want true in request body", capturedBody["dryRun"])
	}
}

func TestRunConfig_ApplyDispatch(t *testing.T) {
	srv := makeApplyServer(t, applyNoDriftFixture(), nil)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfig(c, false, []string{"apply", "--yes", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
}

func TestRunConfig_HelpIncludesApply(t *testing.T) {
	srv := makeApplyServer(t, applyNoDriftFixture(), nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfig(c, false, []string{"help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "apply") {
		t.Errorf("config help missing 'apply': %s", out.String())
	}
}

func TestRunConfigApply_UnknownFlag_Exit2(t *testing.T) {
	srv := makeApplyServer(t, applyNoDriftFixture(), nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--unknown-flag"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunConfigApply_WithWarnings_WarningsShown(t *testing.T) {
	applyWithWarnings := map[string]any{
		"ok": true, "dryRun": false,
		"appliedAt": "2026-01-01T00:00:00.000Z",
		"plan":      planWithWarningsFixture(),
		"applied":   map[string]any{"add": 1, "update": 0, "remove": 0, "unchanged": 0},
	}
	srv := makeApplyServer(t, applyWithWarnings, nil)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigApply(c, false, []string{"--yes", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "LAN_EXPOSURE") {
		t.Errorf("output missing 'LAN_EXPOSURE' warning: %s", out.String())
	}
}
