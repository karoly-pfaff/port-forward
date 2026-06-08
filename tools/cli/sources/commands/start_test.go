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

// makeStartStopServer creates a test server that serves GET /api/forwards and
// responds 200 OK to POST /api/forwards/:id/start or /stop.
func makeStartServer(t *testing.T, rules []client.ForwardRuleResponse, wantPath string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/forwards":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rules)
		case r.Method == http.MethodPost && r.URL.Path == wantPath:
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func TestRunStart_ByIDHumanOutput(t *testing.T) {
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "Dev API", Protocol: "tcp",
		ListenHost: "127.0.0.1", ListenPort: 48000,
		TargetHost: "192.168.1.10", TargetPort: 8080, Enabled: true,
	}
	srv := makeStartServer(t, []client.ForwardRuleResponse{rule}, "/api/forwards/rule-1/start")
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStart(c, false, []string{"rule-1"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	output := out.String()
	if !strings.Contains(output, "Started") {
		t.Errorf("output does not contain 'Started': %s", output)
	}
	if !strings.Contains(output, "Dev API") {
		t.Errorf("output does not contain rule name: %s", output)
	}
	if !strings.Contains(output, "127.0.0.1:48000") {
		t.Errorf("output does not contain listen address: %s", output)
	}
}

func TestRunStart_ByNameHumanOutput(t *testing.T) {
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "Dev API", Protocol: "tcp",
		ListenHost: "127.0.0.1", ListenPort: 48000,
		TargetHost: "192.168.1.10", TargetPort: 8080, Enabled: true,
	}
	srv := makeStartServer(t, []client.ForwardRuleResponse{rule}, "/api/forwards/rule-1/start")
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStart(c, false, []string{"Dev API"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Dev API") {
		t.Errorf("output does not contain rule name: %s", out.String())
	}
}

func TestRunStart_JSONOutput(t *testing.T) {
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "Dev API", Protocol: "tcp",
		ListenHost: "127.0.0.1", ListenPort: 48000,
		TargetHost: "192.168.1.10", TargetPort: 8080, Enabled: true,
	}
	srv := makeStartServer(t, []client.ForwardRuleResponse{rule}, "/api/forwards/rule-1/start")
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStart(c, true, []string{"rule-1"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\noutput:\n%s", err, out.String())
	}
	if decoded["ok"] != true {
		t.Errorf("ok = %v, want true", decoded["ok"])
	}
	if decoded["action"] != "start" {
		t.Errorf("action = %v, want start", decoded["action"])
	}
	if decoded["ruleId"] != "rule-1" {
		t.Errorf("ruleId = %v, want rule-1", decoded["ruleId"])
	}
}

func TestRunStart_AmbiguityFails(t *testing.T) {
	rules := []client.ForwardRuleResponse{
		{ID: "id-a", Name: "Dev API", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000},
		{ID: "id-b", Name: "Dev API", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48001},
	}
	srv := makeForwardsServer(t, rules)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStart(c, false, []string{"Dev API"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "multiple rules") {
		t.Errorf("stderr missing 'multiple rules': %s", errBuf.String())
	}
}

func TestRunStart_MissingArg(t *testing.T) {
	srv := makeForwardsServer(t, []client.ForwardRuleResponse{})
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStart(c, false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunStart_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunStart(c, false, []string{"rule-1"}, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}
