package commands_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"portier/cli/sources/client"
	"portier/cli/sources/commands"
)

func makeRuntimeServer(t *testing.T, info client.RuntimeInfo) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
}

func TestRunRuntime_HumanOutput(t *testing.T) {
	info := client.RuntimeInfo{
		Name:           "Portier",
		Version:        "1.2.0",
		Runtime:        "go",
		Platform:       "windows",
		Arch:           "x64",
		UptimeSeconds:  100,
		StartedAt:      "2026-01-01T00:00:00Z",
		ManagementHost: "127.0.0.1",
		ManagementPort: 47831,
		ConfigPath:     "C:\\ProgramData\\Portier\\rules.json",
		StaticDir:      "C:\\Program Files\\Portier\\web",
		ServiceMode:    false,
	}
	srv := makeRuntimeServer(t, info)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunRuntime(c, false, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	output := out.String()
	for _, want := range []string{"Portier", "1.2.0", "go", "47831"} {
		if !strings.Contains(output, want) {
			t.Errorf("human output does not contain %q\nfull output:\n%s", want, output)
		}
	}
}

func TestRunRuntime_JSONOutput(t *testing.T) {
	info := client.RuntimeInfo{
		Name:           "Portier",
		Version:        "1.2.0",
		Runtime:        "node",
		ManagementHost: "127.0.0.1",
		ManagementPort: 47831,
	}
	srv := makeRuntimeServer(t, info)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunRuntime(c, true, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}

	var decoded client.RuntimeInfo
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\noutput:\n%s", err, out.String())
	}
	if decoded.Name != "Portier" {
		t.Errorf("decoded.Name = %q, want %q", decoded.Name, "Portier")
	}
	if decoded.ManagementPort != 47831 {
		t.Errorf("decoded.ManagementPort = %d, want %d", decoded.ManagementPort, 47831)
	}
}

func TestRunRuntime_ConnectionFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunRuntime(c, false, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "could not connect") {
		t.Errorf("stderr does not mention connection failure: %s", errOutput)
	}
	if !strings.Contains(errOutput, "Portier service running") {
		t.Errorf("stderr does not contain hint about service running: %s", errOutput)
	}
}

func TestRunRuntime_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"internal error"}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunRuntime(c, false, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Error") {
		t.Errorf("stderr should contain 'Error': %s", errBuf.String())
	}
}

func TestRunRuntime_JSONPreservesFieldNames(t *testing.T) {
	info := client.RuntimeInfo{
		Name:           "Portier",
		UptimeSeconds:  42,
		ManagementHost: "127.0.0.1",
		ManagementPort: 47831,
		ServiceMode:    true,
	}
	srv := makeRuntimeServer(t, info)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunRuntime(c, true, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	raw := out.String()
	for _, fieldName := range []string{"uptimeSeconds", "managementHost", "managementPort", "serviceMode"} {
		if !strings.Contains(raw, fieldName) {
			t.Errorf("JSON output missing field %q\nfull output:\n%s", fieldName, raw)
		}
	}
}
