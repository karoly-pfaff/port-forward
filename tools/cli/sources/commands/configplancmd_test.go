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

// --- shared plan fixtures ---

func planNoDriftFixture() map[string]any {
	return map[string]any{
		"generatedAt": "2026-01-01T00:00:00.000Z",
		"mode":        "plan",
		"summary": map[string]any{
			"add": 0, "update": 0, "remove": 0, "unchanged": 1,
			"destructive": 0, "hasDrift": false, "hasErrors": false,
		},
		"operations": []map[string]any{
			{
				"type": "unchanged", "ruleId": "r1", "ruleName": "Local DNS",
				"protocol": "udp",
				"current": map[string]any{
					"id": "r1", "name": "Local DNS", "protocol": "udp",
					"listenHost": "127.0.0.1", "listenPort": 5353,
					"targetHost": "8.8.8.8", "targetPort": 53, "enabled": true,
				},
				"desired": map[string]any{
					"name": "Local DNS", "protocol": "udp",
					"listenHost": "127.0.0.1", "listenPort": 5353,
					"targetHost": "8.8.8.8", "targetPort": 53, "enabled": true,
				},
				"changes": []any{}, "destructive": false,
			},
		},
		"errors": []any{}, "warnings": []any{},
	}
}

func planWithDriftFixture() map[string]any {
	return map[string]any{
		"generatedAt": "2026-01-01T00:00:00.000Z",
		"mode":        "plan",
		"summary": map[string]any{
			"add": 1, "update": 1, "remove": 1, "unchanged": 1,
			"destructive": 1, "hasDrift": true, "hasErrors": false,
		},
		"operations": []map[string]any{
			{
				"type": "add", "ruleName": "Local Redis", "protocol": "tcp",
				"desired": map[string]any{
					"name": "Local Redis", "protocol": "tcp",
					"listenHost": "127.0.0.1", "listenPort": 6379,
					"targetHost": "127.0.0.1", "targetPort": 6379, "enabled": true,
				},
				"changes": []any{}, "destructive": false,
			},
			{
				"type": "update", "ruleId": "r2", "ruleName": "Local Postgres", "protocol": "tcp",
				"current": map[string]any{
					"id": "r2", "name": "Local Postgres", "protocol": "tcp",
					"listenHost": "127.0.0.1", "listenPort": 15432,
					"targetHost": "127.0.0.1", "targetPort": 5432, "enabled": true,
				},
				"desired": map[string]any{
					"id": "r2", "name": "Local Postgres", "protocol": "tcp",
					"listenHost": "127.0.0.1", "listenPort": 15433,
					"targetHost": "127.0.0.1", "targetPort": 5433, "enabled": true,
				},
				"changes": []map[string]any{
					{"field": "listenPort", "before": 15432, "after": 15433},
					{"field": "targetPort", "before": 5432, "after": 5433},
				},
				"destructive": true,
			},
			{
				"type": "remove", "ruleId": "r3", "ruleName": "Old Test Tunnel", "protocol": "tcp",
				"current": map[string]any{
					"id": "r3", "name": "Old Test Tunnel", "protocol": "tcp",
					"listenHost": "127.0.0.1", "listenPort": 9999,
					"targetHost": "10.0.0.1", "targetPort": 9999, "enabled": true,
				},
				"changes": []any{}, "destructive": false,
			},
			{
				"type": "unchanged", "ruleId": "r4", "ruleName": "Local DNS", "protocol": "udp",
				"current": map[string]any{
					"id": "r4", "name": "Local DNS", "protocol": "udp",
					"listenHost": "127.0.0.1", "listenPort": 5353,
					"targetHost": "8.8.8.8", "targetPort": 53, "enabled": true,
				},
				"desired": map[string]any{
					"name": "Local DNS", "protocol": "udp",
					"listenHost": "127.0.0.1", "listenPort": 5353,
					"targetHost": "8.8.8.8", "targetPort": 53, "enabled": true,
				},
				"changes": []any{}, "destructive": false,
			},
		},
		"errors": []any{}, "warnings": []any{},
	}
}

func planWithErrorsFixture() map[string]any {
	return map[string]any{
		"generatedAt": "2026-01-01T00:00:00.000Z",
		"mode":        "plan",
		"summary": map[string]any{
			"add": 0, "update": 0, "remove": 0, "unchanged": 0,
			"destructive": 0, "hasDrift": false, "hasErrors": true,
		},
		"operations": []any{},
		"errors":     []map[string]any{{"code": "INVALID_DESIRED_RULE", "message": "rule 1: name is required"}},
		"warnings":   []any{},
	}
}

func planWithWarningsFixture() map[string]any {
	return map[string]any{
		"generatedAt": "2026-01-01T00:00:00.000Z",
		"mode":        "plan",
		"summary": map[string]any{
			"add": 1, "update": 0, "remove": 0, "unchanged": 0,
			"destructive": 0, "hasDrift": true, "hasErrors": false,
		},
		"operations": []map[string]any{
			{
				"type": "add", "ruleName": "LAN Rule", "protocol": "tcp",
				"desired": map[string]any{
					"name": "LAN Rule", "protocol": "tcp",
					"listenHost": "0.0.0.0", "listenPort": 48000,
					"targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true,
				},
				"changes": []any{}, "destructive": false,
			},
		},
		"errors":   []any{},
		"warnings": []map[string]any{{"code": "LAN_EXPOSURE", "message": "0.0.0.0 exposes rule on all interfaces"}},
	}
}

// planWithDriftAndErrorsFixture has both hasDrift and hasErrors true.
func planWithDriftAndErrorsFixture() map[string]any {
	return map[string]any{
		"generatedAt": "2026-01-01T00:00:00.000Z",
		"mode":        "plan",
		"summary": map[string]any{
			"add": 1, "update": 0, "remove": 0, "unchanged": 0,
			"destructive": 0, "hasDrift": true, "hasErrors": true,
		},
		"operations": []map[string]any{
			{
				"type": "add", "ruleName": "", "protocol": "tcp",
				"desired": map[string]any{
					"name": "", "protocol": "tcp",
					"listenHost": "127.0.0.1", "listenPort": 48000,
					"targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true,
				},
				"changes": []any{}, "destructive": false,
			},
		},
		"errors":   []map[string]any{{"code": "INVALID_DESIRED_RULE", "message": "rule 1: name is required"}},
		"warnings": []any{},
	}
}

func makePlanServer(t *testing.T, fixture map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/config/plan" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fixture)
			return
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
}

// --- RunConfigPlan tests ---

func TestRunConfigPlan_NoDrift_HumanOutput(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "No drift") {
		t.Errorf("output missing 'No drift': %s", out.String())
	}
}

func TestRunConfigPlan_WithDrift_Summary(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (no --fail-on-drift); stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "Config Plan") {
		t.Errorf("output missing 'Config Plan': %s", o)
	}
	if !strings.Contains(o, "Add: 1") {
		t.Errorf("output missing 'Add: 1': %s", o)
	}
	if !strings.Contains(o, "Remove: 1") {
		t.Errorf("output missing 'Remove: 1': %s", o)
	}
	if !strings.Contains(o, "Local Redis") {
		t.Errorf("output missing 'Local Redis': %s", o)
	}
	if !strings.Contains(o, "Old Test Tunnel") {
		t.Errorf("output missing 'Old Test Tunnel': %s", o)
	}
}

func TestRunConfigPlan_UpdateShowsChanges(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "listenPort") {
		t.Errorf("output missing 'listenPort' change: %s", o)
	}
	if !strings.Contains(o, "15432") {
		t.Errorf("output missing before value 15432: %s", o)
	}
	if !strings.Contains(o, "15433") {
		t.Errorf("output missing after value 15433: %s", o)
	}
	if !strings.Contains(o, "[destructive]") {
		t.Errorf("output missing '[destructive]': %s", o)
	}
}

func TestRunConfigPlan_WarningsRendered(t *testing.T) {
	srv := makePlanServer(t, planWithWarningsFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "Warnings:") {
		t.Errorf("output missing 'Warnings:': %s", o)
	}
	if !strings.Contains(o, "LAN_EXPOSURE") {
		t.Errorf("output missing 'LAN_EXPOSURE': %s", o)
	}
}

func TestRunConfigPlan_PlanErrors_NonzeroExit(t *testing.T) {
	srv := makePlanServer(t, planWithErrorsFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (plan errors); stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "Errors:") {
		t.Errorf("output missing 'Errors:': %s", o)
	}
	if !strings.Contains(o, "INVALID_DESIRED_RULE") {
		t.Errorf("output missing 'INVALID_DESIRED_RULE': %s", o)
	}
}

func TestRunConfigPlan_JSON_RawResponse(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, true, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["mode"] != "plan" {
		t.Errorf("mode = %v, want plan", result["mode"])
	}
	summary, ok := result["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary is not an object: %v", result["summary"])
	}
	if summary["hasDrift"] != true {
		t.Errorf("hasDrift = %v, want true", summary["hasDrift"])
	}
}

func TestRunConfigPlan_InvalidFilePath_Exit2(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{"/nonexistent/desired.json"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "Error reading") {
		t.Errorf("stderr missing 'Error reading': %s", errBuf.String())
	}
}

func TestRunConfigPlan_InvalidJSON_Exit2(t *testing.T) {
	apiCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(planNoDriftFixture())
	}))
	defer srv.Close()

	file := writeTempConfig(t, "not valid json {{{{")
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if apiCalled {
		t.Error("API should not be called for invalid JSON file")
	}
}

func TestRunConfigPlan_LocalValidationFail_NoAPICall(t *testing.T) {
	apiCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(planNoDriftFixture())
	}))
	defer srv.Close()

	// Invalid protocol — local validation fails
	file := writeTempConfig(t, `[{"name":"Bad","protocol":"quic","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false}]`)
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if apiCalled {
		t.Error("API should not be called when local validation fails")
	}
}

func TestRunConfigPlan_APIError_Exit1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"internal error"}})
	}))
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (API error)", code)
	}
}

func TestRunConfigPlan_ConnectionError_Exit3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3 (connection failure)", code)
	}
}

func TestRunConfigPlan_FailOnDrift_Exit4(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{"--fail-on-drift", file}, &out, &errBuf)
	if code != 4 {
		t.Errorf("exit code = %d, want 4 (drift + --fail-on-drift); stderr: %s", code, errBuf.String())
	}
	// Output should still be printed
	if !strings.Contains(out.String(), "Config Plan") {
		t.Errorf("output should still be printed with --fail-on-drift: %s", out.String())
	}
}

func TestRunConfigPlan_FailOnDrift_NoDrift_Exit0(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{"--fail-on-drift", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (no drift); stderr: %s", code, errBuf.String())
	}
}

func TestRunConfigPlan_PlanErrors_TakePrecedenceOverDrift(t *testing.T) {
	srv := makePlanServer(t, planWithDriftAndErrorsFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	// hasDrift=true AND hasErrors=true — plan errors win → exit 1, not 4
	code := commands.RunConfigPlan(c, false, []string{"--fail-on-drift", file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (plan errors take precedence over drift)", code)
	}
}

func TestRunConfigPlan_MissingArg_Exit2(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunConfigPlan_Help(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "portier config plan") {
		t.Errorf("help output missing usage: %s", out.String())
	}
}

// --- RunConfigDiff tests ---

func TestRunConfigDiff_AddOutput(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "+ ") {
		t.Errorf("output missing '+' prefix for add: %s", o)
	}
	if !strings.Contains(o, "Local Redis") {
		t.Errorf("output missing 'Local Redis': %s", o)
	}
}

func TestRunConfigDiff_UpdateOutput_WithChanges(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "~ ") {
		t.Errorf("output missing '~' prefix for update: %s", o)
	}
	if !strings.Contains(o, "Local Postgres") {
		t.Errorf("output missing 'Local Postgres': %s", o)
	}
	if !strings.Contains(o, "listenPort") {
		t.Errorf("output missing 'listenPort' change: %s", o)
	}
	if !strings.Contains(o, "15432") {
		t.Errorf("output missing before value 15432: %s", o)
	}
	if !strings.Contains(o, "[destructive]") {
		t.Errorf("output missing '[destructive]': %s", o)
	}
}

func TestRunConfigDiff_RemoveOutput(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "- ") {
		t.Errorf("output missing '-' prefix for remove: %s", o)
	}
	if !strings.Contains(o, "Old Test Tunnel") {
		t.Errorf("output missing 'Old Test Tunnel': %s", o)
	}
}

func TestRunConfigDiff_HidesUnchangedByDefault(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	o := out.String()
	// "Local DNS" is unchanged — should not appear without --show-unchanged
	if strings.Contains(o, "= ") {
		t.Errorf("output should not contain '=' (unchanged) without --show-unchanged: %s", o)
	}
}

func TestRunConfigDiff_ShowUnchanged(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{"--show-unchanged", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "= ") {
		t.Errorf("output missing '=' (unchanged) with --show-unchanged: %s", o)
	}
	if !strings.Contains(o, "Local DNS") {
		t.Errorf("output missing 'Local DNS' with --show-unchanged: %s", o)
	}
}

func TestRunConfigDiff_NoDrift(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "No drift") {
		t.Errorf("output missing 'No drift': %s", out.String())
	}
}

func TestRunConfigDiff_JSON(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, true, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["mode"] != "plan" {
		t.Errorf("mode = %v, want plan", result["mode"])
	}
}

func TestRunConfigDiff_FailOnDrift_Exit4(t *testing.T) {
	srv := makePlanServer(t, planWithDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{"--fail-on-drift", file}, &out, &errBuf)
	if code != 4 {
		t.Errorf("exit code = %d, want 4; stderr: %s", code, errBuf.String())
	}
}

func TestRunConfigDiff_MissingArg_Exit2(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunConfigDiff_InvalidFile_Exit2(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{"/nonexistent/file.json"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunConfigDiff_Help(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "portier config diff") {
		t.Errorf("help output missing usage: %s", out.String())
	}
}

func TestRunConfigDiff_LocalValidationFail_NoAPICall(t *testing.T) {
	apiCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(planNoDriftFixture())
	}))
	defer srv.Close()

	file := writeTempConfig(t, `[{"name":"","protocol":"tcp","listenHost":"127.0.0.1","listenPort":48000,"targetHost":"10.0.0.1","targetPort":8080,"enabled":false}]`)
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{file}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if apiCalled {
		t.Error("API should not be called when local validation fails")
	}
}

func TestRunConfigDiff_ConnectionError_Exit3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(addr)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{file}, &out, &errBuf)
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

// --- RunConfig dispatch tests for plan and diff ---

func TestRunConfig_PlanDispatch(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfig(c, false, []string{"plan", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "No drift") {
		t.Errorf("output missing 'No drift': %s", out.String())
	}
}

func TestRunConfig_DiffDispatch(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfig(c, false, []string{"diff", file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "No drift") {
		t.Errorf("output missing 'No drift': %s", out.String())
	}
}

func TestRunConfig_HelpIncludesPlanAndDiff(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfig(c, false, []string{"help"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "plan") {
		t.Errorf("config help missing 'plan': %s", out.String())
	}
	if !strings.Contains(out.String(), "diff") {
		t.Errorf("config help missing 'diff': %s", out.String())
	}
}

func TestRunConfigPlan_UnknownFlag_Exit2(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{"--unknown-flag"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "Error") {
		t.Errorf("stderr should contain 'Error': %s", errBuf.String())
	}
}

func TestRunConfigDiff_UnknownFlag_Exit2(t *testing.T) {
	srv := makePlanServer(t, planNoDriftFixture())
	defer srv.Close()

	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{"--unknown-flag"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "Error") {
		t.Errorf("stderr should contain 'Error': %s", errBuf.String())
	}
}

func TestRunConfigDiff_WarningsShown(t *testing.T) {
	srv := makePlanServer(t, planWithWarningsFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "LAN_EXPOSURE") {
		t.Errorf("diff output missing 'LAN_EXPOSURE' warning: %s", out.String())
	}
}

func TestRunConfigDiff_ErrorsShown(t *testing.T) {
	srv := makePlanServer(t, planWithDriftAndErrorsFixture())
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigDiff(c, false, []string{file}, &out, &errBuf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (plan errors); stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Errors:") {
		t.Errorf("diff output missing 'Errors:': %s", out.String())
	}
	if !strings.Contains(out.String(), "INVALID_DESIRED_RULE") {
		t.Errorf("diff output missing 'INVALID_DESIRED_RULE': %s", out.String())
	}
}

func TestRunConfigPlan_NilChangeValue(t *testing.T) {
	// A fixture where a change has a null "before" value — tests formatChangeValue nil path.
	fixture := map[string]any{
		"generatedAt": "2026-01-01T00:00:00.000Z",
		"mode":        "plan",
		"summary": map[string]any{
			"add": 0, "update": 1, "remove": 0, "unchanged": 0,
			"destructive": 0, "hasDrift": true, "hasErrors": false,
		},
		"operations": []map[string]any{
			{
				"type": "update", "ruleId": "r1", "ruleName": "UDP Rule", "protocol": "udp",
				"current": map[string]any{
					"id": "r1", "name": "UDP Rule", "protocol": "udp",
					"listenHost": "127.0.0.1", "listenPort": 48000,
					"targetHost": "10.0.0.1", "targetPort": 9000, "enabled": true,
					// no udpMode set
				},
				"desired": map[string]any{
					"id": "r1", "name": "UDP Rule", "protocol": "udp",
					"listenHost": "127.0.0.1", "listenPort": 48000,
					"targetHost": "10.0.0.1", "targetPort": 9000, "enabled": true,
					"udpMode": "one-way",
				},
				"changes": []map[string]any{
					{"field": "udpMode", "before": nil, "after": "one-way"},
				},
				"destructive": false,
			},
		},
		"errors": []any{}, "warnings": []any{},
	}
	srv := makePlanServer(t, fixture)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "(none)") {
		t.Errorf("output missing '(none)' for nil change value: %s", out.String())
	}
	if !strings.Contains(out.String(), "one-way") {
		t.Errorf("output missing 'one-way' for after value: %s", out.String())
	}
}

func TestRunConfigPlan_WarningsOnNoDriftPath(t *testing.T) {
	// A plan with no drift but with warnings (e.g. LAN exposure on unchanged rule).
	fixture := map[string]any{
		"generatedAt": "2026-01-01T00:00:00.000Z",
		"mode":        "plan",
		"summary": map[string]any{
			"add": 0, "update": 0, "remove": 0, "unchanged": 1,
			"destructive": 0, "hasDrift": false, "hasErrors": false,
		},
		"operations": []map[string]any{
			{
				"type": "unchanged", "ruleName": "LAN Rule", "protocol": "tcp",
				"current": map[string]any{
					"name": "LAN Rule", "protocol": "tcp",
					"listenHost": "0.0.0.0", "listenPort": 48000,
					"targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true,
				},
				"desired": map[string]any{
					"name": "LAN Rule", "protocol": "tcp",
					"listenHost": "0.0.0.0", "listenPort": 48000,
					"targetHost": "10.0.0.1", "targetPort": 8080, "enabled": true,
				},
				"changes": []any{}, "destructive": false,
			},
		},
		"errors":   []any{},
		"warnings": []map[string]any{{"code": "LAN_EXPOSURE", "message": "0.0.0.0 exposes rule"}},
	}
	srv := makePlanServer(t, fixture)
	defer srv.Close()

	file := writeTempConfig(t, validConfigJSON())
	c := client.New(srv.URL)
	var out, errBuf strings.Builder
	code := commands.RunConfigPlan(c, false, []string{file}, &out, &errBuf)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	o := out.String()
	if !strings.Contains(o, "No drift") {
		t.Errorf("output missing 'No drift': %s", o)
	}
	if !strings.Contains(o, "LAN_EXPOSURE") {
		t.Errorf("output missing 'LAN_EXPOSURE' warning on no-drift path: %s", o)
	}
}
