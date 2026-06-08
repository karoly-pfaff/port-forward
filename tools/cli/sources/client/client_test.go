package client_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"portier/cli/sources/client"
)

func runtimeFixture() map[string]any {
	return map[string]any{
		"name":           "Portier",
		"version":        "1.2.0",
		"runtime":        "go",
		"platform":       "windows",
		"arch":           "x64",
		"uptimeSeconds":  42,
		"startedAt":      "2026-01-01T00:00:00Z",
		"managementHost": "127.0.0.1",
		"managementPort": 47831,
		"configPath":     "/path/to/rules.json",
		"staticDir":      "/path/to/web",
		"serviceMode":    false,
		"pid":            12345,
	}
}

func TestGetRuntime_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/runtime" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(runtimeFixture())
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	info, err := c.GetRuntime()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "Portier" {
		t.Errorf("name = %q, want %q", info.Name, "Portier")
	}
	if info.Version != "1.2.0" {
		t.Errorf("version = %q, want %q", info.Version, "1.2.0")
	}
	if info.Runtime != "go" {
		t.Errorf("runtime = %q, want %q", info.Runtime, "go")
	}
	if info.ManagementPort != 47831 {
		t.Errorf("managementPort = %d, want %d", info.ManagementPort, 47831)
	}
	if info.PID != 12345 {
		t.Errorf("pid = %d, want %d", info.PID, 12345)
	}
}

func TestGetRuntime_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"internal server error"}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	_, err := c.GetRuntime()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *client.APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("status = %d, want 500", apiErr.StatusCode)
	}
	if len(apiErr.Messages) == 0 {
		t.Error("expected error messages, got none")
	}
}

func TestGetRuntime_ConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	_, err := c.GetRuntime()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var connErr *client.ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *client.ConnectionError, got %T: %v", err, err)
	}
	if connErr.URL == "" {
		t.Error("ConnectionError.URL should not be empty")
	}
}

func TestGetRuntime_UserAgent(t *testing.T) {
	var gotUserAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client.RuntimeInfo{Name: "Portier"})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	_, _ = c.GetRuntime()
	if !strings.HasPrefix(gotUserAgent, "PortierCLI/") {
		t.Errorf("User-Agent = %q, want prefix %q", gotUserAgent, "PortierCLI/")
	}
}

func TestGetRuntime_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json {{{"))
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	_, err := c.GetRuntime()
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestAPIError_Error_WithMessages(t *testing.T) {
	err := &client.APIError{StatusCode: 404, Messages: []string{"not found"}}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error string does not contain status code: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error string does not contain message: %s", err.Error())
	}
}

func TestAPIError_Error_NoMessages(t *testing.T) {
	err := &client.APIError{StatusCode: 503}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error string does not contain status code: %s", err.Error())
	}
}

func TestConnectionError_Unwrap(t *testing.T) {
	inner := errors.New("dial: connection refused")
	err := &client.ConnectionError{URL: "http://127.0.0.1:47831", Err: inner}
	if !errors.Is(err, inner) {
		t.Error("Unwrap should expose inner error")
	}
}

// --- GetForwards ---

func forwardRuleFixture() map[string]any {
	return map[string]any{
		"id":         "rule-1",
		"name":       "Dev API",
		"protocol":   "tcp",
		"listenHost": "127.0.0.1",
		"listenPort": 48000,
		"targetHost": "192.168.1.10",
		"targetPort": 8080,
		"enabled":    true,
		"advisories": []any{},
	}
}

func TestGetForwards_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/forwards" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{forwardRuleFixture()})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	rules, err := c.GetForwards()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
	if rules[0].ID != "rule-1" {
		t.Errorf("id = %q, want %q", rules[0].ID, "rule-1")
	}
	if rules[0].Name != "Dev API" {
		t.Errorf("name = %q, want %q", rules[0].Name, "Dev API")
	}
	if rules[0].Protocol != "tcp" {
		t.Errorf("protocol = %q, want %q", rules[0].Protocol, "tcp")
	}
	if rules[0].ListenPort != 48000 {
		t.Errorf("listenPort = %d, want %d", rules[0].ListenPort, 48000)
	}
}

func TestGetForwards_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	rules, err := c.GetForwards()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("len(rules) = %d, want 0", len(rules))
	}
}

func TestGetForwards_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"internal error"}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	_, err := c.GetForwards()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *client.APIError, got %T", err)
	}
}

func TestGetForwards_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	_, err := c.GetForwards()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var connErr *client.ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *client.ConnectionError, got %T", err)
	}
}

// --- GetStatus ---

func TestGetStatus_Success(t *testing.T) {
	conns := 2
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{{
			"ruleId":            "rule-1",
			"running":           true,
			"bytesIn":           1024,
			"bytesOut":          512,
			"activeConnections": conns,
		}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	statuses, err := c.GetStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("len(statuses) = %d, want 1", len(statuses))
	}
	if statuses[0].RuleID != "rule-1" {
		t.Errorf("ruleId = %q, want %q", statuses[0].RuleID, "rule-1")
	}
	if !statuses[0].Running {
		t.Error("running = false, want true")
	}
	if statuses[0].BytesIn != 1024 {
		t.Errorf("bytesIn = %d, want 1024", statuses[0].BytesIn)
	}
	if statuses[0].ActiveConnections == nil || *statuses[0].ActiveConnections != 2 {
		t.Errorf("activeConnections = %v, want 2", statuses[0].ActiveConnections)
	}
}

func TestGetStatus_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	statuses, err := c.GetStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("len(statuses) = %d, want 0", len(statuses))
	}
}

// --- GetActivity ---

func activityEventFixture() map[string]any {
	return map[string]any{
		"id":        "evt-1",
		"timestamp": "2026-01-01T12:00:00Z",
		"type":      "rule.started",
		"severity":  "info",
		"ruleId":    "rule-1",
		"ruleName":  "Dev API",
		"message":   "Rule started successfully.",
	}
}

func TestGetActivity_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"events": []map[string]any{activityEventFixture()},
		})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	events, err := c.GetActivity(client.ActivityQuery{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].ID != "evt-1" {
		t.Errorf("id = %q, want %q", events[0].ID, "evt-1")
	}
	if events[0].Type != "rule.started" {
		t.Errorf("type = %q, want %q", events[0].Type, "rule.started")
	}
	if events[0].RuleName != "Dev API" {
		t.Errorf("ruleName = %q, want %q", events[0].RuleName, "Dev API")
	}
}

func TestGetActivity_QueryParams(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"events": []any{}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	_, err := c.GetActivity(client.ActivityQuery{
		Limit:    25,
		RuleID:   "rule-1",
		Type:     "rule.started",
		Severity: "info",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"limit=25", "ruleId=rule-1", "type=rule.started", "severity=info"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q does not contain %q", gotQuery, want)
		}
	}
}

func TestGetActivity_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"events": []any{}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	events, err := c.GetActivity(client.ActivityQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("len(events) = %d, want 0", len(events))
	}
}
