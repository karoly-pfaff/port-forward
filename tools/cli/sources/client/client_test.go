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
