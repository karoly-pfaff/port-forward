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

func makeStopServer(t *testing.T, rules []client.ForwardRuleResponse, wantPath string) *httptest.Server {
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

func TestRunStop_ByIDHumanOutput(t *testing.T) {
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "Dev API", Protocol: "tcp",
		ListenHost: "127.0.0.1", ListenPort: 48000,
		TargetHost: "192.168.1.10", TargetPort: 8080, Enabled: true,
	}
	srv := makeStopServer(t, []client.ForwardRuleResponse{rule}, "/api/forwards/rule-1/stop")
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStop(c, false, []string{"rule-1"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	output := out.String()
	if !strings.Contains(output, "Stopped") {
		t.Errorf("output does not contain 'Stopped': %s", output)
	}
	if !strings.Contains(output, "Dev API") {
		t.Errorf("output does not contain rule name: %s", output)
	}
}

func TestRunStop_JSONOutput(t *testing.T) {
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "Dev API", Protocol: "tcp",
		ListenHost: "127.0.0.1", ListenPort: 48000,
		TargetHost: "192.168.1.10", TargetPort: 8080, Enabled: true,
	}
	srv := makeStopServer(t, []client.ForwardRuleResponse{rule}, "/api/forwards/rule-1/stop")
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStop(c, true, []string{"rule-1"}, &out, &errBuf)
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
	if decoded["action"] != "stop" {
		t.Errorf("action = %v, want stop", decoded["action"])
	}
	if decoded["ruleId"] != "rule-1" {
		t.Errorf("ruleId = %v, want rule-1", decoded["ruleId"])
	}
}

func TestRunStop_MissingArg(t *testing.T) {
	c := client.New("http://127.0.0.1:0")
	var out, errBuf strings.Builder
	code := commands.RunStop(c, false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunStop_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunStop(c, false, []string{"rule-1"}, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

func TestRunStop_APIError(t *testing.T) {
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "Dev API", Protocol: "tcp",
		ListenHost: "127.0.0.1", ListenPort: 48000,
		TargetHost: "192.168.1.10", TargetPort: 8080, Enabled: true,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/forwards":
			json.NewEncoder(w).Encode([]client.ForwardRuleResponse{rule})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/stop"):
			// Non-connection API error (rule already stopped)
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string][]string{"errors": {"rule is already stopped"}})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStop(c, false, []string{"rule-1"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (API error)", code)
	}
}
