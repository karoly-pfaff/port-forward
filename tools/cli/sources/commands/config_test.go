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

// makeConfigServer creates a test server that handles GET /api/config/export
// and POST /api/config/import with the provided handlers.
func makeConfigServer(t *testing.T, exportBody any, importBody any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/config/export":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(exportBody)
		case r.Method == http.MethodPost && r.URL.Path == "/api/config/import":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(importBody)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func exportFixture() map[string]any {
	return map[string]any{
		"version":    "1",
		"exportedAt": "2026-01-01T12:00:00Z",
		"rules": []map[string]any{
			{"id": "r1", "name": "API", "protocol": "tcp", "listenHost": "127.0.0.1",
				"listenPort": 48000, "targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true},
			{"id": "r2", "name": "DB", "protocol": "tcp", "listenHost": "127.0.0.1",
				"listenPort": 48001, "targetHost": "10.0.0.1", "targetPort": 5432, "enabled": false},
		},
	}
}

func importSuccessFixture() map[string]any {
	return map[string]any{
		"result": map[string]any{"imported": 2, "skipped": 0, "errors": []any{}},
		"rules":  []any{},
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.json")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

// --- RunConfig dispatch ---

func TestRunConfig_NoSubcommand_Exit2(t *testing.T) {
	srv := makeConfigServer(t, exportFixture(), importSuccessFixture())
	defer srv.Close()
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfig(c, false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunConfig_UnknownSubcommand_Exit2(t *testing.T) {
	srv := makeConfigServer(t, exportFixture(), importSuccessFixture())
	defer srv.Close()
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfig(c, false, []string{"sync"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "sync") {
		t.Errorf("stderr missing unknown subcommand name: %s", errBuf.String())
	}
}

func TestRunConfig_HelpSubcommand(t *testing.T) {
	srv := makeConfigServer(t, exportFixture(), importSuccessFixture())
	defer srv.Close()
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfig(c, false, []string{"help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "validate") {
		t.Errorf("help output missing 'validate': %s", out.String())
	}
}

// --- RunConfigValidate: local validation only ---

func TestRunConfigValidate_ValidRawArray_Human(t *testing.T) {
	content := `[{"name":"API","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true}]`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stdout: %s stderr: %s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), "Config is valid") {
		t.Errorf("output missing 'Config is valid': %s", out.String())
	}
	if !strings.Contains(out.String(), "1 rule") {
		t.Errorf("output missing rule count: %s", out.String())
	}
}

func TestRunConfigValidate_ValidRawArray_JSON(t *testing.T) {
	content := `[
		{"name":"API","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true},
		{"name":"UDP","protocol":"udp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"10.0.0.1","targetPort":9090,"enabled":false,"udpMode":"one-way"}
	]`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(true, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["valid"] != true {
		t.Errorf("valid = %v, want true", result["valid"])
	}
	if result["ruleCount"].(float64) != 2 {
		t.Errorf("ruleCount = %v, want 2", result["ruleCount"])
	}
	if result["tcpCount"].(float64) != 1 {
		t.Errorf("tcpCount = %v, want 1", result["tcpCount"])
	}
	if result["udpCount"].(float64) != 1 {
		t.Errorf("udpCount = %v, want 1", result["udpCount"])
	}
	errs, ok := result["errors"].([]any)
	if !ok || len(errs) != 0 {
		t.Errorf("errors = %v, want []", result["errors"])
	}
}

func TestRunConfigValidate_ValidWrapperShape(t *testing.T) {
	content := `{"rules":[{"name":"Dev","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true}]}`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stdout: %s stderr: %s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), "Config is valid") {
		t.Errorf("output missing 'Config is valid': %s", out.String())
	}
}

func TestRunConfigValidate_ValidExportShape(t *testing.T) {
	content := `{"version":"1","exportedAt":"2026-01-01T00:00:00Z","rules":[{"name":"Dev","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true}]}`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stdout: %s stderr: %s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), "Config is valid") {
		t.Errorf("output missing 'Config is valid': %s", out.String())
	}
}

func TestRunConfigValidate_EmptyArray(t *testing.T) {
	file := writeTempConfig(t, "[]")
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stdout: %s stderr: %s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), "Config is valid") {
		t.Errorf("output missing 'Config is valid': %s", out.String())
	}
}

func TestRunConfigValidate_InvalidJSON(t *testing.T) {
	file := writeTempConfig(t, "this is not valid JSON {{{")
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "Config is invalid") {
		t.Errorf("output missing 'Config is invalid': %s", out.String())
	}
}

func TestRunConfigValidate_InvalidProtocol(t *testing.T) {
	content := `[{"name":"Bad","protocol":"quic","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false}]`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "invalid protocol") {
		t.Errorf("output missing 'invalid protocol': %s", out.String())
	}
}

func TestRunConfigValidate_InvalidPort(t *testing.T) {
	content := `[{"name":"Bad","protocol":"tcp","listenHost":"127.0.0.1","listenPort":0,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false}]`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "listenPort") {
		t.Errorf("output missing 'listenPort': %s", out.String())
	}
}

func TestRunConfigValidate_InvalidUDPMode(t *testing.T) {
	content := `[{"name":"Bad","protocol":"udp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false,"udpMode":"roundtrip"}]`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "invalid udpMode") {
		t.Errorf("output missing 'invalid udpMode': %s", out.String())
	}
}

func TestRunConfigValidate_DuplicateBinding(t *testing.T) {
	content := `[
		{"name":"First","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true},
		{"name":"Second","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":9090,"enabled":true}
	]`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "duplicate listen binding") {
		t.Errorf("output missing 'duplicate listen binding': %s", out.String())
	}
}

func TestRunConfigValidate_EmptyName(t *testing.T) {
	content := `[{"name":"","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false}]`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "name is required") {
		t.Errorf("output missing 'name is required': %s", out.String())
	}
}

func TestRunConfigValidate_EmptyHost(t *testing.T) {
	content := `[{"name":"Dev","protocol":"tcp","listenHost":"","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false}]`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "listenHost is required") {
		t.Errorf("output missing 'listenHost is required': %s", out.String())
	}
}

func TestRunConfigValidate_FileNotFound(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{"/nonexistent/path/config.json"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Error reading") {
		t.Errorf("stderr missing 'Error reading': %s", errBuf.String())
	}
}

func TestRunConfigValidate_MissingArg_Exit2(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunConfigValidate_InvalidJSON_JSONOutput(t *testing.T) {
	file := writeTempConfig(t, "not json")
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(true, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["valid"] != false {
		t.Errorf("valid = %v, want false", result["valid"])
	}
}

// --- RunConfigExport ---

func TestRunConfigExport_WritesFile_HumanOutput(t *testing.T) {
	srv := makeConfigServer(t, exportFixture(), nil)
	defer srv.Close()

	outFile := filepath.Join(t.TempDir(), "export.json")
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigExport(c, false, []string{"--out", outFile}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Exported") {
		t.Errorf("output missing 'Exported': %s", out.String())
	}
	if !strings.Contains(out.String(), outFile) {
		t.Errorf("output missing file path: %s", out.String())
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("output file not written: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output file is not valid JSON: %v", err)
	}
	if parsed["version"] != "1" {
		t.Errorf("version = %v, want 1", parsed["version"])
	}
}

func TestRunConfigExport_WritesFile_JSONOutput(t *testing.T) {
	srv := makeConfigServer(t, exportFixture(), nil)
	defer srv.Close()

	outFile := filepath.Join(t.TempDir(), "export.json")
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigExport(c, true, []string{"--out", outFile}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["ok"] != true {
		t.Errorf("ok = %v, want true", result["ok"])
	}
	if result["path"] != outFile {
		t.Errorf("path = %v, want %s", result["path"], outFile)
	}
	if result["ruleCount"].(float64) != 2 {
		t.Errorf("ruleCount = %v, want 2", result["ruleCount"])
	}
}

func TestRunConfigExport_StdoutMode_JSONFlag(t *testing.T) {
	srv := makeConfigServer(t, exportFixture(), nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigExport(c, true, []string{}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	// Output should be the raw config JSON, not a result object.
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out.String()), &cfg); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if cfg["version"] != "1" {
		t.Errorf("version = %v, want 1", cfg["version"])
	}
}

func TestRunConfigExport_HumanMode_RequiresOut(t *testing.T) {
	srv := makeConfigServer(t, exportFixture(), nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigExport(c, false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunConfigExport_APIError_NoFileWritten(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"server error"}})
	}))
	defer srv.Close()

	outFile := filepath.Join(t.TempDir(), "export.json")
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigExport(c, false, []string{"--out", outFile}, &out, &errBuf)
	if code == 0 {
		t.Error("expected non-zero exit code on API error")
	}

	if _, err := os.Stat(outFile); !os.IsNotExist(err) {
		t.Error("output file should not exist after API error")
	}
}

func TestRunConfigExport_ConnectionError_Exit3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	outFile := filepath.Join(t.TempDir(), "export.json")
	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunConfigExport(c, false, []string{"--out", outFile}, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

// --- RunConfigImport ---

func makeImportServer(t *testing.T, wantMode string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/config/import":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if wantMode != "" {
				if got, _ := body["mode"].(string); got != wantMode {
					t.Errorf("mode = %q, want %q", got, wantMode)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(importSuccessFixture())
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func validConfigJSON() string {
	return `[{"name":"API","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":true}]`
}

func TestRunConfigImport_MergeCallsAPI(t *testing.T) {
	srv := makeImportServer(t, "merge")
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--mode", "merge", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "merge") {
		t.Errorf("output missing 'merge': %s", out.String())
	}
}

func TestRunConfigImport_MergeHumanOutput(t *testing.T) {
	srv := makeImportServer(t, "merge")
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--mode", "merge", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Imported config") {
		t.Errorf("output missing 'Imported config': %s", out.String())
	}
}

func TestRunConfigImport_MergeJSONOutput(t *testing.T) {
	srv := makeImportServer(t, "merge")
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, true, []string{"--mode", "merge", file}, &out, &errBuf)
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
	if result["mode"] != "merge" {
		t.Errorf("mode = %v, want merge", result["mode"])
	}
}

func TestRunConfigImport_ReplaceRequiresYes(t *testing.T) {
	srv := makeImportServer(t, "replace")
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--mode", "replace", file}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "--yes") {
		t.Errorf("stderr missing '--yes': %s", errBuf.String())
	}
}

func TestRunConfigImport_ReplaceWithYesCallsAPI(t *testing.T) {
	srv := makeImportServer(t, "replace")
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--mode", "replace", "--yes", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "replace") {
		t.Errorf("output missing 'replace': %s", out.String())
	}
}

func TestRunConfigImport_InvalidLocalFile_NoAPICall(t *testing.T) {
	apiCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(importSuccessFixture())
	}))
	defer srv.Close()

	file := writeTempConfig(t, `[{"name":"Bad","protocol":"quic","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false}]`)
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--mode", "merge", file}, &out, &errBuf)
	if code == 0 {
		t.Error("expected non-zero exit code for invalid config")
	}
	if apiCalled {
		t.Error("API should not be called when local validation fails")
	}
}

func TestRunConfigImport_InvalidMode_Exit2(t *testing.T) {
	srv := makeImportServer(t, "")
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--mode", "overwrite", file}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunConfigImport_MissingMode_Exit2(t *testing.T) {
	srv := makeImportServer(t, "")
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{file}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "--mode") {
		t.Errorf("stderr missing '--mode': %s", errBuf.String())
	}
}

func TestRunConfigImport_MissingArg_Exit2(t *testing.T) {
	srv := makeImportServer(t, "")
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--mode", "merge"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunConfigImport_FileNotFound(t *testing.T) {
	srv := makeImportServer(t, "")
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--mode", "merge", "/nonexistent/config.json"}, &out, &errBuf)
	if code == 0 {
		t.Error("expected non-zero exit code for missing file")
	}
	if !strings.Contains(errBuf.String(), "Error reading") {
		t.Errorf("stderr missing 'Error reading': %s", errBuf.String())
	}
}

func TestRunConfigImport_ConnectionError_Exit3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--mode", "merge", file}, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

func TestRunConfigImport_MalformedJSONFile(t *testing.T) {
	srv := makeImportServer(t, "")
	defer srv.Close()

	file := writeTempConfig(t, "this is not json")
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--mode", "merge", file}, &out, &errBuf)
	if code == 0 {
		t.Error("expected non-zero exit code for malformed JSON")
	}
}

// --- RunConfig dispatch tests ---

func TestRunConfig_ExportDispatch(t *testing.T) {
	srv := makeConfigServer(t, exportFixture(), nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	// --json + no --out → export prints config JSON to stdout
	code := commands.RunConfig(c, true, []string{"export"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
}

func TestRunConfig_ValidateDispatch(t *testing.T) {
	file := writeTempConfig(t, validConfigJSON())
	srv := makeConfigServer(t, nil, nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfig(c, false, []string{"validate", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stdout: %s stderr: %s", code, out.String(), errBuf.String())
	}
}

func TestRunConfig_ImportDispatch(t *testing.T) {
	srv := makeImportServer(t, "merge")
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfig(c, false, []string{"import", "--mode", "merge", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
}

// --- --help flag tests for subcommands ---

func TestRunConfigExport_HelpFlag(t *testing.T) {
	srv := makeConfigServer(t, nil, nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigExport(c, false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier config export") {
		t.Errorf("help output missing usage: %s", out.String())
	}
}

func TestRunConfigImport_HelpFlag(t *testing.T) {
	srv := makeConfigServer(t, nil, nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier config import") {
		t.Errorf("help output missing usage: %s", out.String())
	}
}

func TestRunConfigValidate_HelpFlag(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier config validate") {
		t.Errorf("help output missing usage: %s", out.String())
	}
}

// --- parseLocalConfig edge cases (via RunConfigValidate) ---

func TestRunConfigValidate_EmptyFile(t *testing.T) {
	file := writeTempConfig(t, "")
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "invalid") && !strings.Contains(errBuf.String(), "empty") {
		t.Errorf("expected error about empty file; stdout: %s stderr: %s", out.String(), errBuf.String())
	}
}

func TestRunConfigValidate_InvalidJSONArray(t *testing.T) {
	// Starts with '[' but is invalid JSON
	file := writeTempConfig(t, "[bad json")
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunConfigValidate_InvalidJSONObject(t *testing.T) {
	// Starts with '{' but is invalid JSON
	file := writeTempConfig(t, "{bad json")
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunConfigValidate_ObjectMissingRules(t *testing.T) {
	// Valid JSON object but no "rules" field
	file := writeTempConfig(t, `{"version":"1","exportedAt":"2026-01-01T00:00:00Z"}`)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunConfigValidate_RulesNotArray(t *testing.T) {
	// "rules" is a string, not an array
	file := writeTempConfig(t, `{"rules":"not-an-array"}`)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunConfigValidate_NullRules(t *testing.T) {
	// "rules": null → treated as empty config (valid)
	file := writeTempConfig(t, `{"rules":null}`)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (null rules = empty config); stdout: %s stderr: %s",
			code, out.String(), errBuf.String())
	}
}

// --- flag parse error paths (non-help) ---

func TestRunConfigExport_UnknownFlag(t *testing.T) {
	srv := makeConfigServer(t, exportFixture(), nil)
	defer srv.Close()
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigExport(c, false, []string{"--unknown-flag"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "Error") {
		t.Errorf("stderr should contain 'Error': %s", errBuf.String())
	}
}

func TestRunConfigImport_UnknownFlag(t *testing.T) {
	srv := makeImportServer(t, "")
	defer srv.Close()
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigImport(c, false, []string{"--unknown-flag"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "Error") {
		t.Errorf("stderr should contain 'Error': %s", errBuf.String())
	}
}

func TestRunConfigValidate_UnknownFlag(t *testing.T) {
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{"--unknown-flag"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "Error") {
		t.Errorf("stderr should contain 'Error': %s", errBuf.String())
	}
}

// --- RunConfigExport write failure ---

func TestRunConfigExport_WriteFailure(t *testing.T) {
	srv := makeConfigServer(t, exportFixture(), nil)
	defer srv.Close()
	c := client.New(srv.URL)
	// Non-existent subdirectory → os.WriteFile fails.
	badPath := filepath.Join(t.TempDir(), "nonexistent", "export.json")
	var out, errBuf strings.Builder
	code := commands.RunConfigExport(c, false, []string{"--out", badPath}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "Error writing") {
		t.Errorf("stderr missing 'Error writing': %s", errBuf.String())
	}
}

// --- validateLocalConfig: empty targetHost / invalid targetPort ---

func TestRunConfigValidate_EmptyTargetHost(t *testing.T) {
	content := `[{"name":"Dev","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"","targetPort":8080,"enabled":false}]`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "targetHost is required") {
		t.Errorf("output missing 'targetHost is required': %s", out.String())
	}
}

func TestRunConfigValidate_InvalidTargetPort(t *testing.T) {
	content := `[{"name":"Dev","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":0,"enabled":false}]`
	file := writeTempConfig(t, content)
	var out, errBuf strings.Builder
	code := commands.RunConfigValidate(false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "targetPort") {
		t.Errorf("output missing 'targetPort': %s", out.String())
	}
}
