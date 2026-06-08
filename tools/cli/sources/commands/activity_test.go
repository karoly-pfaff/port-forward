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

func makeActivityServer(t *testing.T, events []client.ActivityEvent) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"events": events})
	}))
}

func TestRunActivity_HumanOutput(t *testing.T) {
	events := []client.ActivityEvent{
		{
			ID: "e1", Timestamp: "2026-01-01T12:00:00Z",
			Type: "rule.started", Severity: "info",
			RuleID: "r1", RuleName: "Dev API",
			Message: "Rule started successfully.",
		},
	}
	srv := makeActivityServer(t, events)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunActivity(c, false, []string{}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	output := out.String()
	for _, want := range []string{"rule.started", "info", "Dev API", "Rule started successfully"} {
		if !strings.Contains(output, want) {
			t.Errorf("human output does not contain %q\nfull output:\n%s", want, output)
		}
	}
}

func TestRunActivity_JSONOutput(t *testing.T) {
	events := []client.ActivityEvent{
		{ID: "e1", Timestamp: "2026-01-01T12:00:00Z", Type: "rule.started",
			Severity: "info", Message: "started"},
	}
	srv := makeActivityServer(t, events)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunActivity(c, true, []string{}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	// JSON output must be a raw array, not wrapped in {"events": [...]}
	var decoded []client.ActivityEvent
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("--json output is not a valid JSON array: %v\noutput:\n%s", err, out.String())
	}
	if len(decoded) != 1 || decoded[0].ID != "e1" {
		t.Errorf("unexpected decoded events: %+v", decoded)
	}
}

func TestRunActivity_EmptyState(t *testing.T) {
	srv := makeActivityServer(t, []client.ActivityEvent{})
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunActivity(c, false, []string{}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "No activity events") {
		t.Errorf("empty output missing friendly message: %q", out.String())
	}
}

func TestRunActivity_FiltersPassedAsQueryParams(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"events": []any{}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	args := []string{"--limit", "25", "--rule", "r-abc", "--type", "rule.started", "--severity", "info"}
	code := commands.RunActivity(c, false, args, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	for _, want := range []string{"limit=25", "ruleId=r-abc", "type=rule.started", "severity=info"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q does not contain %q", gotQuery, want)
		}
	}
}

func TestRunActivity_InvalidLimit_Zero(t *testing.T) {
	srv := makeActivityServer(t, nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunActivity(c, false, []string{"--limit", "0"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (invalid args)", code)
	}
}

func TestRunActivity_InvalidLimit_TooLarge(t *testing.T) {
	srv := makeActivityServer(t, nil)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunActivity(c, false, []string{"--limit", "501"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (invalid args)", code)
	}
}

func TestRunActivity_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunActivity(c, false, []string{}, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
	if !strings.Contains(errBuf.String(), "could not connect") {
		t.Errorf("stderr missing connection message: %s", errBuf.String())
	}
}

func TestRunActivity_RuleNameFallsBackToRuleID(t *testing.T) {
	events := []client.ActivityEvent{
		{ID: "e1", Timestamp: "2026-01-01T12:00:00Z", Type: "rule.error",
			Severity: "error", RuleID: "r-noid", Message: "something failed"},
	}
	srv := makeActivityServer(t, events)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunActivity(c, false, []string{}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "r-noid") {
		t.Errorf("output should show ruleId when ruleName is absent: %s", out.String())
	}
}
