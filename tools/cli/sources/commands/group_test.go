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

// makeGroupListServer serves GET /api/forwards and GET /api/status.
func makeGroupListServer(t *testing.T, rules []client.ForwardRuleResponse, statuses []client.ForwardStatus) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/forwards":
			json.NewEncoder(w).Encode(rules)
		case r.Method == http.MethodGet && r.URL.Path == "/api/status":
			json.NewEncoder(w).Encode(statuses)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

// makeGroupActionServer serves POST /api/forwards/groups/:group/:action with the
// given response, asserting the expected path was hit.
func makeGroupActionServer(t *testing.T, wantPath string, status int, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Compare the escaped path so an encoded "/" or space in a group name is
		// asserted in its on-the-wire form (mirrors how the service parses it).
		if r.Method == http.MethodPost && r.URL.EscapedPath() == wantPath {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(resp)
			return
		}
		t.Errorf("unexpected %s %s (want POST %s)", r.Method, r.URL.EscapedPath(), wantPath)
		http.NotFound(w, r)
	}))
}

func ruleWithGroup(id, name, group string) client.ForwardRuleResponse {
	return client.ForwardRuleResponse{
		ID: id, Name: name, Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000,
		TargetHost: "127.0.0.1", TargetPort: 9000, Enabled: false, Group: group,
	}
}

// --- group list ---

func TestRunGroupList_Human(t *testing.T) {
	rules := []client.ForwardRuleResponse{
		ruleWithGroup("w1", "Web One", "web"),
		ruleWithGroup("a1", "Api One", "api"),
		ruleWithGroup("w2", "Web Two", "web"),
		ruleWithGroup("u1", "Loose", ""), // ungrouped
	}
	statuses := []client.ForwardStatus{
		{RuleID: "w1", Running: true},
		{RuleID: "w2", Running: false},
		{RuleID: "a1", Running: false},
	}
	srv := makeGroupListServer(t, rules, statuses)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, false, []string{"list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, errBuf.String())
	}
	s := out.String()
	// api sorts before web; web has 2 rules, 1 running.
	if !strings.Contains(s, "GROUP") || !strings.Contains(s, "RUNNING") {
		t.Errorf("missing table headers: %s", s)
	}
	if !strings.Contains(s, "web") || !strings.Contains(s, "api") {
		t.Errorf("missing group names: %s", s)
	}
	apiIdx := strings.Index(s, "api")
	webIdx := strings.Index(s, "web")
	if apiIdx < 0 || webIdx < 0 || apiIdx > webIdx {
		t.Errorf("groups not alphabetically sorted: %s", s)
	}
	if strings.Contains(s, "Loose") {
		t.Errorf("ungrouped rule name leaked into group list: %s", s)
	}
}

func TestRunGroupList_JSON(t *testing.T) {
	rules := []client.ForwardRuleResponse{
		ruleWithGroup("w1", "Web One", "web"),
		ruleWithGroup("w2", "Web Two", "web"),
	}
	statuses := []client.ForwardStatus{{RuleID: "w1", Running: true}, {RuleID: "w2", Running: false}}
	srv := makeGroupListServer(t, rules, statuses)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, true, []string{"list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var decoded struct {
		Groups []struct {
			Group   string `json:"group"`
			Total   int    `json:"total"`
			Running int    `json:"running"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(decoded.Groups) != 1 || decoded.Groups[0].Group != "web" || decoded.Groups[0].Total != 2 || decoded.Groups[0].Running != 1 {
		t.Errorf("unexpected groups: %+v", decoded.Groups)
	}
}

func TestRunGroupList_Empty(t *testing.T) {
	rules := []client.ForwardRuleResponse{ruleWithGroup("u1", "Loose", "")}
	srv := makeGroupListServer(t, rules, []client.ForwardStatus{{RuleID: "u1", Running: false}})
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, false, []string{"list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "No rule groups configured.") {
		t.Errorf("missing empty message: %s", out.String())
	}
}

// --- group start ---

func TestRunGroupStart_Success(t *testing.T) {
	resp := client.GroupActionResponse{
		Group: "web", Action: "start", Total: 2, Succeeded: 2, Skipped: 0, Failed: 0,
		Results: []client.GroupActionResult{
			{RuleID: "w1", RuleName: "Web One", Status: "started"},
			{RuleID: "w2", RuleName: "Web Two", Status: "started"},
		},
	}
	srv := makeGroupActionServer(t, "/api/forwards/groups/web/start", http.StatusOK, resp)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, false, []string{"start", "web"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, errBuf.String())
	}
	s := out.String()
	if !strings.Contains(s, "2 succeeded") || !strings.Contains(s, "Web One") || !strings.Contains(s, "started") {
		t.Errorf("unexpected output: %s", s)
	}
}

func TestRunGroupStart_SkippedAlreadyRunning(t *testing.T) {
	resp := client.GroupActionResponse{
		Group: "web", Action: "start", Total: 1, Succeeded: 0, Skipped: 1, Failed: 0,
		Results: []client.GroupActionResult{
			{RuleID: "w1", RuleName: "Web One", Status: "skipped", Reason: "already_running"},
		},
	}
	srv := makeGroupActionServer(t, "/api/forwards/groups/web/start", http.StatusOK, resp)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, false, []string{"start", "web"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (skip is not a failure); stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "already_running") {
		t.Errorf("output missing skip reason: %s", out.String())
	}
}

func TestRunGroupStart_JSON(t *testing.T) {
	resp := client.GroupActionResponse{
		Group: "web", Action: "start", Total: 1, Succeeded: 1,
		Results: []client.GroupActionResult{{RuleID: "w1", RuleName: "Web One", Status: "started"}},
	}
	srv := makeGroupActionServer(t, "/api/forwards/groups/web/start", http.StatusOK, resp)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, true, []string{"start", "web"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var decoded client.GroupActionResponse
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if decoded.Group != "web" || decoded.Action != "start" || decoded.Succeeded != 1 {
		t.Errorf("unexpected JSON: %+v", decoded)
	}
}

func TestRunGroupStart_FailedExit1(t *testing.T) {
	resp := client.GroupActionResponse{
		Group: "web", Action: "start", Total: 2, Succeeded: 1, Skipped: 0, Failed: 1,
		Results: []client.GroupActionResult{
			{RuleID: "bad", RuleName: "Bad", Status: "failed", Reason: "bind: address already in use"},
			{RuleID: "good", RuleName: "Good", Status: "started"},
		},
	}
	srv := makeGroupActionServer(t, "/api/forwards/groups/web/start", http.StatusOK, resp)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, false, []string{"start", "web"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (failed > 0)", code)
	}
	if !strings.Contains(errBuf.String(), "failed") {
		t.Errorf("stderr missing failure note: %s", errBuf.String())
	}
}

func TestRunGroupStart_NoMatch404Exit1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {`No rules found in group "ghost".`}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, false, []string{"start", "ghost"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (404 API error)", code)
	}
	if !strings.Contains(errBuf.String(), "No rules found in group") {
		t.Errorf("stderr missing API error message: %s", errBuf.String())
	}
}

func TestRunGroupStart_MissingArgExit2(t *testing.T) {
	c := client.New("http://127.0.0.1:1") // never called
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, false, []string{"start"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "requires a group name") {
		t.Errorf("stderr missing usage: %s", errBuf.String())
	}
}

func TestRunGroupStart_InvalidGroupExit2(t *testing.T) {
	c := client.New("http://127.0.0.1:1") // never called — local validation fails first
	cases := []struct {
		arg  string
		want string
	}{
		{"   ", "group is required."},
		{strings.Repeat("x", 65), "group must be 64 characters or fewer."},
		{"bad\tgrp", "group must not contain control characters."},
	}
	for _, tc := range cases {
		var out, errBuf strings.Builder
		code := commands.RunGroup(c, false, []string{"start", tc.arg}, &out, &errBuf)
		if code != 2 {
			t.Errorf("arg %q: exit = %d, want 2", tc.arg, code)
		}
		if !strings.Contains(errBuf.String(), tc.want) {
			t.Errorf("arg %q: stderr = %q, want %q", tc.arg, errBuf.String(), tc.want)
		}
	}
}

func TestRunGroupStart_ConnectionErrorExit3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, false, []string{"start", "web"}, &out, &errBuf)
	if code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
}

// --- group stop ---

func TestRunGroupStop_Success(t *testing.T) {
	resp := client.GroupActionResponse{
		Group: "web", Action: "stop", Total: 2, Succeeded: 1, Skipped: 1, Failed: 0,
		Results: []client.GroupActionResult{
			{RuleID: "w1", RuleName: "Web One", Status: "stopped"},
			{RuleID: "w2", RuleName: "Web Two", Status: "skipped", Reason: "not_running"},
		},
	}
	srv := makeGroupActionServer(t, "/api/forwards/groups/web/stop", http.StatusOK, resp)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, false, []string{"stop", "web"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, errBuf.String())
	}
	s := out.String()
	if !strings.Contains(s, "stop") || !strings.Contains(s, "stopped") || !strings.Contains(s, "not_running") {
		t.Errorf("unexpected output: %s", s)
	}
}

func TestRunGroupStop_EncodedGroupPath(t *testing.T) {
	resp := client.GroupActionResponse{Group: "web team", Action: "stop", Total: 1, Succeeded: 1,
		Results: []client.GroupActionResult{{RuleID: "w1", RuleName: "Web One", Status: "stopped"}}}
	srv := makeGroupActionServer(t, "/api/forwards/groups/web%20team/stop", http.StatusOK, resp)
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunGroup(c, false, []string{"stop", "  web team  "}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, errBuf.String())
	}
}

// --- dispatch / help ---

func TestRunGroup_NoArgsExit2(t *testing.T) {
	c := client.New("http://127.0.0.1:1")
	var out, errBuf strings.Builder
	if code := commands.RunGroup(c, false, []string{}, &out, &errBuf); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRunGroup_Help(t *testing.T) {
	c := client.New("http://127.0.0.1:1")
	var out, errBuf strings.Builder
	if code := commands.RunGroup(c, false, []string{"help"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: portier group") {
		t.Errorf("help missing usage: %s", out.String())
	}
}

func TestRunGroup_UnknownSubcommandExit2(t *testing.T) {
	c := client.New("http://127.0.0.1:1")
	var out, errBuf strings.Builder
	if code := commands.RunGroup(c, false, []string{"frobnicate"}, &out, &errBuf); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "Unknown group subcommand") {
		t.Errorf("stderr missing unknown subcommand: %s", errBuf.String())
	}
}

func TestRunGroupList_ConnectionErrorExit3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	var out, errBuf strings.Builder
	if code := commands.RunGroup(c, false, []string{"list"}, &out, &errBuf); code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
}
