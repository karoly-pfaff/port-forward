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

func makeForwardsServer(t *testing.T, rules []client.ForwardRuleResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/forwards" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)
	}))
}

var ruleA = client.ForwardRuleResponse{
	ID: "id-a", Name: "Dev API", Protocol: "tcp",
	ListenHost: "127.0.0.1", ListenPort: 48000,
	TargetHost: "192.168.1.10", TargetPort: 8080,
	Enabled: true,
}

var ruleB = client.ForwardRuleResponse{
	ID: "id-b", Name: "Staging DB", Protocol: "tcp",
	ListenHost: "127.0.0.1", ListenPort: 48001,
	TargetHost: "192.168.1.20", TargetPort: 5432,
	Enabled: true,
}

var ruleDupName = client.ForwardRuleResponse{
	ID: "id-c", Name: "Dev API", Protocol: "tcp",
	ListenHost: "127.0.0.1", ListenPort: 48002,
	TargetHost: "192.168.1.30", TargetPort: 9000,
	Enabled: false,
}

func TestResolveRule_ExactIDMatch(t *testing.T) {
	srv := makeForwardsServer(t, []client.ForwardRuleResponse{ruleA, ruleB})
	defer srv.Close()

	c := client.New(srv.URL)
	var errBuf strings.Builder
	rule, code := commands.ResolveRule(c, "id-a", &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if rule == nil || rule.ID != "id-a" {
		t.Errorf("resolved ID = %v, want id-a", rule)
	}
}

func TestResolveRule_ExactNameMatch(t *testing.T) {
	srv := makeForwardsServer(t, []client.ForwardRuleResponse{ruleA, ruleB})
	defer srv.Close()

	c := client.New(srv.URL)
	var errBuf strings.Builder
	rule, code := commands.ResolveRule(c, "Staging DB", &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if rule == nil || rule.ID != "id-b" {
		t.Errorf("resolved ID = %v, want id-b", rule)
	}
}

func TestResolveRule_IDWinsOverSameTextAsName(t *testing.T) {
	// A rule whose ID equals another rule's name — ID lookup must win.
	ruleIDAsName := client.ForwardRuleResponse{
		ID: "Dev API", Name: "something-else", Protocol: "tcp",
		ListenHost: "127.0.0.1", ListenPort: 48003,
		TargetHost: "10.0.0.1", TargetPort: 80,
		Enabled: true,
	}
	srv := makeForwardsServer(t, []client.ForwardRuleResponse{ruleA, ruleIDAsName})
	defer srv.Close()

	c := client.New(srv.URL)
	var errBuf strings.Builder
	rule, code := commands.ResolveRule(c, "Dev API", &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	// ID match should return ruleIDAsName (whose ID is "Dev API"), not ruleA (whose Name is "Dev API")
	if rule == nil || rule.ID != "Dev API" {
		t.Errorf("resolved ID = %v, want 'Dev API'", rule)
	}
}

func TestResolveRule_DuplicateNameAmbiguity(t *testing.T) {
	srv := makeForwardsServer(t, []client.ForwardRuleResponse{ruleA, ruleDupName})
	defer srv.Close()

	c := client.New(srv.URL)
	var errBuf strings.Builder
	rule, code := commands.ResolveRule(c, "Dev API", &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if rule != nil {
		t.Errorf("expected nil rule on ambiguity, got %+v", rule)
	}
	stderr := errBuf.String()
	if !strings.Contains(stderr, "multiple rules") {
		t.Errorf("stderr does not mention 'multiple rules': %s", stderr)
	}
	// Both IDs should appear in the stderr output
	if !strings.Contains(stderr, "id-a") {
		t.Errorf("stderr does not list id-a: %s", stderr)
	}
	if !strings.Contains(stderr, "id-c") {
		t.Errorf("stderr does not list id-c: %s", stderr)
	}
}

func TestResolveRule_NotFound(t *testing.T) {
	srv := makeForwardsServer(t, []client.ForwardRuleResponse{ruleA, ruleB})
	defer srv.Close()

	c := client.New(srv.URL)
	var errBuf strings.Builder
	rule, code := commands.ResolveRule(c, "nonexistent", &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if rule != nil {
		t.Errorf("expected nil rule on not found, got %+v", rule)
	}
	if !strings.Contains(errBuf.String(), "no rule found") {
		t.Errorf("stderr does not mention 'no rule found': %s", errBuf.String())
	}
}

func TestResolveRule_EmptyRuleList(t *testing.T) {
	srv := makeForwardsServer(t, []client.ForwardRuleResponse{})
	defer srv.Close()

	c := client.New(srv.URL)
	var errBuf strings.Builder
	rule, code := commands.ResolveRule(c, "anything", &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if rule != nil {
		t.Errorf("expected nil rule on empty list, got %+v", rule)
	}
}
