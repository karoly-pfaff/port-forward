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
	"portier/service/sources/app"
	"portier/service/sources/configplan"
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

func TestPortAdvisoryInvalidPort(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	for _, query := range []string{"?port=abc&purpose=forward", "?port=0&purpose=forward", "?port=99999&purpose=forward"} {
		response, err := http.Get(server.URL + "/api/ports/advisory" + query)
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("query %s: status = %d, want 400", query, response.StatusCode)
		}
	}
}

func TestPortAdvisoryInvalidPurpose(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/ports/advisory?port=48001&purpose=proxy")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
}

func TestImportConfigAPIInvalidMode(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/import", "application/json", strings.NewReader(`{
  "mode": "upsert",
  "config": {"version":"1","rules":[]}
}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
}

func TestImportConfigAPIMissingConfigField(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/import", "application/json", strings.NewReader(`{"mode":"replace"}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
}

func TestImportConfigAPIInvalidConfigJSON(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/import", "application/json", strings.NewReader(`{"mode":"replace","config":123}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
}

func TestImportConfigAPIInvalidVersion(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/import", "application/json", strings.NewReader(`{
  "mode": "replace",
  "config": {"version":"2","rules":[]}
}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
}

func TestGetForwardByIDReturns404(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", writeSingleRuleConfig(t)))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/forwards/rule-1")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.StatusCode)
	}
}

func TestForwardUnknownSubActionReturns404(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", writeSingleRuleConfig(t)))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/rule-1/unknown-action", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.StatusCode)
	}
}

func TestReorderForwardsNullIDsReturns400(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", writeSingleRuleConfig(t)))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/reorder", "application/json", strings.NewReader(`{"ids":null}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
}

func TestUpdateForwardEmptyBodyReturns400(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", writeSingleRuleConfig(t)))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPatch, server.URL+"/api/forwards/rule-1", strings.NewReader(""))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
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
	return NewHandler(app.New(testManager, options.Options{
		Host:       "127.0.0.1",
		Port:       47831,
		ConfigPath: configPath,
		StaticDir:  staticDir,
	}, time.Now(), "test"))
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

func TestClearActivityReturns204(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/activity", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/activity failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestClearActivityEmptiesStore(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	createBody := `{"name":"Clear Test","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49920,"targetHost":"127.0.0.1","targetPort":49921,"enabled":false}`
	createResp, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/forwards failed: %v", err)
	}
	createResp.Body.Close()

	beforeResp, err := http.Get(server.URL + "/api/activity")
	if err != nil {
		t.Fatalf("GET /api/activity failed: %v", err)
	}
	defer beforeResp.Body.Close()
	var before struct {
		Events []any `json:"events"`
	}
	if err := json.NewDecoder(beforeResp.Body).Decode(&before); err != nil {
		t.Fatalf("decode before: %v", err)
	}
	if len(before.Events) == 0 {
		t.Fatal("expected events before clear")
	}

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/activity", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/activity failed: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want %d", delResp.StatusCode, http.StatusNoContent)
	}

	afterResp, err := http.Get(server.URL + "/api/activity")
	if err != nil {
		t.Fatalf("GET /api/activity after clear failed: %v", err)
	}
	defer afterResp.Body.Close()
	var after struct {
		Events []any `json:"events"`
	}
	if err := json.NewDecoder(afterResp.Body).Decode(&after); err != nil {
		t.Fatalf("decode after: %v", err)
	}
	if len(after.Events) != 0 {
		t.Fatalf("expected empty events after clear, got %d", len(after.Events))
	}
}

func TestClearActivityDoesNotAffectRules(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	createBody := `{"name":"Persist Rule","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49922,"targetHost":"127.0.0.1","targetPort":49923,"enabled":false}`
	createResp, err := http.Post(server.URL+"/api/forwards", "application/json", strings.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/forwards failed: %v", err)
	}
	createResp.Body.Close()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/activity", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/activity failed: %v", err)
	}
	delResp.Body.Close()

	rulesResp, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer rulesResp.Body.Close()
	if rulesResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/forwards status = %d, want 200", rulesResp.StatusCode)
	}
	var rules []any
	if err := json.NewDecoder(rulesResp.Body).Decode(&rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected rule to still exist after activity clear")
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

// ── Diagnose tests ─────────────────────────────────────────────────────────

func TestDiagnoseUnknownRuleReturns404(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/no-such-id/diagnose", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST diagnose failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	var body struct {
		Errors []string `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Errors) == 0 {
		t.Fatal("expected errors[] in 404 response")
	}
}

func TestDiagnoseResponseShape(t *testing.T) {
	listenPort := freeAPITCPPort(t)
	configPath := writeTestConfig(t, fmt.Sprintf(`[
  {
    "id": "shape-rule",
    "name": "Shape Rule",
    "protocol": "tcp",
    "listenHost": "127.0.0.1",
    "listenPort": %d,
    "targetHost": "127.0.0.1",
    "targetPort": 9999,
    "enabled": false
  }
]`, listenPort))
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/shape-rule/diagnose", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST diagnose failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	requiredFields := []string{"ruleId", "ruleName", "protocol", "summary", "checks", "diagnosedAt"}
	for _, f := range requiredFields {
		if _, ok := body[f]; !ok {
			t.Fatalf("missing field: %s", f)
		}
	}
	if body["ruleId"] != "shape-rule" {
		t.Fatalf("ruleId = %v, want shape-rule", body["ruleId"])
	}
	if body["protocol"] != "tcp" {
		t.Fatalf("protocol = %v, want tcp", body["protocol"])
	}

	summary, ok := body["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary is not an object: %T", body["summary"])
	}
	if _, ok := summary["status"].(string); !ok {
		t.Fatal("summary.status is not a string")
	}
	if _, ok := summary["message"].(string); !ok {
		t.Fatal("summary.message is not a string")
	}

	checks, ok := body["checks"].([]any)
	if !ok || len(checks) == 0 {
		t.Fatal("checks must be a non-empty array")
	}
	for _, raw := range checks {
		check, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("check is not an object: %T", raw)
		}
		for _, f := range []string{"id", "label", "status", "message"} {
			if _, ok := check[f].(string); !ok {
				t.Fatalf("check.%s is not a string: %v", f, check[f])
			}
		}
	}
}

func TestDiagnoseTCPReachableTargetPassesConnect(t *testing.T) {
	targetPort, stopTarget := startAPIEchoServer(t, "diag-reach")
	defer stopTarget()
	listenPort := freeAPITCPPort(t)
	configPath := writeTestConfig(t, fmt.Sprintf(`[
  {
    "id": "diag-reach",
    "name": "Diag Reach",
    "protocol": "tcp",
    "listenHost": "127.0.0.1",
    "listenPort": %d,
    "targetHost": "127.0.0.1",
    "targetPort": %d,
    "enabled": false
  }
]`, listenPort, targetPort))
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/diag-reach/diagnose", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST diagnose failed: %v", err)
	}
	defer response.Body.Close()

	var body struct {
		Checks []map[string]any `json:"checks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var connectCheck map[string]any
	for _, c := range body.Checks {
		if c["id"] == "target-connect" {
			connectCheck = c
		}
	}
	if connectCheck == nil {
		t.Fatal("expected target-connect check")
	}
	if connectCheck["status"] != "pass" {
		t.Fatalf("target-connect status = %v, want pass", connectCheck["status"])
	}
}

func TestDiagnoseTCPUnreachableTargetFailsConnect(t *testing.T) {
	listenPort := freeAPITCPPort(t)
	unreachablePort := freeAPITCPPort(t) // nothing listening
	configPath := writeTestConfig(t, fmt.Sprintf(`[
  {
    "id": "diag-unreach",
    "name": "Diag Unreach",
    "protocol": "tcp",
    "listenHost": "127.0.0.1",
    "listenPort": %d,
    "targetHost": "127.0.0.1",
    "targetPort": %d,
    "enabled": false
  }
]`, listenPort, unreachablePort))
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/diag-unreach/diagnose", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST diagnose failed: %v", err)
	}
	defer response.Body.Close()

	var body struct {
		Checks []map[string]any `json:"checks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var connectCheck map[string]any
	for _, c := range body.Checks {
		if c["id"] == "target-connect" {
			connectCheck = c
		}
	}
	if connectCheck == nil {
		t.Fatal("expected target-connect check")
	}
	if connectCheck["status"] != "fail" {
		t.Fatalf("target-connect status = %v, want fail", connectCheck["status"])
	}
}

func TestDiagnoseUDPSkipsTargetConnect(t *testing.T) {
	listenPort := freeAPIUDPPort(t)
	configPath := writeTestConfig(t, fmt.Sprintf(`[
  {
    "id": "diag-udp",
    "name": "Diag UDP",
    "protocol": "udp",
    "listenHost": "127.0.0.1",
    "listenPort": %d,
    "targetHost": "127.0.0.1",
    "targetPort": 9999,
    "enabled": false,
    "udpMode": "one-way"
  }
]`, listenPort))
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/diag-udp/diagnose", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST diagnose failed: %v", err)
	}
	defer response.Body.Close()

	var body struct {
		Checks []map[string]any `json:"checks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var connectCheck, udpModeCheck map[string]any
	for _, c := range body.Checks {
		switch c["id"] {
		case "target-connect":
			connectCheck = c
		case "udp-mode":
			udpModeCheck = c
		}
	}
	if connectCheck == nil {
		t.Fatal("expected target-connect check")
	}
	if connectCheck["status"] != "skip" {
		t.Fatalf("target-connect status = %v, want skip", connectCheck["status"])
	}
	if udpModeCheck == nil {
		t.Fatal("expected udp-mode check")
	}
	if udpModeCheck["status"] != "pass" {
		t.Fatalf("udp-mode status = %v, want pass (one-way)", udpModeCheck["status"])
	}
}

func TestDiagnoseUDPBidirectionalLastClientWarn(t *testing.T) {
	listenPort := freeAPIUDPPort(t)
	configPath := writeTestConfig(t, fmt.Sprintf(`[
  {
    "id": "diag-udp-bidi",
    "name": "Diag UDP Bidi",
    "protocol": "udp",
    "listenHost": "127.0.0.1",
    "listenPort": %d,
    "targetHost": "127.0.0.1",
    "targetPort": 9999,
    "enabled": false,
    "udpMode": "bidirectional-last-client"
  }
]`, listenPort))
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/diag-udp-bidi/diagnose", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST diagnose failed: %v", err)
	}
	defer response.Body.Close()

	var body struct {
		Checks []map[string]any `json:"checks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var udpModeCheck map[string]any
	for _, c := range body.Checks {
		if c["id"] == "udp-mode" {
			udpModeCheck = c
		}
	}
	if udpModeCheck == nil {
		t.Fatal("expected udp-mode check")
	}
	if udpModeCheck["status"] != "warn" {
		t.Fatalf("udp-mode status = %v, want warn", udpModeCheck["status"])
	}
}

func TestDiagnoseLANExposureWarns(t *testing.T) {
	listenPort := freeAPITCPPort(t)
	configPath := writeTestConfig(t, fmt.Sprintf(`[
  {
    "id": "diag-lan",
    "name": "Diag LAN",
    "protocol": "tcp",
    "listenHost": "0.0.0.0",
    "listenPort": %d,
    "targetHost": "127.0.0.1",
    "targetPort": 9999,
    "enabled": false
  }
]`, listenPort))
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/diag-lan/diagnose", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST diagnose failed: %v", err)
	}
	defer response.Body.Close()

	var body struct {
		Checks []map[string]any `json:"checks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var lanCheck map[string]any
	for _, c := range body.Checks {
		if c["id"] == "lan-exposure" {
			lanCheck = c
		}
	}
	if lanCheck == nil {
		t.Fatal("expected lan-exposure check")
	}
	if lanCheck["status"] != "warn" {
		t.Fatalf("lan-exposure status = %v, want warn", lanCheck["status"])
	}
}

func TestDiagnoseRunningRuleDoesNotFailListenBind(t *testing.T) {
	targetPort, stopTarget := startAPIEchoServer(t, "diag-run")
	defer stopTarget()
	listenPort := freeAPITCPPort(t)
	handler := newTestHandler(t, "", writeTCPRuleConfig(t, "diag-run", listenPort, targetPort, false))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	// Start the rule so it owns the listen socket.
	startResp, err := http.Post(server.URL+"/api/forwards/diag-run/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST start failed: %v", err)
	}
	startResp.Body.Close()
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d", startResp.StatusCode)
	}

	response, err := http.Post(server.URL+"/api/forwards/diag-run/diagnose", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST diagnose failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("diagnose status = %d, want 200", response.StatusCode)
	}

	var body struct {
		Checks []map[string]any `json:"checks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var bindCheck map[string]any
	for _, c := range body.Checks {
		if c["id"] == "listen-bind" {
			bindCheck = c
		}
	}
	if bindCheck == nil {
		t.Fatal("expected listen-bind check")
	}
	if bindCheck["status"] == "fail" {
		t.Fatalf("listen-bind must not be fail when rule is running (got %v)", bindCheck["status"])
	}
}

// ── GET /api/connections tests ─────────────────────────────────────────────

func TestConnectionsEmptyState(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/connections")
	if err != nil {
		t.Fatalf("GET /api/connections failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		GeneratedAt    string `json:"generatedAt"`
		TCPConnections []any  `json:"tcpConnections"`
		UDPSessions    []any  `json:"udpSessions"`
		RuleSummaries  []any  `json:"ruleSummaries"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.GeneratedAt == "" {
		t.Fatal("generatedAt must not be empty")
	}
	if _, err := time.Parse("2006-01-02T15:04:05.000Z", body.GeneratedAt); err != nil {
		t.Fatalf("generatedAt not parseable as ISO timestamp: %v", err)
	}
	if body.TCPConnections == nil {
		t.Fatal("tcpConnections must be an array, not null")
	}
	if body.UDPSessions == nil {
		t.Fatal("udpSessions must be an array, not null")
	}
	if body.RuleSummaries == nil {
		t.Fatal("ruleSummaries must be an array, not null")
	}
	if len(body.TCPConnections) != 0 || len(body.UDPSessions) != 0 || len(body.RuleSummaries) != 0 {
		t.Fatalf("expected all empty arrays; got tcp=%d udp=%d summaries=%d",
			len(body.TCPConnections), len(body.UDPSessions), len(body.RuleSummaries))
	}
}

func TestConnectionsIdleRulesHaveZeroSummaries(t *testing.T) {
	configPath := writeTestConfig(t, `[
  {
    "id": "idle-tcp",
    "name": "Idle TCP",
    "protocol": "tcp",
    "listenHost": "127.0.0.1",
    "listenPort": 48111,
    "targetHost": "127.0.0.1",
    "targetPort": 9999,
    "enabled": false
  },
  {
    "id": "idle-udp",
    "name": "Idle UDP",
    "protocol": "udp",
    "listenHost": "127.0.0.1",
    "listenPort": 48112,
    "targetHost": "127.0.0.1",
    "targetPort": 9998,
    "enabled": false,
    "udpMode": "one-way"
  }
]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/connections")
	if err != nil {
		t.Fatalf("GET /api/connections failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		RuleSummaries  []map[string]any `json:"ruleSummaries"`
		TCPConnections []any            `json:"tcpConnections"`
		UDPSessions    []any            `json:"udpSessions"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.RuleSummaries) != 2 {
		t.Fatalf("expected 2 rule summaries, got %d", len(body.RuleSummaries))
	}
	if body.TCPConnections == nil || len(body.TCPConnections) != 0 {
		t.Fatal("tcpConnections must be an empty array")
	}
	if body.UDPSessions == nil || len(body.UDPSessions) != 0 {
		t.Fatal("udpSessions must be an empty array")
	}

	for _, summary := range body.RuleSummaries {
		for _, field := range []string{"ruleId", "ruleName", "protocol", "activeTcpConnections", "activeUdpSessions", "bytesIn", "bytesOut", "packetsIn", "packetsOut", "lastTrafficAt"} {
			if _, ok := summary[field]; !ok {
				t.Fatalf("summary missing field %q in %v", field, summary)
			}
		}
		if summary["lastTrafficAt"] != nil {
			t.Fatalf("lastTrafficAt should be null for idle rule, got %v", summary["lastTrafficAt"])
		}
		for _, f := range []string{"activeTcpConnections", "activeUdpSessions", "bytesIn", "bytesOut", "packetsIn", "packetsOut"} {
			if v, ok := summary[f].(float64); !ok || v != 0 {
				t.Fatalf("expected %s=0, got %v", f, summary[f])
			}
		}
	}

	var tcpSummary, udpSummary map[string]any
	for _, s := range body.RuleSummaries {
		if s["ruleId"] == "idle-tcp" {
			tcpSummary = s
		}
		if s["ruleId"] == "idle-udp" {
			udpSummary = s
		}
	}
	if tcpSummary == nil || udpSummary == nil {
		t.Fatal("expected summaries for idle-tcp and idle-udp")
	}
	if tcpSummary["protocol"] != "tcp" {
		t.Fatalf("idle-tcp protocol = %v, want tcp", tcpSummary["protocol"])
	}
	if udpSummary["protocol"] != "udp" {
		t.Fatalf("idle-udp protocol = %v, want udp", udpSummary["protocol"])
	}
	if tcpSummary["ruleName"] != "Idle TCP" {
		t.Fatalf("idle-tcp ruleName = %v", tcpSummary["ruleName"])
	}
}

func TestConnectionsResponseJSONFieldNames(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/connections")
	if err != nil {
		t.Fatalf("GET /api/connections failed: %v", err)
	}
	defer response.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, field := range []string{"generatedAt", "tcpConnections", "udpSessions", "ruleSummaries"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("response missing camelCase field %q", field)
		}
	}
	for _, bad := range []string{"GeneratedAt", "TcpConnections", "UdpSessions", "RuleSummaries"} {
		if _, ok := body[bad]; ok {
			t.Fatalf("response has PascalCase field %q; want camelCase", bad)
		}
	}
}

func TestConnectionsActiveTCPConnectionAppearsInResponse(t *testing.T) {
	targetPort, stopTarget := startAPIEchoServer(t, "conn")
	defer stopTarget()
	listenPort := freeAPITCPPort(t)
	handler := newTestHandler(t, "", writeTCPRuleConfig(t, "live-tcp", listenPort, targetPort, true))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	startResp, err := http.Post(server.URL+"/api/forwards/live-tcp/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("start rule: %v", err)
	}
	startResp.Body.Close()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", listenPort), 3*time.Second)
	if err != nil {
		t.Fatalf("connect through forwarder: %v", err)
	}
	defer conn.Close()

	type connectionsBody struct {
		TCPConnections []map[string]any `json:"tcpConnections"`
		RuleSummaries  []map[string]any `json:"ruleSummaries"`
	}
	var result connectionsBody
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r, err := http.Get(server.URL + "/api/connections")
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		_ = json.NewDecoder(r.Body).Decode(&result)
		r.Body.Close()
		if len(result.TCPConnections) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(result.TCPConnections) == 0 {
		t.Fatal("expected at least one TCP connection in /api/connections response")
	}
	c := result.TCPConnections[0]
	if c["ruleId"] != "live-tcp" {
		t.Fatalf("tcpConnections[0].ruleId = %v, want live-tcp", c["ruleId"])
	}
	if c["protocol"] != "tcp" {
		t.Fatalf("tcpConnections[0].protocol = %v, want tcp", c["protocol"])
	}
	if c["status"] != "active" {
		t.Fatalf("tcpConnections[0].status = %v, want active", c["status"])
	}

	var summary map[string]any
	for _, s := range result.RuleSummaries {
		if s["ruleId"] == "live-tcp" {
			summary = s
			break
		}
	}
	if summary == nil {
		t.Fatal("expected summary for live-tcp in ruleSummaries")
	}
	if v, ok := summary["activeTcpConnections"].(float64); !ok || v < 1 {
		t.Fatalf("activeTcpConnections = %v, want >= 1", summary["activeTcpConnections"])
	}
	if summary["lastTrafficAt"] == nil {
		t.Fatal("lastTrafficAt must not be null when connection is active")
	}
}

func TestConnectionsActiveUDPSessionAppearsInResponse(t *testing.T) {
	targetPort, stopTarget := startUDPAPIEchoServer(t, "udpconn")
	defer stopTarget()
	listenPort := freeAPIUDPPort(t)
	handler := newTestHandler(t, "", writeUDPRuleConfig(t, "live-udp", listenPort, targetPort, true))
	defer handler.manager.StopAll()
	server := httptest.NewServer(handler)
	defer server.Close()

	startResp, err := http.Post(server.URL+"/api/forwards/live-udp/start", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("start rule: %v", err)
	}
	startResp.Body.Close()

	clientAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve client addr: %v", err)
	}
	clientConn, err := net.ListenUDP("udp4", clientAddr)
	if err != nil {
		t.Fatalf("listen client udp: %v", err)
	}
	defer clientConn.Close()

	targetAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: listenPort}

	type sessionsBody struct {
		UDPSessions   []map[string]any `json:"udpSessions"`
		RuleSummaries []map[string]any `json:"ruleSummaries"`
	}
	var result sessionsBody
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, _ = clientConn.WriteToUDP([]byte("ping"), targetAddr)
		r, err := http.Get(server.URL + "/api/connections")
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		_ = json.NewDecoder(r.Body).Decode(&result)
		r.Body.Close()
		if len(result.UDPSessions) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(result.UDPSessions) == 0 {
		t.Fatal("expected at least one UDP session in /api/connections response")
	}
	s := result.UDPSessions[0]
	if s["ruleId"] != "live-udp" {
		t.Fatalf("udpSessions[0].ruleId = %v, want live-udp", s["ruleId"])
	}
	if s["protocol"] != "udp" {
		t.Fatalf("udpSessions[0].protocol = %v, want udp", s["protocol"])
	}

	var summary map[string]any
	for _, sm := range result.RuleSummaries {
		if sm["ruleId"] == "live-udp" {
			summary = sm
			break
		}
	}
	if summary == nil {
		t.Fatal("expected summary for live-udp in ruleSummaries")
	}
	if v, ok := summary["activeUdpSessions"].(float64); !ok || v < 1 {
		t.Fatalf("activeUdpSessions = %v, want >= 1", summary["activeUdpSessions"])
	}
	if v, ok := summary["packetsIn"].(float64); !ok || v < 1 {
		t.Fatalf("packetsIn = %v, want >= 1", summary["packetsIn"])
	}
	if summary["lastTrafficAt"] == nil {
		t.Fatal("lastTrafficAt must not be null when session is active")
	}
}

// ── POST /api/config/plan tests ────────────────────────────────────────────

func TestConfigPlanEmptyDesiredNoDrift(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/plan", "application/json", strings.NewReader(`{"desired":[]}`))
	if err != nil {
		t.Fatalf("POST /api/config/plan failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, field := range []string{"generatedAt", "mode", "summary", "operations", "errors", "warnings"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("missing required field: %s", field)
		}
	}
	if body["mode"] != "plan" {
		t.Fatalf("mode = %v, want plan", body["mode"])
	}
	summary, ok := body["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary is not an object")
	}
	if summary["hasDrift"] != false {
		t.Fatalf("hasDrift = %v, want false", summary["hasDrift"])
	}
	if summary["hasErrors"] != false {
		t.Fatalf("hasErrors = %v, want false", summary["hasErrors"])
	}
	if ops, ok := body["operations"].([]any); !ok || len(ops) != 0 {
		t.Fatalf("operations must be empty array, got %v", body["operations"])
	}
}

func TestConfigPlanMissingDesiredReturns400(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/plan", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST /api/config/plan failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for missing desired", response.StatusCode)
	}
	var body struct {
		Errors []string `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Errors) == 0 {
		t.Fatal("expected errors[] in 400 response")
	}
}

func TestConfigPlanEmptyBodyReturns400(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/plan", "application/json", strings.NewReader(""))
	if err != nil {
		t.Fatalf("POST /api/config/plan failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for empty body", response.StatusCode)
	}
}

func TestConfigPlanMalformedBodyReturns400(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/plan", "application/json", strings.NewReader("not-json"))
	if err != nil {
		t.Fatalf("POST /api/config/plan failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for malformed body", response.StatusCode)
	}
}

func TestConfigPlanAddOperationFromDesired(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/plan", "application/json", strings.NewReader(`{
		"desired": [{"name":"New Rule","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49901,"targetHost":"127.0.0.1","targetPort":49801,"enabled":false}]
	}`))
	if err != nil {
		t.Fatalf("POST /api/config/plan failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	summary := body["summary"].(map[string]any)
	if summary["add"] != float64(1) {
		t.Fatalf("add = %v, want 1", summary["add"])
	}
	if summary["hasDrift"] != true {
		t.Fatal("hasDrift must be true when there is an add")
	}

	ops := body["operations"].([]any)
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	op := ops[0].(map[string]any)
	if op["type"] != "add" {
		t.Fatalf("op type = %v, want add", op["type"])
	}
}

func TestConfigPlanInvalidDesiredReturnsStructuredErrors(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/plan", "application/json", strings.NewReader(`{"desired":[{"name":""}]}`))
	if err != nil {
		t.Fatalf("POST /api/config/plan failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 with structured errors for invalid desired", response.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	summary := body["summary"].(map[string]any)
	if summary["hasErrors"] != true {
		t.Fatal("hasErrors must be true for invalid desired rule")
	}
	errs, ok := body["errors"].([]any)
	if !ok || len(errs) == 0 {
		t.Fatal("expected non-empty errors array")
	}
	errObj := errs[0].(map[string]any)
	if _, ok := errObj["code"].(string); !ok {
		t.Fatal("error must have a code string field")
	}
}

func TestConfigPlanDoesNotMutateCurrentConfig(t *testing.T) {
	configPath := writeTestConfig(t, `[
	  {"id":"kept","name":"Kept Rule","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}
	]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	// Plan that would remove the rule
	resp, err := http.Post(server.URL+"/api/config/plan", "application/json", strings.NewReader(`{"desired":[]}`))
	if err != nil {
		t.Fatalf("POST /api/config/plan failed: %v", err)
	}
	resp.Body.Close()

	// Verify the rule still exists
	rulesResp, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer rulesResp.Body.Close()
	var rules []any
	if err := json.NewDecoder(rulesResp.Body).Decode(&rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("plan mutated config: expected 1 rule, got %d", len(rules))
	}
}

func TestConfigPlanResponseShape(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/plan", "application/json", strings.NewReader(`{"desired":[]}`))
	if err != nil {
		t.Fatalf("POST /api/config/plan failed: %v", err)
	}
	defer response.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// generatedAt is a parseable ISO timestamp
	generatedAt, ok := body["generatedAt"].(string)
	if !ok || generatedAt == "" {
		t.Fatalf("generatedAt must be a non-empty string, got %v", body["generatedAt"])
	}

	// summary has all required fields
	summary, ok := body["summary"].(map[string]any)
	if !ok {
		t.Fatal("summary must be an object")
	}
	for _, field := range []string{"add", "update", "remove", "unchanged", "destructive", "hasDrift", "hasErrors"} {
		if _, ok := summary[field]; !ok {
			t.Fatalf("summary missing field: %s", field)
		}
	}

	// operations, errors, warnings are arrays
	if _, ok := body["operations"].([]any); !ok {
		t.Fatalf("operations must be an array, got %T", body["operations"])
	}
	if _, ok := body["errors"].([]any); !ok {
		t.Fatalf("errors must be an array, got %T", body["errors"])
	}
	if _, ok := body["warnings"].([]any); !ok {
		t.Fatalf("warnings must be an array, got %T", body["warnings"])
	}
}

func TestConfigPlanRemoveOperation(t *testing.T) {
	configPath := writeTestConfig(t, `[
	  {"id":"r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48001,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}
	]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/plan", "application/json", strings.NewReader(`{"desired":[]}`))
	if err != nil {
		t.Fatalf("POST /api/config/plan failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	summary := body["summary"].(map[string]any)
	if summary["remove"] != float64(1) {
		t.Fatalf("remove = %v, want 1", summary["remove"])
	}
	ops := body["operations"].([]any)
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	op := ops[0].(map[string]any)
	if op["type"] != "remove" {
		t.Fatalf("op type = %v, want remove", op["type"])
	}
	if op["destructive"] != true {
		t.Fatal("remove operation must be destructive")
	}

	// REMOVE_EXISTING warning
	warnings := body["warnings"].([]any)
	if len(warnings) == 0 {
		t.Fatal("expected REMOVE_EXISTING warning")
	}
	w := warnings[0].(map[string]any)
	if w["code"] != "REMOVE_EXISTING" {
		t.Fatalf("warning code = %v, want REMOVE_EXISTING", w["code"])
	}
}

// ── POST /api/config/apply tests ───────────────────────────────────────────

func TestConfigApplyMissingDesiredReturns400(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{"yes":true}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
	var body struct {
		Errors []string `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Errors) == 0 {
		t.Fatal("expected errors[] in 400 response")
	}
}

func TestConfigApplyPlanErrorsReturnsOkFalse(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	// desired rule is missing required fields — plan will have errors
	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{"desired":[{"name":""}],"yes":true}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 with ok:false for plan errors", response.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != false {
		t.Fatalf("ok = %v, want false", body["ok"])
	}
	plan, ok := body["plan"].(map[string]any)
	if !ok {
		t.Fatal("plan must be an object")
	}
	summary := plan["summary"].(map[string]any)
	if summary["hasErrors"] != true {
		t.Fatal("plan.summary.hasErrors must be true")
	}
}

func TestConfigApplyPlanErrorsDoNotMutate(t *testing.T) {
	configPath := writeTestConfig(t, `[
	  {"id":"ap-kept","name":"Kept","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49310,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}
	]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{"desired":[{"name":""}],"yes":true}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	resp.Body.Close()

	rulesResp, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer rulesResp.Body.Close()
	var rules []any
	if err := json.NewDecoder(rulesResp.Body).Decode(&rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("plan-error apply mutated config: expected 1 rule, got %d", len(rules))
	}
}

func TestApplyImportErrorResponse(t *testing.T) {
	// Covers the construction of the config-apply import-error response
	// (Resilience-C defensive branch). The handler branch that calls this is
	// structurally unreachable (reachable import errors are pre-blocked by the
	// plan engine), so the construction is verified directly here; the TS twin is
	// covered end-to-end via a fake-manager subclass in api.test.ts.
	plan := configplan.Response{Summary: configplan.Summary{HasErrors: false}}
	zero := map[string]int{"add": 0, "update": 0, "remove": 0, "unchanged": 2}
	resp := applyImportErrorResponse(plan, []string{"forced import error"}, "2026-01-01T00:00:00Z", zero)

	if resp.Ok {
		t.Fatal("Ok must be false when import reports errors")
	}
	if !resp.Plan.Summary.HasErrors {
		t.Fatal("Plan.Summary.HasErrors must be true")
	}
	found := false
	for _, e := range resp.Plan.Errors {
		if e.Code == "IMPORT_ERROR" && e.Message == "forced import error" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Plan.Errors must include the IMPORT_ERROR entry; got %+v", resp.Plan.Errors)
	}
	if resp.Applied["add"] != 0 || resp.Applied["unchanged"] != 2 {
		t.Fatalf("Applied must be the passed zero counts; got %+v", resp.Applied)
	}
}

func TestConfigApplyDuplicateDesiredBindingPreBlocked(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	// Two desired rules share protocol+listenHost+listenPort — a plan-level error
	// (detectDuplicateKeys → Summary.HasErrors). Apply must stop before
	// ImportConfig (parity with the TS api.test.ts duplicate-binding apply test).
	desired := `{"desired":[` +
		`{"name":"Dup A","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49340,"targetHost":"127.0.0.1","targetPort":49341,"enabled":false},` +
		`{"name":"Dup B","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49340,"targetHost":"10.0.0.9","targetPort":49342,"enabled":false}` +
		`],"yes":true}`
	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(desired))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 with ok:false", response.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != false {
		t.Fatalf("ok = %v, want false for duplicate desired bindings", body["ok"])
	}
	summary := body["plan"].(map[string]any)["summary"].(map[string]any)
	if summary["hasErrors"] != true {
		t.Fatal("plan.summary.hasErrors must be true for duplicate desired bindings")
	}
	if applied := body["applied"].(map[string]any); applied["add"].(float64) != 0 {
		t.Fatalf("applied.add = %v, want 0 (pre-blocked before import)", applied["add"])
	}

	// Nothing was created — apply stopped before ImportConfig.
	rulesResp, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer rulesResp.Body.Close()
	var rules []any
	if err := json.NewDecoder(rulesResp.Body).Decode(&rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("duplicate-binding apply created rules: got %d", len(rules))
	}
}

func TestConfigApplyDestructiveWithoutYesReturns400(t *testing.T) {
	configPath := writeTestConfig(t, `[
	  {"id":"ap-r1","name":"App","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49311,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}
	]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	// desired=[] would remove the existing rule (destructive), but yes is not set
	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{"desired":[],"yes":false}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for destructive without yes", response.StatusCode)
	}
	var body struct {
		Errors []string `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Errors) == 0 {
		t.Fatal("expected errors[] in 400 response")
	}
}

func TestConfigApplyDryRunReturnsOkTrueWithNoMutation(t *testing.T) {
	configPath := writeTestConfig(t, `[
	  {"id":"ap-dry","name":"Dry Rule","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49312,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}
	]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	// dryRun with empty desired would remove the rule — but must not mutate
	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{"desired":[],"yes":true,"dryRun":true}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
	if body["dryRun"] != true {
		t.Fatalf("dryRun = %v, want true", body["dryRun"])
	}

	// rule must still exist
	rulesResp, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer rulesResp.Body.Close()
	var rules []any
	if err := json.NewDecoder(rulesResp.Body).Decode(&rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("dryRun mutated config: expected 1 rule, got %d", len(rules))
	}
}

func TestConfigApplyAddRuleWithYes(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{
		"desired":[{"name":"New Rule","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49313,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}],
		"yes":true
	}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}

	rulesResp, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer rulesResp.Body.Close()
	var rules []map[string]any
	if err := json.NewDecoder(rulesResp.Body).Decode(&rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule after apply add, got %d", len(rules))
	}
	if rules[0]["name"] != "New Rule" {
		t.Fatalf("rule name = %v, want New Rule", rules[0]["name"])
	}
}

func TestConfigApplyNoDriftSkipsImport(t *testing.T) {
	configPath := writeTestConfig(t, `[
	  {"id":"ap-nd","name":"No Drift","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49314,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}
	]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	// desired matches current exactly by key identity — no drift
	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{
		"desired":[{"id":"ap-nd","name":"No Drift","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49314,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}],
		"yes":false
	}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
	plan := body["plan"].(map[string]any)
	summary := plan["summary"].(map[string]any)
	if summary["hasDrift"] != false {
		t.Fatal("hasDrift must be false for no-drift apply")
	}
}

func TestConfigApplyResponseIncludesPlan(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{"desired":[],"yes":false}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, field := range []string{"ok", "dryRun", "appliedAt", "plan", "applied"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("response missing field: %s", field)
		}
	}

	appliedAt, ok := body["appliedAt"].(string)
	if !ok || appliedAt == "" {
		t.Fatalf("appliedAt must be a non-empty string, got %v", body["appliedAt"])
	}

	plan, ok := body["plan"].(map[string]any)
	if !ok {
		t.Fatal("plan must be an object")
	}
	for _, field := range []string{"generatedAt", "mode", "summary", "operations", "errors", "warnings"} {
		if _, ok := plan[field]; !ok {
			t.Fatalf("plan missing field: %s", field)
		}
	}

	applied, ok := body["applied"].(map[string]any)
	if !ok {
		t.Fatal("applied must be an object")
	}
	for _, field := range []string{"add", "update", "remove", "unchanged"} {
		if _, ok := applied[field]; !ok {
			t.Fatalf("applied missing field: %s", field)
		}
	}
}

func TestConfigApplyRemoveWithYes(t *testing.T) {
	configPath := writeTestConfig(t, `[
	  {"id":"ap-rm","name":"Remove Me","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49315,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}
	]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{"desired":[],"yes":true}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}

	rulesResp, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer rulesResp.Body.Close()
	var rules []any
	if err := json.NewDecoder(rulesResp.Body).Decode(&rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules after apply remove, got %d", len(rules))
	}
}

func TestConfigApplyPreservesRuleIDs(t *testing.T) {
	configPath := writeTestConfig(t, `[
	  {"id":"ap-preserve","name":"Preserve Me","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49316,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}
	]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	// Desired matches by name/key but no explicit id — the existing rule's ID must be preserved
	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{
		"desired":[{"name":"Preserve Me","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49316,"targetHost":"127.0.0.1","targetPort":3001,"enabled":false}],
		"yes":true
	}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}

	rulesResp, err := http.Get(server.URL + "/api/forwards")
	if err != nil {
		t.Fatalf("GET /api/forwards failed: %v", err)
	}
	defer rulesResp.Body.Close()
	var rules []map[string]any
	if err := json.NewDecoder(rulesResp.Body).Decode(&rules); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule after apply update, got %d", len(rules))
	}
	if rules[0]["id"] != "ap-preserve" {
		t.Fatalf("rule id = %v, want ap-preserve (ID not preserved)", rules[0]["id"])
	}
	if rules[0]["targetPort"] != float64(3001) {
		t.Fatalf("targetPort = %v, want 3001 (update not applied)", rules[0]["targetPort"])
	}
}

func TestConfigApplyAppliedCountsMatchPlanSummary(t *testing.T) {
	configPath := writeTestConfig(t, `[
	  {"id":"ap-cnt-rm","name":"Remove Me","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49317,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}
	]`)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	// add one new rule, remove the existing one (destructive)
	response, err := http.Post(server.URL+"/api/config/apply", "application/json", strings.NewReader(`{
		"desired":[{"name":"Added","protocol":"tcp","listenHost":"127.0.0.1","listenPort":49318,"targetHost":"127.0.0.1","targetPort":3000,"enabled":false}],
		"yes":true
	}`))
	if err != nil {
		t.Fatalf("POST /api/config/apply failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	plan := body["plan"].(map[string]any)
	summary := plan["summary"].(map[string]any)
	applied := body["applied"].(map[string]any)

	if applied["add"] != summary["add"] {
		t.Fatalf("applied.add = %v, want %v (from plan.summary.add)", applied["add"], summary["add"])
	}
	if applied["update"] != summary["update"] {
		t.Fatalf("applied.update = %v, want %v", applied["update"], summary["update"])
	}
	if applied["remove"] != summary["remove"] {
		t.Fatalf("applied.remove = %v, want %v", applied["remove"], summary["remove"])
	}
	if applied["unchanged"] != summary["unchanged"] {
		t.Fatalf("applied.unchanged = %v, want %v", applied["unchanged"], summary["unchanged"])
	}
}

// ── Normalization unit tests ─────────────────────────────────────────────────

func TestInternalNormalizePlatform(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"windows", "windows"},
		{"darwin", "macos"},
		{"linux", "linux"},
		{"freebsd", "unknown"},
		{"", "unknown"},
	}
	for _, tc := range cases {
		got := internalNormalizePlatform(tc.input)
		if got != tc.want {
			t.Errorf("internalNormalizePlatform(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestInternalNormalizeArch(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"amd64", "x64"},
		{"arm64", "arm64"},
		{"386", "unknown"},
		{"mips", "unknown"},
		{"", "unknown"},
	}
	for _, tc := range cases {
		got := internalNormalizeArch(tc.input)
		if got != tc.want {
			t.Errorf("internalNormalizeArch(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ── ServeHTTP non-API path without static dir ────────────────────────────────

func TestServeHTTPNonAPIPathNoStaticDir(t *testing.T) {
	// newTestHandler uses a non-existent static dir, so staticAvailable=false.
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for non-API path without static dir", response.StatusCode)
	}
}

// ── updateForward missing and validation error paths ─────────────────────────

func TestUpdateForwardEmptyBody(t *testing.T) {
	configPath := writeSingleRuleConfig(t)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/forwards/rule-1", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/forwards/rule-1 failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for empty body", response.StatusCode)
	}
}

func TestUpdateForwardInvalidPatch(t *testing.T) {
	configPath := writeSingleRuleConfig(t)
	server := httptest.NewServer(newTestHandler(t, "", configPath))
	defer server.Close()

	// listenPort:0 fails port range validation
	req, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/forwards/rule-1", strings.NewReader(`{"listenPort":0}`))
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/forwards/rule-1 failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for invalid patch", response.StatusCode)
	}
	var body map[string][]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body["errors"]) == 0 {
		t.Fatal("expected validation errors in response")
	}
}

// ── deleteForward not-found path ─────────────────────────────────────────────

func TestDeleteForwardNotFound(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/forwards/no-such-rule", nil)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/forwards/no-such-rule failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for delete of missing rule", response.StatusCode)
	}
}

// ── stopForward not-found path ───────────────────────────────────────────────

func TestStopForwardNotFound(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/no-such-rule/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/forwards/no-such-rule/stop failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for stop of missing rule", response.StatusCode)
	}
}

// ── reorderForwards error paths ───────────────────────────────────────────────

func TestReorderForwardsNullIDsField(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	// Body is valid JSON but has no "ids" key → should return 400
	response, err := http.Post(server.URL+"/api/forwards/reorder", "application/json",
		strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST /api/forwards/reorder failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when ids field is missing", response.StatusCode)
	}
}

func TestReorderForwardsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/reorder", "application/json",
		strings.NewReader(`not-json`))
	if err != nil {
		t.Fatalf("POST /api/forwards/reorder failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for malformed JSON", response.StatusCode)
	}
}

func TestReorderForwardsUnknownID(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, "", "missing"))
	defer server.Close()

	response, err := http.Post(server.URL+"/api/forwards/reorder", "application/json",
		strings.NewReader(`{"ids":["no-such-id"]}`))
	if err != nil {
		t.Fatalf("POST /api/forwards/reorder failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for reorder with unknown rule ID", response.StatusCode)
	}
}
