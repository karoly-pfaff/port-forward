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

// --- StartForward ---

func TestStartForward_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/forwards/rule-1/start" {
			t.Errorf("path = %q, want /api/forwards/rule-1/start", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	if err := c.StartForward("rule-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartForward_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"rule already running"}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	err := c.StartForward("rule-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *client.APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want %d", apiErr.StatusCode, http.StatusConflict)
	}
}

func TestStartForward_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	err := c.StartForward("rule-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var connErr *client.ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *client.ConnectionError, got %T: %v", err, err)
	}
}

// --- StopForward ---

func TestStopForward_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/forwards/rule-1/stop" {
			t.Errorf("path = %q, want /api/forwards/rule-1/stop", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	if err := c.StopForward("rule-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopForward_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"rule not found"}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	err := c.StopForward("missing-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *client.APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

// --- DiagnoseForward ---

func diagnoseFixture() map[string]any {
	return map[string]any{
		"ruleId":   "rule-1",
		"ruleName": "Dev API",
		"protocol": "tcp",
		"summary": map[string]any{
			"status":  "pass",
			"message": "All checks passed.",
		},
		"checks": []map[string]any{
			{"id": "listen-host", "label": "Listen address", "status": "pass", "message": "Listening on loopback."},
			{"id": "target-connect", "label": "Target connection", "status": "pass", "message": "Connected to 192.168.1.10:8080."},
		},
		"diagnosedAt": "2026-01-01T12:00:00Z",
	}
}

func TestDiagnoseForward_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/forwards/rule-1/diagnose" {
			t.Errorf("path = %q, want /api/forwards/rule-1/diagnose", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(diagnoseFixture())
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	result, err := c.DiagnoseForward("rule-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RuleID != "rule-1" {
		t.Errorf("ruleId = %q, want %q", result.RuleID, "rule-1")
	}
	if result.RuleName != "Dev API" {
		t.Errorf("ruleName = %q, want %q", result.RuleName, "Dev API")
	}
	if result.Summary.Status != "pass" {
		t.Errorf("summary.status = %q, want %q", result.Summary.Status, "pass")
	}
	if len(result.Checks) != 2 {
		t.Errorf("len(checks) = %d, want 2", len(result.Checks))
	}
	if result.Checks[0].ID != "listen-host" {
		t.Errorf("checks[0].id = %q, want %q", result.Checks[0].ID, "listen-host")
	}
}

func TestDiagnoseForward_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"rule not found"}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	_, err := c.DiagnoseForward("missing-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *client.APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func TestDiagnoseForward_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	_, err := c.DiagnoseForward("rule-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var connErr *client.ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *client.ConnectionError, got %T: %v", err, err)
	}
}

// --- ExportConfig ---

func exportConfigFixture() map[string]any {
	return map[string]any{
		"version":    "1",
		"exportedAt": "2026-01-01T12:00:00Z",
		"rules": []map[string]any{
			{
				"id": "rule-1", "name": "Dev API", "protocol": "tcp",
				"listenHost": "127.0.0.1", "listenPort": 48000,
				"targetHost": "192.168.1.10", "targetPort": 8080, "enabled": true,
			},
		},
	}
}

func TestExportConfig_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/config/export" {
			t.Errorf("path = %q, want /api/config/export", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exportConfigFixture())
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	cfg, err := c.ExportConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Version != "1" {
		t.Errorf("version = %q, want %q", cfg.Version, "1")
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(cfg.Rules))
	}
	if cfg.Rules[0].Name != "Dev API" {
		t.Errorf("rules[0].name = %q, want %q", cfg.Rules[0].Name, "Dev API")
	}
	if cfg.Rules[0].ListenPort != 48000 {
		t.Errorf("rules[0].listenPort = %d, want 48000", cfg.Rules[0].ListenPort)
	}
}

func TestExportConfig_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"internal error"}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	_, err := c.ExportConfig()
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
}

func TestExportConfig_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	_, err := c.ExportConfig()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var connErr *client.ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *client.ConnectionError, got %T: %v", err, err)
	}
}

// --- ImportConfig ---

func importConfigFixture() map[string]any {
	return map[string]any{
		"result": map[string]any{"imported": 1, "skipped": 0, "errors": []any{}},
		"rules": []map[string]any{
			{
				"id": "rule-1", "name": "Dev API", "protocol": "tcp",
				"listenHost": "127.0.0.1", "listenPort": 48000,
				"targetHost": "192.168.1.10", "targetPort": 8080, "enabled": true,
				"advisories": []any{},
			},
		},
	}
}

func TestImportConfig_MergeSuccess(t *testing.T) {
	var gotMode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/config/import" {
			t.Errorf("path = %q, want /api/config/import", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotMode, _ = body["mode"].(string)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(importConfigFixture())
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	req := client.ConfigImportRequest{
		Mode:   "merge",
		Config: client.ConfigExportResponse{Version: "1", Rules: []client.ConfigRule{{Name: "Dev API", Protocol: "tcp", ListenHost: "127.0.0.1", ListenPort: 48000, TargetHost: "192.168.1.10", TargetPort: 8080}}},
	}
	resp, err := c.ImportConfig(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMode != "merge" {
		t.Errorf("mode sent = %q, want %q", gotMode, "merge")
	}
	if resp.Result.Imported != 1 {
		t.Errorf("imported = %d, want 1", resp.Result.Imported)
	}
	if len(resp.Rules) != 1 {
		t.Errorf("len(rules) = %d, want 1", len(resp.Rules))
	}
}

func TestImportConfig_ReplaceSuccess(t *testing.T) {
	var gotMode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotMode, _ = body["mode"].(string)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(importConfigFixture())
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	req := client.ConfigImportRequest{
		Mode:   "replace",
		Config: client.ConfigExportResponse{Version: "1", Rules: []client.ConfigRule{}},
	}
	_, err := c.ImportConfig(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMode != "replace" {
		t.Errorf("mode sent = %q, want %q", gotMode, "replace")
	}
}

func TestImportConfig_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]any{"errors": []string{"listenPort: 0 is invalid"}})
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	req := client.ConfigImportRequest{
		Mode:   "merge",
		Config: client.ConfigExportResponse{Version: "1", Rules: []client.ConfigRule{}},
	}
	_, err := c.ImportConfig(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *client.APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", apiErr.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestImportConfig_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := client.New(addr)
	req := client.ConfigImportRequest{
		Mode:   "merge",
		Config: client.ConfigExportResponse{Version: "1", Rules: []client.ConfigRule{}},
	}
	_, err := c.ImportConfig(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var connErr *client.ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *client.ConnectionError, got %T: %v", err, err)
	}
}
