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

func makeListServer(t *testing.T, rules []client.ForwardRuleResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)
	}))
}

func TestRunList_HumanOutput(t *testing.T) {
	rules := []client.ForwardRuleResponse{
		{
			ID: "r1", Name: "Dev API", Protocol: "tcp",
			ListenHost: "127.0.0.1", ListenPort: 48000,
			TargetHost: "192.168.1.10", TargetPort: 8080,
			Enabled: true,
		},
		{
			ID: "r2", Name: "Game Server", Protocol: "udp", UDPMode: "bidirectional-last-client",
			ListenHost: "0.0.0.0", ListenPort: 48001,
			TargetHost: "192.168.1.20", TargetPort: 7777,
			Enabled: false,
		},
	}
	srv := makeListServer(t, rules)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunList(c, false, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	output := out.String()
	for _, want := range []string{"Dev API", "tcp", "127.0.0.1:48000", "Game Server", "udp/lc", "0.0.0.0:48001"} {
		if !strings.Contains(output, want) {
			t.Errorf("human output does not contain %q\nfull output:\n%s", want, output)
		}
	}
}

func TestRunList_JSONOutput(t *testing.T) {
	rules := []client.ForwardRuleResponse{
		{ID: "r1", Name: "Dev API", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000,
			TargetHost: "192.168.1.10", TargetPort: 8080, Enabled: true},
	}
	srv := makeListServer(t, rules)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunList(c, true, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	var decoded []client.ForwardRuleResponse
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("--json output is not a valid JSON array: %v\noutput:\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0].Name != "Dev API" {
		t.Errorf("unexpected decoded rules: %+v", decoded)
	}
}

func TestRunList_EmptyState(t *testing.T) {
	srv := makeListServer(t, []client.ForwardRuleResponse{})
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunList(c, false, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "No forwarding rules") {
		t.Errorf("empty output missing friendly message: %q", out.String())
	}
}

func TestRunList_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunList(c, false, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
	if !strings.Contains(errBuf.String(), "could not connect") {
		t.Errorf("stderr missing connection message: %s", errBuf.String())
	}
}

func TestRunList_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"internal error"}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunList(c, false, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunList_UDPOneWay(t *testing.T) {
	rules := []client.ForwardRuleResponse{
		{ID: "r1", Name: "Stream", Protocol: "udp", UDPMode: "one-way",
			ListenHost: "127.0.0.1", ListenPort: 48002,
			TargetHost: "10.0.0.1", TargetPort: 9001, Enabled: true},
	}
	srv := makeListServer(t, rules)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunList(c, false, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "udp/1way") {
		t.Errorf("output should contain 'udp/1way': %s", out.String())
	}
}

func TestRunList_UDPMultiClient(t *testing.T) {
	rules := []client.ForwardRuleResponse{
		{ID: "r1", Name: "Game", Protocol: "udp", UDPMode: "bidirectional-multi-client",
			ListenHost: "127.0.0.1", ListenPort: 48003,
			TargetHost: "10.0.0.1", TargetPort: 9002, Enabled: true},
	}
	srv := makeListServer(t, rules)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunList(c, false, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "udp/mc") {
		t.Errorf("output should contain 'udp/mc': %s", out.String())
	}
}

func TestRunList_UDPDefaultMode(t *testing.T) {
	// No UDPMode set → formatProto default branch → "udp"
	rules := []client.ForwardRuleResponse{
		{ID: "r1", Name: "UDP", Protocol: "udp",
			ListenHost: "127.0.0.1", ListenPort: 48004,
			TargetHost: "10.0.0.1", TargetPort: 9003, Enabled: true},
	}
	srv := makeListServer(t, rules)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunList(c, false, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if strings.Contains(out.String(), "udp/") {
		t.Errorf("output should not contain a UDP mode suffix when UDPMode is unset: %s", out.String())
	}
}
