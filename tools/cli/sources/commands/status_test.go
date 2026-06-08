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

// makeStatusServer serves /api/status and /api/forwards from the same server.
func makeStatusServer(
	t *testing.T,
	statuses []client.ForwardStatus,
	rules []client.ForwardRuleResponse,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			json.NewEncoder(w).Encode(statuses)
		case "/api/forwards":
			json.NewEncoder(w).Encode(rules)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestRunStatus_HumanOutput(t *testing.T) {
	conns := 2
	statuses := []client.ForwardStatus{
		{RuleID: "r1", Running: true, BytesIn: 2048, BytesOut: 512, ActiveConnections: &conns},
	}
	rules := []client.ForwardRuleResponse{
		{ID: "r1", Name: "Dev API", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000,
			TargetHost: "192.168.1.10", TargetPort: 8080, Enabled: true},
	}
	srv := makeStatusServer(t, statuses, rules)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStatus(c, false, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	output := out.String()
	for _, want := range []string{"Dev API", "tcp", "running", "2", "2.0 KB"} {
		if !strings.Contains(output, want) {
			t.Errorf("human output does not contain %q\nfull output:\n%s", want, output)
		}
	}
}

func TestRunStatus_JSONOutput(t *testing.T) {
	statuses := []client.ForwardStatus{
		{RuleID: "r1", Running: false, BytesIn: 0, BytesOut: 0},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statuses)
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStatus(c, true, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	var decoded []client.ForwardStatus
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("--json output is not a valid JSON array: %v\noutput:\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0].RuleID != "r1" {
		t.Errorf("unexpected decoded statuses: %+v", decoded)
	}
}

func TestRunStatus_EmptyState(t *testing.T) {
	srv := makeStatusServer(t, []client.ForwardStatus{}, []client.ForwardRuleResponse{})
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStatus(c, false, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "No rule status") {
		t.Errorf("empty output missing friendly message: %q", out.String())
	}
}

func TestRunStatus_FallsBackToRuleIDWhenForwardsFails(t *testing.T) {
	statuses := []client.ForwardStatus{
		{RuleID: "r-unknown", Running: false, BytesIn: 0, BytesOut: 0},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			json.NewEncoder(w).Encode(statuses)
		case "/api/forwards":
			// Simulate failure — human output should fall back to ruleId
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string][]string{"errors": {"unavailable"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStatus(c, false, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "r-unknown") {
		t.Errorf("output should contain ruleId fallback: %s", out.String())
	}
}

func TestRunStatus_ActiveUDPSessions(t *testing.T) {
	udpSessions := 3
	statuses := []client.ForwardStatus{
		{RuleID: "r1", Running: true, BytesIn: 0, BytesOut: 0, ActiveUDPSessions: &udpSessions},
	}
	rules := []client.ForwardRuleResponse{
		{ID: "r1", Name: "UDP Rule", Protocol: "udp", ListenHost: "127.0.0.1", ListenPort: 48001,
			TargetHost: "192.168.1.10", TargetPort: 9090, Enabled: true},
	}
	srv := makeStatusServer(t, statuses, rules)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunStatus(c, false, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "3") {
		t.Errorf("output should show UDP session count (3): %s", out.String())
	}
}

func TestRunStatus_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunStatus(c, false, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
	if !strings.Contains(errBuf.String(), "could not connect") {
		t.Errorf("stderr missing connection message: %s", errBuf.String())
	}
}
