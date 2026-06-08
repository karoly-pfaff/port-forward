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

func makeDiagnoseServer(
	t *testing.T,
	rules []client.ForwardRuleResponse,
	result *client.RuleDiagnosticsResult,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/forwards":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rules)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/diagnose"):
			if result == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string][]string{"errors": {"rule not found"}})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

var diagnoseResultAllStatuses = &client.RuleDiagnosticsResult{
	RuleID:   "rule-1",
	RuleName: "Local Postgres",
	Protocol: "tcp",
	Summary:  client.DiagnosticSummary{Status: "warn", Message: "Rule is reachable, but listens on all interfaces."},
	Checks: []client.DiagnosticCheck{
		{ID: "listen-host", Label: "Listen address", Status: "warn", Message: "Listening on all interfaces (0.0.0.0)."},
		{ID: "target-connect", Label: "Target connection", Status: "pass", Message: "Connected to 127.0.0.1:5432."},
		{ID: "privileged-port", Label: "Privileged port", Status: "skip", Message: "Port is above 1024."},
		{ID: "target-host", Label: "Target host", Status: "fail", Message: "DNS lookup failed for badhost."},
	},
	DiagnosedAt: "2026-01-01T12:00:00Z",
}

func TestRunDiagnose_HumanOutput(t *testing.T) {
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "Local Postgres", Protocol: "tcp",
		ListenHost: "0.0.0.0", ListenPort: 48010,
		TargetHost: "127.0.0.1", TargetPort: 5432, Enabled: true,
	}
	srv := makeDiagnoseServer(t, []client.ForwardRuleResponse{rule}, diagnoseResultAllStatuses)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunDiagnose(c, false, []string{"rule-1"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	output := out.String()
	// Header line
	if !strings.Contains(output, "Local Postgres") {
		t.Errorf("output missing rule name: %s", output)
	}
	if !strings.Contains(output, "warn") {
		t.Errorf("output missing summary status: %s", output)
	}
	if !strings.Contains(output, "listens on all interfaces") {
		t.Errorf("output missing summary message: %s", output)
	}
	// Check rows
	for _, want := range []string{"Listen address", "Target connection", "Privileged port", "Target host"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing check label %q: %s", want, output)
		}
	}
	for _, want := range []string{"pass", "fail", "skip"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing status %q: %s", want, output)
		}
	}
}

func TestRunDiagnose_JSONOutput(t *testing.T) {
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "Local Postgres", Protocol: "tcp",
		ListenHost: "0.0.0.0", ListenPort: 48010,
		TargetHost: "127.0.0.1", TargetPort: 5432, Enabled: true,
	}
	srv := makeDiagnoseServer(t, []client.ForwardRuleResponse{rule}, diagnoseResultAllStatuses)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunDiagnose(c, true, []string{"rule-1"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	var decoded client.RuleDiagnosticsResult
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("--json output is not valid RuleDiagnosticsResult: %v\noutput:\n%s", err, out.String())
	}
	if decoded.RuleID != "rule-1" {
		t.Errorf("ruleId = %q, want rule-1", decoded.RuleID)
	}
	if decoded.Summary.Status != "warn" {
		t.Errorf("summary.status = %q, want warn", decoded.Summary.Status)
	}
	if len(decoded.Checks) != 4 {
		t.Errorf("len(checks) = %d, want 4", len(decoded.Checks))
	}
}

func TestRunDiagnose_NotFound(t *testing.T) {
	srv := makeDiagnoseServer(t, []client.ForwardRuleResponse{}, nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	// Use an ID that won't match any rule in the empty list — resolver exits 1
	code := commands.RunDiagnose(c, false, []string{"missing-rule"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunDiagnose_MissingArg(t *testing.T) {
	c := client.New("http://127.0.0.1:0")
	var out, errBuf strings.Builder
	code := commands.RunDiagnose(c, false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunDiagnose_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunDiagnose(c, false, []string{"rule-1"}, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

func TestRunDiagnose_EmptyChecks(t *testing.T) {
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "API", Protocol: "tcp",
		ListenHost: "127.0.0.1", ListenPort: 48000,
		TargetHost: "10.0.0.1", TargetPort: 8080, Enabled: true,
	}
	// Checks is nil → printDiagnoseHuman early-return branch
	result := &client.RuleDiagnosticsResult{
		RuleID:      "rule-1",
		RuleName:    "API",
		Protocol:    "tcp",
		Summary:     client.DiagnosticSummary{Status: "pass", Message: "No checks run."},
		Checks:      nil,
		DiagnosedAt: "2026-01-01T00:00:00Z",
	}
	srv := makeDiagnoseServer(t, []client.ForwardRuleResponse{rule}, result)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunDiagnose(c, false, []string{"rule-1"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "API") {
		t.Errorf("output should contain rule name: %s", out.String())
	}
	// No checks table when Checks is empty
	if strings.Contains(out.String(), "CHECK") {
		t.Errorf("output should not show checks table when Checks is empty: %s", out.String())
	}
}

func TestRunDiagnose_DiagnoseAPIError(t *testing.T) {
	// ResolveRule succeeds but DiagnoseForward returns a non-connection API error.
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "API Rule", Protocol: "tcp",
		ListenHost: "127.0.0.1", ListenPort: 48000,
		TargetHost: "10.0.0.1", TargetPort: 8080, Enabled: true,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/forwards":
			json.NewEncoder(w).Encode([]client.ForwardRuleResponse{rule})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/diagnose"):
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string][]string{"errors": {"service unavailable"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunDiagnose(c, false, []string{"rule-1"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "API error") {
		t.Errorf("stderr should mention API error: %s", errBuf.String())
	}
}

func TestRunDiagnose_ByNameHumanOutput(t *testing.T) {
	rule := client.ForwardRuleResponse{
		ID: "rule-1", Name: "Local Postgres", Protocol: "tcp",
		ListenHost: "127.0.0.1", ListenPort: 48010,
		TargetHost: "127.0.0.1", TargetPort: 5432, Enabled: true,
	}
	result := &client.RuleDiagnosticsResult{
		RuleID:   "rule-1",
		RuleName: "Local Postgres",
		Protocol: "tcp",
		Summary:  client.DiagnosticSummary{Status: "pass", Message: "All checks passed."},
		Checks: []client.DiagnosticCheck{
			{ID: "listen-host", Label: "Listen address", Status: "pass", Message: "Listening on loopback."},
		},
		DiagnosedAt: "2026-01-01T12:00:00Z",
	}
	srv := makeDiagnoseServer(t, []client.ForwardRuleResponse{rule}, result)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunDiagnose(c, false, []string{"Local Postgres"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Local Postgres") {
		t.Errorf("output missing rule name: %s", out.String())
	}
}
