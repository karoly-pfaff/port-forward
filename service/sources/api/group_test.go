package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"portier/service/sources/domain"
	"portier/service/sources/manager"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func gptr(s string) *string { return &s }

func groupTestRule(t *testing.T, id, name, group string) domain.ForwardRule {
	t.Helper()
	r := domain.ForwardRule{
		ID: id, Name: name, Protocol: domain.ProtocolTCP, ListenHost: "127.0.0.1",
		ListenPort: freePort(t), TargetHost: "127.0.0.1", TargetPort: 49999, Enabled: false,
	}
	if group != "" {
		r.Group = gptr(group)
	}
	return r
}

func newGroupServer(t *testing.T, rules []domain.ForwardRule) *httptest.Server {
	t.Helper()
	m, err := manager.New(rules)
	if err != nil {
		t.Fatalf("manager.New: %v", err)
	}
	h := NewHandler(Options{Manager: m, Version: "test"})
	srv := httptest.NewServer(h)
	t.Cleanup(func() {
		srv.Close()
		m.StopAll()
	})
	return srv
}

func postGroup(t *testing.T, srv *httptest.Server, path string) (*http.Response, domain.GroupActionResponse) {
	t.Helper()
	resp, err := http.Post(srv.URL+path, "application/json", nil)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	var body domain.GroupActionResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	return resp, body
}

func TestGroupStartReturnsSummary(t *testing.T) {
	srv := newGroupServer(t, []domain.ForwardRule{
		groupTestRule(t, "w1", "Web One", "web"),
		groupTestRule(t, "a1", "Api One", "api"),
		groupTestRule(t, "w2", "Web Two", "web"),
	})

	resp, body := postGroup(t, srv, "/api/forwards/groups/web/start")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body.Group != "web" || body.Action != "start" || body.Total != 2 || body.Succeeded != 2 {
		t.Fatalf("unexpected summary: %#v", body)
	}
	if len(body.Results) != 2 || body.Results[0].RuleID != "w1" || body.Results[1].RuleID != "w2" {
		t.Fatalf("results out of order: %#v", body.Results)
	}
	if body.Results[0].Status != "started" {
		t.Fatalf("expected started, got %#v", body.Results[0])
	}
}

func TestGroupStartSkipsAlreadyRunning(t *testing.T) {
	srv := newGroupServer(t, []domain.ForwardRule{groupTestRule(t, "w1", "Web One", "web")})

	postGroup(t, srv, "/api/forwards/groups/web/start")
	resp, body := postGroup(t, srv, "/api/forwards/groups/web/start")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body.Skipped != 1 || body.Results[0].Status != "skipped" || body.Results[0].Reason != "already_running" {
		t.Fatalf("expected skipped already_running, got %#v", body)
	}
}

func TestGroupStopStopsRunning(t *testing.T) {
	srv := newGroupServer(t, []domain.ForwardRule{groupTestRule(t, "w1", "Web One", "web")})

	postGroup(t, srv, "/api/forwards/groups/web/start")
	resp, body := postGroup(t, srv, "/api/forwards/groups/web/stop")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body.Action != "stop" || body.Succeeded != 1 || body.Results[0].Status != "stopped" {
		t.Fatalf("expected stopped, got %#v", body)
	}
}

func TestGroupNoMatch404(t *testing.T) {
	srv := newGroupServer(t, []domain.ForwardRule{groupTestRule(t, "w1", "Web One", "web")})

	resp, err := http.Post(srv.URL+"/api/forwards/groups/ghost/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGroupInvalidGroup400(t *testing.T) {
	srv := newGroupServer(t, []domain.ForwardRule{groupTestRule(t, "w1", "Web One", "web")})

	cases := []struct {
		path string
		want string
	}{
		{"/api/forwards/groups/" + strings.Repeat("x", 65) + "/start", "group must be 64 characters or fewer."},
		{"/api/forwards/groups/%20%20/start", "group is required."},
	}
	for _, c := range cases {
		resp, err := http.Post(srv.URL+c.path, "application/json", nil)
		if err != nil {
			t.Fatalf("POST %s: %v", c.path, err)
		}
		var body struct {
			Errors []string `json:"errors"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", c.path, resp.StatusCode)
		}
		found := false
		for _, e := range body.Errors {
			if e == c.want {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s errors = %#v, want %q", c.path, body.Errors, c.want)
		}
	}
}

func TestGroupActionRejectsNonPost(t *testing.T) {
	srv := newGroupServer(t, []domain.ForwardRule{groupTestRule(t, "w1", "Web One", "web")})

	resp, err := http.Get(srv.URL + "/api/forwards/groups/web/start")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGroupActionEncodedSpace(t *testing.T) {
	srv := newGroupServer(t, []domain.ForwardRule{groupTestRule(t, "w1", "Web One", "web team")})

	resp, body := postGroup(t, srv, "/api/forwards/groups/web%20team/start")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body.Group != "web team" || body.Succeeded != 1 {
		t.Fatalf("unexpected summary: %#v", body)
	}
}
