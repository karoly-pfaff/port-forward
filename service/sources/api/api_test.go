package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portier/service/sources/activity"
	"portier/service/sources/manager"
	"portier/service/sources/options"
)

func TestHealthEndpoint(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["ok"] != true || body["server"] != "go" || body["name"] != "Portier" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestUnknownAPIRouteReturnsJSON404(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html><body>Portier UI</body></html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	server := httptest.NewServer(newTestHandler(t, staticDir, "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/nope")
	if err != nil {
		t.Fatalf("GET /api/nope failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("content type = %q, want application/json", contentType)
	}

	var body struct {
		Errors []string `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Errors) != 1 || body.Errors[0] != "API route was not found." {
		t.Fatalf("errors = %#v", body.Errors)
	}
}

func TestMissingStaticDirDoesNotPreventAPI(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, filepath.Join(t.TempDir(), "missing"), "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}

func TestStaticIndexFallbackWorks(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html><body>Portier UI</body></html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	server := httptest.NewServer(newTestHandler(t, staticDir, "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/some/spa/route")
	if err != nil {
		t.Fatalf("GET fallback route failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !strings.Contains(string(body), "Portier UI") {
		t.Fatalf("body = %q", string(body))
	}
}

func TestAPILikeSPARouteFallsBackToIndex(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html><body>Portier UI</body></html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	server := httptest.NewServer(newTestHandler(t, staticDir, "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/apiary")
	if err != nil {
		t.Fatalf("GET /apiary failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}

func TestActivityEndpointReturnsEmptyEvents(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/activity")
	if err != nil {
		t.Fatalf("GET /api/activity failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Events []any `json:"events"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Events == nil {
		t.Fatal("events should be an array, not null")
	}
	if len(body.Events) != 0 {
		t.Fatalf("expected empty events, got %d", len(body.Events))
	}
}

func TestUnknownMutatingAPIRouteReturnsJSON404(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/unknown-action", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST /api/unknown-action failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
}

func TestForwardsReturnsLoadedRules(t *testing.T) {
	configPath := writeTestConfig(t, `[
  {
    "id": "rule-1",
    "name": "Local app",
    "protocol": "tcp",
    "listenHost": "0.0.0.0",
    "listenPort": 3000,
    "targetHost": "127.0.0.1",
    "targetPort": 3000,
    "enabled": true
  }
]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body []map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("rule count = %d, want 1", len(body))
	}
	if body[0]["id"] != "rule-1" {
		t.Fatalf("rule = %#v", body[0])
	}
	advisories, ok := body[0]["advisories"].([]any)
	if !ok || len(advisories) == 0 {
		t.Fatalf("advisories = %#v", body[0]["advisories"])
	}
}

func TestForwardsReturnsEmptyArrayWhenConfigMissing(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", filepath.Join(t.TempDir(), "missing.json")))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer response.Body.Close()

	var body []any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("body = %#v, want empty array", body)
	}
}

func TestPortAdvisoryEndpoint(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/ports/advisory?port=48001&listenHost=0.0.0.0&purpose=forward")
	if err != nil {
		t.Fatalf("GET /api/ports/advisory failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body []map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 || body[0]["code"] != "LAN_EXPOSURE" {
		t.Fatalf("body = %#v", body)
	}
}

func TestStatusReturnsStatusesForLoadedRules(t *testing.T) {
	configPath := writeTestConfig(t, `[
  {
    "id": "tcp-1",
    "name": "Local app",
    "protocol": "tcp",
    "listenHost": "127.0.0.1",
    "listenPort": 48001,
    "targetHost": "127.0.0.1",
    "targetPort": 3000,
    "enabled": true
  },
  {
    "id": "udp-1",
    "name": "Stats",
    "protocol": "udp",
    "listenHost": "127.0.0.1",
    "listenPort": 48002,
    "targetHost": "127.0.0.1",
    "targetPort": 9000,
    "enabled": false,
    "udpMode": "bidirectional-multi-client"
  }
]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatalf("GET /api/status failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body []map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("status count = %d, want 2", len(body))
	}
	if body[0]["ruleId"] != "tcp-1" || body[0]["running"] != false || body[0]["activeConnections"] != float64(0) {
		t.Fatalf("tcp status = %#v", body[0])
	}
	if body[1]["ruleId"] != "udp-1" || body[1]["packetsIn"] != float64(0) || body[1]["activeUdpSessions"] != float64(0) {
		t.Fatalf("udp status = %#v", body[1])
	}
}

func TestStatusReturnsEmptyListWhenConfigMissing(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", filepath.Join(t.TempDir(), "missing.json")))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatalf("GET /api/status failed: %v", err)
	}
	defer response.Body.Close()

	var body []any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("body = %#v, want empty array", body)
	}
}

func TestConfigExportReturnsVersionTimestampAndRules(t *testing.T) {
	configPath := writeTestConfig(t, `[
  {
    "id": "rule-1",
    "name": "Local app",
    "protocol": "tcp",
    "listenHost": "127.0.0.1",
    "listenPort": 48001,
    "targetHost": "127.0.0.1",
    "targetPort": 3000,
    "enabled": true
  }
]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/config/export")
	if err != nil {
		t.Fatalf("GET /api/config/export failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Version    string           `json:"version"`
		ExportedAt string           `json:"exportedAt"`
		Rules      []map[string]any `json:"rules"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Version != "1" || body.ExportedAt == "" || len(body.Rules) != 1 {
		t.Fatalf("body = %#v", body)
	}
}

func TestCreateForward(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "forwards.json")
	targetPort, stopTarget := startAPIEchoServer(t, "create")
	defer stopTarget()
	listenPort := freeAPITCPPort(t)
	handler := newTestHandler(t, "", configPath)
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(fmt.Sprintf(`{
  "name": "Local app",
  "protocol": "tcp",
  "listenHost": "127.0.0.1",
  "listenPort": %d,
  "targetHost": "127.0.0.1",
  "targetPort": %d,
  "enabled": true
}`, listenPort, targetPort)))
	if err != nil {
		t.Fatalf("POST /api/forwards failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["id"] == "" || body["name"] != "Local app" {
		t.Fatalf("body = %#v", body)
	}

	statusResponse, err := http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatalf("GET /api/status failed: %v", err)
	}
	defer statusResponse.Body.Close()
	var statuses []map[string]any
	if err := json.NewDecoder(statusResponse.Body).Decode(&statuses); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if len(statuses) != 1 || statuses[0]["running"] != true {
		t.Fatalf("enabled created TCP rule should be running: %#v", statuses)
	}
	if got := requestAPIForwarder(t, listenPort, "hello"); got != "create:hello\n" {
		t.Fatalf("forwarded response = %q", got)
	}
}

func TestPatchForward(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", writeSingleRuleConfig(t)))
	defer server.Close()

	request, err := http.NewRequest(http.MethodPatch, server.URL+"/api/forwards/rule-1", strings.NewReader(`{"enabled":false}`))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("PATCH /api/forwards/rule-1 failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["enabled"] != false || body["name"] != "Local app" {
		t.Fatalf("body = %#v", body)
	}
}

func TestPatchForwardingFieldRestartsTCPForwarder(t *testing.T) {
	firstPort, stopFirst := startAPIEchoServer(t, "first")
	defer stopFirst()
	secondPort, stopSecond := startAPIEchoServer(t, "second")
	defer stopSecond()
	listenPort := freeAPITCPPort(t)
	handler := newTestHandler(t, "", writeTCPRuleConfig(t, "rule-1", listenPort, firstPort, true))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	startResponse, err := http.Post(server.URL+"/api/forwards/rule-1/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST start failed: %v", err)
	}
	startResponse.Body.Close()
	if startResponse.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d", startResponse.StatusCode)
	}
	if got := requestAPIForwarder(t, listenPort, "ping"); got != "first:ping\n" {
		t.Fatalf("before patch response = %q", got)
	}

	request, err := http.NewRequest(http.MethodPatch, server.URL+"/api/forwards/rule-1", strings.NewReader(fmt.Sprintf(`{"targetPort":%d}`, secondPort)))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("PATCH /api/forwards/rule-1 failed: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d", response.StatusCode)
	}
	if got := requestAPIForwarder(t, listenPort, "ping"); got != "second:ping\n" {
		t.Fatalf("after patch response = %q", got)
	}
}

func TestDeleteForward(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", writeSingleRuleConfig(t)))
	defer server.Close()

	request, err := http.NewRequest(http.MethodDelete, server.URL+"/api/forwards/rule-1", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("DELETE /api/forwards/rule-1 failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
}

func TestDeleteForwardStopsRunningTCPForwarder(t *testing.T) {
	targetPort, stopTarget := startAPIEchoServer(t, "delete")
	defer stopTarget()
	listenPort := freeAPITCPPort(t)
	handler := newTestHandler(t, "", writeTCPRuleConfig(t, "rule-1", listenPort, targetPort, true))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	startResponse, err := http.Post(server.URL+"/api/forwards/rule-1/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST start failed: %v", err)
	}
	startResponse.Body.Close()
	if startResponse.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d", startResponse.StatusCode)
	}

	request, err := http.NewRequest(http.MethodDelete, server.URL+"/api/forwards/rule-1", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("DELETE /api/forwards/rule-1 failed: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", response.StatusCode)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", listenPort)), 100*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected deleted listener to reject new connections")
	}
}

func TestStartAndStopForward(t *testing.T) {
	targetPort, stopTarget := startAPIEchoServer(t, "start")
	defer stopTarget()
	listenPort := freeAPITCPPort(t)
	handler := newTestHandler(t, "", writeTCPRuleConfig(t, "rule-1", listenPort, targetPort, true))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	startResponse, err := http.Post(server.URL+"/api/forwards/rule-1/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST start failed: %v", err)
	}
	defer startResponse.Body.Close()
	if startResponse.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d", startResponse.StatusCode)
	}
	var startBody map[string]any
	if err := json.NewDecoder(startResponse.Body).Decode(&startBody); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	if startBody["running"] != true || startBody["startedAt"] == "" {
		t.Fatalf("start body = %#v", startBody)
	}
	statusResponse, err := http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatalf("GET /api/status failed: %v", err)
	}
	defer statusResponse.Body.Close()
	var statuses []map[string]any
	if err := json.NewDecoder(statusResponse.Body).Decode(&statuses); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if len(statuses) != 1 || statuses[0]["running"] != true {
		t.Fatalf("status after start = %#v", statuses)
	}
	if got := requestAPIForwarder(t, listenPort, "hello"); got != "start:hello\n" {
		t.Fatalf("forwarded response = %q", got)
	}

	stopResponse, err := http.Post(server.URL+"/api/forwards/rule-1/stop", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST stop failed: %v", err)
	}
	defer stopResponse.Body.Close()
	var stopBody map[string]any
	if err := json.NewDecoder(stopResponse.Body).Decode(&stopBody); err != nil {
		t.Fatalf("decode stop: %v", err)
	}
	if stopBody["running"] != false {
		t.Fatalf("stop body = %#v", stopBody)
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", listenPort)), 100*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected stopped listener to reject new connections")
	}
}

func TestReorderForwards(t *testing.T) {
	configPath := writeTestConfig(t, `[
  {"id":"one","name":"One","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"127.0.0.1","targetPort":3000,"enabled":true},
  {"id":"two","name":"Two","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48002,"targetHost":"127.0.0.1","targetPort":3001,"enabled":true}
]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/reorder", "application/json", strings.NewReader(`{"ids":["two","one"]}`))
	if err != nil {
		t.Fatalf("POST reorder failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var body []map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body[0]["id"] != "two" || body[1]["id"] != "one" {
		t.Fatalf("body = %#v", body)
	}
}

func TestConfigImport(t *testing.T) {
	targetPort, stopTarget := startAPIEchoServer(t, "import")
	defer stopTarget()
	listenPort := freeAPITCPPort(t)
	handler := newTestHandler(t, "", writeSingleRuleConfig(t))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/import", "application/json", strings.NewReader(fmt.Sprintf(`{
  "mode": "replace",
  "config": {
    "version": "1",
    "exportedAt": "2026-06-06T00:00:00.000Z",
    "rules": [
      {"id":"imported","name":"Imported","protocol":"tcp","listenHost":"127.0.0.1","listenPort":%d,"targetHost":"127.0.0.1","targetPort":%d,"enabled":true}
    ]
  }
}`, listenPort, targetPort)))
	if err != nil {
		t.Fatalf("POST import failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var body struct {
		Result struct {
			Imported int `json:"imported"`
		} `json:"result"`
		Rules []map[string]any `json:"rules"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Result.Imported != 1 || body.Rules[0]["id"] != "imported" {
		t.Fatalf("body = %#v", body)
	}
	statusResponse, err := http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatalf("GET /api/status failed: %v", err)
	}
	defer statusResponse.Body.Close()
	var statuses []map[string]any
	if err := json.NewDecoder(statusResponse.Body).Decode(&statuses); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if len(statuses) != 1 || statuses[0]["ruleId"] != "imported" || statuses[0]["running"] != true {
		t.Fatalf("status after import = %#v", statuses)
	}
	if got := requestAPIForwarder(t, listenPort, "hello"); got != "import:hello\n" {
		t.Fatalf("forwarded response = %q", got)
	}
}

func TestConfigImportConflictDoesNotPartiallyApply(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", writeSingleRuleConfig(t)))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/import", "application/json", strings.NewReader(`{
  "mode": "merge",
  "config": {
    "version": "1",
    "rules": [
      {"id":"conflict","name":"Conflict","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"127.0.0.1","targetPort":3010,"enabled":false}
    ]
  }
}`))
	if err != nil {
		t.Fatalf("POST import failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnprocessableEntity)
	}

	rulesResponse, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer rulesResponse.Body.Close()
	var rules []map[string]any
	if err := json.NewDecoder(rulesResponse.Body).Decode(&rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) != 1 || rules[0]["id"] != "rule-1" {
		t.Fatalf("partial import applied: %#v", rules)
	}
}

func TestErrorStatusCodes(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", writeSingleRuleConfig(t)))
	defer server.Close()

	validationResponse, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(`{"name":""}`))
	if err != nil {
		t.Fatalf("POST invalid failed: %v", err)
	}
	defer validationResponse.Body.Close()
	if validationResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("validation status = %d", validationResponse.StatusCode)
	}

	conflictResponse, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(`{
  "name": "Conflict",
  "protocol": "tcp",
  "listenHost": "127.0.0.1",
  "listenPort": 48001,
  "targetHost": "127.0.0.1",
  "targetPort": 3001,
  "enabled": true
}`))
	if err != nil {
		t.Fatalf("POST conflict failed: %v", err)
	}
	defer conflictResponse.Body.Close()
	if conflictResponse.StatusCode != http.StatusConflict {
		t.Fatalf("conflict status = %d", conflictResponse.StatusCode)
	}

	notFoundResponse, err := http.Post(server.URL+"/api/forwards/nope/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST not found failed: %v", err)
	}
	defer notFoundResponse.Body.Close()
	if notFoundResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("not found status = %d", notFoundResponse.StatusCode)
	}

	importResponse, err := http.Post(server.URL+"/api/config/import", "application/json", strings.NewReader(`{
  "mode": "replace",
  "config": {"version":"1","rules":[{"id":"bad","name":"","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48002,"targetHost":"127.0.0.1","targetPort":3000,"enabled":true}]}
}`))
	if err != nil {
		t.Fatalf("POST invalid import failed: %v", err)
	}
	defer importResponse.Body.Close()
	if importResponse.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("import status = %d", importResponse.StatusCode)
	}
}

func newTestHandler(t *testing.T, staticDir string, configPath string) *Handler {
	t.Helper()
	if staticDir == "" {
		staticDir = filepath.Join(t.TempDir(), "missing-static")
	}
	if configPath == "missing" {
		configPath = filepath.Join(t.TempDir(), "missing-config.json")
	}
	testManager, err := manager.NewFromConfig(configPath)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	testManager.SetActivityStore(&activity.Store{})
	return NewHandler(Options{
		StaticDir: staticDir,
		Manager:   testManager,
		StartedAt: time.Now(),
		ServiceOptions: options.Options{
			Host:       "127.0.0.1",
			Port:       47831,
			ConfigPath: configPath,
			StaticDir:  staticDir,
		},
		Version: "test",
	})
}

func TestRuntimeEndpoint(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/runtime")
	if err != nil {
		t.Fatalf("GET /api/runtime failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["runtime"] != "go" {
		t.Fatalf("runtime = %v, want go", body["runtime"])
	}
	if body["name"] != "Portier" {
		t.Fatalf("name = %v, want Portier", body["name"])
	}
	if _, ok := body["version"].(string); !ok {
		t.Fatalf("version should be a string, got %T", body["version"])
	}
	if _, ok := body["configPath"].(string); !ok {
		t.Fatalf("configPath should be a string, got %T", body["configPath"])
	}
	if _, ok := body["staticDir"].(string); !ok {
		t.Fatalf("staticDir should be a string, got %T", body["staticDir"])
	}
	uptimeSeconds, ok := body["uptimeSeconds"].(float64)
	if !ok {
		t.Fatalf("uptimeSeconds should be a number, got %T", body["uptimeSeconds"])
	}
	if uptimeSeconds < 0 {
		t.Fatalf("uptimeSeconds = %v, want >= 0", uptimeSeconds)
	}
	startedAt, ok := body["startedAt"].(string)
	if !ok || startedAt == "" {
		t.Fatalf("startedAt should be a non-empty string, got %v", body["startedAt"])
	}
	if _, err := time.Parse(time.RFC3339, startedAt); err != nil {
		t.Fatalf("startedAt is not valid RFC3339: %v", err)
	}
	if _, ok := body["pid"].(float64); !ok {
		t.Fatalf("pid should be a number, got %T", body["pid"])
	}
}

func TestActivityEndpointEventsAfterRuleCreate(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "forwards.json")
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	listenPort := freeAPITCPPort(t)
	resp, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(fmt.Sprintf(`{
  "name": "Activity rule",
  "protocol": "tcp",
  "listenHost": "127.0.0.1",
  "listenPort": %d,
  "targetHost": "127.0.0.1",
  "targetPort": 3000,
  "enabled": false
}`, listenPort)))
	if err != nil {
		t.Fatalf("POST /api/forwards failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", resp.StatusCode)
	}

	actResp, err := http.Get(server.URL + "/api/activity")
	if err != nil {
		t.Fatalf("GET /api/activity failed: %v", err)
	}
	defer actResp.Body.Close()

	var body struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.NewDecoder(actResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode activity: %v", err)
	}
	if len(body.Events) == 0 {
		t.Fatal("expected at least one activity event after rule create")
	}
	var found bool
	for _, e := range body.Events {
		if e["type"] == "rule.created" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected rule.created event, got: %#v", body.Events)
	}
}

func TestActivityEndpointLimitParam(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "forwards.json")
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	for i := 0; i < 3; i++ {
		listenPort := freeAPITCPPort(t)
		resp, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(fmt.Sprintf(`{
  "name": "Rule %d",
  "protocol": "tcp",
  "listenHost": "127.0.0.1",
  "listenPort": %d,
  "targetHost": "127.0.0.1",
  "targetPort": 3000,
  "enabled": false
}`, i, listenPort)))
		if err != nil {
			t.Fatalf("POST rule %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	actResp, err := http.Get(server.URL + "/api/activity?limit=1")
	if err != nil {
		t.Fatalf("GET /api/activity?limit=1 failed: %v", err)
	}
	defer actResp.Body.Close()
	var body struct {
		Events []any `json:"events"`
	}
	if err := json.NewDecoder(actResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode activity: %v", err)
	}
	if len(body.Events) != 1 {
		t.Fatalf("expected 1 event with limit=1, got %d", len(body.Events))
	}
}

func TestActivityEndpointFilterByRuleId(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "forwards.json")
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	createRule := func(name string) string {
		listenPort := freeAPITCPPort(t)
		resp, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(fmt.Sprintf(`{
  "name": %q,
  "protocol": "tcp",
  "listenHost": "127.0.0.1",
  "listenPort": %d,
  "targetHost": "127.0.0.1",
  "targetPort": 3000,
  "enabled": false
}`, name, listenPort)))
		if err != nil {
			t.Fatalf("POST %s failed: %v", name, err)
		}
		defer resp.Body.Close()
		var b map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
			t.Fatalf("decode rule: %v", err)
		}
		return b["id"].(string)
	}

	ruleID1 := createRule("Rule A")
	_ = createRule("Rule B")

	actResp, err := http.Get(server.URL + "/api/activity?ruleId=" + ruleID1)
	if err != nil {
		t.Fatalf("GET /api/activity?ruleId=... failed: %v", err)
	}
	defer actResp.Body.Close()
	var body struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.NewDecoder(actResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode activity: %v", err)
	}
	if len(body.Events) == 0 {
		t.Fatal("expected events for ruleId filter")
	}
	for _, e := range body.Events {
		if e["ruleId"] != ruleID1 {
			t.Fatalf("expected all events with ruleId=%q, got %#v", ruleID1, e)
		}
	}
}

func TestActivityEndpointFilterByType(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "forwards.json")
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	listenPort := freeAPITCPPort(t)
	resp, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(fmt.Sprintf(`{
  "name": "Type filter rule",
  "protocol": "tcp",
  "listenHost": "127.0.0.1",
  "listenPort": %d,
  "targetHost": "127.0.0.1",
  "targetPort": 3000,
  "enabled": false
}`, listenPort)))
	if err != nil {
		t.Fatalf("POST /api/forwards failed: %v", err)
	}
	resp.Body.Close()

	actResp, err := http.Get(server.URL + "/api/activity?type=rule.created")
	if err != nil {
		t.Fatalf("GET /api/activity?type=rule.created failed: %v", err)
	}
	defer actResp.Body.Close()
	var body struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.NewDecoder(actResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode activity: %v", err)
	}
	if len(body.Events) == 0 {
		t.Fatal("expected rule.created events")
	}
	for _, e := range body.Events {
		if e["type"] != "rule.created" {
			t.Fatalf("expected type=rule.created, got %#v", e)
		}
	}
}

func TestActivityEndpointFilterBySeverity(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "forwards.json")
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	listenPort := freeAPITCPPort(t)
	resp, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(fmt.Sprintf(`{
  "name": "Severity filter rule",
  "protocol": "tcp",
  "listenHost": "127.0.0.1",
  "listenPort": %d,
  "targetHost": "127.0.0.1",
  "targetPort": 3000,
  "enabled": false
}`, listenPort)))
	if err != nil {
		t.Fatalf("POST /api/forwards failed: %v", err)
	}
	resp.Body.Close()

	actResp, err := http.Get(server.URL + "/api/activity?severity=success")
	if err != nil {
		t.Fatalf("GET /api/activity?severity=success failed: %v", err)
	}
	defer actResp.Body.Close()
	var body struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.NewDecoder(actResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode activity: %v", err)
	}
	if len(body.Events) == 0 {
		t.Fatal("expected success-severity events")
	}
	for _, e := range body.Events {
		if e["severity"] != "success" {
			t.Fatalf("expected severity=success, got %#v", e)
		}
	}
}

func TestActivityEndpointUnknownTypeFilterReturnsEmpty(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "forwards.json")
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	listenPort := freeAPITCPPort(t)
	resp, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(fmt.Sprintf(`{
  "name": "Unknown type rule",
  "protocol": "tcp",
  "listenHost": "127.0.0.1",
  "listenPort": %d,
  "targetHost": "127.0.0.1",
  "targetPort": 3000,
  "enabled": false
}`, listenPort)))
	if err != nil {
		t.Fatalf("POST /api/forwards failed: %v", err)
	}
	resp.Body.Close()

	actResp, err := http.Get(server.URL + "/api/activity?type=no.such.type")
	if err != nil {
		t.Fatalf("GET /api/activity?type=no.such.type failed: %v", err)
	}
	defer actResp.Body.Close()
	var body struct {
		Events []any `json:"events"`
	}
	if err := json.NewDecoder(actResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode activity: %v", err)
	}
	if len(body.Events) != 0 {
		t.Fatalf("expected no events for unknown type, got %d", len(body.Events))
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "forwards.json")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func writeSingleRuleConfig(t *testing.T) string {
	t.Helper()
	return writeTCPRuleConfig(t, "rule-1", 48001, 3000, true)
}

func writeTCPRuleConfig(t *testing.T, id string, listenPort int, targetPort int, enabled bool) string {
	t.Helper()
	return writeTestConfig(t, fmt.Sprintf(`[
  {
    "id": "%s",
    "name": "Local app",
    "protocol": "tcp",
    "listenHost": "127.0.0.1",
    "listenPort": %d,
    "targetHost": "127.0.0.1",
    "targetPort": %d,
    "enabled": %t
  }
]`, id, listenPort, targetPort, enabled))
}

func freeAPITCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func startAPIEchoServer(t *testing.T, prefix string) (int, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start echo server: %v", err)
	}
	stop := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							return
						}
						return
					}
					_, _ = conn.Write([]byte(prefix + ":" + line))
				}
			}(conn)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, func() {
		close(stop)
		_ = listener.Close()
	}
}

func requestAPIForwarder(t *testing.T, port int, message string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port)), time.Second)
	if err != nil {
		t.Fatalf("dial tcp forwarder: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	if _, err := conn.Write([]byte(message + "\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return line
}

func TestStartStopUDPForward(t *testing.T) {
	targetPort, stopTarget := startUDPAPIEchoServer(t, "udp")
	defer stopTarget()
	listenPort := freeAPIUDPPort(t)
	handler := newTestHandler(t, "", writeUDPRuleConfig(t, "udp-1", listenPort, targetPort, false))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	startResponse, err := http.Post(server.URL+"/api/forwards/udp-1/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST start failed: %v", err)
	}
	defer startResponse.Body.Close()
	if startResponse.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d", startResponse.StatusCode)
	}
	var startBody map[string]any
	if err := json.NewDecoder(startResponse.Body).Decode(&startBody); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	if startBody["running"] != true || startBody["startedAt"] == "" {
		t.Fatalf("start body = %#v", startBody)
	}

	// Verify UDP traffic actually flows.
	reply := sendAndReceiveUDP(t, listenPort, "hello", 500*time.Millisecond)
	if reply != "udp:hello" {
		t.Fatalf("udp reply = %q, want udp:hello", reply)
	}

	stopResponse, err := http.Post(server.URL+"/api/forwards/udp-1/stop", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST stop failed: %v", err)
	}
	defer stopResponse.Body.Close()
	var stopBody map[string]any
	if err := json.NewDecoder(stopResponse.Body).Decode(&stopBody); err != nil {
		t.Fatalf("decode stop: %v", err)
	}
	if stopBody["running"] != false {
		t.Fatalf("stop body = %#v", stopBody)
	}
}

func TestStatusShowsRunningUDPRuleWithStats(t *testing.T) {
	targetPort, stopTarget := startUDPAPIEchoServer(t, "stat")
	defer stopTarget()
	listenPort := freeAPIUDPPort(t)
	handler := newTestHandler(t, "", writeUDPRuleConfig(t, "udp-stat", listenPort, targetPort, false))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	startResponse, err := http.Post(server.URL+"/api/forwards/udp-stat/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST start failed: %v", err)
	}
	startResponse.Body.Close()
	if startResponse.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d", startResponse.StatusCode)
	}

	// Send a packet through the forwarder.
	sendAndReceiveUDP(t, listenPort, "ping", 500*time.Millisecond)

	statusResponse, err := http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatalf("GET /api/status failed: %v", err)
	}
	defer statusResponse.Body.Close()
	var statuses []map[string]any
	if err := json.NewDecoder(statusResponse.Body).Decode(&statuses); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("status count = %d", len(statuses))
	}
	s := statuses[0]
	if s["running"] != true {
		t.Fatalf("running = %v", s["running"])
	}
	if s["packetsIn"] == nil || s["packetsIn"].(float64) < 1 {
		t.Fatalf("packetsIn = %v, want >= 1", s["packetsIn"])
	}
}

func TestPatchForwardingFieldRestartsUDPForwarder(t *testing.T) {
	firstTarget, stopFirst := startUDPAPIEchoServer(t, "first")
	defer stopFirst()
	secondTarget, stopSecond := startUDPAPIEchoServer(t, "second")
	defer stopSecond()
	listenPort := freeAPIUDPPort(t)
	handler := newTestHandler(t, "", writeUDPRuleConfig(t, "udp-patch", listenPort, firstTarget, false))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	startResponse, err := http.Post(server.URL+"/api/forwards/udp-patch/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST start failed: %v", err)
	}
	startResponse.Body.Close()
	if startResponse.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d", startResponse.StatusCode)
	}

	if got := sendAndReceiveUDP(t, listenPort, "ping", 500*time.Millisecond); got != "first:ping" {
		t.Fatalf("before patch reply = %q", got)
	}

	request, err := http.NewRequest(http.MethodPatch, server.URL+"/api/forwards/udp-patch",
		strings.NewReader(fmt.Sprintf(`{"targetPort":%d}`, secondTarget)))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("PATCH failed: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d", response.StatusCode)
	}

	if got := sendAndReceiveUDP(t, listenPort, "ping", 500*time.Millisecond); got != "second:ping" {
		t.Fatalf("after patch reply = %q", got)
	}
}

func TestDeleteForwardStopsRunningUDPForwarder(t *testing.T) {
	targetPort, stopTarget := startUDPAPIEchoServer(t, "del")
	defer stopTarget()
	listenPort := freeAPIUDPPort(t)
	handler := newTestHandler(t, "", writeUDPRuleConfig(t, "udp-del", listenPort, targetPort, false))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	startResponse, err := http.Post(server.URL+"/api/forwards/udp-del/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST start failed: %v", err)
	}
	startResponse.Body.Close()
	if startResponse.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d", startResponse.StatusCode)
	}

	request, err := http.NewRequest(http.MethodDelete, server.URL+"/api/forwards/udp-del", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", response.StatusCode)
	}

	rulesResponse, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer rulesResponse.Body.Close()
	var rules []any
	if err := json.NewDecoder(rulesResponse.Body).Decode(&rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("rule should be deleted, got %d rules", len(rules))
	}
}

// --- UDP test helpers ---

func freeAPIUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral UDP port: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return port
}

func startUDPAPIEchoServer(t *testing.T, prefix string) (int, func()) {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve UDP echo addr: %v", err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		t.Fatalf("start UDP echo server: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	go func() {
		buf := make([]byte, 65536)
		for {
			n, remote, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			response := append([]byte(prefix+":"), buf[:n]...)
			_, _ = conn.WriteToUDP(response, remote)
		}
	}()
	return port, func() { conn.Close() }
}

func writeUDPRuleConfig(t *testing.T, id string, listenPort int, targetPort int, enabled bool) string {
	t.Helper()
	return writeTestConfig(t, fmt.Sprintf(`[
  {
    "id": "%s",
    "name": "UDP rule",
    "protocol": "udp",
    "listenHost": "127.0.0.1",
    "listenPort": %d,
    "targetHost": "127.0.0.1",
    "targetPort": %d,
    "enabled": %t,
    "udpMode": "bidirectional-last-client"
  }
]`, id, listenPort, targetPort, enabled))
}

// sendAndReceiveUDP sends one UDP datagram through the forwarder listen port and
// returns the echoed payload (without prefix inspection overhead).
func sendAndReceiveUDP(t *testing.T, listenPort int, payload string, timeout time.Duration) string {
	t.Helper()
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: listenPort})
	if err != nil {
		t.Fatalf("dial UDP: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatalf("write UDP: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read UDP reply: %v", err)
	}
	return string(buf[:n])
}
